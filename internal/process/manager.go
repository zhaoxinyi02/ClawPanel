package process

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

// Status 进程状态
type Status struct {
	Running           bool      `json:"running"`
	PID               int       `json:"pid"`
	StartedAt         time.Time `json:"startedAt,omitempty"`
	Uptime            int64     `json:"uptime"` // 秒
	ExitCode          int       `json:"exitCode,omitempty"`
	Daemonized        bool      `json:"daemonized,omitempty"`
	ManagedExternally bool      `json:"managedExternally,omitempty"`
}

// Manager 进程管理器
type Manager struct {
	cfg                *config.Config
	cmd                *exec.Cmd
	daemonized         bool
	bindHostCheck      func(host string) bool
	gatewayProbe       func(host, port string) bool
	lastGatewayProbeAt time.Time
	lastGatewayProbeOK bool
	status             Status
	mu                 sync.RWMutex
	logLines           []string
	logMu              sync.RWMutex
	maxLog             int
	stopCh             chan struct{}
	logReader          io.ReadCloser
}

const gatewayProbeCacheTTL = 3 * time.Second

func gatewayStartupTimeout() time.Duration {
	if runtime.GOOS == "windows" {
		return 30 * time.Second
	}
	return 15 * time.Second
}

var (
	tailnetIPv4Net = mustCIDR("100.64.0.0/10")
	tailnetIPv6Net = mustCIDR("fd7a:115c:a1e0::/48")
)

// NewManager 创建进程管理器
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:    cfg,
		maxLog: 5000,
		stopCh: make(chan struct{}),
	}
}

// Start 启动 OpenClaw 进程
func (m *Manager) Start() error {
	if status := m.GetStatus(); status.Running {
		if status.ManagedExternally {
			return fmt.Errorf("OpenClaw 网关已由外部进程管理并在运行中")
		}
		return fmt.Errorf("OpenClaw 已在运行中 (PID: %d)", status.PID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status.Running {
		return fmt.Errorf("OpenClaw 已在运行中 (PID: %d)", m.status.PID)
	}
	if gatewayPort := m.getGatewayPort(); gatewayPort != "" && m.isPortListening(gatewayPort) {
		if m.detectGatewayListening() {
			return fmt.Errorf("OpenClaw 网关已由外部进程管理并在运行中")
		}
		return fmt.Errorf("OpenClaw 网关端口 %s 已被其他本地服务占用", gatewayPort)
	}

	// 启动前确保 openclaw.json 配置正确
	m.ensureOpenClawConfig()

	// 查找 openclaw 可执行文件
	openclawBin := m.findOpenClawBin()
	if openclawBin == "" {
		return fmt.Errorf("未找到 openclaw 可执行文件，请确保已安装 OpenClaw")
	}

	// 构建启动命令
	m.cmd = exec.Command(openclawBin, "gateway")
	m.cmd.Dir = m.cfg.OpenClawDir
	m.cmd.Env = append(buildProcessEnv(),
		fmt.Sprintf("OPENCLAW_DIR=%s", m.cfg.OpenClawDir),
		fmt.Sprintf("OPENCLAW_STATE_DIR=%s", m.cfg.OpenClawDir),
		fmt.Sprintf("OPENCLAW_CONFIG_PATH=%s/openclaw.json", m.cfg.OpenClawDir),
	)

	// 捕获 stdout 和 stderr
	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}
	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 stderr 管道失败: %w", err)
	}

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("启动 OpenClaw 失败: %w", err)
	}

	m.status = Status{
		Running:   true,
		PID:       m.cmd.Process.Pid,
		StartedAt: time.Now(),
	}
	m.daemonized = false
	m.lastGatewayProbeAt = time.Time{}
	m.lastGatewayProbeOK = false

	// 合并 stdout 和 stderr
	m.logReader = io.NopCloser(io.MultiReader(stdout, stderr))

	// 后台监控进程退出
	go m.waitForExit()

	log.Printf("[ProcessMgr] OpenClaw 已启动 (PID: %d)", m.status.PID)
	return nil
}

func buildProcessEnv() []string {
	return config.BuildExecEnv()
}

