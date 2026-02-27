package handler

import (
	"crypto/sha256"
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

// DiagnoseStep represents one step in the diagnosis/repair process
type DiagnoseStep struct {
	Step    string `json:"step"`
	Status  string `json:"status"` // ok, warning, error, fixed, skip
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// DiagnoseResult is the response for the NapCat diagnose API
type DiagnoseResult struct {
	OK      bool           `json:"ok"`
	Steps   []DiagnoseStep `json:"steps"`
	Summary string         `json:"summary"`
}

// DiagnoseNapCat runs a full diagnosis and optional repair of NapCat
func DiagnoseNapCat(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Repair bool `json:"repair"`
		}
		c.ShouldBindJSON(&req)

		var steps []DiagnoseStep
		repair := req.Repair

		if runtime.GOOS == "windows" {
			steps = diagnoseNapCatWindows(cfg, repair)
		} else {
			steps = diagnoseNapCatLinux(cfg, repair)
		}

		// Build summary
		errors := 0
		warnings := 0
		fixed := 0
		for _, s := range steps {
			switch s.Status {
			case "error":
				errors++
			case "warning":
				warnings++
			case "fixed":
				fixed++
			}
		}

		summary := ""
		if errors == 0 && warnings == 0 {
			summary = "✅ NapCat 状态正常，所有检查项通过"
		} else if repair && fixed > 0 && errors == 0 {
			summary = fmt.Sprintf("🔧 已自动修复 %d 个问题", fixed)
		} else if errors > 0 {
			summary = fmt.Sprintf("❌ 发现 %d 个错误", errors)
			if warnings > 0 {
				summary += fmt.Sprintf("，%d 个警告", warnings)
			}
			if fixed > 0 {
				summary += fmt.Sprintf("，已修复 %d 个", fixed)
			}
		} else {
			summary = fmt.Sprintf("⚠️ 发现 %d 个警告", warnings)
			if fixed > 0 {
				summary += fmt.Sprintf("，已修复 %d 个", fixed)
			}
		}

		c.JSON(http.StatusOK, DiagnoseResult{
			OK:      true,
			Steps:   steps,
			Summary: summary,
		})
	}
}

