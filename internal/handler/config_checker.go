package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

// ConfigIssue represents a detected configuration problem
type ConfigIssue struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`  // error, warning, info
	Component   string `json:"component"` // napcat, openclaw
	Title       string `json:"title"`
	Description string `json:"description"`
	Fixable     bool   `json:"fixable"`
	CurrentVal  string `json:"currentValue,omitempty"`
	ExpectedVal string `json:"expectedValue,omitempty"`
	FilePath    string `json:"filePath,omitempty"`
}

// ConfigCheckResult is the response for config check API
type ConfigCheckResult struct {
	OK       bool          `json:"ok"`
	Issues   []ConfigIssue `json:"issues"`
	Checked  int           `json:"checked"`
	Problems int           `json:"problems"`
}

// CheckConfig scans OpenClaw and NapCat configuration files for common issues
func CheckConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var issues []ConfigIssue
		checked := 0

		// === NapCat onebot11.json checks ===
		napcatIssues, napcatChecked := checkNapCatConfig(cfg)
		issues = append(issues, napcatIssues...)
		checked += napcatChecked

		// === NapCat webui.json checks ===
		webuiIssues, webuiChecked := checkNapCatWebUI(cfg)
		issues = append(issues, webuiIssues...)
		checked += webuiChecked

		// === OpenClaw config checks ===
		ocIssues, ocChecked := checkOpenClawConfig(cfg)
		issues = append(issues, ocIssues...)
		checked += ocChecked

		// === Port conflict checks ===
		portIssues, portChecked := checkPortConflicts()
		issues = append(issues, portIssues...)
		checked += portChecked

		problems := 0
		for _, iss := range issues {
			if iss.Severity == "error" || iss.Severity == "warning" {
				problems++
			}
		}

		c.JSON(http.StatusOK, ConfigCheckResult{
			OK:       true,
			Issues:   issues,
			Checked:  checked,
			Problems: problems,
		})
	}
}

// FixConfig applies automatic fixes for known configuration issues
func FixConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			IssueIDs []string `json:"issueIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		fixed := []string{}
		failed := []string{}

		for _, id := range req.IssueIDs {
			err := fixIssue(id, cfg)
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s: %s", id, err.Error()))
			} else {
				fixed = append(fixed, id)
			}
		}

		// If any NapCat config was fixed, restart the container
		napcatRestart := false
		for _, id := range fixed {
			if strings.HasPrefix(id, "napcat-") {
				napcatRestart = true
				break
			}
		}
		if napcatRestart {
			go restartNapCatProcess(cfg)
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":            true,
			"fixed":         fixed,
			"failed":        failed,
			"napcatRestart": napcatRestart,
		})
	}
}

// --- NapCat onebot11.json ---

// napCatConfigPath returns the NapCat config file path based on platform
func napCatConfigPath(cfg *config.Config, filename string) (string, error) {
	if runtime.GOOS == "windows" {
		napcatDir := getNapCatShellDir(cfg)
		if napcatDir == "" {
			return "", fmt.Errorf("NapCat Shell 未安装")
		}
		return filepath.Join(napcatDir, "config", filename), nil
	}
	return "", nil // Docker mode, path handled differently
}

