package updater

import (
	"archive/tar"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zhaoxinyi02/ClawPanel/internal/updatemirror"
)

const (
	UpdaterPort        = 19528
	TokenValidDuration = 5 * time.Minute
	TokenSecret        = "clawpanel-updater-secret-2026"
)

// UpdateStep represents a step in the update process
type UpdateStep struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pending, running, done, error, skipped
	Message string `json:"message,omitempty"`
}

// UpdateState represents the full update state
type UpdateState struct {
	Phase      string       `json:"phase"` // idle, validating, checking, stopping, downloading, backing_up, replacing, starting, done, error, rolled_back
	Steps      []UpdateStep `json:"steps"`
	Progress   int          `json:"progress"` // 0-100
	Message    string       `json:"message"`
	Log        []string     `json:"log"`
	Error      string       `json:"error,omitempty"`
	StartedAt  string       `json:"started_at,omitempty"`
	FinishedAt string       `json:"finished_at,omitempty"`
	Source     string       `json:"source,omitempty"` // github, accel, upload
	FromVer    string       `json:"from_ver,omitempty"`
	ToVer      string       `json:"to_ver,omitempty"`
}

// VersionInfo from remote
type VersionInfo struct {
	LatestVersion string            `json:"latest_version"`
	ReleaseTime   string            `json:"release_time"`
	ReleaseNote   string            `json:"release_note"`
	DownloadURLs  map[string]string `json:"download_urls"`
	SHA256        map[string]string `json:"sha256"`
	MajorChange   bool              `json:"major_change,omitempty"`
	ChangeWarning string            `json:"change_warning,omitempty"`
}

// Server is the independent updater HTTP server
type Server struct {
	currentVersion string
	dataDir        string
	openClawDir    string // OpenClaw config directory
	panelBin       string // path to clawpanel binary
	panelPort      int
	editionCfg     editionConfig
	mu             sync.Mutex
	state          UpdateState // ClawPanel update state
	ocState        UpdateState // OpenClaw update state
	srv            *http.Server
	running        bool
}

// NewServer creates a new updater server
func NewServer(currentVersion, dataDir, openClawDir string, panelPort int, edition string) *Server {
	bin, _ := os.Executable()
	bin, _ = filepath.EvalSymlinks(bin)
	return &Server{
		currentVersion: currentVersion,
		dataDir:        dataDir,
		openClawDir:    openClawDir,
		panelBin:       bin,
		panelPort:      panelPort,
		editionCfg:     newEditionConfig(edition),
		state: UpdateState{
			Phase: "idle",
			Steps: defaultSteps(),
			Log:   []string{},
		},
		ocState: UpdateState{
			Phase: "idle",
			Steps: defaultOCSteps(),
			Log:   []string{},
		},
	}
}

func defaultSteps() []UpdateStep {
	return []UpdateStep{
		{Name: "验证授权", Status: "pending"},
		{Name: "检测版本", Status: "pending"},
		{Name: "停止服务", Status: "pending"},
		{Name: "下载更新", Status: "pending"},
		{Name: "备份文件", Status: "pending"},
		{Name: "替换文件", Status: "pending"},
		{Name: "启动服务", Status: "pending"},
	}
}

func defaultOCSteps() []UpdateStep {
	return []UpdateStep{
		{Name: "验证授权", Status: "pending"},
		{Name: "检测版本", Status: "pending"},
		{Name: "执行更新", Status: "pending"},
		{Name: "重启服务", Status: "pending"},
	}
}

// GenerateToken generates a temporary auth token for the updater page
func GenerateToken(panelPort int) string {
	now := time.Now().Unix()
	payload := fmt.Sprintf("%d:%d", panelPort, now)
	mac := hmac.New(sha256.New, []byte(TokenSecret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%d", sig, now)
}

// ValidateToken validates a token
func ValidateToken(token string, panelPort int) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	sig := parts[0]
	tsStr := parts[1]
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}
	// Check expiry
	if time.Since(time.Unix(ts, 0)) > TokenValidDuration {
		return false
	}
	// Verify signature
	payload := fmt.Sprintf("%d:%d", panelPort, ts)
	mac := hmac.New(sha256.New, []byte(TokenSecret))
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expectedSig))
}

// Start spawns the updater as a detached child process.
// Uses systemd-run --scope to launch outside the clawpanel.service cgroup,
// so systemctl stop clawpanel won't kill the updater child.
func (s *Server) Start() {
	// Kill any leftover standalone updater from a previous run
	s.killStandaloneUpdater()
	time.Sleep(500 * time.Millisecond)

	bin := s.panelBin
	logFile := filepath.Join(s.dataDir, "updater.log")

	if runtime.GOOS != "windows" {
		// Use systemd-run --scope to escape the parent's cgroup.
		// Use a timestamp-based unit name to avoid collision on rapid restarts.
		unitName := fmt.Sprintf("clawpanel-updater-%d", time.Now().Unix())
		cmd := exec.Command("systemd-run", "--scope", "--unit="+unitName,
			"/bin/bash", "-c",
			fmt.Sprintf("%s --updater-standalone %s %s %d %s >%s 2>&1",
				bin, s.currentVersion, s.dataDir, s.panelPort, s.openClawDir, logFile),
		)
		cmd.SysProcAttr = sysProcAttr()
		cmd.Dir = filepath.Dir(bin)
		if err := cmd.Start(); err != nil {
			if isLikelySystemdServiceProcess() {
				log.Printf("[Updater] systemd-run 启动失败: %v，当前运行在 systemd service 中，已拒绝 direct 模式以避免停服后更新器被连带终止", err)
				return
			}
			log.Printf("[Updater] systemd-run 启动失败: %v, 尝试直接启动...", err)
			s.startDirectChild(bin, logFile)
			return
		}
		// Wait briefly then check if systemd-run itself failed (e.g. unit name conflict)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			// systemd-run exited immediately — inner command failed to launch
			if isLikelySystemdServiceProcess() {
				log.Printf("[Updater] systemd-run 立即退出 (err=%v)，当前运行在 systemd service 中，已拒绝 direct 模式以避免停服后更新器被连带终止", err)
				return
			}
			log.Printf("[Updater] systemd-run 立即退出 (err=%v), 尝试直接启动...", err)
			s.startDirectChild(bin, logFile)
			return
		case <-time.After(800 * time.Millisecond):
			// Still running after 800ms — assume it successfully launched the child
			cmd.Process.Release()
		}
	} else {
		s.startDirectChild(bin, logFile)
		return
	}

	s.running = true
	log.Printf("[Updater] 独立更新子进程已启动 (systemd-run scope) → http://0.0.0.0:%d/updater", UpdaterPort)
}

func isLikelySystemdServiceProcess() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}
	if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		text := string(data)
		if strings.Contains(text, ".service") || strings.Contains(text, "system.slice") {
			return true
		}
	}
	return false
}