// Stop 停止 OpenClaw 进程
func (m *Manager) Stop() error {
	if status := m.GetStatus(); status.ManagedExternally {
		return fmt.Errorf("OpenClaw 网关当前由外部进程管理，无法在 ClawPanel 内停止")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.daemonized {
		return fmt.Errorf("OpenClaw 当前以 daemon fork 模式运行，ClawPanel 暂不支持直接停止；请使用网关重启或在外部环境中停止")
	}
	if !m.status.Running {
		return fmt.Errorf("OpenClaw 未在运行")
	}

	log.Printf("[ProcessMgr] 正在停止 OpenClaw (PID: %d)...", m.status.PID)

	gatewayPort := m.getGatewayPort()

	// First, ask OpenClaw CLI to stop the daemon gateway process.
	if bin := m.findOpenClawBin(); bin != "" {
		cmd := exec.Command(bin, "gateway", "stop")
		cmd.Dir = m.cfg.OpenClawDir
		cmd.Env = append(buildProcessEnv(),
			fmt.Sprintf("OPENCLAW_DIR=%s", m.cfg.OpenClawDir),
			fmt.Sprintf("OPENCLAW_STATE_DIR=%s", m.cfg.OpenClawDir),
			fmt.Sprintf("OPENCLAW_CONFIG_PATH=%s/openclaw.json", m.cfg.OpenClawDir),
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[ProcessMgr] gateway stop 命令失败: %v (%s)", err, strings.TrimSpace(string(out)))
		}
	}

	// Wait briefly for the gateway port to go down.
	for i := 0; i < 10; i++ {
		if !m.isPortListening(gatewayPort) {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	// On Windows, if daemon still holds the port, force-kill by listening PID.
	if runtime.GOOS == "windows" && m.isPortListening(gatewayPort) {
		if killed := m.killWindowsPortListeners(gatewayPort); killed > 0 {
			log.Printf("[ProcessMgr] 已强制终止 %d 个占用端口 %s 的进程", killed, gatewayPort)
		}
		for i := 0; i < 10; i++ {
			if !m.isPortListening(gatewayPort) {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
	}

	// 先尝试优雅关闭
	if m.cmd != nil && m.cmd.Process != nil {
		if runtime.GOOS == "windows" {
			_ = m.cmd.Process.Kill()
		} else {
			_ = m.cmd.Process.Signal(os.Interrupt)
			// 等待 5 秒，如果还没退出则强制杀死
			done := make(chan struct{})
			go func() {
				_ = m.cmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = m.cmd.Process.Kill()
			}
		}
	}

	if m.isPortListening(gatewayPort) {
		return fmt.Errorf("OpenClaw 网关端口 %s 仍被占用，停止失败", gatewayPort)
	}

	m.status.Running = false
	m.status.PID = 0
	m.status.Daemonized = false
	m.lastGatewayProbeAt = time.Time{}
	m.lastGatewayProbeOK = false
	log.Println("[ProcessMgr] OpenClaw 已停止")
	return nil
}

func (m *Manager) killWindowsPortListeners(port string) int {
	cmd := exec.Command("cmd", "/C", "netstat -ano -p tcp")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	pidSet := map[int]struct{}{}
	needle := ":" + port
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, needle) || !strings.Contains(strings.ToUpper(line), "LISTENING") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil || pid <= 0 {
			continue
		}
		pidSet[pid] = struct{}{}
	}
	killed := 0
	for pid := range pidSet {
		k := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F")
		if err := k.Run(); err == nil {
			killed++
		}
	}
	return killed
}

// Restart 重启 OpenClaw 进程
func (m *Manager) Restart() error {
	status := m.GetStatus()
	if status.ManagedExternally {
		return fmt.Errorf("OpenClaw 网关当前由外部进程管理，请在外部环境中重启")
	}
	m.mu.RLock()
	daemonized := m.daemonized
	m.mu.RUnlock()
	if daemonized {
		return fmt.Errorf("OpenClaw 当前以 daemon fork 模式运行，请使用网关重启或在外部环境中重启")
	}
	if status.Running {
		if err := m.Stop(); err != nil {
			log.Printf("[ProcessMgr] 停止失败: %v", err)
		}
		time.Sleep(time.Second)
	}
	return m.Start()
}

// GatewayListening reports whether an OpenClaw control gateway is already
// reachable on the configured bind targets. The probe requires an HTTP response
// that looks like the OpenClaw control UI, so unrelated listeners on the same
// port do not suppress startup.
func (m *Manager) GatewayListening() bool {
	return m.gatewayListening(false)
}

func (m *Manager) gatewayListening(force bool) bool {
	if !force {
		m.mu.RLock()
		cachedAt := m.lastGatewayProbeAt
		cachedOK := m.lastGatewayProbeOK
		m.mu.RUnlock()
		if !cachedAt.IsZero() && time.Since(cachedAt) < gatewayProbeCacheTTL {
			return cachedOK
		}
	}

	ok := m.detectGatewayListening()
	m.mu.Lock()
	m.lastGatewayProbeAt = time.Now()
	m.lastGatewayProbeOK = ok
	m.mu.Unlock()
	return ok
}

func (m *Manager) detectGatewayListening() bool {
	port, hosts := m.getGatewayProbeTargets()
	if port == "" {
		return false
	}
	probe := m.gatewayProbe
	if probe == nil {
		probe = m.isOpenClawGateway
	}
	for _, host := range hosts {
		if probe(host, port) {
			return true
		}
	}
	return false
}

// StopAll 停止所有进程
func (m *Manager) StopAll() {
	status := m.GetStatus()
	if status.ManagedExternally {
		return
	}
	if status.Running {
		m.Stop()
	}
}

// GetStatus 获取进程状态
func (m *Manager) GetStatus() Status {
	m.mu.RLock()
	s := m.status
	m.mu.RUnlock()

	if s.Running {
		s.Uptime = int64(time.Since(s.StartedAt).Seconds())
		return s
	}
	if m.GatewayListening() {
		s.Running = true
		s.PID = 0
		s.StartedAt = time.Time{}
		s.Uptime = 0
		s.ExitCode = 0
		s.Daemonized = false
		s.ManagedExternally = true
		return s
	}
	if pid, ok := m.detectExternalGatewayProcess(); ok {
		s.Running = true
		s.PID = pid
		s.StartedAt = time.Time{}
		s.Uptime = 0
		s.ExitCode = 0
		s.Daemonized = false
		s.ManagedExternally = true
	}
	return s
}

// GetLogs 获取日志
func (m *Manager) GetLogs(n int) []string {
	m.logMu.RLock()
	defer m.logMu.RUnlock()

	if n <= 0 || n > len(m.logLines) {
		n = len(m.logLines)
	}
	start := len(m.logLines) - n
	if start < 0 {
		start = 0
	}
	result := make([]string, n)
	copy(result, m.logLines[start:])
	return result
}

// StreamLogs 将进程日志流式推送到 WebSocket Hub
func (m *Manager) StreamLogs(hub *websocket.Hub) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastIdx := 0
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.logMu.RLock()
			newLines := m.logLines[lastIdx:]
			lastIdx = len(m.logLines)
			m.logMu.RUnlock()

			for _, line := range newLines {
				hub.Broadcast([]byte(line))
			}
		}
	}
}

// addLogLine 添加日志行
func (m *Manager) addLogLine(line string) {
	m.logMu.Lock()
	defer m.logMu.Unlock()

	m.logLines = append(m.logLines, line)
	if len(m.logLines) > m.maxLog {
		m.logLines = m.logLines[len(m.logLines)-m.maxLog:]
	}
}

// waitForExit 等待进程退出，异常退出时自动重启
func (m *Manager) waitForExit() {
	if m.logReader != nil {
		scanner := bufio.NewScanner(m.logReader)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			m.addLogLine(scanner.Text())
		}
	}

	if m.cmd != nil {
		err := m.cmd.Wait()
		m.mu.Lock()
		wasRunning := m.status.Running
		daemonized := m.daemonized
		startedAt := m.status.StartedAt
		m.status.Running = false
		m.status.Daemonized = false
		m.daemonized = false
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				m.status.ExitCode = exitCode
			}
		}
		m.mu.Unlock()
		log.Printf("[ProcessMgr] OpenClaw 进程已退出 (code: %d)", exitCode)

		// OpenClaw gateway uses a daemon fork pattern: it spawns a child
		// process "openclaw-gateway" that holds the port, then the parent
		// exits (often with code 1). If the gateway port is listening after
		// the parent exits, the daemon started successfully.
		if wasRunning && !daemonized && !startedAt.IsZero() && time.Since(startedAt) < 20*time.Second {
			if m.waitForGatewayReady(gatewayStartupTimeout()) {
				log.Printf("[ProcessMgr] OpenClaw 父进程已退出但网关守护进程仍可探测（daemon fork 模式），视为正常")
				m.mu.Lock()
				m.status.Running = true
				m.status.ExitCode = 0
				m.status.PID = 0
				m.status.Daemonized = true
				m.cmd = nil
				m.daemonized = true
				m.mu.Unlock()
				// Monitor the daemon process; when port goes down, restart
				go m.monitorDaemon()
				return
			}
		}
		if wasRunning && exitCode != 0 && m.gatewayListening(true) {
			log.Printf("[ProcessMgr] OpenClaw 进程退出后网关仍可访问，视为已由现存守护进程/外部实例接管")
			m.mu.Lock()
			m.status.Running = true
			m.status.ExitCode = 0
			m.status.PID = 0
			m.status.Daemonized = false
			m.status.ManagedExternally = true
			m.cmd = nil
			m.daemonized = false
			m.mu.Unlock()
			return
		}
		if wasRunning && exitCode != 0 {
			log.Println("[ProcessMgr] 检测到 OpenClaw 异常退出，3秒后自动重启...")
			time.Sleep(2 * time.Second)
			if err := m.Start(); err != nil {
				log.Printf("[ProcessMgr] 自动重启失败: %v", err)
			} else {
				log.Println("[ProcessMgr] OpenClaw 已自动重启")
			}
		}
	}
}

