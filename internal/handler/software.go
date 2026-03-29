package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/monitor"
	"github.com/zhaoxinyi02/ClawPanel/internal/plugin"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
	"github.com/zhaoxinyi02/ClawPanel/internal/taskman"
)

const pinnedOpenClawVersion = "2026.2.26"

// SoftwareInfo 软件信息
type SoftwareInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Installed   bool   `json:"installed"`
	Status      string `json:"status"`   // installed, not_installed, running, stopped, plugin_missing
	Category    string `json:"category"` // runtime, container, service
	Installable bool   `json:"installable"`
	Icon        string `json:"icon,omitempty"`
}

type QQChannelState struct {
	PluginInstalled bool   `json:"pluginInstalled"`
	NapCatInstalled bool   `json:"napcatInstalled"`
	Configured      bool   `json:"configured"`
	Enabled         bool   `json:"enabled"`
	ResidualConfig  bool   `json:"residualConfig"`
	InstallPath     string `json:"installPath,omitempty"`
	Message         string `json:"message,omitempty"`
}

func hasQQPlugin(cfg *config.Config) bool {
	if ocConfig, err := cfg.ReadOpenClawJSON(); err == nil && ocConfig != nil {
		if plugins, ok := ocConfig["plugins"].(map[string]interface{}); ok && plugins != nil {
			if installs, ok := plugins["installs"].(map[string]interface{}); ok && installs != nil {
				if qqInstall, ok := installs["qq"].(map[string]interface{}); ok && qqInstall != nil {
					if installPath := strings.TrimSpace(fmt.Sprint(qqInstall["installPath"])); installPath != "" {
						if info, err := os.Stat(installPath); err == nil && info.IsDir() {
							return true
						}
					}
				}
			}
		}
	}
	for _, candidate := range []string{
		filepath.Join(cfg.OpenClawDir, "extensions", "qq"),
		filepath.Join(filepath.Dir(cfg.OpenClawDir), "extensions", "qq"),
		filepath.Join(cfg.BundledOpenClawAppDir(), "extensions", "qq"),
		cfg.BundledPluginDir("qq"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func qqPluginInstallPath(cfg *config.Config) string {
	if ocConfig, err := cfg.ReadOpenClawJSON(); err == nil && ocConfig != nil {
		if plugins, ok := ocConfig["plugins"].(map[string]interface{}); ok && plugins != nil {
			if installs, ok := plugins["installs"].(map[string]interface{}); ok && installs != nil {
				if qqInstall, ok := installs["qq"].(map[string]interface{}); ok && qqInstall != nil {
					if installPath := strings.TrimSpace(fmt.Sprint(qqInstall["installPath"])); installPath != "" {
						return installPath
					}
				}
			}
		}
	}
	for _, candidate := range []string{
		filepath.Join(cfg.OpenClawDir, "extensions", "qq"),
		filepath.Join(filepath.Dir(cfg.OpenClawDir), "extensions", "qq"),
		filepath.Join(cfg.BundledOpenClawAppDir(), "extensions", "qq"),
		cfg.BundledPluginDir("qq"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join(cfg.OpenClawDir, "extensions", "qq")
}

func napcatInstalled(cfg *config.Config) bool {
	if runtime.GOOS == "windows" {
		return getNapCatShellDir(cfg) != ""
	}
	exists, _ := getDockerContainerStatus("openclaw-qq")
	return exists
}

func readQQChannelState(cfg *config.Config) QQChannelState {
	state := QQChannelState{
		PluginInstalled: hasQQPlugin(cfg),
		NapCatInstalled: napcatInstalled(cfg),
		InstallPath:     qqPluginInstallPath(cfg),
	}
	ocConfig, err := cfg.ReadOpenClawJSON()
	if err != nil || ocConfig == nil {
		state.Message = "OpenClaw 配置文件不存在或尚未初始化"
		return state
	}
	channels, _ := ocConfig["channels"].(map[string]interface{})
	plugins, _ := ocConfig["plugins"].(map[string]interface{})
	entries, _ := plugins["entries"].(map[string]interface{})
	installs, _ := plugins["installs"].(map[string]interface{})
	qqChannel, _ := channels["qq"].(map[string]interface{})
	qqEntry, _ := entries["qq"].(map[string]interface{})
	qqInstall, _ := installs["qq"].(map[string]interface{})
	state.Configured = qqChannel != nil || qqEntry != nil || qqInstall != nil
	state.Enabled = false
	if qqChannel != nil {
		if enabled, _ := qqChannel["enabled"].(bool); enabled {
			state.Enabled = true
		}
	}
	if qqEntry != nil {
		if enabled, _ := qqEntry["enabled"].(bool); enabled {
			state.Enabled = true
		}
	}
	state.ResidualConfig = state.Configured && !state.PluginInstalled && !state.NapCatInstalled
	if state.ResidualConfig {
		state.Message = "检测到旧版本遗留的 QQ 配置，可清理后恢复为纯净状态"
	}
	return state
}

func applyQQChannelConfig(ocConfig map[string]interface{}, installPath string, enabled bool) {
	if ocConfig == nil {
		return
	}
	gw, _ := ocConfig["gateway"].(map[string]interface{})
	if gw == nil {
		gw = map[string]interface{}{}
		ocConfig["gateway"] = gw
	}
	gw["mode"] = "local"

	channels, _ := ocConfig["channels"].(map[string]interface{})
	if channels == nil {
		channels = map[string]interface{}{}
		ocConfig["channels"] = channels
	}
	qqChannel, _ := channels["qq"].(map[string]interface{})
	if qqChannel == nil {
		qqChannel = map[string]interface{}{}
		channels["qq"] = qqChannel
	}
	if strings.TrimSpace(fmt.Sprint(qqChannel["wsUrl"])) == "" {
		qqChannel["wsUrl"] = "ws://127.0.0.1:3001"
	}
	qqChannel["enabled"] = enabled

	plugins, _ := ocConfig["plugins"].(map[string]interface{})
	if plugins == nil {
		plugins = map[string]interface{}{}
		ocConfig["plugins"] = plugins
	}
	entries, _ := plugins["entries"].(map[string]interface{})
	if entries == nil {
		entries = map[string]interface{}{}
		plugins["entries"] = entries
	}
	entries["qq"] = map[string]interface{}{"enabled": enabled}

	installs, _ := plugins["installs"].(map[string]interface{})
	if installs == nil {
		installs = map[string]interface{}{}
		plugins["installs"] = installs
	}
	installs["qq"] = map[string]interface{}{
		"installPath": installPath,
		"source":      "path",
		"version":     "latest",
	}
}

func removeQQChannelConfig(ocConfig map[string]interface{}) {
	if ocConfig == nil {
		return
	}
	if channels, ok := ocConfig["channels"].(map[string]interface{}); ok && channels != nil {
		delete(channels, "qq")
	}
	if plugins, ok := ocConfig["plugins"].(map[string]interface{}); ok && plugins != nil {
		if entries, ok := plugins["entries"].(map[string]interface{}); ok && entries != nil {
			delete(entries, "qq")
		}
		if installs, ok := plugins["installs"].(map[string]interface{}); ok && installs != nil {
			delete(installs, "qq")
		}
	}
}

func backupOpenClawConfigRaw(cfg *config.Config) ([]byte, error) {
	cfgPath := filepath.Join(cfg.OpenClawDir, "openclaw.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func restoreOpenClawConfigRaw(cfg *config.Config, data []byte) error {
	cfgPath := filepath.Join(cfg.OpenClawDir, "openclaw.json")
	if data == nil {
		if err := os.Remove(cfgPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0644)
}

func qqChannelFlagPath(cfg *config.Config) string {
	return filepath.Join(cfg.DataDir, "qq-channel-enabled.flag")
}

func markQQChannelManaged(cfg *config.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(qqChannelFlagPath(cfg), []byte("managed\n"), 0644)
}

func clearQQChannelManaged(cfg *config.Config) {
	_ = os.Remove(qqChannelFlagPath(cfg))
}

func restartOpenClawGateway(cfg *config.Config, procMgr *process.Manager) error {
	if procMgr != nil {
		return procMgr.Restart()
	}
	return nil
}

// OpenClawInstance 检测到的 OpenClaw 实例
type OpenClawInstance struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // npm, source, docker, systemd
	Label   string `json:"label"`
	Version string `json:"version"`
	Path    string `json:"path,omitempty"`
	Active  bool   `json:"active"`
	Status  string `json:"status"` // running, stopped, unknown
}

func detectCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Env = config.BuildExecEnv()
	out, err := cmd.Output()
	if err != nil {
		if runtime.GOOS == "darwin" && name != "arch" {
			for _, archFlag := range []string{"-arm64", "-x86_64"} {
				altArgs := append([]string{archFlag, name}, args...)
				alt := exec.Command("arch", altArgs...)
				alt.Env = cmd.Env
				if out2, err2 := alt.Output(); err2 == nil {
					return strings.TrimSpace(string(out2))
				}
			}
		}
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectPythonVersion() string {
	return detectPythonVersionWith(func(name string, args ...string) string {
		return detectCmd(name, args...)
	})
}

func detectPythonVersionWith(run func(string, ...string) string) string {
	for _, candidate := range [][]string{{"python3", "--version"}, {"python", "--version"}, {"py", "--version"}} {
		if out := run(candidate[0], candidate[1:]...); out != "" {
			return out
		}
	}
	return ""
}

func isDockerContainerRunning(name string) bool {
	out := detectCmd("docker", "inspect", "--format", "{{.State.Running}}", name)
	return out == "true"
}

func getDockerContainerStatus(name string) (bool, string) {
	out := detectCmd("docker", "inspect", "--format", "{{.State.Status}}", name)
	if out == "" {
		return false, "not_installed"
	}
	return true, out // running, exited, etc.
}

// GetSoftwareList 获取软件环境列表
func GetSoftwareList(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var list []SoftwareInfo

		if cfg.IsLiteEdition() {
			ocVer := detectOpenClawVersion(cfg)
			if strings.TrimSpace(ocVer) == "" || ocVer == "installed" {
				ocVer = "2026.2.26"
			}
			list = append(list, SoftwareInfo{
				ID: "openclaw", Name: "OpenClaw", Description: "Lite 内嵌 AI 助手核心引擎",
				Version: ocVer, Installed: true, Installable: false,
				Status: boolStatus(true), Category: "service", Icon: "brain",
			})
			c.JSON(http.StatusOK, gin.H{"ok": true, "software": list, "platform": runtime.GOOS})
			return
		}

		// Node.js
		nodeVer := detectCmd("node", "--version")
		list = append(list, SoftwareInfo{
			ID: "nodejs", Name: "Node.js", Description: "JavaScript 运行时",
			Version: nodeVer, Installed: nodeVer != "", Installable: !cfg.IsLiteEdition(),
			Status: boolStatus(nodeVer != ""), Category: "runtime", Icon: "terminal",
		})

		// npm
		npmVer := detectCmd("npm", "--version")
		list = append(list, SoftwareInfo{
			ID: "npm", Name: "npm", Description: "Node.js 包管理器",
			Version: npmVer, Installed: npmVer != "", Installable: false,
			Status: boolStatus(npmVer != ""), Category: "runtime", Icon: "package",
		})

		// Docker
		dockerVer := detectCmd("docker", "--version")
		list = append(list, SoftwareInfo{
			ID: "docker", Name: "Docker", Description: "容器运行时",
			Version: dockerVer, Installed: dockerVer != "", Installable: true,
			Status: boolStatus(dockerVer != ""), Category: "runtime", Icon: "box",
		})

		// Git
		gitVer := detectCmd("git", "--version")
		list = append(list, SoftwareInfo{
			ID: "git", Name: "Git", Description: "版本控制系统",
			Version: gitVer, Installed: gitVer != "", Installable: !cfg.IsLiteEdition(),
			Status: boolStatus(gitVer != ""), Category: "runtime", Icon: "git-branch",
		})

		// Python
		pythonVer := detectPythonVersion()
		list = append(list, SoftwareInfo{
			ID: "python", Name: "Python 3", Description: "Python 运行时",
			Version: pythonVer, Installed: pythonVer != "", Installable: true,
			Status: boolStatus(pythonVer != ""), Category: "runtime", Icon: "code",
		})

		// OpenClaw
		ocVer := detectOpenClawVersion(cfg)
		list = append(list, SoftwareInfo{
			ID: "openclaw", Name: "OpenClaw", Description: "AI 助手核心引擎",
			Version: ocVer, Installed: ocVer != "", Installable: !cfg.IsLiteEdition(),
			Status: boolStatus(ocVer != ""), Category: "service", Icon: "brain",
		})

		// NapCat (QQ) - detect Docker container OR native Windows Shell install
		napcatExists := false
		napcatStatus := "not_installed"
		napcatVer := ""
		if runtime.GOOS == "windows" {
			// Windows: detect NapCat Shell installation
			napcatDir := getNapCatShellDir(cfg)
			if napcatDir != "" {
				napcatExists = true
				napcatVer = "Shell (Windows)"
				// Check if napcat process is running
				if isNapCatShellRunning() {
					napcatStatus = "running"
				} else {
					napcatStatus = "installed"
				}
			}
		} else {
			// Linux/macOS: detect Docker container
			napcatExists, napcatStatus = getDockerContainerStatus("openclaw-qq")
			if napcatExists {
				napcatVer = "Docker"
			}
		}
		if napcatExists && !hasQQPlugin(cfg) {
			napcatStatus = "plugin_missing"
		}
		list = append(list, SoftwareInfo{
			ID: "napcat", Name: "NapCat (QQ个人号)", Description: "QQ 机器人 OneBot11 协议",
			Version: napcatVer, Installed: napcatExists, Installable: true,
			Status: napcatStatus, Category: "container", Icon: "message-circle",
		})

		// WeChat Ferry Bridge
		wechatCfg := loadWechatConfigMap(cfg)
		wechatExists := strings.TrimSpace(fmt.Sprint(wechatCfg["bridgeUrl"])) != ""
		wechatStatus := "not_installed"
		wechatVer := ""
		if wechatExists {
			wechatVer = "WeChatFerry Bridge"
			wechatStatus = "installed"
			if statusResp, err := wechatBridgeRequest(cfg, http.MethodGet, "/status", nil); err == nil {
				wechatStatus = "running"
				if version := strings.TrimSpace(fmt.Sprint(statusResp["version"])); version != "" {
					wechatVer = version
				}
			}
		}
		list = append(list, SoftwareInfo{
			ID: "wechat", Name: "微信个人号", Description: "Windows 宿主机 WeChatFerry Bridge",
			Version: wechatVer, Installed: wechatExists, Installable: true,
			Status: wechatStatus, Category: "service", Icon: "message-square",
		})

		c.JSON(http.StatusOK, gin.H{"ok": true, "software": list, "platform": runtime.GOOS})
	}
}

func GetQQChannelState(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := readQQChannelState(cfg)
		c.JSON(http.StatusOK, gin.H{"ok": true, "state": state})
	}
}

func SetupQQChannel(cfg *config.Config, tm *taskman.Manager, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.OpenClawInstalled() {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "请先安装 OpenClaw，再安装 QQ 通道"})
			return
		}
		if tm.HasRunningTask("setup_qq_channel") {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "已有 QQ 通道安装任务正在进行中"})
			return
		}
		task := tm.CreateTask("安装 QQ 通道", "setup_qq_channel")
		go func() {
			task.AppendLog("🔍 开始检查 QQ 通道依赖")
			state := readQQChannelState(cfg)
			if state.PluginInstalled && state.NapCatInstalled && state.Configured {
				task.AppendLog("✅ QQ 通道组件与配置已存在，跳过安装，仅保持当前状态")
				tm.FinishTask(task, nil)
				return
			}
			if !state.PluginInstalled || !state.NapCatInstalled {
				task.AppendLog("📦 开始安装 QQ 插件与 NapCat 组件")
				var err error
				if sudoPass := getSudoPass(cfg); sudoPass != "" && runtime.GOOS != "windows" {
					err = tm.RunScriptWithSudo(task, sudoPass, buildNapCatInstallScript(cfg))
				} else if runtime.GOOS == "windows" {
					err = tm.RunScript(task, buildNapCatWindowsInstallScript(cfg))
				} else {
					err = tm.RunScript(task, buildNapCatInstallScript(cfg))
				}
				if err != nil {
					tm.FinishTask(task, err)
					return
				}
			}
			backup, err := backupOpenClawConfigRaw(cfg)
			if err != nil {
				tm.FinishTask(task, err)
				return
			}
			task.AppendLog("💾 已备份当前 OpenClaw 配置")
			ocConfig, err := cfg.ReadOpenClawJSON()
			if err != nil || ocConfig == nil {
				ocConfig = map[string]interface{}{}
			}
			applyQQChannelConfig(ocConfig, qqPluginInstallPath(cfg), false)
			if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
				tm.FinishTask(task, err)
				return
			}
			task.AppendLog("📝 已写入 QQ 通道配置（默认未启用）")
			if err := restartOpenClawGateway(cfg, procMgr); err != nil {
				_ = restoreOpenClawConfigRaw(cfg, backup)
				tm.FinishTask(task, fmt.Errorf("OpenClaw 重启失败，已回滚配置: %w", err))
				return
			}
			_ = markQQChannelManaged(cfg)
			task.AppendLog("🔄 OpenClaw 网关已重启")
			task.AppendLog("✅ QQ 通道安装完成，请在通道页手动启用并登录 QQ")
			tm.FinishTask(task, nil)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID})
	}
}

