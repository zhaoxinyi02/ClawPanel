package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestGetSessionsAgentAllAggregates(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	oc := map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main"},
				map[string]interface{}{"id": "work"},
			},
		},
	}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), oc)

	mainSessionFile := filepath.Join(dir, "agents", "main", "sessions", "s_main.jsonl")
	workSessionFile := filepath.Join(dir, "agents", "work", "sessions", "s_work.jsonl")
	if err := os.MkdirAll(filepath.Dir(mainSessionFile), 0755); err != nil {
		t.Fatalf("mkdir main sessions dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(workSessionFile), 0755); err != nil {
		t.Fatalf("mkdir work sessions dir: %v", err)
	}
	_ = os.WriteFile(mainSessionFile, []byte(`{"type":"message","id":"1","message":{"role":"user","content":"hello"}}`+"\n"), 0644)
	_ = os.WriteFile(workSessionFile, []byte(`{"type":"assistant","id":"2","message":{"role":"assistant","content":"world"}}`+"\n"), 0644)

	writeJSON(t, filepath.Join(dir, "agents", "main", "sessions", "sessions.json"), map[string]interface{}{
		"main-key": map[string]interface{}{
			"sessionId":   "session-main",
			"chatType":    "direct",
			"updatedAt":   float64(1000),
			"sessionFile": mainSessionFile,
		},
	})
	writeJSON(t, filepath.Join(dir, "agents", "work", "sessions", "sessions.json"), map[string]interface{}{
		"work-key": map[string]interface{}{
			"sessionId":   "session-work",
			"chatType":    "group",
			"updatedAt":   float64(2000),
			"sessionFile": workSessionFile,
		},
	})

	r := gin.New()
	r.GET("/sessions", GetSessions(cfg))
	req := httptest.NewRequest(http.MethodGet, "/sessions?agent=all", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK       bool          `json:"ok"`
		Sessions []SessionInfo `json:"sessions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true")
	}
	if len(resp.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(resp.Sessions))
	}
	if resp.Sessions[0].AgentID != "work" || resp.Sessions[0].SessionID != "session-work" {
		t.Fatalf("sessions should be aggregated and sorted by updatedAt desc")
	}
}

func TestPreviewRouteRespectsBindingOrder(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	oc := map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main"},
				map[string]interface{}{"id": "work"},
			},
		},
		"bindings": []interface{}{
			map[string]interface{}{
				"name":    "work-group",
				"enabled": true,
				"agentId": "work",
				"match": map[string]interface{}{
					"channel": "qq",
					"peer":    "group:*",
				},
			},
			map[string]interface{}{
				"name":    "fallback-qq",
				"enabled": true,
				"agentId": "main",
				"match": map[string]interface{}{
					"channel": "qq",
				},
			},
		},
	}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), oc)

	r := gin.New()
	r.POST("/route/preview", PreviewOpenClawRoute(cfg))

	body := []byte(`{"meta":{"channel":"qq","peer":"group:123"}}`)
	req := httptest.NewRequest(http.MethodPost, "/route/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK     bool `json:"ok"`
		Result struct {
			Agent     string   `json:"agent"`
			MatchedBy string   `json:"matchedBy"`
			Trace     []string `json:"trace"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true")
	}
	if resp.Result.Agent != "work" {
		t.Fatalf("expected first binding to match work, got %s", resp.Result.Agent)
	}
	if !strings.HasPrefix(resp.Result.MatchedBy, "bindings[0]") {
		t.Fatalf("expected matchedBy to reference first binding, got %s", resp.Result.MatchedBy)
	}
}

func TestSaveCronJobsRejectsUnknownSessionTarget(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSON(t, filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"default": "main",
			"list": []interface{}{
				map[string]interface{}{"id": "main"},
			},
		},
	})

	r := gin.New()
	r.PUT("/system/cron", SaveCronJobs(cfg))

	body := []byte(`{"jobs":[{"id":"job_1","name":"bad","enabled":true,"schedule":{"kind":"cron","expr":"0 9 * * *"},"sessionTarget":"work","wakeMode":"now","payload":{"kind":"agentTurn","message":"hi"},"state":{},"createdAtMs":1}]}`)
	req := httptest.NewRequest(http.MethodPut, "/system/cron", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sessionTarget, got %d: %s", w.Code, w.Body.String())
	}
}

func writeJSON(t *testing.T, path string, data any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