// startDirectChild starts the updater as a direct detached child (fallback for non-systemd or Windows)
func (s *Server) startDirectChild(bin, logFile string) {
	cmd := exec.Command(bin,
		"--updater-standalone",
		s.currentVersion,
		s.dataDir,
		fmt.Sprintf("%d", s.panelPort),
		s.openClawDir,
	)
	cmd.SysProcAttr = sysProcAttr()
	cmd.Dir = filepath.Dir(bin)
	lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		cmd.Stdout = lf
		cmd.Stderr = lf
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[Updater] 启动独立更新子进程失败: %v", err)
		return
	}
	cmd.Process.Release()
	s.running = true
	log.Printf("[Updater] 独立更新子进程已启动 (direct) → http://0.0.0.0:%d/updater", UpdaterPort)
}

// Stop kills the standalone updater child process
func (s *Server) Stop() {
	s.killStandaloneUpdater()
}

// killStandaloneUpdater kills any running standalone updater process
func (s *Server) killStandaloneUpdater() {
	if runtime.GOOS == "windows" {
		return
	}
	// Use grep via shell to avoid --updater-standalone being parsed as pgrep flag
	out, _ := exec.Command("sh", "-c", "pgrep -f 'updater-standalone'").Output()
	pids := strings.Fields(strings.TrimSpace(string(out)))
	myPid := fmt.Sprintf("%d", os.Getpid())
	for _, pid := range pids {
		if pid == myPid {
			continue
		}
		exec.Command("kill", pid).Run()
	}
}

// IsRunning returns whether the updater server is running
func (s *Server) IsRunning() bool {
	return s.running
}

// RunStandalone runs the updater as a standalone process (called from --updater-standalone mode).
// This is a BLOCKING call — it runs the HTTP server and only exits after the update is done
// and an auto-shutdown timer fires (5 minutes after update completes or 30 minutes idle).
func (s *Server) RunStandalone() {
	mux := http.NewServeMux()
	mux.HandleFunc("/updater", s.handlePage)
	mux.HandleFunc("/updater/", s.handlePage)
	mux.HandleFunc("/updater/api/validate", s.handleValidate)
	mux.HandleFunc("/updater/api/check-version", s.handleCheckVersion)
	mux.HandleFunc("/updater/api/start-update", s.handleStartUpdate)
	mux.HandleFunc("/updater/api/upload-update", s.handleUploadUpdate)
	mux.HandleFunc("/updater/api/progress", s.handleProgress)
	// OpenClaw update endpoints
	mux.HandleFunc("/updater/api/check-openclaw-version", s.handleCheckOCVersion)
	mux.HandleFunc("/updater/api/start-openclaw-update", s.handleStartOCUpdate)
	mux.HandleFunc("/updater/api/openclaw-progress", s.handleOCProgress)

	s.srv = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", UpdaterPort),
		Handler: mux,
	}
	s.running = true

	// Auto-shutdown: exit after 30 min idle or 5 min after both updates finish
	go func() {
		for {
			time.Sleep(10 * time.Second)
			s.mu.Lock()
			phase := s.state.Phase
			finishedAt := s.state.FinishedAt
			ocPhase := s.ocState.Phase
			ocFinishedAt := s.ocState.FinishedAt
			s.mu.Unlock()

			// Don't exit if either update is still running
			panelDone := phase == "idle" || phase == "done" || phase == "error" || phase == "rolled_back"
			ocDone := ocPhase == "idle" || ocPhase == "done" || ocPhase == "error" || ocPhase == "rolled_back"
			if !panelDone || !ocDone {
				continue
			}

			// Check if at least one update was performed and finished > 5 min ago
			latestFinish := finishedAt
			if ocFinishedAt > latestFinish {
				latestFinish = ocFinishedAt
			}
			if latestFinish != "" {
				if t, err := time.Parse(time.RFC3339, latestFinish); err == nil {
					if time.Since(t) > 5*time.Minute {
						log.Println("[Updater-Standalone] 更新完成超过5分钟，自动退出")
						s.srv.Close()
						return
					}
				}
			}
		}
	}()

	log.Printf("[Updater-Standalone] 独立更新服务已启动 → http://0.0.0.0:%d/updater", UpdaterPort)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("[Updater-Standalone] 服务启动失败: %v", err)
	}
	log.Println("[Updater-Standalone] 服务已退出")
}

// --- HTTP Handlers ---

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	// Only allow access with valid token parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "⛔ 禁止直接访问更新页面。请从 ClawPanel 面板的「版本管理」页面点击「前往更新」进入。", http.StatusForbidden)
		return
	}
	if !ValidateToken(token, s.panelPort) {
		http.Error(w, "⛔ 授权令牌已失效或无效。请返回 ClawPanel 面板重新点击「前往更新」。", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(updaterHTML(s.currentVersion, token, s.panelPort, s.editionCfg.Edition)))
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	token := r.URL.Query().Get("token")
	valid := ValidateToken(token, s.panelPort)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    valid,
		"error": ternary(!valid, "授权令牌无效或已过期", ""),
	})
}

func (s *Server) handleCheckVersion(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if !s.checkToken(w, r) {
		return
	}

	info, err := s.resolveLocalVersion(false)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	hasUpdate := info.LatestVersion != "" && info.LatestVersion != s.currentVersion &&
		isNewerVersion(info.LatestVersion, s.currentVersion)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":              true,
		"currentVersion":  s.currentVersion,
		"latestVersion":   info.LatestVersion,
		"releaseTime":     info.ReleaseTime,
		"releaseNote":     info.ReleaseNote,
		"hasUpdate":       hasUpdate,
		"source":          "local",
		"preferredSource": "local",
		"edition":         s.editionCfg.Edition,
		"fullPackage":     s.editionCfg.isLiteFullPackage(),
		"majorChange":     info.MajorChange,
		"changeWarning":   info.ChangeWarning,
	})
}

func (s *Server) handleStartUpdate(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if !s.checkToken(w, r) {
		return
	}

	s.mu.Lock()
	if s.state.Phase != "idle" && s.state.Phase != "done" && s.state.Phase != "error" && s.state.Phase != "rolled_back" {
		s.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": "更新正在进行中",
		})
		return
	}
	s.state = UpdateState{
		Phase:     "validating",
		Steps:     defaultSteps(),
		Log:       []string{},
		StartedAt: time.Now().Format(time.RFC3339),
	}
	s.mu.Unlock()

	go s.doUpdate("local")

	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (s *Server) handleUploadUpdate(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if !s.checkToken(w, r) {
		return
	}

	s.mu.Lock()
	if s.state.Phase != "idle" && s.state.Phase != "done" && s.state.Phase != "error" && s.state.Phase != "rolled_back" {
		s.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": "更新正在进行中",
		})
		return
	}
	s.mu.Unlock()

	// Parse multipart (max 512MB, Lite full package may exceed 200MB)
	r.ParseMultipartForm(512 << 20)
	file, _, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": "读取上传文件失败: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// Save to temp
	tmpDir := filepath.Join(s.dataDir, "update-tmp")
	os.MkdirAll(tmpDir, 0755)
	tmpFile := filepath.Join(tmpDir, "clawpanel-upload")
	if s.editionCfg.isLiteFullPackage() {
		tmpFile += ".tar.gz"
	} else if runtime.GOOS == "windows" {
		tmpFile += ".exe"
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": "保存文件失败: " + err.Error(),
		})
		return
	}
	io.Copy(out, file)
	out.Close()
	os.Chmod(tmpFile, 0755)

	s.mu.Lock()
	s.state = UpdateState{
		Phase:     "validating",
		Steps:     defaultSteps(),
		Log:       []string{},
		StartedAt: time.Now().Format(time.RFC3339),
		Source:    "upload",
	}
	// Skip download step for upload
	s.state.Steps[3].Status = "skipped"
	s.state.Steps[3].Message = "使用本地上传文件"
	s.mu.Unlock()

	go s.doUpdateWithFile(tmpFile)

	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	// Progress endpoint does NOT require token — the update page needs to keep
	// polling even after the token expires during a long update.
	s.mu.Lock()
	state := s.state
	logCopy := make([]string, len(s.state.Log))
	copy(logCopy, s.state.Log)
	state.Log = logCopy
	stepsCopy := make([]UpdateStep, len(s.state.Steps))
	copy(stepsCopy, s.state.Steps)
	state.Steps = stepsCopy
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"state": state,
	})
}

