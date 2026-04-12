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
	"sort"
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
		m.offlineCount++
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
		// Container running but OneBot services (3001/3000) not responding.
		// If port 6099 is up, NapCat WebUI is alive — it's just waiting for QR scan.
		// Treat as login_expired (don't auto-restart — user needs to scan QR code).
		webuiUp := isPortReachable(6099)
		m.offlineCount++
		if webuiUp {
			// NapCat is running and reachable — login needed, not a crash
			m.status.Status = "login_expired"
			m.loginFailCount++
			m.offlineCount = 0 // reset so auto-reconnect doesn't fire
		} else if m.offlineCount <= 3 {
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

	// When NapCat is running but WS/HTTP not yet listening, ensure network config exists.
	// This handles the case where QQ just logged in and config was empty.
	if containerRunning && (!wsConnected || !httpAvailable) && isPortReachable(6099) {
		if runtime.GOOS == "windows" {
			napcatDir := findNapCatShellDir(m.cfg)
			if napcatDir != "" {
				go ensureNapCatNetworkConfig(napcatDir)
			}
		} else {
			go ensureNapCatDockerNetworkConfig(m.cfg)
		}
	}

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

	// Auto-reconnect: trigger for "offline" (services down) or "stopped" (process not running).
	// NOT for "login_expired" — that means NapCat WebUI is up but QQ session expired;
	// user needs to re-scan QR code.
	if autoReconnect && (currentStatus == "offline" || currentStatus == "stopped") && offlineCount >= 3 {
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

	// Wait for NapCat to start — on Windows with schtasks there's a delay before the
	// process actually launches. Poll port 6099 (WebUI) since 3001/3000 only come up
	// after QQ login which requires a QR scan.
	time.Sleep(8 * time.Second)

	webuiOK := false
	for i := 0; i < 12; i++ {
		if isPortReachable(6099) {
			webuiOK = true
			break
		}
		time.Sleep(5 * time.Second)
	}

	if webuiOK {
		rlog.Success = true
		rlog.Detail = "NapCat 已重启，WebUI 已可用，等待 QQ 扫码登录"
		m.addLog(rlog)

		m.mu.Lock()
		m.status.ContainerRunning = true
		m.status.Status = "login_expired"
		m.mu.Unlock()
		m.broadcastStatus()

		m.sysLog.Log("system", "napcat.reconnected_no_login", "NapCat 已重启，请扫码登录 QQ")
		return nil
	}

	rlog.Success = false
	rlog.Detail = "NapCat 重启后 WebUI (port 6099) 未恢复"
	m.addLog(rlog)

	m.mu.Lock()
	m.status.Status = "offline"
	m.mu.Unlock()
	m.broadcastStatus()

	m.sysLog.Log("system", "napcat.reconnect_failed", "NapCat 重连后 WebUI 未恢复")
	return fmt.Errorf("NapCat WebUI 未恢复")
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

// isNapCatProcessRunning checks if NapCat is running in the interactive session (session 1).
// On Windows we check for NapCatWinBootMain.exe AND that port 6099 is reachable,
// because a zombie session-0 process with no port is not "running" for our purposes.
func isNapCatProcessRunning() bool {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq NapCatWinBootMain.exe", "/NH").Output()
		if err != nil || !strings.Contains(string(out), "NapCatWinBootMain") {
			out2, err2 := exec.Command("tasklist", "/FI", "IMAGENAME eq napcat.exe", "/NH").Output()
			if err2 != nil || !strings.Contains(string(out2), "napcat.exe") {
				return false
			}
		}
		// Process exists — consider it running only if port 6099 is actually listening.
		// A zombie NapCat (lost QQ pipe, no port) should be treated as stopped so
		// the monitor can restart it.
		return isPortReachable(6099)
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

// restartNapCatPlatform restarts NapCat based on platform.
// On Windows: only kills if port 6099 is already down (NapCat crashed), then relaunches
// via schtasks in the interactive user session.
func restartNapCatPlatform(cfg *config.Config) ([]byte, error) {
	if runtime.GOOS == "windows" {
		napcatDir := findNapCatShellDir(cfg)
		if napcatDir == "" {
			return []byte("NapCat Shell directory not found"), fmt.Errorf("NapCat Shell not installed")
		}
		exePath := filepath.Join(napcatDir, "NapCatWinBootMain.exe")
		if _, err := os.Stat(exePath); err != nil {
			return []byte("No NapCatWinBootMain.exe found"), fmt.Errorf("NapCat executable not found")
		}
		// Only kill if WebUI port is already down — don't destroy a working session-1 instance.
		if !isPortReachable(6099) {
			exec.Command("taskkill", "/F", "/IM", "NapCatWinBootMain.exe").Run()
			exec.Command("taskkill", "/F", "/IM", "napcat.exe").Run()
			exec.Command("taskkill", "/F", "/IM", "QQ.exe").Run()
			time.Sleep(3 * time.Second)
		} else {
			log.Println("[NapCat] port 6099 is up — skipping kill, NapCat is alive")
			return []byte("NapCat already running"), nil
		}
		// Write network config (WS+HTTP) before launching so NapCat picks it up.
		ensureNapCatNetworkConfig(napcatDir)
		if err := launchNapCatInUserSession(exePath, napcatDir); err != nil {
			return []byte(err.Error()), err
		}
		return []byte("NapCat Shell restarted"), nil
	}
	// Linux/macOS: Docker
	return dockerOutput("restart", "openclaw-qq")
}

// ensureNapCatNetworkConfig writes the OneBot network config (WS+HTTP) for all known
// QQ UINs found in the NapCat config directory. NapCat reads onebot11_<uin>.json on
// startup; if the file has an empty network block, ports 3001/3000 will never open.
func ensureNapCatNetworkConfig(napcatShellDir string) {
	// Find the inner napcat dir (where config/ lives)
	innerDir := findNapCatInnerDir(napcatShellDir)
	if innerDir == "" {
		innerDir = napcatShellDir
	}
	cfgDir := filepath.Join(innerDir, "config")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		log.Printf("[NapCat] ensureNapCatNetworkConfig: mkdir %s: %v", cfgDir, err)
		return
	}

	networkCfg := map[string]interface{}{
		"network": map[string]interface{}{
			"httpServers": []interface{}{
				map[string]interface{}{
					"name":              "ClawPanel-HTTP",
					"enable":            true,
					"port":              3000,
					"host":              "0.0.0.0",
					"enableCors":        true,
					"enableWebsocket":   false,
					"messagePostFormat": "array",
					"token":             "",
					"debug":             false,
				},
			},
			"httpSseServers": []interface{}{},
			"httpClients":    []interface{}{},
			"websocketServers": []interface{}{
				map[string]interface{}{
					"name":                 "ClawPanel-WS",
					"enable":               true,
					"port":                 3001,
					"host":                 "0.0.0.0",
					"messagePostFormat":    "array",
					"token":                "",
					"reportSelfMessage":    false,
					"enableForcePushEvent": true,
					"debug":                false,
					"heartInterval":        30000,
				},
			},
			"websocketClients": []interface{}{},
			"plugins":          []interface{}{},
		},
		"musicSignUrl":        "",
		"enableLocalFile2Url": false,
		"parseMultMsg":        false,
		"imageDownloadProxy":  "",
	}
	data, _ := json.MarshalIndent(networkCfg, "", "  ")

	// Write for all existing onebot11_<uin>.json files, and also a default napcat.json
	entries, _ := os.ReadDir(cfgDir)
	uins := []string{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "onebot11_") && strings.HasSuffix(name, ".json") {
			uin := strings.TrimSuffix(strings.TrimPrefix(name, "onebot11_"), ".json")
			if uin != "" {
				uins = append(uins, uin)
			}
		}
	}
	// Also check napcat_<uin>.json to find UINs even if onebot11 file doesn't exist yet
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "napcat_") && strings.HasSuffix(name, ".json") &&
			!strings.HasPrefix(name, "napcat_protocol_") {
			uin := strings.TrimSuffix(strings.TrimPrefix(name, "napcat_"), ".json")
			if uin != "" {
				found := false
				for _, u := range uins {
					if u == uin {
						found = true
						break
					}
				}
				if !found {
					uins = append(uins, uin)
				}
			}
		}
	}

	// Always write at least one config
	if len(uins) == 0 {
		p := filepath.Join(cfgDir, "onebot11.json")
		if err := os.WriteFile(p, data, 0644); err != nil {
			log.Printf("[NapCat] write %s: %v", p, err)
		} else {
			log.Printf("[NapCat] wrote default network config: %s", p)
		}
		return
	}

	for _, uin := range uins {
		p := filepath.Join(cfgDir, "onebot11_"+uin+".json")
		// Only overwrite if network is empty (don't clobber user customisations)
		existing, err := os.ReadFile(p)
		if err == nil {
			var cur map[string]interface{}
			if json.Unmarshal(existing, &cur) == nil {
				if net, ok := cur["network"].(map[string]interface{}); ok {
					wsServers, _ := net["websocketServers"].([]interface{})
					httpServers, _ := net["httpServers"].([]interface{})
					if len(wsServers) > 0 || len(httpServers) > 0 {
						log.Printf("[NapCat] network config already set for UIN %s, skipping", uin)
						continue
					}
				}
			}
		}
		if err := os.WriteFile(p, data, 0644); err != nil {
			log.Printf("[NapCat] write %s: %v", p, err)
		} else {
			log.Printf("[NapCat] wrote network config for UIN %s: %s", uin, p)
		}
	}
}

