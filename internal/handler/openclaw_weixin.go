package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
)

const (
	openClawWeixinDefaultBaseURL    = "https://ilinkai.weixin.qq.com"
	openClawWeixinDefaultCDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"
	openClawWeixinDefaultBotType    = "3"
	openClawWeixinLoginTTL          = 5 * time.Minute
)

var (
	openClawWeixinValidIDRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	openClawWeixinInvalidIDRe  = regexp.MustCompile(`[^a-z0-9_-]+`)
	openClawWeixinLeadingDash  = regexp.MustCompile(`^-+`)
	openClawWeixinTrailingDash = regexp.MustCompile(`-+$`)

	openClawWeixinLoginsMu sync.Mutex
	openClawWeixinLogins   = map[string]*openClawWeixinLogin{}
)

type openClawWeixinLogin struct {
	SessionKey     string
	QRCode         string
	QRCodeURL      string
	StartedAt      time.Time
	CurrentBaseURL string
}

type openClawWeixinQRCodeResponse struct {
	QRCode    string `json:"qrcode"`
	QRCodeImg string `json:"qrcode_img_content"`
}

type openClawWeixinStatusResponse struct {
	Status       string `json:"status"`
	BotToken     string `json:"bot_token"`
	ILinkBotID   string `json:"ilink_bot_id"`
	BaseURL      string `json:"baseurl"`
	ILinkUserID  string `json:"ilink_user_id"`
	RedirectHost string `json:"redirect_host"`
}

type openClawWeixinAccountData struct {
	Token   string `json:"token,omitempty"`
	SavedAt string `json:"savedAt,omitempty"`
	BaseURL string `json:"baseUrl,omitempty"`
	UserID  string `json:"userId,omitempty"`
}

func openClawWeixinStateDir(cfg *config.Config) string {
	return filepath.Join(cfg.OpenClawDir, "openclaw-weixin")
}

func openClawWeixinAccountsDir(cfg *config.Config) string {
	return filepath.Join(openClawWeixinStateDir(cfg), "accounts")
}

func openClawWeixinAccountsIndexPath(cfg *config.Config) string {
	return filepath.Join(openClawWeixinStateDir(cfg), "accounts.json")
}

func openClawWeixinAllowFromPath(cfg *config.Config, accountID string) string {
	safeChannel := "openclaw-weixin"
	safeAccount := strings.ToLower(strings.TrimSpace(accountID))
	safeAccount = strings.ReplaceAll(safeAccount, "\\", "_")
	safeAccount = strings.ReplaceAll(safeAccount, "/", "_")
	safeAccount = strings.ReplaceAll(safeAccount, ":", "_")
	safeAccount = strings.ReplaceAll(safeAccount, "*", "_")
	safeAccount = strings.ReplaceAll(safeAccount, "?", "_")
	safeAccount = strings.ReplaceAll(safeAccount, "\"", "_")
	safeAccount = strings.ReplaceAll(safeAccount, "<", "_")
	safeAccount = strings.ReplaceAll(safeAccount, ">", "_")
	safeAccount = strings.ReplaceAll(safeAccount, "|", "_")
	safeAccount = strings.ReplaceAll(safeAccount, "..", "_")
	return filepath.Join(cfg.OpenClawDir, "credentials", fmt.Sprintf("%s-%s-allowFrom.json", safeChannel, safeAccount))
}

func normalizeOpenClawWeixinAccountID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "default"
	}
	if openClawWeixinValidIDRe.MatchString(trimmed) {
		return strings.ToLower(trimmed)
	}
	normalized := strings.ToLower(trimmed)
	normalized = openClawWeixinInvalidIDRe.ReplaceAllString(normalized, "-")
	normalized = openClawWeixinLeadingDash.ReplaceAllString(normalized, "")
	normalized = openClawWeixinTrailingDash.ReplaceAllString(normalized, "")
	if len(normalized) > 64 {
		normalized = normalized[:64]
	}
	if normalized == "" {
		return "default"
	}
	return normalized
}

