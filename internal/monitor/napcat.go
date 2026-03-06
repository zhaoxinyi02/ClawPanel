package monitor

import (
	"crypto/sha256"
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
	"strings"
	"sync"
	"time"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/eventlog"
	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

// NapCatStatus represents the current NapCat connection state
type NapCatStatus struct {
	ContainerRunning bool      `json:"containerRunning"`
	WSConnected      bool      `json:"wsConnected"`
	HTTPAvailable    bool      `json:"httpAvailable"`
	QQLoggedIn       bool      `json:"qqLoggedIn"`
	QQNickname       string    `json:"qqNickname,omitempty"`
	QQID             string    `json:"qqId,omitempty"`
	LastCheck        time.Time `json:"lastCheck"`
	LastOnline       time.Time `json:"lastOnline,omitempty"`
	ReconnectCount   int       `json:"reconnectCount"`
	MaxReconnect     int       `json:"maxReconnect"`
	AutoReconnect    bool      `json:"autoReconnect"`
	Status           string    `json:"status"` // online, offline, reconnecting, login_expired, stopped
}

// ReconnectLog records a reconnection attempt
type ReconnectLog struct {
	Time    time.Time `json:"time"`
	Reason  string    `json:"reason"`
	Success bool      `json:"success"`
	Detail  string    `json:"detail,omitempty"`
}

// NapCatMonitor monitors NapCat connection and handles auto-reconnect
type NapCatMonitor struct {
	cfg            *config.Config
	hub            *websocket.Hub
	sysLog         *eventlog.SystemLogger
	mu             sync.RWMutex
	status         NapCatStatus
	logs           []ReconnectLog
	maxLogs        int
	stopCh         chan struct{}
	running        bool
	paused         bool // true when QQ channel is disabled — skip checks
	checkInterval  time.Duration
	reconnecting   bool // true while a reconnect is in progress
	offlineCount   int  // consecutive offline checks before triggering reconnect
	loginFailCount int  // consecutive login check failures before declaring login_expired
}

// NewNapCatMonitor creates a new NapCat monitor
func NewNapCatMonitor(cfg *config.Config, hub *websocket.Hub, sysLog *eventlog.SystemLogger) *NapCatMonitor {
	return &NapCatMonitor{
		cfg:           cfg,
		hub:           hub,
		sysLog:        sysLog,
		maxLogs:       100,
		checkInterval: 30 * time.Second,
		status: NapCatStatus{
			MaxReconnect:  10,
			AutoReconnect: true,
			Status:        "offline",
		},
		stopCh: make(chan struct{}),
	}
}

// Start begins the monitoring loop
func (m *NapCatMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.monitorLoop()
	log.Println("[NapCatMonitor] 监控已启动，检测间隔:", m.checkInterval)
}

// Stop stops the monitoring loop
func (m *NapCatMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
}

// GetStatus returns current NapCat status
func (m *NapCatMonitor) GetStatus() NapCatStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// GetLogs returns reconnection logs
func (m *NapCatMonitor) GetLogs() []ReconnectLog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ReconnectLog, len(m.logs))
	copy(result, m.logs)
	return result
}

// Reconnect manually triggers a reconnection
func (m *NapCatMonitor) Reconnect() error {
	m.mu.Lock()
	m.status.Status = "reconnecting"
	m.mu.Unlock()

	m.broadcastStatus()
	return m.doReconnect("手动触发重连")
}

// SetAutoReconnect enables/disables auto-reconnect
func (m *NapCatMonitor) SetAutoReconnect(enabled bool) {
	m.mu.Lock()
	m.status.AutoReconnect = enabled
	m.mu.Unlock()
}

// SetMaxReconnect sets the maximum reconnection attempts
func (m *NapCatMonitor) SetMaxReconnect(max int) {
	m.mu.Lock()
	m.status.MaxReconnect = max
	m.mu.Unlock()
}

// Pause suspends monitoring checks (call when QQ channel is disabled)
func (m *NapCatMonitor) Pause() {
	m.mu.Lock()
	m.paused = true
	m.mu.Unlock()
	log.Println("[NapCatMonitor] 监控已暂停（QQ 通道已关闭）")
}

// Resume resumes monitoring checks (call when QQ channel is re-enabled)
func (m *NapCatMonitor) Resume() {
	m.mu.Lock()
	m.paused = false
	m.offlineCount = 0
	m.loginFailCount = 0
	m.status.ReconnectCount = 0
	m.mu.Unlock()
	log.Println("[NapCatMonitor] 监控已恢复")
}

