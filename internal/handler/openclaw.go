package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/eventlog"
	"github.com/zhaoxinyi02/ClawPanel/internal/monitor"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
)

func normalizeProviderAPI(api string) string {
	switch api {
	case "anthropic":
		return "anthropic-messages"
	case "google-genai":
		return "google-generative-ai"
	default:
		return api
	}
}

func normalizeProviderAPIs(providers map[string]interface{}) {
	for _, prov := range providers {
		p, ok := prov.(map[string]interface{})
		if !ok {
			continue
		}
		if api, ok := p["api"].(string); ok {
			p["api"] = normalizeProviderAPI(api)
		}
		modelList, _ := p["models"].([]interface{})
		for _, m := range modelList {
			model, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			if api, ok := model["api"].(string); ok {
				model["api"] = normalizeProviderAPI(api)
			}
		}
	}
}

func preserveMissingMapFields(dst, src map[string]interface{}) {
	if dst == nil || src == nil {
		return
	}
	for key, srcVal := range src {
		dstVal, exists := dst[key]
		if !exists {
			dst[key] = srcVal
			continue
		}
		srcMap, srcIsMap := srcVal.(map[string]interface{})
		dstMap, dstIsMap := dstVal.(map[string]interface{})
		if srcIsMap && dstIsMap {
			preserveMissingMapFields(dstMap, srcMap)
		}
	}
}

func preserveHiddenOpenClawFields(dst, src map[string]interface{}) {
	if dst == nil || src == nil {
		return
	}
	if srcTools, ok := src["tools"].(map[string]interface{}); ok {
		dstTools, _ := dst["tools"].(map[string]interface{})
		if dstTools == nil {
			dstTools = map[string]interface{}{}
		}
		preserveMissingMapFields(dstTools, srcTools)
		if len(dstTools) > 0 {
			dst["tools"] = dstTools
		}
	} else if tools, ok := src["tools"]; ok {
		if _, exists := dst["tools"]; !exists {
			dst["tools"] = tools
		}
	}
	if srcSession, ok := src["session"].(map[string]interface{}); ok {
		dstSession, _ := dst["session"].(map[string]interface{})
		if dstSession == nil {
			dstSession = map[string]interface{}{}
		}
		preserveMissingMapFields(dstSession, srcSession)
		if len(dstSession) > 0 {
			dst["session"] = dstSession
		}
	} else if session, ok := src["session"]; ok {
		if _, exists := dst["session"]; !exists {
			dst["session"] = session
		}
	}
	if srcCron, ok := src["cron"].(map[string]interface{}); ok {
		dstCron, _ := dst["cron"].(map[string]interface{})
		if jobs, ok := srcCron["jobs"]; ok {
			if dstCron == nil {
				dstCron = map[string]interface{}{}
			}
			if _, exists := dstCron["jobs"]; !exists {
				dstCron["jobs"] = jobs
			}
			dst["cron"] = dstCron
		}
	}
}

// GetOpenClawConfig 获取 OpenClaw 配置
func GetOpenClawConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ocConfig, err := cfg.ReadOpenClawJSON()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "config": gin.H{}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "config": ocConfig})
	}
}

