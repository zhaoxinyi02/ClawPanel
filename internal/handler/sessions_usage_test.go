package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestGetSessionUsageAggregatesUsageWindows(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	sessionsDir := resolveAgentSessionsDir(cfg, "main")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions dir: %v", err)
	}

	now := time.Now()
	lines := []map[string]interface{}{
		{
			"type":      "message",
			"timestamp": now.Format(time.RFC3339Nano),
			"message": map[string]interface{}{
				"role": "assistant",
				"usage": map[string]interface{}{
					"input":       100,
					"output":      40,
					"cacheRead":   10,
					"cacheWrite":  0,
					"totalTokens": 150,
					"cost": map[string]interface{}{
						"total": 0.12,
					},
				},
			},
		},
		{
			"type":      "message",
			"timestamp": now.AddDate(0, 0, -10).Format(time.RFC3339Nano),
			"message": map[string]interface{}{
				"role": "assistant",
				"usage": map[string]interface{}{
					"input":       50,
					"output":      20,
					"cacheRead":   0,
					"cacheWrite":  0,
					"totalTokens": 70,
					"cost": map[string]interface{}{
						"total": 0.04,
					},
				},
			},
		},
		{
			"type":      "message",
			"timestamp": now.AddDate(0, 0, -40).Format(time.RFC3339Nano),
			"message": map[string]interface{}{
				"role": "assistant",
				"usage": map[string]interface{}{
					"input":       999,
					"output":      999,
					"cacheRead":   0,
					"cacheWrite":  0,
					"totalTokens": 1998,
					"cost": map[string]interface{}{
						"total": 9.99,
					},
				},
			},
		},
	}

	f, err := os.Create(filepath.Join(sessionsDir, "demo.jsonl"))
	if err != nil {
		t.Fatalf("create jsonl: %v", err)
	}
	defer f.Close()
	for _, line := range lines {
		raw, _ := json.Marshal(line)
		if _, err := f.Write(append(raw, '\n')); err != nil {
			t.Fatalf("write jsonl: %v", err)
		}
	}

	r := gin.New()
	r.GET("/sessions/usage", GetSessionUsage(cfg))

	req := httptest.NewRequest(http.MethodGet, "/sessions/usage?agent=main", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		OK      bool `json:"ok"`
		Summary struct {
			Today struct {
				TotalTokens int64 `json:"totalTokens"`
				Requests    int   `json:"requests"`
			} `json:"today"`
			Last7d struct {
				TotalTokens int64 `json:"totalTokens"`
			} `json:"last7d"`
			Last30d struct {
				TotalTokens int64 `json:"totalTokens"`
				Requests    int   `json:"requests"`
			} `json:"last30d"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Summary.Today.TotalTokens != 150 {
		t.Fatalf("expected today totalTokens=150, got %d", resp.Summary.Today.TotalTokens)
	}
	if resp.Summary.Today.Requests != 1 {
		t.Fatalf("expected today requests=1, got %d", resp.Summary.Today.Requests)
	}
	if resp.Summary.Last7d.TotalTokens != 150 {
		t.Fatalf("expected last7d totalTokens=150, got %d", resp.Summary.Last7d.TotalTokens)
	}
	if resp.Summary.Last30d.TotalTokens != 220 {
		t.Fatalf("expected last30d totalTokens=220, got %d", resp.Summary.Last30d.TotalTokens)
	}
	if resp.Summary.Last30d.Requests != 2 {
		t.Fatalf("expected last30d requests=2, got %d", resp.Summary.Last30d.Requests)
	}
}