func ensureNapCatDockerNetworkConfig(cfg *config.Config) {
	if runtime.GOOS == "windows" || cfg == nil {
		return
	}
	_, wsToken, _ := cfg.ReadQQChannelState()
	data, err := marshalMonitorOneBot11Config(wsToken)
	if err != nil {
		log.Printf("[NapCat] ensureNapCatDockerNetworkConfig marshal failed: %v", err)
		return
	}

	paths := []string{"/app/napcat/config/onebot11.json"}
	out, err := dockerOutput("exec", "openclaw-qq", "sh", "-c", "ls /app/napcat/config/onebot11_*.json /app/napcat/config/napcat_*.json 2>/dev/null || true")
	if err == nil {
		uins := map[string]bool{}
		for _, raw := range strings.Fields(string(out)) {
			base := filepath.Base(raw)
			switch {
			case strings.HasPrefix(base, "onebot11_") && strings.HasSuffix(base, ".json"):
				uin := strings.TrimSuffix(strings.TrimPrefix(base, "onebot11_"), ".json")
				if uin != "" {
					uins[uin] = true
				}
			case strings.HasPrefix(base, "napcat_") && strings.HasSuffix(base, ".json") && !strings.HasPrefix(base, "napcat_protocol_"):
				uin := strings.TrimSuffix(strings.TrimPrefix(base, "napcat_"), ".json")
				if uin != "" {
					uins[uin] = true
				}
			}
		}
		ordered := make([]string, 0, len(uins))
		for uin := range uins {
			ordered = append(ordered, uin)
		}
		sort.Strings(ordered)
		for _, uin := range ordered {
			paths = append(paths, "/app/napcat/config/onebot11_"+uin+".json")
		}
	}

	for _, target := range paths {
		existing, err := dockerOutput("exec", "openclaw-qq", "cat", target)
		if err == nil && !shouldRewriteOneBotNetworkConfig(existing) {
			continue
		}
		if err := dockerWriteFile(target, data); err != nil {
			log.Printf("[NapCat] write docker network config %s failed: %v", target, err)
			continue
		}
		log.Printf("[NapCat] ensured docker network config: %s", target)
	}
}

