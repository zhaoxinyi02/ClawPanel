package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/model"
)

func newPanelChatTestConfig(t *testing.T) *config.Config {
	t.Helper()

	root, _ := filepath.EvalSymlinks(t.TempDir())
	cfg := &config.Config{
		DataDir:      filepath.Join(root, "data"),
		OpenClawDir:  filepath.Join(root, ".openclaw"),
		OpenClawApp:  filepath.Join(root, "openclaw-app"),
		OpenClawWork: filepath.Join(root, "work"),
		Edition:      "pro",
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll data dir failed: %v", err)
	}
	if err := os.MkdirAll(cfg.OpenClawDir, 0o755); err != nil {
		t.Fatalf("MkdirAll openclaw dir failed: %v", err)
	}
	if err := os.MkdirAll(cfg.OpenClawApp, 0o755); err != nil {
		t.Fatalf("MkdirAll openclaw app dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.OpenClawApp, "package.json"), []byte(`{"name":"openclaw"}`), 0o644); err != nil {
		t.Fatalf("write package.json failed: %v", err)
	}
	if err := cfg.WriteOpenClawJSON(map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main", "default": true},
				map[string]interface{}{"id": "writer"},
				map[string]interface{}{"id": "reviewer"},
			},
		},
	}); err != nil {
		t.Fatalf("WriteOpenClawJSON failed: %v", err)
	}
	return cfg
}

func newPanelChatTestDB(t *testing.T, cfg *config.Config) *sql.DB {
	t.Helper()
	db, err := model.InitDB(cfg.DataDir)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func performPanelChatJSONRequest(t *testing.T, handler gin.HandlerFunc, method, target string, body any, params map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body failed: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	for key, value := range params {
		ctx.Params = append(ctx.Params, gin.Param{Key: key, Value: value})
	}
	handler(ctx)
	return w
}

func decodeBodyMap(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v; body=%s", err, w.Body.String())
	}
	return payload
}

func writePanelChatSessionFixture(t *testing.T, cfg *config.Config, db *sql.DB, session panelChatSession, participantIDs ...string) {
	t.Helper()
	if err := model.ReplacePanelChatParticipants(db, session.ID, normalizePanelChatParticipantInput(participantIDs, session.AgentID, session.SummaryAgentID)); err != nil {
		t.Fatalf("ReplacePanelChatParticipants failed: %v", err)
	}
	if err := savePanelChatSessions(cfg, []panelChatSession{session}); err != nil {
		t.Fatalf("savePanelChatSessions failed: %v", err)
	}
}

