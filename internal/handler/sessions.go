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

// SessionInfo represents a session entry from sessions.json
type SessionInfo struct {
	AgentID        string `json:"agentId,omitempty"`
	Key            string `json:"key"`
	SessionID      string `json:"sessionId"`
	ChatType       string `json:"chatType"`
	LastChannel    string `json:"lastChannel"`
	LastTo         string `json:"lastTo"`
	UpdatedAt      int64  `json:"updatedAt"`
	OriginLabel    string `json:"originLabel"`
	OriginProvider string `json:"originProvider"`
	OriginFrom     string `json:"originFrom"`
	SessionFile    string `json:"sessionFile"`
	MessageCount   int    `json:"messageCount"`
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

// MsgContent represents the message content
type MsgContent struct {
	Role      string      `json:"role"`
	Content   interface{} `json:"content"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

// GetSessions returns the list of all sessions
func GetSessions(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := c.DefaultQuery("agent", "main")
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
		agentID := c.DefaultQuery("agent", "main")
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

		sessionsDir := filepath.Join(cfg.OpenClawDir, "agents", agentID, "sessions")
		sessionFile := filepath.Join(sessionsDir, sessionID+".jsonl")

		if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
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
		agentID := c.DefaultQuery("agent", "main")
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

		sessionsPath := filepath.Join(cfg.OpenClawDir, "agents", agentID, "sessions", "sessions.json")

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

func readSessionMessages(filePath string, limit int) ([]map[string]interface{}, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var allMessages []map[string]interface{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

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

func extractMessage(entry map[string]interface{}) map[string]interface{} {
	msg, ok := entry["message"].(map[string]interface{})
	if !ok {
		return nil
	}

	role, _ := msg["role"].(string)
	content := extractTextContent(msg["content"])
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

	content := extractTextContent(msg["content"])
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

func countSessionMessages(filePath string) int {
	f, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if t, _ := entry["type"].(string); t == "message" || t == "assistant" {
			count++
		}
	}
	return count
}

func loadSessionsByAgent(cfg *config.Config, agentID string) []SessionInfo {
	sessionsPath := filepath.Join(cfg.OpenClawDir, "agents", agentID, "sessions", "sessions.json")
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
			si.MessageCount = countSessionMessages(si.SessionFile)
		}
		sessions = append(sessions, si)
	}
	return sessions
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
