package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func findAgentByID(list []map[string]interface{}, id string) map[string]interface{} {
	for _, item := range list {
		if strings.TrimSpace(toString(item["id"])) == id {
			return item
		}
	}
	return nil
}

// GetOpenClawAgents 返回多智能体配置与统计信息。
func GetOpenClawAgents(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isLegacySingleAgentMode() {
			c.JSON(http.StatusOK, gin.H{
				"ok": true,
				"agents": gin.H{
					"default":           "main",
					"defaultConfigured": true,
					"defaults":          gin.H{},
					"list": []gin.H{
						{"id": "main", "default": true, "implicit": false},
					},
					"bindings": []interface{}{},
				},
			})
			return
		}

		ocConfig, err := cfg.ReadOpenClawJSON()
		if err != nil || ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}
		agentsCfg := ensureAgentsConfig(ocConfig)
		list := parseAgentsListFromConfig(ocConfig)
		hasExplicitList := len(list) > 0
		defaultConfigured := hasExplicitDefaultAgent(ocConfig, list)
		defaultID := loadDefaultAgentID(cfg)
		if defaultID == "" {
			defaultID = "main"
		}

		if !hasExplicitList {
			ids, agentSet := collectAgentIDsFromConfigAndDisk(cfg, ocConfig)
			if defaultID != "" {
				if _, ok := agentSet[defaultID]; !ok {
					ids = append(ids, defaultID)
					sort.Slice(ids, func(i, j int) bool {
						if ids[i] == "main" {
							return true
						}
						if ids[j] == "main" {
							return false
						}
						return ids[i] < ids[j]
					})
				}
			}
			for _, id := range ids {
				list = append(list, map[string]interface{}{"id": id, "implicit": true})
			}
		}

		enriched := make([]map[string]interface{}, 0, len(list))
		for _, item := range list {
			cur := deepCloneMap(item)
			id := strings.TrimSpace(toString(cur["id"]))
			if id == "" {
				continue
			}
			cur["id"] = id
			cur["default"] = id == defaultID
			cur["implicit"] = asBool(cur["implicit"])
			sessions, lastActive := getAgentSessionStats(cfg, id)
			cur["sessions"] = sessions
			cur["lastActive"] = lastActive
			enriched = append(enriched, cur)
		}

		bindings := getBindingsFromConfig(ocConfig, agentsCfg)
		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"agents": gin.H{
				"default":           defaultID,
				"defaultConfigured": defaultConfigured,
				"defaults":          readMap(agentsCfg["defaults"]),
				"list":              enriched,
				"bindings":          bindings,
			},
		})
	}
}

