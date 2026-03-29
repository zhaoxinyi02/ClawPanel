package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/model"
	ws "github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

func wechatBridgeAuthorized(cfg *config.Config, c *gin.Context) bool {
	expected := strings.TrimSpace(toString(loadWechatConfigMap(cfg)["bridgeToken"]))
	if expected == "" {
		expected = defaultWechatBridgeToken
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		auth = strings.TrimSpace(auth[7:])
	}
	if auth == "" {
		auth = strings.TrimSpace(c.GetHeader("X-Wechat-Bridge-Token"))
	}
	return auth != "" && auth == expected
}

func wechatBridgeString(raw map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(toString(raw[key])); value != "" {
			return value
		}
	}
	return ""
}

func wechatBridgeBool(raw map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		switch v := raw[key].(type) {
		case bool:
			return v
		case float64:
			return v != 0
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "1", "true", "yes":
				return true
			case "0", "false", "no":
				return false
			}
		}
	}
	return false
}

func normalizeWechatInboundPayload(raw map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{
		"event":   strings.TrimSpace(wechatBridgeString(raw, "event", "type")),
		"talker":  strings.TrimSpace(wechatBridgeString(raw, "talker", "roomId", "roomid", "strTalker", "from")),
		"sender":  strings.TrimSpace(wechatBridgeString(raw, "sender", "senderWxid", "fromWxid", "from")),
		"content": strings.TrimSpace(wechatBridgeString(raw, "content", "strContent", "text")),
		"isRoom":  wechatBridgeBool(raw, "isRoom"),
		"isSelf":  wechatBridgeBool(raw, "isSelf", "isSender", "self"),
	}
	if payload["event"] == "" {
		payload["event"] = "message"
	}
	talker := toString(payload["talker"])
	if talker == "" {
		talker = wechatBridgeString(raw, "conversationId")
		payload["talker"] = talker
	}
	if talker != "" && strings.HasSuffix(talker, "@chatroom") {
		payload["isRoom"] = true
	}
	sender := toString(payload["sender"])
	if sender == "" {
		if talker != "" && !wechatBridgeBool(payload, "isRoom") {
			sender = talker
		} else {
			sender = wechatBridgeString(raw, "fromUser", "wxid")
		}
		payload["sender"] = sender
	}
	content := toString(payload["content"])
	if wechatBridgeBool(payload, "isRoom") {
		if idx := strings.Index(content, ":\n"); idx > 0 {
			if sender == "" {
				payload["sender"] = strings.TrimSpace(content[:idx])
			}
			payload["content"] = strings.TrimSpace(content[idx+2:])
		}
	}
	payload["raw"] = raw
	return payload
}

func appendWechatEvent(db *sql.DB, hub *ws.Hub, source, eventType, summary, detail string) {
	e := &model.Event{
		Time:    time.Now().UnixMilli(),
		Source:  source,
		Type:    eventType,
		Summary: summary,
		Detail:  detail,
	}
	id, err := model.AddEvent(db, e)
	if err != nil {
		return
	}
	if hub == nil {
		return
	}
	entry := map[string]interface{}{
		"id":      id,
		"time":    e.Time,
		"source":  e.Source,
		"type":    e.Type,
		"summary": e.Summary,
		"detail":  e.Detail,
	}
	if payload, err := json.Marshal(map[string]interface{}{"type": "log-entry", "data": entry}); err == nil {
		hub.Broadcast(payload)
	}
}

func WechatBridgeCallback(db *sql.DB, hub *ws.Hub, rt *workflowRuntime, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !wechatBridgeAuthorized(cfg, c) {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "unauthorized"})
			return
		}
		var raw map[string]interface{}
		if err := c.ShouldBindJSON(&raw); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		payload := normalizeWechatInboundPayload(raw)
		eventType := strings.TrimSpace(toString(payload["event"]))
		if eventType != "message" {
			c.JSON(http.StatusOK, gin.H{"ok": true, "ignored": true, "reason": "non_message"})
			return
		}
		talker := strings.TrimSpace(toString(payload["talker"]))
		sender := strings.TrimSpace(toString(payload["sender"]))
		content := strings.TrimSpace(toString(payload["content"]))
		isRoom := wechatBridgeBool(payload, "isRoom")
		isSelf := wechatBridgeBool(payload, "isSelf")

		scope := "私聊"
		if isRoom {
			scope = "群聊"
		}
		if content == "" {
			content = "[非文本消息]"
		}
		summary := "微信" + scope + "消息"
		if sender != "" {
			summary += " · " + sender
		}
		appendWechatEvent(db, hub, "wechat", "message", summary, content)

		if !isSelf && rt != nil && strings.TrimSpace(content) != "" {
			conversationID := talker
			if conversationID == "" {
				conversationID = sender
			}
			extra := map[string]interface{}{
				"scope":    scope,
				"isRoom":   isRoom,
				"talker":   talker,
				"sender":   sender,
				"provider": "wechat",
			}
			handled, reply, reason := rt.interceptInboundMessage("wechat", conversationID, sender, content, extra)
			c.JSON(http.StatusOK, gin.H{
				"ok":      true,
				"handled": handled,
				"reply":   reply,
				"reason":  reason,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "handled": false})
	}
}
