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
	"time"

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
				merged[k] = deepCloneAny(v)
			}
			merged["id"] = id
			workspace := strings.TrimSpace(toString(merged["workspace"]))
			agentDir := strings.TrimSpace(toString(merged["agentDir"]))
			if err := validateAgentUniqueness(list, id, workspace, agentDir, id); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
			list[existingIdx] = merged
		} else {
			workspace := strings.TrimSpace(toString(newItem["workspace"]))
			agentDir := strings.TrimSpace(toString(newItem["agentDir"]))
			if err := validateAgentUniqueness(list, id, workspace, agentDir, ""); err != nil {
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

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			if stagedSessionsDir != "" {
				if restoreErr := os.Rename(stagedSessionsDir, sessionsDir); restoreErr != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": fmt.Sprintf("写入配置失败，且恢复 sessions 失败: %v / %v", err, restoreErr)})
					return
				}
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
			match, ok := b["match"].(map[string]interface{})
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("bindings[%d] 缺少有效 match", i)})
				return
			}
			if err := validateBindingMatch(i, match); err != nil {
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
		resultAgent, matchedBy, trace := evaluateRoutePreview(req.Meta, getBindingsFromConfig(ocConfig, agentsCfg), defaultID, channelDefaultAccounts)
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

func stageAgentSessionsRemoval(cfg *config.Config, agentID string) (string, string, error) {
	sessionsDir := filepath.Join(cfg.OpenClawDir, "agents", agentID, "sessions")
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

var bindingMatchAllowedKeys = []string{
	"channel",
	"sender",
	"peer",
	"parentPeer",
	"guildId",
	"teamId",
	"accountId",
	"roles",
}

var bindingMatchAllowedSet = func() map[string]struct{} {
	out := make(map[string]struct{}, len(bindingMatchAllowedKeys))
	for _, k := range bindingMatchAllowedKeys {
		out[k] = struct{}{}
	}
	return out
}()

var bindingMatchEvalOrder = []string{
	"channel",
	"sender",
	"peer",
	"parentPeer",
	"guildId",
	"roles",
	"teamId",
	"accountId",
}

type routePreviewCandidate struct {
	index       int
	agent       string
	priority    int
	priorityKey string
}

func evaluateRoutePreview(meta map[string]interface{}, bindings []map[string]interface{}, defaultAgent string, channelDefaultAccounts map[string]string) (string, string, []string) {
	if isLegacySingleAgentMode() {
		return "main", "legacy-single-agent", []string{"LEGACY_SINGLE_AGENT=true", "fallback main"}
	}

	trace := make([]string, 0, len(bindings)*2+2)
	candidates := make([]routePreviewCandidate, 0, len(bindings))
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
		if err := validateBindingMatch(i, match); err != nil {
			trace = append(trace, fmt.Sprintf("skip bindings[%d]: invalid match (%s)", i, err.Error()))
			continue
		}
		matched, reason := matchBindingRule(meta, match)
		if !matched {
			trace = append(trace, fmt.Sprintf("bindings[%d] %s", i, reason))
			continue
		}
		if ok, reason := matchImplicitDefaultAccount(meta, match, channelDefaultAccounts); !ok {
			trace = append(trace, fmt.Sprintf("bindings[%d] %s", i, reason))
			continue
		}
		priority, priorityKey := bindingMatchPriority(match)
		trace = append(trace, fmt.Sprintf("hit bindings[%d]: priority %s", i, priorityLabel(priorityKey)))
		candidates = append(candidates, routePreviewCandidate{
			index:       i,
			agent:       agent,
			priority:    priority,
			priorityKey: priorityKey,
		})
	}

	if len(candidates) > 0 {
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].priority != candidates[j].priority {
				return candidates[i].priority < candidates[j].priority
			}
			return candidates[i].index < candidates[j].index
		})
		best := candidates[0]
		trace = append(trace, fmt.Sprintf("select bindings[%d]: %s", best.index, priorityLabel(best.priorityKey)))
		return best.agent, fmt.Sprintf("bindings[%d].match.%s", best.index, best.priorityKey), trace
	}

	if strings.TrimSpace(defaultAgent) == "" {
		defaultAgent = "main"
	}
	trace = append(trace, "fallback default")
	return defaultAgent, "default", trace
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