func readNapCatOneBot11(cfg *config.Config) (map[string]interface{}, string, error) {
	var out []byte
	var err error
	var filePath string

	if runtime.GOOS == "windows" {
		p, perr := napCatConfigPath(cfg, "onebot11.json")
		if perr != nil {
			return nil, "", perr
		}
		filePath = p
		out, err = os.ReadFile(p)
	} else {
		filePath = "/app/napcat/config/onebot11.json (Docker: openclaw-qq)"
		out, err = exec.Command("docker", "exec", "openclaw-qq", "cat", "/app/napcat/config/onebot11.json").Output()
	}
	if err != nil {
		return nil, filePath, fmt.Errorf("无法读取 NapCat onebot11.json: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, filePath, fmt.Errorf("onebot11.json 解析失败: %v", err)
	}
	return data, filePath, nil
}

func checkNapCatConfig(cfg *config.Config) ([]ConfigIssue, int) {
	var issues []ConfigIssue
	checked := 0

	data, filePath, err := readNapCatOneBot11(cfg)
	if err != nil {
		// Check if NapCat is installed at all
		napcatInstalled := false
		if runtime.GOOS == "windows" {
			napcatInstalled = getNapCatShellDir(cfg) != ""
		} else {
			napcatInstalled = isDockerContainerExists("openclaw-qq")
		}
		if napcatInstalled {
			issues = append(issues, ConfigIssue{
				ID: "napcat-onebot11-missing", Severity: "error", Component: "napcat",
				Title:       "onebot11.json 配置文件不存在或无法读取",
				Description: err.Error(), Fixable: true, FilePath: filePath,
			})
		}
		return issues, 1
	}

	network, _ := data["network"].(map[string]interface{})
	if network == nil {
		issues = append(issues, ConfigIssue{
			ID: "napcat-no-network", Severity: "error", Component: "napcat",
			Title:       "缺少 network 配置块",
			Description: "onebot11.json 中没有 network 字段，WS/HTTP 服务无法启动",
			Fixable:     true, FilePath: filePath,
		})
		return issues, 1
	}

	// Check WebSocket servers
	checked++
	wsServers, _ := network["websocketServers"].([]interface{})
	if len(wsServers) == 0 {
		issues = append(issues, ConfigIssue{
			ID: "napcat-no-ws", Severity: "error", Component: "napcat",
			Title:       "WebSocket 服务未配置",
			Description: "未配置 websocketServers，ClawPanel 无法接收消息事件",
			Fixable:     true, FilePath: filePath,
		})
	} else {
		for i, ws := range wsServers {
			wsMap, _ := ws.(map[string]interface{})
			if wsMap == nil {
				continue
			}

			// Check enable
			checked++
			enabled, _ := wsMap["enable"].(bool)
			if !enabled {
				issues = append(issues, ConfigIssue{
					ID: fmt.Sprintf("napcat-ws-%d-disabled", i), Severity: "error", Component: "napcat",
					Title:       fmt.Sprintf("WebSocket 服务 #%d 未启用", i+1),
					Description: "websocketServers[" + fmt.Sprint(i) + "].enable = false",
					Fixable:     true, CurrentVal: "false", ExpectedVal: "true", FilePath: filePath,
				})
			}

			// Check port
			checked++
			port, _ := wsMap["port"].(float64)
			if port == 0 {
				issues = append(issues, ConfigIssue{
					ID: fmt.Sprintf("napcat-ws-%d-noport", i), Severity: "error", Component: "napcat",
					Title:       fmt.Sprintf("WebSocket 服务 #%d 端口未设置", i+1),
					Description: "端口为 0 或未设置",
					Fixable:     true, CurrentVal: "0", ExpectedVal: "3001", FilePath: filePath,
				})
			}

			// Check host is not 127.0.0.1 (NapCat bug: binding 127.0.0.1 causes "Server Started undefined:undefined")
			checked++
			wsHost, _ := wsMap["host"].(string)
			if wsHost == "127.0.0.1" {
				issues = append(issues, ConfigIssue{
					ID: fmt.Sprintf("napcat-ws-%d-host-loopback", i), Severity: "error", Component: "napcat",
					Title:       "NapCat WebSocket 绑定地址错误",
					Description: "host 为 127.0.0.1 导致 NapCat WebSocket 服务启动失败（显示 undefined:undefined），必须改为 0.0.0.0",
					Fixable:     true, CurrentVal: "127.0.0.1", ExpectedVal: "0.0.0.0", FilePath: filePath,
				})
			}

			// Check token matches openclaw.json channels.qq.accessToken
			checked++
			expectedToken := getNapCatExpectedWSToken(cfg)
			if expectedToken != "" {
				actualToken, _ := wsMap["token"].(string)
				if actualToken != expectedToken {
					issues = append(issues, ConfigIssue{
						ID: fmt.Sprintf("napcat-ws-%d-token-mismatch", i), Severity: "error", Component: "napcat",
						Title:       "NapCat WebSocket token 与 openclaw.json 不一致",
						Description: fmt.Sprintf("openclaw.json 中 channels.qq.accessToken=%q，但 NapCat onebot11.json WS token=%q，两者不一致导致 OpenClaw 无法连接 NapCat", expectedToken, actualToken),
						Fixable:     true, CurrentVal: actualToken, ExpectedVal: expectedToken, FilePath: filePath,
					})
				}
			}

			// Check reportSelfMessage
			checked++
			reportSelf, reportSelfExists := wsMap["reportSelfMessage"].(bool)
			if !reportSelfExists || !reportSelf {
				issues = append(issues, ConfigIssue{
					ID: fmt.Sprintf("napcat-ws-%d-no-self-msg", i), Severity: "warning", Component: "napcat",
					Title:       "reportSelfMessage 未启用",
					Description: "Bot 发送的消息不会转发到 WebSocket，活动日志中将看不到 Bot 回复",
					Fixable:     true, CurrentVal: "false", ExpectedVal: "true", FilePath: filePath,
				})
			}
		}
	}

	// Check HTTP servers
	checked++
	httpServers, _ := network["httpServers"].([]interface{})
	if len(httpServers) == 0 {
		issues = append(issues, ConfigIssue{
			ID: "napcat-no-http", Severity: "warning", Component: "napcat",
			Title:       "HTTP API 服务未配置",
			Description: "未配置 httpServers，部分 Bot 操作（发消息、获取群列表等）将不可用",
			Fixable:     true, FilePath: filePath,
		})
	} else {
		for i, hs := range httpServers {
			hsMap, _ := hs.(map[string]interface{})
			if hsMap == nil {
				continue
			}
			checked++
			enabled, _ := hsMap["enable"].(bool)
			if !enabled {
				issues = append(issues, ConfigIssue{
					ID: fmt.Sprintf("napcat-http-%d-disabled", i), Severity: "warning", Component: "napcat",
					Title:       fmt.Sprintf("HTTP API 服务 #%d 未启用", i+1),
					Description: "httpServers[" + fmt.Sprint(i) + "].enable = false",
					Fixable:     true, CurrentVal: "false", ExpectedVal: "true", FilePath: filePath,
				})
			}
		}
	}

	return issues, checked
}

// --- NapCat webui.json ---

func checkNapCatWebUI(cfg *config.Config) ([]ConfigIssue, int) {
	var issues []ConfigIssue
	checked := 0

	// Check if NapCat is installed
	if runtime.GOOS == "windows" {
		if getNapCatShellDir(cfg) == "" {
			return issues, 0
		}
	} else {
		if !isDockerContainerExists("openclaw-qq") {
			return issues, 0
		}
	}

	var out []byte
	var err error
	var filePath string
	if runtime.GOOS == "windows" {
		p, perr := napCatConfigPath(cfg, "webui.json")
		if perr != nil {
			return issues, 0
		}
		filePath = p
		out, err = os.ReadFile(p)
	} else {
		filePath = "/app/napcat/config/webui.json (Docker: openclaw-qq)"
		out, err = exec.Command("docker", "exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json").Output()
	}
	if err != nil {
		return issues, 0
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		return issues, 0
	}

	// Check token
	checked++
	token, _ := data["token"].(string)
	if token == "" {
		issues = append(issues, ConfigIssue{
			ID: "napcat-webui-no-token", Severity: "warning", Component: "napcat",
			Title:       "NapCat WebUI Token 为空",
			Description: "WebUI 没有设置访问令牌，ClawPanel 可能无法调用 NapCat 管理 API",
			Fixable:     true, CurrentVal: "(空)", ExpectedVal: "clawpanel-qq",
			FilePath: filePath,
		})
	}

	// Check token consistency with Docker WEBUI_TOKEN env (Linux only)
	if runtime.GOOS != "windows" && token != "" {
		checked++
		envOut, envErr := exec.Command("docker", "inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", "openclaw-qq").Output()
		if envErr == nil {
			dockerEnvToken := ""
			for _, line := range strings.Split(string(envOut), "\n") {
				if strings.HasPrefix(line, "WEBUI_TOKEN=") {
					dockerEnvToken = strings.TrimPrefix(line, "WEBUI_TOKEN=")
					break
				}
			}
			if dockerEnvToken != "" && dockerEnvToken != token {
				issues = append(issues, ConfigIssue{
					ID: "napcat-webui-token-mismatch", Severity: "error", Component: "napcat",
					Title:       "WebUI Token 与 Docker 环境变量不一致",
					Description: fmt.Sprintf("webui.json token=\"%s\" 与 Docker WEBUI_TOKEN=\"%s\" 不一致，会导致扫码登录 Unauthorized 错误", token, dockerEnvToken),
					Fixable:     true, CurrentVal: token, ExpectedVal: dockerEnvToken,
					FilePath: filePath,
				})
			}
		}
	}

	return issues, checked
}

// --- OpenClaw config ---

func checkOpenClawConfig(cfg *config.Config) ([]ConfigIssue, int) {
	var issues []ConfigIssue
	checked := 0

	ocConfig, err := cfg.ReadOpenClawJSON()
	if err != nil {
		if !os.IsNotExist(err) {
			issues = append(issues, ConfigIssue{
				ID: "openclaw-config-error", Severity: "error", Component: "openclaw",
				Title:       "openclaw.json 读取失败",
				Description: err.Error(), Fixable: false,
				FilePath: filepath.Join(cfg.OpenClawDir, "openclaw.json"),
			})
		}
		return issues, 1
	}
	normalizeOpenClawCompatConfig(ocConfig)

	// Check models configuration
	checked++
	models, _ := ocConfig["models"].(map[string]interface{})
	if models == nil {
		issues = append(issues, ConfigIssue{
			ID: "openclaw-no-models", Severity: "warning", Component: "openclaw",
			Title:       "未配置模型提供商",
			Description: "openclaw.json 中没有 models 配置，AI 助手将无法工作",
			Fixable:     false,
			FilePath:    filepath.Join(cfg.OpenClawDir, "openclaw.json"),
		})
	} else {
		providers, _ := models["providers"].(map[string]interface{})
		if providers == nil || len(providers) == 0 {
			issues = append(issues, ConfigIssue{
				ID: "openclaw-no-providers", Severity: "warning", Component: "openclaw",
				Title:       "未配置任何模型提供商",
				Description: "models.providers 为空，请在系统配置中添加至少一个模型服务商",
				Fixable:     false,
				FilePath:    filepath.Join(cfg.OpenClawDir, "openclaw.json"),
			})
		} else {
			// Check each provider has apiKey
			for pid, prov := range providers {
				checked++
				provMap, _ := prov.(map[string]interface{})
				if provMap == nil {
					continue
				}
				apiType, _ := provMap["api"].(string)
				switch apiType {
				case "anthropic":
					issues = append(issues, ConfigIssue{
						ID: "openclaw-provider-" + pid + "-legacy-api", Severity: "error", Component: "openclaw",
						Title: fmt.Sprintf("模型提供商 %s 使用了旧 API 类型", pid),
						Description: fmt.Sprintf("providers.%s.api 应为 anthropic-messages，当前为 anthropic", pid),
						Fixable: false, CurrentVal: "anthropic", ExpectedVal: "anthropic-messages",
						FilePath: filepath.Join(cfg.OpenClawDir, "openclaw.json"),
					})
				case "google-genai":
					issues = append(issues, ConfigIssue{
						ID: "openclaw-provider-" + pid + "-legacy-api", Severity: "error", Component: "openclaw",
						Title: fmt.Sprintf("模型提供商 %s 使用了旧 API 类型", pid),
						Description: fmt.Sprintf("providers.%s.api 应为 google-generative-ai，当前为 google-genai", pid),
						Fixable: false, CurrentVal: "google-genai", ExpectedVal: "google-generative-ai",
						FilePath: filepath.Join(cfg.OpenClawDir, "openclaw.json"),
					})
				}
				apiKey, _ := provMap["apiKey"].(string)
				if apiKey == "" {
					issues = append(issues, ConfigIssue{
						ID: "openclaw-provider-" + pid + "-no-key", Severity: "warning", Component: "openclaw",
						Title:       fmt.Sprintf("模型提供商 %s 缺少 API Key", pid),
						Description: fmt.Sprintf("providers.%s.apiKey 为空，该提供商的模型将无法使用", pid),
						Fixable:     false,
						FilePath:    filepath.Join(cfg.OpenClawDir, "openclaw.json"),
					})
				}
			}
		}
	}

	// Check primary model
	checked++
	agents, _ := ocConfig["agents"].(map[string]interface{})
	if agents != nil {
		defaults, _ := agents["defaults"].(map[string]interface{})
		if defaults != nil {
			model, _ := defaults["model"].(map[string]interface{})
			if model != nil {
				primary, _ := model["primary"].(string)
				if primary == "" {
					issues = append(issues, ConfigIssue{
						ID: "openclaw-no-primary-model", Severity: "warning", Component: "openclaw",
						Title:       "未设置主模型",
						Description: "agents.defaults.model.primary 为空，请在系统配置中选择一个主模型",
						Fixable:     false,
						FilePath:    filepath.Join(cfg.OpenClawDir, "openclaw.json"),
					})
				}
			}
		}
	}

	// Check channels configuration
	checked++
	channels, _ := ocConfig["channels"].(map[string]interface{})
	if channels != nil {
		for chID, ch := range channels {
			chMap, _ := ch.(map[string]interface{})
			if chMap == nil {
				continue
			}
			enabled, _ := chMap["enabled"].(bool)
			if !enabled {
				continue
			}
			// For QQ channel, check wsUrl config
			if chID == "qq" {
				checked++
				wsUrl, _ := chMap["wsUrl"].(string)
				if wsUrl == "" {
					issues = append(issues, ConfigIssue{
						ID: "openclaw-qq-no-wsurl", Severity: "warning", Component: "openclaw",
						Title:       "QQ 通道缺少 WebSocket 地址",
						Description: "channels.qq.wsUrl 未配置，QQ 通道将无法连接 NapCat",
						Fixable:     false,
						FilePath:    filepath.Join(cfg.OpenClawDir, "openclaw.json"),
					})
				}
			}
		}
	}

	return issues, checked
}

// --- Port conflicts ---

func checkPortConflicts() ([]ConfigIssue, int) {
	var issues []ConfigIssue
	checked := 0

	ports := map[int]string{
		3000: "NapCat HTTP API",
		3001: "NapCat WebSocket",
		6099: "NapCat WebUI",
	}

	for port, name := range ports {
		checked++
		if !isPortListening(port) && isDockerContainerExists("openclaw-qq") {
			issues = append(issues, ConfigIssue{
				ID:          fmt.Sprintf("port-%d-not-listening", port),
				Severity:    "warning",
				Component:   "napcat",
				Title:       fmt.Sprintf("端口 %d (%s) 未监听", port, name),
				Description: fmt.Sprintf("NapCat 容器已存在但端口 %d 未响应，可能服务未启动或端口映射错误", port),
				Fixable:     false,
			})
		}
	}

	return issues, checked
}

// --- Fix logic ---

func fixIssue(issueID string, cfg *config.Config) error {
	switch {
	case issueID == "napcat-onebot11-missing" || issueID == "napcat-no-network" || issueID == "napcat-no-ws" || issueID == "napcat-no-http":
		return writeDefaultOneBot11Config(cfg)

	case strings.HasPrefix(issueID, "napcat-ws-") && strings.HasSuffix(issueID, "-disabled"):
		return fixNapCatWSField(cfg, "enable", true)
	case strings.HasPrefix(issueID, "napcat-ws-") && strings.HasSuffix(issueID, "-noport"):
		return fixNapCatWSField(cfg, "port", float64(3001))
	case strings.HasPrefix(issueID, "napcat-ws-") && strings.HasSuffix(issueID, "-no-self-msg"):
		return fixNapCatWSField(cfg, "reportSelfMessage", true)

	case strings.HasPrefix(issueID, "napcat-http-") && strings.HasSuffix(issueID, "-disabled"):
		return fixNapCatHTTPField(cfg, "enable", true)

	case strings.HasPrefix(issueID, "napcat-ws-") && strings.HasSuffix(issueID, "-host-loopback"):
		return fixNapCatAllAccountsWSHost(cfg)

	case strings.HasPrefix(issueID, "napcat-ws-") && strings.HasSuffix(issueID, "-token-mismatch"):
		return fixNapCatWSToken(cfg)

	case issueID == "napcat-webui-no-token":
		return fixNapCatWebUIToken(cfg)

	case issueID == "napcat-webui-token-mismatch":
		return fixNapCatWebUITokenMismatch(cfg)

	default:
		return fmt.Errorf("该问题不支持自动修复")
	}
}

// getNapCatExpectedWSToken returns the accessToken that openclaw.json expects NapCat WS to use.
// If openclaw.json has channels.qq.accessToken set, NapCat WS must use the same token.
func getNapCatExpectedWSToken(cfg *config.Config) string {
	_, token, err := cfg.ReadQQChannelState()
	if err != nil {
		return ""
	}
	return token
}

func writeDefaultOneBot11Config(cfg *config.Config) error {
	wsToken := getNapCatExpectedWSToken(cfg)
	defaultConfig, err := marshalDefaultOneBot11Config(wsToken)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		p, err := napCatConfigPath(cfg, "onebot11.json")
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(p), 0755)
		return os.WriteFile(p, defaultConfig, 0644)
	}
	cmd := exec.Command("docker", "exec", "openclaw-qq", "bash", "-c",
		fmt.Sprintf("cat > /app/napcat/config/onebot11.json << 'FIXEOF'\n%s\nFIXEOF", string(defaultConfig)))
	return cmd.Run()
}