// --- OpenClaw Update Handlers ---

func (s *Server) handleCheckOCVersion(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if !s.checkToken(w, r) {
		return
	}
	if s.editionCfg.isLiteFullPackage() {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":             true,
			"currentVersion": "managed-by-lite-package",
			"latestVersion":  "managed-by-lite-package",
			"hasUpdate":      false,
		})
		return
	}

	// Get current installed version via 'openclaw --version'
	currentVersion := "unknown"
	if verOut, verErr := exec.Command("openclaw", "--version").Output(); verErr == nil {
		currentVersion = strings.TrimSpace(string(verOut))
	}
	// Fallback: read from openclaw.json meta
	if currentVersion == "unknown" {
		cfgPath := filepath.Join(s.openClawDir, "openclaw.json")
		if data, err := os.ReadFile(cfgPath); err == nil {
			var cfg map[string]interface{}
			if json.Unmarshal(data, &cfg) == nil {
				if meta, ok := cfg["meta"].(map[string]interface{}); ok {
					if v, ok := meta["lastTouchedVersion"].(string); ok {
						currentVersion = v
					}
				}
			}
		}
	}

	// Check latest version via npm view
	latestVersion := ""
	cmd := exec.Command("npm", "view", "openclaw", "version")
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/usr/local/bin:/usr/bin:/bin:/snap/bin")
	if out, err := cmd.Output(); err == nil {
		latestVersion = strings.TrimSpace(string(out))
	}

	hasUpdate := latestVersion != "" && latestVersion != currentVersion && latestVersion > currentVersion

	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":             true,
		"currentVersion": currentVersion,
		"latestVersion":  latestVersion,
		"hasUpdate":      hasUpdate,
	})
}

func (s *Server) handleStartOCUpdate(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if s.editionCfg.isLiteFullPackage() {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "Lite 版使用整包更新，不支持单独更新 OpenClaw",
		})
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if !s.checkToken(w, r) {
		return
	}

	s.mu.Lock()
	if s.ocState.Phase != "idle" && s.ocState.Phase != "done" && s.ocState.Phase != "error" && s.ocState.Phase != "rolled_back" {
		s.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": "OpenClaw 更新正在进行中",
		})
		return
	}
	s.ocState = UpdateState{
		Phase:     "validating",
		Steps:     defaultOCSteps(),
		Log:       []string{},
		StartedAt: time.Now().Format(time.RFC3339),
	}
	s.mu.Unlock()

	go s.doOCUpdate()

	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func (s *Server) handleOCProgress(w http.ResponseWriter, r *http.Request) {
	s.setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	s.mu.Lock()
	state := s.ocState
	logCopy := make([]string, len(s.ocState.Log))
	copy(logCopy, s.ocState.Log)
	state.Log = logCopy
	stepsCopy := make([]UpdateStep, len(s.ocState.Steps))
	copy(stepsCopy, s.ocState.Steps)
	state.Steps = stepsCopy
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"state": state,
	})
}

// --- OpenClaw Update Logic ---

