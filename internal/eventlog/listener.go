package eventlog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/zhaoxinyi02/ClawPanel/internal/model"
	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

// Listener monitors OneBot11 WebSocket for message events and records them
type Listener struct {
	db             *sql.DB
	hub            *websocket.Hub
	wsURL          string
	token          string
	tokenProvider  func() string
	inboundHandler func(channelID, conversationID, userID, text string)
	conn           *gorilla.Conn
	mu             sync.Mutex
	stopCh         chan struct{}
	running        bool
	sysLog         *SystemLogger
}

// NewListener creates a new event listener
func NewListener(db *sql.DB, hub *websocket.Hub, wsURL string, tokenProvider func() string) *Listener {
	l := &Listener{
		db:            db,
		hub:           hub,
		wsURL:         wsURL,
		tokenProvider: tokenProvider,
		stopCh:        make(chan struct{}),
		sysLog:        NewSystemLogger(db, hub),
	}
	return l
}

// Start begins listening for OneBot11 events
func (l *Listener) Start() {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return
	}
	// Recreate stopCh in case Stop() was called before (channel is closed after Stop)
	l.stopCh = make(chan struct{})
	l.running = true
	l.mu.Unlock()

	go l.connectLoop()
}

// Stop stops the listener
func (l *Listener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		return
	}
	l.running = false
	close(l.stopCh)
	if l.conn != nil {
		l.conn.Close()
	}
}