func randomOpenClawWeixinSessionKey() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("wx-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

func purgeExpiredOpenClawWeixinLoginsLocked() {
	now := time.Now()
	for key, login := range openClawWeixinLogins {
		if login == nil || now.Sub(login.StartedAt) >= openClawWeixinLoginTTL {
			delete(openClawWeixinLogins, key)
		}
	}
}

func loadOpenClawWeixinAccountIndex(cfg *config.Config) []string {
	raw, err := os.ReadFile(openClawWeixinAccountsIndexPath(cfg))
	if err != nil {
		return nil
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil
	}
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		norm := normalizeOpenClawWeixinAccountID(id)
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	return out
}

func saveOpenClawWeixinAccountIndex(cfg *config.Config, ids []string) error {
	if err := os.MkdirAll(openClawWeixinStateDir(cfg), 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(openClawWeixinAccountsIndexPath(cfg), payload, 0644)
}

func registerOpenClawWeixinAccountID(cfg *config.Config, accountID string) error {
	ids := loadOpenClawWeixinAccountIndex(cfg)
	norm := normalizeOpenClawWeixinAccountID(accountID)
	for _, existing := range ids {
		if existing == norm {
			return nil
		}
	}
	ids = append(ids, norm)
	return saveOpenClawWeixinAccountIndex(cfg, ids)
}

func unregisterOpenClawWeixinAccountID(cfg *config.Config, accountID string) error {
	ids := loadOpenClawWeixinAccountIndex(cfg)
	norm := normalizeOpenClawWeixinAccountID(accountID)
	next := make([]string, 0, len(ids))
	for _, existing := range ids {
		if existing != norm {
			next = append(next, existing)
		}
	}
	return saveOpenClawWeixinAccountIndex(cfg, next)
}

func openClawWeixinAccountPath(cfg *config.Config, accountID string) string {
	return filepath.Join(openClawWeixinAccountsDir(cfg), normalizeOpenClawWeixinAccountID(accountID)+".json")
}

func loadOpenClawWeixinAccount(cfg *config.Config, accountID string) (*openClawWeixinAccountData, error) {
	raw, err := os.ReadFile(openClawWeixinAccountPath(cfg, accountID))
	if err != nil {
		return nil, err
	}
	var account openClawWeixinAccountData
	if err := json.Unmarshal(raw, &account); err != nil {
		return nil, err
	}
	return &account, nil
}

func saveOpenClawWeixinAccount(cfg *config.Config, accountID string, update openClawWeixinAccountData) error {
	if err := os.MkdirAll(openClawWeixinAccountsDir(cfg), 0755); err != nil {
		return err
	}
	existing, _ := loadOpenClawWeixinAccount(cfg, accountID)
	merged := openClawWeixinAccountData{}
	if existing != nil {
		merged = *existing
	}
	if strings.TrimSpace(update.Token) != "" {
		merged.Token = strings.TrimSpace(update.Token)
		merged.SavedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(update.BaseURL) != "" {
		merged.BaseURL = strings.TrimSpace(update.BaseURL)
	}
	if update.UserID != "" {
		merged.UserID = strings.TrimSpace(update.UserID)
	}
	payload, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	path := openClawWeixinAccountPath(cfg, accountID)
	if err := os.WriteFile(path, payload, 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func clearOpenClawWeixinAccount(cfg *config.Config, accountID string) {
	norm := normalizeOpenClawWeixinAccountID(accountID)
	files := []string{
		filepath.Join(openClawWeixinAccountsDir(cfg), norm+".json"),
		filepath.Join(openClawWeixinAccountsDir(cfg), norm+".sync.json"),
		filepath.Join(openClawWeixinAccountsDir(cfg), norm+".context-tokens.json"),
		openClawWeixinAllowFromPath(cfg, norm),
	}
	for _, path := range files {
		_ = os.Remove(path)
	}
}

func clearStaleOpenClawWeixinAccountsForUser(cfg *config.Config, currentAccountID, userID string) {
	if strings.TrimSpace(userID) == "" {
		return
	}
	for _, accountID := range loadOpenClawWeixinAccountIndex(cfg) {
		if accountID == currentAccountID {
			continue
		}
		account, err := loadOpenClawWeixinAccount(cfg, accountID)
		if err != nil || account == nil {
			continue
		}
		if strings.TrimSpace(account.UserID) != strings.TrimSpace(userID) {
			continue
		}
		clearOpenClawWeixinAccount(cfg, accountID)
		_ = unregisterOpenClawWeixinAccountID(cfg, accountID)
	}
}

func triggerOpenClawWeixinChannelReload(cfg *config.Config) error {
	ocConfig, err := cfg.ReadOpenClawJSON()
	if err != nil || ocConfig == nil {
		ocConfig = map[string]interface{}{}
	}
	channels, _ := ocConfig["channels"].(map[string]interface{})
	if channels == nil {
		channels = map[string]interface{}{}
	}
	section, _ := channels["openclaw-weixin"].(map[string]interface{})
	if section == nil {
		section = map[string]interface{}{}
	}
	section["channelConfigUpdatedAt"] = time.Now().UTC().Format(time.RFC3339)
	channels["openclaw-weixin"] = section
	ocConfig["channels"] = channels
	return cfg.WriteOpenClawJSON(ocConfig)
}

func getOpenClawWeixinSection(ocConfig map[string]interface{}) map[string]interface{} {
	if ocConfig == nil {
		return map[string]interface{}{}
	}
	channels, _ := ocConfig["channels"].(map[string]interface{})
	if channels == nil {
		return map[string]interface{}{}
	}
	section, _ := channels["openclaw-weixin"].(map[string]interface{})
	if section == nil {
		return map[string]interface{}{}
	}
	return section
}

func openClawWeixinConfiguredBaseURL(cfg *config.Config, accountID string) string {
	account, _ := loadOpenClawWeixinAccount(cfg, accountID)
	if account != nil && strings.TrimSpace(account.BaseURL) != "" {
		return strings.TrimSpace(account.BaseURL)
	}
	return openClawWeixinDefaultBaseURL
}

func openClawWeixinHTTPGet(baseURL, endpoint string, timeout time.Duration) ([]byte, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	req, err := http.NewRequest(http.MethodGet, baseURL+"/"+strings.TrimLeft(endpoint, "/"), nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}

func startOpenClawWeixinQRCode(force bool, sessionKey string, apiBaseURL string) (*openClawWeixinLogin, string, error) {
	openClawWeixinLoginsMu.Lock()
	defer openClawWeixinLoginsMu.Unlock()

	purgeExpiredOpenClawWeixinLoginsLocked()
	if sessionKey == "" {
		sessionKey = randomOpenClawWeixinSessionKey()
	}
	if !force {
		if existing := openClawWeixinLogins[sessionKey]; existing != nil && time.Since(existing.StartedAt) < openClawWeixinLoginTTL && existing.QRCodeURL != "" {
			return existing, sessionKey, nil
		}
	}

	body, err := openClawWeixinHTTPGet(openClawWeixinDefaultBaseURL, "ilink/bot/get_bot_qrcode?bot_type="+openClawWeixinDefaultBotType, 20*time.Second)
	if err != nil {
		return nil, sessionKey, err
	}
	var qr openClawWeixinQRCodeResponse
	if err := json.Unmarshal(body, &qr); err != nil {
		return nil, sessionKey, err
	}
	login := &openClawWeixinLogin{
		SessionKey:     sessionKey,
		QRCode:         strings.TrimSpace(qr.QRCode),
		QRCodeURL:      strings.TrimSpace(qr.QRCodeImg),
		StartedAt:      time.Now(),
		CurrentBaseURL: strings.TrimSpace(apiBaseURL),
	}
	if login.CurrentBaseURL == "" {
		login.CurrentBaseURL = openClawWeixinDefaultBaseURL
	}
	openClawWeixinLogins[sessionKey] = login
	return login, sessionKey, nil
}

func waitOpenClawWeixinQRCode(sessionKey string, timeoutMs int) (*openClawWeixinStatusResponse, *openClawWeixinLogin, error) {
	openClawWeixinLoginsMu.Lock()
	purgeExpiredOpenClawWeixinLoginsLocked()
	login := openClawWeixinLogins[sessionKey]
	openClawWeixinLoginsMu.Unlock()
	if login == nil {
		return nil, nil, fmt.Errorf("当前没有进行中的登录，请先生成二维码")
	}
	if timeoutMs <= 0 {
		timeoutMs = 35000
	}
	baseURL := strings.TrimSpace(login.CurrentBaseURL)
	if baseURL == "" {
		baseURL = openClawWeixinDefaultBaseURL
	}
	endpoint := "ilink/bot/get_qrcode_status?qrcode=" + login.QRCode
	body, err := openClawWeixinHTTPGet(baseURL, endpoint, time.Duration(timeoutMs)*time.Millisecond)
	if err != nil {
		if os.IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
			return &openClawWeixinStatusResponse{Status: "wait"}, login, nil
		}
		return &openClawWeixinStatusResponse{Status: "wait"}, login, nil
	}
	var status openClawWeixinStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, login, err
	}
	openClawWeixinLoginsMu.Lock()
	defer openClawWeixinLoginsMu.Unlock()
	current := openClawWeixinLogins[sessionKey]
	if current != nil {
		if status.Status == "scaned_but_redirect" && strings.TrimSpace(status.RedirectHost) != "" {
			current.CurrentBaseURL = "https://" + strings.TrimSpace(status.RedirectHost)
		}
		if status.Status == "expired" || status.Status == "confirmed" {
			delete(openClawWeixinLogins, sessionKey)
		}
	}
	return &status, login, nil
}

func restartOpenClawIfRunning(procMgr *process.Manager) {
	if procMgr == nil {
		return
	}
	if !(procMgr.GetStatus().Running || procMgr.GatewayListening()) {
		return
	}
	_ = procMgr.Restart()
}

func GetOpenClawWeixinStatus(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ocConfig, _ := cfg.ReadOpenClawJSON()
		section := getOpenClawWeixinSection(ocConfig)
		entries, _ := ocConfig["plugins"].(map[string]interface{})
		pluginEntries, _ := entries["entries"].(map[string]interface{})
		pluginInstalled := false
		if info, err := os.Stat(filepath.Join(cfg.OpenClawDir, "extensions", "openclaw-weixin")); err == nil && info.IsDir() {
			pluginInstalled = true
		}
		enabled := false
		if entry, ok := pluginEntries["openclaw-weixin"].(map[string]interface{}); ok {
			enabled, _ = entry["enabled"].(bool)
		}
		type accountSurface struct {
			AccountID  string `json:"accountId"`
			Name       string `json:"name,omitempty"`
			BaseURL    string `json:"baseUrl"`
			CDNBaseURL string `json:"cdnBaseUrl"`
			UserID     string `json:"userId,omitempty"`
			Enabled    bool   `json:"enabled"`
			Configured bool   `json:"configured"`
			SavedAt    string `json:"savedAt,omitempty"`
		}
		rawAccounts, _ := section["accounts"].(map[string]interface{})
		out := make([]accountSurface, 0)
		for _, accountID := range loadOpenClawWeixinAccountIndex(cfg) {
			accountData, _ := loadOpenClawWeixinAccount(cfg, accountID)
			cfgEntry, _ := rawAccounts[accountID].(map[string]interface{})
			accountEnabled := true
			if cfgEntry != nil {
				if v, ok := cfgEntry["enabled"].(bool); ok {
					accountEnabled = v
				}
			}
			baseURL := openClawWeixinDefaultBaseURL
			if accountData != nil && strings.TrimSpace(accountData.BaseURL) != "" {
				baseURL = strings.TrimSpace(accountData.BaseURL)
			}
			cdnBaseURL := openClawWeixinDefaultCDNBaseURL
			if cfgEntry != nil {
				if v, ok := cfgEntry["cdnBaseUrl"].(string); ok && strings.TrimSpace(v) != "" {
					cdnBaseURL = strings.TrimSpace(v)
				}
			}
			name := ""
			if cfgEntry != nil {
				if v, ok := cfgEntry["name"].(string); ok {
					name = strings.TrimSpace(v)
				}
			}
			out = append(out, accountSurface{
				AccountID:  accountID,
				Name:       name,
				BaseURL:    baseURL,
				CDNBaseURL: cdnBaseURL,
				UserID: strings.TrimSpace(func() string {
					if accountData == nil {
						return ""
					}
					return accountData.UserID
				}()),
				Enabled:    accountEnabled,
				Configured: accountData != nil && strings.TrimSpace(accountData.Token) != "",
				SavedAt: func() string {
					if accountData == nil {
						return ""
					}
					return strings.TrimSpace(accountData.SavedAt)
				}(),
			})
		}
		openClawWeixinLoginsMu.Lock()
		purgeExpiredOpenClawWeixinLoginsLocked()
		pending := len(openClawWeixinLogins)
		openClawWeixinLoginsMu.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"ok":              true,
			"pluginInstalled": pluginInstalled,
			"enabled":         enabled,
			"accounts":        out,
			"pendingLogins":   pending,
		})
	}
}

func StartOpenClawWeixinQRCode(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Force      bool   `json:"force"`
			SessionKey string `json:"sessionKey"`
			AccountID  string `json:"accountId"`
		}
		_ = c.ShouldBindJSON(&req)
		baseURL := openClawWeixinConfiguredBaseURL(cfg, req.AccountID)
		sessionKey := strings.TrimSpace(req.SessionKey)
		if sessionKey == "" && strings.TrimSpace(req.AccountID) != "" {
			sessionKey = normalizeOpenClawWeixinAccountID(req.AccountID)
		}
		login, resolvedSessionKey, err := startOpenClawWeixinQRCode(req.Force, sessionKey, baseURL)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":         true,
			"sessionKey": resolvedSessionKey,
			"qrcodeUrl":  login.QRCodeURL,
			"message":    "使用微信扫描以下二维码，以完成连接。",
		})
	}
}