func matchImplicitDefaultAccount(meta, match map[string]interface{}, channelDefaultAccounts map[string]string) (bool, string) {
	if hasMatchField(match, "accountId") {
		return true, ""
	}
	channel, ok := singleLiteralChannel(match)
	if !ok {
		return true, ""
	}
	defaultAccount := strings.TrimSpace(channelDefaultAccounts[channel])
	if defaultAccount == "" {
		return true, ""
	}
	actual, ok := readMetaForMatch(meta, "accountId")
	if !ok {
		// 预览 meta 未提供 accountId 时，无法确定具体账号，保持兼容。
		return true, ""
	}
	if matchMetaValue(actual, defaultAccount) {
		return true, ""
	}
	return false, fmt.Sprintf("mismatch implicit default account (channel=%s, default=%s)", channel, defaultAccount)
}

func singleLiteralChannel(match map[string]interface{}) (string, bool) {
	raw, ok := match["channel"]
	if !ok {
		return "", false
	}
	channel, ok := raw.(string)
	if !ok {
		return "", false
	}
	channel = strings.TrimSpace(channel)
	if channel == "" || strings.ContainsAny(channel, "*?[]") {
		return "", false
	}
	return channel, true
}

func priorityLabel(key string) string {
	switch key {
	case "sender":
		return "sender"
	case "peer":
		return "peer"
	case "parentPeer":
		return "parentPeer"
	case "guildId+roles":
		return "guildId+roles"
	case "guildId":
		return "guildId"
	case "teamId":
		return "teamId"
	case "accountId":
		return "accountId"
	case "accountId:*":
		return "accountId:*"
	case "channel":
		return "channel"
	default:
		return "generic"
	}
}

func bindingMatchPriority(match map[string]interface{}) (int, string) {
	if hasMatchField(match, "sender") {
		return 1, "sender"
	}
	if hasMatchField(match, "peer") {
		return 2, "peer"
	}
	if hasMatchField(match, "parentPeer") {
		return 3, "parentPeer"
	}
	if hasMatchField(match, "guildId") && hasMatchField(match, "roles") {
		return 4, "guildId+roles"
	}
	if hasMatchField(match, "guildId") {
		return 5, "guildId"
	}
	if hasMatchField(match, "teamId") {
		return 6, "teamId"
	}
	if hasMatchField(match, "accountId") {
		if isWildcardMatchValue(match["accountId"]) {
			return 8, "accountId:*"
		}
		return 7, "accountId"
	}
	if hasMatchField(match, "channel") {
		return 9, "channel"
	}
	return 10, "generic"
}

func hasMatchField(match map[string]interface{}, key string) bool {
	if match == nil {
		return false
	}
	_, ok := match[key]
	return ok
}