func RepairQQChannel(cfg *config.Config, tm *taskman.Manager, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := readQQChannelState(cfg)
		if !state.PluginInstalled || !state.NapCatInstalled {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "请先完成 QQ 通道安装，再执行修复"})
			return
		}
		if tm.HasRunningTask("repair_qq_channel") {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "已有 QQ 通道修复任务正在进行中"})
			return
		}
		task := tm.CreateTask("修复 QQ 通道配置", "repair_qq_channel")
		go func() {
			backup, err := backupOpenClawConfigRaw(cfg)
			if err != nil {
				tm.FinishTask(task, err)
				return
			}
			task.AppendLog("💾 已备份当前 OpenClaw 配置")
			ocConfig, err := cfg.ReadOpenClawJSON()
			if err != nil || ocConfig == nil {
				ocConfig = map[string]interface{}{}
			}
			applyQQChannelConfig(ocConfig, qqPluginInstallPath(cfg), state.Enabled)
			if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
				tm.FinishTask(task, err)
				return
			}
			task.AppendLog("🛠️ 已重新写入 QQ 通道配置")
			if err := restartOpenClawGateway(cfg, procMgr); err != nil {
				_ = restoreOpenClawConfigRaw(cfg, backup)
				tm.FinishTask(task, fmt.Errorf("OpenClaw 重启失败，已回滚配置: %w", err))
				return
			}
			_ = markQQChannelManaged(cfg)
			task.AppendLog("🔄 OpenClaw 网关已重启")
			tm.FinishTask(task, nil)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID})
	}
}

func CleanupQQChannel(cfg *config.Config, tm *taskman.Manager, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tm.HasRunningTask("cleanup_qq_channel") {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "已有 QQ 通道清理任务正在进行中"})
			return
		}
		task := tm.CreateTask("清理 QQ 通道残留配置", "cleanup_qq_channel")
		go func() {
			backup, err := backupOpenClawConfigRaw(cfg)
			if err != nil {
				tm.FinishTask(task, err)
				return
			}
			task.AppendLog("💾 已备份当前 OpenClaw 配置")
			ocConfig, err := cfg.ReadOpenClawJSON()
			if err != nil || ocConfig == nil {
				ocConfig = map[string]interface{}{}
			}
			removeQQChannelConfig(ocConfig)
			if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
				tm.FinishTask(task, err)
				return
			}
			task.AppendLog("🧹 已移除 openclaw.json 中的 QQ 配置")
			if err := restartOpenClawGateway(cfg, procMgr); err != nil {
				_ = restoreOpenClawConfigRaw(cfg, backup)
				tm.FinishTask(task, fmt.Errorf("OpenClaw 重启失败，已回滚配置: %w", err))
				return
			}
			clearQQChannelManaged(cfg)
			task.AppendLog("🔄 OpenClaw 网关已重启")
			tm.FinishTask(task, nil)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID})
	}
}

func DeleteQQChannel(cfg *config.Config, tm *taskman.Manager, procMgr *process.Manager, pm *plugin.Manager, napcatMon *monitor.NapCatMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tm.HasRunningTask("delete_qq_channel") {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "已有 QQ 通道删除任务正在进行中"})
			return
		}
		task := tm.CreateTask("删除 QQ 通道", "delete_qq_channel")
		go func() {
			backup, err := backupOpenClawConfigRaw(cfg)
			if err != nil {
				tm.FinishTask(task, err)
				return
			}
			task.AppendLog("💾 已备份当前 OpenClaw 配置")

			if napcatMon != nil {
				napcatMon.Pause()
			}

			if err := removeNapCatInstall(task.AppendLog, cfg); err != nil {
				tm.FinishTask(task, err)
				return
			}

			if pm != nil {
				if err := pm.UninstallWithProgress("qq", true, task.AppendLog); err != nil {
					if !strings.Contains(err.Error(), "未安装") {
						tm.FinishTask(task, fmt.Errorf("卸载 QQ 插件失败: %w", err))
						return
					}
					task.AppendLog("ℹ️ 未检测到 QQ 插件安装记录，跳过插件卸载")
				}
			}

			ocConfig, err := cfg.ReadOpenClawJSON()
			if err != nil || ocConfig == nil {
				ocConfig = map[string]interface{}{}
			}
			removeQQChannelConfig(ocConfig)
			if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
				tm.FinishTask(task, err)
				return
			}
			task.AppendLog("🧹 已清空 openclaw.json 中的 QQ 通道配置")

			clearQQChannelManaged(cfg)
			task.AppendLog("🧽 已清理 QQ 通道托管标记")

			if err := restartOpenClawGateway(cfg, procMgr); err != nil {
				_ = restoreOpenClawConfigRaw(cfg, backup)
				tm.FinishTask(task, fmt.Errorf("OpenClaw 重启失败，已回滚配置: %w", err))
				return
			}
			task.AppendLog("🔄 OpenClaw 网关已重启")
			task.AppendLog("✅ QQ 通道、NapCat、插件与相关配置已删除")
			tm.FinishTask(task, nil)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID, "message": "QQ 通道删除任务已创建，请在消息中心查看进度"})
	}
}

func removeNapCatInstall(logf func(string), cfg *config.Config) error {
	if runtime.GOOS == "windows" {
		monitor.StopNapCatPlatform()
		napcatDir := getNapCatShellDir(cfg)
		if strings.TrimSpace(napcatDir) == "" {
			if logf != nil {
				logf("ℹ️ 未检测到 NapCat Shell 安装目录，跳过 NapCat 删除")
			}
			return nil
		}
		if logf != nil {
			logf("🗑️ 正在删除 NapCat Shell 安装目录")
		}
		if err := os.RemoveAll(napcatDir); err != nil {
			return fmt.Errorf("删除 NapCat Shell 目录失败: %w", err)
		}
		return nil
	}

	if logf != nil {
		logf("🛑 正在停止并删除 NapCat Docker 容器")
	}
	commands := [][]string{
		{"rm", "-f", "openclaw-qq"},
		{"volume", "rm", "-f", "napcat-qq-session", "napcat-config"},
	}
	for _, args := range commands {
		cmd := exec.Command("docker", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			if strings.Contains(msg, "No such container") || strings.Contains(msg, "No such volume") {
				continue
			}
			return fmt.Errorf("docker %s 失败: %s", strings.Join(args, " "), msg)
		}
	}
	return nil
}