func marshalMonitorOneBot11Config(wsToken string) ([]byte, error) {
	payload := map[string]interface{}{
		"network": map[string]interface{}{
			"websocketServers": []map[string]interface{}{{
				"name":                 "ClawPanel-WS",
				"enable":               true,
				"host":                 "0.0.0.0",
				"port":                 3001,
				"messagePostFormat":    "array",
				"token":                strings.TrimSpace(wsToken),
				"reportSelfMessage":    true,
				"enableForcePushEvent": true,
				"debug":                false,
				"heartInterval":        30000,
			}},
			"httpServers": []map[string]interface{}{{
				"name":              "ClawPanel-HTTP",
				"enable":            true,
				"port":              3000,
				"host":              "0.0.0.0",
				"enableCors":        true,
				"enableWebsocket":   false,
				"messagePostFormat": "array",
				"token":             "",
				"debug":             false,
			}},
			"httpSseServers":   []interface{}{},
			"httpClients":      []interface{}{},
			"websocketClients": []interface{}{},
			"plugins":          []interface{}{},
		},
		"musicSignUrl":        "",
		"enableLocalFile2Url": true,
		"parseMultMsg":        true,
		"imageDownloadProxy":  "",
	}
	return json.MarshalIndent(payload, "", "  ")
}

func shouldRewriteOneBotNetworkConfig(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	var cur map[string]interface{}
	if json.Unmarshal(raw, &cur) != nil {
		return true
	}
	netCfg, _ := cur["network"].(map[string]interface{})
	if netCfg == nil {
		return true
	}
	wsServers, _ := netCfg["websocketServers"].([]interface{})
	httpServers, _ := netCfg["httpServers"].([]interface{})
	return len(wsServers) == 0 && len(httpServers) == 0
}