func diagnoseNapCatLinux(cfg *config.Config, repair bool) []DiagnoseStep {
	var steps []DiagnoseStep

	// Step 1: Check Docker installed
	dockerVer, err := exec.Command("docker", "--version").Output()
	if err != nil {
		steps = append(steps, DiagnoseStep{
			Step: "Docker 安装检测", Status: "error",
			Message: "Docker 未安装",
			Detail:  "请先安装 Docker，可在系统配置 → 运行环境中一键安装",
		})
		return steps
	}
	steps = append(steps, DiagnoseStep{
		Step: "Docker 安装检测", Status: "ok",
		Message: "Docker 已安装",
		Detail:  strings.TrimSpace(string(dockerVer)),
	})

	// Step 2: Check Docker daemon running
	err = exec.Command("docker", "info").Run()
	if err != nil {
		steps = append(steps, DiagnoseStep{
			Step: "Docker 服务状态", Status: "error",
			Message: "Docker 服务未运行",
			Detail:  "请运行 systemctl start docker 启动 Docker 服务",
		})
		return steps
	}
	steps = append(steps, DiagnoseStep{
		Step: "Docker 服务状态", Status: "ok",
		Message: "Docker 服务正常运行",
	})

	// Step 3: Check container exists
	containerStatus := ""
	out, err := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", "openclaw-qq").Output()
	if err != nil {
		steps = append(steps, DiagnoseStep{
			Step: "NapCat 容器检测", Status: "error",
			Message: "openclaw-qq 容器不存在",
			Detail:  "请在通道管理或系统配置中一键安装 NapCat",
		})
		return steps
	}
	containerStatus = strings.TrimSpace(string(out))
	if containerStatus == "running" {
		steps = append(steps, DiagnoseStep{
			Step: "NapCat 容器检测", Status: "ok",
			Message: "openclaw-qq 容器正在运行",
		})
	} else {
		if repair {
			exec.Command("docker", "start", "openclaw-qq").Run()
			time.Sleep(5 * time.Second)
			out2, _ := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", "openclaw-qq").Output()
			if strings.TrimSpace(string(out2)) == "running" {
				steps = append(steps, DiagnoseStep{
					Step: "NapCat 容器检测", Status: "fixed",
					Message: "容器已启动",
					Detail:  fmt.Sprintf("容器状态从 %s 恢复为 running", containerStatus),
				})
			} else {
				steps = append(steps, DiagnoseStep{
					Step: "NapCat 容器检测", Status: "error",
					Message: fmt.Sprintf("容器状态: %s，启动失败", containerStatus),
					Detail:  "请检查 docker logs openclaw-qq 查看错误日志",
				})
				return steps
			}
		} else {
			steps = append(steps, DiagnoseStep{
				Step: "NapCat 容器检测", Status: "error",
				Message: fmt.Sprintf("容器状态: %s（未运行）", containerStatus),
				Detail:  "点击「诊断并修复」可尝试启动容器",
			})
			return steps
		}
	}

	// Step 4: Check port mappings
	portOut, _ := exec.Command("docker", "port", "openclaw-qq").Output()
	portStr := string(portOut)
	has6099 := strings.Contains(portStr, "6099")
	has3001 := strings.Contains(portStr, "3001")
	has3000 := strings.Contains(portStr, "3000")
	if has6099 && has3001 && has3000 {
		steps = append(steps, DiagnoseStep{
			Step: "端口映射检测", Status: "ok",
			Message: "端口映射正常 (6099/3001/3000)",
		})
	} else {
		missing := []string{}
		if !has6099 {
			missing = append(missing, "6099(WebUI)")
		}
		if !has3001 {
			missing = append(missing, "3001(WS)")
		}
		if !has3000 {
			missing = append(missing, "3000(HTTP)")
		}
		steps = append(steps, DiagnoseStep{
			Step: "端口映射检测", Status: "error",
			Message: fmt.Sprintf("缺少端口映射: %s", strings.Join(missing, ", ")),
			Detail:  "需要重新安装 NapCat 容器以修复端口映射，请在通道管理中重新安装",
		})
	}

	// Step 5: Check onebot11.json config
	obOut, err := exec.Command("docker", "exec", "openclaw-qq", "cat", "/app/napcat/config/onebot11.json").Output()
	if err != nil {
		if repair {
			err2 := writeDefaultOneBot11Config(cfg)
			if err2 == nil {
				steps = append(steps, DiagnoseStep{
					Step: "OneBot11 配置检测", Status: "fixed",
					Message: "已创建默认 onebot11.json 配置",
					Detail:  "WebSocket 端口 3001, HTTP 端口 3000",
				})
			} else {
				steps = append(steps, DiagnoseStep{
					Step: "OneBot11 配置检测", Status: "error",
					Message: "onebot11.json 不存在且创建失败",
					Detail:  err2.Error(),
				})
			}
		} else {
			steps = append(steps, DiagnoseStep{
				Step: "OneBot11 配置检测", Status: "error",
				Message: "onebot11.json 配置文件不存在",
				Detail:  "点击「诊断并修复」可自动创建默认配置",
			})
		}
	} else {
		var obData map[string]interface{}
		if json.Unmarshal(obOut, &obData) != nil {
			steps = append(steps, DiagnoseStep{
				Step: "OneBot11 配置检测", Status: "error",
				Message: "onebot11.json 格式错误",
				Detail:  "配置文件无法解析为 JSON",
			})
		} else {
			wsOK := false
			network, _ := obData["network"].(map[string]interface{})
			if network != nil {
				wsList, _ := network["websocketServers"].([]interface{})
				for _, ws := range wsList {
					wsMap, _ := ws.(map[string]interface{})
					if wsMap != nil {
						enabled, _ := wsMap["enable"].(bool)
						port, _ := wsMap["port"].(float64)
						if enabled && port == 3001 {
							wsOK = true
						}
					}
				}
			}
			if wsOK {
				steps = append(steps, DiagnoseStep{
					Step: "OneBot11 配置检测", Status: "ok",
					Message: "WebSocket 服务配置正确 (端口 3001, 已启用)",
				})
			} else {
				if repair {
					err2 := writeDefaultOneBot11Config(cfg)
					if err2 == nil {
						steps = append(steps, DiagnoseStep{
							Step: "OneBot11 配置检测", Status: "fixed",
							Message: "已修复 WebSocket 服务配置",
							Detail:  "已重写 onebot11.json，WebSocket 端口 3001 已启用",
						})
					} else {
						steps = append(steps, DiagnoseStep{
							Step: "OneBot11 配置检测", Status: "error",
							Message: "WebSocket 未正确配置且修复失败",
							Detail:  err2.Error(),
						})
					}
				} else {
					steps = append(steps, DiagnoseStep{
						Step: "OneBot11 配置检测", Status: "error",
						Message: "WebSocket 服务未正确配置",
						Detail:  "WebSocket 端口 3001 未启用或配置错误，点击「诊断并修复」可自动修复",
					})
				}
			}
		}
	}

	// Step 6: Check webui.json
	webuiOut, err := exec.Command("docker", "exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json").Output()
	if err != nil {
		if repair {
			defaultWebUI := `{"host":"0.0.0.0","port":6099,"token":"clawpanel-qq","loginRate":3}`
			cmd := exec.Command("docker", "exec", "openclaw-qq", "bash", "-c",
				fmt.Sprintf("cat > /app/napcat/config/webui.json << 'EOF'\n%s\nEOF", defaultWebUI))
			if cmd.Run() == nil {
				steps = append(steps, DiagnoseStep{
					Step: "WebUI 配置检测", Status: "fixed",
					Message: "已创建默认 webui.json 配置",
				})
			} else {
				steps = append(steps, DiagnoseStep{
					Step: "WebUI 配置检测", Status: "error",
					Message: "webui.json 不存在且创建失败",
				})
			}
		} else {
			steps = append(steps, DiagnoseStep{
				Step: "WebUI 配置检测", Status: "error",
				Message: "webui.json 配置文件不存在",
				Detail:  "点击「诊断并修复」可自动创建",
			})
		}
	} else {
		var webuiData map[string]interface{}
		if json.Unmarshal(webuiOut, &webuiData) == nil {
			token, _ := webuiData["token"].(string)
			if token != "" {
				steps = append(steps, DiagnoseStep{
					Step: "WebUI 配置检测", Status: "ok",
					Message: "WebUI Token 已配置",
				})
			} else {
				if repair {
					webuiData["token"] = "clawpanel-qq"
					jsonBytes, _ := json.MarshalIndent(webuiData, "", "  ")
					cmd := exec.Command("docker", "exec", "openclaw-qq", "bash", "-c",
						fmt.Sprintf("cat > /app/napcat/config/webui.json << 'EOF'\n%s\nEOF", string(jsonBytes)))
					if cmd.Run() == nil {
						steps = append(steps, DiagnoseStep{
							Step: "WebUI 配置检测", Status: "fixed",
							Message: "已设置 WebUI Token",
						})
					}
				} else {
					steps = append(steps, DiagnoseStep{
						Step: "WebUI 配置检测", Status: "warning",
						Message: "WebUI Token 为空",
						Detail:  "点击「诊断并修复」可自动设置",
					})
				}
			}
		}
	}

	// If we did repairs that affected config, restart container
	needsRestart := false
	for _, s := range steps {
		if s.Status == "fixed" {
			needsRestart = true
			break
		}
	}
	if needsRestart && repair {
		exec.Command("docker", "restart", "openclaw-qq").Run()
		steps = append(steps, DiagnoseStep{
			Step: "重启容器", Status: "ok",
			Message: "已重启 openclaw-qq 容器使配置生效",
		})
		time.Sleep(8 * time.Second)
	}

	// Step 7: Check port reachability (after potential restart)
	webUIReachable := isPortDialable(6099)
	wsReachable := isPortDialable(3001)
	httpReachable := isPortDialable(3000)

	if webUIReachable {
		steps = append(steps, DiagnoseStep{
			Step: "WebUI 端口 (6099)", Status: "ok",
			Message: "WebUI 端口 6099 可达",
		})
	} else {
		steps = append(steps, DiagnoseStep{
			Step: "WebUI 端口 (6099)", Status: "warning",
			Message: "WebUI 端口 6099 不可达",
			Detail:  "NapCat WebUI 可能还在启动中，请等待几秒后重试",
		})
	}
	if wsReachable {
		steps = append(steps, DiagnoseStep{
			Step: "WS 端口 (3001)", Status: "ok",
			Message: "OneBot11 WebSocket 端口 3001 可达",
		})
	} else {
		steps = append(steps, DiagnoseStep{
			Step: "WS 端口 (3001)", Status: "warning",
			Message: "OneBot11 WebSocket 端口 3001 不可达",
			Detail:  "可能需要先登录 QQ 后 WS 服务才会启动",
		})
	}
	if httpReachable {
		steps = append(steps, DiagnoseStep{
			Step: "HTTP 端口 (3000)", Status: "ok",
			Message: "OneBot11 HTTP API 端口 3000 可达",
		})
	} else {
		steps = append(steps, DiagnoseStep{
			Step: "HTTP 端口 (3000)", Status: "warning",
			Message: "OneBot11 HTTP API 端口 3000 不可达",
			Detail:  "可能需要先登录 QQ 后 HTTP 服务才会启动",
		})
	}

	// Step 8: Check QQ login status
	if webUIReachable {
		loggedIn, nickname, qqID := checkQQLoginStatusDirect()
		if loggedIn {
			steps = append(steps, DiagnoseStep{
				Step: "QQ 登录状态", Status: "ok",
				Message: fmt.Sprintf("QQ 已登录: %s (%s)", nickname, qqID),
			})
		} else {
			steps = append(steps, DiagnoseStep{
				Step: "QQ 登录状态", Status: "warning",
				Message: "QQ 未登录",
				Detail:  "请在通道管理中扫码登录 QQ",
			})
		}
	}

	return steps
}

