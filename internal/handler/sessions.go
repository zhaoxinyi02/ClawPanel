package handler

import (
	"bufio"
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

const sessionScannerMaxTokenSize = 16 * 1024 * 1024
const dashboardSessionPreviewLimit = 20

// SessionInfo represents a session entry from sessions.json
type SessionInfo struct {
	AgentID        string                   `json:"agentId,omitempty"`
	Key            string                   `json:"key"`
	SessionID      string                   `json:"sessionId"`
	ChatType       string                   `json:"chatType"`
	LastChannel    string                   `json:"lastChannel"`
	LastTo         string                   `json:"lastTo"`
	UpdatedAt      int64                    `json:"updatedAt"`
	OriginLabel    string                   `json:"originLabel"`
	OriginProvider string                   `json:"originProvider"`
	OriginFrom     string                   `json:"originFrom"`
	SessionFile    string                   `json:"sessionFile"`
	MessageCount   int                      `json:"messageCount"`
	RecentMessages []map[string]interface{} `json:"recentMessages,omitempty"`
}

// SessionMessage represents a message in a session JSONL file
type SessionMessage struct {
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	ParentID  string      `json:"parentId,omitempty"`
	Timestamp string      `json:"timestamp"`
	Message   *MsgContent `json:"message,omitempty"`
	// For non-message types
	CustomType string      `json:"customType,omitempty"`
	Data       interface{} `json:"data,omitempty"`
}

type UsageTotals struct {
	Input       int64   `json:"input"`
	Output      int64   `json:"output"`
	CacheRead   int64   `json:"cacheRead"`
	CacheWrite  int64   `json:"cacheWrite"`
	TotalTokens int64   `json:"totalTokens"`
	TotalCost   float64 `json:"totalCost"`
	Requests    int     `json:"requests"`
	Sessions    int     `json:"sessions"`
}

type UsageSummary struct {
	Today   UsageTotals `json:"today"`
	Last7d  UsageTotals `json:"last7d"`
	Last30d UsageTotals `json:"last30d"`
}

type AgentUsageSummary struct {
	AgentID string      `json:"agentId"`
	Today   UsageTotals `json:"today"`
	Last7d  UsageTotals `json:"last7d"`
	Last30d UsageTotals `json:"last30d"`
}

// MsgContent represents the message content
type MsgContent struct {
	Role      string      `json:"role"`
	Content   interface{} `json:"content"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

// GetSessions returns the list of all sessions
func GetSessions(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := strings.TrimSpace(c.Query("agent"))
		if agentID == "" {
			agentID = loadDefaultAgentID(cfg)
		}
		if isLegacySingleAgentMode() {
			agentID = "main"
		}

		if agentID == "all" {
			agentIDs, _ := loadAgentIDs(cfg)
			merged := make([]SessionInfo, 0, 128)
			for _, id := range agentIDs {
				items := loadSessionsByAgent(cfg, id)
				merged = append(merged, items...)
			}
			sort.Slice(merged, func(i, j int) bool {
				return merged[i].UpdatedAt > merged[j].UpdatedAt
			})
			c.JSON(http.StatusOK, gin.H{"ok": true, "sessions": merged})
			return
		}

		if err := validateAgentQuery(cfg, agentID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		sessions := loadSessionsByAgent(cfg, agentID)
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].UpdatedAt > sessions[j].UpdatedAt
		})

		c.JSON(http.StatusOK, gin.H{"ok": true, "sessions": sessions})
	}
}

// GetSessionDetail returns the messages in a specific session
func GetSessionDetail(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")
		agentID := strings.TrimSpace(c.Query("agent"))
		if agentID == "" {
			agentID = loadDefaultAgentID(cfg)
		}
		if isLegacySingleAgentMode() {
			agentID = "main"
		}
		if agentID == "all" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "agent=all 不支持会话详情查询"})
			return
		}
		if err := validateAgentQuery(cfg, agentID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		limit := 100
		if l := c.Query("limit"); l != "" {
			if v, err := json.Number(l).Int64(); err == nil && v > 0 {
				limit = int(v)
			}
		}

		sessionsDir := resolveAgentSessionsDir(cfg, agentID)
		sessionFile := resolveSessionTranscriptPath(sessionsDir, sessionID)

		if sessionFile == "" {
			c.JSON(http.StatusOK, gin.H{"ok": true, "messages": []interface{}{}, "error": "会话文件不存在"})
			return
		}

		messages, err := readSessionMessages(sessionFile, limit)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "messages": []interface{}{}, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "messages": messages})
	}
}

// DeleteSession deletes a session
func DeleteSession(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")
		agentID := strings.TrimSpace(c.Query("agent"))
		if agentID == "" {
			agentID = loadDefaultAgentID(cfg)
		}
		if isLegacySingleAgentMode() {
			agentID = "main"
		}
		if agentID == "all" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "agent=all 不支持删除会话"})
			return
		}
		if err := validateAgentQuery(cfg, agentID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		sessionsPath := filepath.Join(resolveAgentSessionsDir(cfg, agentID), "sessions.json")

		data, err := os.ReadFile(sessionsPath)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "无法读取会话列表"})
			return
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "解析失败"})
			return
		}

		// Find and remove the session by sessionId
		found := false
		for key, val := range raw {
			if v, ok := val.(map[string]interface{}); ok {
				if getString(v, "sessionId") == sessionID {
					// Delete session file
					if sf := getString(v, "sessionFile"); sf != "" {
						os.Remove(sf)
					}
					delete(raw, key)
					found = true
					break
				}
			}
		}

		if !found {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "会话不存在"})
			return
		}

		// Write back
		newData, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "序列化失败"})
			return
		}
		if err := os.WriteFile(sessionsPath, newData, 0644); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "写入失败: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "会话已删除"})
	}
}

func GetSessionUsage(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := strings.TrimSpace(c.Query("agent"))
		if agentID == "" {
			agentID = "all"
		}
		if isLegacySingleAgentMode() {
			agentID = "main"
		}

		var agentIDs []string
		if agentID == "all" {
			agentIDs, _ = loadAgentIDs(cfg)
		} else {
			if err := validateAgentQuery(cfg, agentID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
			agentIDs = []string{agentID}
		}

		summary := UsageSummary{}
		agents := make([]AgentUsageSummary, 0, len(agentIDs))
		for _, id := range agentIDs {
			cur := collectAgentUsage(cfg, id)
			agents = append(agents, cur)
			mergeUsageTotals(&summary.Today, cur.Today)
			mergeUsageTotals(&summary.Last7d, cur.Last7d)
			mergeUsageTotals(&summary.Last30d, cur.Last30d)
		}

		sort.Slice(agents, func(i, j int) bool {
			if agents[i].AgentID == "main" {
				return true
			}
			if agents[j].AgentID == "main" {
				return false
			}
			if agents[i].Last30d.TotalTokens == agents[j].Last30d.TotalTokens {
				return agents[i].AgentID < agents[j].AgentID
			}
			return agents[i].Last30d.TotalTokens > agents[j].Last30d.TotalTokens
		})

		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"summary": summary,
			"agents":  agents,
		})
	}
}

func readSessionMessages(filePath string, limit int) ([]map[string]interface{}, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var allMessages []map[string]interface{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), sessionScannerMaxTokenSize)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		entryType, _ := entry["type"].(string)

		// Only include message entries and assistant responses
		if entryType == "message" {
			msg := extractMessage(entry)
			if msg != nil {
				allMessages = append(allMessages, msg)
			}
		} else if entryType == "assistant" {
			msg := extractAssistantMessage(entry)
			if msg != nil {
				allMessages = append(allMessages, msg)
			}
		}
	}

	// Return last N messages
	if len(allMessages) > limit {
		allMessages = allMessages[len(allMessages)-limit:]
	}

	return allMessages, nil
}

func resolveSessionTranscriptPath(sessionsDir, sessionID string) string {
	if strings.TrimSpace(sessionsDir) == "" || strings.TrimSpace(sessionID) == "" {
		return ""
	}
	defaultPath := filepath.Join(sessionsDir, sessionID+".jsonl")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	sessionsIndex := filepath.Join(sessionsDir, "sessions.json")
	data, err := os.ReadFile(sessionsIndex)
	if err != nil {
		return ""
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	for _, val := range raw {
		entry, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		if getString(entry, "sessionId") != sessionID {
			continue
		}
		if sf := strings.TrimSpace(getString(entry, "sessionFile")); sf != "" {
			return sf
		}
	}
	return ""
}

func extractMessage(entry map[string]interface{}) map[string]interface{} {
	msg, ok := entry["message"].(map[string]interface{})
	if !ok {
		return nil
	}

	role, _ := msg["role"].(string)
	content := cleanSessionMessageText(extractTextContent(msg["content"]), role)
	ts, _ := entry["timestamp"].(string)

	if content == "" {
		return nil
	}

	result := map[string]interface{}{
		"id":        entry["id"],
		"role":      role,
		"content":   content,
		"timestamp": ts,
	}

	return result
}

func extractAssistantMessage(entry map[string]interface{}) map[string]interface{} {
	msg, ok := entry["message"].(map[string]interface{})
	if !ok {
		return nil
	}

	content := cleanSessionMessageText(extractTextContent(msg["content"]), "assistant")
	ts, _ := entry["timestamp"].(string)

	if content == "" {
		return nil
	}

	return map[string]interface{}{
		"id":        entry["id"],
		"role":      "assistant",
		"content":   content,
		"timestamp": ts,
	}
}

func extractTextContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, _ := m["type"].(string); t == "text" {
					if text, _ := m["text"].(string); text != "" {
						parts = append(parts, text)
					}
				} else if t == "tool_use" {
					name, _ := m["name"].(string)
					parts = append(parts, "[工具调用: "+name+"]")
				} else if t == "tool_result" {
					parts = append(parts, "[工具结果]")
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func cleanSessionMessageText(content, role string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	if role == "assistant" {
		text = strings.TrimPrefix(text, "[[reply_to_current]]")
		text = strings.TrimPrefix(text, "[[reply_to_parent]]")
		text = strings.TrimPrefix(text, "[[reply_to_thread]]")
		return strings.TrimSpace(text)
	}

	if strings.HasPrefix(text, "[") {
		if idx := strings.Index(text, "] "); idx > 0 && idx < 80 {
			text = strings.TrimSpace(text[idx+2:])
		}
	}

	if strings.HasPrefix(text, "Conversation info (untrusted metadata):") {
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = strings.TrimSpace(text[idx+3:])
		}
	}
	if strings.Contains(text, "<qqimg>") || strings.Contains(text, "<qqvoice>") || strings.Contains(text, "<qqfile>") || strings.Contains(text, "<qqvideo>") {
		if idx := strings.LastIndex(text, ">"); idx >= 0 && idx+1 < len(text) {
			tail := strings.TrimSpace(text[idx+1:])
			if tail != "" {
				text = tail
			}
		}
	}

	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if looksLikeSessionInstructionLine(trimmed) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	if len(filtered) == 0 {
		return strings.TrimSpace(text)
	}

	start := len(filtered) - 1
	for start > 0 {
		prev := filtered[start-1]
		if looksLikeCompactUserLine(prev) {
			start--
			continue
		}
		break
	}
	return strings.TrimSpace(strings.Join(filtered[start:], "\n"))
}

func looksLikeSessionInstructionLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if lower == "```" || strings.HasPrefix(lower, "{") || strings.HasPrefix(lower, "}") {
		return true
	}
	prefixes := []string{
		"conversation info",
		"你正在通过",
		"【会话上下文】",
		"- 用户:",
		"- 场景:",
		"- 消息id:",
		"- 投递目标:",
		"- 当前时间戳",
		"- 定时提醒投递地址:",
		"【发送图片",
		"【发送语音",
		"【发送文件",
		"【发送视频",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	if strings.HasPrefix(lower, "support") || strings.HasPrefix(lower, "示例:") || strings.HasPrefix(lower, "图片来源:") {
		return true
	}
	if strings.HasPrefix(lower, "1.") || strings.HasPrefix(lower, "2.") || strings.HasPrefix(lower, "3.") || strings.HasPrefix(lower, "4.") || strings.HasPrefix(lower, "5.") || strings.HasPrefix(lower, "6.") || strings.HasPrefix(lower, "7.") {
		return true
	}
	return false
}

func looksLikeCompactUserLine(line string) bool {
	if len(line) > 240 {
		return false
	}
	if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
		return false
	}
	return true
}