// CreateOpenClawAgent 创建 Agent。
func CreateOpenClawAgent(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isLegacySingleAgentMode() {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "LEGACY_SINGLE_AGENT=true 模式下不可修改 agents"})
			return
		}

		payload, err := readAgentPayload(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		id := strings.TrimSpace(toString(payload["id"]))
		if err := validateAgentID(id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}
		agentsCfg := ensureAgentsConfig(ocConfig)
		explicitList := parseAgentsListFromConfig(ocConfig)
		list := materializeAgentList(cfg, ocConfig)

		newItem := deepCloneMap(payload)
		newItem["id"] = id
		existingIdx := -1
		for i, item := range list {
			if strings.TrimSpace(toString(item["id"])) == id {
				existingIdx = i
				break
			}
		}
		if existingIdx >= 0 {
			if len(explicitList) > 0 {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("agent id 已存在: %s", id)})
				return
			}
			merged := deepCloneMap(list[existingIdx])
			for k, v := range newItem {
				if k == "id" {
					continue
				}
				if v == nil {
					delete(merged, k)
					continue
				}
				merged[k] = deepCloneAny(v)
			}
			merged["id"] = id
			stripUnsupportedAgentContextOverrides(merged)
			if err := validateAgentIdentityConfig(cfg, id, merged, true); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
			workspace := strings.TrimSpace(toString(merged["workspace"]))
			agentDir := strings.TrimSpace(toString(merged["agentDir"]))
			if err := validateAgentUniqueness(cfg, list, id, workspace, agentDir, id); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
			list[existingIdx] = merged
		} else {
			stripUnsupportedAgentContextOverrides(newItem)
			if err := validateAgentIdentityConfig(cfg, id, newItem, true); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
			workspace := strings.TrimSpace(toString(newItem["workspace"]))
			agentDir := strings.TrimSpace(toString(newItem["agentDir"]))
			if err := validateAgentUniqueness(cfg, list, id, workspace, agentDir, ""); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
			list = append(list, newItem)
		}

		defaultID := getDefaultAgentID(ocConfig, list)
		if asBool(newItem["default"]) {
			defaultID = id
		}
		writeAgentsList(agentsCfg, list, defaultID)

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if created := findAgentByID(list, id); created != nil {
			_ = scaffoldAgentFiles(cfg, created)
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// UpdateOpenClawAgent 更新 Agent。
func UpdateOpenClawAgent(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isLegacySingleAgentMode() {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "LEGACY_SINGLE_AGENT=true 模式下不可修改 agents"})
			return
		}

		id := strings.TrimSpace(c.Param("id"))
		if err := validateAgentID(id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		payload, err := readAgentPayload(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		if payloadID := strings.TrimSpace(toString(payload["id"])); payloadID != "" && payloadID != id {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "不允许修改 agent id"})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}
		agentsCfg := ensureAgentsConfig(ocConfig)
		list := materializeAgentList(cfg, ocConfig)

		idx := -1
		for i, item := range list {
			if strings.TrimSpace(toString(item["id"])) == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "agent 不存在"})
			return
		}

		merged := deepCloneMap(list[idx])
		for k, v := range payload {
			if k == "id" {
				continue
			}
			if v == nil {
				delete(merged, k)
				continue
			}
			merged[k] = deepCloneAny(v)
		}
		merged["id"] = id
		stripUnsupportedAgentContextOverrides(merged)
		if err := validateAgentIdentityConfig(cfg, id, merged, shouldStrictValidateAgentAvatar(list[idx], payload)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		workspace := strings.TrimSpace(toString(merged["workspace"]))
		agentDir := strings.TrimSpace(toString(merged["agentDir"]))
		if err := validateAgentUniqueness(cfg, list, id, workspace, agentDir, id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		list[idx] = merged
		defaultID := getDefaultAgentID(ocConfig, list)
		if asBool(merged["default"]) {
			defaultID = id
		} else if defaultID == id && hasExplicitDefaultFalse(payload) {
			defaultID = ""
		}
		if defaultID == "" {
			defaultID = pickFallbackDefault(list)
		}
		writeAgentsList(agentsCfg, list, defaultID)

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		_ = scaffoldAgentFiles(cfg, merged)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// DeleteOpenClawAgent 删除 Agent。
func DeleteOpenClawAgent(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isLegacySingleAgentMode() {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "LEGACY_SINGLE_AGENT=true 模式下不可修改 agents"})
			return
		}

		id := strings.TrimSpace(c.Param("id"))
		if err := validateAgentID(id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		preserveSessions := strings.EqualFold(c.DefaultQuery("preserveSessions", "false"), "true")
		openClawPath := filepath.Join(cfg.OpenClawDir, "openclaw.json")
		originalOpenClawJSON, err := os.ReadFile(openClawPath)
		if err != nil && !os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}
		agentsCfg := ensureAgentsConfig(ocConfig)
		list := materializeAgentList(cfg, ocConfig)

		filtered := make([]map[string]interface{}, 0, len(list))
		found := false
		for _, item := range list {
			if strings.TrimSpace(toString(item["id"])) == id {
				found = true
				continue
			}
			filtered = append(filtered, item)
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "agent 不存在"})
			return
		}
		if len(filtered) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "至少保留一个 agent"})
			return
		}

		bindings := getBindingsFromConfig(ocConfig, agentsCfg)
		nextBindings := make([]map[string]interface{}, 0, len(bindings))
		for _, b := range bindings {
			if extractBindingAgentID(b) == id {
				continue
			}
			nextBindings = append(nextBindings, b)
		}
		setBindingsToConfig(ocConfig, agentsCfg, nextBindings)

		defaultID := getDefaultAgentID(ocConfig, list)
		if defaultID == id {
			defaultID = pickFallbackDefault(filtered)
		}
		if cronCfg, ok := ocConfig["cron"].(map[string]interface{}); ok {
			if jobs, ok := cronCfg["jobs"].([]interface{}); ok {
				rewriteDeletedAgentCronJobs(jobs, id, defaultID)
			}
		}
		cronPath, originalCronData, updatedCronData, cronChanged, err := rewriteCronSessionTargetsForDeletedAgent(cfg, id, defaultID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		writeAgentsList(agentsCfg, filtered, defaultID)

		sessionsDir, stagedSessionsDir := "", ""
		if !preserveSessions {
			var err error
			sessionsDir, stagedSessionsDir, err = stageAgentSessionsRemoval(cfg, id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
		}

		if cronChanged {
			if err := replaceFileAtomically(cronPath, updatedCronData, 0644); err != nil {
				if stagedSessionsDir != "" {
					if restoreErr := os.Rename(stagedSessionsDir, sessionsDir); restoreErr != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": fmt.Sprintf("写入 cron 配置失败，且恢复 sessions 失败: %v / %v", err, restoreErr)})
						return
					}
				}
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": fmt.Sprintf("写入 cron 配置失败: %v", err)})
				return
			}
		}

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			restoreFailures := make([]string, 0, 3)
			if restoreErr := restoreFile(openClawPath, originalOpenClawJSON); restoreErr != nil {
				restoreFailures = append(restoreFailures, fmt.Sprintf("恢复 openclaw.json 失败: %v", restoreErr))
			}
			if stagedSessionsDir != "" {
				if restoreErr := os.Rename(stagedSessionsDir, sessionsDir); restoreErr != nil {
					restoreFailures = append(restoreFailures, fmt.Sprintf("恢复 sessions 失败: %v", restoreErr))
				}
			}
			if cronChanged {
				if restoreErr := restoreFile(cronPath, originalCronData); restoreErr != nil {
					restoreFailures = append(restoreFailures, fmt.Sprintf("恢复 cron 失败: %v", restoreErr))
				}
			}
			if len(restoreFailures) > 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": fmt.Sprintf("写入配置失败: %v；%s", err, strings.Join(restoreFailures, "；"))})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if stagedSessionsDir != "" {
			if err := os.RemoveAll(stagedSessionsDir); err != nil {
				c.JSON(http.StatusOK, gin.H{"ok": true, "warning": fmt.Sprintf("agent 已删除，但清理 sessions 失败: %v", err)})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetOpenClawBindings 获取 bindings 列表。
func GetOpenClawBindings(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}
		agentsCfg := ensureAgentsConfig(ocConfig)
		c.JSON(http.StatusOK, gin.H{"ok": true, "bindings": getBindingsFromConfig(ocConfig, agentsCfg)})
	}
}