func dockerWriteFile(target string, data []byte) error {
	tmp, err := os.CreateTemp("", "clawpanel-napcat-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if out, err := exec.Command("docker", "cp", tmpPath, "openclaw-qq:"+target).CombinedOutput(); err != nil {
		return fmt.Errorf("docker cp failed: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// findQQInstallDir returns the directory containing QQ.exe by checking running processes
func findQQInstallDir() string {
	// Try to find QQ.exe path from running processes via WMIC
	out, err := exec.Command("wmic", "process", "where", "name='QQ.exe'", "get", "ExecutablePath", "/VALUE").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "executablepath=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 && parts[1] != "" {
					return filepath.Dir(strings.TrimSpace(parts[1]))
				}
			}
		}
	}
	// Common QQ install locations
	for _, p := range []string{`C:\Program Files\Tencent\QQ`, `D:\QQ`, `C:\QQ`, `D:\Program Files\Tencent\QQ`} {
		if _, err := os.Stat(filepath.Join(p, "QQ.exe")); err == nil {
			return p
		}
	}
	return ""
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
	// Note: do NOT add the QQ install dir — NapCat Shell is a separate directory.
	// QQ's own dir may contain NapCatWinBootMain.exe in subdirectories which would
	// cause us to return the wrong dir.
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

// launchNapCatInUserSession launches NapCatWinBootMain.exe in the interactive user session.
// ClawPanel runs as a SYSTEM service (session 0) and cannot directly start GUI processes
// in session 1. We write a temp .ps1 file and use schtasks to run it as the interactive
// user, which executes in their desktop session.
// getInteractiveUsername returns the username of the currently logged-in interactive user.
func getInteractiveUsername() string {
	// WMIC outputs UTF-16LE; decode it properly.
	out, err := exec.Command("wmic", "computersystem", "get", "UserName", "/VALUE").Output()
	if err == nil {
		text := decodeUTF16LE(out)
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "username=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	// Fallback: qwinsta.exe — shows active console sessions
	out2, err := exec.Command(`C:\Windows\System32\qwinsta.exe`).Output()
	if err == nil {
		for _, line := range strings.Split(string(out2), "\n") {
			if strings.Contains(line, "Active") || strings.Contains(line, "活动") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					u := strings.TrimPrefix(fields[0], ">")
					if u != "" && !strings.EqualFold(u, "services") && !strings.EqualFold(u, "console") && !strings.HasPrefix(strings.ToLower(u), "rdp-") {
						// qwinsta lists session name not username; use field[1] if field[0] looks like session name
						if strings.HasPrefix(strings.ToLower(u), "console") || strings.HasPrefix(strings.ToLower(u), "rdp") {
							u = fields[1]
						}
						return u
					}
				}
			}
		}
	}
	return ""
}