func summarizeSessionTranscript(filePath string, previewLimit int) (int, []map[string]interface{}) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, nil
	}
	defer f.Close()

	count := 0
	recent := make([]map[string]interface{}, 0, previewLimit)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), sessionScannerMaxTokenSize)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entryType, _ := entry["type"].(string)
		var msg map[string]interface{}
		if entryType == "message" {
			msg = extractMessage(entry)
		} else if entryType == "assistant" {
			msg = extractAssistantMessage(entry)
		}
		if msg != nil {
			count++
			if previewLimit > 0 {
				recent = append(recent, msg)
				if len(recent) > previewLimit {
					recent = recent[len(recent)-previewLimit:]
				}
			}
		}
	}
	return count, recent
}

func loadSessionsByAgent(cfg *config.Config, agentID string) []SessionInfo {
	sessionsPath := filepath.Join(resolveAgentSessionsDir(cfg, agentID), "sessions.json")
	data, err := os.ReadFile(sessionsPath)
	if err != nil {
		return []SessionInfo{}
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return []SessionInfo{}
	}

	sessions := make([]SessionInfo, 0, len(raw))
	for key, val := range raw {
		v, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		si := SessionInfo{
			AgentID:     agentID,
			Key:         key,
			SessionID:   getString(v, "sessionId"),
			ChatType:    getString(v, "chatType"),
			LastChannel: getString(v, "lastChannel"),
			LastTo:      getString(v, "lastTo"),
		}
		if updatedAt, ok := v["updatedAt"].(float64); ok {
			si.UpdatedAt = int64(updatedAt)
		}
		if origin, ok := v["origin"].(map[string]interface{}); ok {
			si.OriginLabel = getString(origin, "label")
			si.OriginProvider = getString(origin, "provider")
			si.OriginFrom = getString(origin, "from")
		}
		si.SessionFile = getString(v, "sessionFile")
		if si.SessionFile != "" {
			// The dashboard recent-activity feed relies on this preview window.
			// Keep a wider tail so slightly older chat turns do not disappear.
			si.MessageCount, si.RecentMessages = summarizeSessionTranscript(si.SessionFile, dashboardSessionPreviewLimit)
		}
		sessions = append(sessions, si)
	}
	return sessions
}

