package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
	"github.com/zhaoxinyi02/ClawPanel/internal/update"
	updaterPkg "github.com/zhaoxinyi02/ClawPanel/internal/updater"
)

// GetVersion 获取 OpenClaw 版本信息
func GetVersion(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentVersion := resolveOpenClawCurrentVersion(cfg)

		var updateInfo map[string]interface{}
		updateCheckPath := filepath.Join(cfg.OpenClawDir, "update-check.json")
		if data, err := os.ReadFile(updateCheckPath); err == nil {
			json.Unmarshal(data, &updateInfo)
		}

		latestVersion := ""
		lastCheckedAt := ""
		updateAvailable := false
		if updateInfo != nil {
			latestVersion, _ = updateInfo["lastNotifiedVersion"].(string)
			latestVersion = normalizeVersion(latestVersion)
			lastCheckedAt, _ = updateInfo["lastCheckedAt"].(string)
			// Re-evaluate against live currentVersion (not the cached one)
			if latestVersion != "" && currentVersion != "unknown" && latestVersion != currentVersion {
				updateAvailable = true
			}
		}
		if latestVersion == "" && currentVersion != "unknown" {
			latestVersion = currentVersion
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":              true,
			"currentVersion":  currentVersion,
			"latestVersion":   latestVersion,
			"lastCheckedAt":   lastCheckedAt,
			"updateAvailable": updateAvailable,
		})
	}
}

// GetPanelVersion 获取 ClawPanel 面板版本
func GetPanelVersion(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"version": version,
		})
	}
}

// Backup 备份配置
func Backup(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		timestamp := time.Now().Format("2006-01-02T15-04-05")
		backupDir := filepath.Join(cfg.OpenClawDir, "backups")
		os.MkdirAll(backupDir, 0755)

		configSrc := filepath.Join(cfg.OpenClawDir, "openclaw.json")
		if _, err := os.Stat(configSrc); err == nil {
			data, _ := os.ReadFile(configSrc)
			os.WriteFile(filepath.Join(backupDir, fmt.Sprintf("openclaw-%s.json", timestamp)), data, 0644)
		}

		cronSrc := filepath.Join(cfg.OpenClawDir, "cron", "jobs.json")
		if _, err := os.Stat(cronSrc); err == nil {
			data, _ := os.ReadFile(cronSrc)
			os.WriteFile(filepath.Join(backupDir, fmt.Sprintf("cron-jobs-%s.json", timestamp)), data, 0644)
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "backupId": timestamp})
	}
}

// ListBackups 列出备份
func ListBackups(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		backupDir := filepath.Join(cfg.OpenClawDir, "backups")
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "backups": []interface{}{}})
			return
		}

		type backupInfo struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Size int64  `json:"size"`
			Time string `json:"time"`
		}

		var backups []backupInfo
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			info, _ := e.Info()
			backups = append(backups, backupInfo{
				Name: e.Name(),
				Path: filepath.Join(backupDir, e.Name()),
				Size: info.Size(),
				Time: info.ModTime().Format(time.RFC3339),
			})
		}
		sort.Slice(backups, func(i, j int) bool { return backups[i].Name > backups[j].Name })

		if backups == nil {
			backups = []backupInfo{}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "backups": backups})
	}
}

