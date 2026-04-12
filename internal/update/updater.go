package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/zhaoxinyi02/ClawPanel/internal/updatemirror"
)

const (
	httpTimeout     = 30 * time.Second
	downloadTimeout = 300 * time.Second
)

// UpdateInfo represents the server-side update.json
type UpdateInfo struct {
	LatestVersion string            `json:"latest_version"`
	ReleaseTime   string            `json:"release_time"`
	ReleaseNote   string            `json:"release_note"`
	DownloadURLs  map[string]string `json:"download_urls"`
	SHA256        map[string]string `json:"sha256"`
	LocalPaths    map[string]string `json:"-"`
}

// UpdatePopup represents the popup info saved after a successful update
type UpdatePopup struct {
	Show        bool   `json:"show"`
	Version     string `json:"version"`
	ReleaseNote string `json:"release_note"`
	ShownAt     string `json:"shown_at,omitempty"`
}

// UpdateProgress represents the current update progress
type UpdateProgress struct {
	Status     string   `json:"status"`   // idle, checking, downloading, verifying, replacing, restarting, done, error
	Progress   int      `json:"progress"` // 0-100
	Message    string   `json:"message"`
	Log        []string `json:"log"`
	Error      string   `json:"error,omitempty"`
	StartedAt  string   `json:"started_at,omitempty"`
	FinishedAt string   `json:"finished_at,omitempty"`
}

// Updater handles self-update logic
type Updater struct {
	currentVersion string
	dataDir        string
	cfg            editionConfig
	mu             sync.Mutex
	progress       UpdateProgress
}

// NewUpdater creates a new Updater
func NewUpdater(currentVersion, dataDir, edition string) *Updater {
	return &Updater{
		currentVersion: currentVersion,
		dataDir:        dataDir,
		cfg:            newEditionConfig(edition),
		progress: UpdateProgress{
			Status: "idle",
			Log:    []string{},
		},
	}
}

// getPlatformKey returns the platform key for download URLs
func getPlatformKey() string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	switch {
	case os == "linux" && arch == "amd64":
		return "linux_amd64"
	case os == "linux" && arch == "arm64":
		return "linux_arm64"
	case os == "windows" && arch == "amd64":
		return "windows_amd64"
	case os == "darwin" && arch == "amd64":
		return "darwin_amd64"
	case os == "darwin" && arch == "arm64":
		return "darwin_arm64"
	default:
		return os + "_" + arch
	}
}

// CheckUpdate checks for available updates
func (u *Updater) CheckUpdate() (*UpdateInfo, bool, error) {
	info, err := u.resolveLocalUpdate(false, false)
	if err != nil {
		return nil, false, fmt.Errorf("请求更新信息失败: %v", err)
	}

	hasUpdate := info.LatestVersion != "" && info.LatestVersion != u.currentVersion && isNewerVersion(info.LatestVersion, u.currentVersion)

	return info, hasUpdate, nil
}

// GetProgress returns the current update progress
func (u *Updater) GetProgress() UpdateProgress {
	u.mu.Lock()
	defer u.mu.Unlock()
	p := u.progress
	logCopy := make([]string, len(u.progress.Log))
	copy(logCopy, u.progress.Log)
	p.Log = logCopy
	return p
}

// DoUpdate performs the self-update
func (u *Updater) DoUpdate(info *UpdateInfo) {
	if u.cfg.isLiteFullPackage() {
		u.mu.Lock()
		u.progress = UpdateProgress{
			Status:     "error",
			Progress:   0,
			Message:    "Lite 版请使用整包更新入口",
			Log:        []string{"Lite 版当前不支持通过面板二进制热替换更新，请使用整包更新入口。"},
			Error:      "Lite 版请使用整包更新入口",
			FinishedAt: time.Now().Format(time.RFC3339),
		}
		u.mu.Unlock()
		return
	}
	u.mu.Lock()
	if u.progress.Status == "downloading" || u.progress.Status == "verifying" || u.progress.Status == "replacing" {
		u.mu.Unlock()
		return
	}
	u.progress = UpdateProgress{
		Status:    "downloading",
		Progress:  0,
		Message:   "准备下载更新...",
		Log:       []string{},
		StartedAt: time.Now().Format(time.RFC3339),
	}
	u.mu.Unlock()

	go u.doUpdateAsync(info)
}