type usageAccumulator struct {
	UsageTotals
	sessionSet map[string]struct{}
}

type usageRecord struct {
	Input       int64
	Output      int64
	CacheRead   int64
	CacheWrite  int64
	TotalTokens int64
	TotalCost   float64
}

func collectAgentUsage(cfg *config.Config, agentID string) AgentUsageSummary {
	result := AgentUsageSummary{AgentID: agentID}
	sessionsDir := resolveAgentSessionsDir(cfg, agentID)
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return result
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	last7dStart := todayStart.AddDate(0, 0, -6)
	last30dStart := todayStart.AddDate(0, 0, -29)

	todayAcc := usageAccumulator{sessionSet: map[string]struct{}{}}
	last7dAcc := usageAccumulator{sessionSet: map[string]struct{}{}}
	last30dAcc := usageAccumulator{sessionSet: map[string]struct{}{}}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		collectUsageFromFile(filepath.Join(sessionsDir, entry.Name()), sessionID, todayStart, last7dStart, last30dStart, &todayAcc, &last7dAcc, &last30dAcc)
	}

	result.Today = todayAcc.UsageTotals
	result.Last7d = last7dAcc.UsageTotals
	result.Last30d = last30dAcc.UsageTotals
	return result
}

func collectUsageFromFile(filePath, sessionID string, todayStart, last7dStart, last30dStart time.Time, todayAcc, last7dAcc, last30dAcc *usageAccumulator) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), sessionScannerMaxTokenSize)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if t, _ := entry["type"].(string); t != "message" {
			continue
		}

		msg, ok := entry["message"].(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := msg["role"].(string); role != "assistant" {
			continue
		}

		usage, ok := parseUsageRecord(msg["usage"])
		if !ok {
			continue
		}
		ts := extractUsageTimestamp(entry, msg)
		if ts.IsZero() || ts.Before(last30dStart) {
			continue
		}

		addUsage(todayAcc, sessionID, usage, !ts.Before(todayStart))
		addUsage(last7dAcc, sessionID, usage, !ts.Before(last7dStart))
		addUsage(last30dAcc, sessionID, usage, true)
	}
}

