package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

// GetOpenClawAgents 返回多智能体配置与统计信息。
func GetOpenClawAgents(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isLegacySingleAgentMode() {
			c.JSON(http.StatusOK, gin.H{
				"ok": true,
				"agents": gin.H{
					"default":  "main",
					"defaults": gin.H{},
					"list": []gin.H{
						{"id": "main", "default": true},
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
		defaultID := getDefaultAgentID(ocConfig, list)
		if defaultID == "" {
			defaultID = "main"
		}

		if len(list) == 0 {
			ids, _ := collectAgentIDsFromConfigAndDisk(cfg, ocConfig)
			for _, id := range ids {
				list = append(list, map[string]interface{}{"id": id})
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
			sessions, lastActive := getAgentSessionStats(cfg, id)
			cur["sessions"] = sessions
			cur["lastActive"] = lastActive
			enriched = append(enriched, cur)
		}

		bindings := getBindingsFromConfig(ocConfig, agentsCfg)
		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"agents": gin.H{
				"default":  defaultID,
				"defaults": readMap(agentsCfg["defaults"]),
				"list":     enriched,
				"bindings": bindings,
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
		list := parseAgentsListFromConfig(ocConfig)

		workspace := strings.TrimSpace(toString(payload["workspace"]))
		agentDir := strings.TrimSpace(toString(payload["agentDir"]))
		if err := validateAgentUniqueness(list, id, workspace, agentDir, ""); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		newItem := deepCloneMap(payload)
		newItem["id"] = id
		list = append(list, newItem)

		defaultID := strings.TrimSpace(toString(agentsCfg["default"]))
		if asBool(newItem["default"]) || defaultID == "" {
			defaultID = id
		}
		writeAgentsList(agentsCfg, list, defaultID)

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
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
		list := parseAgentsListFromConfig(ocConfig)

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
			merged[k] = deepCloneAny(v)
		}
		merged["id"] = id

		workspace := strings.TrimSpace(toString(merged["workspace"]))
		agentDir := strings.TrimSpace(toString(merged["agentDir"]))
		if err := validateAgentUniqueness(list, id, workspace, agentDir, id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		list[idx] = merged
		defaultID := strings.TrimSpace(toString(agentsCfg["default"]))
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

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}
		agentsCfg := ensureAgentsConfig(ocConfig)
		list := parseAgentsListFromConfig(ocConfig)

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

		defaultID := strings.TrimSpace(toString(agentsCfg["default"]))
		if defaultID == id {
			defaultID = pickFallbackDefault(filtered)
		}
		writeAgentsList(agentsCfg, filtered, defaultID)

		if !preserveSessions {
			_ = os.RemoveAll(filepath.Join(cfg.OpenClawDir, "agents", id, "sessions"))
		}

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
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
			normalizeBinding(b)
			agent := extractBindingAgentID(b)
			if agent == "" {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("bindings[%d] 缺少 agent", i)})
				return
			}
			if _, ok := agentSet[agent]; !ok {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("bindings[%d] 指向不存在的 agent: %s", i, agent)})
				return
			}
			if _, ok := b["enabled"]; !ok {
				b["enabled"] = true
			}
			if _, ok := b["match"].(map[string]interface{}); !ok {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("bindings[%d] 缺少有效 match", i)})
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
		list := parseAgentsListFromConfig(ocConfig)
		defaultID := getDefaultAgentID(ocConfig, list)
		if defaultID == "" {
			defaultID = "main"
		}
		if isLegacySingleAgentMode() {
			defaultID = "main"
		}

		resultAgent, matchedBy, trace := evaluateRoutePreview(req.Meta, getBindingsFromConfig(ocConfig, agentsCfg), defaultID)
		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"result": gin.H{
				"agent":     resultAgent,
				"matchedBy": matchedBy,
				"trace":     trace,
			},
		})
	}
}