// SaveOpenClawBindings 保存 bindings（全量替换，保序）。
func SaveOpenClawBindings(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isLegacySingleAgentMode() {
			c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "LEGACY_SINGLE_AGENT=true 模式下不可修改 bindings"})
			return
		}

		var req struct {
			Bindings []map[string]interface{} `json:"bindings"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		agentIDs, agentSet := loadAgentIDs(cfg)
		if len(agentIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "未发现可用 agent"})
			return
		}
		for i, b := range req.Bindings {
			if err := validateBindingForWrite(i, b, agentSet); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}
		agentsCfg := ensureAgentsConfig(ocConfig)
		setBindingsToConfig(ocConfig, agentsCfg, req.Bindings)

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// PreviewOpenClawRoute 基于 bindings 规则预览路由结果。
func PreviewOpenClawRoute(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Meta map[string]interface{} `json:"meta"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		if req.Meta == nil {
			req.Meta = map[string]interface{}{}
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}
		agentsCfg := ensureAgentsConfig(ocConfig)
		defaultID := loadDefaultAgentID(cfg)
		if defaultID == "" {
			defaultID = "main"
		}
		if isLegacySingleAgentMode() {
			defaultID = "main"
		}

		channelDefaultAccounts := collectChannelDefaultAccounts(ocConfig)
		resultAgent, matchedBy, matchedIndex, trace := evaluateOfficialRoutePreview(req.Meta, getBindingsFromConfig(ocConfig, agentsCfg), defaultID, channelDefaultAccounts)
		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"result": gin.H{
				"agent":        resultAgent,
				"matchedBy":    matchedBy,
				"matchedIndex": matchedIndex,
				"trace":        trace,
			},
		})
	}
}