func isWildcardMatchValue(v interface{}) bool {
	switch t := v.(type) {
	case string:
		return strings.ContainsAny(strings.TrimSpace(t), "*?[]")
	case []interface{}:
		for _, item := range t {
			if isWildcardMatchValue(item) {
				return true
			}
		}
		return false
	case []string:
		for _, item := range t {
			if isWildcardMatchValue(item) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func matchBindingRule(meta, match map[string]interface{}) (bool, string) {
	keys := orderedMatchKeys(match)
	for _, key := range keys {
		expected := match[key]
		actual, ok := readMetaForMatch(meta, key)
		if !ok {
			return false, fmt.Sprintf("miss meta.%s", key)
		}
		if !matchBindingFieldValue(key, actual, expected) {
			return false, fmt.Sprintf("mismatch %s", key)
		}
	}
	return true, "matched"
}

func orderedMatchKeys(match map[string]interface{}) []string {
	keys := make([]string, 0, len(match))
	seen := map[string]struct{}{}
	for _, key := range bindingMatchEvalOrder {
		if _, ok := match[key]; ok {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	extras := make([]string, 0)
	for key := range match {
		if _, ok := seen[key]; ok {
			continue
		}
		extras = append(extras, key)
	}
	sort.Strings(extras)
	keys = append(keys, extras...)
	return keys
}

func readMetaForMatch(meta map[string]interface{}, key string) (interface{}, bool) {
	actual, ok := meta[key]
	if ok {
		return actual, true
	}
	// parentPeer 与 peer 允许双向兜底，适配不同 channel 的元数据格式。
	if key == "parentPeer" {
		actual, ok = meta["peer"]
		return actual, ok
	}
	if key == "peer" {
		actual, ok = meta["parentPeer"]
		return actual, ok
	}
	return nil, false
}

func matchBindingFieldValue(field string, actual, expected interface{}) bool {
	switch field {
	case "peer", "parentPeer":
		return matchPeerField(actual, expected)
	default:
		return matchMetaValue(actual, expected)
	}
}

func matchPeerField(actual, expected interface{}) bool {
	if expectedObj, ok := expected.(map[string]interface{}); ok {
		actualPeer, ok := normalizePeerValue(actual)
		if !ok {
			return false
		}
		if kindExp, exists := expectedObj["kind"]; exists && !matchMetaValue(actualPeer["kind"], kindExp) {
			return false
		}
		if idExp, exists := expectedObj["id"]; exists && !matchMetaValue(actualPeer["id"], idExp) {
			return false
		}
		return true
	}
	return matchMetaValue(actual, expected)
}

func normalizePeerValue(v interface{}) (map[string]interface{}, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		kind := strings.TrimSpace(toString(t["kind"]))
		id := strings.TrimSpace(toString(t["id"]))
		if kind == "" {
			if raw := strings.TrimSpace(toString(t["raw"])); raw != "" {
				kind, id = splitPeerKindID(raw)
			}
		}
		if kind == "" {
			if tp := strings.TrimSpace(toString(t["type"])); tp != "" {
				kind = tp
			}
		}
		if kind == "" {
			return nil, false
		}
		return map[string]interface{}{"kind": kind, "id": id}, true
	case string:
		kind, id := splitPeerKindID(strings.TrimSpace(t))
		if kind == "" {
			return nil, false
		}
		return map[string]interface{}{"kind": kind, "id": id}, true
	default:
		raw := strings.TrimSpace(fmt.Sprint(v))
		if raw == "" || raw == "<nil>" {
			return nil, false
		}
		kind, id := splitPeerKindID(raw)
		if kind == "" {
			return nil, false
		}
		return map[string]interface{}{"kind": kind, "id": id}, true
	}
}

func splitPeerKindID(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(parts[0]), ""
}

func validateBindingMatch(index int, match map[string]interface{}) error {
	if len(match) == 0 {
		return fmt.Errorf("bindings[%d] 缺少有效 match", index)
	}
	if _, ok := match["channel"]; !ok {
		return fmt.Errorf("bindings[%d].match.channel 必填", index)
	}
	if _, ok := match["roles"]; ok {
		if _, hasGuild := match["guildId"]; !hasGuild {
			return fmt.Errorf("bindings[%d].match.roles 需与 guildId 同时使用", index)
		}
	}
	for key, val := range match {
		if _, ok := bindingMatchAllowedSet[key]; !ok {
			return fmt.Errorf("bindings[%d].match.%s 不支持，允许字段: %s", index, key, strings.Join(bindingMatchAllowedKeys, ", "))
		}
		var err error
		switch key {
		case "peer", "parentPeer":
			err = validatePeerMatchField(val)
		default:
			err = validateStringMatchField(val)
		}
		if err != nil {
			return fmt.Errorf("bindings[%d].match.%s %s", index, key, err.Error())
		}
	}
	return nil
}

func validateStringMatchField(v interface{}) error {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return fmt.Errorf("不能为空字符串")
		}
		return nil
	case []string:
		if len(t) == 0 {
			return fmt.Errorf("不能为空数组")
		}
		for i, item := range t {
			if strings.TrimSpace(item) == "" {
				return fmt.Errorf("数组项[%d] 不能为空字符串", i)
			}
		}
		return nil
	case []interface{}:
		if len(t) == 0 {
			return fmt.Errorf("不能为空数组")
		}
		for i, item := range t {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("数组项[%d] 仅支持字符串", i)
			}
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("数组项[%d] 不能为空字符串", i)
			}
		}
		return nil
	default:
		return fmt.Errorf("仅支持字符串或字符串数组")
	}
}

func validatePeerMatchField(v interface{}) error {
	if err := validateStringMatchField(v); err == nil {
		return nil
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf("仅支持字符串、字符串数组或对象")
	}
	for key := range obj {
		if key != "kind" && key != "id" {
			return fmt.Errorf("对象仅允许 kind/id 字段")
		}
	}
	kindRaw, ok := obj["kind"]
	if !ok {
		return fmt.Errorf("对象模式缺少 kind")
	}
	if err := validateStringMatchField(kindRaw); err != nil {
		return fmt.Errorf("kind %s", err.Error())
	}
	if idRaw, ok := obj["id"]; ok {
		if err := validateStringMatchField(idRaw); err != nil {
			return fmt.Errorf("id %s", err.Error())
		}
	}
	return nil
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