func (l *Listener) connectLoop() {
	for {
		select {
		case <-l.stopCh:
			return
		default:
		}

		err := l.connect()
		if err != nil {
			log.Printf("[EventLog] OneBot11 连接失败: %v, 10秒后重试", err)
			select {
			case <-l.stopCh:
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		l.listen()
		log.Println("[EventLog] OneBot11 连接断开, 5秒后重连")
		l.sysLog.Log("system", "napcat.disconnected", "NapCat OneBot11 WebSocket 连接断开")
		select {
		case <-l.stopCh:
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (l *Listener) connect() error {
	dialer := gorilla.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	token := l.currentToken()
	var header map[string][]string
	if token != "" {
		header = map[string][]string{
			"Authorization": {"Bearer " + token},
		}
	}
	conn, _, err := dialer.Dial(l.wsURL, header)
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.conn = conn
	l.mu.Unlock()
	log.Printf("[EventLog] 已连接 OneBot11 WebSocket: %s", l.wsURL)
	l.sysLog.Log("system", "napcat.connected", "NapCat OneBot11 WebSocket 已连接")
	return nil
}

func (l *Listener) currentToken() string {
	if l.tokenProvider != nil {
		return strings.TrimSpace(l.tokenProvider())
	}
	return strings.TrimSpace(l.token)
}

func (l *Listener) SetInboundHandler(handler func(channelID, conversationID, userID, text string)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inboundHandler = handler
}

func (l *Listener) listen() {
	defer func() {
		l.mu.Lock()
		if l.conn != nil {
			l.conn.Close()
			l.conn = nil
		}
		l.mu.Unlock()
	}()

	for {
		select {
		case <-l.stopCh:
			return
		default:
		}

		_, msg, err := l.conn.ReadMessage()
		if err != nil {
			return
		}

		l.processMessage(msg)
	}
}

func (l *Listener) processMessage(raw []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	postType, _ := msg["post_type"].(string)
	if postType == "" {
		return
	}

	// Skip heartbeat events
	if postType == "meta_event" {
		return
	}

	var event *model.Event

	switch postType {
	case "message", "message_sent":
		l.handleInboundMessage(msg)
		event = l.parseMessageEvent(msg)
	case "notice":
		event = l.parseNoticeEvent(msg)
	case "request":
		event = l.parseRequestEvent(msg)
	}

	if event == nil {
		return
	}

	// Save to DB
	id, err := model.AddEvent(l.db, event)
	if err != nil {
		log.Printf("[EventLog] 保存事件失败: %v", err)
		return
	}
	event.ID = id

	// Broadcast to WebSocket clients as log-entry
	entry := map[string]interface{}{
		"id":      event.ID,
		"time":    event.Time,
		"source":  event.Source,
		"type":    event.Type,
		"summary": event.Summary,
		"detail":  event.Detail,
	}
	wsMsg, _ := json.Marshal(map[string]interface{}{
		"type": "log-entry",
		"data": entry,
	})
	l.hub.Broadcast(wsMsg)
}

func (l *Listener) handleInboundMessage(msg map[string]interface{}) {
	selfID := normalizeOneBotID(msg["self_id"])
	userID := normalizeOneBotID(msg["user_id"])
	if selfID == "" || userID == "" || selfID == userID {
		return
	}
	messageType, _ := msg["message_type"].(string)
	conversationID := ""
	switch messageType {
	case "group":
		groupID := normalizeOneBotID(msg["group_id"])
		if groupID != "" {
			conversationID = "qq:group:" + groupID
		}
	default:
		conversationID = "qq:private:" + userID
	}
	text := extractRawMessageText(msg)
	if conversationID == "" || strings.TrimSpace(text) == "" {
		return
	}
	l.mu.Lock()
	handler := l.inboundHandler
	l.mu.Unlock()
	if handler != nil {
		go handler("qq", conversationID, userID, text)
	}
}

func normalizeOneBotID(v interface{}) string {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return ""
		}
		return fmt.Sprintf("%.0f", n)
	case float32:
		if math.IsNaN(float64(n)) || math.IsInf(float64(n), 0) {
			return ""
		}
		return fmt.Sprintf("%.0f", n)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", n)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", n)
	case string:
		return strings.TrimSpace(n)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func extractRawMessageText(msg map[string]interface{}) string {
	rawMsg, _ := msg["raw_message"].(string)
	if rawMsg != "" {
		return strings.TrimSpace(rawMsg)
	}
	if msgArr, ok := msg["message"].([]interface{}); ok {
		var parts []string
		for _, m := range msgArr {
			mm, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			t, _ := mm["type"].(string)
			switch t {
			case "text":
				if d, ok := mm["data"].(map[string]interface{}); ok {
					if text, _ := d["text"].(string); text != "" {
						parts = append(parts, text)
					}
				}
			case "image":
				parts = append(parts, "[图片]")
			case "face":
				parts = append(parts, "[表情]")
			case "at":
				parts = append(parts, "[At]")
			default:
				parts = append(parts, fmt.Sprintf("[%s]", t))
			}
		}
		return strings.TrimSpace(strings.Join(parts, ""))
	}
	return ""
}

func (l *Listener) parseMessageEvent(msg map[string]interface{}) *model.Event {
	msgType, _ := msg["message_type"].(string)
	rawMsg := extractRawMessageText(msg)

	selfID := fmt.Sprintf("%v", msg["self_id"])
	userID := fmt.Sprintf("%v", msg["user_id"])
	sender := ""
	if s, ok := msg["sender"].(map[string]interface{}); ok {
		nickname, _ := s["nickname"].(string)
		card, _ := s["card"].(string)
		if card != "" {
			sender = card
		} else if nickname != "" {
			sender = nickname
		}
	}

	source := "qq"
	isSelf := userID == selfID

	var summary string
	var eventType string

	if msgType == "group" {
		groupID := fmt.Sprintf("%v", msg["group_id"])
		if isSelf {
			source = "openclaw"
			eventType = "message.group.sent"
			summary = fmt.Sprintf("[群%s] → %s", groupID, truncate(rawMsg, 100))
		} else {
			eventType = "message.group.received"
			if sender != "" {
				summary = fmt.Sprintf("[群%s] %s: %s", groupID, sender, truncate(rawMsg, 80))
			} else {
				summary = fmt.Sprintf("[群%s] %s: %s", groupID, userID, truncate(rawMsg, 80))
			}
		}
	} else {
		if isSelf {
			source = "openclaw"
			eventType = "message.private.sent"
			summary = fmt.Sprintf("[私聊] → %s", truncate(rawMsg, 100))
		} else {
			eventType = "message.private.received"
			if sender != "" {
				summary = fmt.Sprintf("[私聊] %s: %s", sender, truncate(rawMsg, 80))
			} else {
				summary = fmt.Sprintf("[私聊] %s: %s", userID, truncate(rawMsg, 80))
			}
		}
	}

	return &model.Event{
		Time:    time.Now().UnixMilli(),
		Source:  source,
		Type:    eventType,
		Summary: summary,
		Detail:  rawMsg,
	}
}

func (l *Listener) parseNoticeEvent(msg map[string]interface{}) *model.Event {
	noticeType, _ := msg["notice_type"].(string)
	if noticeType == "notify" {
		return nil
	}
	summary := ""

	switch noticeType {
	case "group_increase":
		userID := fmt.Sprintf("%v", msg["user_id"])
		groupID := fmt.Sprintf("%v", msg["group_id"])
		summary = fmt.Sprintf("用户 %s 加入群 %s", userID, groupID)
	case "group_decrease":
		userID := fmt.Sprintf("%v", msg["user_id"])
		groupID := fmt.Sprintf("%v", msg["group_id"])
		summary = fmt.Sprintf("用户 %s 离开群 %s", userID, groupID)
	case "friend_add":
		userID := fmt.Sprintf("%v", msg["user_id"])
		summary = fmt.Sprintf("新好友: %s", userID)
	case "group_recall":
		summary = "群消息撤回"
	case "friend_recall":
		summary = "好友消息撤回"
	case "poke":
		summary = "戳一戳"
	default:
		summary = fmt.Sprintf("通知: %s", noticeType)
	}

	return &model.Event{
		Time:    time.Now().UnixMilli(),
		Source:  "qq",
		Type:    "notice." + noticeType,
		Summary: summary,
	}
}

func (l *Listener) parseRequestEvent(msg map[string]interface{}) *model.Event {
	reqType, _ := msg["request_type"].(string)
	comment, _ := msg["comment"].(string)
	userID := fmt.Sprintf("%v", msg["user_id"])

	summary := ""
	switch reqType {
	case "friend":
		summary = fmt.Sprintf("好友请求: %s", userID)
		if comment != "" {
			summary += fmt.Sprintf(" (%s)", comment)
		}
	case "group":
		groupID := fmt.Sprintf("%v", msg["group_id"])
		summary = fmt.Sprintf("入群请求: %s → 群%s", userID, groupID)
		if comment != "" {
			summary += fmt.Sprintf(" (%s)", comment)
		}
	default:
		summary = fmt.Sprintf("请求: %s from %s", reqType, userID)
	}

	return &model.Event{
		Time:    time.Now().UnixMilli(),
		Source:  "qq",
		Type:    "request." + reqType,
		Summary: summary,
		Detail:  comment,
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
