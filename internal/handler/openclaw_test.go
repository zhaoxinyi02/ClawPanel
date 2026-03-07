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

func TestPatchModelsJSONForAgentUsesConfiguredAgentDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	writeJSONRaw := func(path string, data map[string]interface{}) {
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

	writeJSONRaw(filepath.Join(dir, "openclaw.json"), map[string]interface{}{
		"agents": map[string]interface{}{
			"list": []interface{}{
				map[string]interface{}{"id": "work", "agentDir": "custom/work-agent"},
			},
		},
	})
	modelsPath := filepath.Join(dir, "custom", "work-agent", "agent", "models.json")
	writeJSONRaw(modelsPath, map[string]interface{}{
		"providers": map[string]interface{}{
			"deepseek": map[string]interface{}{
				"baseUrl": "https://api.deepseek.com/v1",
				"models": []interface{}{
					map[string]interface{}{"id": "deepseek-chat", "compat": map[string]interface{}{"supportsDeveloperRole": true}},
				},
			},
		},
	})

	patchModelsJSONForAgent(cfg, "work")

	raw, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	var saved map[string]interface{}
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("decode models.json: %v", err)
	}
	providers, _ := saved["providers"].(map[string]interface{})
	provider, _ := providers["deepseek"].(map[string]interface{})
	models, _ := provider["models"].([]interface{})
	model, _ := models[0].(map[string]interface{})
	compat, _ := model["compat"].(map[string]interface{})
	if got, _ := compat["supportsDeveloperRole"].(bool); got {
		t.Fatalf("expected compat.supportsDeveloperRole to be forced false")
	}
}

func TestSwitchFeishuVariantRejectsUninstalledTarget(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	initial := map[string]interface{}{
		"plugins": map[string]interface{}{
			"entries": map[string]interface{}{
				"feishu": map[string]interface{}{"enabled": true},
			},
			"installs": map[string]interface{}{
				"feishu": map[string]interface{}{"installPath": filepath.Join(dir, "extensions", "feishu")},
			},
		},
	}
	raw, _ := json.Marshal(initial)
	if err := os.MkdirAll(filepath.Join(dir, "extensions", "feishu"), 0755); err != nil {
		t.Fatalf("mkdir feishu extension: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openclaw.json"), raw, 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	r := gin.New()
	r.POST("/openclaw/feishu-variant", SwitchFeishuVariant(cfg, nil))
	body := []byte(`{"variant":"official"}`)
	req := httptest.NewRequest(http.MethodPost, "/openclaw/feishu-variant", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}

	saved, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	entries := saved["plugins"].(map[string]interface{})["entries"].(map[string]interface{})
	if enabled, _ := entries["feishu"].(map[string]interface{})["enabled"].(bool); !enabled {
		t.Fatalf("expected existing feishu variant to remain enabled")
	}
	if entries["feishu-openclaw-plugin"] != nil {
		t.Fatalf("expected official variant entry not to be synthesized when not installed")
	}
}

func TestSwitchFeishuVariantEnablesInstalledTarget(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	if err := os.MkdirAll(filepath.Join(dir, "extensions", "feishu"), 0755); err != nil {
		t.Fatalf("mkdir feishu extension: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "extensions", "feishu-openclaw-plugin"), 0755); err != nil {
		t.Fatalf("mkdir official extension: %v", err)
	}
	initial := map[string]interface{}{
		"plugins": map[string]interface{}{
			"entries": map[string]interface{}{
				"feishu":                 map[string]interface{}{"enabled": true},
				"feishu-openclaw-plugin": map[string]interface{}{"enabled": false},
			},
			"installs": map[string]interface{}{
				"feishu":                 map[string]interface{}{"installPath": filepath.Join(dir, "extensions", "feishu")},
				"feishu-openclaw-plugin": map[string]interface{}{"installPath": filepath.Join(dir, "extensions", "feishu-openclaw-plugin")},
			},
		},
	}
	raw, _ := json.Marshal(initial)
	if err := os.WriteFile(filepath.Join(dir, "openclaw.json"), raw, 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	r := gin.New()
	r.POST("/openclaw/feishu-variant", SwitchFeishuVariant(cfg, nil))
	body := []byte(`{"variant":"official"}`)
	req := httptest.NewRequest(http.MethodPost, "/openclaw/feishu-variant", bytes.NewReader(body))
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
	entries := saved["plugins"].(map[string]interface{})["entries"].(map[string]interface{})
	if enabled, _ := entries["feishu-openclaw-plugin"].(map[string]interface{})["enabled"].(bool); !enabled {
		t.Fatalf("expected official variant to be enabled")
	}
	if enabled, _ := entries["feishu"].(map[string]interface{})["enabled"].(bool); enabled {
		t.Fatalf("expected clawteam variant to be disabled")
	}
}