func (s *Server) doOCUpdate() {
	if s.editionCfg.isLiteFullPackage() {
		s.setOCError("Lite 版使用整包更新，不支持单独更新 OpenClaw")
		return
	}

	// Step 1: Validate
	s.setOCStep(0, "running", "验证授权中...")
	s.ocLog("🔐 验证更新授权...")
	s.setOCStep(0, "done", "授权验证通过")
	s.setOCProgress(10)

	// Step 2: Check version
	s.setOCStep(1, "running", "正在检测 OpenClaw 版本...")
	s.ocLog("🔍 检测 OpenClaw 版本...")

	currentVersion := "unknown"
	// Use 'openclaw --version' to get the actual installed binary version
	if verOut, verErr := exec.Command("openclaw", "--version").Output(); verErr == nil {
		currentVersion = strings.TrimSpace(string(verOut))
	}
	// Fallback: read from openclaw.json meta.lastTouchedVersion
	cfgPath := filepath.Join(s.openClawDir, "openclaw.json")
	if currentVersion == "unknown" {
		if data, err := os.ReadFile(cfgPath); err == nil {
			var cfg map[string]interface{}
			if json.Unmarshal(data, &cfg) == nil {
				if meta, ok := cfg["meta"].(map[string]interface{}); ok {
					if v, ok := meta["lastTouchedVersion"].(string); ok {
						currentVersion = v
					}
				}
			}
		}
	}
	s.mu.Lock()
	s.ocState.FromVer = currentVersion
	s.mu.Unlock()

	latestVersion := ""
	cmd := exec.Command("npm", "view", "openclaw", "version")
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/usr/local/bin:/usr/bin:/bin:/snap/bin")
	if out, err := cmd.Output(); err == nil {
		latestVersion = strings.TrimSpace(string(out))
	}

	if latestVersion == "" {
		s.ocLog("⚠️ 无法获取最新版本号，继续执行更新...")
	} else {
		s.ocLog("📦 当前版本: %s → 最新版本: %s", currentVersion, latestVersion)
		s.mu.Lock()
		s.ocState.ToVer = latestVersion
		s.mu.Unlock()
	}
	s.setOCStep(1, "done", fmt.Sprintf("当前: %s → 最新: %s", currentVersion, ternary(latestVersion == "", "未知", latestVersion)))
	s.setOCProgress(20)

	// Step 3: Execute update via npm install -g openclaw@latest
	// We use npm instead of 'openclaw update' because 'openclaw update' may update
	// a different installation (e.g. /usr/lib) than the one actually in PATH (e.g. nvm).
	// Using 'npm install -g' from PATH ensures the correct installation gets updated.
	s.setOCStep(2, "running", "正在更新 OpenClaw ...")

	// Find which npm to use (same one that manages the openclaw in PATH)
	npmBin := "npm"
	envPath := os.Getenv("PATH") + ":/usr/local/bin:/usr/bin:/bin:/snap/bin"
	// Try to find npm in the same directory as openclaw
	if ocBin, err := exec.LookPath("openclaw"); err == nil {
		ocDir := filepath.Dir(ocBin)
		candidate := filepath.Join(ocDir, "npm")
		if _, err := os.Stat(candidate); err == nil {
			npmBin = candidate
			s.ocLog("� 使用 npm: %s (与 openclaw 同目录)", npmBin)
		}
	}

	targetVersion := "latest"
	if latestVersion != "" {
		targetVersion = latestVersion
	}
	s.ocLog("🚀 执行 %s install -g openclaw@%s ...", npmBin, targetVersion)

	updateCmd := exec.Command(npmBin, "install", "-g", "openclaw@"+targetVersion)
	updateCmd.Env = append(os.Environ(), "PATH="+envPath)
	updateCmd.Dir = filepath.Dir(s.openClawDir)

	// Capture stdout+stderr in real time via pipe
	stdout, err := updateCmd.StdoutPipe()
	if err != nil {
		s.setOCStepError(2, "创建输出管道失败: "+err.Error())
		s.setOCError("创建输出管道失败: " + err.Error())
		return
	}
	updateCmd.Stderr = updateCmd.Stdout // merge stderr into stdout

	if err := updateCmd.Start(); err != nil {
		s.setOCStepError(2, "启动 npm install 失败: "+err.Error())
		s.setOCError("启动 npm install 失败: " + err.Error())
		return
	}

	// Read output line by line, collect all output for post-analysis
	var allOutput []string
	var outputMu sync.Mutex
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				lines := strings.Split(string(buf[:n]), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" {
						s.ocLog("%s", line)
						outputMu.Lock()
						allOutput = append(allOutput, line)
						outputMu.Unlock()
						s.mu.Lock()
						pct := s.ocState.Progress
						s.mu.Unlock()
						if pct < 80 {
							s.setOCProgress(pct + 2)
						}
					}
				}
			}
			if err != nil {
				break
			}
		}
	}()

	err = updateCmd.Wait()
	if err != nil {
		exitErr := ""
		if e, ok := err.(*exec.ExitError); ok {
			exitErr = fmt.Sprintf("退出码: %d", e.ExitCode())
		} else {
			exitErr = err.Error()
		}
		s.ocLog("❌ npm install 失败: %s", exitErr)
		s.setOCStepError(2, "更新失败: "+exitErr)
		s.setOCError("npm install -g openclaw 失败: " + exitErr)
		return
	}

	// Verify the new version
	verCmd := exec.Command("openclaw", "--version")
	verCmd.Env = append(os.Environ(), "PATH="+envPath)
	if verOut, verErr := verCmd.Output(); verErr == nil {
		newVer := strings.TrimSpace(string(verOut))
		s.ocLog("📦 更新后版本: %s", newVer)
		s.mu.Lock()
		s.ocState.ToVer = newVer
		s.mu.Unlock()
	}

	s.setOCStep(2, "done", "OpenClaw 更新完成")
	s.ocLog("✅ OpenClaw 更新完成")
	s.setOCProgress(80)

	// Step 4: Restart gateway daemon
	// Kill the running openclaw-gateway daemon process so ClawPanel's
	// monitorDaemon detects the port going down and auto-restarts with the new binary.
	s.setOCStep(3, "running", "正在重启 OpenClaw 网关...")
	s.ocLog("🔄 终止旧网关守护进程...")

	// Kill openclaw-gateway daemon processes
	killCmd := exec.Command("pkill", "-f", "openclaw-gateway")
	killCmd.Run() // ignore error (may not be running)

	s.ocLog("⏳ 等待 ClawPanel 自动重启网关...")
	// Wait for ClawPanel's monitorDaemon to detect port down and restart
	time.Sleep(15 * time.Second)

	s.setOCStep(3, "done", "网关已重启")
	s.ocLog("✅ 网关重启完成")
	s.setOCProgress(100)

	// Verify new version via command
	newVersion := ""
	finalVerCmd := exec.Command("openclaw", "--version")
	finalVerCmd.Env = append(os.Environ(), "PATH="+envPath)
	if out, err := finalVerCmd.Output(); err == nil {
		newVersion = strings.TrimSpace(string(out))
	}
	if newVersion != "" && newVersion != currentVersion {
		s.mu.Lock()
		s.ocState.ToVer = newVersion
		s.mu.Unlock()
		s.ocLog("🎉 OpenClaw 更新完成！%s → %s", currentVersion, newVersion)
	} else {
		s.ocLog("🎉 OpenClaw 更新完成！")
	}

	s.mu.Lock()
	s.ocState.Phase = "done"
	s.ocState.Message = "OpenClaw 更新完成！"
	s.ocState.FinishedAt = time.Now().Format(time.RFC3339)
	s.mu.Unlock()
}

// --- OpenClaw state helpers ---

func (s *Server) setOCStep(idx int, status, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < len(s.ocState.Steps) {
		s.ocState.Steps[idx].Status = status
		s.ocState.Steps[idx].Message = message
	}
}

func (s *Server) setOCStepError(idx int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < len(s.ocState.Steps) {
		s.ocState.Steps[idx].Status = "error"
		s.ocState.Steps[idx].Message = message
	}
	s.ocState.Phase = "error"
	s.ocState.FinishedAt = time.Now().Format(time.RFC3339)
}

func (s *Server) setOCProgress(pct int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ocState.Progress = pct
}

func (s *Server) setOCError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ocState.Phase = "error"
	s.ocState.Error = msg
	s.ocState.Message = "更新失败"
	s.ocState.FinishedAt = time.Now().Format(time.RFC3339)
	s.ocState.Log = append(s.ocState.Log, "❌ "+msg)
}

func (s *Server) ocLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[Updater-OC] %s", msg)
	s.mu.Lock()
	s.ocState.Log = append(s.ocState.Log, msg)
	s.mu.Unlock()
}

// --- Update Logic ---