func stageAgentSessionsRemoval(cfg *config.Config, agentID string) (string, string, error) {
	sessionsDir := resolveAgentSessionsDir(cfg, agentID)
	info, err := os.Stat(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("检查 sessions 目录失败: %w", err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("sessions 路径不是目录: %s", sessionsDir)
	}

	stagedDir := filepath.Join(filepath.Dir(sessionsDir), fmt.Sprintf(".sessions-delete-%d", time.Now().UnixNano()))
	if err := os.Rename(sessionsDir, stagedDir); err != nil {
		return "", "", fmt.Errorf("暂存 sessions 目录失败: %w", err)
	}
	return sessionsDir, stagedDir, nil
}

func rewriteCronSessionTargetsForDeletedAgent(cfg *config.Config, deletedAgentID, fallbackAgentID string) (string, []byte, []byte, bool, error) {
	cronPath := filepath.Join(cfg.OpenClawDir, "cron", "jobs.json")
	original, err := os.ReadFile(cronPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cronPath, nil, nil, false, nil
		}
		return "", nil, nil, false, err
	}

	var cronData map[string]interface{}
	if err := json.Unmarshal(original, &cronData); err != nil {
		return "", nil, nil, false, fmt.Errorf("解析 cron jobs 失败: %w", err)
	}

	jobs, _ := cronData["jobs"].([]interface{})
	changed := rewriteDeletedAgentCronJobs(jobs, deletedAgentID, fallbackAgentID)
	if !changed {
		return cronPath, original, nil, false, nil
	}

	updated, err := json.MarshalIndent(cronData, "", "  ")
	if err != nil {
		return "", nil, nil, false, fmt.Errorf("序列化 cron jobs 失败: %w", err)
	}
	return cronPath, original, updated, true, nil
}

func rewriteDeletedAgentCronJobs(jobs []interface{}, deletedAgentID, fallbackAgentID string) bool {
	changed := false
	for _, rawJob := range jobs {
		job, ok := rawJob.(map[string]interface{})
		if !ok {
			continue
		}
		agentID := strings.TrimSpace(toString(job["agentId"]))
		sessionTarget := strings.TrimSpace(toString(job["sessionTarget"]))
		rewroteJob := false
		if agentID == deletedAgentID {
			job["agentId"] = fallbackAgentID
			changed = true
			rewroteJob = true
		}
		if sessionTarget == deletedAgentID {
			if agentID == "" || agentID == deletedAgentID {
				job["agentId"] = fallbackAgentID
			}
			job["sessionTarget"] = "main"
			changed = true
			rewroteJob = true
		}
		if rewroteJob && strings.TrimSpace(toString(job["sessionTarget"])) == "" {
			job["sessionTarget"] = "main"
			changed = true
		}
	}
	return changed
}

func restoreFile(path string, content []byte) error {
	if content == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func collectChannelDefaultAccounts(ocConfig map[string]interface{}) map[string]string {
	result := map[string]string{}
	if ocConfig == nil {
		return result
	}
	channels, _ := ocConfig["channels"].(map[string]interface{})
	if channels == nil {
		return result
	}

	for channelID, raw := range channels {
		ch, _ := raw.(map[string]interface{})
		if ch == nil {
			continue
		}
		channelID = strings.TrimSpace(channelID)
		if channelID == "" {
			continue
		}
		if accountID := resolveChannelDefaultAccountID(ch); accountID != "" {
			result[channelID] = accountID
		}
	}
	return result
}

func resolveChannelDefaultAccountID(channelCfg map[string]interface{}) string {
	if channelCfg == nil {
		return ""
	}
	if accountID := strings.TrimSpace(toString(channelCfg["defaultAccount"])); accountID != "" {
		return accountID
	}
	accounts, _ := channelCfg["accounts"].(map[string]interface{})
	if len(accounts) == 0 {
		return ""
	}
	if _, ok := accounts["default"]; ok {
		return "default"
	}
	keys := make([]string, 0, len(accounts))
	for key := range accounts {
		if s := strings.TrimSpace(key); s != "" {
			keys = append(keys, s)
		}
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		return keys[0]
	}
	return ""
}

func ensureAgentsConfig(ocConfig map[string]interface{}) map[string]interface{} {
	if ocConfig == nil {
		ocConfig = map[string]interface{}{}
	}
	agents, _ := ocConfig["agents"].(map[string]interface{})
	if agents == nil {
		agents = map[string]interface{}{}
		ocConfig["agents"] = agents
	}
	return agents
}

func readMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return deepCloneMap(m)
	}
	return map[string]interface{}{}
}

