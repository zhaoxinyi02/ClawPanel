package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/monitor"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
)

func TestGetStatusReturnsResolvedGatewayPortFromConfig(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	openclawDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(openclawDir, "openclaw.json"), []byte(`{"gateway":{"port":19527}}`), 0644); err != nil {
		t.Fatalf("write openclaw config: %v", err)
	}

	cfg := &config.Config{OpenClawDir: openclawDir, Edition: "pro"}
	procMgr := process.NewManager(cfg)
	napcatMon := monitor.NewNapCatMonitor(cfg, nil, nil)

	r := gin.New()
	r.GET("/status", GetStatus(nil, cfg, procMgr, napcatMon))

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	openclaw, ok := resp["openclaw"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing openclaw payload: %v", resp["openclaw"])
	}

	if got := int(openclaw["gatewayPort"].(float64)); got != 19527 {
		t.Fatalf("expected gatewayPort 19527, got %d", got)
	}
}

func TestGetStatusFallsBackToDefaultGatewayPort(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{OpenClawDir: t.TempDir(), Edition: "pro"}
	procMgr := process.NewManager(cfg)
	napcatMon := monitor.NewNapCatMonitor(cfg, nil, nil)

	r := gin.New()
	r.GET("/status", GetStatus(nil, cfg, procMgr, napcatMon))

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	openclaw, ok := resp["openclaw"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing openclaw payload: %v", resp["openclaw"])
	}

	if got := int(openclaw["gatewayPort"].(float64)); got != cfg.DefaultGatewayPort() {
		t.Fatalf("expected gatewayPort %d, got %d", cfg.DefaultGatewayPort(), got)
	}
}
