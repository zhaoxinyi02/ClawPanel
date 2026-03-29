package handler

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

const (
	defaultWechatBridgeURL   = "http://127.0.0.1:19088"
	defaultWechatBridgeToken = "clawpanel-wcf"
)

// === Admin Config ===

func GetAdminConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminCfg := loadAdminConfig(cfg)
		c.JSON(200, gin.H{"ok": true, "config": adminCfg})
	}
}

func SaveAdminConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"ok": false, "error": err.Error()})
			return
		}
		saveAdminConfigData(cfg, body)
		c.JSON(200, gin.H{"ok": true})
	}
}

func SaveAdminSection(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		section := c.Param("section")
		var body interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"ok": false, "error": err.Error()})
			return
		}
		adminCfg := loadAdminConfig(cfg)
		adminCfg[section] = body
		saveAdminConfigData(cfg, adminCfg)
		c.JSON(200, gin.H{"ok": true})
	}
}

func adminConfigPath(cfg *config.Config) string {
	return filepath.Join(cfg.DataDir, "admin-config.json")
}

func loadAdminConfig(cfg *config.Config) map[string]interface{} {
	result := map[string]interface{}{
		"server": map[string]interface{}{
			"token": cfg.AdminToken,
			"port":  cfg.Port,
		},
	}
	data, err := os.ReadFile(adminConfigPath(cfg))
	if err == nil {
		json.Unmarshal(data, &result)
	}
	return result
}

func saveAdminConfigData(cfg *config.Config, data map[string]interface{}) {
	out, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(adminConfigPath(cfg), out, 0644)
}

// === Admin Token ===

func GetAdminToken(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "token": cfg.AdminToken})
	}
}

// === Sudo Password ===

func GetSudoPassword(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminCfg := loadAdminConfig(cfg)
		sys, _ := adminCfg["system"].(map[string]interface{})
		hasPwd := false
		if sys != nil {
			if pwd, ok := sys["sudoPassword"].(string); ok && pwd != "" {
				hasPwd = true
			}
		}
		c.JSON(200, gin.H{"ok": true, "configured": hasPwd})
	}
}

func SetSudoPassword(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Password string `json:"password"`
		}
		c.ShouldBindJSON(&body)
		adminCfg := loadAdminConfig(cfg)
		sys, ok := adminCfg["system"].(map[string]interface{})
		if !ok {
			sys = map[string]interface{}{}
		}
		sys["sudoPassword"] = body.Password
		adminCfg["system"] = sys
		saveAdminConfigData(cfg, adminCfg)
		c.JSON(200, gin.H{"ok": true})
	}
}

// === Bot Operations (OneBot proxy) ===

func onebotWsUrl(cfg *config.Config) string {
	ocCfg := readOpenClawJSON(cfg)
	if channels, ok := ocCfg["channels"].(map[string]interface{}); ok {
		if qq, ok := channels["qq"].(map[string]interface{}); ok {
			if wsUrl, ok := qq["wsUrl"].(string); ok && wsUrl != "" {
				return wsUrl
			}
		}
	}
	return "ws://127.0.0.1:3001"
}

func GetBotGroups(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "groups": []interface{}{}})
	}
}

func GetBotFriends(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "friends": []interface{}{}})
	}
}

func BotSend(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	}
}

func BotReconnect(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	}
}

// === Requests (approval) ===

func GetRequests(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "requests": []interface{}{}})
	}
}

func ApproveRequest(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	}
}

func RejectRequest(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	}
}

// === NapCat Login Proxy ===

const NAPCAT_WEBUI = "http://127.0.0.1:6099"

var napcatCredential string
var napcatCredentialToken string

func napcatProxy(method, path string, body interface{}, credential string) (map[string]interface{}, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, NAPCAT_WEBUI+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]interface{}{"raw": string(data)}, nil
	}
	return result, nil
}