func TestCreatePanelChatSessionRejectsUnknownSummaryAgent(t *testing.T) {
	cfg := newPanelChatTestConfig(t)
	db := newPanelChatTestDB(t, cfg)

	w := performPanelChatJSONRequest(t, CreatePanelChatSession(db, cfg), http.MethodPost, "/panel-chat/sessions", map[string]any{
		"chatType":       "group",
		"agentId":        "main",
		"agentIds":       []string{"main", "writer"},
		"summaryAgentId": "ghost",
	}, nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	body := decodeBodyMap(t, w)
	if !strings.Contains(body["error"].(string), "agent ghost 不存在") {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestCreatePanelChatSessionRejectsSummaryOutsideParticipants(t *testing.T) {
	cfg := newPanelChatTestConfig(t)
	db := newPanelChatTestDB(t, cfg)

	w := performPanelChatJSONRequest(t, CreatePanelChatSession(db, cfg), http.MethodPost, "/panel-chat/sessions", map[string]any{
		"chatType":       "group",
		"agentId":        "main",
		"agentIds":       []string{"main", "writer"},
		"summaryAgentId": "reviewer",
	}, nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	body := decodeBodyMap(t, w)
	if body["error"] != "summaryAgentId 必须包含在 agentIds 中" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestRenamePanelChatSessionRejectsBusySession(t *testing.T) {
	cfg := newPanelChatTestConfig(t)
	sessionID := "panel-busy-rename"
	writePanelChatSessionFixture(t, cfg, newPanelChatTestDB(t, cfg), panelChatSession{
		ID:                sessionID,
		OpenClawSessionID: sessionID,
		AgentID:           "main",
		ChatType:          "direct",
		Title:             "before",
		CreatedAt:         time.Now().UnixMilli(),
		UpdatedAt:         time.Now().UnixMilli(),
	}, "main")

	panelChatSessionBusy.Store(sessionID, struct{}{})
	defer panelChatSessionBusy.Delete(sessionID)

	w := performPanelChatJSONRequest(t, RenamePanelChatSession(cfg), http.MethodPut, "/panel-chat/sessions/"+sessionID, map[string]any{
		"title": "after",
	}, map[string]string{"id": sessionID})

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
	body := decodeBodyMap(t, w)
	if body["error"] != "当前会话正在处理中，无法重命名" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestDeletePanelChatSessionRejectsBusySession(t *testing.T) {
	cfg := newPanelChatTestConfig(t)
	db := newPanelChatTestDB(t, cfg)
	sessionID := "panel-busy-delete"
	writePanelChatSessionFixture(t, cfg, db, panelChatSession{
		ID:                sessionID,
		OpenClawSessionID: sessionID,
		AgentID:           "main",
		ChatType:          "direct",
		Title:             "delete me",
		CreatedAt:         time.Now().UnixMilli(),
		UpdatedAt:         time.Now().UnixMilli(),
	}, "main")

	panelChatSessionBusy.Store(sessionID, struct{}{})
	defer panelChatSessionBusy.Delete(sessionID)

	w := performPanelChatJSONRequest(t, DeletePanelChatSession(db, cfg), http.MethodDelete, "/panel-chat/sessions/"+sessionID, nil, map[string]string{"id": sessionID})

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
	body := decodeBodyMap(t, w)
	if body["error"] != "当前会话正在处理中，无法删除" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestSendPanelChatMessageRejectsBusySession(t *testing.T) {
	cfg := newPanelChatTestConfig(t)
	db := newPanelChatTestDB(t, cfg)
	sessionID := "panel-busy-send"
	writePanelChatSessionFixture(t, cfg, db, panelChatSession{
		ID:                sessionID,
		OpenClawSessionID: sessionID,
		AgentID:           "main",
		ChatType:          "direct",
		Title:             "send",
		CreatedAt:         time.Now().UnixMilli(),
		UpdatedAt:         time.Now().UnixMilli(),
	}, "main")

	panelChatSessionBusy.Store(sessionID, struct{}{})
	defer panelChatSessionBusy.Delete(sessionID)

	w := performPanelChatJSONRequest(t, SendPanelChatMessage(db, cfg), http.MethodPost, "/panel-chat/sessions/"+sessionID+"/messages", map[string]any{
		"message": "hello",
	}, map[string]string{"id": sessionID})

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
	body := decodeBodyMap(t, w)
	if body["error"] != "当前会话正在处理中，请稍后重试或先取消当前请求" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestRewritePanelChatRuntimeConfigSynthesizesImplicitAgent(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		DataDir:     filepath.Join(root, "data"),
		OpenClawDir: filepath.Join(root, ".openclaw"),
		Edition:     "pro",
	}
	if err := os.MkdirAll(filepath.Join(cfg.OpenClawDir, "agents", "main", "agent"), 0o755); err != nil {
		t.Fatalf("mkdir agent dir failed: %v", err)
	}
	src := filepath.Join(root, "src-openclaw.json")
	dst := filepath.Join(root, "dst-openclaw.json")
	if err := os.WriteFile(src, []byte(`{"agents":{"defaults":{"workspace":"/tmp/work"}},"models":{"providers":{}}}`), 0o644); err != nil {
		t.Fatalf("write src config failed: %v", err)
	}
	session := panelChatSession{ID: "panel-1", AgentID: "main"}
	if err := rewritePanelChatRuntimeConfig(cfg, src, dst, session); err != nil {
		t.Fatalf("rewritePanelChatRuntimeConfig failed: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst config failed: %v", err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal dst config failed: %v", err)
	}
	agents := obj["agents"].(map[string]interface{})
	list := agents["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected 1 synthesized agent, got %#v", list)
	}
	agent := list[0].(map[string]interface{})
	if got := agent["id"]; got != panelChatScopedAgentID(session.ID, session.AgentID) {
		t.Fatalf("unexpected synthesized id: %#v", got)
	}
}