func boolStatus(installed bool) string {
	if installed {
		return "installed"
	}
	return "not_installed"
}

func nodeMajorVersion(ver string) int {
	v := strings.TrimSpace(strings.TrimPrefix(ver, "v"))
	if v == "" {
		return -1
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return -1
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return -1
	}
	return major
}

func formatOpenClawManualPrerequisiteError(platform, nodeVer, npmVer, gitVer string) error {
	if platform != "windows" && platform != "darwin" {
		return nil
	}
	label := platform
	if platform == "darwin" {
		label = "macOS"
	}
	missing := make([]string, 0, 3)
	if nodeVer == "" {
		missing = append(missing, "Node.js (>=20) https://nodejs.org")
	} else if nodeMajorVersion(nodeVer) < 20 {
		missing = append(missing, fmt.Sprintf("Node.js >=20（当前 %s） https://nodejs.org", nodeVer))
	}
	if npmVer == "" {
		missing = append(missing, "npm（需随 Node.js 一起可用） https://nodejs.org")
	}
	if gitVer == "" {
		missing = append(missing, "Git https://git-scm.com/downloads")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("检测到 %s 平台缺少前置依赖：%s。为避免一键安装中途报错，请先手动安装后再执行 OpenClaw 安装", label, strings.Join(missing, "；"))
}

func ensureOpenClawManualPrerequisites() error {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return nil
	}
	nodeVer := detectCmd("node", "--version")
	npmVer := detectCmd("npm", "--version")
	gitVer := detectCmd("git", "--version")
	return formatOpenClawManualPrerequisiteError(runtime.GOOS, nodeVer, npmVer, gitVer)
}

func detectOpenClawVersion(cfg *config.Config) string {
	if cfg != nil && cfg.IsLiteEdition() {
		if cfg.OpenClawApp != "" {
			pkgPath := filepath.Join(cfg.OpenClawApp, "package.json")
			if v := readVersionFromPackageJSON(pkgPath); v != "" {
				return v
			}
		}
		if cmd, err := cfg.OpenClawCommand("--version"); err == nil && cmd != nil {
			cmd.Env = config.BuildExecEnv()
			if out, err := cmd.Output(); err == nil {
				if v := strings.TrimPrefix(strings.TrimSpace(string(out)), "v"); v != "" {
					return v
				}
			}
		}
		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig != nil {
			if meta, ok := ocConfig["meta"].(map[string]interface{}); ok {
				if v, ok := meta["lastTouchedVersion"].(string); ok && v != "" {
					return v
				}
			}
		}
		return ""
	}

	// 1. Try reading from cfg.OpenClawApp FIRST (most reliable for SYSTEM service)
	if cfg.OpenClawApp != "" {
		pkgPath := filepath.Join(cfg.OpenClawApp, "package.json")
		if v := readVersionFromPackageJSON(pkgPath); v != "" {
			return v
		}
	}

	// 2. Try binary path detection independent of PATH (nvm/fnm/service env safe)
	if bin := config.DetectOpenClawBinaryPath(); bin != "" {
		if out := detectCmd(bin, "--version"); out != "" {
			return strings.TrimPrefix(strings.TrimSpace(out), "v")
		}
	}

	// 3. Try npm global package.json (may fail when running as SYSTEM service)
	npmRoot := detectCmd("npm", "root", "-g")
	if npmRoot != "" {
		pkgPath := filepath.Join(npmRoot, "openclaw", "package.json")
		if v := readVersionFromPackageJSON(pkgPath); v != "" {
			return v
		}
	}

	// 3.5 Try common app path discovery from config layer
	if appDir := configPathDiscoverOpenClawApp(); appDir != "" {
		if v := readVersionFromPackageJSON(filepath.Join(appDir, "package.json")); v != "" {
			return v
		}
	}

	// 4. Try openclaw CLI (may not work when running as SYSTEM service)
	ver := detectCmd("openclaw", "--version")
	if ver != "" {
		return strings.TrimPrefix(strings.TrimSpace(ver), "v")
	}

	// 5. Try from config meta.lastTouchedVersion
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig != nil {
		if meta, ok := ocConfig["meta"].(map[string]interface{}); ok {
			if v, ok := meta["lastTouchedVersion"].(string); ok && v != "" {
				return v
			}
		}
	}

	// 6. Try common binary paths
	home, _ := os.UserHomeDir()
	commonPaths := []string{
		"/usr/local/bin/openclaw",
		"/usr/bin/openclaw",
		filepath.Join(home, ".local/bin/openclaw"),
		filepath.Join(home, ".npm-global/bin/openclaw"),
	}
	if runtime.GOOS == "windows" {
		commonPaths = append(commonPaths,
			filepath.Join(home, "AppData", "Roaming", "npm", "openclaw.cmd"),
			`C:\Program Files\nodejs\openclaw.cmd`,
		)
		// Scan real user profiles (when running as SYSTEM service)
		usersDir := `C:\Users`
		if entries, err := os.ReadDir(usersDir); err == nil {
			skip := map[string]bool{"Public": true, "Default": true, "Default User": true, "All Users": true}
			for _, e := range entries {
				if e.IsDir() && !skip[e.Name()] {
					commonPaths = append(commonPaths,
						filepath.Join(usersDir, e.Name(), "AppData", "Roaming", "npm", "openclaw.cmd"),
					)
				}
			}
		}
		// SYSTEM account path (when running as Windows service)
		systemRoot := os.Getenv("SYSTEMROOT")
		if systemRoot != "" {
			commonPaths = append(commonPaths,
				filepath.Join(systemRoot, "system32", "config", "systemprofile", "AppData", "Roaming", "npm", "openclaw.cmd"),
			)
		}
		// npm prefix path
		npmPrefix := detectCmd("npm", "config", "get", "prefix")
		if npmPrefix != "" {
			commonPaths = append(commonPaths, filepath.Join(npmPrefix, "openclaw.cmd"))
		}
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			out := detectCmd(p, "--version")
			if out != "" {
				return strings.TrimPrefix(strings.TrimSpace(out), "v")
			}
		}
	}

	// 7. Try source installs: check common directories for package.json
	sourcePaths := []string{
		filepath.Join(os.Getenv("HOME"), "openclaw"),
		filepath.Join(os.Getenv("HOME"), "openclaw/app"),
		"/opt/openclaw",
		"/usr/lib/node_modules/openclaw",
	}
	for _, sp := range sourcePaths {
		pkgPath := filepath.Join(sp, "package.json")
		if v := readVersionFromPackageJSON(pkgPath); v != "" {
			return v
		}
	}

	// 8. Try Docker container
	dockerVer := detectCmd("docker", "exec", "openclaw", "openclaw", "--version")
	if dockerVer != "" {
		return strings.TrimPrefix(strings.TrimSpace(dockerVer), "v")
	}

	// 9. Try systemd: parse ExecStart from service file to find binary path
	svcContent := detectCmd("systemctl", "cat", "openclaw")
	if svcContent != "" {
		for _, line := range strings.Split(svcContent, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ExecStart=") {
				parts := strings.Fields(strings.TrimPrefix(line, "ExecStart="))
				if len(parts) > 0 {
					bin := parts[0]
					out := detectCmd(bin, "--version")
					if out != "" {
						return strings.TrimPrefix(strings.TrimSpace(out), "v")
					}
				}
				break
			}
		}
	}

	// 8. Config file exists but no version extractable
	if ocConfig != nil {
		return "installed"
	}

	return ""
}