// IsPaused returns whether the monitor is currently paused
func (m *NapCatMonitor) IsPaused() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.paused
}

func (m *NapCatMonitor) monitorLoop() {
	// Initial check after a short delay
	select {
	case <-m.stopCh:
		return
	case <-time.After(5 * time.Second):
	}

	m.checkAndUpdate()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAndUpdate()
		}
	}
}

func (m *NapCatMonitor) checkAndUpdate() {
	// Skip checks while paused (QQ channel disabled) or reconnecting
	m.mu.RLock()
	if m.paused || m.reconnecting {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	containerRunning := isNapCatProcessRunning()
	wsConnected := isPortReachable(3001)
	httpAvailable := isPortReachable(3000)
	qqLoggedIn := false
	qqNickname := ""
	qqID := ""

	// Only check login status if WebUI port is reachable
	if isPortReachable(6099) {
		qqLoggedIn, qqNickname, qqID = checkQQLoginStatus(m.cfg)
	}

	m.mu.Lock()
	prevStatus := m.status.Status

	m.status.ContainerRunning = containerRunning
	m.status.WSConnected = wsConnected
	m.status.HTTPAvailable = httpAvailable
	m.status.QQLoggedIn = qqLoggedIn
	m.status.QQNickname = qqNickname
	m.status.QQID = qqID
	m.status.LastCheck = time.Now()

	// Determine overall status
	if !containerRunning {
		m.status.Status = "stopped"
		m.loginFailCount = 0
	} else if qqLoggedIn {
		m.status.Status = "online"
		m.status.LastOnline = time.Now()
		m.status.ReconnectCount = 0
		m.offlineCount = 0
		m.loginFailCount = 0
	} else if wsConnected || httpAvailable {
		// Container running, services up, but QQ not logged in.
		// Require multiple consecutive failures before declaring login_expired
		// to avoid false positives from transient HTTP timeouts.
		m.loginFailCount++
		if prevStatus == "online" && m.loginFailCount < 3 {
			// Keep online status until confirmed offline (3 checks = ~90s)
			m.status.Status = "online"
		} else if prevStatus == "online" || prevStatus == "login_expired" {
			m.status.Status = "login_expired"
		} else {
			m.status.Status = "offline"
		}
		m.offlineCount = 0
	} else if containerRunning {
		// Container running but services not responding yet (booting up)
		m.offlineCount++
		if m.offlineCount <= 3 {
			// Give it a few checks to boot up before declaring offline
			m.status.Status = prevStatus // keep previous status
		} else {
			m.status.Status = "offline"
		}
	} else {
		m.offlineCount++
		m.status.Status = "stopped"
	}

	currentStatus := m.status.Status
	autoReconnect := m.status.AutoReconnect
	reconnectCount := m.status.ReconnectCount
	maxReconnect := m.status.MaxReconnect
	offlineCount := m.offlineCount
	m.mu.Unlock()

	// Broadcast status change
	if currentStatus != prevStatus {
		m.broadcastStatus()

		// Log status changes (only log meaningful transitions)
		switch currentStatus {
		case "online":
			m.sysLog.Log("system", "napcat.online", fmt.Sprintf("NapCat QQ 已上线 (%s: %s)", qqID, qqNickname))
		case "login_expired":
			if prevStatus == "online" {
				m.sysLog.Log("system", "napcat.login_expired", "NapCat QQ 登录已失效，需要重新扫码")
			}
		case "stopped":
			if prevStatus == "online" || prevStatus == "login_expired" {
				m.sysLog.Log("system", "napcat.stopped", "NapCat 容器已停止")
			}
		case "offline":
			if prevStatus == "online" {
				m.sysLog.Log("system", "napcat.offline", "NapCat QQ 已离线")
			}
		}
	}

	// Auto-reconnect: only trigger for "offline" (services down), NOT for "login_expired"
	// login_expired means container is fine but QQ session expired — restarting container won't help,
	// the user needs to re-scan QR code.
	if autoReconnect && currentStatus == "offline" && offlineCount >= 3 {
		if reconnectCount < maxReconnect {
			m.mu.Lock()
			m.status.Status = "reconnecting"
			m.reconnecting = true
			m.mu.Unlock()
			m.broadcastStatus()

			go func() {
				m.doReconnect("检测到连接断开，自动重连")
				m.mu.Lock()
				m.reconnecting = false
				m.offlineCount = 0
				m.mu.Unlock()
			}()
		} else if reconnectCount == maxReconnect {
			m.sysLog.Log("system", "napcat.reconnect_limit", fmt.Sprintf("NapCat 自动重连已达上限 (%d 次)，停止重连", maxReconnect))
			m.mu.Lock()
			m.status.ReconnectCount = maxReconnect + 1 // prevent repeated log
			m.mu.Unlock()
		}
	}
}

func (m *NapCatMonitor) doReconnect(reason string) error {
	m.mu.Lock()
	m.status.ReconnectCount++
	count := m.status.ReconnectCount
	m.mu.Unlock()

	m.sysLog.Log("system", "napcat.reconnecting", fmt.Sprintf("NapCat 正在重连 (第 %d 次): %s", count, reason))

	rlog := ReconnectLog{
		Time:   time.Now(),
		Reason: reason,
	}

	// Clear cached credential since container restart invalidates the old session
	cachedMonitorCred = ""

	// Restart NapCat (Docker on Linux/macOS, Shell process on Windows)
	out, err := restartNapCatPlatform(m.cfg)

	if err != nil {
		rlog.Success = false
		rlog.Detail = fmt.Sprintf("docker restart 失败: %s %s", err.Error(), string(out))
		m.addLog(rlog)

		m.mu.Lock()
		m.status.Status = "offline"
		m.mu.Unlock()
		m.broadcastStatus()

		m.sysLog.Log("system", "napcat.reconnect_failed", fmt.Sprintf("NapCat 重连失败: %v", err))
		return err
	}

	// Wait for container to come back up
	time.Sleep(10 * time.Second)

	// Check if services are back
	wsOK := false
	for i := 0; i < 6; i++ {
		if isPortReachable(3001) {
			wsOK = true
			break
		}
		time.Sleep(5 * time.Second)
	}

	if wsOK {
		rlog.Success = true
		rlog.Detail = "容器重启成功，WebSocket 服务已恢复"
		m.addLog(rlog)

		// Check login status
		loggedIn, nickname, qqid := checkQQLoginStatus(m.cfg)
		m.mu.Lock()
		m.status.WSConnected = true
		m.status.HTTPAvailable = isPortReachable(3000)
		m.status.QQLoggedIn = loggedIn
		m.status.QQNickname = nickname
		m.status.QQID = qqid
		if loggedIn {
			m.status.Status = "online"
			m.status.LastOnline = time.Now()
			m.status.ReconnectCount = 0
		} else {
			m.status.Status = "login_expired"
		}
		m.mu.Unlock()
		m.broadcastStatus()

		if loggedIn {
			m.sysLog.Log("system", "napcat.reconnected", fmt.Sprintf("NapCat 重连成功，QQ 已上线 (%s: %s)", qqid, nickname))
		} else {
			m.sysLog.Log("system", "napcat.reconnected_no_login", "NapCat 容器已重启，但 QQ 需要重新登录")
		}
		return nil
	}

	rlog.Success = false
	rlog.Detail = "容器重启后 WebSocket 服务未恢复"
	m.addLog(rlog)

	m.mu.Lock()
	m.status.Status = "offline"
	m.mu.Unlock()
	m.broadcastStatus()

	m.sysLog.Log("system", "napcat.reconnect_failed", "NapCat 重连后服务未恢复")
	return fmt.Errorf("WebSocket 服务未恢复")
}

func (m *NapCatMonitor) addLog(rlog ReconnectLog) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, rlog)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