func (s *Server) doUpdate(preferredSource string) {
	// Step 1: Validate
	s.setStep(0, "running", "验证授权中...")
	s.logMsg("🔐 验证更新授权...")
	s.setStep(0, "done", "授权验证通过")
	s.setProgress(5)

	// Step 2: Check version
	s.setStep(1, "running", "正在检测最新版本...")
	s.logMsg("🔍 检测最新版本...")
	info, err := s.resolveLocalVersion(false)
	if err != nil {
		s.setStepError(1, "检测版本失败: "+err.Error())
		s.setError("检测版本失败: " + err.Error())
		return
	}
	if !isNewerVersion(info.LatestVersion, s.currentVersion) {
		s.setStep(1, "done", "当前已是最新版本")
		s.logMsg("✅ 当前版本 %s 已是最新", s.currentVersion)
		s.setPhase("done")
		return
	}
	s.mu.Lock()
	s.state.FromVer = s.currentVersion
	s.state.ToVer = info.LatestVersion
	s.state.Source = "local"
	s.mu.Unlock()
	s.setStep(1, "done", fmt.Sprintf("发现新版本 %s → %s (本机更新镜像)", s.currentVersion, info.LatestVersion))
	s.logMsg("📦 %s → %s (本机更新镜像)", s.currentVersion, info.LatestVersion)
	s.setProgress(15)

	platformKey := getPlatformKey()
	localAssetPath, err := s.ensureLocalAsset(info, platformKey)
	if err != nil {
		s.setStepError(1, "准备本机更新包失败: "+err.Error())
		s.setError("准备本机更新包失败: " + err.Error())
		return
	}

	// Step 3: Stop service
	s.setStep(2, "running", "正在停止 ClawPanel 服务...")
	s.logMsg("⏹️ 停止 ClawPanel 服务...")
	if err := s.stopPanel(); err != nil {
		s.logMsg("⚠️ 停止服务出错: %v (继续更新)", err)
	}
	s.setStep(2, "done", "ClawPanel 服务已停止")
	s.setProgress(25)

	// Step 4: Stage the cached update package from local mirror.
	s.setStep(3, "running", "正在装载本机更新包...")
	tmpDir := filepath.Join(s.dataDir, "update-tmp")
	os.MkdirAll(tmpDir, 0755)
	tmpFile := filepath.Join(tmpDir, "clawpanel-new")
	if s.editionCfg.isLiteFullPackage() {
		tmpFile += ".tar.gz"
	} else if runtime.GOOS == "windows" {
		tmpFile += ".exe"
	}

	s.logMsg("📥 读取本机缓存更新包: %s", localAssetPath)
	if err := copyFile(localAssetPath, tmpFile); err != nil {
		s.setStepError(3, "复制本机更新包失败: "+err.Error())
		s.setError("复制本机更新包失败: " + err.Error())
		return
	}
	s.setStep(3, "done", "本机更新包已就绪")
	s.logMsg("✅ 本机更新包已就绪")
	s.setProgress(60)

	// Verify SHA256
	expectedSHA := ""
	if info.SHA256 != nil {
		expectedSHA = info.SHA256[platformKey]
	}
	if expectedSHA != "" {
		s.logMsg("🔒 校验文件 SHA256...")
		actualSHA, err := fileSHA256(tmpFile)
		if err != nil {
			s.setStepError(3, "校验失败: "+err.Error())
			s.setError("SHA256 校验失败: " + err.Error())
			return
		}
		if !strings.EqualFold(actualSHA, expectedSHA) {
			s.setStepError(3, "SHA256 不匹配，文件可能损坏")
			s.setError(fmt.Sprintf("SHA256 校验失败: 期望 %s..., 实际 %s...", expectedSHA[:16], actualSHA[:16]))
			os.Remove(tmpFile)
			return
		}
		s.logMsg("✅ SHA256 校验通过")
	} else {
		s.setStepError(3, "当前平台缺少 SHA256 校验值")
		s.setError("当前平台缺少 SHA256 校验值，已拒绝更新")
		return
	}

	// Continue with file replacement
	s.doReplace(tmpFile)
}

func (s *Server) resolveLocalVersion(force bool) (*VersionInfo, error) {
	manifest, err := updatemirror.ResolveLatest(s.dataDir, s.mirrorSpec(), "/api/panel/update-mirror", force)
	if err != nil {
		return nil, err
	}
	return &VersionInfo{
		LatestVersion: manifest.LatestVersion,
		ReleaseTime:   manifest.ReleaseTime,
		ReleaseNote:   manifest.ReleaseNote,
		DownloadURLs:  manifest.DownloadURLs,
		SHA256:        manifest.SHA256,
	}, nil
}

func (s *Server) ensureLocalAsset(info *VersionInfo, platformKey string) (string, error) {
	manifest, err := updatemirror.ResolveLatest(s.dataDir, s.mirrorSpec(), "/api/panel/update-mirror", false)
	if err != nil {
		return "", err
	}
	if info != nil && strings.TrimSpace(info.LatestVersion) != "" && manifest.LatestVersion != info.LatestVersion {
		manifest, err = updatemirror.ResolveLatest(s.dataDir, s.mirrorSpec(), "/api/panel/update-mirror", true)
		if err != nil {
			return "", err
		}
	}
	return updatemirror.EnsureAsset(s.dataDir, s.mirrorSpec(), manifest, platformKey)
}

func (s *Server) mirrorSpec() updatemirror.EditionSpec {
	return updatemirror.EditionSpec{
		Edition:           s.editionCfg.Edition,
		GitHubReleasesAPI: s.editionCfg.GitHubReleasesAPI,
		GitHubTagPrefix:   s.editionCfg.GitHubTagPrefix,
	}
}

func (s *Server) doUpdateWithFile(tmpFile string) {
	// Step 1: Validate
	s.setStep(0, "running", "验证授权中...")
	s.logMsg("🔐 验证更新授权...")
	s.setStep(0, "done", "授权验证通过")
	s.setProgress(5)

	// Step 2: Check uploaded file
	s.setStep(1, "running", "检测上传文件...")
	fi, err := os.Stat(tmpFile)
	if err != nil || fi.Size() < 1024 {
		s.setStepError(1, "上传文件无效")
		s.setError("上传文件无效或过小")
		return
	}
	s.mu.Lock()
	s.state.FromVer = s.currentVersion
	s.state.ToVer = "离线上传"
	s.state.Source = "upload"
	s.mu.Unlock()
	s.setStep(1, "done", fmt.Sprintf("上传文件有效 (%.1f MB)", float64(fi.Size())/1048576))
	s.logMsg("📦 上传文件: %.1f MB", float64(fi.Size())/1048576)
	s.setProgress(15)

	// Step 3: Stop service
	s.setStep(2, "running", "正在停止 ClawPanel 服务...")
	s.logMsg("⏹️ 停止 ClawPanel 服务...")
	if err := s.stopPanel(); err != nil {
		s.logMsg("⚠️ 停止服务出错: %v (继续更新)", err)
	}
	s.setStep(2, "done", "ClawPanel 服务已停止")
	s.setProgress(25)

	// Step 4: Skip download (already uploaded)
	s.setProgress(60)

	// Continue with file replacement
	s.doReplace(tmpFile)
}

