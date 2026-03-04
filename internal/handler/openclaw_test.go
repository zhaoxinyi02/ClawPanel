package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestSaveOpenClawConfigPreservesCriticalFields(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	initial := map[string]interface{}{
		"tools": map[string]interface{}{
			"agentToAgent": true,
		},
		"session": map[string]interface{}{
			"maxMessages": 50,
		},
		"cron": map[string]interface{}{
			"jobs": []interface{}{
				map[string]interface{}{"id": "job_1"},
			},
		},
		"models": map[string]interface{}{
			"providers": map[string]interface{}{},
		},
	}
	raw, _ := json.Marshal(initial)
	if err := os.WriteFile(filepath.Join(dir, "openclaw.json"), raw, 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	r := gin.New()
	r.PUT("/openclaw/config", SaveOpenClawConfig(cfg))

	body, _ := json.Marshal(map[string]interface{}{"config": initial})
	req := httptest.NewRequest(http.MethodPut, "/openclaw/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	saved, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if _, ok := saved["tools"]; !ok {
		t.Fatalf("tools should be preserved")
	}
	if _, ok := saved["session"]; !ok {
		t.Fatalf("session should be preserved")
	}
	cron, ok := saved["cron"].(map[string]interface{})
	if !ok {
		t.Fatalf("cron should be preserved")
	}
	if _, ok := cron["jobs"]; !ok {
		t.Fatalf("cron.jobs should be preserved")
	}
}