func parseBindings(v interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0)
	switch t := v.(type) {
	case []interface{}:
		for _, item := range t {
			if m, ok := item.(map[string]interface{}); ok {
				cp := deepCloneMap(m)
				normalizeBinding(cp)
				result = append(result, cp)
			}
		}
	case []map[string]interface{}:
		for _, item := range t {
			cp := deepCloneMap(item)
			normalizeBinding(cp)
			result = append(result, cp)
		}
	}
	return result
}

func getBindingsFromConfig(ocConfig, agentsCfg map[string]interface{}) []map[string]interface{} {
	if ocConfig != nil {
		if _, ok := ocConfig["bindings"]; ok {
			parsed := parseBindings(ocConfig["bindings"])
			if len(parsed) > 0 {
				return parsed
			}
		}
	}
	if agentsCfg != nil {
		if _, ok := agentsCfg["bindings"]; ok {
			parsed := parseBindings(agentsCfg["bindings"])
			if len(parsed) > 0 {
				return parsed
			}
		}
	}
	return []map[string]interface{}{}
}

func setBindingsToConfig(ocConfig, agentsCfg map[string]interface{}, bindings []map[string]interface{}) {
	normalized := make([]map[string]interface{}, 0, len(bindings))
	for _, b := range bindings {
		cp := deepCloneMap(b)
		normalized = append(normalized, normalizeBindingForWrite(cp))
	}
	if ocConfig != nil {
		ocConfig["bindings"] = mapSliceToAny(normalized)
	}
	if agentsCfg != nil {
		delete(agentsCfg, "bindings")
	}
}

func normalizeBinding(binding map[string]interface{}) {
	normalizeBindingTopLevel(binding)
}

func extractBindingAgentID(binding map[string]interface{}) string {
	if binding == nil {
		return ""
	}
	agentID := strings.TrimSpace(toString(binding["agentId"]))
	if agentID != "" {
		return agentID
	}
	return strings.TrimSpace(toString(binding["agent"]))
}

func stringSliceFromAny(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if s := strings.TrimSpace(t); s != "" {
			return []string{s}
		}
	}
	return nil
}

func writeAgentsList(agentsCfg map[string]interface{}, list []map[string]interface{}, defaultID string) {
	if strings.TrimSpace(defaultID) == "" || !listContainsAgent(list, defaultID) {
		defaultID = pickFallbackDefault(list)
	}
	delete(agentsCfg, "default")

	for _, item := range list {
		id := strings.TrimSpace(toString(item["id"]))
		item["id"] = id
		if id != "" && id == defaultID {
			item["default"] = true
			continue
		}
		delete(item, "default")
	}
	agentsCfg["list"] = mapSliceToAny(list)
}

func materializeAgentList(cfg *config.Config, ocConfig map[string]interface{}) []map[string]interface{} {
	list := parseAgentsListFromConfig(ocConfig)
	if len(list) > 0 {
		return list
	}

	ids, agentSet := collectAgentIDsFromConfigAndDisk(cfg, ocConfig)
	defaultID := loadDefaultAgentID(cfg)
	if defaultID != "" {
		if _, ok := agentSet[defaultID]; !ok {
			ids = append(ids, defaultID)
			sortAgentIDs(ids)
		}
	}

	materialized := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			continue
		}
		materialized = append(materialized, map[string]interface{}{"id": id})
	}
	return materialized
}

func mapSliceToAny(list []map[string]interface{}) []interface{} {
	out := make([]interface{}, 0, len(list))
	for _, item := range list {
		out = append(out, deepCloneMap(item))
	}
	return out
}

func readAgentPayload(c *gin.Context) (map[string]interface{}, error) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		return nil, err
	}
	if nested, ok := body["agent"].(map[string]interface{}); ok && nested != nil {
		return deepCloneMap(nested), nil
	}
	return deepCloneMap(body), nil
}

func validateAgentID(id string) error {
	if id == "" {
		return fmt.Errorf("agent id 不能为空")
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("agent id 仅支持字母数字下划线中划线")
	}
	return nil
}