// decodeUTF16LE decodes a UTF-16LE byte slice to a UTF-8 string.
// WMIC on Windows outputs UTF-16LE; reading it as []byte gives garbled text.
func decodeUTF16LE(b []byte) string {
	// Strip BOM if present
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		b = b[2:]
	}
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	var sb strings.Builder
	for i := 0; i < len(u16); {
		c := rune(u16[i])
		i++
		if c >= 0xD800 && c <= 0xDBFF && i < len(u16) {
			low := rune(u16[i])
			if low >= 0xDC00 && low <= 0xDFFF {
				c = 0x10000 + (c-0xD800)*0x400 + (low - 0xDC00)
				i++
			}
		}
		sb.WriteRune(c)
	}
	return sb.String()
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

func napCatBool(v interface{}) bool {
	switch value := v.(type) {
	case bool:
		return value
	case string:
		value = strings.TrimSpace(strings.ToLower(value))
		return value == "true" || value == "1" || value == "yes" || value == "online"
	case float64:
		return value != 0
	case int:
		return value != 0
	case int64:
		return value != 0
	default:
		return false
	}
}

func napCatString(v interface{}) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		if value == float64(int64(value)) {
			return fmt.Sprintf("%.0f", value)
		}
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	case int:
		return fmt.Sprintf("%d", value)
	case int64:
		return fmt.Sprintf("%d", value)
	default:
		return ""
	}
}

func getNapCatWebUIToken(cfg *config.Config) string {
	if runtime.GOOS == "windows" {
		napcatDir := findNapCatShellDir(cfg)
		if napcatDir == "" {
			return ""
		}
		if tok := readTokenFromNapCatLogs(napcatDir); tok != "" {
			return tok
		}
		webuiPath := filepath.Join(napcatDir, "config", "webui.json")
		if data, err := os.ReadFile(webuiPath); err == nil {
			var webui map[string]interface{}
			if json.Unmarshal(data, &webui) == nil {
				if t, ok := webui["token"].(string); ok && t != "" {
					return t
				}
			}
		}
		return ""
	}

	out, err := dockerOutput("exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json")
	if err != nil {
		return ""
	}
	var webui map[string]interface{}
	if json.Unmarshal(out, &webui) != nil {
		return ""
	}
	if t, ok := webui["token"].(string); ok && t != "" {
		return t
	}
	return ""
}

// readTokenFromNapCatLogs finds NapCat's logs dir (walking up/down from bootmain dir)
// and extracts the live token logged as "[WebUi] WebUi Token: <token>".
func readTokenFromNapCatLogs(bootmainDir string) string {
	// Walk down from bootmain dir looking for a logs/ directory with .log files
	logsDir := ""
	filepath.WalkDir(bootmainDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || logsDir != "" {
			return nil
		}
		if d.IsDir() && strings.EqualFold(d.Name(), "logs") {
			entries, _ := os.ReadDir(path)
			for _, e := range entries {
				if strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
					logsDir = path
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	// Also walk up parent dirs
	if logsDir == "" {
		dir := bootmainDir
		for i := 0; i < 8; i++ {
			candidate := filepath.Join(dir, "logs")
			if entries, err := os.ReadDir(candidate); err == nil {
				for _, e := range entries {
					if strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
						logsDir = candidate
						break
					}
				}
			}
			if logsDir != "" {
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if logsDir == "" {
		return ""
	}
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return ""
	}
	var newest os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			if newest == nil || e.Name() > newest.Name() {
				newest = e
			}
		}
	}
	if newest == nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(logsDir, newest.Name()))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if idx := strings.Index(line, "[WebUi] WebUi Token: "); idx >= 0 {
			tok := strings.TrimSpace(line[idx+len("[WebUi] WebUi Token: "):])
			if tok != "" {
				return tok
			}
		}
	}
	return ""
}