// getGatewayPort reads the gateway port from openclaw.json config
func (m *Manager) getGatewayPort() string {
	if gw := m.readGatewayConfig(); gw != nil {
		if port, ok := gw["port"].(float64); ok && port > 0 {
			return fmt.Sprintf("%d", int(port))
		}
	}
	return "18789"
}

func (m *Manager) readGatewayConfig() map[string]interface{} {
	ocDir := m.cfg.OpenClawDir
	if ocDir == "" {
		home, _ := os.UserHomeDir()
		ocDir = filepath.Join(home, ".openclaw")
	}
	if strings.TrimSpace(ocDir) == "" {
		return nil
	}
	cfgPath := filepath.Join(ocDir, "openclaw.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil
	}
	var cfg map[string]interface{}
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	if gw, ok := cfg["gateway"].(map[string]interface{}); ok && gw != nil {
		return gw
	}
	return nil
}

func defaultGatewayLoopbackTargets() []string {
	return []string{"127.0.0.1", "localhost", "::1"}
}

func (m *Manager) getGatewayProbeTargets() (string, []string) {
	port := m.getGatewayPort()
	bind, custom := m.getGatewayBindSettings()
	return port, gatewayConfiguredTargets(bind, custom, collectGatewayCandidateTargets(), m.canBindGatewayHost)
}

func (m *Manager) getGatewayPortCheckTargets() []string {
	bind, custom := m.getGatewayBindSettings()
	return gatewayConfiguredTargets(bind, custom, collectGatewayCandidateTargets(), m.canBindGatewayHost)
}

func (m *Manager) getGatewayBindSettings() (string, string) {
	gw := m.readGatewayConfig()
	if gw == nil {
		return "", ""
	}
	bind, _ := gw["bind"].(string)
	custom, _ := gw["customBindHost"].(string)
	if strings.TrimSpace(custom) == "" {
		if legacy, ok := gw["bindAddress"].(string); ok {
			custom = legacy
		}
	}
	return strings.ToLower(strings.TrimSpace(bind)), custom
}

func gatewayPortCheckTargets(bind, custom string, allTargets []string) []string {
	return gatewayConfiguredTargets(bind, custom, allTargets, func(string) bool { return true })
}

func gatewayConfiguredTargets(bind, custom string, allTargets []string, canBindHost func(host string) bool) []string {
	loopbacks := defaultGatewayLoopbackTargets()
	switch strings.ToLower(strings.TrimSpace(bind)) {
	case "", "auto", "loopback":
		if canBindAnyLoopback(canBindHost) {
			return loopbacks
		}
		return allTargets
	case "tailnet":
		if targets := tailnetGatewayTargets(allTargets); len(targets) > 0 {
			return targets
		}
		if canBindAnyLoopback(canBindHost) {
			return loopbacks
		}
		return allTargets
	case "lan":
		return allTargets
	case "custom":
		custom = normalizeGatewayProbeHost(custom)
		if custom == "localhost" {
			if canBindAnyLoopback(canBindHost) {
				return loopbacks
			}
			return allTargets
		}
		if ip := net.ParseIP(custom); ip != nil && ip.IsLoopback() {
			if canBindHost(custom) {
				return []string{custom}
			}
			return allTargets
		}
		if custom != "" {
			if canBindHost(custom) {
				return []string{custom}
			}
			return allTargets
		}
		return allTargets
	}
	return loopbacks
}

func collectGatewayCandidateTargets() []string {
	targets := defaultGatewayLoopbackTargets()
	ifaces, err := net.Interfaces()
	if err != nil {
		return targets
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				targets = appendGatewayProbeTarget(targets, v.IP.String())
			case *net.IPAddr:
				targets = appendGatewayProbeTarget(targets, v.IP.String())
			}
		}
	}
	return targets
}

func tailnetGatewayTargets(allTargets []string) []string {
	targets := make([]string, 0, len(allTargets))
	for _, host := range allTargets {
		ip := net.ParseIP(host)
		if ip == nil {
			continue
		}
		if tailnetIPv4Net.Contains(ip) || tailnetIPv6Net.Contains(ip) {
			targets = appendGatewayProbeTarget(targets, host)
		}
	}
	return targets
}

func mustCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return network
}

func canBindAnyLoopback(canBindHost func(host string) bool) bool {
	if canBindHost == nil {
		return true
	}
	for _, host := range defaultGatewayLoopbackTargets() {
		if canBindHost(host) {
			return true
		}
	}
	return false
}

func (m *Manager) canBindGatewayHost(host string) bool {
	if m.bindHostCheck != nil {
		return m.bindHostCheck(host)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func appendGatewayProbeTarget(targets []string, host string) []string {
	host = normalizeGatewayProbeHost(host)
	if host == "" {
		return targets
	}
	for _, existing := range targets {
		if existing == host {
			return targets
		}
	}
	return append(targets, host)
}

func normalizeGatewayProbeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		if parsed, err := url.Parse(host); err == nil {
			host = parsed.Hostname()
		}
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			host = parsedHost
		}
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsUnspecified() || ip.IsMulticast() {
			return ""
		}
		return ip.String()
	}
	return host
}

func (m *Manager) isOpenClawGateway(host, port string) bool {
	client := &http.Client{
		Timeout:   1500 * time.Millisecond,
		Transport: &http.Transport{},
	}
	for _, path := range []string{"/healthz", "/health", "/"} {
		u := (&url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort(host, port),
			Path:   path,
		}).String()
		resp, err := client.Get(u)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		if readErr != nil {
			continue
		}
		if looksLikeOpenClawGatewayResponse(path, resp.StatusCode, resp.Header, body) {
			return true
		}
	}
	return false
}