func diagnoseNapCatWindows(cfg *config.Config, repair bool) []DiagnoseStep {
	var steps []DiagnoseStep
	napcatDir := getNapCatShellDir(cfg)
	if napcatDir == "" {
		steps = append(steps, DiagnoseStep{
			Step: "NapCat Shell 检测", Status: "error",
			Message: "NapCat Shell 未安装",
			Detail:  "请在通道管理或系统配置中安装 NapCat",
		})
		return steps
	}
	steps = append(steps, DiagnoseStep{
		Step: "NapCat Shell 检测", Status: "ok",
		Message: "NapCat Shell 已安装",
		Detail:  napcatDir,
	})

	if isNapCatShellRunning() {
		steps = append(steps, DiagnoseStep{
			Step: "NapCat 进程检测", Status: "ok",
			Message: "NapCat 进程正在运行",
		})
	} else {
		if repair {
			restartNapCatProcess(cfg)
			time.Sleep(5 * time.Second)
			if isNapCatShellRunning() {
				steps = append(steps, DiagnoseStep{
					Step: "NapCat 进程检测", Status: "fixed",
					Message: "已启动 NapCat 进程",
				})
			} else {
				steps = append(steps, DiagnoseStep{
					Step: "NapCat 进程检测", Status: "error",
					Message: "NapCat 进程未运行且启动失败",
				})
			}
		} else {
			steps = append(steps, DiagnoseStep{
				Step: "NapCat 进程检测", Status: "error",
				Message: "NapCat 进程未运行",
				Detail:  "点击「诊断并修复」可尝试启动",
			})
		}
	}

	return steps
}