// readNapCatWebuiToken reads the actual token NapCat is using.
// NapCat 4.x generates a random token on each startup and logs it as:
//
//	[WebUi] WebUi Token: <token>
//
// So we parse the latest log file to get the live token.
func readNapCatWebuiToken(cfg *config.Config) string {
	if runtime.GOOS == "windows" {
		napcatDir := getNapCatShellDir(cfg)
		if napcatDir == "" {
			return ""
		}
		// Walk up from bootmain dir to find the napcat logs directory
		// Structure: <install>/versions/<ver>/resources/app/napcat/logs
		logsDir := ""
		filepath.WalkDir(napcatDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || logsDir != "" {
				return nil
			}
			if d.IsDir() && strings.EqualFold(d.Name(), "logs") {
				// Verify it looks like a napcat logs dir (has .log files)
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
		// Also check parent dirs up 6 levels from napcatDir
		if logsDir == "" {
			dir := napcatDir
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
		if logsDir != "" {
			if tok := tokenFromNapCatLogs(logsDir); tok != "" {
				return tok
			}
		}
		// Fallback: try webui.json in bootmain config
		data, err := os.ReadFile(filepath.Join(napcatDir, "config", "webui.json"))
		if err == nil {
			var webui map[string]interface{}
			if json.Unmarshal(data, &webui) == nil {
				if t, ok := webui["token"].(string); ok && t != "" {
					return t
				}
			}
		}
	} else {
		out, err := exec.Command("docker", "exec", "openclaw-qq", "cat", "/app/napcat/config/webui.json").Output()
		if err == nil {
			var webui map[string]interface{}
			if json.Unmarshal(out, &webui) == nil {
				if t, ok := webui["token"].(string); ok && t != "" {
					return t
				}
			}
		}
	}
	return ""
}

// tokenFromNapCatLogs finds the most recent NapCat log file and extracts the WebUI token.
func tokenFromNapCatLogs(logsDir string) string {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return ""
	}
	// Find the newest .log file
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
	// Look for: [WebUi] WebUi Token: <token>
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

// napcatLoginWithToken attempts to authenticate with NapCat WebUI using a given token.
func napcatLoginWithToken(token string) string {
	hash := sha256.Sum256([]byte(token + ".napcat"))
	hashStr := fmt.Sprintf("%x", hash)
	r, err := napcatProxy("POST", "/api/auth/login", map[string]string{"hash": hashStr}, "")
	if err == nil {
		if code, ok := r["code"].(float64); ok && code == 0 {
			if data, ok := r["data"].(map[string]interface{}); ok {
				if cred, ok := data["Credential"].(string); ok {
					return cred
				}
			}
		}
	}
	return ""
}

func napcatAuth(cfg *config.Config) string {
	if napcatCredential != "" {
		containerToken := readNapCatWebuiToken(cfg)
		if containerToken == "" || napcatCredentialToken == "" || containerToken == napcatCredentialToken {
			return napcatCredential
		}
		// Token changed after NapCat restart; cached credential is no longer trustworthy.
		napcatCredential = ""
		napcatCredentialToken = ""
	}

	// 1. Try the actual token from NapCat's log (NapCat 4.x generates random token each startup)
	containerToken := readNapCatWebuiToken(cfg)
	if containerToken != "" {
		if cred := napcatLoginWithToken(containerToken); cred != "" {
			napcatCredential = cred
			napcatCredentialToken = containerToken
			return napcatCredential
		}
		// Token changed (NapCat restarted) — clear stale credential
		napcatCredential = ""
		napcatCredentialToken = ""
	}

	// 2. Fallback: admin-config or default token (may differ from container)
	adminCfg := loadAdminConfig(cfg)
	webuiToken := "clawpanel-qq"
	if napcat, ok := adminCfg["napcat"].(map[string]interface{}); ok {
		if t, ok := napcat["webuiToken"].(string); ok && t != "" {
			webuiToken = t
		}
	}
	if envToken := os.Getenv("WEBUI_TOKEN"); envToken != "" {
		webuiToken = envToken
	}
	if webuiToken != containerToken {
		if cred := napcatLoginWithToken(webuiToken); cred != "" {
			napcatCredential = cred
			napcatCredentialToken = webuiToken
			return napcatCredential
		}
	}

	return napcatCredential
}

func isNapcatUnauthorized(r map[string]interface{}) bool {
	// Check code == -1 with unauthorized message
	if code, ok := r["code"].(float64); ok && code == -1 {
		if msg, ok := r["message"].(string); ok {
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "auth") {
				return true
			}
		}
	}
	// Check raw response (NapCat may return non-JSON 401)
	if raw, ok := r["raw"].(string); ok {
		lower := strings.ToLower(raw)
		if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "401") {
			return true
		}
	}
	return false
}

func napcatApiCall(cfg *config.Config, method, path string, body interface{}) (map[string]interface{}, error) {
	cred := napcatAuth(cfg)
	r, err := napcatProxy(method, path, body, cred)
	if err != nil {
		return nil, fmt.Errorf("NapCat 服务不可达，可能正在启动中，请稍候重试")
	}
	// If unauthorized, clear cache and retry with fresh auth
	if isNapcatUnauthorized(r) {
		napcatCredential = ""
		napcatCredentialToken = ""
		cred = napcatAuth(cfg)
		if cred != "" {
			return napcatProxy(method, path, body, cred)
		}
		// Auth still fails — NapCat may be restarting, return friendly message
		return nil, fmt.Errorf("NapCat 认证失败，服务可能正在重启中，请等待几秒后重试")
	}
	return r, nil
}

func NapcatLoginStatus(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		r, err := napcatApiCall(cfg, "POST", "/api/QQLogin/CheckLoginStatus", nil)
		if err != nil {
			c.JSON(200, gin.H{"ok": false, "error": err.Error()})
			return
		}
		r["ok"] = true
		c.JSON(200, r)
	}
}