func looksLikeOpenClawGatewayResponse(path string, statusCode int, headers http.Header, body []byte) bool {
	text := strings.ToLower(string(body))
	contentType := strings.ToLower(headers.Get("Content-Type"))
	server := strings.ToLower(headers.Get("Server"))
	location := strings.ToLower(headers.Get("Location"))

	if strings.Contains(server, "openclaw") {
		return true
	}
	if strings.Contains(location, "openclaw") || strings.Contains(location, "control") {
		return true
	}
	if strings.Contains(text, "openclaw control") || strings.Contains(text, "<openclaw-app") {
		return true
	}

	if path == "/health" || path == "/healthz" {
		if statusCode >= 200 && statusCode < 500 {
			if strings.Contains(contentType, "json") && (strings.Contains(text, "\"ok\":true") || strings.Contains(text, "\"status\":\"ok\"") || strings.Contains(text, "\"status\":\"live\"") || strings.Contains(text, "healthy") || strings.Contains(text, "openclaw")) {
				return true
			}
		}
	}

	return false
}

func (m *Manager) waitForGatewayReady(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if m.gatewayListening(true) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// isPortListening checks if a TCP port is currently listening
func (m *Manager) isPortListening(port string) bool {
	hosts := m.getGatewayPortCheckTargets()
	for _, host := range hosts {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 300*time.Millisecond)
		if err != nil {
			continue
		}
		conn.Close()
		return true
	}
	return false
}

func (m *Manager) detectExternalGatewayProcess() (int, bool) {
	port := m.getGatewayPort()
	if port == "" {
		return 0, false
	}
	pids, err := listeningPIDsForPort(port)
	if err != nil {
		return 0, false
	}
	for _, pid := range pids {
		if pid <= 0 {
			continue
		}
		cmdline, err := processCommandLine(pid)
		if err != nil {
			continue
		}
		lower := strings.ToLower(cmdline)
		if strings.Contains(lower, "openclaw") && strings.Contains(lower, "gateway") {
			return pid, true
		}
	}
	return 0, false
}

func listeningPIDsForPort(port string) ([]int, error) {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
		if err != nil {
			return nil, err
		}
		needle := ":" + port
		seen := map[int]struct{}{}
		var pids []int
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !strings.Contains(line, needle) || !strings.Contains(line, "LISTENING") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) < 5 {
				continue
			}
			pid, err := strconv.Atoi(parts[len(parts)-1])
			if err != nil {
				continue
			}
			if _, ok := seen[pid]; ok {
				continue
			}
			seen[pid] = struct{}{}
			pids = append(pids, pid)
		}
		return pids, nil
	}
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+port, "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return nil, err
	}
	seen := map[int]struct{}{}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		pids = append(pids, pid)
	}
	return pids, nil
}

func processCommandLine(pid int) (string, error) {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", fmt.Sprintf("$p=Get-CimInstance Win32_Process -Filter \"ProcessId=%d\" -ErrorAction SilentlyContinue; if($p){ $p.CommandLine }", pid))
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	return strings.TrimSpace(string(out)), err
}

// monitorDaemon monitors the OpenClaw daemon process (fork pattern).
// When the OpenClaw control probe fails repeatedly, mark process as stopped and restart.
func (m *Manager) monitorDaemon() {
	failCount := 0
	for {
		time.Sleep(5 * time.Second)
		m.mu.RLock()
		running := m.status.Running
		daemonized := m.daemonized
		m.mu.RUnlock()
		if !running || !daemonized {
			return // manually stopped
		}
		if m.gatewayListening(true) {
			failCount = 0
			continue
		}
		failCount++
		if failCount >= 3 { // 3 consecutive failures (15s)
			log.Printf("[ProcessMgr] OpenClaw 守护进程已不可达，尝试重启...")
			m.mu.Lock()
			m.status.Running = false
			m.status.Daemonized = false
			m.daemonized = false
			m.lastGatewayProbeAt = time.Time{}
			m.lastGatewayProbeOK = false
			m.mu.Unlock()
			time.Sleep(2 * time.Second)
			if err := m.Start(); err != nil {
				log.Printf("[ProcessMgr] 自动重启失败: %v", err)
			} else {
				log.Println("[ProcessMgr] OpenClaw 已自动重启")
			}
			return
		}
	}
}

