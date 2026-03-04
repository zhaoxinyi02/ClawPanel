package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/eventlog"
	"github.com/zhaoxinyi02/ClawPanel/internal/monitor"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
)

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

		// 自动为非 OpenAI 提供商注入 compat.supportsDeveloperRole=false
		injectCompatFlags(ocCfg)

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
			if models, ok := ocConfig["models"].(map[string]interface{}); ok {
				models["providers"] = providers
			} else {
				ocConfig["models"] = map[string]interface{}{"providers": providers}
			}
		}

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
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

// SaveChannel 保存通道配置
func SaveChannel(cfg *config.Config) gin.HandlerFunc {
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

		channels, _ := ocConfig["channels"].(map[string]interface{})
		if channels == nil {
			channels = map[string]interface{}{}
		}
		channels[id] = body
		ocConfig["channels"] = channels

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
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
		pe, _ := entries[req.ChannelID].(map[string]interface{})
		if pe == nil {
			pe = map[string]interface{}{}
		}
		pe["enabled"] = req.Enabled
		entries[req.ChannelID] = pe
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
	modelsPath := filepath.Join(cfg.OpenClawDir, "agents", agentID, "agent", "models.json")
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