func (s *Server) doReplace(tmpFile string) {
	if s.editionCfg.isLiteFullPackage() {
		s.doReplaceLitePackage(tmpFile)
		return
	}

	// Step 5: Backup
	s.setStep(4, "running", "正在备份当前程序...")
	s.logMsg("💾 备份当前程序...")
	backupPath := s.panelBin + ".bak"
	if err := copyFile(s.panelBin, backupPath); err != nil {
		s.logMsg("⚠️ 备份失败: %v (继续更新)", err)
		s.setStep(4, "done", "备份跳过 ("+err.Error()+")")
	} else {
		s.setStep(4, "done", "已备份至 "+filepath.Base(backupPath))
		s.logMsg("✅ 已备份至 %s", backupPath)
	}
	s.setProgress(70)

	// Step 6: Replace
	s.setStep(5, "running", "正在替换程序文件...")
	s.logMsg("🔄 替换程序文件...")

	if runtime.GOOS == "windows" {
		// Windows: rename old, copy new
		os.Remove(s.panelBin + ".old")
		os.Rename(s.panelBin, s.panelBin+".old")
		if err := copyFile(tmpFile, s.panelBin); err != nil {
			// Rollback
			s.logMsg("❌ 替换失败，回滚...")
			os.Rename(s.panelBin+".old", s.panelBin)
			s.setStepError(5, "替换失败，已回滚: "+err.Error())
			s.setError("替换失败，已回滚: " + err.Error())
			s.startPanel()
			return
		}
		os.Remove(s.panelBin + ".old")
	} else {
		// Linux/macOS: remove + copy
		if err := os.Remove(s.panelBin); err != nil {
			s.logMsg("⚠️ 删除旧文件失败: %v, 尝试覆盖写入...", err)
		}
		if err := copyFile(tmpFile, s.panelBin); err != nil {
			// Rollback
			s.logMsg("❌ 替换失败，回滚...")
			if _, berr := os.Stat(backupPath); berr == nil {
				copyFile(backupPath, s.panelBin)
			}
			os.Chmod(s.panelBin, 0755)
			s.setStepError(5, "替换失败，已回滚: "+err.Error())
			s.setError("替换失败，已回滚: " + err.Error())
			s.startPanel()
			return
		}
	}
	os.Chmod(s.panelBin, 0755)
	s.setStep(5, "done", "程序替换完成")
	s.logMsg("✅ 程序替换完成")
	s.setProgress(85)

	// Clean up temp
	os.Remove(tmpFile)
	os.RemoveAll(filepath.Join(s.dataDir, "update-tmp"))

	// Step 7: Start service
	s.setStep(6, "running", "正在启动 ClawPanel 服务...")
	s.logMsg("🚀 启动 ClawPanel 服务...")
	if err := s.startPanel(); err != nil {
		// Try rollback
		s.logMsg("❌ 启动失败: %v, 尝试回滚...", err)
		if _, berr := os.Stat(backupPath); berr == nil {
			os.Remove(s.panelBin)
			copyFile(backupPath, s.panelBin)
			os.Chmod(s.panelBin, 0755)
			s.logMsg("🔄 已回滚至备份文件，尝试重新启动...")
			if err2 := s.startPanel(); err2 != nil {
				s.setStepError(6, "启动失败且回滚后仍无法启动: "+err2.Error())
				s.setError("启动失败且回滚后仍无法启动，请手动处理")
				s.setPhase("rolled_back")
				return
			}
		} else {
			s.setStepError(6, "启动失败: "+err.Error())
			s.setError("启动失败: " + err.Error())
			return
		}
		s.setStep(6, "done", "已回滚并启动旧版本")
		s.logMsg("⚠️ 已回滚并启动旧版本")
		s.setPhase("rolled_back")
		return
	}

	// Verify service is actually running
	time.Sleep(3 * time.Second)
	if !s.isPanelRunning() {
		s.logMsg("⚠️ 服务似乎未成功启动，等待更长时间...")
		time.Sleep(5 * time.Second)
		if !s.isPanelRunning() {
			s.logMsg("❌ 服务启动失败，尝试回滚...")
			if _, berr := os.Stat(backupPath); berr == nil {
				exec.Command("systemctl", "stop", s.editionCfg.ServiceName).Run()
				time.Sleep(1 * time.Second)
				os.Remove(s.panelBin)
				copyFile(backupPath, s.panelBin)
				os.Chmod(s.panelBin, 0755)
				s.startPanel()
			}
			s.setStepError(6, "新版本启动失败，已回滚")
			s.setPhase("rolled_back")
			return
		}
	}

	s.setStep(6, "done", "ClawPanel 服务已启动")
	s.logMsg("✅ ClawPanel 服务已启动")
	s.setProgress(100)

	s.mu.Lock()
	s.state.Phase = "done"
	s.state.Message = "更新完成！"
	s.state.FinishedAt = time.Now().Format(time.RFC3339)
	s.mu.Unlock()
	s.logMsg("🎉 更新完成！")

	// Record update log
	s.recordUpdateLog()
}

func (s *Server) doReplaceLitePackage(tmpFile string) {
	installDir := filepath.Dir(s.panelBin)
	tmpDir := filepath.Dir(tmpFile)
	extractDir := filepath.Join(tmpDir, "extract")
	backupDir := filepath.Join(tmpDir, "backup")

	s.setStep(4, "running", "正在校验并备份 Lite 运行环境...")
	s.logMsg("📦 校验 Lite 整包结构...")
	if err := os.RemoveAll(extractDir); err != nil {
		s.setStepError(4, "清理临时目录失败: "+err.Error())
		s.setError("清理临时目录失败: " + err.Error())
		return
	}
	if err := extractTarGz(tmpFile, extractDir); err != nil {
		s.setStepError(4, "解压 Lite 整包失败: "+err.Error())
		s.setError("解压 Lite 整包失败: " + err.Error())
		return
	}
	if err := validateLitePackage(extractDir); err != nil {
		s.setStepError(4, "Lite 整包校验失败: "+err.Error())
		s.setError("Lite 整包校验失败: " + err.Error())
		return
	}
	if err := os.RemoveAll(backupDir); err != nil {
		s.setStepError(4, "清理备份目录失败: "+err.Error())
		s.setError("清理备份目录失败: " + err.Error())
		return
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		s.setStepError(4, "创建备份目录失败: "+err.Error())
		s.setError("创建备份目录失败: " + err.Error())
		return
	}
	for _, name := range []string{s.editionCfg.BinaryName, "bin", "runtime"} {
		src := filepath.Join(installDir, name)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, filepath.Join(backupDir, name)); err != nil {
				s.setStepError(4, "备份当前安装失败: "+err.Error())
				s.setError("备份当前安装失败: " + err.Error())
				return
			}
		}
	}
	s.setStep(4, "done", "Lite 运行环境已完成备份")
	s.setProgress(70)

	s.setStep(5, "running", "正在替换 Lite 面板与内置运行环境...")
	s.logMsg("🔄 替换 Lite 面板与 runtime...")
	applyErr := applyLitePackage(extractDir, installDir, s.editionCfg.BinaryName)
	if applyErr != nil {
		s.logMsg("❌ 替换失败，尝试回滚: %v", applyErr)
		_ = rollbackLitePackage(backupDir, installDir, s.editionCfg.BinaryName)
		s.setStepError(5, "替换失败，已回滚: "+applyErr.Error())
		s.setError("替换失败，已回滚: " + applyErr.Error())
		_ = s.startPanel()
		return
	}
	if err := os.Chmod(s.panelBin, 0755); err == nil {
		if runtime.GOOS != "windows" {
			_ = os.Chmod(filepath.Join(installDir, "bin", s.editionCfg.launcherName()), 0755)
		}
	}
	s.setStep(5, "done", "Lite 面板与运行环境替换完成")
	s.logMsg("✅ Lite 面板与运行环境替换完成")
	s.setProgress(85)

	os.Remove(tmpFile)

	s.setStep(6, "running", "正在启动 ClawPanel Lite...")
	s.logMsg("🚀 启动 ClawPanel Lite...")
	if err := s.startPanel(); err != nil {
		s.logMsg("❌ 启动失败: %v, 尝试回滚...", err)
		_ = rollbackLitePackage(backupDir, installDir, s.editionCfg.BinaryName)
		_ = s.startPanel()
		s.setStepError(6, "启动失败，已回滚: "+err.Error())
		s.setError("启动失败，已回滚: " + err.Error())
		s.setPhase("rolled_back")
		return
	}

	time.Sleep(3 * time.Second)
	if !s.isPanelRunning() || !s.isLiteRuntimeReady() {
		s.logMsg("❌ Lite 更新后健康检查失败，尝试回滚...")
		_ = exec.Command("systemctl", "stop", s.editionCfg.ServiceName).Run()
		_ = rollbackLitePackage(backupDir, installDir, s.editionCfg.BinaryName)
		_ = s.startPanel()
		s.setStepError(6, "健康检查失败，已回滚")
		s.setPhase("rolled_back")
		return
	}

	_ = os.RemoveAll(backupDir)
	_ = os.RemoveAll(tmpDir)
	s.setStep(6, "done", "ClawPanel Lite 已启动")
	s.logMsg("✅ ClawPanel Lite 已启动")
	s.setProgress(100)

	s.mu.Lock()
	s.state.Phase = "done"
	s.state.Message = "更新完成！"
	s.state.FinishedAt = time.Now().Format(time.RFC3339)
	s.mu.Unlock()
	s.logMsg("🎉 Lite 整包更新完成！")
	s.recordUpdateLog()
}