func configPathDiscoverOpenClawApp() string {
	// Keep this helper in handler layer so we don't expose internal config methods.
	// It derives app path from known binary path layout when package.json is missing from cfg.
	if bin := config.DetectOpenClawBinaryPath(); bin != "" {
		// .../bin/openclaw -> .../lib/node_modules/openclaw
		parent := filepath.Dir(filepath.Dir(bin))
		candidate := filepath.Join(parent, "lib", "node_modules", "openclaw")
		if info, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// readVersionFromPackageJSON reads the "version" field from a package.json file
func readVersionFromPackageJSON(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	if v, ok := pkg["version"].(string); ok && v != "" {
		return v
	}
	return ""
}

// DetectOpenClawInstances 检测所有 OpenClaw 安装实例
func DetectOpenClawInstances(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.IsLiteEdition() {
			c.JSON(http.StatusOK, gin.H{"ok": true, "instances": []OpenClawInstance{}})
			return
		}
		var instances []OpenClawInstance

		// 1. npm global install
		var npmPath string
		npmPath = config.DetectOpenClawBinaryPath()
		// Fallback to shell lookup for unusual wrappers
		if npmPath == "" {
			if runtime.GOOS == "windows" {
				npmPath = detectCmd("where", "openclaw")
				if idx := strings.Index(npmPath, "\n"); idx > 0 {
					npmPath = strings.TrimSpace(npmPath[:idx])
				}
			} else {
				npmPath = detectCmd("which", "openclaw")
			}
		}
		if npmPath != "" {
			ver := detectCmd(npmPath, "--version")
			instances = append(instances, OpenClawInstance{
				ID: "npm-global", Type: "npm", Label: "npm 全局安装",
				Version: ver, Path: npmPath, Active: true, Status: "installed",
			})
		}
		// Windows: also check npm global dir directly
		if runtime.GOOS == "windows" && npmPath == "" {
			home, _ := os.UserHomeDir()
			winNpmPath := filepath.Join(home, "AppData", "Roaming", "npm", "openclaw.cmd")
			if _, err := os.Stat(winNpmPath); err == nil {
				ver := detectCmd("openclaw", "--version")
				instances = append(instances, OpenClawInstance{
					ID: "npm-global", Type: "npm", Label: "npm 全局安装",
					Version: ver, Path: winNpmPath, Active: true, Status: "installed",
				})
			}
		}

		// 2. systemd service
		systemdOut := detectCmd("systemctl", "is-active", "openclaw")
		if systemdOut == "active" || systemdOut == "inactive" {
			ver := ""
			if ocConfig, _ := cfg.ReadOpenClawJSON(); ocConfig != nil {
				if meta, ok := ocConfig["meta"].(map[string]interface{}); ok {
					ver, _ = meta["lastTouchedVersion"].(string)
				}
			}
			instances = append(instances, OpenClawInstance{
				ID: "systemd", Type: "systemd", Label: "systemd 服务",
				Version: ver, Active: systemdOut == "active", Status: systemdOut,
			})
		}

		// 3. Docker container
		dockerOut := detectCmd("docker", "ps", "-a", "--filter", "name=openclaw", "--format", "{{.Names}}|{{.Status}}|{{.Image}}")
		if dockerOut != "" {
			for _, line := range strings.Split(dockerOut, "\n") {
				parts := strings.SplitN(line, "|", 3)
				if len(parts) >= 2 {
					name := parts[0]
					status := parts[1]
					image := ""
					if len(parts) >= 3 {
						image = parts[2]
					}
					// Skip our management containers
					if name == "openclaw-qq" || name == "openclaw-wechat" {
						continue
					}
					running := strings.HasPrefix(status, "Up")
					instances = append(instances, OpenClawInstance{
						ID: "docker-" + name, Type: "docker", Label: "Docker: " + name,
						Version: image, Path: name, Active: running,
						Status: func() string {
							if running {
								return "running"
							}
							return "stopped"
						}(),
					})
				}
			}
		}

		// 4. Source code install (check common paths)
		sourcePaths := []string{
			filepath.Join(os.Getenv("HOME"), "openclaw"),
			"/opt/openclaw",
		}
		for _, sp := range sourcePaths {
			pkgPath := filepath.Join(sp, "package.json")
			if _, err := os.Stat(pkgPath); err == nil {
				var pkg map[string]interface{}
				if data, err := os.ReadFile(pkgPath); err == nil {
					json.Unmarshal(data, &pkg)
				}
				ver, _ := pkg["version"].(string)
				instances = append(instances, OpenClawInstance{
					ID: "source-" + sp, Type: "source", Label: "源码: " + sp,
					Version: ver, Path: sp, Active: false, Status: "installed",
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "instances": instances})
	}
}

// InstallSoftware 一键安装软件
func InstallSoftware(cfg *config.Config, tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Software string `json:"software"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Software == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "software required"})
			return
		}

		if cfg.IsLiteEdition() {
			switch req.Software {
			case "openclaw", "nodejs", "git":
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "Lite 版已内置 OpenClaw 运行环境，不支持安装或切换此组件"})
				return
			}
		}

		if tm.HasRunningTask("install_" + req.Software) {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "该软件正在安装中"})
			return
		}

		// Read sudo password
		sudoPass := ""
		if sp := getSudoPass(cfg); sp != "" {
			sudoPass = sp
		}

		var script string
		var taskName string

		switch req.Software {
		case "nodejs":
			taskName = "安装 Node.js"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 Node.js (v22 LTS)..."
$nodeCheck = Get-Command node -ErrorAction SilentlyContinue
if ($nodeCheck) {
  Write-Output "⚠️ Node.js 已安装: $(node --version)"
  Write-Output "如需更新，请从 https://nodejs.org 下载最新版"
  npm config set registry https://registry.npmmirror.com
  exit 0
}
# Check if winget is available
$wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
if ($wingetCheck) {
  Write-Output "📥 通过 winget 安装 Node.js..."
  winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements
} else {
  Write-Output "❌ 请从 https://nodejs.org 手动下载安装 Node.js"
  Write-Output "或安装 winget (App Installer) 后重试"
  exit 1
}
# Refresh PATH
$env:PATH = [System.Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("PATH","User")
npm config set registry https://registry.npmmirror.com
Write-Output "✅ Node.js $(node --version) 安装完成"
`
			} else {
				script = `
set -e
export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
echo "📦 安装 Node.js (v22 LTS)..."

# --- Helper: install Node.js v22 from official binary archive (Linux/macOS) ---
install_node_tarball() {
  local NODE_VER="v22.14.0"
  local OS_NAME
  local PKG_EXT
  local ARCH
  OS_NAME=$(uname | tr '[:upper:]' '[:lower:]')

  case "$(uname -m)" in
    x86_64|amd64) ARCH="x64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="armv7l" ;;
    *) echo "❌ 不支持的 CPU 架构: $(uname -m)"; exit 1 ;;
  esac

  if [ "$OS_NAME" = "darwin" ]; then
    if [ "$ARCH" = "armv7l" ]; then
      echo "❌ macOS 不支持 armv7l 架构"; exit 1
    fi
    PKG_EXT="tar.gz"
  else
    PKG_EXT="tar.xz"
  fi

  local FILE="node-${NODE_VER}-${OS_NAME}-${ARCH}.${PKG_EXT}"
  local URL="https://nodejs.org/dist/${NODE_VER}/${FILE}"
  local MIRROR_URL="https://npmmirror.com/mirrors/node/${NODE_VER}/${FILE}"
  echo "📥 下载 Node.js ${NODE_VER} (${OS_NAME}-${ARCH}) 官方二进制..."
  local TMP_DIR=$(mktemp -d)
  curl -fsSL "$MIRROR_URL" -o "$TMP_DIR/node.tar" 2>/dev/null || \
  curl -fsSL "$URL" -o "$TMP_DIR/node.tar" || {
    echo "❌ Node.js 下载失败"; rm -rf "$TMP_DIR"; exit 1
  }
  echo "📦 解压到 /usr/local ..."
  if [ "$PKG_EXT" = "tar.gz" ]; then
    tar -xzf "$TMP_DIR/node.tar" -C /usr/local --strip-components=1
  else
    tar -xJf "$TMP_DIR/node.tar" -C /usr/local --strip-components=1
  fi
  rm -rf "$TMP_DIR"
  hash -r
}

node_version_ok() {
  local ver=$(node --version 2>/dev/null | sed 's/^v//')
  [ -z "$ver" ] && return 1
  [ "$(echo "$ver" | cut -d. -f1)" -ge 20 ] 2>/dev/null
}

if node_version_ok; then
  echo "✅ Node.js $(node --version) 已安装且版本满足要求"
else
  if command -v node &>/dev/null; then
    echo "⚠️ 当前 Node.js $(node --version) 版本过低 (需要 >= 20)，正在升级..."
  fi

  if [ "$(uname)" = "Darwin" ]; then
    install_node_tarball
  else
    # Try NodeSource first
    DISTRO_FAMILY=""
    if [ -f /etc/os-release ]; then
      . /etc/os-release
      case "$ID $ID_LIKE" in
        *debian*|*ubuntu*) DISTRO_FAMILY="debian" ;;
        *rhel*|*centos*|*fedora*|*amzn*|*rocky*|*alma*) DISTRO_FAMILY="rhel" ;;
      esac
    fi
    [ -z "$DISTRO_FAMILY" ] && command -v apt-get &>/dev/null && DISTRO_FAMILY="debian"
    [ -z "$DISTRO_FAMILY" ] && (command -v dnf &>/dev/null || command -v yum &>/dev/null) && DISTRO_FAMILY="rhel"

    case "$DISTRO_FAMILY" in
      debian)
        echo "📥 尝试 NodeSource (deb)..."
        if curl -fsSL https://deb.nodesource.com/setup_22.x | bash - 2>/dev/null; then
          apt-get install -y nodejs || true
        fi ;;
      rhel)
        echo "📥 尝试 NodeSource (rpm)..."
        if curl -fsSL https://rpm.nodesource.com/setup_22.x | bash - 2>/dev/null; then
          if command -v dnf &>/dev/null; then dnf install -y nodejs || true
          else yum install -y nodejs || true; fi
        fi ;;
    esac

    # Fallback to binary tarball if still not good enough
    if ! node_version_ok; then
      echo "⚠️ 包管理器未能提供 Node.js >= 20，使用官方二进制安装..."
      install_node_tarball
    fi
  fi

  if ! node_version_ok; then
    echo "❌ Node.js >= 20 安装失败，请手动安装"; exit 1
  fi
fi

npm config set registry https://registry.npmmirror.com 2>/dev/null || true
echo "✅ Node.js $(node --version) 安装完成"
echo "✅ npm $(npm --version)"
`
			}
		case "docker":
			taskName = "安装 Docker"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 Docker Desktop..."
$dockerCheck = Get-Command docker -ErrorAction SilentlyContinue
if ($dockerCheck) {
  Write-Output "⚠️ Docker 已安装: $(docker --version)"
  exit 0
}
$wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
if ($wingetCheck) {
  Write-Output "📥 通过 winget 安装 Docker Desktop..."
  winget install Docker.DockerDesktop --accept-source-agreements --accept-package-agreements
  Write-Output "✅ Docker Desktop 已安装，请重启电脑后启动 Docker Desktop"
} else {
  Write-Output "❌ 请从 https://www.docker.com/products/docker-desktop 手动下载安装 Docker Desktop"
  exit 1
}
`
			} else {
				script = `
set -e
export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
export HOME="${HOME:-/var/root}"
echo "📦 安装 Docker..."
if command -v docker &>/dev/null; then
  echo "⚠️ Docker 已安装: $(docker --version)"
  exit 0
fi
if [ "$(uname)" = "Darwin" ]; then
  echo "⚠️ macOS 请手动安装 Docker Desktop: https://www.docker.com/products/docker-desktop"
  echo "或使用 Homebrew: brew install --cask docker"
  if command -v brew &>/dev/null; then
    if [ "$(id -u)" -eq 0 ]; then
      CONSOLE_USER=$(stat -f%Su /dev/console 2>/dev/null || true)
      if [ -n "$CONSOLE_USER" ] && [ "$CONSOLE_USER" != "root" ] && id "$CONSOLE_USER" &>/dev/null; then
        USER_HOME=$(dscl . -read "/Users/$CONSOLE_USER" NFSHomeDirectory 2>/dev/null | awk '{print $2}')
        [ -z "$USER_HOME" ] && USER_HOME="/Users/$CONSOLE_USER"
        echo "📥 检测到 root 环境，切换到用户 $CONSOLE_USER 执行 brew..."
        sudo -u "$CONSOLE_USER" HOME="$USER_HOME" PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin" brew install --cask docker
      else
        echo "❌ 未检测到可用登录用户，无法自动执行 brew 安装"
        exit 1
      fi
    else
      brew install --cask docker
    fi
    echo "✅ Docker Desktop 已安装，请从应用程序中启动 Docker"
  else
    exit 1
  fi
else
  DISTRO_FAMILY=""
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    case "$ID $ID_LIKE" in
      *debian*|*ubuntu*) DISTRO_FAMILY="debian" ;;
      *rhel*|*centos*|*fedora*|*amzn*|*rocky*|*alma*|*opencloudos*|*tencentos*) DISTRO_FAMILY="rhel" ;;
    esac
  fi
  [ -z "$DISTRO_FAMILY" ] && command -v apt-get &>/dev/null && DISTRO_FAMILY="debian"
  [ -z "$DISTRO_FAMILY" ] && (command -v dnf &>/dev/null || command -v yum &>/dev/null) && DISTRO_FAMILY="rhel"

  case "$DISTRO_FAMILY" in
    debian)
      echo "📥 通过阿里云镜像安装 Docker (deb)..."
      apt-get update
      apt-get install -y ca-certificates curl gnupg
      install -m 0755 -d /etc/apt/keyrings
      curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg 2>/dev/null || true
      chmod a+r /etc/apt/keyrings/docker.gpg
      echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME" || echo "jammy") stable" > /etc/apt/sources.list.d/docker.list
      apt-get update
      apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      ;;
    rhel)
      echo "📥 通过阿里云镜像安装 Docker (rpm)..."
      if command -v dnf &>/dev/null; then
        dnf install -y dnf-plugins-core
        dnf config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo 2>/dev/null || true
        dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      else
        yum install -y yum-utils
        yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
        yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      fi
      ;;
    *)
      echo "❌ 不支持的系统发行版，请手动安装 Docker"
      echo "   参考: https://docs.docker.com/engine/install/"
      exit 1
      ;;
  esac

  # Configure Docker mirror
  mkdir -p /etc/docker
  cat > /etc/docker/daemon.json << 'DOCKEREOF'
{
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me"
  ]
}
DOCKEREOF
  systemctl enable docker
  systemctl restart docker
fi
echo "✅ Docker $(docker --version) 安装完成"
`
			}
		case "git":
			taskName = "安装 Git"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 Git..."
$gitCheck = Get-Command git -ErrorAction SilentlyContinue
if ($gitCheck) {
  Write-Output "⚠️ Git 已安装: $(git --version)"
  exit 0
}
$wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
if ($wingetCheck) {
  Write-Output "📥 通过 winget 安装 Git..."
  winget install Git.Git --accept-source-agreements --accept-package-agreements
  Write-Output "✅ Git 安装完成，请重启终端使 git 命令生效"
} else {
  Write-Output "❌ 请从 https://git-scm.com/download/win 手动下载安装 Git"
  exit 1
}
`
			} else {
				script = `
set -e
echo "📦 安装 Git..."
if [ "$(uname)" = "Darwin" ]; then
  if command -v brew &>/dev/null; then
    brew install git
  else
    xcode-select --install 2>/dev/null || echo "Git 应该已通过 Xcode CLT 安装"
  fi
elif command -v apt-get &>/dev/null; then
  apt-get update
  apt-get install -y git
elif command -v yum &>/dev/null; then
  yum install -y git
else
  echo "❌ 不支持的包管理器，请手动安装 Git"
  exit 1
fi
echo "✅ $(git --version) 安装完成"
`
			}
		case "python":
			taskName = "安装 Python 3"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 Python 3..."
