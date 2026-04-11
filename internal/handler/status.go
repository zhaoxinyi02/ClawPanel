package handler

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/buildinfo"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/monitor"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
)

var startTime = time.Now()

// GetStatus 获取系统状态总览
func GetStatus(db *sql.DB, cfg *config.Config, procMgr *process.Manager, napcatMon *monitor.NapCatMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		ocConfig, _ := cfg.ReadOpenClawJSON()
		injectWecomVirtualChannel(cfg, ocConfig)
		gatewayPort := cfg.DefaultGatewayPort()
		if p := procMgr.GatewayPortInt(); p > 0 {
			gatewayPort = p
		}

		// 提取已启用的通道
		channelLabels := map[string]string{
			"qq":              "QQ (NapCat)",
			"wechat":          "微信",
			"whatsapp":        "WhatsApp",
			"telegram":        "Telegram",
			"discord":         "Discord",
			"irc":             "IRC",
			"slack":           "Slack",
			"signal":          "Signal",
			"googlechat":      "Google Chat",
			"bluebubbles":     "BlueBubbles",
			"imessage":        "iMessage",
			"webchat":         "WebChat",
			"feishu":          "飞书 / Lark",
			"qqbot":           "QQ 官方机器人",
			"dingtalk":        "钉钉",
			"wecom":           "企业微信（智能机器人）",
			"wecom-app":       "企业微信（自建应用）",
			"msteams":         "Microsoft Teams",
			"mattermost":      "Mattermost",
			"line":            "LINE",
			"matrix":          "Matrix",
			"nextcloud-talk":  "Nextcloud Talk",
			"nostr":           "Nostr",
			"qa-channel":      "QA Channel",
			"synology-chat":   "Synology Chat",
			"tlon":            "Tlon",
			"twitch":          "Twitch",
			"voice-call":      "Voice Call",
			"zalo":            "Zalo",
			"zalouser":        "Zalo User",
			"openclaw-weixin": "微信（ClawBot）",
		}

		type enabledChannel struct {
			ID    string `json:"id"`
			Label string `json:"label"`
			Type  string `json:"type"`
		}
		var channels []enabledChannel

		if ocConfig != nil {
			// 扫描 channels
			if ch, ok := ocConfig["channels"].(map[string]interface{}); ok {
				for id, conf := range ch {
					if m, ok := conf.(map[string]interface{}); ok {
						if enabled, _ := m["enabled"].(bool); enabled {
							label := channelLabels[id]
							if label == "" {
								label = id
							}
							channels = append(channels, enabledChannel{ID: id, Label: label, Type: "builtin"})
						}
					}
				}
			}
			// 扫描 plugins.entries
			if plugins, ok := ocConfig["plugins"].(map[string]interface{}); ok {
				if entries, ok := plugins["entries"].(map[string]interface{}); ok {
					// 飞书别名映射：官方版插件 ID 映射为 feishu，避免重复
					channelAliases := map[string]string{
						canonicalFeishuOfficialPluginID: canonicalFeishuCommunityPluginID,
						"wecom-openclaw-plugin":         "wecom",
						"qqbot-community":               "qqbot",
					}
					for id, conf := range entries {
						if id == "wecom-app" {
							continue
						}
						canonicalID := id
						if alias, ok := channelAliases[id]; ok {
							canonicalID = alias
						}
						if _, known := channelLabels[canonicalID]; !known {
							continue
						}
						// 检查是否已在 channels 中（用规范 ID 检查）
						found := false
						for _, ch := range channels {
							if ch.ID == canonicalID || ch.ID == id {
								found = true
								break
							}
						}
						if found {
							continue
						}
						if m, ok := conf.(map[string]interface{}); ok {
							if enabled, _ := m["enabled"].(bool); enabled {
								label := channelLabels[canonicalID]
								if label == "" {
									label = channelLabels[id]
								}
								if label == "" {
									label = id
								}
								channels = append(channels, enabledChannel{ID: canonicalID, Label: label, Type: "plugin"})
							}
						}
					}
				}
			}
		}
		if channels == nil {
			channels = []enabledChannel{}
		}

		// 获取当前模型
		currentModel := ""
		if ocConfig != nil {
			if agents, ok := ocConfig["agents"].(map[string]interface{}); ok {
				if defaults, ok := agents["defaults"].(map[string]interface{}); ok {
					if model, ok := defaults["model"].(map[string]interface{}); ok {
						currentModel, _ = model["primary"].(string)
					}
				}
			}
		}

		// 进程状态
		procStatus := procMgr.GetStatus()
		gatewayRunning := procMgr.GatewayListening()
		runtimeHealth := buildOpenClawRuntimeHealth(cfg.OpenClawInstalled(), procStatus, gatewayRunning)
		taskPressure := openClawTaskPressureSummary{
			ByStatus: map[string]int{
				"queued":    0,
				"running":   0,
				"succeeded": 0,
				"failed":    0,
				"timed_out": 0,
				"cancelled": 0,
				"lost":      0,
			},
			ByRuntime: map[string]int{
				"subagent": 0,
				"acp":      0,
				"cli":      0,
				"cron":     0,
			},
			UpdatedAt: time.Now().UnixMilli(),
		}
		if records, err := readOpenClawTasks(cfg, 200); err == nil {
			taskPressure = summarizeOpenClawTasks(records, time.Now())
		}

		// 内存使用
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// NapCat 登录状态
		napcatInfo := gin.H{"connected": false}
		if napcatMon != nil {
			monStatus := napcatMon.GetStatus()
			napcatInfo["connected"] = monStatus.QQLoggedIn
			if monStatus.QQNickname != "" {
				napcatInfo["nickname"] = monStatus.QQNickname
			}
			if monStatus.QQID != "" {
				napcatInfo["selfId"] = monStatus.QQID
			}
			if monStatus.QQLoggedIn {
				if groupR, err := onebotApiCallSafe("POST", "/get_group_list", nil); err == nil {
					if groupData, ok := groupR["data"].([]interface{}); ok {
						napcatInfo["groupCount"] = len(groupData)
					}
				}
				if friendR, err := onebotApiCallSafe("POST", "/get_friend_list", nil); err == nil {
					if friendData, ok := friendR["data"].([]interface{}); ok {
						napcatInfo["friendCount"] = len(friendData)
					}
				}
			}
		} else if loginR, err := napcatApiCallSafe(cfg, "POST", "/api/QQLogin/CheckLoginStatus", nil); err == nil {
			if data, ok := loginR["data"].(map[string]interface{}); ok {
				if isLogin, _ := data["isLogin"].(bool); isLogin {
					napcatInfo["connected"] = true
					// 获取登录信息
					if infoR, err := napcatApiCallSafe(cfg, "POST", "/api/QQLogin/GetQQLoginInfo", nil); err == nil {
						if infoData, ok := infoR["data"].(map[string]interface{}); ok {
							napcatInfo["nickname"], _ = infoData["nick"].(string)
							if uin, ok := infoData["uin"].(string); ok {
								napcatInfo["selfId"] = uin
							}
						}
					}
					// 获取群数和好友数 (通过 OneBot11 HTTP API)
					if groupR, err := onebotApiCallSafe("POST", "/get_group_list", nil); err == nil {
						if groupData, ok := groupR["data"].([]interface{}); ok {
							napcatInfo["groupCount"] = len(groupData)
						}
					}
					if friendR, err := onebotApiCallSafe("POST", "/get_friend_list", nil); err == nil {
						if friendData, ok := friendR["data"].([]interface{}); ok {
							napcatInfo["friendCount"] = len(friendData)
						}
					}
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"panel": gin.H{
				"version": buildinfo.Version,
				"edition": buildinfo.NormalizedEdition(),
			},
			"openclaw": gin.H{
				"configured":      cfg.OpenClawInstalled(),
				"currentModel":    currentModel,
				"enabledChannels": channels,
				"taskPressure":    taskPressure,
				"runtime":         runtimeHealth,
				"edition":         cfg.Edition,
				"managedRuntime":  cfg.IsLiteEdition(),
				"bundledRuntime":  cfg.IsLiteEdition(),
				"gatewayPort":     gatewayPort,
			},
			"gateway": gin.H{
				"running": gatewayRunning,
			},
			"napcat":  napcatInfo,
			"process": procStatus,
			"admin": gin.H{
				"uptime":     int64(time.Since(startTime).Seconds()),
				"memoryMB":   int(memStats.Sys / 1024 / 1024),
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
				"goroutines": runtime.NumGoroutine(),
			},
		})
	}
}