func (u *Updater) doUpdateAsync(info *UpdateInfo) {
	u.log("🔍 检测平台: %s/%s", runtime.GOOS, runtime.GOARCH)

	platformKey := getPlatformKey()
	localAssetPath := strings.TrimSpace(info.LocalPaths[platformKey])
	if localAssetPath == "" {
		refreshed, err := u.resolveLocalUpdate(true, true)
		if err != nil {
			u.setError("刷新本机更新镜像失败: %v", err)
			return
		}
		info = refreshed
		localAssetPath = strings.TrimSpace(info.LocalPaths[platformKey])
	}
	if localAssetPath != "" {
		if _, err := os.Stat(localAssetPath); err != nil {
			localAssetPath = ""
		}
	}
	if localAssetPath == "" {
		refreshed, err := u.resolveLocalUpdate(true, true)
		if err != nil {
			u.setError("刷新本机更新镜像失败: %v", err)
			return
		}
		info = refreshed
		localAssetPath = strings.TrimSpace(info.LocalPaths[platformKey])
	}
	if localAssetPath == "" {
		u.setError("不支持的平台: %s", platformKey)
		return
	}
	expectedSHA, _ := info.SHA256[platformKey]
	if strings.TrimSpace(expectedSHA) == "" {
		u.setError("当前平台缺少 SHA256 校验值，已拒绝更新")
		return
	}

	u.log("📥 同步本机更新缓存: %s → %s", info.LatestVersion, localAssetPath)
	u.setStatus("downloading", 10, "正在准备本机更新包...")

	tmpDir := filepath.Join(u.dataDir, "update-tmp")
	os.MkdirAll(tmpDir, 0755)
	tmpFile := filepath.Join(tmpDir, "clawpanel-new")
	if runtime.GOOS == "windows" {
		tmpFile += ".exe"
	}

	if err := copyFile(localAssetPath, tmpFile); err != nil {
		u.setError("复制本机更新包失败: %v", err)
		return
	}
	u.log("✅ 本机更新包已就绪")

	// SHA256 verify
	u.setStatus("verifying", 60, "正在校验文件完整性...")
	if expectedSHA != "" {
		actualSHA, err := fileSHA256(tmpFile)
		if err != nil {
			u.setError("校验失败: %v", err)
			return
		}
		if !strings.EqualFold(actualSHA, expectedSHA) {
			u.setError("SHA256 校验失败: 期望 %s, 实际 %s\n更新包可能损坏，请重新尝试", expectedSHA[:16]+"...", actualSHA[:16]+"...")
			os.Remove(tmpFile)
			return
		}
		u.log("✅ SHA256 校验通过")
	} else {
		u.log("⚠️ 未提供 SHA256 校验值，跳过校验")
	}

	// Replace binary
	u.setStatus("replacing", 80, "正在替换程序...")
	currentBin, err := os.Executable()
	if err != nil {
		u.setError("获取当前程序路径失败: %v", err)
		return
	}
	currentBin, _ = filepath.EvalSymlinks(currentBin)
	u.log("📍 当前程序: %s", currentBin)

	// Save update popup info before replace (in case restart kills us)
	u.saveUpdatePopup(info)
	u.log("💾 更新信息已保存")

	// On Linux/macOS, write an external updater script that:
	// 1. Stops the service
	// 2. Replaces the binary (rm + cp)
	// 3. Starts the service
	// This avoids corrupting a running binary and the self-restart race condition.
	if runtime.GOOS != "windows" {
		scriptPath := filepath.Join(tmpDir, "do-update.sh")
		script := fmt.Sprintf(`#!/bin/bash
set -e
echo "[ClawPanel Updater] 开始更新..."

# Stop service
echo "[ClawPanel Updater] 停止 ClawPanel 服务..."
systemctl stop %s 2>/dev/null || true
sleep 1

# Wait for process to exit (up to 10s)
for i in $(seq 1 10); do
  if ! pgrep -x %s >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

# Kill if still running
if pgrep -x %s >/dev/null 2>&1; then
  echo "[ClawPanel Updater] 强制停止旧进程..."
  pkill -9 -x %s 2>/dev/null || true
  sleep 1
fi

# Backup old binary
if [ -f "%s" ]; then
  cp -f "%s" "%s.bak" 2>/dev/null || true
  echo "[ClawPanel Updater] 已备份旧程序"
fi

# Replace: remove old then copy new
rm -f "%s"
cp -f "%s" "%s"
chmod +x "%s"
echo "[ClawPanel Updater] 程序替换完成"

# Start service
echo "[ClawPanel Updater] 启动 ClawPanel 服务..."
systemctl start %s 2>/dev/null || ( echo "[ClawPanel Updater] systemctl 启动失败，尝试直接启动..." && nohup "%s" >/dev/null 2>&1 & )
echo "[ClawPanel Updater] 更新完成!"

# Clean up
rm -f "%s"
rm -rf "%s"
`, u.cfg.ServiceName, u.cfg.BinaryName, u.cfg.BinaryName, u.cfg.BinaryName, u.cfg.ServiceName, currentBin, currentBin, currentBin, currentBin, tmpFile, currentBin, currentBin, currentBin, scriptPath, tmpDir)

		if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
			u.setError("写入更新脚本失败: %v", err)
			return
		}
		u.log("📝 更新脚本已生成: %s", scriptPath)

		u.setStatus("restarting", 95, "即将停止服务并替换程序...")
		u.log("🔄 ClawPanel 即将停止，更新脚本将接管...")

		u.mu.Lock()
		u.progress.Status = "done"
		u.progress.Progress = 100
		u.progress.Message = "更新完成，正在重启..."
		u.progress.FinishedAt = time.Now().Format(time.RFC3339)
		u.mu.Unlock()

		// Spawn the external script (detached, won't die with us)
		go func() {
			time.Sleep(1 * time.Second)
			cmd := exec.Command("bash", "-c", "setsid bash "+scriptPath+" </dev/null >/dev/null 2>&1 &")
			cmd.Stdout = nil
			cmd.Stderr = nil
			if err := cmd.Start(); err != nil {
				log.Printf("[Updater] 启动更新脚本失败: %v, 尝试直接替换...", err)
				// Fallback: direct replace + systemctl restart
				os.Remove(currentBin)
				copyFile(tmpFile, currentBin)
				os.Chmod(currentBin, 0755)
				execCmd("systemctl", "restart", u.cfg.ServiceName)
				return
			}
			log.Printf("[Updater] 更新脚本已启动 (PID: %d)，等待接管...", cmd.Process.Pid)
			// Release the child so it doesn't become a zombie
			cmd.Process.Release()
		}()
		return
	}

	// Windows path: rename approach
	backupPath := currentBin + ".bak"
	os.Remove(backupPath)
	if err := os.Rename(currentBin, backupPath); err != nil {
		u.setError("备份旧程序失败: %v", err)
		return
	}
	u.log("📦 已备份旧程序: %s", backupPath)

	if err := copyFile(tmpFile, currentBin); err != nil {
		os.Rename(backupPath, currentBin)
		u.setError("替换程序失败: %v", err)
		return
	}
	os.Chmod(currentBin, 0755)
	u.log("✅ 程序替换完成")

	os.Remove(tmpFile)
	os.RemoveAll(tmpDir)

	u.setStatus("restarting", 95, "即将重启 ClawPanel...")
	u.log("🔄 ClawPanel 即将重启，请等待...")

	u.mu.Lock()
	u.progress.Status = "done"
	u.progress.Progress = 100
	u.progress.Message = "更新完成，正在重启..."
	u.progress.FinishedAt = time.Now().Format(time.RFC3339)
	u.mu.Unlock()

	go func() {
		time.Sleep(1 * time.Second)
		if err := execCmd("net", "stop", "ClawPanel"); err == nil {
			execCmd("net", "start", "ClawPanel")
			return
		}
		log.Printf("[Updater] Windows service restart failed, exiting...")
		os.Exit(0)
	}()
}