$pyCheck = Get-Command python -ErrorAction SilentlyContinue
if ($pyCheck) {
  Write-Output "⚠️ Python 已安装: $(python --version)"
  exit 0
}
$wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
if ($wingetCheck) {
  Write-Output "📥 通过 winget 安装 Python 3..."
  winget install Python.Python.3.12 --accept-source-agreements --accept-package-agreements
  Write-Output "✅ Python 安装完成，请重启终端"
} else {
  Write-Output "❌ 请从 https://www.python.org/downloads/ 手动下载安装 Python"
  exit 1
}
`
			} else {
				script = `
set -e
echo "📦 安装 Python 3..."
if [ "$(uname)" = "Darwin" ]; then
  if command -v brew &>/dev/null; then
    brew install python@3
  else
    echo "❌ macOS 请先安装 Homebrew 或从 python.org 下载"
    exit 1
  fi
elif command -v apt-get &>/dev/null; then
  apt-get update
  apt-get install -y python3 python3-pip python3-venv
elif command -v yum &>/dev/null; then
  yum install -y python3 python3-pip
else
  echo "❌ 不支持的包管理器，请手动安装 Python 3"
  exit 1
fi
# Set pip mirror
pip3 config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple 2>/dev/null || true
echo "✅ $(python3 --version) 安装完成"
`
			}
		case "openclaw":
			if err := ensureOpenClawManualPrerequisites(); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
			taskName = "安装 OpenClaw"
			if runtime.GOOS == "windows" {
				script = fmt.Sprintf(`
$ErrorActionPreference = "Continue"
Write-Output "📦 安装 OpenClaw..."

# ---- 工具函数 ----
function Refresh-Path {
  $machinePath = [System.Environment]::GetEnvironmentVariable("PATH", "Machine")
  $userPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
  $paths = @()
  if ($machinePath) { $paths += $machinePath }
  if ($userPath) { $paths += $userPath }

  $commonBins = @()
  if ($env:ProgramFiles) { $commonBins += (Join-Path $env:ProgramFiles "nodejs") }
  if (${env:ProgramFiles(x86)}) { $commonBins += (Join-Path ${env:ProgramFiles(x86)} "nodejs") }
  if ($env:APPDATA) { $commonBins += (Join-Path $env:APPDATA "npm") }
  foreach ($bin in $commonBins) {
    if ($bin -and (Test-Path $bin)) { $paths += $bin }
  }

  $env:PATH = ($paths | Where-Object { $_ } | Select-Object -Unique) -join ";"
}

function Invoke-NativeStep([scriptblock]$Command) {
  $previousPreference = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try {
    & $Command 2>&1 | ForEach-Object {
      $text = $_.ToString()
      if (-not [string]::IsNullOrWhiteSpace($text)) {
        Write-Output $text.TrimEnd()
      }
    }
    return $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $previousPreference
  }
}

function Get-NodeCommand {
  $cmd = Get-Command node -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd }

  $candidates = @()
  if ($env:ProgramFiles) { $candidates += (Join-Path $env:ProgramFiles "nodejs\node.exe") }
  if (${env:ProgramFiles(x86)}) { $candidates += (Join-Path ${env:ProgramFiles(x86)} "nodejs\node.exe") }

  $nvmRoots = @(
    [System.Environment]::GetEnvironmentVariable("NVM_HOME", "Machine"),
    [System.Environment]::GetEnvironmentVariable("NVM_HOME", "User"),
    $(if ($env:APPDATA) { Join-Path $env:APPDATA "nvm" })
  ) | Where-Object { $_ -and (Test-Path $_) }

  foreach ($root in $nvmRoots) {
    $versions = Get-ChildItem $root -Directory -ErrorAction SilentlyContinue | Sort-Object Name -Descending
    foreach ($versionDir in $versions) {
      $candidates += (Join-Path $versionDir.FullName "node.exe")
    }
  }

  foreach ($candidate in $candidates | Where-Object { $_ }) {
    if (Test-Path $candidate) {
      return Get-Item $candidate
    }
  }

  return $null
}

function Get-NodeVersionText {
  param([object]$NodeCmd)

  if (-not $NodeCmd) { return $null }
  $nodePath = if ($NodeCmd.Source) { $NodeCmd.Source } else { $NodeCmd.FullName }
  if (-not $nodePath) { return $null }
  return (& $nodePath --version 2>$null)
}

Refresh-Path

# ---- 检查并自动安装 Node.js ----
$nodeCheck = Get-NodeCommand
if (-not $nodeCheck) {
  Write-Output "⚠️ 未检测到 Node.js，正在自动安装..."
  $wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
  if ($wingetCheck) {
    Write-Output "📥 通过 winget 安装 Node.js LTS..."
    winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements --silent 2>&1
    Start-Sleep -Seconds 2
    Refresh-Path
    $nodeCheck = Get-NodeCommand
    if (-not $nodeCheck) {
      Write-Output "⚠️ winget 安装后仍未检测到 node，尝试从常见目录再次探测..."
      $nodeCheck = Get-NodeCommand
    }
  }

  if (-not $nodeCheck) {
    Write-Output "📥 尝试下载安装 Node.js 官方 MSI..."
    $arch = if ([Environment]::Is64BitOperatingSystem) { "x64" } else { "x86" }
    $nodeVersion = "v22.14.0"
    $msiName = "node-$nodeVersion-$arch.msi"
    $downloadUrls = @(
      "https://npmmirror.com/mirrors/node/$nodeVersion/$msiName",
      "https://nodejs.org/dist/$nodeVersion/$msiName"
    )
    $msiPath = Join-Path $env:TEMP $msiName
    $downloaded = $false

    foreach ($url in $downloadUrls) {
      try {
        Write-Output "🌐 下载: $url"
        Invoke-WebRequest -Uri $url -OutFile $msiPath -UseBasicParsing
        if (Test-Path $msiPath) {
          $downloaded = $true
          break
        }
      } catch {
        Write-Output "⚠️ 下载失败: $url"
      }
    }

    if ($downloaded) {
      $installProc = Start-Process msiexec.exe -ArgumentList @('/i', $msiPath, '/qn', '/norestart') -Wait -PassThru
      if ($installProc.ExitCode -ne 0) {
        Write-Output "⚠️ MSI 安装返回码: $($installProc.ExitCode)"
      }
      Remove-Item $msiPath -Force -ErrorAction SilentlyContinue
      Start-Sleep -Seconds 2
      Refresh-Path
      $nodeCheck = Get-NodeCommand
    }
  }

  if (-not $nodeCheck) {
    if (-not $wingetCheck) {
      Write-Output "❌ 未找到 winget，且官方 Node.js 安装也失败，请手动从 https://nodejs.org 下载安装后重试"
    } else {
      Write-Output "❌ Node.js 自动安装失败，请手动从 https://nodejs.org 下载安装后重试"
    }
    exit 1
  }

  $nodeVersionText = Get-NodeVersionText $nodeCheck
  if ($nodeVersionText) {
    Write-Output "✅ Node.js $nodeVersionText 安装完成"
  } else {
    Write-Output "✅ Node.js 已安装完成"
  }
}

# Configure npm to use Chinese mirror for faster downloads
Write-Output "📝 配置 npm 镜像源..."
npm config set registry https://registry.npmmirror.com 2>$null

# Some OpenClaw dependencies still resolve GitHub repos via git@github.com/ssh URLs.
# Force npm/git to rewrite them to anonymous https URLs during installation.
$env:GIT_CONFIG_COUNT = "2"
$env:GIT_CONFIG_KEY_0 = "url.https://github.com/.insteadof"
$env:GIT_CONFIG_VALUE_0 = "ssh://git@github.com/"
$env:GIT_CONFIG_KEY_1 = "url.https://github.com/.insteadOf"
$env:GIT_CONFIG_VALUE_1 = "git@github.com:"

# Ensure npm global prefix is set to user-accessible path
$npmPrefix = npm config get prefix 2>$null
Write-Output "📁 npm 全局安装目录: $npmPrefix"

# Create npm global directory if it doesn't exist (critical for first-time npm users)
if ($npmPrefix -and -not (Test-Path $npmPrefix)) {
  Write-Output "📁 创建 npm 全局目录: $npmPrefix"
  New-Item -ItemType Directory -Path $npmPrefix -Force | Out-Null
}
$npmModules = Join-Path $npmPrefix "node_modules"
if (-not (Test-Path $npmModules)) {
  Write-Output "📁 创建 node_modules 目录: $npmModules"
  New-Item -ItemType Directory -Path $npmModules -Force | Out-Null
}

Write-Output "📥 正在通过 npm 安装 OpenClaw %s..."
$npmExit = Invoke-NativeStep { npm install -g openclaw@%s --registry=https://registry.npmmirror.com --no-fund --no-audit }
$openclawCmd = Join-Path $npmPrefix "openclaw.cmd"
if ($npmExit -ne 0 -and -not (Test-Path $openclawCmd)) {
  Write-Output "⚠️ 首次安装失败 (exit code: $npmExit)，正在重试..."
  Invoke-NativeStep { npm cache verify } | Out-Null
  $npmExit = Invoke-NativeStep { npm install -g openclaw@%s --registry=https://registry.npmmirror.com --force --no-fund --no-audit }
}
if ($npmExit -ne 0 -and -not (Test-Path $openclawCmd)) {
  Write-Output "❌ OpenClaw 安装失败，请检查网络连接或手动运行: npm install -g openclaw@%s"
  exit 1
}
if ($npmExit -ne 0 -and (Test-Path $openclawCmd)) {
  Write-Output "⚠️ npm 返回退出码 $npmExit，但 openclaw 已安装成功，继续初始化配置"
}

# Refresh PATH so we can find openclaw.cmd
Refresh-Path
# Also add npm global bin to PATH
if ($npmPrefix -and (Test-Path $npmPrefix)) {
  $env:Path = "$npmPrefix;$env:Path"
}

# Verify installation
$ocCmd = Get-Command openclaw -ErrorAction SilentlyContinue
if ($ocCmd) {
  $ocVer = & openclaw --version 2>$null
  Write-Output "✅ OpenClaw $ocVer 安装完成"
} else {
  # Check common locations
  $possiblePaths = @(
    (Join-Path $npmPrefix "openclaw.cmd"),
    (Join-Path $env:APPDATA "npm\openclaw.cmd"),
    "C:\Program Files\nodejs\openclaw.cmd"
  )
  $found = $false
  foreach ($p in $possiblePaths) {
    if (Test-Path $p) {
      Write-Output "✅ OpenClaw 安装完成 (位置: $p)"
      $found = $true
      break
    }
  }
  if (-not $found) {
    Write-Output "⚠️ npm 安装完成但未找到 openclaw 命令，可能需要重启面板"
  }
}

Write-Output "📝 初始化配置..."
# Create basic openclaw.json if it doesn't exist
$openclawDir = Join-Path $env:USERPROFILE ".openclaw"
$openclawConfig = Join-Path $openclawDir "openclaw.json"
if (-not (Test-Path $openclawDir)) {
  New-Item -ItemType Directory -Path $openclawDir -Force | Out-Null
}
if (-not (Test-Path $openclawConfig)) {
  Write-Output "📝 创建基础配置文件..."
  $ocVer = & openclaw --version 2>$null
  if (-not $ocVer) { $ocVer = "2026.3.2" }
  $workDir = Join-Path $env:USERPROFILE "work"
  @"
{
  "meta": {
    "version": "$ocVer",
    "lastTouchedVersion": "$ocVer"
  },
  "gateway": {
    "mode": "local",
    "port": 19000
  },
  "workspace": {
    "path": "$($workDir -replace '\\', '\\')"
  }
}
"@ | Set-Content $openclawConfig -Force
  Write-Output "✅ 配置文件已创建: $openclawConfig"
}
if (Test-Path $openclawCmd) {
  $env:OPENCLAW_DIR = $openclawDir
  $env:OPENCLAW_STATE_DIR = $openclawDir
  $env:OPENCLAW_CONFIG_PATH = $openclawConfig
  $initExit = Invoke-NativeStep { & $openclawCmd init }
  if ($initExit -ne 0) {
    Write-Output "⚠️ OpenClaw 初始化返回退出码 $initExit，通常稍后网关仍会继续拉起"
  }
}
Write-Output "ℹ️ 初次安装后，网关状态同步可能需要 10-30 秒"
Write-Output "✅ 全部完成"
`, pinnedOpenClawVersion, pinnedOpenClawVersion, pinnedOpenClawVersion, pinnedOpenClawVersion)
			} else {
				script = fmt.Sprintf(`
set -e
echo "📦 安装 OpenClaw..."
export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"

# --- Helper: install Node.js v22 from official binary archive (Linux/macOS) ---
install_node_tarball() {
  local NODE_VER="v22.14.0"
  local OS_NAME
  local PKG_EXT
  local ARCH
  OS_NAME=$(uname | tr '[:upper:]' '[:lower:]')

  case "$(uname -m)" in
    x86_64|amd64) ARCH="x64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="armv7l" ;;
    *) echo "❌ 不支持的 CPU 架构: $(uname -m)"; exit 1 ;;
  esac

  if [ "$OS_NAME" = "darwin" ]; then
    if [ "$ARCH" = "armv7l" ]; then
      echo "❌ macOS 不支持 armv7l 架构"; exit 1
    fi
    PKG_EXT="tar.gz"
  else
    PKG_EXT="tar.xz"
  fi

  local FILE="node-${NODE_VER}-${OS_NAME}-${ARCH}.${PKG_EXT}"
  local URL="https://nodejs.org/dist/${NODE_VER}/${FILE}"
  local MIRROR_URL="https://npmmirror.com/mirrors/node/${NODE_VER}/${FILE}"
  echo "📥 下载 Node.js ${NODE_VER} (${OS_NAME}-${ARCH}) 官方二进制..."
  local TMP_DIR=$(mktemp -d)
  # Try China mirror first, then official
  curl -fsSL "$MIRROR_URL" -o "$TMP_DIR/node.tar" 2>/dev/null || \
  curl -fsSL "$URL" -o "$TMP_DIR/node.tar" || {
    echo "❌ Node.js 下载失败"
    rm -rf "$TMP_DIR"
    exit 1
  }
  echo "📦 解压到 /usr/local ..."
  if [ "$PKG_EXT" = "tar.gz" ]; then
    tar -xzf "$TMP_DIR/node.tar" -C /usr/local --strip-components=1
  else
    tar -xJf "$TMP_DIR/node.tar" -C /usr/local --strip-components=1
  fi
  rm -rf "$TMP_DIR"
  hash -r
  echo "✅ Node.js $(node --version) 安装完成 (官方二进制)"
}