// Restore 恢复备份
func Restore(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BackupName string `json:"backupName"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.BackupName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "backupName required"})
			return
		}

		backupDir := filepath.Join(cfg.OpenClawDir, "backups")
		backupPath := filepath.Join(backupDir, req.BackupName)
		if _, err := os.Stat(backupPath); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "备份文件不存在"})
			return
		}

		// 恢复前自动备份当前配置
		timestamp := time.Now().Format("2006-01-02T15-04-05")
		configPath := filepath.Join(cfg.OpenClawDir, "openclaw.json")
		if data, err := os.ReadFile(configPath); err == nil {
			os.WriteFile(filepath.Join(backupDir, fmt.Sprintf("pre-restore-%s.json", timestamp)), data, 0644)
		}

		data, err := os.ReadFile(backupPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		if strings.HasPrefix(req.BackupName, "openclaw-") || strings.HasPrefix(req.BackupName, "pre-restore-") {
			os.WriteFile(configPath, data, 0644)
		} else if strings.HasPrefix(req.BackupName, "cron-jobs-") {
			cronPath := filepath.Join(cfg.OpenClawDir, "cron", "jobs.json")
			os.WriteFile(cronPath, data, 0644)
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// RestartGateway 重启 OpenClaw 网关
func RestartGateway(cfg *config.Config, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		patchModelsJSON(cfg)
		status := procMgr.GetStatus()

		if status.ManagedExternally || status.Daemonized {
			if err := restartGatewayViaCLI(cfg, procMgr); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "通过 CLI 重启网关失败: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已通过 CLI 发起网关重启"})
			return
		}

		// If OpenClaw process is not running, start it
		if !status.Running {
			if err := restartGatewayViaCLI(cfg, procMgr); err == nil {
				c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已通过 CLI 拉起网关"})
				return
			}
			if err := procMgr.Start(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "启动 OpenClaw 失败: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "message": "OpenClaw 已启动"})
			return
		}

		if err := procMgr.Restart(); err != nil {
			if cliErr := restartGatewayViaCLI(cfg, procMgr); cliErr == nil {
				c.JSON(http.StatusOK, gin.H{"ok": true, "message": "进程内重启失败，已回退到 CLI 发起网关重启"})
				return
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "重启 OpenClaw 网关失败: " + err.Error() + " | CLI 回退失败: " + cliErr.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "OpenClaw 网关已重启"})
	}
}

func restartGatewayViaCLI(cfg *config.Config, procMgr *process.Manager) error {
	bins := candidateOpenClawBins(cfg)
	errMsgs := make([]string, 0, len(bins))
	for _, bin := range bins {
		if err := restartGatewayWithBinary(cfg, procMgr, bin); err == nil {
			return nil
		} else {
			errMsgs = append(errMsgs, err.Error())
		}
	}
	if len(errMsgs) > 0 {
		return fmt.Errorf("%s", strings.Join(errMsgs, " | "))
	}
	return fmt.Errorf("未找到可用的 openclaw 启动命令")
}

func candidateOpenClawBins(cfg *config.Config) []string {
	bins := []string{}
	if p := config.DetectOpenClawBinaryPath(); p != "" {
		bins = append(bins, p)
	}
	if cfg != nil {
		if app := strings.TrimSpace(cfg.OpenClawApp); app != "" {
			if runtime.GOOS == "windows" {
				bins = append(bins,
					filepath.Join(filepath.Dir(app), "npm-global", "openclaw.cmd"),
					filepath.Join(filepath.Dir(app), "npm-global", "node_modules", ".bin", "openclaw.cmd"),
				)
			} else {
				bins = append(bins,
					filepath.Join(filepath.Dir(app), ".npm-global", "bin", "openclaw"),
				)
			}
		}
		if runtime.GOOS == "windows" {
			bins = append(bins,
				filepath.Join(filepath.Dir(cfg.OpenClawDir), "npm-global", "openclaw.cmd"),
				filepath.Join(filepath.Dir(cfg.OpenClawDir), "npm-global", "node_modules", ".bin", "openclaw.cmd"),
			)
		}
	}
	bins = append(bins, "openclaw")

	seen := map[string]struct{}{}
	uniq := make([]string, 0, len(bins))
	for _, bin := range bins {
		bin = strings.TrimSpace(bin)
		if bin == "" {
			continue
		}
		if _, ok := seen[bin]; ok {
			continue
		}
		seen[bin] = struct{}{}
		uniq = append(uniq, bin)
	}
	return uniq
}

func restartGatewayWithBinary(cfg *config.Config, procMgr *process.Manager, bin string) error {
	env := append(config.BuildExecEnv(),
		fmt.Sprintf("OPENCLAW_DIR=%s", cfg.OpenClawDir),
		fmt.Sprintf("OPENCLAW_STATE_DIR=%s", cfg.OpenClawDir),
		fmt.Sprintf("OPENCLAW_CONFIG_PATH=%s", filepath.Join(cfg.OpenClawDir, "openclaw.json")),
	)

	if restartErr, restartOut := runGatewayCommand(cfg, env, bin, "gateway", "restart"); restartErr == nil {
		if procMgr == nil || waitGatewayState(procMgr, true, gatewayStartWaitTimeout()) {
			return nil
		}
		_ = restartOut
	} else if procMgr != nil && waitGatewayState(procMgr, true, 3*time.Second) {
		return nil
	}

	stopErr, stopOut := runGatewayCommand(cfg, env, bin, "gateway", "stop")
	if stopErr != nil {
		_ = stopOut
	}

	if procMgr != nil {
		_ = waitGatewayState(procMgr, false, 8*time.Second)
	}

	startVariants := [][]string{{"gateway"}, {"gateway", "start"}}
	startErrs := make([]string, 0, len(startVariants))
	for _, args := range startVariants {
		cmd := exec.Command(bin, args...)
		cmd.Dir = cfg.OpenClawDir
		cmd.Env = env
		if err := cmd.Start(); err != nil {
			startErrs = append(startErrs, fmt.Sprintf("%s: %v", strings.Join(args, " "), err))
			continue
		}
		go func(c *exec.Cmd) {
			_, _ = c.Process.Wait()
		}(cmd)

		if procMgr == nil {
			return nil
		}
		if waitGatewayState(procMgr, true, gatewayStartWaitTimeout()) {
			return nil
		}
		startErrs = append(startErrs, fmt.Sprintf("%s: 网关未在超时时间内就绪", strings.Join(args, " ")))
	}

	msg := fmt.Sprintf("%s 重启失败", bin)
	if stopErr != nil {
		msg += "; stop: " + strings.TrimSpace(stopOut)
	}
	if len(startErrs) > 0 {
		msg += "; start: " + strings.Join(startErrs, " | ")
	}
	return fmt.Errorf("%s", msg)
}

func runGatewayCommand(cfg *config.Config, env []string, bin string, args ...string) (error, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = cfg.OpenClawDir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("命令超时"), string(out)
	}
	return err, string(out)
}

func waitGatewayState(procMgr *process.Manager, expected bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if gatewayStateMatches(procMgr, expected) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return gatewayStateMatches(procMgr, expected)
}

func gatewayStateMatches(procMgr *process.Manager, expected bool) bool {
	if procMgr == nil {
		return false
	}
	status := procMgr.GetStatus()
	if expected {
		return status.Running || procMgr.GatewayListening()
	}
	return !status.Running && !procMgr.GatewayListening()
}

func gatewayStartWaitTimeout() time.Duration {
	if runtime.GOOS == "windows" {
		return 30 * time.Second
	}
	return 15 * time.Second
}

// RestartPanel 重启 ClawPanel 自身 (通过 systemctl)
func RestartPanel() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "ClawPanel 即将重启"})
		// Delay restart so the response can be sent
		go func() {
			time.Sleep(500 * time.Millisecond)
			exec.Command("systemctl", "restart", "clawpanel").Run()
			// Fallback: exit and let systemd restart us
			os.Exit(0)
		}()
	}
}

// RestartGatewayStatus 获取网关重启状态
func RestartGatewayStatus(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		resultPath := filepath.Join(cfg.OpenClawDir, "restart-gateway-result.json")
		data, err := os.ReadFile(resultPath)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "status": "idle"})
			return
		}
		var result map[string]interface{}
		json.Unmarshal(data, &result)
		c.JSON(http.StatusOK, gin.H{"ok": true, "result": result})
	}
}

// CheckUpdate 检查 OpenClaw 更新
func CheckUpdate(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentVersion := resolveOpenClawCurrentVersion(cfg)

		latestVersion := ""
		updateAvailable := false

		// Try openclaw update --check
		if out := runCmd("openclaw", "update", "--check"); out != "" {
			latestVersion = normalizeVersion(strings.TrimSpace(out))
		}

		// Fallback: try npm view
		if latestVersion == "" {
			if out := runCmd("npm", "view", "openclaw", "version"); out != "" {
				latestVersion = normalizeVersion(strings.TrimSpace(out))
			}
		}

		if latestVersion != "" && currentVersion != "unknown" && latestVersion != currentVersion {
			updateAvailable = true
		}
		if latestVersion == "" && currentVersion != "unknown" {
			latestVersion = currentVersion
		}

		// Save check result
		checkData := map[string]interface{}{
			"lastCheckedAt":       time.Now().Format(time.RFC3339),
			"lastNotifiedVersion": latestVersion,
			"updateAvailable":     updateAvailable,
		}
		if data, err := json.MarshalIndent(checkData, "", "  "); err == nil {
			os.WriteFile(filepath.Join(cfg.OpenClawDir, "update-check.json"), data, 0644)
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":              true,
			"currentVersion":  currentVersion,
			"latestVersion":   latestVersion,
			"updateAvailable": updateAvailable,
			"checkedAt":       time.Now().Format(time.RFC3339),
		})
	}
}

func resolveOpenClawCurrentVersion(cfg *config.Config) string {
	// Prefer live CLI version first: this reflects what users see in terminal
	// and avoids stale values from historical config/app paths.
	if out := runCmd("openclaw", "--version"); out != "" {
		if v := normalizeVersion(out); v != "" {
			return v
		}
	}
	if v := normalizeVersion(detectOpenClawVersion(cfg)); v != "" {
		return v
	}
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig != nil {
		if meta, ok := ocConfig["meta"].(map[string]interface{}); ok {
			if v, ok := meta["lastTouchedVersion"].(string); ok {
				if vv := normalizeVersion(v); vv != "" {
					return vv
				}
			}
		}
	}
	return "unknown"
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	re := regexp.MustCompile(`(?i)\bv?([0-9]+(?:\.[0-9]+){1,3}(?:[-+][0-9a-z.-]+)?)\b`)
	if m := re.FindStringSubmatch(v); len(m) > 1 {
		return m[1]
	}
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}([-+][0-9A-Za-z.-]+)?$`).MatchString(v) {
		return v
	}
	return ""
}