func (u *Updater) resolveLocalUpdate(force bool, ensureAsset bool) (*UpdateInfo, error) {
	manifest, err := updatemirror.ResolveLatest(u.dataDir, updatemirror.EditionSpec{
		Edition:           u.cfg.Edition,
		GitHubReleasesAPI: u.cfg.GitHubReleasesAPI,
		GitHubTagPrefix:   u.cfg.GitHubTagPrefix,
	}, "/api/panel/update-mirror", force)
	if err != nil {
		return nil, err
	}
	if ensureAsset {
		platformKey := getPlatformKey()
		if _, err := updatemirror.EnsureAsset(u.dataDir, updatemirror.EditionSpec{
			Edition:           u.cfg.Edition,
			GitHubReleasesAPI: u.cfg.GitHubReleasesAPI,
			GitHubTagPrefix:   u.cfg.GitHubTagPrefix,
		}, manifest, platformKey); err != nil {
			return nil, err
		}
	}
	return &UpdateInfo{
		LatestVersion: manifest.LatestVersion,
		ReleaseTime:   manifest.ReleaseTime,
		ReleaseNote:   manifest.ReleaseNote,
		DownloadURLs:  manifest.DownloadURLs,
		SHA256:        manifest.SHA256,
		LocalPaths:    manifest.LocalPaths,
	}, nil
}