# --- Helper: check if node version >= 20 ---
node_version_ok() {
  local ver
  ver=$(node --version 2>/dev/null | sed 's/^v//')
  [ -z "$ver" ] && return 1
  local major
  major=$(echo "$ver" | cut -d. -f1)
  [ "$major" -ge 20 ] 2>/dev/null
}

# 1. Ensure Node.js >= 20 is available
if ! node_version_ok; then
  if command -v node &>/dev/null; then
    echo "⚠️ 当前 Node.js $(node --version) 版本过低 (需要 >= 20)，正在升级..."
  else
    echo "⚠️ 未检测到 Node.js，正在自动安装..."
  fi

  if [ "$(uname)" = "Darwin" ]; then
    install_node_tarball
  else
    # Try NodeSource first, then fallback to binary tarball
    DISTRO_FAMILY=""
    if [ -f /etc/os-release ]; then
      . /etc/os-release
      case "$ID $ID_LIKE" in
        *debian*|*ubuntu*) DISTRO_FAMILY="debian" ;;
        *rhel*|*centos*|*fedora*|*amzn*|*rocky*|*alma*) DISTRO_FAMILY="rhel" ;;
      esac
    fi
    [ -z "$DISTRO_FAMILY" ] && command -v apt-get &>/dev/null && DISTRO_FAMILY="debian"
    [ -z "$DISTRO_FAMILY" ] && (command -v dnf &>/dev/null || command -v yum &>/dev/null) && DISTRO_FAMILY="rhel"

    NODESOURCE_OK=false
    case "$DISTRO_FAMILY" in
      debian)
        echo "📥 尝试 NodeSource (deb)..."
        if curl -fsSL https://deb.nodesource.com/setup_22.x | bash - 2>/dev/null; then
          apt-get install -y nodejs && NODESOURCE_OK=true
        fi
        ;;
      rhel)
        echo "📥 尝试 NodeSource (rpm)..."
        if curl -fsSL https://rpm.nodesource.com/setup_22.x | bash - 2>/dev/null; then
          if command -v dnf &>/dev/null; then
            dnf install -y nodejs && NODESOURCE_OK=true
          else
            yum install -y nodejs && NODESOURCE_OK=true
          fi
        fi
        ;;
    esac

    # If NodeSource didn't work or gave old version, use binary tarball
    if ! node_version_ok; then
      echo "⚠️ 包管理器未能提供 Node.js >= 20，使用官方二进制安装..."
      install_node_tarball
    fi
  fi

  # Final check
  if ! node_version_ok; then
    echo "❌ Node.js >= 20 安装失败，请手动安装后重试"
    echo "   参考: https://nodejs.org/en/download/"
    exit 1
  fi
fi
echo "✅ Node.js $(node --version) ready"

# 2. Ensure npm mirror is set
npm config set registry https://registry.npmmirror.com 2>/dev/null || true

# Some OpenClaw dependencies still resolve GitHub repos via git@github.com/ssh URLs.
# Force npm/git to rewrite them to anonymous https URLs during installation.
export GIT_CONFIG_COUNT=2
export GIT_CONFIG_KEY_0="url.https://github.com/.insteadof"
export GIT_CONFIG_VALUE_0="ssh://git@github.com/"
export GIT_CONFIG_KEY_1="url.https://github.com/.insteadOf"
export GIT_CONFIG_VALUE_1="git@github.com:"

# 3. Install OpenClaw
install_openclaw_offline() {
  local arch
  case "$(uname -m)" in
    x86_64|amd64) arch="x64" ;;
    *) return 1 ;;
  esac

  local base_url="http://39.102.53.188:16198/clawpanel/bin/openclaw"
  local pkg="openclaw-%s-linux-${arch}-prefix.tar.gz"
  local tmp_dir
  tmp_dir=$(mktemp -d)
  local pkg_path="$tmp_dir/$pkg"

  echo "📦 尝试从加速服务器安装 OpenClaw %s 离线包..."
  if ! curl -fsSL --max-time 600 "$base_url/$pkg" -o "$pkg_path"; then
    rm -rf "$tmp_dir"
    return 1
  fi

  mkdir -p /usr/local/lib
  rm -rf "/usr/local/lib/openclaw-%s"
  mkdir -p "/usr/local/lib/openclaw-%s"
  tar -xzf "$pkg_path" -C "/usr/local/lib/openclaw-%s"
  ln -sfn "/usr/local/lib/openclaw-%s/node_modules/.bin/openclaw" /usr/local/bin/openclaw
  rm -rf "$tmp_dir"
  hash -r
  return 0
}

if install_openclaw_offline; then
  echo "✅ OpenClaw $(openclaw --version 2>/dev/null || echo '已安装') 安装完成 (离线包)"
else
  echo "📥 正在通过 npm 安装 OpenClaw %s..."
  npm install -g openclaw@%s --registry=https://registry.npmmirror.com
fi
echo "✅ OpenClaw $(openclaw --version 2>/dev/null || echo '已安装') 安装完成"

# 4. Initialize config
echo "📝 初始化配置..."
openclaw init 2>/dev/null || true

echo "✅ 全部完成"
`, pinnedOpenClawVersion, pinnedOpenClawVersion, pinnedOpenClawVersion, pinnedOpenClawVersion, pinnedOpenClawVersion, pinnedOpenClawVersion, pinnedOpenClawVersion, pinnedOpenClawVersion)
			}
		case "napcat":
			taskName = "安装 NapCat (QQ个人号)"
			if runtime.GOOS == "windows" {
				script = buildNapCatWindowsInstallScript(cfg)
			} else {
				script = buildNapCatInstallScript(cfg)
			}

		case "wechat":
			taskName = "安装微信机器人"
			script = buildWeChatInstallScript(cfg)

		default:
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "不支持的软件: " + req.Software})
			return
		}

		task := tm.CreateTask(taskName, "install_"+req.Software)

		go func() {
			var err error
			if sudoPass != "" && runtime.GOOS != "windows" {
				// Linux/macOS installs need sudo (including OpenClaw which auto-installs Node.js)
				err = tm.RunScriptWithSudo(task, sudoPass, script)
			} else {
				err = tm.RunScript(task, script)
			}
			tm.FinishTask(task, err)
		}()

		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID})
	}
}

// GetTasks 获取任务列表
func GetTasks(tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tasks := tm.GetRecentTasks()
		c.JSON(http.StatusOK, gin.H{"ok": true, "tasks": tasks})
	}
}

// GetTaskDetail 获取任务详情
func GetTaskDetail(tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		task := tm.GetTask(id)
		if task == nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "任务不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "task": task})
	}
}

func getSudoPass(cfg *config.Config) string {
	spPath := filepath.Join(cfg.DataDir, "sudo-password.txt")
	data, err := os.ReadFile(spPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func buildNapCatInstallScript(cfg *config.Config) string {
	_, wsToken, _ := cfg.ReadQQChannelState()
	wsTokenB64 := base64.StdEncoding.EncodeToString([]byte(wsToken))
	qqPluginDir := qqPluginInstallPath(cfg)

	return fmt.Sprintf(`
set -e
export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
echo "📦 安装 NapCat (QQ个人号) Docker 容器..."

QQ_PLUGIN_DIR=%q
if [ ! -d "$QQ_PLUGIN_DIR" ]; then
  echo "📥 安装 QQ (OneBot11) 通道插件..."
  mkdir -p "$(dirname "$QQ_PLUGIN_DIR")"
  QQ_TGZ="$(mktemp /tmp/qq-plugin.XXXXXX.tgz)"
  if curl -fsSL --max-time 60 "http://39.102.53.188:16198/clawpanel/bin/qq-plugin.tgz" -o "$QQ_TGZ"; then
    tar -xzf "$QQ_TGZ" -C "$(dirname "$QQ_PLUGIN_DIR")"
    chown -R root:root "$QQ_PLUGIN_DIR" 2>/dev/null || true
    rm -f "$QQ_TGZ"
  else
    rm -f "$QQ_TGZ"
    echo "❌ QQ 个人号插件安装失败，无法继续安装 NapCat"
    exit 1
  fi
fi

if [ ! -d "$QQ_PLUGIN_DIR" ]; then
  echo "❌ QQ 个人号插件未安装，无法继续安装 NapCat"
  exit 1
fi

# Auto-install Docker if missing
if ! command -v docker &>/dev/null; then
  echo "⚠️ 未检测到 Docker，正在自动安装..."
  DISTRO_FAMILY=""
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    case "$ID $ID_LIKE" in
      *debian*|*ubuntu*) DISTRO_FAMILY="debian" ;;
      *rhel*|*centos*|*fedora*|*amzn*|*rocky*|*alma*|*opencloudos*|*tencentos*) DISTRO_FAMILY="rhel" ;;
    esac
  fi
  [ -z "$DISTRO_FAMILY" ] && command -v apt-get &>/dev/null && DISTRO_FAMILY="debian"
  [ -z "$DISTRO_FAMILY" ] && (command -v dnf &>/dev/null || command -v yum &>/dev/null) && DISTRO_FAMILY="rhel"

  case "$DISTRO_FAMILY" in
    debian)
      echo "📥 通过阿里云镜像安装 Docker (deb)..."
      apt-get update
      apt-get install -y ca-certificates curl gnupg
      install -m 0755 -d /etc/apt/keyrings
      curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg 2>/dev/null || true
      chmod a+r /etc/apt/keyrings/docker.gpg
      echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME" || echo "jammy") stable" > /etc/apt/sources.list.d/docker.list
      apt-get update
      apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      ;;
    rhel)
      echo "📥 通过阿里云镜像安装 Docker (rpm)..."
      if command -v dnf &>/dev/null; then
        dnf install -y dnf-plugins-core
        dnf config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo 2>/dev/null || true
        dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      else
        yum install -y yum-utils
        yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
        yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      fi
      ;;
    *)
      echo "❌ 不支持的系统发行版，请手动安装 Docker 后重试"
      echo "   参考: https://docs.docker.com/engine/install/"
      exit 1
      ;;
  esac

  systemctl enable docker
  systemctl start docker
  echo "✅ Docker $(docker --version) 安装完成"