func buildOpenClawRuntimeHealth(configured bool, procStatus process.Status, gatewayRunning bool) gin.H {
	if !configured {
		return gin.H{
			"state":          "not_configured",
			"healthy":        false,
			"degraded":       false,
			"processRunning": procStatus.Running,
			"gatewayRunning": gatewayRunning,
			"title":          "OpenClaw 尚未安装或配置",
			"message":        "当前页面可浏览，但模型、通道和网关相关能力尚未就绪。",
		}
	}

	if procStatus.Running && gatewayRunning {
		return gin.H{
			"state":          "healthy",
			"healthy":        true,
			"degraded":       false,
			"processRunning": procStatus.Running,
			"gatewayRunning": gatewayRunning,
			"title":          "OpenClaw 运行正常",
			"message":        "OpenClaw 与网关均在线，消息处理与配置写入可正常进行。",
		}
	}

	if gatewayRunning {
		title := "网关在线，但运行状态异常"
		message := "OpenClaw 网关仍可访问，但面板未确认到稳定主进程；运行相关页面可能出现状态不同步。"
		if procStatus.ManagedExternally {
			title = "网关已由外部实例接管"
			message = "网关仍在线，但当前实例不受 ClawPanel 直接管理；如需重启，请在外部环境处理或使用网关按钮。"
		}
		return gin.H{
			"state":          "degraded",
			"healthy":        false,
			"degraded":       true,
			"processRunning": procStatus.Running,
			"gatewayRunning": gatewayRunning,
			"title":          title,
			"message":        message,
		}
	}

	if procStatus.Running {
		return gin.H{
			"state":          "degraded",
			"healthy":        false,
			"degraded":       true,
			"processRunning": procStatus.Running,
			"gatewayRunning": gatewayRunning,
			"title":          "OpenClaw 进程存在，但网关离线",
			"message":        "ClawPanel 检测到 OpenClaw 进程仍在运行，但消息网关当前不可达；通道收发和 AI 请求可能失败。",
		}
	}

	return gin.H{
		"state":          "offline",
		"healthy":        false,
		"degraded":       true,
		"processRunning": false,
		"gatewayRunning": false,
		"title":          "OpenClaw 与网关均离线",
		"message":        "当前运行环境异常，依赖 OpenClaw 的页面可能无法保存配置、安装插件或处理消息。",
	}
}