func (u *Updater) fetchFromAccel() (*UpdateInfo, error) {
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(u.cfg.AccelUpdateURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var info UpdateInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (u *Updater) fetchFromGitHub() (*UpdateInfo, error) {
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(u.cfg.GitHubReleasesAPI)
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
		if !u.cfg.matchesTag(release.TagName) {
			continue
		}
		info := &UpdateInfo{
			LatestVersion: u.cfg.trimTag(release.TagName),
			ReleaseTime:   release.PubAt,
			ReleaseNote:   release.Body,
			DownloadURLs:  map[string]string{},
			SHA256:        map[string]string{},
		}
		for _, a := range release.Assets {
			if platformKey, ok := u.cfg.matchUpdateAsset(info.LatestVersion, a.Name); ok {
				info.DownloadURLs[platformKey] = a.URL
			}
		}
		return info, nil
	}
	return nil, fmt.Errorf("未找到 %s 版本发布", u.cfg.Edition)
}

func (u *Updater) downloadFile(url, dest string) error {
	client := &http.Client{Timeout: downloadTimeout}
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
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			if totalSize > 0 {
				pct := int(float64(downloaded)/float64(totalSize)*50) + 10 // 10-60%
				u.setStatus("downloading", pct, fmt.Sprintf("正在下载... %.1f MB / %.1f MB", float64(downloaded)/1048576, float64(totalSize)/1048576))
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

func (u *Updater) saveUpdatePopup(info *UpdateInfo) {
	popup := UpdatePopup{
		Show:        true,
		Version:     info.LatestVersion,
		ReleaseNote: info.ReleaseNote,
	}
	data, _ := json.MarshalIndent(popup, "", "  ")
	os.WriteFile(filepath.Join(u.dataDir, "update_popup.json"), data, 0644)
}

// GetUpdatePopup reads the popup info
func (u *Updater) GetUpdatePopup() *UpdatePopup {
	data, err := os.ReadFile(filepath.Join(u.dataDir, "update_popup.json"))
	if err != nil {
		return nil
	}
	var popup UpdatePopup
	if err := json.Unmarshal(data, &popup); err != nil {
		return nil
	}
	return &popup
}

// MarkPopupShown marks the popup as shown
func (u *Updater) MarkPopupShown() {
	popup := u.GetUpdatePopup()
	if popup == nil {
		return
	}
	popup.Show = false
	popup.ShownAt = time.Now().Format(time.RFC3339)
	data, _ := json.MarshalIndent(popup, "", "  ")
	os.WriteFile(filepath.Join(u.dataDir, "update_popup.json"), data, 0644)
}

func (u *Updater) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[Updater] %s", msg)
	u.mu.Lock()
	u.progress.Log = append(u.progress.Log, msg)
	u.mu.Unlock()
}

func (u *Updater) setStatus(status string, progress int, message string) {
	u.mu.Lock()
	u.progress.Status = status
	u.progress.Progress = progress
	u.progress.Message = message
	u.mu.Unlock()
}

func (u *Updater) setError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[Updater] ERROR: %s", msg)
	u.mu.Lock()
	u.progress.Status = "error"
	u.progress.Error = msg
	u.progress.Message = "更新失败"
	u.progress.Log = append(u.progress.Log, "❌ "+msg)
	u.progress.FinishedAt = time.Now().Format(time.RFC3339)
	u.mu.Unlock()
}

// isNewerVersion compares semver strings like "v5.0.2" > "v5.0.1"
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

func execCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}
