package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/eventlog"
	"github.com/zhaoxinyi02/ClawPanel/internal/monitor"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
)

const (
	canonicalFeishuOfficialPluginID  = "openclaw-lark"
	canonicalFeishuCommunityPluginID = "feishu"
)

// 飞书官方版当前仅认可新 ID，旧 ID 仅用于清理历史脏配置。
var feishuOfficialPluginIDs = []string{canonicalFeishuOfficialPluginID}

// 当前有效的飞书插件 ID（官方版 + 社区版）。
var feishuAllPluginIDs = []string{canonicalFeishuOfficialPluginID, canonicalFeishuCommunityPluginID}

// 企业微信机器人插件 ID（优先级从高到低：新 ID 优先）
var wecomBotPluginIDs = []string{"wecom-openclaw-plugin", "wecom"}

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
		if apiKey, ok := p["apiKey"].(string); ok {
			p["apiKey"] = sanitizeProviderAPIKey(apiKey)
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

var knownAPIKeyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`nvapi-[A-Za-z0-9._\-=]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9._\-=]+`),
	regexp.MustCompile(`sess-[A-Za-z0-9._\-=]+`),
	regexp.MustCompile(`AIza[0-9A-Za-z_\-=]+`),
}

func sanitizeProviderAPIKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	for _, pattern := range knownAPIKeyPatterns {
		if match := pattern.FindString(raw); match != "" {
			return match
		}
	}
	return raw
}

func stripTransientProviderFields(providers map[string]interface{}) {
	for _, prov := range providers {
		providerMap, ok := prov.(map[string]interface{})
		if !ok {
			continue
		}
		delete(providerMap, "_note")
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
		// OpenClaw forbids both "allow" and "alsoAllow" in the same scope.
		// If merge introduced a conflict, merge alsoAllow items into allow,
		// then drop alsoAllow to resolve the conflict without losing tools.
		if allowVal, hasAllow := dstTools["allow"]; hasAllow {
			if alsoAllowVal, hasAlsoAllow := dstTools["alsoAllow"]; hasAlsoAllow {
				if allowList, ok := allowVal.([]interface{}); ok {
					if alsoList, ok2 := alsoAllowVal.([]interface{}); ok2 {
						existing := make(map[string]bool, len(allowList))
						for _, v := range allowList {
							if s, ok := v.(string); ok {
								existing[s] = true
							}
						}
						for _, v := range alsoList {
							if s, ok := v.(string); ok {
								if !existing[s] {
									allowList = append(allowList, v)
								}
							}
						}
						dstTools["allow"] = allowList
					}
				}
				delete(dstTools, "alsoAllow")
			}
		}
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

func wecomAppActiveDir(cfg *config.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.OpenClawDir) == "" {
		return ""
	}
	return filepath.Join(cfg.OpenClawDir, "extensions", "wecom-app")
}

func isWecomAppEnabled(cfg *config.Config) bool {
	activeDir := wecomAppActiveDir(cfg)
	if activeDir == "" {
		return false
	}
	info, err := os.Stat(activeDir)
	return err == nil && info.IsDir()
}

func injectWecomVirtualChannel(cfg *config.Config, ocConfig map[string]interface{}) {
	if ocConfig == nil {
		return
	}
	channels, _ := ocConfig["channels"].(map[string]interface{})
	if channels == nil {
		return
	}
	wecom, _ := channels["wecom"].(map[string]interface{})
	if wecom == nil {
		delete(channels, "wecom-app")
		return
	}
	delete(channels, "wecom-app")
	agent, _ := wecom["agent"].(map[string]interface{})
	if len(agent) == 0 && !isWecomAppEnabled(cfg) {
		return
	}
	virtual := map[string]interface{}{}
	for k, v := range agent {
		virtual[k] = v
	}
	virtual["enabled"] = isWecomAppEnabled(cfg)
	channels["wecom-app"] = virtual
}

func normalizeOpenClawCompatConfig(ocConfig map[string]interface{}) {
	if ocConfig == nil {
		return
	}

	legacyModel, _ := ocConfig["model"].(map[string]interface{})
	agents, _ := ocConfig["agents"].(map[string]interface{})
	defaults := map[string]interface{}(nil)
	if agents != nil {
		defaults, _ = agents["defaults"].(map[string]interface{})
	}

	currentModel := map[string]interface{}(nil)
	if defaults != nil {
		currentModel, _ = defaults["model"].(map[string]interface{})
	}
	if currentModel == nil && legacyModel == nil {
		return
	}

	if agents == nil {
		agents = map[string]interface{}{}
		ocConfig["agents"] = agents
	}
	if defaults == nil {
		defaults = map[string]interface{}{}
		agents["defaults"] = defaults
	}
	if currentModel == nil && legacyModel != nil {
		currentModel = deepCloneMap(legacyModel)
		defaults["model"] = currentModel
	}
	if currentModel != nil {
		ocConfig["model"] = deepCloneMap(currentModel)
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
		normalizeOpenClawCompatConfig(ocConfig)
		injectWecomVirtualChannel(cfg, ocConfig)
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
		normalizeOpenClawCompatConfig(ocCfg)
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
		normalizeOpenClawCompatConfig(ocConfig)
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
				stripTransientProviderFields(provMap)
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
		injectWecomVirtualChannel(cfg, ocConfig)
		channels, _ = ocConfig["channels"].(map[string]interface{})
		c.JSON(http.StatusOK, gin.H{"ok": true, "channels": channels, "plugins": plugins})
	}
}

func pluginInstalled(cfg *config.Config, pluginID string) bool {
	return pluginInstalledAny(cfg, pluginID)

}

func pluginInstalledAny(cfg *config.Config, pluginIDs ...string) bool {
	for _, pluginID := range pluginIDs {
		pluginID = strings.TrimSpace(pluginID)
		if pluginID == "" {
			continue
		}
		for _, candidate := range []string{
			filepath.Join(cfg.OpenClawDir, "extensions", pluginID),
			filepath.Join(filepath.Dir(cfg.OpenClawDir), "extensions", pluginID),
		} {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return true
			}
		}
	}
	return false
}