// ensureOpenClawConfig 启动前检查并修复 openclaw.json 关键配置
// 始终确保 gateway.mode=local；当 QQ 插件已安装时写入
// channels.qq / plugins.entries.qq / plugins.installs.qq。
func (m *Manager) ensureOpenClawConfig() {
	ocDir := m.cfg.OpenClawDir
	if ocDir == "" {
		home, _ := os.UserHomeDir()
		ocDir = filepath.Join(home, ".openclaw")
	}
	cfgPath := filepath.Join(ocDir, "openclaw.json")

	var cfg map[string]interface{}
	created := false

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		// 配置文件不存在，创建目录并初始化空配置
		os.MkdirAll(filepath.Dir(cfgPath), 0755)
		cfg = map[string]interface{}{}
		created = true
	} else {
		if err := json.Unmarshal(data, &cfg); err != nil {
			cfg = map[string]interface{}{}
			created = true
		}
	}

	changed := created
	if config.NormalizeOpenClawConfig(cfg) {
		changed = true
	}

	// Always ensure gateway.mode = "local" — safe regardless of plugins
	gw, _ := cfg["gateway"].(map[string]interface{})
	if gw == nil {
		gw = map[string]interface{}{}
		cfg["gateway"] = gw
	}
	if gw["mode"] != "local" {
		gw["mode"] = "local"
		changed = true
	}

	// Remove keys rejected by some OpenClaw gateway versions.
	if _, ok := cfg["meta"]; ok {
		delete(cfg, "meta")
		changed = true
	}
	if _, ok := cfg["workspace"]; ok {
		delete(cfg, "workspace")
		changed = true
	}

	qqExtDir := filepath.Join(ocDir, "extensions", "qq")
	qqExtInstalled := false
	if _, err := os.Stat(qqExtDir); err == nil {
		qqExtInstalled = true
	}
	qqShouldManage, qqManagedByNapCat := m.shouldManageQQIntegration(cfg, qqExtDir)
	qqInstallPath := ""
	if pl, ok := cfg["plugins"].(map[string]interface{}); ok {
		if ins, ok := pl["installs"].(map[string]interface{}); ok {
			if qqIns, ok := ins["qq"].(map[string]interface{}); ok {
				if p, ok := qqIns["installPath"].(string); ok {
					qqInstallPath = p
				}
			}
		}
	}

	if qqExtInstalled && qqShouldManage {
		// Ensure channels.qq with wsUrl
		ch, _ := cfg["channels"].(map[string]interface{})
		if ch == nil {
			ch = map[string]interface{}{}
			cfg["channels"] = ch
		}
		qq, _ := ch["qq"].(map[string]interface{})
		if qq == nil {
			qq = map[string]interface{}{}
			ch["qq"] = qq
		}
		if qq["wsUrl"] == nil || qq["wsUrl"] == "" {
			qq["wsUrl"] = "ws://127.0.0.1:3001"
			changed = true
		}
		if qqManagedByNapCat && qq["enabled"] == nil {
			qq["enabled"] = true
			changed = true
		}

		// Ensure plugins.entries.qq
		pl, _ := cfg["plugins"].(map[string]interface{})
		if pl == nil {
			pl = map[string]interface{}{}
			cfg["plugins"] = pl
		}
		ent, _ := pl["entries"].(map[string]interface{})
		if ent == nil {
			ent = map[string]interface{}{}
			pl["entries"] = ent
		}
		if ent["qq"] == nil {
			ent["qq"] = map[string]interface{}{"enabled": qqManagedByNapCat}
			changed = true
		} else if entry, ok := ent["qq"].(map[string]interface{}); ok && entry != nil {
			if qqManagedByNapCat {
				if _, ok := entry["enabled"]; !ok {
					entry["enabled"] = true
					changed = true
				}
			}
		}

		// Ensure plugins.installs.qq
		ins, _ := pl["installs"].(map[string]interface{})
		if ins == nil {
			ins = map[string]interface{}{}
			pl["installs"] = ins
		}
		qqIns, _ := ins["qq"].(map[string]interface{})
		if qqIns == nil {
			qqIns = map[string]interface{}{}
			ins["qq"] = qqIns
		}
		if p, _ := qqIns["installPath"].(string); p == "" {
			qqIns["installPath"] = qqExtDir
			changed = true
		} else if _, err := os.Stat(p); err != nil {
			qqIns["installPath"] = qqExtDir
			changed = true
		}
		source, _ := qqIns["source"].(string)
		if source != "npm" && source != "archive" && source != "path" {
			qqIns["source"] = "path"
			changed = true
		}
		if qqIns["version"] == nil || qqIns["version"] == "" {
			qqIns["version"] = "latest"
			changed = true
		}
	}

	if !qqShouldManage {
		if ch, ok := cfg["channels"].(map[string]interface{}); ok && ch != nil {
			if _, exists := ch["qq"]; exists {
				delete(ch, "qq")
				changed = true
			}
		}
		if pl, ok := cfg["plugins"].(map[string]interface{}); ok && pl != nil {
			if ent, ok := pl["entries"].(map[string]interface{}); ok && ent != nil {
				if _, exists := ent["qq"]; exists {
					delete(ent, "qq")
					changed = true
				}
			}
			if ins, ok := pl["installs"].(map[string]interface{}); ok && ins != nil {
				if _, exists := ins["qq"]; exists {
					delete(ins, "qq")
					changed = true
				}
			}
		}
	}

	if changed {
		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			log.Printf("[ProcessMgr] openclaw.json 序列化失败: %v", err)
		} else if err := os.WriteFile(cfgPath, out, 0644); err != nil {
			log.Printf("[ProcessMgr] openclaw.json 写入失败: %v", err)
		} else {
			log.Println("[ProcessMgr] openclaw.json 配置已自动修复 (gateway.mode/channels.qq/plugins)")
		}
	}

	// Patch QQ plugin channel implementation: startAccount must return
	// a long-lived Promise (covers both src/channel.ts and dist/channel.js,
	// and both config-local/extensions and npm-global install paths).
	if qqExtInstalled || strings.TrimSpace(qqInstallPath) != "" {
		m.patchQQPluginChannel(ocDir, qqInstallPath)
	}
	m.patchWecomPluginChannel(ocDir)
	m.patchFeishuPluginChannel(ocDir)
}

func (m *Manager) shouldManageQQIntegration(ocConfig map[string]interface{}, qqExtDir string) (bool, bool) {
	pluginInstalled := m.isPanelPluginInstalled("qq")
	napcatInstalled := m.hasManagedNapCatInstall()
	setupMarked := m.qqChannelSetupMarked()
	if strings.TrimSpace(qqExtDir) != "" {
		if _, err := os.Stat(qqExtDir); err != nil {
			return false, napcatInstalled
		}
	}
	hasExistingQQConfig := hasExistingQQIntegrationConfig(ocConfig)
	return shouldManageQQIntegrationState(pluginInstalled, napcatInstalled, setupMarked, hasExistingQQConfig)
}

func hasExistingQQIntegrationConfig(ocConfig map[string]interface{}) bool {
	if ocConfig == nil {
		return false
	}
	if channels, ok := ocConfig["channels"].(map[string]interface{}); ok && channels != nil {
		if _, ok := channels["qq"].(map[string]interface{}); ok {
			return true
		}
	}
	if pl, ok := ocConfig["plugins"].(map[string]interface{}); ok && pl != nil {
		if ent, ok := pl["entries"].(map[string]interface{}); ok && ent != nil {
			if _, ok := ent["qq"].(map[string]interface{}); ok {
				return true
			}
		}
		if ins, ok := pl["installs"].(map[string]interface{}); ok && ins != nil {
			if _, ok := ins["qq"].(map[string]interface{}); ok {
				return true
			}
		}
	}
	return false
}

func shouldManageQQIntegrationState(pluginInstalled, napcatInstalled, setupMarked, hasExistingQQConfig bool) (bool, bool) {
	if !setupMarked && !hasExistingQQConfig {
		return false, false
	}
	return pluginInstalled || napcatInstalled || hasExistingQQConfig, napcatInstalled
}

func (m *Manager) qqChannelSetupMarked() bool {
	if m.cfg == nil || strings.TrimSpace(m.cfg.DataDir) == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(m.cfg.DataDir, "qq-channel-enabled.flag"))
	return err == nil
}

func (m *Manager) isPanelPluginInstalled(pluginID string) bool {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" || m.cfg == nil || strings.TrimSpace(m.cfg.DataDir) == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(m.cfg.DataDir, "plugins.json"))
	if err != nil || len(data) == 0 {
		return false
	}
	var state map[string]map[string]interface{}
	if json.Unmarshal(data, &state) != nil {
		return false
	}
	item, ok := state[pluginID]
	if !ok || item == nil {
		return false
	}
	if dir, _ := item["dir"].(string); strings.TrimSpace(dir) != "" {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}
	return false
}