// DoUpdate 执行 OpenClaw 更新
func DoUpdate(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		signalPath := filepath.Join(cfg.OpenClawDir, "update-signal.json")
		resultPath := filepath.Join(cfg.OpenClawDir, "update-result.json")

		if data, err := os.ReadFile(resultPath); err == nil {
			var result map[string]interface{}
			json.Unmarshal(data, &result)
			if status, _ := result["status"].(string); status == "running" {
				c.JSON(http.StatusOK, gin.H{"ok": false, "error": "更新正在进行中"})
				return
			}
		}

		signalData, _ := json.Marshal(map[string]interface{}{
			"requestedAt": time.Now().Format(time.RFC3339),
		})
		resultData, _ := json.Marshal(map[string]interface{}{
			"status":    "running",
			"log":       []string{"等待宿主机执行更新..."},
			"startedAt": time.Now().Format(time.RFC3339),
		})

		os.WriteFile(signalPath, signalData, 0644)
		os.WriteFile(resultPath, resultData, 0644)

		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "更新请求已发送"})
	}
}

// UpdateStatus 获取 OpenClaw 更新状态
func UpdateStatus(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		resultPath := filepath.Join(cfg.OpenClawDir, "update-result.json")
		logPath := filepath.Join(cfg.OpenClawDir, "update-log.txt")

		data, err := os.ReadFile(resultPath)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "status": "idle", "log": []string{}})
			return
		}

		var result map[string]interface{}
		json.Unmarshal(data, &result)

		var logLines []string
		if logData, err := os.ReadFile(logPath); err == nil {
			content := strings.TrimSpace(string(logData))
			if content != "" {
				logLines = strings.Split(content, "\n")
			}
		}
		if logLines == nil {
			if l, ok := result["log"].([]interface{}); ok {
				for _, v := range l {
					if s, ok := v.(string); ok {
						logLines = append(logLines, s)
					}
				}
			}
		}
		if logLines == nil {
			logLines = []string{}
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":         true,
			"status":     result["status"],
			"log":        logLines,
			"startedAt":  result["startedAt"],
			"finishedAt": result["finishedAt"],
		})
	}
}