func (s *Server) stopPanel() error {
	if runtime.GOOS == "windows" {
		// Try service stop first
		exec.Command("net", "stop", s.editionCfg.ServiceName).Run()
		time.Sleep(2 * time.Second)

		// Kill clawpanel.exe processes by PID, excluding ourselves and our parent.
		// taskkill /IM kills ALL clawpanel.exe including the updater itself — use PID-based kill instead.
		selfPID := os.Getpid()
		parentPID := os.Getppid()

		// Use WMIC to list all clawpanel.exe PIDs
		processName := s.editionCfg.BinaryName + ".exe"
		out, err := exec.Command("wmic", "process", "where", "name='"+processName+"'", "get", "processid", "/value").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(strings.ToUpper(line), "PROCESSID=") {
					continue
				}
				pidStr := strings.TrimPrefix(strings.TrimPrefix(line, "ProcessId="), "PROCESSID=")
				pidStr = strings.TrimSpace(pidStr)
				pid, err := strconv.Atoi(pidStr)
				if err != nil || pid == 0 {
					continue
				}
				// Skip the updater process and its parent
				if pid == selfPID || pid == parentPID {
					continue
				}
				exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
			}
		}
		time.Sleep(1 * time.Second)
	} else {
		if runtime.GOOS == "darwin" {
			_ = exec.Command("launchctl", "stop", s.editionCfg.ServiceLabel).Run()
			_ = exec.Command("launchctl", "bootout", "system", "/Library/LaunchDaemons/"+s.editionCfg.ServiceLabel+".plist").Run()
			time.Sleep(2 * time.Second)
		} else {
			if commandExists("systemctl") {
				exec.Command("systemctl", "stop", s.editionCfg.ServiceName).Run()
				time.Sleep(3 * time.Second)
				// If systemd service is still active, wait a bit more
				for i := 0; i < 5; i++ {
					out, _ := exec.Command("systemctl", "is-active", s.editionCfg.ServiceName).Output()
					if strings.TrimSpace(string(out)) != "active" {
						break
					}
					time.Sleep(1 * time.Second)
				}
			} else {
				// Non-systemd Linux fallback: kill panel process but keep updater child alive
				_ = killPanelProcessesExceptUpdater(os.Getpid(), os.Getppid())
				time.Sleep(1 * time.Second)
			}
		}
	}
	return nil
}

func (s *Server) startPanel() error {
	if runtime.GOOS == "windows" {
		err := exec.Command("net", "start", s.editionCfg.ServiceName).Run()
		if err != nil {
			// Try direct start
			cmd := exec.Command(s.panelBin)
			cmd.Dir = filepath.Dir(s.panelBin)
			return cmd.Start()
		}
		return nil
	}
	err := exec.Command("systemctl", "start", s.editionCfg.ServiceName).Run()
	if runtime.GOOS == "darwin" {
		if err := exec.Command("launchctl", "kickstart", "-k", "system/"+s.editionCfg.ServiceLabel).Run(); err == nil {
			return nil
		}
		_ = exec.Command("launchctl", "load", "-w", "/Library/LaunchDaemons/"+s.editionCfg.ServiceLabel+".plist").Run()
		if err := exec.Command("launchctl", "kickstart", "-k", "system/"+s.editionCfg.ServiceLabel).Run(); err == nil {
			return nil
		}
		cmd := exec.Command("bash", "-c", fmt.Sprintf("nohup %s >/dev/null 2>&1 &", s.panelBin))
		return cmd.Run()
	}
	if err != nil {
		// Try direct start
		cmd := exec.Command("bash", "-c", fmt.Sprintf("nohup %s >/dev/null 2>&1 &", s.panelBin))
		return cmd.Run()
	}
	return nil
}

func (s *Server) isPanelRunning() bool {
	if runtime.GOOS == "windows" {
		imageName := s.editionCfg.BinaryName + ".exe"
		out, _ := exec.Command("tasklist", "/FI", "IMAGENAME eq "+imageName, "/NH").Output()
		return strings.Contains(strings.ToLower(string(out)), strings.ToLower(s.editionCfg.BinaryName))
	}
	if runtime.GOOS == "darwin" {
		return isPortOpen(s.panelPort)
	}
	if commandExists("systemctl") {
		// Use systemctl is-active instead of pgrep (pgrep would match updater child too)
		out, _ := exec.Command("systemctl", "is-active", s.editionCfg.ServiceName).Output()
		return strings.TrimSpace(string(out)) == "active"
	}
	return isPortOpen(s.panelPort)
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func killPanelProcessesExceptUpdater(selfPID, parentPID int) error {
	out, err := exec.Command("pgrep", "-f", "clawpanel").Output()
	if err != nil {
		return nil
	}
	for _, pidStr := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err != nil || pid <= 0 {
			continue
		}
		if pid == selfPID || pid == parentPID {
			continue
		}
		cmdlineRaw, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		cmdline := strings.ReplaceAll(string(cmdlineRaw), "\x00", " ")
		if strings.Contains(cmdline, "--updater-standalone") {
			continue
		}
		_ = exec.Command("kill", "-TERM", strconv.Itoa(pid)).Run()
	}
	return nil
}

func (s *Server) fetchLatestVersion(preferredSource string) (*VersionInfo, string, error) {
	var errs []string
	for _, source := range downloadSourceOrder(preferredSource) {
		info, err := s.fetchVersionFromSource(source)
		if err == nil {
			return info, source, nil
		}
		errMsg := fmt.Sprintf("%s: %v", source, err)
		errs = append(errs, errMsg)
		log.Printf("[Updater] %s 线路请求失败: %v", source, err)
	}
	return nil, "", fmt.Errorf("所有线路均失败: %s", strings.Join(errs, "; "))
}