func qqPluginInstalled(cfg *config.Config) bool {
	return pluginInstalled(cfg, "qq")
}

func qqbotPluginInstalled(cfg *config.Config) bool {
	return pluginInstalled(cfg, "qqbot")
}

func wecomBotPluginInstalled(cfg *config.Config) bool {
	return pluginInstalledAny(cfg, wecomBotPluginIDs...)
}

func resolveWecomBotEntryID(entries map[string]interface{}) string {
	for _, id := range wecomBotPluginIDs {
		if entries[id] != nil {
			return id
		}
	}
	return wecomBotPluginIDs[0]
}

// SaveChannel 保存通道配置
func SaveChannel(cfg *config.Config, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "qq" && !qqPluginInstalled(cfg) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "QQ plugin is not installed; install QQ before configuring the channel"})
			return
		}

		if id == "qqbot" && !qqbotPluginInstalled(cfg) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "QQBot plugin is not installed; install QQBot before configuring the channel"})
			return
		}

		if id == "wecom" && !wecomBotPluginInstalled(cfg) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "WeCom plugin is not installed; install WeCom before configuring the channel"})
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
		if id == "qqbot" {
			body = normalizeQQBotChannelConfig(body)
		}
		if id == "telegram" {
			body = normalizeTelegramChannelConfig(body)
		}
		if id == "feishu" {
			if err := normalizeFeishuChannelConfigInPlace(body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
		}
		if id == "wecom" {
			body = normalizeWeComChannelConfig(body)
			if existing, ok := channels["wecom"].(map[string]interface{}); ok {
				preserveMissingMapFields(body, existing)
			}
		}
		// wecom-app is a virtual channel backed by channels.wecom.agent
		if id == "wecom-app" {
			wecom, _ := channels["wecom"].(map[string]interface{})
			if wecom == nil {
				wecom = map[string]interface{}{}
			}
			agent := map[string]interface{}{}
			for _, k := range []string{"corpId", "corpSecret", "agentId", "token", "encodingAesKey", "encodingAESKey"} {
				if v, ok := body[k]; ok {
					if k == "encodingAesKey" {
						agent["encodingAESKey"] = v
					} else {
						agent[k] = v
					}
				}
			}
			wecom["agent"] = agent
			// carry top-level token/encodingAesKey for wecom channel too
			if t, ok := agent["token"].(string); ok && t != "" {
				wecom["token"] = t
			}
			if k, ok := agent["encodingAESKey"].(string); ok && k != "" {
				wecom["encodingAESKey"] = k
			}
			if _, ok := wecom["enabled"]; !ok {
				wecom["enabled"] = false
			}
			channels["wecom"] = wecom
			delete(channels, "wecom-app")
			ocConfig["channels"] = channels
			if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
			resp := gin.H{"ok": true}
			if procMgr != nil && procMgr.GetStatus().Running {
				if err := procMgr.Restart(); err != nil {
					resp["message"] = "企业微信自建应用配置已保存，但自动重启网关失败，请手动重启 OpenClaw 网关后生效"
				} else {
					resp["message"] = "企业微信自建应用配置已保存，并已自动重启网关使配置生效"
					resp["restarted"] = true
				}
			} else {
				resp["message"] = "企业微信自建应用配置已保存；OpenClaw 网关下次启动时生效"
			}
			c.JSON(http.StatusOK, resp)
			return
		}
		// 保留现有的 enabled 状态：如果前端未传 enabled 字段，沿用原有值
		if _, hasEnabled := body["enabled"]; !hasEnabled {
			if existing, ok := channels[id].(map[string]interface{}); ok {
				if v, ok := existing["enabled"]; ok {
					body["enabled"] = v
				}
			}
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
		} else if procMgr != nil && procMgr.GetStatus().Running {
			if err := procMgr.Restart(); err != nil {
				resp["message"] = "通道配置已保存，但自动重启网关失败，请手动重启 OpenClaw 网关后生效"
				resp["restartWarning"] = err.Error()
			} else {
				resp["message"] = "通道配置已保存，并已自动重启网关使配置生效"
				resp["restarted"] = true
			}
		} else {
			resp["message"] = "通道配置已保存；OpenClaw 网关下次启动时生效"
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

func normalizeQQBotChannelConfig(body map[string]interface{}) map[string]interface{} {
	if body == nil {
		return map[string]interface{}{}
	}
	if appID := strings.TrimSpace(toString(body["appId"])); appID != "" {
		body["appId"] = appID
	} else {
		delete(body, "appId")
	}
	secret := strings.TrimSpace(toString(body["clientSecret"]))
	if secret == "" {
		secret = strings.TrimSpace(toString(body["appSecret"]))
	}
	if secret != "" {
		body["clientSecret"] = secret
	} else {
		delete(body, "clientSecret")
	}
	delete(body, "appSecret")
	delete(body, "token")
	return body
}

func normalizeTelegramChannelConfig(body map[string]interface{}) map[string]interface{} {
	if body == nil {
		return map[string]interface{}{}
	}
	if raw, ok := body["token"]; ok {
		if token := strings.TrimSpace(fmt.Sprint(raw)); token != "" {
			body["botToken"] = token
		}
		delete(body, "token")
	}
	if strings.TrimSpace(fmt.Sprint(body["botToken"])) != "" {
		if strings.TrimSpace(fmt.Sprint(body["dmPolicy"])) == "" {
			body["dmPolicy"] = "open"
		}
		if strings.TrimSpace(fmt.Sprint(body["groupPolicy"])) == "" {
			body["groupPolicy"] = "open"
		}
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(body["dmPolicy"])), "open") {
		body["allowFrom"] = ensureWildcardAllowList(body["allowFrom"])
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(body["groupPolicy"])), "open") {
		body["groupAllowFrom"] = ensureWildcardAllowList(body["groupAllowFrom"])
	}
	return body
}

