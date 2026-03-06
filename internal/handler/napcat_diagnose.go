package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
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
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
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
func DiagnoseNapCat(cfg *config.Config, procMgr ...*process.Manager) gin.HandlerFunc {
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

		// If channel.ts was fixed, auto-restart OpenClaw so the fix takes effect
		if repair && len(procMgr) > 0 && procMgr[0] != nil {
			for _, s := range steps {
				if s.Step == "QQ 插件 startAccount 检测" && s.Status == "fixed" {
					log.Println("[Diagnose] channel.ts 已修复，自动重启 OpenClaw...")
					if err := procMgr[0].Restart(); err != nil {
						steps = append(steps, DiagnoseStep{
							Step: "重启 OpenClaw", Status: "error",
							Message: "OpenClaw 自动重启失败: " + err.Error(),
						})
					} else {
						steps = append(steps, DiagnoseStep{
							Step: "重启 OpenClaw", Status: "ok",
							Message: "已自动重启 OpenClaw 使 channel.ts 修复生效",
						})
					}
					break
				}
			}
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
	dockerVer, err := dockerOutput("--version")
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
	err = dockerRun("info")
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
	out, err := dockerOutput("inspect", "--format", "{{.State.Status}}", "openclaw-qq")
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
			dockerRun("start", "openclaw-qq")
			time.Sleep(5 * time.Second)
			out2, _ := dockerOutput("inspect", "--format", "{{.State.Status}}", "openclaw-qq")
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
	portOut, _ := dockerOutput("port", "openclaw-qq")
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
	obOut, err := dockerOutput("exec", "openclaw-qq", "cat", "/app/napcat/config/onebot11.json")
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
	webuiOut, err := dockerOutput("exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json")
	if err != nil {
		if repair {
			defaultWebUI := `{"host":"0.0.0.0","port":6099,"token":"clawpanel-qq","loginRate":3}`
			if dockerRun("exec", "openclaw-qq", "bash", "-c", fmt.Sprintf("cat > /app/napcat/config/webui.json << 'EOF'\n%s\nEOF", defaultWebUI)) == nil {
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
					if dockerRun("exec", "openclaw-qq", "bash", "-c", fmt.Sprintf("cat > /app/napcat/config/webui.json << 'EOF'\n%s\nEOF", string(jsonBytes))) == nil {
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

	// Step 6.5: Check token consistency (Docker WEBUI_TOKEN env vs webui.json) and auth
	webuiJsonToken := ""
	if webuiOut != nil {
		var webuiParsed map[string]interface{}
		if json.Unmarshal(webuiOut, &webuiParsed) == nil {
			webuiJsonToken, _ = webuiParsed["token"].(string)
		}
	}
	// Read Docker WEBUI_TOKEN env
	dockerEnvToken := ""
	envOut, envErr := dockerOutput("inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", "openclaw-qq")
	if envErr == nil {
		for _, line := range strings.Split(string(envOut), "\n") {
			if strings.HasPrefix(line, "WEBUI_TOKEN=") {
				dockerEnvToken = strings.TrimPrefix(line, "WEBUI_TOKEN=")
				break
			}
		}
	}
	if webuiJsonToken != "" && dockerEnvToken != "" && webuiJsonToken != dockerEnvToken {
		if repair {
			// Fix: update webui.json to match Docker env var (env var is more authoritative since NapCat uses it on startup)
			var webuiFixData map[string]interface{}
			json.Unmarshal(webuiOut, &webuiFixData)
			if webuiFixData == nil {
				webuiFixData = map[string]interface{}{}
			}
			webuiFixData["token"] = dockerEnvToken
			fixBytes, _ := json.MarshalIndent(webuiFixData, "", "  ")
			if dockerRun("exec", "openclaw-qq", "bash", "-c", fmt.Sprintf("cat > /app/napcat/config/webui.json << 'FIXEOF'\n%s\nFIXEOF", string(fixBytes))) == nil {
				webuiJsonToken = dockerEnvToken
				steps = append(steps, DiagnoseStep{
					Step: "Token 一致性检测", Status: "fixed",
					Message: "已修复 webui.json Token 与 Docker 环境变量不一致",
					Detail:  fmt.Sprintf("webui.json Token 已更新为 \"%s\"（与 WEBUI_TOKEN 环境变量一致）", dockerEnvToken),
				})
			} else {
				steps = append(steps, DiagnoseStep{
					Step: "Token 一致性检测", Status: "error",
					Message: "webui.json Token 与 Docker WEBUI_TOKEN 不一致且修复失败",
					Detail:  fmt.Sprintf("webui.json: \"%s\", WEBUI_TOKEN: \"%s\"", webuiJsonToken, dockerEnvToken),
				})
			}
		} else {
			steps = append(steps, DiagnoseStep{
				Step: "Token 一致性检测", Status: "error",
				Message: "webui.json Token 与 Docker WEBUI_TOKEN 环境变量不一致",
				Detail:  fmt.Sprintf("webui.json: \"%s\", WEBUI_TOKEN: \"%s\"。这会导致扫码登录时出现 Unauthorized 错误。点击「诊断并修复」可自动修复", webuiJsonToken, dockerEnvToken),
			})
		}
	} else if webuiJsonToken != "" {
		steps = append(steps, DiagnoseStep{
			Step: "Token 一致性检测", Status: "ok",
			Message: "WebUI Token 一致",
		})
	}

	// Step 6.6: Verify ClawPanel can actually authenticate to NapCat WebUI
	if webuiJsonToken != "" && isPortDialable(6099) {
		authCred := napcatAuthWithToken(webuiJsonToken)
		if authCred != "" {
			steps = append(steps, DiagnoseStep{
				Step: "WebUI 鉴权测试", Status: "ok",
				Message: "ClawPanel 可成功认证 NapCat WebUI",
			})
		} else {
			if repair {
				// Try the Docker env token if different
				if dockerEnvToken != "" && dockerEnvToken != webuiJsonToken {
					authCred = napcatAuthWithToken(dockerEnvToken)
				}
				if authCred == "" {
					// Last resort: reset token to default and restart
					defaultToken := "clawpanel-qq"
					fixData := map[string]interface{}{"host": "0.0.0.0", "port": 6099, "token": defaultToken, "loginRate": 3}
					fixBytes, _ := json.MarshalIndent(fixData, "", "  ")
					dockerRun("exec", "openclaw-qq", "bash", "-c", fmt.Sprintf("cat > /app/napcat/config/webui.json << 'FIXEOF'\n%s\nFIXEOF", string(fixBytes)))
					steps = append(steps, DiagnoseStep{
						Step: "WebUI 鉴权测试", Status: "fixed",
						Message: "WebUI 鉴权失败，已重置 Token 为默认值",
						Detail:  "Token 已重置为 \"clawpanel-qq\"，需重启容器生效",
					})
				} else {
					steps = append(steps, DiagnoseStep{
						Step: "WebUI 鉴权测试", Status: "ok",
						Message: "使用 Docker 环境变量 Token 鉴权成功",
					})
				}
			} else {
				steps = append(steps, DiagnoseStep{
					Step: "WebUI 鉴权测试", Status: "error",
					Message: "ClawPanel 无法认证 NapCat WebUI（Unauthorized）",
					Detail:  "Token 可能不匹配或 NapCat 内部状态异常。点击「诊断并修复」可尝试重置 Token",
				})
			}
		}
	}

	// Step 6.7: Check QQ plugin channel.ts startAccount bug
	// ROOT CAUSE: startAccount returns cleanup function instead of long-lived Promise,
	// causing OpenClaw gateway auto-restart loop and eventually dead channel handler.
	steps = append(steps, diagnoseChannelTSStartAccount(cfg, repair)...)

	// If we did repairs that affected config, restart container
	needsRestart := false
	for _, s := range steps {
		if s.Status == "fixed" {
			needsRestart = true
			break
		}
	}
	if needsRestart && repair {
		dockerRun("restart", "openclaw-qq")
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
	out, err := dockerOutput("exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json")
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
		swChecks := []struct {
			name, cmd string
			args      []string
		}{
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
			containerOut, _ := dockerOutput("inspect", "--format", "{{.State.Status}}", "openclaw-qq")
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

// diagnoseChannelTSStartAccount checks and optionally fixes the QQ plugin's
// startAccount bug where it returns a cleanup function instead of a long-lived
// Promise, causing OpenClaw gateway to trigger auto-restart loop.
func diagnoseChannelTSStartAccount(cfg *config.Config, repair bool) []DiagnoseStep {
	var steps []DiagnoseStep
	ocDir := cfg.OpenClawDir
	if ocDir == "" {
		home, _ := os.UserHomeDir()
		ocDir = filepath.Join(home, ".openclaw")
	}
	channelTS := filepath.Join(ocDir, "extensions", "qq", "src", "channel.ts")
	data, err := os.ReadFile(channelTS)
	if err != nil {
		return steps // plugin not installed, skip silently
	}
	content := string(data)

	if strings.Contains(content, "new Promise") {
		steps = append(steps, DiagnoseStep{
			Step: "QQ 插件 startAccount 检测", Status: "ok",
			Message: "channel.ts startAccount 已使用 Promise 模式",
		})
		return steps
	}

	if !strings.Contains(content, "return () => {") || !strings.Contains(content, "client.disconnect") {
		return steps // unknown structure, skip
	}

	// Bug detected
	if !repair {
		steps = append(steps, DiagnoseStep{
			Step: "QQ 插件 startAccount 检测", Status: "error",
			Message: "channel.ts startAccount 返回 cleanup 函数（导致消息不回复）",
			Detail:  "startAccount 应返回 long-lived Promise。当前写法导致 OpenClaw 触发 auto-restart 循环，10 次后 channel handler 死亡。点击「诊断并修复」可自动修复",
		})
		return steps
	}

	oldCode := "      client.connect();\n      \n      return () => {\n        client.disconnect();\n        clients.delete(account.accountId);\n        stopFileServer();\n      };"
	newCode := `      client.connect();

      // Return a Promise that stays pending until abortSignal fires.
      // OpenClaw gateway expects startAccount to return a long-lived Promise;
      // if it resolves immediately, the framework treats the account as exited
      // and triggers auto-restart attempts.
      const abortSignal = (ctx as any).abortSignal as AbortSignal | undefined;
      return new Promise<void>((resolve) => {
        const cleanup = () => {
          client.disconnect();
          clients.delete(account.accountId);
          stopFileServer();
          resolve();
        };
        if (abortSignal) {
          if (abortSignal.aborted) { cleanup(); return; }
          abortSignal.addEventListener("abort", cleanup, { once: true });
        }
        // Also clean up if the WebSocket closes unexpectedly
        client.on("close", () => {
          cleanup();
        });
      });`

	if !strings.Contains(content, oldCode) {
		steps = append(steps, DiagnoseStep{
			Step: "QQ 插件 startAccount 检测", Status: "warning",
			Message: "channel.ts 需要修复但代码模式不完全匹配",
			Detail:  "请手动编辑 " + channelTS + " 将 startAccount 的 return cleanup 函数改为 return new Promise",
		})
		return steps
	}

	patched := strings.Replace(content, oldCode, newCode, 1)
	if err := os.WriteFile(channelTS, []byte(patched), 0644); err != nil {
		steps = append(steps, DiagnoseStep{
			Step: "QQ 插件 startAccount 检测", Status: "error",
			Message: "channel.ts 修复写入失败: " + err.Error(),
		})
		return steps
	}

	steps = append(steps, DiagnoseStep{
		Step: "QQ 插件 startAccount 检测", Status: "fixed",
		Message: "已修复 channel.ts startAccount（返回 long-lived Promise）",
		Detail:  "这是导致 QQ 消息不回复的根本原因。修复后需重启 OpenClaw 生效",
	})
	return steps
}

func runDiagCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