func (m *Manager) hasManagedNapCatInstall() bool {
	if m.isNapCatRunning() {
		return true
	}
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(home, "NapCat"),
			filepath.Join(home, "Desktop", "NapCat"),
			`C:\NapCat`,
			filepath.Join(home, "AppData", "Local", "NapCat"),
		}
		for _, dir := range candidates {
			if strings.TrimSpace(dir) == "" {
				continue
			}
			if _, err := os.Stat(dir); err == nil {
				return true
			}
		}
		return false
	}
	if out, err := runDockerOutput("inspect", "openclaw-qq"); err == nil && len(out) > 0 {
		return true
	}
	return false
}

// patchQQPluginChannelTS fixes the critical bug where the QQ plugin's startAccount
// returns a cleanup function instead of a long-lived Promise. OpenClaw gateway
// wraps startAccount's return value with Promise.resolve(task); if it resolves
// immediately (non-Promise return), the framework treats the account as exited
// and triggers auto-restart attempts (up to 10), after which the channel handler
// dies and incoming messages are never processed.
func (m *Manager) patchQQPluginChannel(ocDir, installPath string) {
	paths := []string{}
	if ocDir != "" {
		paths = append(paths,
			filepath.Join(ocDir, "extensions", "qq", "src", "channel.ts"),
			filepath.Join(ocDir, "extensions", "qq", "dist", "channel.js"),
		)
	}
	if installPath != "" {
		paths = append(paths,
			filepath.Join(installPath, "src", "channel.ts"),
			filepath.Join(installPath, "dist", "channel.js"),
		)
	}
	if m.cfg != nil && m.cfg.OpenClawApp != "" {
		paths = append(paths,
			filepath.Join(m.cfg.OpenClawApp, "extensions", "qq", "src", "channel.ts"),
			filepath.Join(m.cfg.OpenClawApp, "extensions", "qq", "dist", "channel.js"),
		)
	}

	oldPattern := regexp.MustCompile(`(?s)return\s*\(\)\s*=>\s*\{\s*client\.disconnect\(\);\s*clients\.delete\(account\.accountId\);\s*stopFileServer\(\);\s*\};`)
	newReturn := `return new Promise((resolve) => {
        const cleanup = () => {
          client.disconnect();
          clients.delete(account.accountId);
          stopFileServer();
          resolve();
        };
        const abortSignal = (ctx && ctx.abortSignal) ? ctx.abortSignal : undefined;
        if (abortSignal) {
          if (abortSignal.aborted) { cleanup(); return; }
          abortSignal.addEventListener("abort", cleanup, { once: true });
        }
        client.on("close", () => {
          cleanup();
        });
      });`

	loggerPattern := regexp.MustCompile(`(?s)function\s+postLogEntry\s*\([^)]*\)\s*\{.*?\n\}`)
	managerURLPattern := regexp.MustCompile(`const\s+MANAGER_LOG_URL\s*=\s*"[^"]*";`)
	workflowInterceptPattern := regexp.MustCompile(`const\s+WORKFLOW_INTERCEPT_URL\s*=\s*"[^"]*";`)
	workflowInsertPattern := regexp.MustCompile(`const\s+fromId\s*=\s+isGroup\s*\?\s*"group:"\s*\+\s*groupId\s*:\s*String\(userId\);`)
	managerPort := 19527
	if m.cfg != nil && m.cfg.Port > 0 {
		managerPort = m.cfg.Port
	}
	managerURLLine := fmt.Sprintf(`const MANAGER_LOG_URL = "http://127.0.0.1:%d/api/events/log";`, managerPort)
	workflowURLLine := fmt.Sprintf("const WORKFLOW_INTERCEPT_URL = \"http://127.0.0.1:%d/api/workflows/intercept\";\nconst WORKFLOW_TOKEN = %q;", managerPort, strings.TrimSpace(m.cfg.AdminToken))
	loggerReplacement := `function postLogEntry(summary, detail, source) {
  try {
    const payload = {
      source: source || "openclaw",
      type: "openclaw.reply",
      summary,
      detail,
    };
    const f = (globalThis && globalThis.fetch) ? globalThis.fetch.bind(globalThis) : null;
    if (f) {
      f(MANAGER_LOG_URL, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      }).catch(() => {});
    }
  } catch {}
}

async function shouldInterceptWorkflow(payload) {
  try {
    const f = (globalThis && globalThis.fetch) ? globalThis.fetch.bind(globalThis) : null;
    if (!f) return false;
    const resp = await f(WORKFLOW_INTERCEPT_URL, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Workflow-Token": WORKFLOW_TOKEN,
      },
      body: JSON.stringify(payload),
    });
    if (!resp || !resp.ok) return false;
    const data = await resp.json().catch(() => null);
    return !!(data && data.handled);
  } catch {
    return false;
  }
}`
	workflowInterceptBlock := `const workflowConversationId = isGroup ? "qq:group:" + groupId : "qq:private:" + userId;
        if (await shouldInterceptWorkflow({
          channelId: "qq",
          conversationId: workflowConversationId,
          userId: String(userId || ""),
          text,
        })) {
          console.log("[QQ] Workflow intercepted message for " + workflowConversationId);
          processingMessages.delete(dedupKey);
          return;
        }

        const fromId = isGroup ? "group:" + groupId : String(userId);`

	patchedAny := false
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := string(data)
		patched := content
		fileChanged := false

		if !strings.Contains(patched, "return new Promise") && oldPattern.MatchString(patched) {
			patched = oldPattern.ReplaceAllString(patched, newReturn)
			fileChanged = true
		}

		if managerURLPattern.MatchString(patched) {
			next := managerURLPattern.ReplaceAllString(patched, managerURLLine)
			if next != patched {
				patched = next
				fileChanged = true
			}
		}

		if !workflowInterceptPattern.MatchString(patched) && strings.Contains(patched, managerURLLine) {
			next := strings.Replace(patched, managerURLLine, managerURLLine+"\n"+workflowURLLine, 1)
			if next != patched {
				patched = next
				fileChanged = true
			}
		}

		if loggerPattern.MatchString(patched) && !strings.Contains(patched, "async function shouldInterceptWorkflow") {
			patched = loggerPattern.ReplaceAllString(patched, loggerReplacement)
			fileChanged = true
		}

		if !strings.Contains(patched, "Workflow intercepted message") && workflowInsertPattern.MatchString(patched) {
			next := workflowInsertPattern.ReplaceAllString(patched, workflowInterceptBlock)
			if next != patched {
				patched = next
				fileChanged = true
			}
		}

		if !fileChanged {
			continue
		}

		if err := os.WriteFile(p, []byte(patched), 0644); err != nil {
			log.Printf("[ProcessMgr] QQ channel 补丁写入失败 (%s): %v", p, err)
			continue
		}
		patchedAny = true
		log.Printf("[ProcessMgr] ✅ QQ channel 兼容补丁已应用: %s", p)
	}

	if !patchedAny {
		log.Println("[ProcessMgr] QQ channel 兼容补丁未命中（可能已修复或版本结构不同）")
	}
}