// CheckPanelUpdate 检查 ClawPanel 面板自身更新（国内加速服务器）
func CheckPanelUpdate(updater *update.Updater) gin.HandlerFunc {
	return func(c *gin.Context) {
		info, hasUpdate, err := updater.CheckUpdate()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":            true,
			"hasUpdate":     hasUpdate,
			"latestVersion": info.LatestVersion,
			"releaseTime":   info.ReleaseTime,
			"releaseNote":   info.ReleaseNote,
		})
	}
}

// DoPanelUpdate 执行 ClawPanel 面板自身更新
func DoPanelUpdate(updater *update.Updater) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check progress first
		p := updater.GetProgress()
		if p.Status == "downloading" || p.Status == "verifying" || p.Status == "replacing" {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "更新正在进行中"})
			return
		}

		// Check for update
		info, hasUpdate, err := updater.CheckUpdate()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if !hasUpdate {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "当前已是最新版本"})
			return
		}

		// Start async update
		updater.DoUpdate(info)
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "更新已开始"})
	}
}

// PanelUpdateProgress 获取面板更新进度
func PanelUpdateProgress(updater *update.Updater) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := updater.GetProgress()
		c.JSON(http.StatusOK, gin.H{
			"ok":       true,
			"status":   p.Status,
			"progress": p.Progress,
			"message":  p.Message,
			"log":      p.Log,
			"error":    p.Error,
		})
	}
}

