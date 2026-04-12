package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestSanitizeProviderAPIKeyKeepsTrailingEquals(t *testing.T) {
	got := sanitizeProviderAPIKey("sk-test-token=")
	if got != "sk-test-token=" {
		t.Fatalf("expected trailing '=' to be preserved, got %q", got)
	}
}

func TestSaveModelsStripsTransientProviderFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	cfg := &config.Config{OpenClawDir: tmpDir}
	if err := cfg.WriteOpenClawJSON(map[string]interface{}{}); err != nil {
		t.Fatalf("seed openclaw.json: %v", err)
	}

	body := map[string]interface{}{
		"providers": map[string]interface{}{
			"openai": map[string]interface{}{
				"baseUrl": "https://api.openai.com/v1",
				"apiKey":  "sk-test-token=",
				"api":     "openai-completions",
				"_note":   "temporary note",
				"models": []interface{}{
					map[string]interface{}{"id": "gpt-4o", "name": "gpt-4o"},
				},
			},
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/openclaw/models", bytes.NewReader(data))
	ctx.Request.Header.Set("Content-Type", "application/json")

	SaveModels(cfg)(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	saved, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("read saved openclaw.json: %v", err)
	}

	models, _ := saved["models"].(map[string]interface{})
	providers, _ := models["providers"].(map[string]interface{})
	provider, _ := providers["openai"].(map[string]interface{})
	if got, _ := provider["apiKey"].(string); got != "sk-test-token=" {
		t.Fatalf("expected apiKey to keep trailing '=' , got %q", got)
	}
	if _, exists := provider["_note"]; exists {
		t.Fatalf("expected transient _note to be stripped before writing: %s", filepath.Join(tmpDir, "openclaw.json"))
	}
	if got, _ := provider["baseUrl"].(string); got != "https://api.openai.com/v1" {
		t.Fatalf("expected provider fields to remain intact, got baseUrl=%q", got)
	}
}