func (m *Manager) patchWecomPluginChannel(ocDir string) {
	paths := []string{}
	if ocDir != "" {
		paths = append(paths, filepath.Join(ocDir, "extensions", "wecom", "dist", "index.js"))
	}
	if m.cfg != nil && m.cfg.OpenClawApp != "" {
		paths = append(paths, filepath.Join(m.cfg.OpenClawApp, "extensions", "wecom", "dist", "index.js"))
	}
	managerPort := 19527
	if m.cfg != nil && m.cfg.Port > 0 {
		managerPort = m.cfg.Port
	}
	inboundLogSnippet := fmt.Sprintf("try {\n      const f = globalThis && globalThis.fetch ? globalThis.fetch.bind(globalThis) : null;\n      if (f && rawBody && rawBody.trim()) {\n        f(\"http://127.0.0.1:%d/api/events/log\", {\n          method: \"POST\",\n          headers: { \"Content-Type\": \"application/json\" },\n          body: JSON.stringify({\n            source: \"wecom\",\n            type: \"wecom.message.received\",\n            summary: \"[企微] \" + fromLabel + \": \" + rawBody,\n            detail: \"to=\" + to + \"\\nmsgid=\" + (msg.msgid || \"\")\n          })\n        }).catch(() => {});\n      }\n    } catch {}", managerPort)
	replyLogSnippet := fmt.Sprintf("if (normalized.trim()) {\n            try {\n              const f = globalThis && globalThis.fetch ? globalThis.fetch.bind(globalThis) : null;\n              if (f) {\n                f(\"http://127.0.0.1:%d/api/events/log\", {\n                  method: \"POST\",\n                  headers: { \"Content-Type\": \"application/json\" },\n                  body: JSON.stringify({\n                    source: \"openclaw\",\n                    type: \"openclaw.reply\",\n                    summary: \"[企微回复] \" + normalized,\n                    detail: \"channel=wecom\\nto=\" + to\n                  })\n                }).catch(() => {});\n              }\n            } catch {}\n          }\n          await hooks.onChunk(normalized);", managerPort)
	interceptBlock := fmt.Sprintf(`const workflowInterceptResp = await (async () => {
      try {
        const f = globalThis && globalThis.fetch ? globalThis.fetch.bind(globalThis) : null;
        if (!f) return null;
        const resp = await f("http://127.0.0.1:%d/api/workflows/intercept", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Workflow-Token": %q
          },
          body: JSON.stringify({
            channelId: "wecom",
            conversationId: from,
            userId: senderId,
            text: rawBody,
            context: {
              responseUrl,
              wecomTo: to,
              sessionKey: route.sessionKey
            }
          })
        });
        if (!resp || !resp.ok) return null;
        return await resp.json().catch(() => null);
      } catch {
        return null;
      }
    })();
    if (workflowInterceptResp && workflowInterceptResp.handled) {
      if (workflowInterceptResp.reply) {
        await hooks.onChunk(String(workflowInterceptResp.reply));
      }
      return;
    }
    await channel.reply.dispatchReplyWithBufferedBlockDispatcher({`, managerPort, strings.TrimSpace(m.cfg.AdminToken))
	patchedAny := false
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := string(data)
		next := content
		changed := false
		if !strings.Contains(next, "/api/workflows/intercept") {
			replaced := strings.Replace(next, `await channel.reply.dispatchReplyWithBufferedBlockDispatcher({`, interceptBlock, 1)
			if replaced != next {
				next = replaced
				changed = true
			}
		}
		if !strings.Contains(next, "wecom.message.received") {
			replaced := strings.Replace(next, `const fromLabel = chatType === "group" ? `+"`group:${chatId}`"+` : `+"`user:${senderId}`"+`;`, `const fromLabel = chatType === "group" ? `+"`group:${chatId}`"+` : `+"`user:${senderId}`"+`;
    `+inboundLogSnippet, 1)
			if replaced != next {
				next = replaced
				changed = true
			}
		}
		if !strings.Contains(next, "[企微回复]") {
			replaced := strings.Replace(next, `await hooks.onChunk(normalized);`, replyLogSnippet, 1)
			if replaced != next {
				next = replaced
				changed = true
			}
		}
		if strings.Contains(next, `text: rawBody`) && !strings.Contains(next, `responseUrl`) {
			replaced := strings.Replace(next, `text: rawBody`, `text: rawBody,
            context: {
              responseUrl,
              wecomTo: to,
              sessionKey: route.sessionKey
            }`, 1)
			if replaced != next {
				next = replaced
				changed = true
			}
		}
		if !changed {
			continue
		}
		if err := os.WriteFile(p, []byte(next), 0644); err != nil {
			log.Printf("[ProcessMgr] WeCom 插件补丁写入失败 (%s): %v", p, err)
			continue
		}
		patchedAny = true
		log.Printf("[ProcessMgr] ✅ WeCom 工作流拦截补丁已应用: %s", p)
	}
	if !patchedAny {
		log.Println("[ProcessMgr] WeCom 工作流拦截补丁未命中（可能已应用或文件结构不同）")
	}
}

