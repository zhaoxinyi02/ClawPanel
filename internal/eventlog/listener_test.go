package eventlog

import (
	"database/sql"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/zhaoxinyi02/ClawPanel/internal/model"
	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

type inboundCall struct {
	channelID      string
	conversationID string
	userID         string
	text           string
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := model.InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func waitForInboundCall(t *testing.T, ch <-chan inboundCall) inboundCall {
	t.Helper()

	select {
	case call := <-ch:
		return call
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for inbound handler")
		return inboundCall{}
	}
}

func TestListenerCurrentTokenUsesProvider(t *testing.T) {
	t.Parallel()

	token := "first-token"
	listener := NewListener(nil, nil, "ws://example", func() string {
		return token
	})

	if got := listener.currentToken(); got != "first-token" {
		t.Fatalf("expected first token, got %q", got)
	}

	token = "second-token"
	if got := listener.currentToken(); got != "second-token" {
		t.Fatalf("expected refreshed token, got %q", got)
	}
}

func TestListenerCurrentTokenFallsBackToStaticToken(t *testing.T) {
	t.Parallel()

	listener := &Listener{token: "  static-token  "}
	if got := listener.currentToken(); got != "static-token" {
		t.Fatalf("expected static token fallback, got %q", got)
	}
}

func TestNormalizeOneBotID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{name: "float64", input: float64(12345), want: "12345"},
		{name: "nan", input: math.NaN(), want: ""},
		{name: "float32", input: float32(42), want: "42"},
		{name: "float32 nan", input: float32(math.NaN()), want: ""},
		{name: "float32 inf", input: float32(math.Inf(1)), want: ""},
		{name: "int64", input: int64(-9), want: "-9"},
		{name: "uint64", input: uint64(77), want: "77"},
		{name: "trimmed string", input: "  user-1  ", want: "user-1"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeOneBotID(tt.input); got != tt.want {
				t.Fatalf("normalizeOneBotID(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractRawMessageText(t *testing.T) {
	t.Parallel()

	t.Run("prefers raw message", func(t *testing.T) {
		t.Parallel()

		msg := map[string]interface{}{
			"raw_message": "  hello world  ",
			"message": []interface{}{
				map[string]interface{}{
					"type": "text",
					"data": map[string]interface{}{"text": "ignored"},
				},
			},
		}

		if got := extractRawMessageText(msg); got != "hello world" {
			t.Fatalf("extractRawMessageText() = %q, want hello world", got)
		}
	})

	t.Run("joins segmented message parts", func(t *testing.T) {
		t.Parallel()

		msg := map[string]interface{}{
			"message": []interface{}{
				map[string]interface{}{
					"type": "text",
					"data": map[string]interface{}{"text": "hello"},
				},
				map[string]interface{}{"type": "image"},
				map[string]interface{}{"type": "face"},
				map[string]interface{}{"type": "at"},
				map[string]interface{}{"type": "record"},
				"ignored",
			},
		}

		if got := extractRawMessageText(msg); got != "hello[图片][表情][At][record]" {
			t.Fatalf("extractRawMessageText() = %q", got)
		}
	})
}

func TestHandleInboundMessageDispatchesConversationKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  map[string]interface{}
		want inboundCall
	}{
		{
			name: "group conversation",
			msg: map[string]interface{}{
				"self_id":      "1000",
				"user_id":      "2000",
				"group_id":     "3000",
				"message_type": "group",
				"raw_message":  "  hello group  ",
			},
			want: inboundCall{
				channelID:      "qq",
				conversationID: "qq:group:3000",
				userID:         "2000",
				text:           "hello group",
			},
		},
		{
			name: "private conversation",
			msg: map[string]interface{}{
				"self_id":      "1000",
				"user_id":      "2000",
				"message_type": "private",
				"message": []interface{}{
					map[string]interface{}{
						"type": "text",
						"data": map[string]interface{}{"text": "hello private"},
					},
				},
			},
			want: inboundCall{
				channelID:      "qq",
				conversationID: "qq:private:2000",
				userID:         "2000",
				text:           "hello private",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ch := make(chan inboundCall, 1)
			listener := &Listener{}
			listener.SetInboundHandler(func(channelID, conversationID, userID, text string) {
				ch <- inboundCall{
					channelID:      channelID,
					conversationID: conversationID,
					userID:         userID,
					text:           text,
				}
			})

			listener.handleInboundMessage(tt.msg)

			if got := waitForInboundCall(t, ch); got != tt.want {
				t.Fatalf("inbound call = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHandleInboundMessageIgnoresSelfMessagesAndBlankText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  map[string]interface{}
	}{
		{
			name: "self message",
			msg: map[string]interface{}{
				"self_id":      "1000",
				"user_id":      "1000",
				"message_type": "private",
				"raw_message":  "hello",
			},
		},
		{
			name: "blank text",
			msg: map[string]interface{}{
				"self_id":      "1000",
				"user_id":      "2000",
				"message_type": "private",
				"raw_message":  "   ",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ch := make(chan inboundCall, 1)
			listener := &Listener{}
			listener.SetInboundHandler(func(channelID, conversationID, userID, text string) {
				ch <- inboundCall{
					channelID:      channelID,
					conversationID: conversationID,
					userID:         userID,
					text:           text,
				}
			})

			listener.handleInboundMessage(tt.msg)

			select {
			case got := <-ch:
				t.Fatalf("unexpected inbound call: %+v", got)
			case <-time.After(200 * time.Millisecond):
			}
		})
	}
}

func TestParseMessageEvent(t *testing.T) {
	t.Parallel()

	listener := &Listener{}

	groupEvent := listener.parseMessageEvent(map[string]interface{}{
		"message_type": "group",
		"self_id":      "1000",
		"user_id":      "2000",
		"group_id":     "3000",
		"raw_message":  "  hello group  ",
		"sender": map[string]interface{}{
			"card": "Alice",
		},
	})
	if groupEvent == nil {
		t.Fatal("parseMessageEvent(group) returned nil")
	}
	if groupEvent.Source != "qq" || groupEvent.Type != "message.group.received" {
		t.Fatalf("unexpected group event identity: %+v", groupEvent)
	}
	if groupEvent.Summary != "[群3000] Alice: hello group" || groupEvent.Detail != "hello group" {
		t.Fatalf("unexpected group event payload: %+v", groupEvent)
	}
	if groupEvent.Time == 0 {
		t.Fatal("group event should have timestamp")
	}

	longMessage := strings.Repeat("x", 101)
	privateSelfEvent := listener.parseMessageEvent(map[string]interface{}{
		"message_type": "private",
		"self_id":      "1000",
		"user_id":      "1000",
		"raw_message":  longMessage,
	})
	if privateSelfEvent == nil {
		t.Fatal("parseMessageEvent(private self) returned nil")
	}
	if privateSelfEvent.Source != "openclaw" || privateSelfEvent.Type != "message.private.sent" {
		t.Fatalf("unexpected private self event identity: %+v", privateSelfEvent)
	}
	if privateSelfEvent.Summary != "[私聊] → "+strings.Repeat("x", 100)+"..." {
		t.Fatalf("unexpected private self summary: %q", privateSelfEvent.Summary)
	}
	if privateSelfEvent.Detail != longMessage {
		t.Fatalf("private self detail = %q, want %q", privateSelfEvent.Detail, longMessage)
	}
}

func TestParseNoticeEvent(t *testing.T) {
	t.Parallel()

	listener := &Listener{}

	if event := listener.parseNoticeEvent(map[string]interface{}{"notice_type": "notify"}); event != nil {
		t.Fatalf("parseNoticeEvent(notify) = %+v, want nil", event)
	}

	event := listener.parseNoticeEvent(map[string]interface{}{
		"notice_type": "group_increase",
		"user_id":     "2000",
		"group_id":    "3000",
	})
	if event == nil {
		t.Fatal("parseNoticeEvent(group_increase) returned nil")
	}
	if event.Type != "notice.group_increase" || event.Summary != "用户 2000 加入群 3000" {
		t.Fatalf("unexpected group increase event: %+v", event)
	}

	event = listener.parseNoticeEvent(map[string]interface{}{"notice_type": "custom"})
	if event == nil {
		t.Fatal("parseNoticeEvent(custom) returned nil")
	}
	if event.Type != "notice.custom" || event.Summary != "通知: custom" {
		t.Fatalf("unexpected custom notice event: %+v", event)
	}
}

func TestParseRequestEvent(t *testing.T) {
	t.Parallel()

	listener := &Listener{}

	friendEvent := listener.parseRequestEvent(map[string]interface{}{
		"request_type": "friend",
		"user_id":      "2000",
		"comment":      "hi there",
	})
	if friendEvent == nil {
		t.Fatal("parseRequestEvent(friend) returned nil")
	}
	if friendEvent.Type != "request.friend" || friendEvent.Summary != "好友请求: 2000 (hi there)" || friendEvent.Detail != "hi there" {
		t.Fatalf("unexpected friend request event: %+v", friendEvent)
	}

	groupEvent := listener.parseRequestEvent(map[string]interface{}{
		"request_type": "group",
		"user_id":      "2000",
		"group_id":     "3000",
	})
	if groupEvent == nil {
		t.Fatal("parseRequestEvent(group) returned nil")
	}
	if groupEvent.Type != "request.group" || groupEvent.Summary != "入群请求: 2000 → 群3000" {
		t.Fatalf("unexpected group request event: %+v", groupEvent)
	}

	unknownEvent := listener.parseRequestEvent(map[string]interface{}{
		"request_type": "custom",
		"user_id":      "2000",
	})
	if unknownEvent == nil {
		t.Fatal("parseRequestEvent(custom) returned nil")
	}
	if unknownEvent.Type != "request.custom" || unknownEvent.Summary != "请求: custom from 2000" {
		t.Fatalf("unexpected custom request event: %+v", unknownEvent)
	}
}

func TestProcessMessagePersistsEventsAndInvokesInboundHandler(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	listener := NewListener(db, websocket.NewHub(), "ws://example", nil)
	ch := make(chan inboundCall, 1)
	listener.SetInboundHandler(func(channelID, conversationID, userID, text string) {
		ch <- inboundCall{
			channelID:      channelID,
			conversationID: conversationID,
			userID:         userID,
			text:           text,
		}
	})

	payload, err := json.Marshal(map[string]interface{}{
		"post_type":     "message",
		"message_type":  "group",
		"self_id":       "1000",
		"user_id":       "2000",
		"group_id":      "3000",
		"raw_message":   "  hello from process  ",
		"sender":        map[string]interface{}{"nickname": "Bob"},
		"message_id":    "1",
		"sub_type":      "normal",
		"target_id":     "0",
		"sender_id_opt": "ignored",
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	listener.processMessage(payload)

	if got := waitForInboundCall(t, ch); got != (inboundCall{
		channelID:      "qq",
		conversationID: "qq:group:3000",
		userID:         "2000",
		text:           "hello from process",
	}) {
		t.Fatalf("unexpected inbound call: %+v", got)
	}

	events, total, err := model.GetEvents(db, 10, 0, "", "")
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if total != 1 || len(events) != 1 {
		t.Fatalf("expected 1 persisted event, total=%d events=%+v", total, events)
	}
	if events[0].Type != "message.group.received" || events[0].Summary != "[群3000] Bob: hello from process" || events[0].Detail != "hello from process" {
		t.Fatalf("unexpected persisted event: %+v", events[0])
	}
}

func TestProcessMessageIgnoresInvalidPayloads(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	listener := NewListener(db, websocket.NewHub(), "ws://example", nil)

	listener.processMessage([]byte("{invalid"))

	metaPayload, err := json.Marshal(map[string]interface{}{"post_type": "meta_event"})
	if err != nil {
		t.Fatalf("Marshal(meta) error = %v", err)
	}
	listener.processMessage(metaPayload)

	events, total, err := model.GetEvents(db, 10, 0, "", "")
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if total != 0 || len(events) != 0 {
		t.Fatalf("expected no events for ignored payloads, total=%d events=%+v", total, events)
	}
}

func TestSystemLoggerPersistsEvents(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	logger := NewSystemLogger(db, websocket.NewHub())

	logger.LogDetail("system", "napcat.connected", "NapCat connected", "detail")
	logger.Log("system", "napcat.disconnected", "NapCat disconnected")

	events, total, err := model.GetEvents(db, 10, 0, "system", "")
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if total != 2 || len(events) != 2 {
		t.Fatalf("expected 2 system events, total=%d events=%+v", total, events)
	}

	found := map[string]model.Event{}
	for _, event := range events {
		found[event.Type] = event
	}
	if found["napcat.connected"].Detail != "detail" || found["napcat.connected"].Summary != "NapCat connected" {
		t.Fatalf("unexpected connected event: %+v", found["napcat.connected"])
	}
	if found["napcat.disconnected"].Detail != "" || found["napcat.disconnected"].Summary != "NapCat disconnected" {
		t.Fatalf("unexpected disconnected event: %+v", found["napcat.disconnected"])
	}
}