fi

if ! docker info &>/dev/null; then
  echo "⚠️ Docker 服务未运行，正在启动..."
  if [ "$(uname)" = "Darwin" ]; then
    open -a Docker 2>/dev/null || true
    for i in $(seq 1 30); do
      if docker info &>/dev/null; then
        break
      fi
      sleep 2
    done
  else
    systemctl start docker
    sleep 2
  fi
fi

# Configure Docker mirror (Linux only)
if [ "$(uname)" = "Darwin" ]; then
  echo "ℹ️ macOS 跳过 /etc/docker/daemon.json 镜像配置"
else
  if [ ! -f /etc/docker/daemon.json ] || ! grep -q "registry-mirrors" /etc/docker/daemon.json 2>/dev/null; then
    echo "🔧 配置 Docker 镜像加速器..."
    mkdir -p /etc/docker
    cat > /etc/docker/daemon.json << 'DOCKEREOF'
{
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me"
  ]
}
DOCKEREOF
    systemctl daemon-reload
    systemctl restart docker
    sleep 2
  fi
fi

if ! docker info &>/dev/null; then
  echo "❌ Docker 服务仍未就绪，请手动启动 Docker Desktop 后重试"
  exit 1
fi

# Check if already exists
if docker inspect openclaw-qq &>/dev/null; then
  echo "⚠️ openclaw-qq 容器已存在，正在重新创建..."
  docker stop openclaw-qq 2>/dev/null || true
  docker rm openclaw-qq 2>/dev/null || true
fi

echo "📥 拉取 NapCat 镜像..."
docker pull mlikiowa/napcat-docker:latest

# Detect existing logged-in account for quick re-login
ACCOUNT_ARG=""
if docker inspect openclaw-qq &>/dev/null; then
  PREV_ACCOUNT=$(docker inspect openclaw-qq --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null | grep '^ACCOUNT=' | cut -d= -f2)
  if [ -n "$PREV_ACCOUNT" ]; then
    ACCOUNT_ARG="-e ACCOUNT=$PREV_ACCOUNT"
    echo "� 使用快速登录账号: $PREV_ACCOUNT"
  fi
fi

echo "� 创建容器..."
docker run -d \
  --name openclaw-qq \
  --restart unless-stopped \
  -p 3000:3000 \
  -p 3001:3001 \
  -p 6099:6099 \
  -e NAPCAT_GID=0 \
  -e NAPCAT_UID=0 \
  -e WEBUI_TOKEN=clawpanel-qq \
  $ACCOUNT_ARG \
  -v napcat-qq-session:/app/.config/QQ \
  -v napcat-config:/app/napcat/config \
  -v %s:/root/.openclaw:rw \
  -v %s:/root/openclaw/work:rw \
  mlikiowa/napcat-docker:latest

echo "⏳ 等待容器启动..."
sleep 5

# Configure OneBot11 WebSocket + HTTP
echo "🔧 配置 OneBot11 (WS + HTTP)..."
export WS_TOKEN_B64=%q
WS_TOKEN_JSON=$(python3 - <<'PY'
import base64, json, os
token = base64.b64decode(os.environ.get("WS_TOKEN_B64", "")).decode("utf-8")
print(json.dumps(token), end="")
PY
)

docker exec openclaw-qq bash -c "cat > /app/napcat/config/onebot11.json << OBEOF
{
  \"network\": {
    \"websocketServers\": [{
      \"name\": \"ws-server\",
      \"enable\": true,
      \"host\": \"0.0.0.0\",
      \"port\": 3001,
      \"token\": ${WS_TOKEN_JSON},
      \"reportSelfMessage\": true,
      \"enableForcePushEvent\": true,
      \"messagePostFormat\": \"array\",
      \"debug\": false,
      \"heartInterval\": 30000
    }],
    \"httpServers\": [{
      \"name\": \"http-api\",
      \"enable\": true,
      \"host\": \"0.0.0.0\",
      \"port\": 3000,
      \"token\": \"\"
    }],
    \"httpSseServers\": [],
    \"httpClients\": [],
    \"websocketClients\": [],
    \"plugins\": []
  },
  \"musicSignUrl\": \"\",
  \"enableLocalFile2Url\": true,
  \"parseMultMsg\": true,
  \"imageDownloadProxy\": \"\"
}
OBEOF"

# Configure WebUI
docker exec openclaw-qq bash -c 'cat > /app/napcat/config/webui.json << WUEOF
{
  "host": "0.0.0.0",
  "port": 6099,
  "token": "clawpanel-qq",
  "loginRate": 3
}
WUEOF'

echo "🔄 重启容器使配置生效..."
docker restart openclaw-qq

echo "⏳ 等待 NapCat 服务启动..."
for i in $(seq 1 30); do
  if curl -s -o /dev/null -w '' http://127.0.0.1:6099 2>/dev/null; then
    echo "✅ NapCat WebUI (6099) 已就绪"
    break
  fi
  sleep 2
done

# Verify ports
WS_OK=false
HTTP_OK=false
for i in $(seq 1 10); do
  if bash -c 'echo > /dev/tcp/127.0.0.1/3001' 2>/dev/null; then WS_OK=true; fi
  if bash -c 'echo > /dev/tcp/127.0.0.1/3000' 2>/dev/null; then HTTP_OK=true; fi
  if $WS_OK && $HTTP_OK; then break; fi
  sleep 2
done

if $WS_OK; then
  echo "✅ OneBot11 WebSocket (3001) 已就绪"
else
  echo "⚠️ OneBot11 WebSocket (3001) 尚未就绪，可能需要先扫码登录QQ"
fi
if $HTTP_OK; then
  echo "✅ OneBot11 HTTP API (3000) 已就绪"
else
  echo "⚠️ OneBot11 HTTP API (3000) 尚未就绪，可能需要先扫码登录QQ"
fi

echo "✅ NapCat (QQ个人号) 安装完成"
echo "📝 请在通道管理中扫码登录 QQ"
`, qqPluginDir, cfg.OpenClawDir, cfg.OpenClawWork, wsTokenB64)
}

// getNapCatShellDir returns the NapCat Shell installation directory on Windows, or "" if not found
func getNapCatShellDir(cfg *config.Config) string {
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
		// Recursive walk to find napcat.bat or NapCatWinBootMain.exe anywhere under candidate
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

// isNapCatShellRunning checks if a NapCat Shell process is running on Windows
func isNapCatShellRunning() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	out := detectCmd("tasklist", "/FI", "IMAGENAME eq NapCatWinBootMain.exe", "/NH")
	if strings.Contains(out, "NapCatWinBootMain") {
		return true
	}
	// Also check for napcat.exe process
	out2 := detectCmd("tasklist", "/FI", "IMAGENAME eq napcat.exe", "/NH")
	return strings.Contains(out2, "napcat.exe")
}

// buildNapCatWindowsInstallScript builds a PowerShell script to install NapCat Shell on Windows
func buildNapCatWindowsInstallScript(cfg *config.Config) string {
	installDir := filepath.Join(cfg.DataDir, "napcat")
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 NapCat (QQ个人号) Windows Shell 版本..."
Write-Output "无需 Docker，直接运行 NapCat Shell"

$INSTALL_DIR = "%s"
$OPENCLAW_DIR = "%s"
$QQ_PLUGIN_DIR = Join-Path $OPENCLAW_DIR "extensions\qq"
if (-not (Test-Path $INSTALL_DIR)) {
  New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
}

if (-not (Test-Path $QQ_PLUGIN_DIR)) {
  Write-Output "📥 安装 QQ (OneBot11) 通道插件..."
  New-Item -ItemType Directory -Force -Path (Split-Path $QQ_PLUGIN_DIR -Parent) | Out-Null
  $QQ_TGZ = Join-Path $env:TEMP "qq-plugin.tgz"
  try {
    Invoke-WebRequest -Uri "http://39.102.53.188:16198/clawpanel/bin/qq-plugin.tgz" -OutFile $QQ_TGZ -UseBasicParsing -TimeoutSec 60
    tar -xzf $QQ_TGZ -C (Split-Path $QQ_PLUGIN_DIR -Parent)
  } catch {
    Write-Output "❌ QQ 个人号插件安装失败，无法继续安装 NapCat"
    exit 1
  } finally {
    Remove-Item -Force $QQ_TGZ -ErrorAction SilentlyContinue
  }
}

if (-not (Test-Path $QQ_PLUGIN_DIR)) {
  Write-Output "❌ QQ 个人号插件未安装，无法继续安装 NapCat"
  exit 1
}

Write-Output "📥 下载 NapCat Shell Windows OneKey 版..."
$NAPCAT_ZIP = Join-Path $INSTALL_DIR "napcat-onekey.zip"
$urls = @(
  "https://github.com/NapNeko/NapCatQQ/releases/latest/download/NapCat.Shell.Windows.OneKey.zip",
  "https://ghfast.top/https://github.com/NapNeko/NapCatQQ/releases/latest/download/NapCat.Shell.Windows.OneKey.zip"
)
$downloaded = $false
foreach ($url in $urls) {
  try {
    Write-Output "尝试下载: $url"
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $url -OutFile $NAPCAT_ZIP -UseBasicParsing -TimeoutSec 120
    $downloaded = $true
    Write-Output "✅ 下载完成"
    break
  } catch {
    Write-Output "⚠️ 下载失败: $_"
  }
}
if (-not $downloaded) {
  Write-Output "❌ 所有下载源均失败，请手动从 GitHub 下载 NapCat.Shell.Windows.OneKey.zip"
  exit 1
}

Write-Output "📦 解压 NapCat Shell..."
Expand-Archive -Force -Path $NAPCAT_ZIP -DestinationPath $INSTALL_DIR
Remove-Item -Force $NAPCAT_ZIP -ErrorAction SilentlyContinue

# Find the NapCat Shell directory (could be nested)
$shellDir = ""
$batFiles = Get-ChildItem -Path $INSTALL_DIR -Recurse -Filter "napcat.bat" -ErrorAction SilentlyContinue
if ($batFiles) {
  $shellDir = $batFiles[0].DirectoryName
} else {
  $exeFiles = Get-ChildItem -Path $INSTALL_DIR -Recurse -Filter "NapCatWinBootMain.exe" -ErrorAction SilentlyContinue
  if ($exeFiles) {
    $shellDir = $exeFiles[0].DirectoryName
  }
}

if ($shellDir -ne "") {
  Write-Output "🔧 配置 OneBot11 (WS + HTTP)..."
  $configDir = Join-Path $shellDir "config"
  if (-not (Test-Path $configDir)) {
    New-Item -ItemType Directory -Force -Path $configDir | Out-Null
  }

  $onebot11 = @'
{
  "network": {
    "websocketServers": [{
      "name": "ws-server",
      "enable": true,
      "host": "0.0.0.0",
      "port": 3001,
      "token": "",
      "reportSelfMessage": true,
      "enableForcePushEvent": true,
      "messagePostFormat": "array",
      "debug": false,
      "heartInterval": 30000
    }],
    "httpServers": [{
      "name": "http-api",
      "enable": true,
      "host": "0.0.0.0",
      "port": 3000,
      "token": ""
    }],
    "httpSseServers": [],
    "httpClients": [],
    "websocketClients": [],
    "plugins": []
  },
  "musicSignUrl": "",
  "enableLocalFile2Url": true,
  "parseMultMsg": true,
  "imageDownloadProxy": ""
}
'@
  $onebot11 | Set-Content -Path (Join-Path $configDir "onebot11.json") -Encoding UTF8

  $webui = @'
{
  "host": "0.0.0.0",
  "port": 6099,
  "token": "clawpanel-qq",
  "loginRate": 3
}
'@
  $webui | Set-Content -Path (Join-Path $configDir "webui.json") -Encoding UTF8

  # Start NapCat Shell process
  Write-Output "🚀 启动 NapCat Shell..."
  $batFile = Join-Path $shellDir "napcat.bat"
  $exeFile = Join-Path $shellDir "NapCatWinBootMain.exe"
  if (Test-Path $batFile) {
    $q = [char]34
    $batArgs = "/c " + $q + $batFile + $q
    Start-Process -FilePath "cmd.exe" -ArgumentList $batArgs -WorkingDirectory $shellDir -WindowStyle Hidden
    Write-Output "✅ NapCat Shell 已通过 napcat.bat 启动"
  } elseif (Test-Path $exeFile) {
    Start-Process -FilePath $exeFile -WorkingDirectory $shellDir -WindowStyle Hidden
    Write-Output "✅ NapCat Shell 已通过 NapCatWinBootMain.exe 启动"
  } else {
    Write-Output "⚠️ 未找到启动文件，请手动启动 NapCat Shell"
  }
  Start-Sleep -Seconds 3

  Write-Output "✅ NapCat Shell (Windows) 安装完成"
  Write-Output "📁 安装目录: $shellDir"
  Write-Output "📝 请在通道管理中配置 QQ 并扫码登录"
} else {
  Write-Output "⚠️ NapCat 已下载但未找到启动文件，请手动检查: $INSTALL_DIR"
}
`, installDir, cfg.OpenClawDir)
}