func marshalDefaultOneBot11Config(wsToken string) ([]byte, error) {
	payload := map[string]interface{}{
		"network": map[string]interface{}{
			"websocketServers": []map[string]interface{}{
				{
					"name":                 "ws-server",
					"enable":               true,
					"host":                 "0.0.0.0",
					"port":                 3001,
					"token":                wsToken,
					"reportSelfMessage":    true,
					"enableForcePushEvent": true,
					"messagePostFormat":    "array",
					"debug":                false,
					"heartInterval":        30000,
				},
			},
			"httpServers": []map[string]interface{}{
				{
					"name":   "http-api",
					"enable": true,
					"host":   "0.0.0.0",
					"port":   3000,
					"token":  "",
				},
			},
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

func fixNapCatWSToken(cfg *config.Config) error {
	expected := getNapCatExpectedWSToken(cfg)
	return fixNapCatWSField(cfg, "token", expected)
}

// fixNapCatAllAccountsWSHost fixes host=127.0.0.1 → 0.0.0.0 in onebot11.json and all onebot11_*.json account files.
// NapCat has a bug where binding 127.0.0.1 causes "Server Started undefined:undefined" and WS never starts.
func fixNapCatAllAccountsWSHost(cfg *config.Config) error {
	// Fix the base onebot11.json first
	if err := fixNapCatWSField(cfg, "host", "0.0.0.0"); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	// Fix all per-account onebot11_*.json files inside the container
	out, err := exec.Command("docker", "exec", "openclaw-qq", "sh", "-c",
		"ls /app/napcat/config/onebot11_*.json 2>/dev/null").Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	for _, fpath := range strings.Fields(string(out)) {
		raw, err := exec.Command("docker", "exec", "openclaw-qq", "cat", fpath).Output()
		if err != nil {
			continue
		}
		var data map[string]interface{}
		if json.Unmarshal(raw, &data) != nil {
			continue
		}
		network, _ := data["network"].(map[string]interface{})
		if network == nil {
			continue
		}
		wsServers, _ := network["websocketServers"].([]interface{})
		changed := false
		for _, ws := range wsServers {
			wsMap, _ := ws.(map[string]interface{})
			if wsMap != nil {
				if h, _ := wsMap["host"].(string); h == "127.0.0.1" {
					wsMap["host"] = "0.0.0.0"
					changed = true
				}
			}
		}
		if !changed {
			continue
		}
		jsonBytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			continue
		}
		tmpFile := fmt.Sprintf("/tmp/ob11_fix_%d.json", len(fpath))
		if os.WriteFile(tmpFile, jsonBytes, 0644) != nil {
			continue
		}
		exec.Command("docker", "cp", tmpFile, "openclaw-qq:"+fpath).Run()
		os.Remove(tmpFile)
	}
	return nil
}

func fixNapCatWSField(cfg *config.Config, field string, value interface{}) error {
	data, _, err := readNapCatOneBot11(cfg)
	if err != nil {
		return err
	}

	network, _ := data["network"].(map[string]interface{})
	if network == nil {
		return writeDefaultOneBot11Config(cfg)
	}

	wsServers, _ := network["websocketServers"].([]interface{})
	if len(wsServers) == 0 {
		return writeDefaultOneBot11Config(cfg)
	}

	for _, ws := range wsServers {
		wsMap, _ := ws.(map[string]interface{})
		if wsMap != nil {
			wsMap[field] = value
		}
	}

	return writeNapCatOneBot11(cfg, data)
}

func fixNapCatHTTPField(cfg *config.Config, field string, value interface{}) error {
	data, _, err := readNapCatOneBot11(cfg)
	if err != nil {
		return err
	}

	network, _ := data["network"].(map[string]interface{})
	if network == nil {
		return writeDefaultOneBot11Config(cfg)
	}

	httpServers, _ := network["httpServers"].([]interface{})
	if len(httpServers) == 0 {
		return writeDefaultOneBot11Config(cfg)
	}

	for _, hs := range httpServers {
		hsMap, _ := hs.(map[string]interface{})
		if hsMap != nil {
			hsMap[field] = value
		}
	}

	return writeNapCatOneBot11(cfg, data)
}

func writeNapCatOneBot11(cfg *config.Config, data map[string]interface{}) error {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		p, perr := napCatConfigPath(cfg, "onebot11.json")
		if perr != nil {
			return perr
		}
		os.MkdirAll(filepath.Dir(p), 0755)
		return os.WriteFile(p, jsonBytes, 0644)
	}
	cmd := exec.Command("docker", "exec", "openclaw-qq", "bash", "-c",
		fmt.Sprintf("cat > /app/napcat/config/onebot11.json << 'FIXEOF'\n%s\nFIXEOF", string(jsonBytes)))
	return cmd.Run()
}

func fixNapCatWebUIToken(cfg *config.Config) error {
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		p, perr := napCatConfigPath(cfg, "webui.json")
		if perr != nil {
			return perr
		}
		out, err = os.ReadFile(p)
	} else {
		out, err = exec.Command("docker", "exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json").Output()
	}
	if err != nil {
		return err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		return err
	}
	data["token"] = "clawpanel-qq"
	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	if runtime.GOOS == "windows" {
		p, perr := napCatConfigPath(cfg, "webui.json")
		if perr != nil {
			return perr
		}
		return os.WriteFile(p, jsonBytes, 0644)
	}
	cmd := exec.Command("docker", "exec", "openclaw-qq", "bash", "-c",
		fmt.Sprintf("cat > /app/napcat/config/webui.json << 'FIXEOF'\n%s\nFIXEOF", string(jsonBytes)))
	return cmd.Run()
}