func (s *Server) fetchVersionFromSource(source string) (*VersionInfo, error) {
	switch normalizeDownloadSource(source) {
	case "accel":
		return s.fetchFromAccel()
	default:
		return s.fetchFromGitHub()
	}
}

func (s *Server) fetchFromAccel() (*VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(s.editionCfg.AccelUpdateJSON)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *Server) fetchFromGitHub() (*VersionInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(s.editionCfg.GitHubReleasesAPI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var releases []struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		PubAt   string `json:"published_at"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	for _, release := range releases {
		if !s.editionCfg.matchesTag(release.TagName) {
			continue
		}
		info := &VersionInfo{
			LatestVersion: s.editionCfg.trimTag(release.TagName),
			ReleaseTime:   release.PubAt,
			ReleaseNote:   release.Body,
			DownloadURLs:  map[string]string{},
			SHA256:        map[string]string{},
		}
		for _, a := range release.Assets {
			if platformKey, ok := s.editionCfg.matchUpdateAsset(info.LatestVersion, a.Name); ok {
				info.DownloadURLs[platformKey] = a.URL
			}
		}
		return info, nil
	}
	return nil, fmt.Errorf("未找到 %s 版本发布", s.editionCfg.Edition)
}

func (s *Server) downloadFile(url, dest, source string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	totalSize := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 64*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			if totalSize > 0 {
				pct := int(float64(downloaded)/float64(totalSize)*35) + 25 // 25-60%
				s.setProgress(pct)
				s.setStep(3, "running", fmt.Sprintf("下载中 %.1f MB / %.1f MB (%d%%)",
					float64(downloaded)/1048576, float64(totalSize)/1048576,
					int(float64(downloaded)/float64(totalSize)*100)))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) recordUpdateLog() {
	s.mu.Lock()
	state := s.state
	s.mu.Unlock()

	logEntry := map[string]interface{}{
		"time":        time.Now().Format(time.RFC3339),
		"from":        state.FromVer,
		"to":          state.ToVer,
		"source":      state.Source,
		"result":      state.Phase,
		"started_at":  state.StartedAt,
		"finished_at": state.FinishedAt,
	}

	logFile := filepath.Join(s.dataDir, "update_history.json")
	var history []map[string]interface{}
	if data, err := os.ReadFile(logFile); err == nil {
		json.Unmarshal(data, &history)
	}
	history = append(history, logEntry)
	// Keep last 50
	if len(history) > 50 {
		history = history[len(history)-50:]
	}
	data, _ := json.MarshalIndent(history, "", "  ")
	os.WriteFile(logFile, data, 0644)
}

// --- State helpers ---

func (s *Server) setStep(idx int, status, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < len(s.state.Steps) {
		s.state.Steps[idx].Status = status
		s.state.Steps[idx].Message = message
	}
}

func (s *Server) setStepError(idx int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < len(s.state.Steps) {
		s.state.Steps[idx].Status = "error"
		s.state.Steps[idx].Message = message
	}
	s.state.Phase = "error"
	s.state.FinishedAt = time.Now().Format(time.RFC3339)
}

func (s *Server) setProgress(pct int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Progress = pct
}

func (s *Server) setPhase(phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Phase = phase
	if phase == "done" || phase == "error" || phase == "rolled_back" {
		s.state.FinishedAt = time.Now().Format(time.RFC3339)
	}
}

func (s *Server) setError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Phase = "error"
	s.state.Error = msg
	s.state.Message = "更新失败"
	s.state.FinishedAt = time.Now().Format(time.RFC3339)
	s.state.Log = append(s.state.Log, "❌ "+msg)
}

func (s *Server) logMsg(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[Updater] %s", msg)
	s.mu.Lock()
	s.state.Log = append(s.state.Log, msg)
	s.mu.Unlock()
}

func (s *Server) setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")
}

func (s *Server) checkToken(w http.ResponseWriter, r *http.Request) bool {
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Update-Token")
	}
	if !ValidateToken(token, s.panelPort) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": "授权令牌无效或已过期",
		})
		return false
	}
	return true
}

// --- Utilities ---

func getPlatformKey() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	return goos + "_" + goarch
}

func isNewerVersion(latest, current string) bool {
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")
	lp := strings.Split(latest, ".")
	cp := strings.Split(current, ".")
	for i := 0; i < len(lp) && i < len(cp); i++ {
		lv := 0
		cv := 0
		fmt.Sscanf(lp[i], "%d", &lv)
		fmt.Sscanf(cp[i], "%d", &cv)
		if lv > cv {
			return true
		}
		if lv < cv {
			return false
		}
	}
	return len(lp) > len(cp)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func normalizeDownloadSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "github":
		return "github"
	case "accel":
		return "accel"
	default:
		return ""
	}
}

func downloadSourceOrder(preferred string) []string {
	switch normalizeDownloadSource(preferred) {
	case "accel":
		return []string{"accel", "github"}
	case "github":
		return []string{"github", "accel"}
	default:
		return []string{"github", "accel"}
	}
}

func extractTarGz(src, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) && filepath.Clean(target) != filepath.Clean(dest) {
			return fmt.Errorf("非法归档路径: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLitePackage(root string) error {
	required := []string{
		"clawpanel-lite",
		"runtime",
		filepath.Join("runtime", "openclaw"),
		filepath.Join("runtime", "node"),
	}
	launcherName := "clawlite-openclaw"
	nodeRel := filepath.Join("runtime", "node", "bin", "node")
	if runtime.GOOS == "windows" {
		required[0] = "clawpanel-lite.exe"
		launcherName = "clawlite-openclaw.cmd"
		nodeRel = filepath.Join("runtime", "node", "node.exe")
	}
	required = append(required, filepath.Join("bin", launcherName), nodeRel)
	for _, rel := range required {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			return fmt.Errorf("缺少 %s", rel)
		}
	}
	return nil
}

func applyLitePackage(extractDir, installDir, binaryName string) error {
	for _, name := range []string{binaryName, "bin", "runtime"} {
		src := filepath.Join(extractDir, name)
		if _, err := os.Stat(src); err != nil {
			return err
		}
		dst := filepath.Join(installDir, name)
		_ = os.RemoveAll(dst)
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func rollbackLitePackage(backupDir, installDir, binaryName string) error {
	for _, name := range []string{binaryName, "bin", "runtime"} {
		dst := filepath.Join(installDir, name)
		_ = os.RemoveAll(dst)
		src := filepath.Join(backupDir, name)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) isLiteRuntimeReady() bool {
	if !s.editionCfg.isLiteFullPackage() {
		return true
	}
	cmd := exec.Command(filepath.Join(filepath.Dir(s.panelBin), "bin", s.editionCfg.launcherName()), "--version")
	cmd.Env = os.Environ()
	cmd.Dir = filepath.Dir(s.panelBin)
	return cmd.Run() == nil
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