func (m *NapCatMonitor) broadcastStatus() {
	m.mu.RLock()
	status := m.status
	m.mu.RUnlock()

	msg, _ := json.Marshal(map[string]interface{}{
		"type": "napcat-status",
		"data": status,
	})
	m.hub.Broadcast(msg)
}

// --- Helpers ---

// Cached NapCat WebUI credential to avoid re-authenticating every check cycle.
// NapCat credentials are long-lived; re-auth only when this becomes stale.
var (
	cachedMonitorCred     string
	cachedMonitorCredTime time.Time
)

func isContainerRunning(name string) bool {
	out, err := dockerOutput("inspect", "--format", "{{.State.Running}}", name)
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// isNapCatProcessRunning checks if NapCat is running, platform-aware
func isNapCatProcessRunning() bool {
	if runtime.GOOS == "windows" {
		// Check for NapCat Shell process on Windows
		out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq NapCatWinBootMain.exe", "/NH").Output()
		if err == nil && strings.Contains(string(out), "NapCatWinBootMain") {
			return true
		}
		out2, err := exec.Command("tasklist", "/FI", "IMAGENAME eq napcat.exe", "/NH").Output()
		if err == nil && strings.Contains(string(out2), "napcat.exe") {
			return true
		}
		return false
	}
	return isContainerRunning("openclaw-qq")
}

// StopNapCatPlatform stops NapCat (Docker on Linux, process on Windows)
func StopNapCatPlatform() {
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/F", "/IM", "NapCatWinBootMain.exe").Run()
		exec.Command("taskkill", "/F", "/IM", "napcat.exe").Run()
		exec.Command("taskkill", "/F", "/IM", "QQ.exe").Run()
	} else {
		dockerRun("stop", "openclaw-qq")
	}
}