func (m *Manager) patchFeishuPluginChannel(ocDir string) {
	paths := []string{}
	if ocDir != "" {
		paths = append(paths, filepath.Join(ocDir, "extensions", "feishu", "src", "channel.ts"))
	}
	if m.cfg != nil && m.cfg.OpenClawApp != "" {
		paths = append(paths, filepath.Join(m.cfg.OpenClawApp, "extensions", "feishu", "src", "channel.ts"))
	}
	managerPort := 19527
	if m.cfg != nil && m.cfg.Port > 0 {
		managerPort = m.cfg.Port
	}
	managerURLLine := fmt.Sprintf("const MANAGER_LOG_URL = \"http://127.0.0.1:%d/api/events/log\";", managerPort)
	logHelper := `async function postLogEntry(source: string, type: string, summary: string, detail = "") {
  try {
    const f = globalThis && globalThis.fetch ? globalThis.fetch.bind(globalThis) : null;
    if (!f || !summary) return;
    await f(MANAGER_LOG_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ source, type, summary, detail }),
    }).catch(() => {});
  } catch {}
}`
	interceptBlock := fmt.Sprintf(`const workflowInterceptResp = await (async () => {
            try {
              const f = globalThis && globalThis.fetch ? globalThis.fetch.bind(globalThis) : null;
              if (!f) return null;
              const resp = await f("http://127.0.0.1:%d/api/workflows/intercept", {
                method: "POST",
                headers: {
                  "Content-Type": "application/json",
                  "X-Workflow-Token": %q,
                },
                body: JSON.stringify({
                  channelId: "feishu",
                  conversationId: msgCtx.From,
                  userId: message.senderId,
                  text: message.text,
                }),
              });
              if (!resp || !resp.ok) return null;
              return await resp.json().catch(() => null);
            } catch {
              return null;
            }
          })();

          if (workflowInterceptResp?.handled) {
            if (workflowInterceptResp.reply) {
              await sendTextMessage(account, message.chatId, String(workflowInterceptResp.reply));
            }
            return;
          }

          await runtime.channel.reply.dispatchReplyWithBufferedBlockDispatcher({`, managerPort, strings.TrimSpace(m.cfg.AdminToken))
	patchedAny := false
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := string(data)
		next := content
		changed := false
		if !strings.Contains(next, "MANAGER_LOG_URL") && strings.Contains(next, `const CHANNEL_ID = "feishu" as const;`) {
			replaced := strings.Replace(next, `const CHANNEL_ID = "feishu" as const;`, "const CHANNEL_ID = \"feishu\" as const;\n"+managerURLLine+"\n"+logHelper, 1)
			if replaced != next {
				next = replaced
				changed = true
			}
		}
		if !strings.Contains(next, "/api/workflows/intercept") {
			replaced := strings.Replace(next, `await runtime.channel.reply.dispatchReplyWithBufferedBlockDispatcher({`, interceptBlock, 1)
			if replaced != next {
				next = replaced
				changed = true
			}
		}
		if strings.Contains(next, `conversationId: msgCtx.From,`) {
			replaced := strings.ReplaceAll(next, `conversationId: msgCtx.From,`, `conversationId: message.chatId,`)
			if replaced != next {
				next = replaced
				changed = true
			}
		}
		if !strings.Contains(next, "feishu.message.received") {
			replaced := strings.Replace(next, "console.log(`[feishu:${account.accountId}] 收到消息: ${message.text}`);", "console.log(`[feishu:${account.accountId}] 收到消息: ${message.text}`);\n          await postLogEntry(\"feishu\", \"feishu.message.received\", \"[飞书] \" + message.senderId + \": \" + message.text, \"chatId=\" + message.chatId + \"\\nchatType=\" + message.chatType);", 1)
			if replaced != next {
				next = replaced
				changed = true
			}
		}
		if !strings.Contains(next, "[飞书回复]") {
			replaced := strings.Replace(next, "await sendTextMessage(account, message.chatId, String(workflowInterceptResp.reply));", "await sendTextMessage(account, message.chatId, String(workflowInterceptResp.reply));\n              await postLogEntry(\"openclaw\", \"openclaw.reply\", \"[飞书回复] \" + String(workflowInterceptResp.reply), \"channel=feishu\\nchatId=\" + message.chatId);", 1)
			if replaced != next {
				next = replaced
				changed = true
			}
			replaced = strings.Replace(next, "await sendTextMessage(account, message.chatId, text);", "await sendTextMessage(account, message.chatId, text);\n                    await postLogEntry(\"openclaw\", \"openclaw.reply\", \"[飞书回复] \" + text, \"channel=feishu\\nchatId=\" + message.chatId);", 1)
			if replaced != next {
				next = replaced
				changed = true
			}
		}
		if !changed {
			continue
		}
		if err := os.WriteFile(p, []byte(next), 0644); err != nil {
			log.Printf("[ProcessMgr] Feishu 插件补丁写入失败 (%s): %v", p, err)
			continue
		}
		patchedAny = true
		log.Printf("[ProcessMgr] ✅ Feishu 工作流拦截补丁已应用: %s", p)
	}
	if !patchedAny {
		log.Println("[ProcessMgr] Feishu 工作流拦截补丁未命中（可能已应用或文件结构不同）")
	}
}

// findOpenClawBin 查找 openclaw 可执行文件
func (m *Manager) findOpenClawBin() string {
	if p := config.DetectOpenClawBinaryPath(); p != "" {
		return p
	}

	candidates := []string{
		"openclaw",
	}

	// 添加常见路径
	home, _ := os.UserHomeDir()
	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".local", "bin", "openclaw"),
			filepath.Join(home, "openclaw", "app", "openclaw"),
		)
	}

	switch runtime.GOOS {
	case "linux":
		candidates = append(candidates,
			"/usr/local/bin/openclaw",
			"/usr/bin/openclaw",
			"/snap/bin/openclaw",
		)
	case "darwin":
		candidates = append(candidates,
			"/usr/local/bin/openclaw",
			"/opt/homebrew/bin/openclaw",
		)
	case "windows":
		candidates = append(candidates,
			`C:\Program Files\openclaw\openclaw.exe`,
			`C:\ClawPanel\npm-global\openclaw.cmd`,
			`C:\ClawPanel\npm-global\node_modules\.bin\openclaw.cmd`,
			filepath.Join(home, "AppData", "Roaming", "npm", "openclaw.cmd"),
		)
	}

	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// isNapCatRunning returns true if NapCat is currently running.
// On Linux it checks for the "openclaw-qq" Docker container;
// on Windows it checks for NapCat shell processes.
func (m *Manager) isNapCatRunning() bool {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq NapCatWinBootMain.exe", "/NH").Output()
		if err == nil && strings.Contains(string(out), "NapCatWinBootMain") {
			return true
		}
		out2, err2 := exec.Command("tasklist", "/FI", "IMAGENAME eq napcat.exe", "/NH").Output()
		return err2 == nil && strings.Contains(string(out2), "napcat.exe")
	}
	// Linux/macOS: check Docker container state with robust env/path handling
	out, err := runDockerOutput("inspect", "--format", "{{.State.Running}}", "openclaw-qq")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func runDockerOutput(args ...string) ([]byte, error) {
	bins := []string{"docker", "/usr/local/bin/docker", "/opt/homebrew/bin/docker"}
	for _, bin := range bins {
		cmd := exec.Command(bin, args...)
		cmd.Env = buildProcessEnv()
		if out, err := cmd.Output(); err == nil {
			return out, nil
		}
		if runtime.GOOS == "darwin" {
			for _, archFlag := range []string{"-arm64", "-x86_64"} {
				altArgs := append([]string{archFlag, bin}, args...)
				alt := exec.Command("arch", altArgs...)
				alt.Env = buildProcessEnv()
				if out, err := alt.Output(); err == nil {
					return out, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("docker command unavailable")
}