// SaveOpenClawConfig 保存 OpenClaw 配置
func SaveOpenClawConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Config map[string]interface{} `json:"config"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		ocCfg := req.Config
		if ocCfg == nil {
			ocCfg = map[string]interface{}{}
		}
		existingCfg, _ := cfg.ReadOpenClawJSON()

		// 自动为非 OpenAI 提供商注入 compat.supportsDeveloperRole=false
		injectCompatFlags(ocCfg)
		normalizeOpenClawModelAPIs(ocCfg)
		syncAllowedModels(ocCfg)
		preserveHiddenOpenClawFields(ocCfg, existingCfg)
		if err := validateOpenClawNumericConfig(ocCfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		if err := cfg.WriteOpenClawJSON(ocCfg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		// 同时修补运行时 models.json
		patchModelsJSON(cfg)

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetModels 获取模型配置
func GetModels(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ocConfig, err := cfg.ReadOpenClawJSON()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "providers": gin.H{}, "defaults": gin.H{}})
			return
		}
		models, _ := ocConfig["models"].(map[string]interface{})
		if models == nil {
			models = map[string]interface{}{}
		}
		agents, _ := ocConfig["agents"].(map[string]interface{})
		defaults := map[string]interface{}{}
		if agents != nil {
			defaults, _ = agents["defaults"].(map[string]interface{})
			if defaults == nil {
				defaults = map[string]interface{}{}
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":        true,
			"providers": models["providers"],
			"defaults":  defaults,
		})
	}
}

// SaveModels 保存模型配置
func SaveModels(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}

		if providers, ok := body["providers"]; ok {
			if provMap, ok := providers.(map[string]interface{}); ok {
				normalizeProviderAPIs(provMap)
			}
			if models, ok := ocConfig["models"].(map[string]interface{}); ok {
				models["providers"] = providers
			} else {
				ocConfig["models"] = map[string]interface{}{"providers": providers}
			}
		}
		syncAllowedModels(ocConfig)

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		patchModelsJSON(cfg)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetChannels 获取通道配置
func GetChannels(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ocConfig, err := cfg.ReadOpenClawJSON()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "channels": gin.H{}, "plugins": gin.H{}})
			return
		}
		channels, _ := ocConfig["channels"].(map[string]interface{})
		plugins, _ := ocConfig["plugins"].(map[string]interface{})
		if channels == nil {
			channels = map[string]interface{}{}
		}
		if plugins == nil {
			plugins = map[string]interface{}{}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "channels": channels, "plugins": plugins})
	}
}

func qqPluginInstalled(cfg *config.Config) bool {
	for _, candidate := range []string{
		filepath.Join(cfg.OpenClawDir, "extensions", "qq"),
		filepath.Join(filepath.Dir(cfg.OpenClawDir), "extensions", "qq"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// SaveChannel 保存通道配置
func SaveChannel(cfg *config.Config, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "qq" && !qqPluginInstalled(cfg) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "QQ 个人号插件未安装，请先安装 QQ 个人号插件后再配置通道"})
			return
		}
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}

		channels, _ := ocConfig["channels"].(map[string]interface{})
		if channels == nil {
			channels = map[string]interface{}{}
		}
		if id == "qq" {
			body = normalizeQQChannelConfig(body)
		}
		if id == "feishu" {
			body = normalizeFeishuChannelConfig(body)
		}
		channels[id] = body
		ocConfig["channels"] = channels

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		resp := gin.H{"ok": true}
		if id == "qq" && procMgr != nil {
			status := procMgr.GetStatus()
			if status.Running {
				if err := procMgr.Restart(); err != nil {
					resp["message"] = "QQ 配置已保存，但自动重启网关失败，请手动重启 OpenClaw 网关后生效"
					resp["restartWarning"] = err.Error()
				} else {
					resp["message"] = "QQ 配置已保存，并已自动重启网关使配置生效"
					resp["restarted"] = true
				}
			} else {
				resp["message"] = "QQ 配置已保存；OpenClaw 网关下次启动时生效"
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}

func normalizeQQChannelConfig(body map[string]interface{}) map[string]interface{} {
	if body == nil {
		return map[string]interface{}{}
	}

	rateLimit := ensureMap(body, "rateLimit")
	wakeTrigger := ensureMap(rateLimit, "wakeTrigger")
	autoApprove := ensureMap(body, "autoApprove")
	friendApprove := ensureMap(autoApprove, "friend")
	groupApprove := ensureMap(autoApprove, "group")

	if v, ok := toFloat64(body["wakeProbability"]); ok {
		rateLimit["wakeProbability"] = v
		delete(body, "wakeProbability")
	}

	if v, ok := toFloat64(body["minSendIntervalMs"]); ok {
		rateLimit["minIntervalSec"] = v / 1000
		delete(body, "minSendIntervalMs")
	}

	if raw, exists := body["wakeTrigger"]; exists {
		switch t := raw.(type) {
		case string:
			parts := strings.Split(t, ",")
			keywords := make([]interface{}, 0, len(parts))
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					keywords = append(keywords, trimmed)
				}
			}
			wakeTrigger["keywords"] = keywords
		case []interface{}:
			wakeTrigger["keywords"] = t
		}
		delete(body, "wakeTrigger")
	}

	if enabled, ok := body["autoApproveFriend"].(bool); ok {
		friendApprove["enabled"] = enabled
		delete(body, "autoApproveFriend")
	}

	if enabled, ok := body["autoApproveGroup"].(bool); ok {
		groupApprove["enabled"] = enabled
		delete(body, "autoApproveGroup")
	}

	return body
}

func normalizeFeishuChannelConfig(body map[string]interface{}) map[string]interface{} {
	if body == nil {
		return map[string]interface{}{}
	}
	delete(body, "dmScope")

	if raw, exists := body["requireMention"]; exists {
		switch v := raw.(type) {
		case string:
			switch trimmed := strings.TrimSpace(v); trimmed {
			case "":
				delete(body, "requireMention")
			case "true":
				body["requireMention"] = true
			case "false":
				body["requireMention"] = false
			default:
				body["requireMention"] = trimmed
			}
		}
	}

	if raw, exists := body["groupAllowFrom"]; exists {
		switch v := raw.(type) {
		case string:
			parts := strings.FieldsFunc(v, func(r rune) bool {
				return r == ',' || r == '，'
			})
			items := make([]interface{}, 0, len(parts))
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					items = append(items, trimmed)
				}
			}
			if len(items) > 0 {
				body["groupAllowFrom"] = items
			} else {
				delete(body, "groupAllowFrom")
			}
		case []interface{}:
			items := make([]interface{}, 0, len(v))
			for _, item := range v {
				trimmed := strings.TrimSpace(toString(item))
				if trimmed != "" {
					items = append(items, trimmed)
				}
			}
			if len(items) > 0 {
				body["groupAllowFrom"] = items
			} else {
				delete(body, "groupAllowFrom")
			}
		}
	}

	groupPolicy := strings.TrimSpace(toString(body["groupPolicy"]))
	if groupPolicy != "" {
		body["groupPolicy"] = groupPolicy
	} else {
		delete(body, "groupPolicy")
	}
	if groupPolicy != "allowlist" {
		delete(body, "groupAllowFrom")
	}

	dmPolicy := strings.TrimSpace(toString(body["dmPolicy"]))
	if dmPolicy != "" {
		body["dmPolicy"] = dmPolicy
	} else {
		delete(body, "dmPolicy")
	}

	topAppID := strings.TrimSpace(toString(body["appId"]))
	if topAppID != "" {
		body["appId"] = topAppID
	} else {
		delete(body, "appId")
	}
	topAppSecret := strings.TrimSpace(toString(body["appSecret"]))
	if topAppSecret != "" {
		body["appSecret"] = topAppSecret
	} else {
		delete(body, "appSecret")
	}

	defaultAccount := strings.TrimSpace(toString(body["defaultAccount"]))
	rawAccounts, _ := body["accounts"].(map[string]interface{})
	normalizedAccounts := map[string]interface{}{}
	accountIDs := make([]string, 0, len(rawAccounts))
	for rawID, rawEntry := range rawAccounts {
		accountID := strings.TrimSpace(rawID)
		if accountID == "" {
			continue
		}
		entry, _ := rawEntry.(map[string]interface{})
		if entry == nil {
			entry = map[string]interface{}{}
		}
		nextEntry := normalizeFeishuAccountEntry(entry)
		if len(nextEntry) > 0 || accountID == defaultAccount {
			normalizedAccounts[accountID] = nextEntry
			accountIDs = append(accountIDs, accountID)
		}
	}
	sort.Strings(accountIDs)
	if defaultAccount != "" && !containsString(accountIDs, defaultAccount) && topAppID == "" && topAppSecret == "" {
		defaultAccount = ""
	}
	runnableAccountIDs := filterFeishuRunnableAccountIDs(normalizedAccounts, accountIDs)
	if defaultAccount == "" {
		if matched := findFeishuMirroredDefaultAccount(normalizedAccounts, accountIDs, topAppID, topAppSecret); matched != "" {
			defaultAccount = matched
		} else if matched := findFeishuEnabledDefaultAccount(normalizedAccounts, runnableAccountIDs); matched != "" {
			defaultAccount = matched
		} else if containsString(runnableAccountIDs, "default") {
			defaultAccount = "default"
		} else if len(accountIDs) == 0 && (topAppID != "" || topAppSecret != "") {
			defaultAccount = "default"
		} else if len(runnableAccountIDs) > 0 {
			defaultAccount = runnableAccountIDs[0]
		} else if len(accountIDs) > 0 {
			defaultAccount = accountIDs[0]
		}
	}
	if defaultAccount != "" && len(runnableAccountIDs) > 0 {
		entry, _ := normalizedAccounts[defaultAccount].(map[string]interface{})
		if !hasFeishuRunnableCredentials(entry) {
			if matched := findFeishuMirroredDefaultAccount(normalizedAccounts, runnableAccountIDs, topAppID, topAppSecret); matched != "" {
				defaultAccount = matched
			} else if matched := findFeishuEnabledDefaultAccount(normalizedAccounts, runnableAccountIDs); matched != "" {
				defaultAccount = matched
			} else if containsString(runnableAccountIDs, "default") {
				defaultAccount = "default"
			} else {
				defaultAccount = runnableAccountIDs[0]
			}
		}
	}
	if defaultAccount != "" {
		entry, _ := normalizedAccounts[defaultAccount].(map[string]interface{})
		if entry == nil {
			entry = map[string]interface{}{}
		}
		if _, ok := entry["appId"]; !ok && topAppID != "" {
			entry["appId"] = topAppID
		}
		if _, ok := entry["appSecret"]; !ok && topAppSecret != "" {
			entry["appSecret"] = topAppSecret
		}
		entry["enabled"] = true
		normalizedAccounts[defaultAccount] = entry
		if appID := strings.TrimSpace(toString(entry["appId"])); appID != "" {
			body["appId"] = appID
		} else {
			delete(body, "appId")
		}
		if appSecret := strings.TrimSpace(toString(entry["appSecret"])); appSecret != "" {
			body["appSecret"] = appSecret
		} else {
			delete(body, "appSecret")
		}
		body["defaultAccount"] = defaultAccount
	} else {
		delete(body, "defaultAccount")
	}
	if len(normalizedAccounts) > 0 {
		body["accounts"] = normalizedAccounts
	} else {
		delete(body, "accounts")
	}

	return body
}

func findFeishuMirroredDefaultAccount(accounts map[string]interface{}, accountIDs []string, topAppID, topAppSecret string) string {
	if topAppID == "" || topAppSecret == "" {
		return ""
	}
	matchedAccount := ""
	for _, accountID := range accountIDs {
		entry, _ := accounts[accountID].(map[string]interface{})
		if entry == nil {
			continue
		}
		if strings.TrimSpace(toString(entry["appId"])) != topAppID {
			continue
		}
		if strings.TrimSpace(toString(entry["appSecret"])) != topAppSecret {
			continue
		}
		if matchedAccount != "" {
			return ""
		}
		matchedAccount = accountID
	}
	return matchedAccount
}

func findFeishuEnabledDefaultAccount(accounts map[string]interface{}, accountIDs []string) string {
	matchedAccount := ""
	for _, accountID := range accountIDs {
		entry, _ := accounts[accountID].(map[string]interface{})
		if entry == nil || !isFeishuAccountEnabledEntry(entry) {
			continue
		}
		if matchedAccount != "" {
			return ""
		}
		matchedAccount = accountID
	}
	return matchedAccount
}

func isFeishuAccountEnabledEntry(entry map[string]interface{}) bool {
	if enabled, ok := normalizeOptionalBool(entry["enabled"]); ok {
		return enabled
	}
	return hasFeishuRunnableCredentials(entry)
}

func hasFeishuRunnableCredentials(entry map[string]interface{}) bool {
	return strings.TrimSpace(toString(entry["appId"])) != "" && strings.TrimSpace(toString(entry["appSecret"])) != ""
}

func filterFeishuRunnableAccountIDs(accounts map[string]interface{}, accountIDs []string) []string {
	ids := make([]string, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		entry, _ := accounts[accountID].(map[string]interface{})
		if entry == nil || !hasFeishuRunnableCredentials(entry) {
			continue
		}
		ids = append(ids, accountID)
	}
	return ids
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func normalizeFeishuAccountEntry(entry map[string]interface{}) map[string]interface{} {
	nextEntry := map[string]interface{}{}
	for rawKey, rawValue := range entry {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		switch key {
		case "appId", "appSecret", "botName":
			if trimmed := strings.TrimSpace(toString(rawValue)); trimmed != "" {
				nextEntry[key] = trimmed
			}
		case "enabled":
			if enabled, ok := normalizeOptionalBool(rawValue); ok {
				nextEntry[key] = enabled
			}
		default:
			nextEntry[key] = rawValue
		}
	}
	if _, ok := nextEntry["enabled"]; !ok && hasFeishuRunnableCredentials(nextEntry) {
		nextEntry["enabled"] = true
	}
	return nextEntry
}

func normalizeOptionalBool(raw interface{}) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case float64:
		if v == 1 {
			return true, true
		}
		if v == 0 {
			return false, true
		}
	case int:
		if v == 1 {
			return true, true
		}
		if v == 0 {
			return false, true
		}
	case int64:
		if v == 1 {
			return true, true
		}
		if v == 0 {
			return false, true
		}
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			if parsed == 1 {
				return true, true
			}
			if parsed == 0 {
				return false, true
			}
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return false, false
		}
		parsed, err := strconv.ParseBool(trimmed)
		if err != nil {
			return false, false
		}
		return parsed, true
	default:
		return false, false
	}
	return false, false
}

func ensureMap(parent map[string]interface{}, key string) map[string]interface{} {
	if child, ok := parent[key].(map[string]interface{}); ok && child != nil {
		return child
	}
	child := map[string]interface{}{}
	parent[key] = child
	return child
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case string:
		if n == "" {
			return 0, false
		}
		var parsed float64
		if _, err := fmt.Sscanf(n, "%f", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

// SavePlugin 保存插件配置
func SavePlugin(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}

		plugins, _ := ocConfig["plugins"].(map[string]interface{})
		if plugins == nil {
			plugins = map[string]interface{}{}
		}
		entries, _ := plugins["entries"].(map[string]interface{})
		if entries == nil {
			entries = map[string]interface{}{}
		}
		entries[id] = body
		plugins["entries"] = entries
		ocConfig["plugins"] = plugins

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ToggleChannel 切换通道启用/禁用
// napcatMon 可选：传入后，关闭 QQ 通道时自动停止 NapCat 并暂停监控，开启时恢复监控
func ToggleChannel(cfg *config.Config, procMgr *process.Manager, napcatMon *monitor.NapCatMonitor, sysLog ...*eventlog.SystemLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ChannelID string `json:"channelId"`
			Enabled   bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.ChannelID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "channelId required"})
			return
		}
		if req.ChannelID == "qq" && !qqPluginInstalled(cfg) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "QQ 个人号插件未安装，请先安装 QQ 个人号插件"})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}

		// 更新 channels
		channels, _ := ocConfig["channels"].(map[string]interface{})
		if channels == nil {
			channels = map[string]interface{}{}
		}
		ch, _ := channels[req.ChannelID].(map[string]interface{})
		if ch == nil {
			ch = map[string]interface{}{}
		}
		ch["enabled"] = req.Enabled
		channels[req.ChannelID] = ch
		ocConfig["channels"] = channels

		// 更新 plugins.entries
		plugins, _ := ocConfig["plugins"].(map[string]interface{})
		if plugins == nil {
			plugins = map[string]interface{}{}
		}
		entries, _ := plugins["entries"].(map[string]interface{})
		if entries == nil {
			entries = map[string]interface{}{}
		}

		// 飞书特殊处理：确定当前活跃的 plugin entry ID，启用/禁用正确的条目
		if req.ChannelID == "feishu" {
			activeEntryID := resolveActiveFeishuEntryID(entries)
			pe, _ := entries[activeEntryID].(map[string]interface{})
			if pe == nil {
				pe = map[string]interface{}{}
			}
			pe["enabled"] = req.Enabled
			entries[activeEntryID] = pe
			// 禁用另一个变体（如果存在）
			otherID := "feishu"
			if activeEntryID == "feishu" {
				otherID = "feishu-openclaw-plugin"
			}
			if otherEntry, ok := entries[otherID].(map[string]interface{}); ok {
				otherEntry["enabled"] = false
				entries[otherID] = otherEntry
			}
		} else {
			pe, _ := entries[req.ChannelID].(map[string]interface{})
			if pe == nil {
				pe = map[string]interface{}{}
			}
			pe["enabled"] = req.Enabled
			entries[req.ChannelID] = pe
		}
		plugins["entries"] = entries
		ocConfig["plugins"] = plugins

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		// 如果是 QQ 通道，关闭时停止 NapCat 并暂停监控；开启时恢复监控
		if req.ChannelID == "qq" && napcatMon != nil {
			if !req.Enabled {
				napcatMon.Pause()
				go monitor.StopNapCatPlatform()
			} else {
				napcatMon.Resume()
			}
		}

		// 发送网关重启信号
		writeRestartSignal(cfg, req.ChannelID+" toggled")

		channelNames := map[string]string{
			"qq": "QQ (NapCat)", "wechat": "微信", "feishu": "飞书",
			"qqbot": "QQ Bot", "dingtalk": "钉钉", "wecom": "企业微信",
		}
		label := channelNames[req.ChannelID]
		if label == "" {
			label = req.ChannelID
		}
		action := "已启用"
		eventType := "channel.enabled"
		if !req.Enabled {
			action = "已禁用"
			eventType = "channel.disabled"
		}

		if len(sysLog) > 0 && sysLog[0] != nil {
			sysLog[0].Log("system", eventType, label+" 通道"+action)
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "message": label + " 通道" + action})
	}
}

// injectCompatFlags 为非 OpenAI 提供商注入兼容性标志
func injectCompatFlags(ocCfg map[string]interface{}) {
	models, _ := ocCfg["models"].(map[string]interface{})
	if models == nil {
		return
	}
	providers, _ := models["providers"].(map[string]interface{})
	if providers == nil {
		return
	}

	for _, prov := range providers {
		p, ok := prov.(map[string]interface{})
		if !ok {
			continue
		}
		baseURL, _ := p["baseUrl"].(string)
		if isNativeOpenAI(baseURL) {
			continue
		}
		modelList, _ := p["models"].([]interface{})
		for _, m := range modelList {
			model, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			compat, _ := model["compat"].(map[string]interface{})
			if compat == nil {
				compat = map[string]interface{}{}
			}
			compat["supportsDeveloperRole"] = false
			model["compat"] = compat
		}
	}
}

func normalizeOpenClawModelAPIs(ocCfg map[string]interface{}) {
	models, _ := ocCfg["models"].(map[string]interface{})
	if models == nil {
		return
	}
	providers, _ := models["providers"].(map[string]interface{})
	if providers == nil {
		return
	}
	normalizeProviderAPIs(providers)
}

func syncAllowedModels(ocCfg map[string]interface{}) {
	agents, _ := ocCfg["agents"].(map[string]interface{})
	if agents == nil {
		return
	}
	defaults, _ := agents["defaults"].(map[string]interface{})
	if defaults == nil {
		return
	}
	current, hasCurrent := defaults["models"]
	if !hasCurrent {
		return
	}
	currentMap, ok := current.(map[string]interface{})
	if !ok {
		return
	}

	models, _ := ocCfg["models"].(map[string]interface{})
	providers, _ := models["providers"].(map[string]interface{})
	if providers == nil {
		defaults["models"] = map[string]interface{}{}
		return
	}

	next := map[string]interface{}{}
	for pid, prov := range providers {
		p, ok := prov.(map[string]interface{})
		if !ok {
			continue
		}
		modelList, _ := p["models"].([]interface{})
		for _, rawModel := range modelList {
			var modelID string
			switch model := rawModel.(type) {
			case string:
				modelID = model
			case map[string]interface{}:
				modelID, _ = model["id"].(string)
			}
			if pid == "" || modelID == "" {
				continue
			}
			key := pid + "/" + modelID
			if existing, ok := currentMap[key]; ok {
				next[key] = existing
			} else {
				next[key] = map[string]interface{}{}
			}
		}
	}

	defaults["models"] = next
}

// patchModelsJSON 修补运行时 models.json
func patchModelsJSON(cfg *config.Config) {
	agentIDs, _ := loadAgentIDs(cfg)
	if isLegacySingleAgentMode() {
		agentIDs = []string{"main"}
	}
	for _, agentID := range agentIDs {
		patchModelsJSONForAgent(cfg, agentID)
	}
}

func patchModelsJSONForAgent(cfg *config.Config, agentID string) {
	modelsPath := resolveAgentPath(cfg, agentID, "agent", "models.json")
	data, err := os.ReadFile(modelsPath)
	if err != nil {
		return
	}

	var modelsData map[string]interface{}
	if err := json.Unmarshal(data, &modelsData); err != nil {
		return
	}

	providers, _ := modelsData["providers"].(map[string]interface{})
	if providers == nil {
		return
	}

	changed := false
	for _, prov := range providers {
		p, ok := prov.(map[string]interface{})
		if !ok {
			continue
		}
		if api, ok := p["api"].(string); ok {
			normalized := normalizeProviderAPI(api)
			if normalized != api {
				p["api"] = normalized
				changed = true
			}
		}
		modelList, _ := p["models"].([]interface{})
		for _, m := range modelList {
			model, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			if api, ok := model["api"].(string); ok {
				normalized := normalizeProviderAPI(api)
				if normalized != api {
					model["api"] = normalized
					changed = true
				}
			}
		}
	}
	for _, prov := range providers {
		p, ok := prov.(map[string]interface{})
		if !ok {
			continue
		}
		baseURL, _ := p["baseUrl"].(string)
		if isNativeOpenAI(baseURL) {
			continue
		}
		modelList, _ := p["models"].([]interface{})
		for _, m := range modelList {
			model, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			compat, _ := model["compat"].(map[string]interface{})
			if compat == nil {
				compat = map[string]interface{}{}
			}
			if v, ok := compat["supportsDeveloperRole"].(bool); !ok || v {
				compat["supportsDeveloperRole"] = false
				model["compat"] = compat
				changed = true
			}
		}
	}

	if changed {
		newData, err := json.MarshalIndent(modelsData, "", "  ")
		if err == nil {
			_ = os.WriteFile(modelsPath, newData, 0644)
		}
	}
}

func isNativeOpenAI(baseURL string) bool {
	return strings.Contains(strings.ToLower(baseURL), "api.openai.com")
}

// writeRestartSignal 写入网关重启信号文件
func writeRestartSignal(cfg *config.Config, reason string) {
	signalPath := filepath.Join(cfg.OpenClawDir, "restart-gateway-signal.json")
	data, _ := json.Marshal(map[string]interface{}{
		"requestedAt": "now",
		"reason":      reason,
	})
	os.WriteFile(signalPath, data, 0644)
}

// resolveActiveFeishuEntryID 返回当前启用的飞书插件 entry ID
func resolveActiveFeishuEntryID(entries map[string]interface{}) string {
	for _, id := range []string{"feishu-openclaw-plugin", "feishu"} {
		if entry, ok := entries[id].(map[string]interface{}); ok {
			if enabled, _ := entry["enabled"].(bool); enabled {
				return id
			}
		}
	}
	// 没有 enabled 的，返回有 entry 的第一个
	for _, id := range []string{"feishu-openclaw-plugin", "feishu"} {
		if entries[id] != nil {
			return id
		}
	}
	return "feishu"
}

// SwitchFeishuVariant 切换飞书插件版本（官方版 / ClawTeam 版）
func SwitchFeishuVariant(cfg *config.Config, procMgr *process.Manager, sysLog ...*eventlog.SystemLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Variant string `json:"variant"` // "official" 或 "clawteam"
		}
		if err := c.ShouldBindJSON(&req); err != nil || (req.Variant != "official" && req.Variant != "clawteam") {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "variant must be 'official' or 'clawteam'"})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}

		plugins, _ := ocConfig["plugins"].(map[string]interface{})
		if plugins == nil {
			plugins = map[string]interface{}{}
		}
		entries, _ := plugins["entries"].(map[string]interface{})
		if entries == nil {
			entries = map[string]interface{}{}
		}

		// 互斥设置 enabled
		enableID := "feishu"
		disableID := "feishu-openclaw-plugin"
		label := "ClawTeam 社区版"
		if req.Variant == "official" {
			enableID = "feishu-openclaw-plugin"
			disableID = "feishu"
			label = "飞书官方版"
		}

		enableEntry, _ := entries[enableID].(map[string]interface{})
		if enableEntry == nil {
			enableEntry = map[string]interface{}{}
		}
		enableEntry["enabled"] = true
		entries[enableID] = enableEntry

		disableEntry, _ := entries[disableID].(map[string]interface{})
		if disableEntry == nil {
			disableEntry = map[string]interface{}{}
		}
		disableEntry["enabled"] = false
		entries[disableID] = disableEntry

		plugins["entries"] = entries
		ocConfig["plugins"] = plugins

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		writeRestartSignal(cfg, "feishu variant switched to "+req.Variant)

		if len(sysLog) > 0 && sysLog[0] != nil {
			sysLog[0].Log("system", "channel.variant_switched", "飞书通道切换为"+label)
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "飞书通道已切换为" + label})
	}
}