func NapcatGetQRCode(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retry up to 5 times (~10s) waiting for NapCat to become ready
		var r map[string]interface{}
		var err error
		for i := 0; i < 5; i++ {
			r, err = napcatApiCall(cfg, "POST", "/api/QQLogin/GetQQLoginQrcode", nil)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
			napcatCredential = ""
		}

		// If API failed, try CheckLoginStatus which also returns qrcodeurl
		if err != nil || r == nil {
			if r2, err2 := napcatApiCall(cfg, "POST", "/api/QQLogin/CheckLoginStatus", nil); err2 == nil {
				if data2, ok := r2["data"].(map[string]interface{}); ok {
					if u, ok := data2["qrcodeurl"].(string); ok && u != "" {
						r = map[string]interface{}{"data": map[string]interface{}{"qrcode": u}}
						err = nil
					}
				}
			}
		}

		if err != nil {
			// Last resort: serve the cached qrcode.png NapCat wrote to disk
			if pngData := readNapCatQRCodePNG(cfg); len(pngData) > 0 {
				c.JSON(200, gin.H{
					"ok":   true,
					"code": 0,
					"data": gin.H{"qrcode": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData)},
				})
				return
			}
			c.JSON(200, gin.H{"ok": false, "error": err.Error()})
			return
		}

		r["ok"] = true
		// NapCat returns a URL in data.qrcode — convert to base64 QR image
		if data, ok := r["data"].(map[string]interface{}); ok {
			if qrURL, ok := data["qrcode"].(string); ok && qrURL != "" && !strings.HasPrefix(qrURL, "data:") {
				if png, encErr := qrcode.Encode(qrURL, qrcode.Medium, 256); encErr == nil {
					data["qrcode"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
				}
			}
		}
		c.JSON(200, r)
	}
}

// readNapCatQRCodePNG reads the qrcode.png that NapCat saves to its cache directory.
func readNapCatQRCodePNG(cfg *config.Config) []byte {
	napcatDir := getNapCatShellDir(cfg)
	if napcatDir == "" {
		return nil
	}
	// Walk to find cache/qrcode.png
	var pngPath string
	filepath.WalkDir(napcatDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || pngPath != "" {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "qrcode.png") {
			pngPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if pngPath == "" {
		return nil
	}
	// Only use if written within last 3 minutes (QR codes expire)
	info, err := os.Stat(pngPath)
	if err != nil || time.Since(info.ModTime()) > 3*time.Minute {
		return nil
	}
	data, _ := os.ReadFile(pngPath)
	return data
}

func NapcatRefreshQRCode(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the old QR URL first so we can detect if it actually changed
		oldR, _ := napcatApiCall(cfg, "POST", "/api/QQLogin/GetQQLoginQrcode", nil)
		oldURL := ""
		if oldData, ok := oldR["data"].(map[string]interface{}); ok {
			oldURL, _ = oldData["qrcode"].(string)
		}

		// Call RefreshQRcode to invalidate old QR
		napcatApiCall(cfg, "POST", "/api/QQLogin/RefreshQRcode", nil)
		time.Sleep(500 * time.Millisecond)

		// Retry up to 5 times to get a genuinely new QR code
		var r map[string]interface{}
		var err error
		for i := 0; i < 5; i++ {
			r, err = napcatApiCall(cfg, "POST", "/api/QQLogin/GetQQLoginQrcode", nil)
			if err == nil {
				if data, ok := r["data"].(map[string]interface{}); ok {
					if newURL, ok := data["qrcode"].(string); ok && newURL != "" && newURL != oldURL {
						break // Got a new QR code
					}
				}
			}
			time.Sleep(800 * time.Millisecond)
		}

		if err != nil {
			c.JSON(200, gin.H{"ok": false, "error": err.Error()})
			return
		}
		r["ok"] = true
		// NapCat returns a URL in data.qrcode — convert to base64 QR image
		if data, ok := r["data"].(map[string]interface{}); ok {
			if qrURL, ok := data["qrcode"].(string); ok && qrURL != "" && !strings.HasPrefix(qrURL, "data:") {
				if png, err := qrcode.Encode(qrURL, qrcode.Medium, 256); err == nil {
					data["qrcode"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
				}
			}
		}
		c.JSON(200, r)
	}
}

func NapcatQuickLoginList(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		r, err := napcatApiCall(cfg, "POST", "/api/QQLogin/GetQuickLoginQQ", nil)
		if err != nil {
			c.JSON(200, gin.H{"ok": false, "error": err.Error()})
			return
		}
		r["ok"] = true
		c.JSON(200, r)
	}
}

func NapcatQuickLogin(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Uin string `json:"uin"`
		}
		c.ShouldBindJSON(&body)
		r, err := napcatApiCall(cfg, "POST", "/api/QQLogin/SetQuickLogin", map[string]string{"uin": body.Uin})
		if err != nil {
			c.JSON(200, gin.H{"ok": false, "error": err.Error()})
			return
		}
		r["ok"] = true
		c.JSON(200, r)
	}
}