// checkQQLoginStatusDirect checks QQ login via WebUI using docker exec to read token
func checkQQLoginStatusDirect() (bool, string, string) {
	out, err := exec.Command("docker", "exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json").Output()
	if err != nil {
		return false, "", ""
	}
	var webui map[string]interface{}
	if json.Unmarshal(out, &webui) != nil {
		return false, "", ""
	}
	token, _ := webui["token"].(string)
	if token == "" {
		return false, "", ""
	}

	cred := napcatAuthWithToken(token)
	if cred == "" {
		return false, "", ""
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", "http://127.0.0.1:6099/api/QQLogin/CheckLoginStatus", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cred)
	resp, err := client.Do(req)
	if err != nil {
		return false, "", ""
	}
	defer resp.Body.Close()

	var statusResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&statusResp)
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

	req2, _ := http.NewRequest("POST", "http://127.0.0.1:6099/api/QQLogin/GetQQLoginInfo", nil)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+cred)
	resp2, err := client.Do(req2)
	if err != nil {
		return true, "", ""
	}
	defer resp2.Body.Close()
	var infoResp map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&infoResp)
	nickname := ""
	qqID := ""
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

// napcatAuthWithToken authenticates to NapCat WebUI with a given token and returns credential
func napcatAuthWithToken(token string) string {
	hash := sha256.Sum256([]byte(token + ".napcat"))
	hashStr := fmt.Sprintf("%x", hash)

	client := &http.Client{Timeout: 5 * time.Second}
	body := fmt.Sprintf(`{"hash":"%s"}`, hashStr)
	resp, err := client.Post("http://127.0.0.1:6099/api/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if code, ok := result["code"].(float64); ok && code == 0 {
		if data, ok := result["data"].(map[string]interface{}); ok {
			if cred, ok := data["Credential"].(string); ok {
				return cred
			}
		}
	}
	return ""
}