func evaluateRoutePreview(meta map[string]interface{}, bindings []map[string]interface{}, defaultAgent string) (string, string, []string) {
	if isLegacySingleAgentMode() {
		return "main", "legacy-single-agent", []string{"LEGACY_SINGLE_AGENT=true", "fallback main"}
	}

	trace := make([]string, 0, len(bindings)*2+1)
	for i, rule := range bindings {
		enabled := true
		if v, ok := rule["enabled"].(bool); ok {
			enabled = v
		}
		if !enabled {
			trace = append(trace, fmt.Sprintf("skip bindings[%d]: disabled", i))
			continue
		}
		agent := extractBindingAgentID(rule)
		if agent == "" {
			trace = append(trace, fmt.Sprintf("skip bindings[%d]: missing agent", i))
			continue
		}
		match, _ := rule["match"].(map[string]interface{})
		if len(match) == 0 {
			trace = append(trace, fmt.Sprintf("skip bindings[%d]: empty match", i))
			continue
		}

		keys := make([]string, 0, len(match))
		for k := range match {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		hitField := ""
		matched := true
		for _, key := range keys {
			expected := match[key]
			actual, ok := meta[key]
			if !ok {
				// 兼容 parentPeer 与 peer 的双向兜底
				if key == "parentPeer" {
					actual, ok = meta["peer"]
				} else if key == "peer" {
					actual, ok = meta["parentPeer"]
				}
			}
			if !ok {
				trace = append(trace, fmt.Sprintf("bindings[%d] miss meta.%s", i, key))
				matched = false
				break
			}
			if !matchMetaValue(actual, expected) {
				trace = append(trace, fmt.Sprintf("bindings[%d] mismatch %s", i, key))
				matched = false
				break
			}
			hitField = key
		}

		if matched {
			trace = append(trace, fmt.Sprintf("hit bindings[%d]", i))
			if hitField == "" {
				hitField = "unknown"
			}
			return agent, fmt.Sprintf("bindings[%d].match.%s", i, hitField), trace
		}
	}

	if strings.TrimSpace(defaultAgent) == "" {
		defaultAgent = "main"
	}
	trace = append(trace, "fallback default")
	return defaultAgent, "default", trace
}

func matchMetaValue(actual, expected interface{}) bool {
	// roles 语义：任一角色命中即视为匹配
	if expectedRoles, ok := expected.([]interface{}); ok {
		if actualRoles := stringSliceFromAny(actual); len(actualRoles) > 0 {
			exp := stringSliceFromAny(expectedRoles)
			for _, av := range actualRoles {
				for _, ev := range exp {
					if matchMetaValue(av, ev) {
						return true
					}
				}
			}
			return false
		}
	}

	if actualArr := anySlice(actual); len(actualArr) > 0 {
		for _, item := range actualArr {
			if matchMetaValue(item, expected) {
				return true
			}
		}
		return false
	}

	switch e := expected.(type) {
	case []interface{}:
		for _, item := range e {
			if matchMetaValue(actual, item) {
				return true
			}
		}
		return false
	case string:
		actualStr := strings.TrimSpace(fmt.Sprint(actual))
		expectedStr := strings.TrimSpace(e)
		if strings.ContainsAny(expectedStr, "*?[]") {
			ok, err := path.Match(expectedStr, actualStr)
			return err == nil && ok
		}
		return actualStr == expectedStr
	default:
		return fmt.Sprint(actual) == fmt.Sprint(expected)
	}
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
			return parseBindings(ocConfig["bindings"])
		}
	}
	if agentsCfg != nil {
		if _, ok := agentsCfg["bindings"]; ok {
			return parseBindings(agentsCfg["bindings"])
		}
	}
	return []map[string]interface{}{}
}

func setBindingsToConfig(ocConfig, agentsCfg map[string]interface{}, bindings []map[string]interface{}) {
	normalized := make([]map[string]interface{}, 0, len(bindings))
	for _, b := range bindings {
		cp := deepCloneMap(b)
		normalizeBinding(cp)
		normalized = append(normalized, cp)
	}
	if ocConfig != nil {
		ocConfig["bindings"] = mapSliceToAny(normalized)
	}
	if agentsCfg != nil {
		// 向后兼容旧实现：同步写入 agents.bindings
		agentsCfg["bindings"] = mapSliceToAny(normalized)
	}
}

func normalizeBinding(binding map[string]interface{}) {
	if binding == nil {
		return
	}
	agentID := strings.TrimSpace(toString(binding["agentId"]))
	if agentID == "" {
		agentID = strings.TrimSpace(toString(binding["agent"]))
	}
	if agentID != "" {
		binding["agentId"] = agentID
		binding["agent"] = agentID
	}
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

func anySlice(v interface{}) []interface{} {
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	if arr, ok := v.([]string); ok {
		out := make([]interface{}, 0, len(arr))
		for _, s := range arr {
			out = append(out, s)
		}
		return out
	}
	return nil
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
	if strings.TrimSpace(defaultID) == "" {
		defaultID = pickFallbackDefault(list)
	}
	agentsCfg["default"] = defaultID

	for _, item := range list {
		id := strings.TrimSpace(toString(item["id"]))
		item["id"] = id
		item["default"] = id != "" && id == defaultID
	}
	agentsCfg["list"] = mapSliceToAny(list)
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

func validateAgentUniqueness(list []map[string]interface{}, id, workspace, agentDir, skipID string) error {
	for _, item := range list {
		curID := strings.TrimSpace(toString(item["id"]))
		if curID == "" || curID == skipID {
			continue
		}
		if curID == id {
			return fmt.Errorf("agent id 已存在: %s", id)
		}
		if workspace != "" && workspace == strings.TrimSpace(toString(item["workspace"])) {
			return fmt.Errorf("workspace 已被占用: %s", workspace)
		}
		if agentDir != "" && agentDir == strings.TrimSpace(toString(item["agentDir"])) {
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

func getAgentSessionStats(cfg *config.Config, agentID string) (int, int64) {
	sessionsPath := filepath.Join(cfg.OpenClawDir, "agents", agentID, "sessions", "sessions.json")
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