func addUsage(acc *usageAccumulator, sessionID string, usage usageRecord, enabled bool) {
	if !enabled {
		return
	}
	acc.Input += usage.Input
	acc.Output += usage.Output
	acc.CacheRead += usage.CacheRead
	acc.CacheWrite += usage.CacheWrite
	acc.TotalTokens += usage.TotalTokens
	acc.TotalCost += usage.TotalCost
	acc.Requests++
	if sessionID != "" {
		acc.sessionSet[sessionID] = struct{}{}
		acc.Sessions = len(acc.sessionSet)
	}
}

func mergeUsageTotals(dst *UsageTotals, src UsageTotals) {
	dst.Input += src.Input
	dst.Output += src.Output
	dst.CacheRead += src.CacheRead
	dst.CacheWrite += src.CacheWrite
	dst.TotalTokens += src.TotalTokens
	dst.TotalCost += src.TotalCost
	dst.Requests += src.Requests
	dst.Sessions += src.Sessions
}

func parseUsageRecord(raw interface{}) (usageRecord, bool) {
	usage, ok := raw.(map[string]interface{})
	if !ok {
		return usageRecord{}, false
	}

	result := usageRecord{
		Input:       toInt64Value(usage["input"]),
		Output:      toInt64Value(usage["output"]),
		CacheRead:   toInt64Value(usage["cacheRead"]),
		CacheWrite:  toInt64Value(usage["cacheWrite"]),
		TotalTokens: toInt64Value(usage["totalTokens"]),
	}
	if cost, ok := usage["cost"].(map[string]interface{}); ok {
		result.TotalCost = toFloat64Value(cost["total"])
	}
	if result.TotalTokens <= 0 && result.Input <= 0 && result.Output <= 0 && result.CacheRead <= 0 && result.CacheWrite <= 0 && result.TotalCost <= 0 {
		return usageRecord{}, false
	}
	return result, true
}

func extractUsageTimestamp(entry, msg map[string]interface{}) time.Time {
	if ts, _ := entry["timestamp"].(string); ts != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			return parsed
		}
	}
	ms := toInt64Value(msg["timestamp"])
	if ms > 0 {
		return time.UnixMilli(ms)
	}
	return time.Time{}
}

func toInt64Value(raw interface{}) int64 {
	switch v := raw.(type) {
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	}
	return 0
}

func toFloat64Value(raw interface{}) float64 {
	switch v := raw.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	}
	return 0
}

func validateAgentQuery(cfg *config.Config, agentID string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil
	}
	_, agentSet := loadAgentIDs(cfg)
	if _, ok := agentSet[agentID]; ok {
		return nil
	}
	return fmt.Errorf("agent 不存在: %s", agentID)
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// formatTime formats a Unix millisecond timestamp
func formatSessionTime(ms int64) string {
	return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
}