func isPortDialable(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// SystemDiagnose runs a comprehensive system environment diagnostic script
func SystemDiagnose(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var lines []string

		lines = append(lines, "=== ClawPanel 系统诊断报告 ===")
		lines = append(lines, fmt.Sprintf("诊断时间: %s", time.Now().Format("2006-01-02 15:04:05")))
		lines = append(lines, "")

		// OS Info
		lines = append(lines, "--- 操作系统 ---")
		lines = append(lines, fmt.Sprintf("平台: %s/%s", runtime.GOOS, runtime.GOARCH))
		lines = append(lines, fmt.Sprintf("CPU 核心: %d", runtime.NumCPU()))
		if runtime.GOOS != "windows" {
			if out := runDiagCmd("uname", "-a"); out != "" {
				lines = append(lines, "内核: "+out)
			}
			if out := runDiagCmd("cat", "/etc/os-release"); out != "" {
				for _, l := range strings.Split(out, "\n") {
					if strings.HasPrefix(l, "PRETTY_NAME=") || strings.HasPrefix(l, "VERSION=") {
						lines = append(lines, "  "+l)
					}
				}
			}
			if out := runDiagCmd("free", "-h"); out != "" {
				for _, l := range strings.Split(out, "\n") {
					if strings.HasPrefix(l, "Mem:") || strings.HasPrefix(l, "Swap:") {
						lines = append(lines, "  "+l)
					}
				}
			}
			if out := runDiagCmd("df", "-h", "/"); out != "" {
				lines = append(lines, "磁盘:")
				for _, l := range strings.Split(out, "\n") {
					lines = append(lines, "  "+l)
				}
			}
		}
		lines = append(lines, "")

		// Software versions
		lines = append(lines, "--- 软件环境 ---")
		swChecks := []struct{ name, cmd string; args []string }{
			{"Node.js", "node", []string{"--version"}},
			{"npm", "npm", []string{"--version"}},
			{"Docker", "docker", []string{"--version"}},
			{"Git", "git", []string{"--version"}},
			{"Python3", "python3", []string{"--version"}},
		}
		for _, sw := range swChecks {
			v := runDiagCmd(sw.cmd, sw.args...)
			if v != "" {
				lines = append(lines, fmt.Sprintf("%s: %s", sw.name, v))
			} else {
				lines = append(lines, fmt.Sprintf("%s: 未安装", sw.name))
			}
		}
		lines = append(lines, "")

		// OpenClaw specific
		lines = append(lines, "--- OpenClaw ---")
		ocVer := detectCmd("openclaw", "--version")
		if ocVer != "" {
			lines = append(lines, "版本: "+ocVer)
		} else {
			lines = append(lines, "版本: 未检测到 (openclaw 不在 PATH 中)")
		}
		// Which openclaw
		if runtime.GOOS != "windows" {
			if wp := runDiagCmd("which", "openclaw"); wp != "" {
				lines = append(lines, "路径: "+wp)
			}
		}
		// npm global root
		if nr := runDiagCmd("npm", "root", "-g"); nr != "" {
			lines = append(lines, "npm 全局模块: "+nr)
		}
		lines = append(lines, fmt.Sprintf("配置目录: %s", cfg.OpenClawDir))
		lines = append(lines, fmt.Sprintf("工作目录: %s", cfg.OpenClawWork))
		lines = append(lines, fmt.Sprintf("应用目录: %s", cfg.OpenClawApp))

		// Config file check
		ocConfig, err := cfg.ReadOpenClawJSON()
		if err != nil {
			lines = append(lines, "openclaw.json: ❌ "+err.Error())
		} else {
			lines = append(lines, "openclaw.json: ✅ 存在")
			// Model check
			if models, ok := ocConfig["models"].(map[string]interface{}); ok {
				if providers, ok := models["providers"].(map[string]interface{}); ok {
					lines = append(lines, fmt.Sprintf("模型提供商: %d 个", len(providers)))
					for pid, prov := range providers {
						pm, _ := prov.(map[string]interface{})
						apiKey := ""
						if pm != nil {
							apiKey, _ = pm["apiKey"].(string)
						}
						if apiKey != "" {
							lines = append(lines, fmt.Sprintf("  %s: ✅ 已配置 API Key", pid))
						} else {
							lines = append(lines, fmt.Sprintf("  %s: ❌ 缺少 API Key", pid))
						}
					}
				}
				if defaultModel, ok := models["default"].(string); ok && defaultModel != "" {
					lines = append(lines, "默认模型: "+defaultModel)
				}
			} else {
				lines = append(lines, "模型配置: ❌ 未配置")
			}
			// Channels
			if channels, ok := ocConfig["channels"].(map[string]interface{}); ok {
				for chID, ch := range channels {
					chMap, _ := ch.(map[string]interface{})
					if chMap == nil {
						continue
					}
					enabled, _ := chMap["enabled"].(bool)
					if enabled {
						lines = append(lines, fmt.Sprintf("通道 %s: ✅ 已启用", chID))
					}
				}
			}
		}

		// Skills
		lines = append(lines, "")
		lines = append(lines, "--- 技能 ---")
		skillDirs := []string{
			filepath.Join(cfg.OpenClawDir, "skills"),
			filepath.Join(cfg.OpenClawApp, "skills"),
		}
		totalSkills := 0
		for _, sd := range skillDirs {
			entries, err := os.ReadDir(sd)
			if err != nil {
				lines = append(lines, fmt.Sprintf("技能目录 %s: 不存在或无法读取", sd))
				continue
			}
			count := 0
			for _, e := range entries {
				if e.IsDir() {
					count++
				}
			}
			totalSkills += count
			lines = append(lines, fmt.Sprintf("技能目录 %s: %d 个技能", sd, count))
		}
		lines = append(lines, fmt.Sprintf("总技能数: %d", totalSkills))

		// Extensions
		lines = append(lines, "")
		lines = append(lines, "--- 扩展/插件 ---")
		extDirs := []string{
			filepath.Join(cfg.OpenClawDir, "extensions"),
			filepath.Join(cfg.OpenClawApp, "extensions"),
		}
		for _, ed := range extDirs {
			entries, err := os.ReadDir(ed)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() {
					lines = append(lines, fmt.Sprintf("  %s", e.Name()))
				}
			}
		}

		// NapCat
		lines = append(lines, "")
		lines = append(lines, "--- NapCat ---")
		if runtime.GOOS == "windows" {
			napcatDir := getNapCatShellDir(cfg)
			if napcatDir != "" {
				lines = append(lines, "安装模式: Shell (Windows)")
				lines = append(lines, "安装路径: "+napcatDir)
				if isNapCatShellRunning() {
					lines = append(lines, "状态: ✅ 运行中")
				} else {
					lines = append(lines, "状态: ❌ 未运行")
				}
			} else {
				lines = append(lines, "状态: 未安装")
			}
		} else {
			containerOut, _ := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", "openclaw-qq").Output()
			cs := strings.TrimSpace(string(containerOut))
			if cs != "" {
				lines = append(lines, "安装模式: Docker 容器")
				lines = append(lines, "容器状态: "+cs)
				// Port check
				for _, port := range []int{6099, 3001, 3000} {
					if isPortDialable(port) {
						lines = append(lines, fmt.Sprintf("端口 %d: ✅ 可达", port))
					} else {
						lines = append(lines, fmt.Sprintf("端口 %d: ❌ 不可达", port))
					}
				}
			} else {
				lines = append(lines, "状态: 未安装")
			}
		}

		// PATH
		lines = append(lines, "")
		lines = append(lines, "--- PATH ---")
		pathVal := os.Getenv("PATH")
		for _, p := range strings.Split(pathVal, string(os.PathListSeparator)) {
			lines = append(lines, "  "+p)
		}

		lines = append(lines, "")
		lines = append(lines, "=== 诊断完成 ===")

		c.JSON(http.StatusOK, gin.H{
			"ok":     true,
			"report": strings.Join(lines, "\n"),
		})
	}
}

func runDiagCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