// restartNapCatPlatform restarts NapCat based on platform
func restartNapCatPlatform(cfg *config.Config) ([]byte, error) {
	if runtime.GOOS == "windows" {
		// Kill NapCat processes
		exec.Command("taskkill", "/F", "/IM", "NapCatWinBootMain.exe").Run()
		exec.Command("taskkill", "/F", "/IM", "napcat.exe").Run()
		exec.Command("taskkill", "/F", "/IM", "QQ.exe").Run()
		time.Sleep(2 * time.Second)
		// Find and restart NapCat Shell
		napcatDir := findNapCatShellDir(cfg)
		if napcatDir == "" {
			return []byte("NapCat Shell directory not found"), fmt.Errorf("NapCat Shell not installed")
		}
		batPath := filepath.Join(napcatDir, "napcat.bat")
		if _, err := os.Stat(batPath); err == nil {
			cmd := exec.Command("cmd", "/C", "start", "/B", batPath)
			cmd.Dir = napcatDir
			err := cmd.Start()
			if err != nil {
				return []byte(err.Error()), err
			}
			return []byte("NapCat Shell restarted"), nil
		}
		exePath := filepath.Join(napcatDir, "NapCatWinBootMain.exe")
		if _, err := os.Stat(exePath); err == nil {
			cmd := exec.Command(exePath)
			cmd.Dir = napcatDir
			err := cmd.Start()
			if err != nil {
				return []byte(err.Error()), err
			}
			return []byte("NapCat Shell restarted"), nil
		}
		return []byte("No napcat.bat or NapCatWinBootMain.exe found"), fmt.Errorf("NapCat executable not found")
	}
	// Linux/macOS: Docker
	return dockerOutput("restart", "openclaw-qq")
}