func fixNapCatWebUITokenMismatch(cfg *config.Config) error {
	// Read Docker WEBUI_TOKEN env var
	envOut, err := exec.Command("docker", "inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", "openclaw-qq").Output()
	if err != nil {
		return fmt.Errorf("无法读取 Docker 环境变量: %v", err)
	}
	dockerEnvToken := ""
	for _, line := range strings.Split(string(envOut), "\n") {
		if strings.HasPrefix(line, "WEBUI_TOKEN=") {
			dockerEnvToken = strings.TrimPrefix(line, "WEBUI_TOKEN=")
			break
		}
	}
	if dockerEnvToken == "" {
		return fmt.Errorf("Docker 容器未设置 WEBUI_TOKEN 环境变量")
	}

	// Read current webui.json
	out, err := exec.Command("docker", "exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json").Output()
	if err != nil {
		return err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		return err
	}
	data["token"] = dockerEnvToken
	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	cmd := exec.Command("docker", "exec", "openclaw-qq", "bash", "-c",
		fmt.Sprintf("cat > /app/napcat/config/webui.json << 'FIXEOF'\n%s\nFIXEOF", string(jsonBytes)))
	return cmd.Run()
}

// --- Helpers ---

func isDockerContainerExists(name string) bool {
	out, err := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", name).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func isPortListening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