func stripUnsupportedAgentContextOverrides(agent map[string]interface{}) {
	if agent == nil {
		return
	}
	delete(agent, "contextTokens")
	delete(agent, "compaction")
	if tools, ok := agent["tools"].(map[string]interface{}); ok && tools != nil {
		delete(tools, "agentToAgent")
		delete(tools, "sessions")
		if len(tools) == 0 {
			delete(agent, "tools")
		}
	}
}

func extractAgentIdentityMap(agent map[string]interface{}) map[string]interface{} {
	if agent == nil {
		return nil
	}
	identity, _ := agent["identity"].(map[string]interface{})
	return identity
}

func shouldStrictValidateAgentAvatar(existingAgent, payload map[string]interface{}) bool {
	rawIdentity, ok := payload["identity"]
	if !ok || rawIdentity == nil {
		return false
	}
	identity, ok := rawIdentity.(map[string]interface{})
	if !ok {
		return true
	}
	rawAvatar, avatarProvided := identity["avatar"]
	if !avatarProvided {
		return false
	}
	nextAvatar := trimStringField(rawAvatar)
	prevAvatar := ""
	if existingIdentity := extractAgentIdentityMap(existingAgent); existingIdentity != nil {
		prevAvatar = trimStringField(existingIdentity["avatar"])
	}
	return nextAvatar != prevAvatar
}

func validateAgentUniqueness(cfg *config.Config, list []map[string]interface{}, id, workspace, agentDir, skipID string) error {
	// workspace 允许绝对路径在 OpenClawDir 外部（如外部硬盘），仅做归一化
	workspace = normalizeAgentPath(cfg.OpenClawDir, workspace)
	if agentDir != "" {
		normalizedAgentDir, err := normalizeAgentPathWithinBase(cfg.OpenClawDir, agentDir)
		if err != nil {
			return fmt.Errorf("agentDir 必须位于 OpenClaw 目录内")
		}
		agentDir = normalizedAgentDir
	}
	canonicalAgentDir := canonicalizeNormalizedAgentDir(agentDir)
	for _, item := range list {
		curID := strings.TrimSpace(toString(item["id"]))
		if curID == "" || curID == skipID {
			continue
		}
		if curID == id {
			return fmt.Errorf("agent id 已存在: %s", id)
		}
		normalizedWorkspace := normalizeAgentPath(cfg.OpenClawDir, toString(item["workspace"]))
		if workspace != "" && workspace == normalizedWorkspace {
			return fmt.Errorf("workspace 已被占用: %s", workspace)
		}
		normalizedAgentDir, err := normalizeAgentPathWithinBase(cfg.OpenClawDir, toString(item["agentDir"]))
		if err != nil {
			continue
		}
		canonicalExistingAgentDir := canonicalizeNormalizedAgentDir(normalizedAgentDir)
		if agentDir != "" && (agentDir == normalizedAgentDir ||
			canonicalAgentDir == canonicalExistingAgentDir ||
			agentDir == filepath.Join(canonicalExistingAgentDir, "agent") ||
			normalizedAgentDir == filepath.Join(canonicalAgentDir, "agent")) {
			return fmt.Errorf("agentDir 已被占用: %s", agentDir)
		}
	}
	return nil
}

func hasExplicitDefaultFalse(payload map[string]interface{}) bool {
	v, ok := payload["default"]
	return ok && !asBool(v)
}

func pickFallbackDefault(list []map[string]interface{}) string {
	for _, item := range list {
		if id := strings.TrimSpace(toString(item["id"])); id != "" {
			return id
		}
	}
	return "main"
}

func listContainsAgent(list []map[string]interface{}, agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return false
	}
	for _, item := range list {
		if strings.TrimSpace(toString(item["id"])) == agentID {
			return true
		}
	}
	return false
}

func getAgentSessionStats(cfg *config.Config, agentID string) (int, int64) {
	sessionsPath := filepath.Join(resolveAgentSessionsDir(cfg, agentID), "sessions.json")
	data, err := os.ReadFile(sessionsPath)
	if err != nil {
		return 0, 0
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, 0
	}
	count := 0
	var lastActive int64
	for _, val := range raw {
		m, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		count++
		if updated, ok := m["updatedAt"].(float64); ok {
			ts := int64(updated)
			if ts > lastActive {
				lastActive = ts
			}
		}
	}
	return count, lastActive
}