// napcatApiCallSafe calls NapCat API with a short timeout, returns nil on error
func napcatApiCallSafe(cfg *config.Config, method, path string, body interface{}) (map[string]interface{}, error) {
	cred := napcatAuth(cfg)
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = strings.NewReader(string(data))
	}
	req, err := http.NewRequest(method, "http://127.0.0.1:6099"+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cred != "" {
		req.Header.Set("Authorization", "Bearer "+cred)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

// onebotApiCallSafe calls OneBot11 HTTP API (port 3000) with a short timeout
func onebotApiCallSafe(method, path string, body interface{}) (map[string]interface{}, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = strings.NewReader(string(data))
	}
	req, err := http.NewRequest(method, "http://127.0.0.1:3000"+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

// runCmd runs a command and returns trimmed stdout, or fallback on error
func runCmd(name string, args ...string) string {
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

// GetSystemEnv 获取系统环境信息
func GetSystemEnv(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostname, _ := os.Hostname()

		// OS info
		osInfo := gin.H{
			"platform": runtime.GOOS,
			"arch":     runtime.GOARCH,
			"hostname": hostname,
			"cpus":     runtime.NumCPU(),
		}

		// Try to read host-env.json first
		var hostEnv map[string]interface{}
		hostEnvPath := filepath.Join(cfg.OpenClawDir, "host-env.json")
		if data, err := os.ReadFile(hostEnvPath); err == nil {
			json.Unmarshal(data, &hostEnv)
		}

		if hostEnv != nil {
			if osData, ok := hostEnv["os"].(map[string]interface{}); ok {
				if v, ok := osData["distro"].(string); ok && v != "" {
					osInfo["distro"] = v
				}
				if v, ok := osData["release"].(string); ok && v != "" {
					osInfo["release"] = v
				}
			}
		}

		// Fallback OS detection (platform-aware)
		if runtime.GOOS == "windows" {
			// Windows: use PowerShell for system info (force UTF-8 output)
			utf8Prefix := `[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; `
			if _, ok := osInfo["distro"]; !ok {
				distro := runCmd("powershell", "-NoProfile", "-Command", utf8Prefix+`(Get-CimInstance Win32_OperatingSystem).Caption`)
				if distro != "" {
					osInfo["distro"] = distro
				}
			}
			if _, ok := osInfo["release"]; !ok {
				release := runCmd("powershell", "-NoProfile", "-Command", utf8Prefix+`(Get-CimInstance Win32_OperatingSystem).Version`)
				if release != "" {
					osInfo["release"] = release
				}
			}
			// Kernel version
			kernel := runCmd("powershell", "-NoProfile", "-Command", utf8Prefix+`(Get-CimInstance Win32_OperatingSystem).BuildNumber`)
			if kernel != "" {
				osInfo["kernel"] = "Build " + kernel
			}
			// User info: show actual logged-in user, not SYSTEM service account
			userInfo := runCmd("powershell", "-NoProfile", "-Command", utf8Prefix+`(Get-CimInstance Win32_ComputerSystem).UserName`)
			if userInfo == "" || strings.Contains(strings.ToLower(userInfo), "system") {
				// Fallback: find first interactive logon session user
				userInfo = runCmd("powershell", "-NoProfile", "-Command", utf8Prefix+`(query user 2>$null | Select-String '^\s*\S+' | Select-Object -First 1).ToString().Trim().Split()[0]`)
			}
			if userInfo == "" {
				userInfo = runCmd("whoami")
			}
			if userInfo != "" {
				osInfo["userInfo"] = userInfo
			}
			// Memory info via PowerShell
			totalMem := runCmd("powershell", "-NoProfile", "-Command", `[math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1MB)`)
			freeMem := runCmd("powershell", "-NoProfile", "-Command", `[math]::Round((Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory / 1KB)`)
			if totalMem != "" {
				if v, err := strconv.Atoi(totalMem); err == nil {
					osInfo["totalMemMB"] = v
				}
			}
			if freeMem != "" {
				if v, err := strconv.Atoi(freeMem); err == nil {
					osInfo["freeMemMB"] = v
				}
			}
			// CPU model
			cpuModel := runCmd("powershell", "-NoProfile", "-Command", `(Get-CimInstance Win32_Processor | Select-Object -First 1).Name`)
			if cpuModel != "" {
				osInfo["cpuModel"] = strings.TrimSpace(cpuModel)
			}
			// Uptime (seconds)
			uptimeStr := runCmd("powershell", "-NoProfile", "-Command", `[math]::Round((Get-CimInstance Win32_OperatingSystem | ForEach-Object { (Get-Date) - $_.LastBootUpTime }).TotalSeconds)`)
			if uptimeStr != "" {
				osInfo["uptime"] = uptimeStr
			}
			// CPU usage as load average approximation
			cpuLoad := runCmd("powershell", "-NoProfile", "-Command", `(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average`)
			if cpuLoad != "" {
				osInfo["loadAvg"] = cpuLoad + "%"
			}
		} else if runtime.GOOS == "darwin" {
			// macOS
			if _, ok := osInfo["distro"]; !ok {
				productName := runCmd("sw_vers", "-productName")
				productVer := runCmd("sw_vers", "-productVersion")
				if productName != "" && productVer != "" {
					osInfo["distro"] = productName + " " + productVer
				} else if productName != "" {
					osInfo["distro"] = productName
				}
			}
			if _, ok := osInfo["release"]; !ok {
				release := runCmd("uname", "-r")
				if release != "" {
					osInfo["release"] = release
				}
			}
			userInfo := runCmd("whoami")
			if userInfo != "" {
				osInfo["userInfo"] = userInfo
			}
			// Memory info
			totalMemBytes := runCmd("sysctl", "-n", "hw.memsize")
			if totalMemBytes != "" {
				if v, err := strconv.ParseInt(strings.TrimSpace(totalMemBytes), 10, 64); err == nil {
					osInfo["totalMemMB"] = int(v / 1024 / 1024)
				}
			}
			freeMem := runCmd("bash", "-c", `vm_stat | awk '/Pages free/ {free=$3} /Pages speculative/ {spec=$3} /Pages inactive/ {inactive=$3} END {gsub("\\.","",free); gsub("\\.","",spec); gsub("\\.","",inactive); printf "%d", ((free+spec+inactive)*4096)/1024/1024}'`)
			if freeMem != "" {
				if v, err := strconv.Atoi(strings.TrimSpace(freeMem)); err == nil {
					osInfo["freeMemMB"] = v
				}
			}
			// CPU model
			cpuModel := runCmd("sysctl", "-n", "machdep.cpu.brand_string")
			if cpuModel == "" {
				cpuModel = runCmd("sysctl", "-n", "hw.model")
			}
			if cpuModel != "" {
				osInfo["cpuModel"] = strings.TrimSpace(cpuModel)
			}
			// Uptime seconds
			bootTs := runCmd("bash", "-c", `sysctl -n kern.boottime | awk -F'[ ,}]+' '{print $4}'`)
			if bootTs != "" {
				if bt, err := strconv.ParseInt(strings.TrimSpace(bootTs), 10, 64); err == nil {
					uptime := time.Now().Unix() - bt
					if uptime > 0 {
						osInfo["uptime"] = strconv.FormatInt(uptime, 10)
					}
				}
			}
			// Load average
			loadAvg := runCmd("bash", "-c", `sysctl -n vm.loadavg | tr -d '{}' | xargs | awk '{print $1", "$2", "$3}'`)
			if loadAvg != "" {
				osInfo["loadAvg"] = strings.TrimSpace(loadAvg)
			}
		} else {
			// Linux
			if _, ok := osInfo["distro"]; !ok {
				distro := runCmd("bash", "-c", `cat /etc/os-release 2>/dev/null | grep "^PRETTY_NAME=" | cut -d= -f2 | tr -d '"'`)
				if distro != "" {
					osInfo["distro"] = distro
				}
			}
			if _, ok := osInfo["release"]; !ok {
				release := runCmd("uname", "-r")
				if release != "" {
					osInfo["release"] = release
				}
			}
			// User info
			userInfo := runCmd("whoami")
			if userInfo != "" {
				osInfo["userInfo"] = userInfo
			}
			// Memory info (from /proc/meminfo, locale-independent)
			totalMem := runCmd("bash", "-c", `awk '/^MemTotal:/{printf "%d", $2/1024}' /proc/meminfo 2>/dev/null`)
			freeMem := runCmd("bash", "-c", `awk '/^MemAvailable:/{printf "%d", $2/1024}' /proc/meminfo 2>/dev/null`)
			if totalMem != "" {
				if v, err := strconv.Atoi(totalMem); err == nil {
					osInfo["totalMemMB"] = v
				}
			}
			if freeMem != "" {
				if v, err := strconv.Atoi(freeMem); err == nil {
					osInfo["freeMemMB"] = v
				}
			}
			// CPU model
			cpuModel := runCmd("bash", "-c", `cat /proc/cpuinfo 2>/dev/null | grep "model name" | head -1 | cut -d: -f2`)
			if cpuModel != "" {
				osInfo["cpuModel"] = strings.TrimSpace(cpuModel)
			}
			// Uptime
			uptimeStr := runCmd("bash", "-c", `cat /proc/uptime 2>/dev/null | awk '{printf "%d", $1}'`)
			if uptimeStr != "" {
				osInfo["uptime"] = uptimeStr
			}
			// Load average
			loadAvg := runCmd("bash", "-c", `cat /proc/loadavg 2>/dev/null | awk '{print $1", "$2", "$3}'`)
			if loadAvg != "" {
				osInfo["loadAvg"] = loadAvg
			}
		}

		// Software detection
		software := gin.H{}

		// Prefer host-env.json software info
		var hostSw map[string]interface{}
		if hostEnv != nil {
			hostSw, _ = hostEnv["software"].(map[string]interface{})
		}

		// Node.js
		nodeVer := ""
		if hostSw != nil {
			if hv, ok := hostSw["node"].(string); ok && isUsableDetectedValue(hv) {
				nodeVer = hv
			}
		}
		if nodeVer == "" {
			nodeVer = runCmd("node", "--version")
		}
		if nodeVer != "" {
			software["node"] = nodeVer
		} else {
			software["node"] = "not installed"
		}

		// npm
		npmVer := ""
		if hostSw != nil {
			if hv, ok := hostSw["npm"].(string); ok && isUsableDetectedValue(hv) {
				npmVer = hv
			}
		}
		if npmVer == "" {
			npmVer = runCmd("npm", "--version")
		}
		if npmVer != "" {
			software["npm"] = npmVer
		} else {
			software["npm"] = "not installed"
		}

		// Docker
		dockerVer := ""
		if hostSw != nil {
			if hv, ok := hostSw["docker"].(string); ok && isUsableDetectedValue(hv) {
				dockerVer = hv
			}
		}
		if dockerVer == "" {
			dockerVer = runCmd("docker", "--version")
		}
		if dockerVer != "" {
			software["docker"] = dockerVer
		} else {
			software["docker"] = "not installed"
		}

		// Git
		gitVer := ""
		if hostSw != nil {
			if hv, ok := hostSw["git"].(string); ok && isUsableDetectedValue(hv) {
				gitVer = hv
			}
		}
		if gitVer == "" {
			gitVer = runCmd("git", "--version")
		}
		if gitVer != "" {
			software["git"] = gitVer
		} else {
			software["git"] = "not installed"
		}

		// Python
		pythonVer := ""
		if hostSw != nil {
			if hv, ok := hostSw["python"].(string); ok && isUsableDetectedValue(hv) {
				pythonVer = hv
			}
		}
		if pythonVer == "" {
			pythonVer = runCmd("python3", "--version")
		}
		if pythonVer != "" {
			software["python"] = pythonVer
		} else {
			software["python"] = "not installed"
		}

		// Go
		software["go"] = runtime.Version()

		// OpenClaw
		software["openclaw"] = detectOpenClaw(cfg)

		c.JSON(http.StatusOK, gin.H{
			"ok":       true,
			"os":       osInfo,
			"software": software,
		})
	}
}

// detectOpenClaw 检测 OpenClaw 版本
func detectOpenClaw(cfg *config.Config) string {
	ver := detectOpenClawVersion(cfg)
	if ver == "" {
		return "not found"
	}
	if ver == "installed" {
		return "installed (config found)"
	}
	if !strings.HasPrefix(ver, "v") {
		return "v" + ver
	}
	return ver
}

func isUsableDetectedValue(v string) bool {
	vv := strings.TrimSpace(strings.ToLower(v))
	if vv == "" || vv == "unknown" || vv == "not installed" || vv == "not found" || vv == "n/a" || vv == "-" {
		return false
	}
	return true
}