// GetUpdatePopup 获取更新弹窗信息
func GetUpdatePopup(updater *update.Updater) gin.HandlerFunc {
	return func(c *gin.Context) {
		popup := updater.GetUpdatePopup()
		if popup == nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "show": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":          true,
			"show":        popup.Show,
			"version":     popup.Version,
			"releaseNote": popup.ReleaseNote,
		})
	}
}

// MarkUpdatePopupShown 标记更新弹窗已显示
func MarkUpdatePopupShown(updater *update.Updater) gin.HandlerFunc {
	return func(c *gin.Context) {
		updater.MarkPopupShown()
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GenerateUpdateToken 生成更新工具的临时授权令牌
func GenerateUpdateToken(cfg *config.Config, panelPort int) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := updaterPkg.GenerateToken(panelPort)
		updaterPort := updaterPkg.UpdaterPort
		// Extract hostname from request Host (may or may not include port)
		host := c.Request.Host
		if idx := strings.LastIndex(host, ":"); idx > 0 {
			host = host[:idx]
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":          true,
			"token":       token,
			"updaterPort": updaterPort,
			"updaterURL":  fmt.Sprintf("http://%s:%d/updater?token=%s", host, updaterPort, token),
		})
	}
}

// GetUpdateHistory 获取更新历史记录
func GetUpdateHistory(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		logFile := filepath.Join(cfg.DataDir, "update_history.json")
		var history []map[string]interface{}
		if data, err := os.ReadFile(logFile); err == nil {
			json.Unmarshal(data, &history)
		}
		if history == nil {
			history = []map[string]interface{}{}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "history": history})
	}
}