func NapcatPasswordLogin(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Uin      string `json:"uin"`
			Password string `json:"password"`
		}
		c.ShouldBindJSON(&body)
		pwd := body.Password
		hash := md5.Sum([]byte(pwd))
		passwordMd5 := fmt.Sprintf("%x", hash)
		r, err := napcatApiCall(cfg, "POST", "/api/QQLogin/PasswordLogin", map[string]string{"uin": body.Uin, "passwordMd5": passwordMd5})
		if err != nil {
			c.JSON(200, gin.H{"ok": false, "error": err.Error()})
			return
		}
		r["ok"] = true
		c.JSON(200, r)
	}
}

func NapcatLoginInfo(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		r, err := napcatApiCall(cfg, "POST", "/api/QQLogin/GetQQLoginInfo", nil)
		if err != nil {
			c.JSON(200, gin.H{"ok": false, "error": err.Error()})
			return
		}
		r["ok"] = true
		c.JSON(200, r)
	}
}

func NapcatLogout(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		go func() {
			restartNapCatProcess(cfg)
		}()
		c.JSON(200, gin.H{"ok": true, "message": "QQ 正在退出登录，NapCat 重启中..."})
	}
}

func RestartNapcat(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		go func() {
			restartNapCatProcess(cfg)
		}()
		c.JSON(200, gin.H{"ok": true, "message": "NapCat 正在重启..."})
	}
}

// restartNapCatProcess restarts NapCat based on platform: Docker on Linux/macOS, Shell process on Windows
func restartNapCatProcess(cfg *config.Config) {
	if runtime.GOOS == "windows" {
		// Kill NapCat processes on Windows
		exec.Command("taskkill", "/F", "/IM", "NapCatWinBootMain.exe").Run()
		exec.Command("taskkill", "/F", "/IM", "napcat.exe").Run()
		exec.Command("taskkill", "/F", "/IM", "QQ.exe").Run()
		// Restart: launch NapCatWinBootMain.exe in the interactive user session via schtasks.
		// ClawPanel runs as SYSTEM (session 0); direct exec cannot create GUI processes in
		// the user's desktop session (session 1). schtasks runs as the interactive user.
		napcatDir := getNapCatShellDir(cfg)
		if napcatDir != "" {
			exePath := filepath.Join(napcatDir, "NapCatWinBootMain.exe")
			if _, err := os.Stat(exePath); err == nil {
				launchInUserSession(exePath, napcatDir)
			}
		}
	} else {
		exec.Command("docker", "restart", "openclaw-qq").Run()
	}
}

// === WeChat API ===

func wechatBridgeKitDir(cfg *config.Config) string {
	return filepath.Join(cfg.DataDir, "wechat-wcf-bridge")
}