func WaitOpenClawWeixinQRCode(cfg *config.Config, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SessionKey string `json:"sessionKey"`
			TimeoutMs  int    `json:"timeoutMs"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SessionKey) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "sessionKey required"})
			return
		}
		status, _, err := waitOpenClawWeixinQRCode(strings.TrimSpace(req.SessionKey), req.TimeoutMs)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if status.Status == "confirmed" && strings.TrimSpace(status.BotToken) != "" && strings.TrimSpace(status.ILinkBotID) != "" {
			accountID := normalizeOpenClawWeixinAccountID(status.ILinkBotID)
			if err := saveOpenClawWeixinAccount(cfg, accountID, openClawWeixinAccountData{
				Token:   status.BotToken,
				BaseURL: status.BaseURL,
				UserID:  status.ILinkUserID,
			}); err != nil {
				c.JSON(http.StatusOK, gin.H{"ok": false, "error": "保存微信账号失败: " + err.Error()})
				return
			}
			_ = registerOpenClawWeixinAccountID(cfg, accountID)
			clearStaleOpenClawWeixinAccountsForUser(cfg, accountID, status.ILinkUserID)
			_ = triggerOpenClawWeixinChannelReload(cfg)
			restartOpenClawIfRunning(procMgr)
			c.JSON(http.StatusOK, gin.H{
				"ok":        true,
				"connected": true,
				"status":    status.Status,
				"accountId": accountID,
				"message":   "与微信连接成功",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":        true,
			"connected": false,
			"status":    status.Status,
			"message":   map[string]string{"wait": "等待扫码或确认", "scaned": "已扫码，请在微信确认", "expired": "二维码已过期，请刷新"}[status.Status],
		})
	}
}

func LogoutOpenClawWeixin(cfg *config.Config, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			AccountID string `json:"accountId"`
		}
		_ = c.ShouldBindJSON(&req)
		accountID := normalizeOpenClawWeixinAccountID(req.AccountID)
		if strings.TrimSpace(req.AccountID) == "" {
			ids := loadOpenClawWeixinAccountIndex(cfg)
			if len(ids) == 1 {
				accountID = ids[0]
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "accountId required"})
				return
			}
		}
		clearOpenClawWeixinAccount(cfg, accountID)
		_ = unregisterOpenClawWeixinAccountID(cfg, accountID)
		_ = triggerOpenClawWeixinChannelReload(cfg)
		restartOpenClawIfRunning(procMgr)
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "微信账号已退出", "accountId": accountID})
	}
}