func doCheckQQLoginStatus(cfg *config.Config) (loggedIn bool, nickname string, qqID string) {
	client := &http.Client{Timeout: 5 * time.Second}

	// Use cached credential if available and fresh (< 5 minutes)
	// NapCat 4.x generates a new random token each startup — clear cache when it changes.
	cred := ""
	if cachedMonitorCred != "" && time.Since(cachedMonitorCredTime) < 5*time.Minute {
		cred = cachedMonitorCred
	} else {
		// Invalidate cache unconditionally so we always re-auth with the fresh token
		cachedMonitorCred = ""
		// Get WebUI token: from NapCat log / webui.json / Docker config.
		token := getNapCatWebUIToken(cfg)
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

	// If unauthorized, clear cached cred and immediately retry once with a fresh auth.
	if code, ok := statusResp["code"].(float64); ok && code == -1 {
		cachedMonitorCred = ""
		cachedMonitorCredTime = time.Time{}

		freshToken := getNapCatWebUIToken(cfg)
		if freshToken == "" {
			return false, "", ""
		}
		hash := sha256.Sum256([]byte(freshToken + ".napcat"))
		loginBody := fmt.Sprintf(`{"hash":"%x"}`, hash)
		respRetry, err := client.Post("http://127.0.0.1:6099/api/auth/login", "application/json", strings.NewReader(loginBody))
		if err != nil {
			return false, "", ""
		}
		defer respRetry.Body.Close()
		bodyRetry, _ := io.ReadAll(respRetry.Body)
		var loginRespRetry map[string]interface{}
		if json.Unmarshal(bodyRetry, &loginRespRetry) != nil {
			return false, "", ""
		}
		if codeRetry, ok := loginRespRetry["code"].(float64); !ok || codeRetry != 0 {
			return false, "", ""
		}
		dataRetry, _ := loginRespRetry["data"].(map[string]interface{})
		freshCred, _ := dataRetry["Credential"].(string)
		if freshCred == "" {
			return false, "", ""
		}
		cred = freshCred
		cachedMonitorCred = freshCred
		cachedMonitorCredTime = time.Now()

		reqRetry, _ := http.NewRequest("POST", "http://127.0.0.1:6099/api/QQLogin/CheckLoginStatus", nil)
		reqRetry.Header.Set("Content-Type", "application/json")
		reqRetry.Header.Set("Authorization", "Bearer "+freshCred)
		respStatusRetry, err := client.Do(reqRetry)
		if err != nil {
			return false, "", ""
		}
		defer respStatusRetry.Body.Close()
		bodyStatusRetry, _ := io.ReadAll(respStatusRetry.Body)
		if json.Unmarshal(bodyStatusRetry, &statusResp) != nil {
			return false, "", ""
		}
		if codeStatusRetry, ok := statusResp["code"].(float64); !ok || codeStatusRetry != 0 {
			return false, "", ""
		}
	}

	if code, ok := statusResp["code"].(float64); !ok || code != 0 {
		return false, "", ""
	}
	statusData, _ := statusResp["data"].(map[string]interface{})
	if statusData == nil {
		return false, "", ""
	}
	isLogin := napCatBool(statusData["isLogin"])
	if !isLogin {
		return false, "", ""
	}
	nickname = napCatString(statusData["nick"])
	if nickname == "" {
		nickname = napCatString(statusData["nickname"])
	}
	qqID = napCatString(statusData["uin"])
	if qqID == "" {
		qqID = napCatString(statusData["uid"])
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
			if nick := napCatString(infoData["nick"]); nick != "" {
				nickname = nick
			}
			if nick := napCatString(infoData["nickname"]); nick != "" && nickname == "" {
				nickname = nick
			}
			if uid := napCatString(infoData["uin"]); uid != "" {
				qqID = uid
			}
			if uid := napCatString(infoData["uid"]); uid != "" && qqID == "" {
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