func buildWeChatInstallScript(cfg *config.Config) string {
	kitDir := wechatBridgeKitDir(cfg)
	packageJSON := `{
  "name": "clawpanel-wechat-wcf-bridge",
  "private": true,
  "type": "module",
  "version": "0.1.0",
  "description": "ClawPanel private WeChatFerry bridge",
  "scripts": {
    "start": "node bridge.mjs"
  },
  "dependencies": {
    "@wechatferry/agent": "^0.0.26",
    "file-box": "^1.4.15"
  }
}`
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/api/wechat/callback", cfg.Port)
	bridgeJS := `import http from "node:http";
import { Buffer } from "node:buffer";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { Agent } from "@wechatferry/agent";
import { FileBox } from "file-box";

const port = Number(process.env.WCF_BRIDGE_PORT || 19088);
const token = String(process.env.WCF_BRIDGE_TOKEN || "clawpanel-wcf").trim();
const callbackUrl = String(process.env.PANEL_CALLBACK_URL || "__PANEL_CALLBACK_URL__").trim();
const callbackToken = String(process.env.PANEL_CALLBACK_TOKEN || token).trim();
const agent = new Agent();
const state = {
  connected: false,
  loggedIn: false,
  name: "",
  selfWxid: "",
  contacts: 0,
  rooms: 0,
  version: "WeChatFerry Bridge",
  message: "等待微信登录",
  lastError: "",
};

async function refreshState() {
  try {
    const contacts = await agent.getContactList();
    const rooms = await agent.getChatRoomList();
    state.contacts = Array.isArray(contacts) ? contacts.length : 0;
    state.rooms = Array.isArray(rooms) ? rooms.length : 0;
  } catch {}
}

agent.on("login", async (user) => {
  state.connected = true;
  state.loggedIn = true;
  state.name = user?.name || user?.wxid || "已登录";
  state.selfWxid = user?.wxid || "";
  state.message = "微信已登录";
  state.lastError = "";
  await refreshState();
});

agent.on("logout", () => {
  state.loggedIn = false;
  state.name = "";
  state.selfWxid = "";
  state.contacts = 0;
  state.rooms = 0;
  state.message = "微信已退出";
});

async function postCallback(payload) {
  if (!callbackUrl) return;
  const headers = { "Content-Type": "application/json" };
  if (callbackToken) {
    headers.Authorization = "Bearer " + callbackToken;
  }
  try {
    await fetch(callbackUrl, {
      method: "POST",
      headers,
      body: JSON.stringify(payload),
    });
  } catch (err) {
    state.lastError = err instanceof Error ? err.message : String(err || "callback failed");
  }
}

agent.on("message", async (msg) => {
  state.connected = true;
  if (!state.message || state.message == "ç­‰å¾…å¾®ä¿¡ç™»å½•") {
    state.message = "æ¡¥æŽ¥è¿è¡Œä¸­";
  }
  const talker = String(msg?.roomid || msg?.roomId || msg?.strTalker || msg?.talker || msg?.from || "");
  const sender = String(msg?.sender || msg?.senderWxid || msg?.fromWxid || msg?.from || "");
  const content = String(msg?.content || msg?.strContent || msg?.text || "");
  const isRoom = Boolean(msg?.roomid || msg?.roomId || String(msg?.strTalker || "").endsWith("@chatroom"));
  const isSelf = Boolean(msg?.isSelf || msg?.isSender || msg?.self);
  await postCallback({
    event: "message",
    talker,
    sender: sender || (!isRoom ? talker : ""),
    content,
    isRoom,
    isSelf,
    raw: msg,
  });
});

agent.on("error", (err) => {
  state.connected = false;
  state.lastError = err instanceof Error ? err.message : String(err || "unknown error");
  state.message = state.lastError || "桥接异常";
});

function sendJson(res, status, payload) {
  const body = JSON.stringify(payload);
  res.writeHead(status, {
    "Content-Type": "application/json; charset=utf-8",
    "Content-Length": Buffer.byteLength(body),
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Headers": "Content-Type, Authorization",
    "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
  });
  res.end(body);
}

function unauthorized(res) {
  sendJson(res, 401, { ok: false, error: "unauthorized" });
}

function collectJson(req) {
  return new Promise((resolve, reject) => {
    let raw = "";
    req.on("data", chunk => { raw += chunk; });
    req.on("end", () => {
      if (!raw.trim()) {
        resolve({});
        return;
      }
      try {
        resolve(JSON.parse(raw));
      } catch (err) {
        reject(err);
      }
    });
    req.on("error", reject);
  });
}

async function downloadToTemp(fileUrl, fileName) {
  const res = await fetch(fileUrl);
  if (!res.ok) throw new Error("下载文件失败: " + res.status + " " + res.statusText);
  const buf = Buffer.from(await res.arrayBuffer());
  const ext = path.extname(fileName || new URL(fileUrl).pathname) || ".bin";
  const tempPath = path.join(os.tmpdir(), "clawpanel-wcf-" + Date.now() + ext);
  await fs.writeFile(tempPath, buf);
  return tempPath;
}

const server = http.createServer(async (req, res) => {
  if (req.method === "OPTIONS") {
    sendJson(res, 200, { ok: true });
    return;
  }
  if (token) {
    const auth = String(req.headers.authorization || "");
    if (auth !== "Bearer " + token) {
      unauthorized(res);
      return;
    }
  }
  try {
    if (req.method === "GET" && req.url === "/status") {
      sendJson(res, 200, { ok: true, ...state });
      return;
    }
    if (req.method === "POST" && req.url === "/send/text") {
      const body = await collectJson(req);
      if (!body.to || !body.content) throw new Error("to and content are required");
      await agent.sendText(body.to, body.content, Array.isArray(body.mentionIdList) ? body.mentionIdList : []);
      sendJson(res, 200, { ok: true });
      return;
    }
    if (req.method === "POST" && req.url === "/send/file") {
      const body = await collectJson(req);
      if (!body.to || !body.fileUrl) throw new Error("to and fileUrl are required");
      const tempPath = await downloadToTemp(body.fileUrl, body.fileName || "");
      try {
        await agent.sendFile(body.to, FileBox.fromLocal(tempPath));
      } finally {
        await fs.unlink(tempPath).catch(() => {});
      }
      sendJson(res, 200, { ok: true });
      return;
    }
    sendJson(res, 404, { ok: false, error: "not found" });
  } catch (err) {
    sendJson(res, 500, { ok: false, error: err instanceof Error ? err.message : String(err || "unknown error") });
  }
});

agent.start();
state.connected = true;
state.message = "桥接已启动，等待微信登录";
server.listen(port, "0.0.0.0", () => {
  console.log("[ClawPanel WCF] bridge listening on " + port);
});
`
	installPS1 := `$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root
Write-Host "[1/5] Checking Node.js..."
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
  throw "Please install Node.js 20+ on Windows first: https://nodejs.org/"
}
Write-Host "[2/5] Installing dependencies..."
npm install
if ($LASTEXITCODE -ne 0) {
  throw "npm install failed with exit code $LASTEXITCODE"
}
Write-Host "[3/5] Writing .env..."
if (-not (Test-Path (Join-Path $Root ".env"))) {
  @"
WCF_BRIDGE_PORT=19088
WCF_BRIDGE_TOKEN=clawpanel-wcf
PANEL_CALLBACK_URL=__PANEL_CALLBACK_URL__
PANEL_CALLBACK_TOKEN=clawpanel-wcf
"@ | Set-Content -Encoding UTF8 (Join-Path $Root ".env")
}
Write-Host "[4/5] Make sure Windows WeChat is logged in."
Write-Host "[5/5] Done. Run start-bridge.bat to launch the bridge."
`
	startBAT := `@echo off
cd /d %~dp0
if exist .env (
  for /f "usebackq tokens=1,2 delims==" %%a in (".env") do (
    set %%a=%%b
  )
)
node bridge.mjs
pause
`
	readme := fmt.Sprintf(`ClawPanel 私有 WeChatFerry Bridge

1. 这套桥接必须运行在 Windows 宿主机，不跑在 WSL。
2. 宿主机先安装 Node.js 20+，并保持 Windows 桌面微信已登录。
3. 在此目录打开 PowerShell，执行：
   powershell -ExecutionPolicy Bypass -File .\install-windows.ps1
4. 然后双击 start-bridge.bat，或在 PowerShell 执行：
   node bridge.mjs
5. 回到 ClawPanel 系统配置，桥接地址填写：
   http://127.0.0.1:19088
6. 如需让 WSL 面板访问 Windows，可改成宿主机实际 IP，例如：
   http://<Windows-IP>:19088

当前桥接目录：%s
`, kitDir)
	return fmt.Sprintf(`
set -e
KIT_DIR=%q
mkdir -p "$KIT_DIR"
cat > "$KIT_DIR/package.json" <<'EOF'
%s
EOF
cat > "$KIT_DIR/bridge.mjs" <<'EOF'
%s
EOF
cat > "$KIT_DIR/install-windows.ps1" <<'EOF'
%s
EOF
cat > "$KIT_DIR/start-bridge.bat" <<'EOF'
%s
EOF
cat > "$KIT_DIR/README.txt" <<'EOF'
%s
EOF
	printf "WCF_BRIDGE_PORT=19088\nWCF_BRIDGE_TOKEN=clawpanel-wcf\nPANEL_CALLBACK_URL=%s\nPANEL_CALLBACK_TOKEN=clawpanel-wcf\n" > "$KIT_DIR/.env.example"
chmod 0644 "$KIT_DIR/package.json" "$KIT_DIR/bridge.mjs" "$KIT_DIR/install-windows.ps1" "$KIT_DIR/start-bridge.bat" "$KIT_DIR/README.txt" "$KIT_DIR/.env.example"
echo "✅ 已生成 WeChatFerry Bridge 安装包"
echo "📁 目录: $KIT_DIR"
echo "➡️ 请把这个目录复制到 Windows 宿主机后执行 install-windows.ps1"
`, kitDir, packageJSON, strings.ReplaceAll(bridgeJS, "__PANEL_CALLBACK_URL__", callbackURL), strings.ReplaceAll(installPS1, "__PANEL_CALLBACK_URL__", callbackURL), startBAT, readme, callbackURL)
}