func normalizeWechatConfig(cfg *config.Config, raw map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"mode":        "wcferry",
		"bridgeUrl":   defaultWechatBridgeURL,
		"bridgeToken": defaultWechatBridgeToken,
		"kitDir":      wechatBridgeKitDir(cfg),
	}
	for k, v := range raw {
		out[k] = v
	}
	if s := strings.TrimSpace(fmt.Sprint(out["mode"])); s != "" {
		out["mode"] = s
	}
	if s := strings.TrimSpace(fmt.Sprint(out["bridgeUrl"])); s != "" {
		out["bridgeUrl"] = strings.TrimRight(s, "/")
	}
	if s := strings.TrimSpace(fmt.Sprint(out["bridgeToken"])); s != "" {
		out["bridgeToken"] = s
	}
	out["kitDir"] = wechatBridgeKitDir(cfg)
	return out
}

func loadWechatConfigMap(cfg *config.Config) map[string]interface{} {
	adminCfg := loadAdminConfig(cfg)
	if wc, ok := adminCfg["wechat"].(map[string]interface{}); ok && wc != nil {
		return normalizeWechatConfig(cfg, wc)
	}
	return normalizeWechatConfig(cfg, map[string]interface{}{})
}

func wechatBridgeRequest(cfg *config.Config, method, path string, body interface{}) (map[string]interface{}, error) {
	wc := loadWechatConfigMap(cfg)
	baseURL := strings.TrimRight(fmt.Sprint(wc["bridgeUrl"]), "/")
	if baseURL == "" {
		baseURL = defaultWechatBridgeURL
	}
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(fmt.Sprint(wc["bridgeToken"])); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if len(data) == 0 {
		result = map[string]interface{}{}
	} else if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("bridge returned invalid json: %w", err)
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(fmt.Sprint(result["error"]))
		if msg == "" {
			msg = strings.TrimSpace(fmt.Sprint(result["message"]))
		}
		if msg == "" {
			msg = resp.Status
		}
			return nil, fmt.Errorf("%s", msg)
	}
	return result, nil
}

func WechatStatus(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wc := loadWechatConfigMap(cfg)
		resp := gin.H{
			"ok":         true,
			"mode":       wc["mode"],
			"bridgeUrl":  wc["bridgeUrl"],
			"kitDir":     wc["kitDir"],
			"configured": strings.TrimSpace(fmt.Sprint(wc["bridgeUrl"])) != "",
			"connected":  false,
			"loggedIn":   false,
			"name":       "",
		}
		bridgeResp, err := wechatBridgeRequest(cfg, http.MethodGet, "/status", nil)
		if err != nil {
			resp["error"] = err.Error()
			c.JSON(http.StatusOK, resp)
			return
		}
		for _, key := range []string{"connected", "loggedIn", "name", "selfWxid", "contacts", "rooms", "version", "message"} {
			if v, ok := bridgeResp[key]; ok {
				resp[key] = v
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}

func WechatLoginUrl(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wc := loadWechatConfigMap(cfg)
		c.JSON(http.StatusOK, gin.H{
			"ok":          true,
			"externalUrl": wc["bridgeUrl"],
			"internalUrl": "",
			"mode":        wc["mode"],
			"kitDir":      wc["kitDir"],
		})
	}
}

func WechatSend(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			To            string   `json:"to"`
			Content       string   `json:"content"`
			IsRoom        bool     `json:"isRoom"`
			MentionIdList []string `json:"mentionIdList"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if strings.TrimSpace(body.To) == "" || strings.TrimSpace(body.Content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "to and content are required"})
			return
		}
		resp, err := wechatBridgeRequest(cfg, http.MethodPost, "/send/text", gin.H{
			"to":            strings.TrimSpace(body.To),
			"content":       body.Content,
			"isRoom":        body.IsRoom,
			"mentionIdList": body.MentionIdList,
		})
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": resp})
	}
}

func WechatSendFile(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			To       string `json:"to"`
			FileURL  string `json:"fileUrl"`
			FileName string `json:"fileName"`
			IsRoom   bool   `json:"isRoom"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if strings.TrimSpace(body.To) == "" || strings.TrimSpace(body.FileURL) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "to and fileUrl are required"})
			return
		}
		resp, err := wechatBridgeRequest(cfg, http.MethodPost, "/send/file", gin.H{
			"to":       strings.TrimSpace(body.To),
			"fileUrl":  strings.TrimSpace(body.FileURL),
			"fileName": strings.TrimSpace(body.FileName),
			"isRoom":   body.IsRoom,
		})
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": resp})
	}
}

func WechatGetConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		wc := loadWechatConfigMap(cfg)
		c.JSON(http.StatusOK, gin.H{"ok": true, "config": wc})
	}
}

func WechatUpdateConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		adminCfg := loadAdminConfig(cfg)
		existing, ok := adminCfg["wechat"].(map[string]interface{})
		if !ok {
			existing = map[string]interface{}{}
		}
		for k, v := range body {
			existing[k] = v
		}
		adminCfg["wechat"] = normalizeWechatConfig(cfg, existing)
		saveAdminConfigData(cfg, adminCfg)
		c.JSON(http.StatusOK, gin.H{"ok": true, "config": adminCfg["wechat"]})
	}
}

// === ClawHub Sync ===

func ClawHubSync(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cachePath := filepath.Join(cfg.OpenClawDir, "clawhub-cache.json")
		// Try to read cache
		if data, err := os.ReadFile(cachePath); err == nil {
			var cached map[string]interface{}
			if json.Unmarshal(data, &cached) == nil {
				c.JSON(200, gin.H{"ok": true, "skills": cached["skills"], "source": "cache", "syncedAt": cached["syncedAt"]})
				return
			}
		}
		c.JSON(200, gin.H{"ok": true, "skills": []interface{}{}, "source": "empty"})
	}
}

// launchInUserSession launches an executable in the interactive user session via schtasks.
// Required because ClawPanel runs as SYSTEM (session 0) and cannot directly spawn GUI
// processes in the user's desktop session (session 1).
// getInteractiveUser returns the username of the currently logged-in interactive user.
// WMIC outputs UTF-16LE; we decode it properly to handle non-ASCII usernames.
func getInteractiveUser() string {
	out, err := exec.Command("wmic", "computersystem", "get", "UserName", "/VALUE").Output()
	if err == nil {
		text := decodeUTF16LEHandler(out)
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
	// Fallback: qwinsta.exe shows active console sessions
	out2, err := exec.Command(`C:\Windows\System32\qwinsta.exe`).Output()
	if err == nil {
		for _, line := range strings.Split(string(out2), "\n") {
			if strings.Contains(line, "Active") || strings.Contains(line, "活动") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					u := strings.TrimPrefix(fields[0], ">")
					if strings.HasPrefix(strings.ToLower(u), "console") || strings.HasPrefix(strings.ToLower(u), "rdp") {
						u = fields[1]
					}
					if u != "" && !strings.EqualFold(u, "services") {
						return u
					}
				}
			}
		}
	}
	return ""
}

// decodeUTF16LEHandler decodes UTF-16LE bytes (WMIC output) to UTF-8 string.
func decodeUTF16LEHandler(b []byte) string {
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

// launchInUserSession launches an executable in the interactive user session.
// Uses schtasks /RU <interactive_user> so the process runs in the user's desktop
// session (session 1) even when called from a SYSTEM service (session 0).
func launchInUserSession(exePath, workDir string) {
	username := getInteractiveUser()
	if username != "" {
		psContent := fmt.Sprintf(
			"Set-Location '%s'\nStart-Process -FilePath '%s' -WorkingDirectory '%s' -WindowStyle Hidden\n",
			workDir, exePath, workDir)
		psFile := filepath.Join(os.TempDir(), "napcat_launch_h.ps1")
		if err := os.WriteFile(psFile, []byte(psContent), 0644); err == nil {
			taskName := "ClawPanelStartNapCatH"
			tr := fmt.Sprintf(
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File \"%s\"",
				psFile)
			exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()
			err := exec.Command("schtasks", "/Create", "/F",
				"/TN", taskName,
				"/SC", "ONCE",
				"/ST", "00:00",
				"/RU", username,
				"/TR", tr,
				"/RL", "HIGHEST",
			).Run()
			if err == nil {
				if err = exec.Command("schtasks", "/Run", "/TN", taskName).Run(); err == nil {
					go func() {
						time.Sleep(15 * time.Second)
						exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()
						os.Remove(psFile)
					}()
					return
				}
			}
		}
	}
	// Fallback: direct exec
	cmd := exec.Command(exePath)
	cmd.Dir = workDir
	cmd.Start()
}

// helper to read openclaw.json
func readOpenClawJSON(cfg *config.Config) map[string]interface{} {
	data, err := os.ReadFile(filepath.Join(cfg.OpenClawDir, "openclaw.json"))
	if err != nil {
		return map[string]interface{}{}
	}
	var result map[string]interface{}
	if json.Unmarshal(data, &result) != nil {
		return map[string]interface{}{}
	}
	return result
}