func ensureWildcardAllowList(raw interface{}) []interface{} {
	entries := make([]interface{}, 0)
	seen := map[string]bool{}
	appendEntry := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		entries = append(entries, v)
	}
	addRaw := func(val interface{}) {
		s := strings.TrimSpace(fmt.Sprint(val))
		if s != "" {
			appendEntry(s)
		}
	}
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			addRaw(item)
		}
	case []string:
		for _, item := range v {
			appendEntry(item)
		}
	case string:
		for _, item := range splitListInput(v) {
			appendEntry(item)
		}
	}
	appendEntry("*")
	return entries
}

func splitListInput(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '，', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func normalizeFeishuRequireMention(raw interface{}) (bool, bool, error) {
	switch v := raw.(type) {
	case nil:
		return false, false, nil
	case bool:
		return v, true, nil
	case string:
		trimmed := strings.TrimSpace(v)
		switch {
		case trimmed == "":
			return false, false, nil
		case strings.EqualFold(trimmed, "open"), strings.EqualFold(trimmed, "allowlist"), strings.EqualFold(trimmed, "closed"):
			// 兼容旧的误写：这些值属于 groupPolicy，不属于 requireMention。
			return false, false, nil
		case strings.EqualFold(trimmed, "true"):
			return true, true, nil
		case strings.EqualFold(trimmed, "false"):
			return false, true, nil
		default:
			return false, false, fmt.Errorf("requireMention 仅支持 true/false，当前值为 %q", trimmed)
		}
	default:
		return false, false, fmt.Errorf("requireMention 仅支持 true/false")
	}
}

func normalizeFeishuChannelConfigInPlace(body map[string]interface{}) error {
	if body == nil {
		return nil
	}
	delete(body, "dmScope")

	if raw, exists := body["requireMention"]; exists {
		normalized, keep, err := normalizeFeishuRequireMention(raw)
		if err != nil {
			return err
		}
		if keep {
			body["requireMention"] = normalized
		} else {
			delete(body, "requireMention")
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

	hasTopLevelCreds := strings.TrimSpace(toString(body["appId"])) != "" && strings.TrimSpace(toString(body["appSecret"])) != ""
	hasRunnableAccount := false
	for _, rawEntry := range normalizedAccounts {
		entry, _ := rawEntry.(map[string]interface{})
		if hasFeishuRunnableCredentials(entry) {
			hasRunnableAccount = true
			break
		}
	}
	if dmPolicy == "" && (hasTopLevelCreds || hasRunnableAccount) {
		body["dmPolicy"] = "open"
	}
	if strings.EqualFold(strings.TrimSpace(toString(body["dmPolicy"])), "open") {
		body["allowFrom"] = ensureWildcardAllowList(body["allowFrom"])
	}
	if groupPolicy == "" && (hasTopLevelCreds || hasRunnableAccount) {
		body["groupPolicy"] = "open"
	}

	return nil
}

func normalizeFeishuChannelConfig(body map[string]interface{}) map[string]interface{} {
	if body == nil {
		return map[string]interface{}{}
	}
	_ = normalizeFeishuChannelConfigInPlace(body)
	return body
}

func normalizeWeComChannelConfig(body map[string]interface{}) map[string]interface{} {
	if body == nil {
		return map[string]interface{}{}
	}
	mode := strings.TrimSpace(strings.ToLower(toString(body["connectionMode"])))
	switch mode {
	case "long_polling", "longpolling":
		mode = "long-polling"
	case "callback", "long-polling":
	default:
		if strings.TrimSpace(toString(body["botId"])) != "" && strings.TrimSpace(toString(body["secret"])) != "" {
			mode = "long-polling"
		} else {
			mode = "callback"
		}
	}
	body["connectionMode"] = mode
	for _, key := range []string{"token", "encodingAESKey", "encodingAesKey", "webhookPath"} {
		if trimmed := strings.TrimSpace(toString(body[key])); trimmed != "" {
			if key == "encodingAesKey" {
				body["encodingAESKey"] = trimmed
				delete(body, "encodingAesKey")
				continue
			}
			body[key] = trimmed
		} else {
			delete(body, key)
		}
	}
	for _, key := range []string{"botId", "secret", "websocketUrl", "name"} {
		if trimmed := strings.TrimSpace(toString(body[key])); trimmed != "" {
			body[key] = trimmed
		} else {
			delete(body, key)
		}
	}
	dmPolicy := strings.TrimSpace(toString(body["dmPolicy"]))
	if dmPolicy == "" && strings.TrimSpace(toString(body["botId"])) != "" && strings.TrimSpace(toString(body["secret"])) != "" {
		dmPolicy = "open"
	}
	if dmPolicy != "" {
		body["dmPolicy"] = dmPolicy
	} else {
		delete(body, "dmPolicy")
	}
	if strings.EqualFold(dmPolicy, "open") {
		body["allowFrom"] = ensureWildcardAllowList(body["allowFrom"])
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
		if id == "wecom" {
			entryID := resolveWecomBotEntryID(entries)
			entries[entryID] = body
			for _, aliasID := range wecomBotPluginIDs {
				if aliasID != entryID {
					delete(entries, aliasID)
				}
			}
		} else {
			entries[id] = body
		}
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
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "QQ plugin is not installed; install QQ before enabling the channel"})
			return
		}

		if req.ChannelID == "qqbot" && !qqbotPluginInstalled(cfg) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "QQBot plugin is not installed; install QQBot before enabling the channel"})
			return
		}

		if req.ChannelID == "wecom" && !wecomBotPluginInstalled(cfg) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "WeCom plugin is not installed; install WeCom before enabling the channel"})
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

		// 更新 plugins.entries
		plugins, _ := ocConfig["plugins"].(map[string]interface{})
		if plugins == nil {
			plugins = map[string]interface{}{}
		}
		if req.ChannelID == "feishu" {
			plugins = cleanupLegacyFeishuPluginIDs(plugins)
		}
		entries, _ := plugins["entries"].(map[string]interface{})
		if entries == nil {
			entries = map[string]interface{}{}
		}

		// 企微互斥：开启其中一个时自动关闭另一个
		wecomMutex := map[string]string{"wecom": "wecom-app", "wecom-app": "wecom"}
		if req.Enabled {
			if mutexID, ok := wecomMutex[req.ChannelID]; ok {
				// 关闭另一个 channel
				if other, ok2 := channels[mutexID].(map[string]interface{}); ok2 {
					other["enabled"] = false
					channels[mutexID] = other
				}
				// 关闭另一个 plugin entry（企业微信机器人使用当前实际插件 ID）
				entryID := resolveWecomBotEntryID(entries)
				if pe, ok2 := entries[entryID].(map[string]interface{}); ok2 {
					pe["enabled"] = false
					entries[entryID] = pe
				}
				// 关闭 channels.wecom 的 enabled（另一个方向时）
				if mutexID == "wecom" {
					if wch, ok2 := channels["wecom"].(map[string]interface{}); ok2 {
						wch["enabled"] = false
						channels["wecom"] = wch
					}
				}
			}
		}

		// wecom-app 是虚拟通道，实际写到 channels.wecom（兼容 @sunnoy/wecom 插件读取 channels.wecom 配置）
		if req.ChannelID == "wecom-app" {
			wecom, _ := channels["wecom"].(map[string]interface{})
			if wecom == nil {
				wecom = map[string]interface{}{}
			}
			wecom["enabled"] = req.Enabled
			channels["wecom"] = wecom
			delete(channels, "wecom-app")
			// plugin entry 使用当前实际的 wecom 机器人插件 ID
			entryID := resolveWecomBotEntryID(entries)
			pe, _ := entries[entryID].(map[string]interface{})
			if pe == nil {
				pe = map[string]interface{}{}
			}
			pe["enabled"] = req.Enabled
			entries[entryID] = pe
			delete(entries, "wecom-app")
			for _, aliasID := range wecomBotPluginIDs {
				if aliasID != entryID {
					delete(entries, aliasID)
				}
			}
		} else {
			ch, _ := channels[req.ChannelID].(map[string]interface{})
			if ch == nil {
				ch = map[string]interface{}{}
			}
			ch["enabled"] = req.Enabled
			channels[req.ChannelID] = ch
			if req.ChannelID == "wecom" {
				delete(channels, "wecom-app")
			}

			// 飞书特殊处理：启用时自愈目标插件 entry，并同步 plugins.allow
			if req.ChannelID == "feishu" {
				targetEntryID := resolveFeishuToggleEntryID(plugins, entries)
				disableIDs := make([]string, 0, len(feishuAllPluginIDs)-1)
				for _, othID := range feishuAllPluginIDs {
					if othID == targetEntryID {
						continue
					}
					disableIDs = append(disableIDs, othID)
				}
				if req.Enabled {
					plugins = ensureFeishuPluginSelection(plugins, targetEntryID, disableIDs)
					entries, _ = plugins["entries"].(map[string]interface{})
				} else {
					for _, pluginID := range feishuAllPluginIDs {
						entry, _ := entries[pluginID].(map[string]interface{})
						if entry == nil {
							continue
						}
						entry["enabled"] = false
						entries[pluginID] = entry
					}
					plugins["entries"] = entries
				}
			} else if req.ChannelID == "wecom" {
				entryID := resolveWecomBotEntryID(entries)
				pe, _ := entries[entryID].(map[string]interface{})
				if pe == nil {
					pe = map[string]interface{}{}
				}
				pe["enabled"] = req.Enabled
				entries[entryID] = pe
				delete(entries, "wecom-app")
				for _, aliasID := range wecomBotPluginIDs {
					if aliasID != entryID {
						delete(entries, aliasID)
					}
				}
			} else {
				pe, _ := entries[req.ChannelID].(map[string]interface{})
				if pe == nil {
					pe = map[string]interface{}{}
				}
				pe["enabled"] = req.Enabled
				entries[req.ChannelID] = pe
				if req.ChannelID == "wecom" {
					delete(entries, "wecom-app")
				}
			}
		}
		ocConfig["channels"] = channels
		plugins["entries"] = entries
		ocConfig["plugins"] = plugins

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		// 企微插件目录弹出式切换：wecom-app 开启时把插件移入 extensions，wecom 开启时移出
		if cfg.OpenClawDir != "" {
			extDir := filepath.Join(cfg.OpenClawDir, "extensions")
			activeDir := filepath.Join(extDir, "wecom-app")
			stashDir := filepath.Join(filepath.Dir(cfg.OpenClawDir), "wecom-app-plugin")
			if req.ChannelID == "wecom-app" && req.Enabled {
				// 移入：把 stash 目录 rename 到 extensions/wecom-app
				if _, err2 := os.Stat(stashDir); err2 == nil {
					if _, err3 := os.Stat(activeDir); err3 != nil {
						_ = os.Rename(stashDir, activeDir)
					}
				}
			} else if (req.ChannelID == "wecom" || req.ChannelID == "wecom-app") && !req.Enabled || (req.ChannelID == "wecom" && req.Enabled) {
				// 移出：把 extensions/wecom-app rename 到 stash
				if _, err2 := os.Stat(activeDir); err2 == nil {
					if _, err3 := os.Stat(stashDir); err3 != nil {
						_ = os.Rename(activeDir, stashDir)
					}
				}
			}
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

		// 重启网关使配置生效
		if procMgr != nil && procMgr.GetStatus().Running {
			_ = procMgr.Restart()
		}

		channelNames := map[string]string{
			"qq": "QQ (NapCat)", "wechat": "微信", "feishu": "飞书",
			"qqbot": "QQ Bot", "dingtalk": "钉钉", "wecom": "企业微信", "openclaw-weixin": "微信（ClawBot）",
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
	modelsPath := filepath.Join(resolveAgentConfigDir(cfg, agentID), "models.json")
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

// resolveFeishuEntryID 从 candidates 中查找飞书插件 entry ID。
// requireEnabled=true 时只返回 enabled 的；false 时先找 enabled 再找 present。
// 未找到返回 ""。
func resolveFeishuEntryID(entries map[string]interface{}, candidates []string, requireEnabled bool) string {
	for _, id := range candidates {
		if entry, ok := entries[id].(map[string]interface{}); ok {
			if enabled, _ := entry["enabled"].(bool); enabled {
				return id
			}
		}
	}
	if requireEnabled {
		return ""
	}
	for _, id := range candidates {
		if entries[id] != nil {
			return id
		}
	}
	return ""
}

func cleanupLegacyFeishuPluginIDs(plugins map[string]interface{}) map[string]interface{} {
	if plugins == nil {
		plugins = map[string]interface{}{}
	}
	entries, _ := plugins["entries"].(map[string]interface{})
	if entries != nil {
		delete(entries, "feishu-openclaw-plugin")
		plugins["entries"] = entries
	}
	allowList, _ := plugins["allow"].([]interface{})
	plugins["allow"] = removeStringFromInterfaceSlice(allowList, "feishu-openclaw-plugin")
	return plugins
}

func resolveFeishuPluginIDFromAllow(plugins map[string]interface{}, candidates []string) string {
	allowList, _ := plugins["allow"].([]interface{})
	for _, value := range allowList {
		pluginID := strings.TrimSpace(fmt.Sprint(value))
		if containsString(candidates, pluginID) {
			return pluginID
		}
	}
	return ""
}

func resolveFeishuPluginIDFromInstalls(plugins map[string]interface{}, candidates []string) string {
	installs, _ := plugins["installs"].(map[string]interface{})
	for _, id := range candidates {
		if installs[id] != nil {
			return id
		}
	}
	return ""
}

// resolvePreferredOfficialFeishuID 确定官方版飞书插件 ID。
// 优先级：entries 中已启用 → allow 列表 → entries 中已存在 → installs → 默认 canonical ID。
func resolvePreferredOfficialFeishuID(plugins map[string]interface{}, entries map[string]interface{}) string {
	if id := resolveFeishuEntryID(entries, feishuOfficialPluginIDs, true); id != "" {
		return id
	}
	if allowID := resolveFeishuPluginIDFromAllow(plugins, feishuOfficialPluginIDs); allowID != "" {
		return allowID
	}
	if id := resolveFeishuEntryID(entries, feishuOfficialPluginIDs, false); id != "" {
		return id
	}
	if installedID := resolveFeishuPluginIDFromInstalls(plugins, feishuOfficialPluginIDs); installedID != "" {
		return installedID
	}
	return canonicalFeishuOfficialPluginID
}

// resolveFeishuToggleEntryID 确定飞书渠道开关操作的目标 entry ID（官方版或社区版均可）。
// 优先级：entries 中已启用 → allow 列表 → entries 中已存在 → 官方已安装 → 社区已安装 → 默认社区版。
func resolveFeishuToggleEntryID(plugins map[string]interface{}, entries map[string]interface{}) string {
	if id := resolveFeishuEntryID(entries, feishuAllPluginIDs, true); id != "" {
		return id
	}
	if allowID := resolveFeishuPluginIDFromAllow(plugins, feishuAllPluginIDs); allowID != "" {
		return allowID
	}
	if id := resolveFeishuEntryID(entries, feishuAllPluginIDs, false); id != "" {
		return id
	}
	if installedID := resolveFeishuPluginIDFromInstalls(plugins, feishuOfficialPluginIDs); installedID != "" {
		return installedID
	}
	if installedID := resolveFeishuPluginIDFromInstalls(plugins, []string{canonicalFeishuCommunityPluginID}); installedID != "" {
		return installedID
	}
	return canonicalFeishuCommunityPluginID
}

func ensureStringSliceContains(values []interface{}, target string) []interface{} {
	for _, value := range values {
		if strings.TrimSpace(fmt.Sprint(value)) == target {
			return values
		}
	}
	return append(values, target)
}

func removeStringFromInterfaceSlice(values []interface{}, targets ...string) []interface{} {
	if len(values) == 0 || len(targets) == 0 {
		return values
	}
	result := make([]interface{}, 0, len(values))
	for _, value := range values {
		current := strings.TrimSpace(fmt.Sprint(value))
		if current != "" && containsString(targets, current) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func ensureFeishuPluginSelection(plugins map[string]interface{}, enableID string, disableIDs []string) map[string]interface{} {
	if plugins == nil {
		plugins = map[string]interface{}{}
	}
	entries, _ := plugins["entries"].(map[string]interface{})
	if entries == nil {
		entries = map[string]interface{}{}
	}
	entry, _ := entries[enableID].(map[string]interface{})
	if entry == nil {
		entry = map[string]interface{}{}
	}
	entry["enabled"] = true
	entries[enableID] = entry
	for _, disableID := range disableIDs {
		if disableID == enableID {
			continue
		}
		if otherEntry, ok := entries[disableID].(map[string]interface{}); ok {
			otherEntry["enabled"] = false
			entries[disableID] = otherEntry
		}
	}
	plugins["entries"] = entries

	allowList, _ := plugins["allow"].([]interface{})
	allowList = removeStringFromInterfaceSlice(allowList, disableIDs...)
	allowList = ensureStringSliceContains(allowList, enableID)
	plugins["allow"] = allowList
	return plugins
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

		// 飞书变体切换：官方版优先保留当前实际存在的官方 ID，社区版固定使用 feishu，并同步 plugins.allow
		var enableID string
		label := "ClawTeam 社区版"
		if req.Variant == "official" {
			enableID = resolvePreferredOfficialFeishuID(plugins, entries)
			label = "飞书官方版"
		} else {
			enableID = canonicalFeishuCommunityPluginID
		}
		disableIDs := make([]string, 0, len(feishuAllPluginIDs)-1)
		for _, pluginID := range feishuAllPluginIDs {
			if pluginID == enableID {
				continue
			}
			disableIDs = append(disableIDs, pluginID)
		}
		plugins = ensureFeishuPluginSelection(plugins, enableID, disableIDs)
		entries, _ = plugins["entries"].(map[string]interface{})

		// Lite 版只有内置插件目录 feishu；同时统一清理旧官方 ID 脏配置
		if cfg.IsLiteEdition() {
			for _, aliasID := range feishuOfficialPluginIDs {
				delete(entries, aliasID)
			}
			plugins["entries"] = entries
		}
		plugins = cleanupLegacyFeishuPluginIDs(plugins)
		ocConfig["plugins"] = plugins

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		if len(sysLog) > 0 && sysLog[0] != nil {
			sysLog[0].Log("system", "channel.variant_switched", "飞书通道切换为"+label)
		}

		resp := gin.H{"ok": true, "message": "飞书通道已切换为" + label}
		if procMgr != nil && procMgr.GetStatus().Running {
			if err := procMgr.Restart(); err != nil {
				resp["message"] = "飞书通道已切换为" + label + "，但自动重启网关失败，请手动重启 OpenClaw 网关后生效"
				resp["restartWarning"] = err.Error()
			} else {
				resp["restarted"] = true
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}