// findNapCatShellDir finds the NapCat Shell installation directory on Windows
func findNapCatShellDir(cfg *config.Config) string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(cfg.DataDir, "napcat"),
		filepath.Join(home, "NapCat"),
		filepath.Join(home, "Desktop", "NapCat"),
		`C:\NapCat`,
		filepath.Join(home, "AppData", "Local", "NapCat"),
	}
	markers := []string{"napcat.bat", "NapCatWinBootMain.exe"}
	for _, dir := range candidates {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		found := ""
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || found != "" {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				for _, m := range markers {
					if strings.EqualFold(d.Name(), m) {
						found = filepath.Dir(path)
						return filepath.SkipAll
					}
				}
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	return ""
}

func isPortReachable(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func checkQQLoginStatus(cfg *config.Config) (loggedIn bool, nickname string, qqID string) {
	// Retry once on failure to reduce false positives from transient HTTP timeouts
	for attempt := 0; attempt < 2; attempt++ {
		loggedIn, nickname, qqID = doCheckQQLoginStatus(cfg)
		if loggedIn {
			return
		}
		if attempt == 0 {
			time.Sleep(2 * time.Second)
		}
	}
	return
}

func doCheckQQLoginStatus(cfg *config.Config) (loggedIn bool, nickname string, qqID string) {
	client := &http.Client{Timeout: 5 * time.Second}

	// Use cached credential if available and fresh (< 10 minutes)
	cred := ""
	if cachedMonitorCred != "" && time.Since(cachedMonitorCredTime) < 10*time.Minute {
		cred = cachedMonitorCred
	} else {
		// Get WebUI token: from local config file on Windows, from Docker on Linux
		token := ""
		if runtime.GOOS == "windows" {
			napcatDir := findNapCatShellDir(cfg)
			if napcatDir != "" {
				webuiPath := filepath.Join(napcatDir, "config", "webui.json")
				if data, err := os.ReadFile(webuiPath); err == nil {
					var webui map[string]interface{}
					if json.Unmarshal(data, &webui) == nil {
						if t, ok := webui["token"].(string); ok && t != "" {
							token = t
						}
					}
				}
			}
		} else {
			out, err := dockerOutput("exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json")
			if err == nil {
				var webui map[string]interface{}
				if json.Unmarshal(out, &webui) == nil {
					if t, ok := webui["token"].(string); ok && t != "" {
						token = t
					}
				}
			}
		}
		if token == "" {
			return false, "", ""
		}

		hash := sha256.Sum256([]byte(token + ".napcat"))
		hashStr := fmt.Sprintf("%x", hash)
		loginBody := fmt.Sprintf(`{"hash":"%s"}`, hashStr)
		resp, err := client.Post("http://127.0.0.1:6099/api/auth/login", "application/json", strings.NewReader(loginBody))
		if err != nil {
			return false, "", ""
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var loginResp map[string]interface{}
		if json.Unmarshal(body, &loginResp) != nil {
			return false, "", ""
		}
		if code, ok := loginResp["code"].(float64); ok && code == 0 {
			if data, ok := loginResp["data"].(map[string]interface{}); ok {
				cred, _ = data["Credential"].(string)
			}
		}
		if cred == "" {
			return false, "", ""
		}
		cachedMonitorCred = cred
		cachedMonitorCredTime = time.Now()
	}

	// Check login status
	req, _ := http.NewRequest("POST", "http://127.0.0.1:6099/api/QQLogin/CheckLoginStatus", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cred)
	resp2, err := client.Do(req)
	if err != nil {
		// HTTP failure — clear cached cred so next check re-auths
		cachedMonitorCred = ""
		return false, "", ""
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	var statusResp map[string]interface{}
	if json.Unmarshal(body2, &statusResp) != nil {
		return false, "", ""
	}

	// If unauthorized, clear cached cred and return false (will retry with fresh cred)
	if code, ok := statusResp["code"].(float64); ok && code == -1 {
		cachedMonitorCred = ""
		return false, "", ""
	}

	if code, ok := statusResp["code"].(float64); !ok || code != 0 {
		return false, "", ""
	}
	statusData, _ := statusResp["data"].(map[string]interface{})
	if statusData == nil {
		return false, "", ""
	}
	isLogin, _ := statusData["isLogin"].(bool)
	if !isLogin {
		return false, "", ""
	}

	// Get login info
	req3, _ := http.NewRequest("POST", "http://127.0.0.1:6099/api/QQLogin/GetQQLoginInfo", nil)
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+cred)
	resp3, err := client.Do(req3)
	if err != nil {
		return true, "", ""
	}
	defer resp3.Body.Close()
	body3, _ := io.ReadAll(resp3.Body)
	var infoResp map[string]interface{}
	if json.Unmarshal(body3, &infoResp) != nil {
		return true, "", ""
	}
	if infoCode, ok := infoResp["code"].(float64); ok && infoCode == 0 {
		infoData, _ := infoResp["data"].(map[string]interface{})
		if infoData != nil {
			nickname, _ = infoData["nick"].(string)
			if uid, ok := infoData["uin"].(float64); ok {
				qqID = fmt.Sprintf("%.0f", uid)
			}
			if uid, ok := infoData["uin"].(string); ok {
				qqID = uid
			}
		}
	}

	return true, nickname, qqID
}

func dockerOutput(args ...string) ([]byte, error) {
	bins := []string{"docker", "/usr/local/bin/docker", "/opt/homebrew/bin/docker"}
	for _, bin := range bins {
		cmd := exec.Command(bin, args...)
		cmd.Env = dockerEnv()
		if out, err := cmd.CombinedOutput(); err == nil {
			return out, nil
		}
		if runtime.GOOS == "darwin" {
			for _, archFlag := range []string{"-arm64", "-x86_64"} {
				altArgs := append([]string{archFlag, bin}, args...)
				alt := exec.Command("arch", altArgs...)
				alt.Env = dockerEnv()
				if out, err := alt.CombinedOutput(); err == nil {
					return out, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("docker not available")
}

func dockerRun(args ...string) error {
	_, err := dockerOutput(args...)
	return err
}

func dockerEnv() []string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		if runtime.GOOS == "darwin" {
			home = "/var/root"
		} else {
			home = "/root"
		}
	}
	path := os.Getenv("PATH")
	extra := "/usr/local/bin:/usr/local/sbin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/opt/homebrew/sbin"
	if path == "" {
		path = extra
	} else {
		path = path + ":" + extra
	}
	return append(os.Environ(), "PATH="+path, "HOME="+home)
}
