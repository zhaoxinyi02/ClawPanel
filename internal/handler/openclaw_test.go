package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
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

func TestSaveOpenClawConfigMirrorsPrimaryModelToLegacyField(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	r := gin.New()
	r.PUT("/openclaw/config", SaveOpenClawConfig(cfg))

	body, _ := json.Marshal(map[string]interface{}{
		"config": map[string]interface{}{
			"agents": map[string]interface{}{
				"defaults": map[string]interface{}{
					"model": map[string]interface{}{
						"primary": "deepseek/deepseek-chat",
					},
				},
			},
			"models": map[string]interface{}{
				"providers": map[string]interface{}{},
			},
		},
	})
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

	model, ok := saved["model"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected legacy model field to be written")
	}
	if got, _ := model["primary"].(string); got != "deepseek/deepseek-chat" {
		t.Fatalf("unexpected legacy primary model: %q", got)
	}
}

func TestGetOpenClawConfigBackfillsAgentDefaultsModelFromLegacyField(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	initial := map[string]interface{}{
		"model": map[string]interface{}{
			"primary": "openai/gpt-4o-mini",
		},
	}
	raw, _ := json.Marshal(initial)
	if err := os.WriteFile(filepath.Join(dir, "openclaw.json"), raw, 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	r := gin.New()
	r.GET("/openclaw/config", GetOpenClawConfig(cfg))

	req := httptest.NewRequest(http.MethodGet, "/openclaw/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		OK     bool                   `json:"ok"`
		Config map[string]interface{} `json:"config"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	agents, _ := resp.Config["agents"].(map[string]interface{})
	defaults, _ := agents["defaults"].(map[string]interface{})
	model, _ := defaults["model"].(map[string]interface{})
	if got, _ := model["primary"].(string); got != "openai/gpt-4o-mini" {
		t.Fatalf("expected backfilled primary model, got %q", got)
	}
}

func TestSaveOpenClawConfigRejectsInvalidNumericFields(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		config map[string]interface{}
		want   string
	}{
		{
			name: "invalid context tokens",
			config: map[string]interface{}{
				"agents": map[string]interface{}{
					"defaults": map[string]interface{}{
						"contextTokens": 0,
					},
				},
			},
			want: "agents.defaults.contextTokens",
		},
		{
			name: "invalid history share",
			config: map[string]interface{}{
				"agents": map[string]interface{}{
					"defaults": map[string]interface{}{
						"compaction": map[string]interface{}{
							"maxHistoryShare": 1.2,
						},
					},
				},
			},
			want: "agents.defaults.compaction.maxHistoryShare",
		},
		{
			name: "invalid bootstrap max chars",
			config: map[string]interface{}{
				"agents": map[string]interface{}{
					"defaults": map[string]interface{}{
						"bootstrapMaxChars": 0,
					},
				},
			},
			want: "agents.defaults.bootstrapMaxChars",
		},
		{
			name: "invalid bootstrap total max chars",
			config: map[string]interface{}{
				"agents": map[string]interface{}{
					"defaults": map[string]interface{}{
						"bootstrapTotalMaxChars": 0,
					},
				},
			},
			want: "agents.defaults.bootstrapTotalMaxChars",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			cfg := &config.Config{OpenClawDir: dir}

			r := gin.New()
			r.PUT("/openclaw/config", SaveOpenClawConfig(cfg))

			body, _ := json.Marshal(map[string]interface{}{"config": tc.config})
			req := httptest.NewRequest(http.MethodPut, "/openclaw/config", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("expected error to mention %q, got %s", tc.want, w.Body.String())
			}
		})
	}
}

func TestSaveOpenClawConfigPreservesMissingHiddenFieldsWithoutOverwritingExplicitChanges(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	initial := map[string]interface{}{
		"tools": map[string]interface{}{
			"profile": "minimal",
			"agentToAgent": map[string]interface{}{
				"enabled": true,
			},
		},
		"session": map[string]interface{}{
			"dmScope": "main",
			"maintenance": map[string]interface{}{
				"maxEntries": 50,
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

	body, _ := json.Marshal(map[string]interface{}{
		"config": map[string]interface{}{
			"tools": map[string]interface{}{
				"profile": "full",
			},
			"session": map[string]interface{}{
				"dmScope": "per-account-channel-peer",
			},
			"models": map[string]interface{}{
				"providers": map[string]interface{}{},
			},
		},
	})
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

	tools, _ := saved["tools"].(map[string]interface{})
	if got := strings.TrimSpace(toString(tools["profile"])); got != "full" {
		t.Fatalf("expected tools.profile to keep explicit change, got %q", got)
	}
	agentToAgent, _ := tools["agentToAgent"].(map[string]interface{})
	if enabled, ok := agentToAgent["enabled"].(bool); !ok || !enabled {
		t.Fatalf("expected missing tools.agentToAgent.enabled to be preserved, got %#v", agentToAgent)
	}

	session, _ := saved["session"].(map[string]interface{})
	if got := strings.TrimSpace(toString(session["dmScope"])); got != "per-account-channel-peer" {
		t.Fatalf("expected session.dmScope to keep explicit change, got %q", got)
	}
	maintenance, _ := session["maintenance"].(map[string]interface{})
	if maxEntries, ok := maintenance["maxEntries"].(float64); !ok || maxEntries != 50 {
		t.Fatalf("expected missing session.maintenance.maxEntries to be preserved as 50, got %#v", maintenance)
	}
}

func TestSaveOpenClawConfigRejectsInvalidDMScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	r := gin.New()
	r.PUT("/openclaw/config", SaveOpenClawConfig(cfg))

	body, _ := json.Marshal(map[string]interface{}{
		"config": map[string]interface{}{
			"session": map[string]interface{}{
				"dmScope": "user",
			},
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/openclaw/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "session.dmScope") {
		t.Fatalf("expected dmScope validation error, got %s", w.Body.String())
	}
}

func TestGetFeishuDMDiagnosisReportsSharedMainSession(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	rawConfig, _ := json.Marshal(map[string]interface{}{
		"channels": map[string]interface{}{
			"feishu": map[string]interface{}{
				"defaultAccount": "fly",
				"accounts": map[string]interface{}{
					"backup": map[string]interface{}{"appId": "cli_backup"},
					"fly":    map[string]interface{}{"appId": "cli_fly"},
				},
				"dmPolicy": "pairing",
			},
		},
	})
	if err := os.WriteFile(filepath.Join(dir, "openclaw.json"), rawConfig, 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	sessionDir := filepath.Join(dir, "agents", "main", "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	rawSessions, _ := json.Marshal(map[string]interface{}{
		"agent:main:main": map[string]interface{}{
			"deliveryContext": map[string]interface{}{
				"channel":   "feishu",
				"accountId": "fly",
			},
		},
	})
	if err := os.WriteFile(filepath.Join(sessionDir, "sessions.json"), rawSessions, 0644); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}

	r := gin.New()
	r.GET("/openclaw/feishu-dm-diagnosis", GetFeishuDMDiagnosis(cfg))
	req := httptest.NewRequest(http.MethodGet, "/openclaw/feishu-dm-diagnosis", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		OK        bool              `json:"ok"`
		Diagnosis FeishuDMDiagnosis `json:"diagnosis"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok response: %s", w.Body.String())
	}
	if resp.Diagnosis.ConfiguredDMScope != "" {
		t.Fatalf("expected configured dmScope to be empty, got %q", resp.Diagnosis.ConfiguredDMScope)
	}
	if resp.Diagnosis.EffectiveDMScope != "main" {
		t.Fatalf("expected effective dmScope main, got %q", resp.Diagnosis.EffectiveDMScope)
	}
	if resp.Diagnosis.RecommendedDMScope != "per-account-channel-peer" {
		t.Fatalf("expected recommended dmScope per-account-channel-peer, got %q", resp.Diagnosis.RecommendedDMScope)
	}
	if !resp.Diagnosis.HasSharedMainSessionKey {
		t.Fatalf("expected shared main session key to be detected")
	}
	if resp.Diagnosis.FeishuSessionCount != 1 {
		t.Fatalf("expected one feishu session, got %d", resp.Diagnosis.FeishuSessionCount)
	}
	if resp.Diagnosis.MainSessionKey != "agent:main:main" {
		t.Fatalf("unexpected main session key: %q", resp.Diagnosis.MainSessionKey)
	}
}

func TestGetFeishuDMDiagnosisScansAllAgents(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	rawConfig, _ := json.Marshal(map[string]interface{}{
		"agents": map[string]interface{}{
			"list": []interface{}{
				map[string]interface{}{"id": "main", "default": true},
				map[string]interface{}{"id": "work"},
			},
		},
		"channels": map[string]interface{}{
			"feishu": map[string]interface{}{
				"appId": "cli_main",
			},
		},
	})
	if err := os.WriteFile(filepath.Join(dir, "openclaw.json"), rawConfig, 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	sessionDir := filepath.Join(dir, "agents", "work", "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("mkdir work sessions: %v", err)
	}
	rawSessions, _ := json.Marshal(map[string]interface{}{
		"agent:work:main": map[string]interface{}{
			"deliveryContext": map[string]interface{}{
				"channel": "feishu",
			},
		},
	})
	if err := os.WriteFile(filepath.Join(sessionDir, "sessions.json"), rawSessions, 0644); err != nil {
		t.Fatalf("write work sessions.json: %v", err)
	}

	diagnosis := buildFeishuDMDiagnosis(cfg)
	if !diagnosis.SessionIndexExists {
		t.Fatalf("expected at least one session index to be detected")
	}
	if diagnosis.FeishuSessionCount != 1 {
		t.Fatalf("expected one feishu session across agents, got %d", diagnosis.FeishuSessionCount)
	}
	if !diagnosis.HasSharedMainSessionKey {
		t.Fatalf("expected shared main session key from non-default agent to be detected")
	}
	if diagnosis.MainSessionKey != "agent:work:main" {
		t.Fatalf("expected shared main session key from work agent, got %q", diagnosis.MainSessionKey)
	}
	if len(diagnosis.ScannedAgentIDs) != 1 || diagnosis.ScannedAgentIDs[0] != "work" {
		t.Fatalf("expected only work agent session index to be scanned, got %#v", diagnosis.ScannedAgentIDs)
	}
}

func TestGetFeishuDMDiagnosisReadsAuthorizedSenders(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	rawConfig, _ := json.Marshal(map[string]interface{}{
		"channels": map[string]interface{}{
			"feishu": map[string]interface{}{
				"defaultAccount": "default",
				"accounts": map[string]interface{}{
					"default": map[string]interface{}{"appId": "cli_default"},
					"fly":     map[string]interface{}{"appId": "cli_fly"},
				},
				"dmPolicy": "pairing",
			},
		},
	})
	if err := os.WriteFile(filepath.Join(dir, "openclaw.json"), rawConfig, 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	credentialsDir := filepath.Join(dir, "credentials")
	if err := os.MkdirAll(credentialsDir, 0755); err != nil {
		t.Fatalf("mkdir credentials: %v", err)
	}

	writeJSON := func(path string, value map[string]interface{}) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal %s: %v", path, err)
		}
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	writeJSON(filepath.Join(credentialsDir, "feishu-default-allowFrom.json"), map[string]interface{}{
		"version":   1,
		"allowFrom": []interface{}{"ou_default_scoped"},
	})
	writeJSON(filepath.Join(credentialsDir, "feishu-allowFrom.json"), map[string]interface{}{
		"version":   1,
		"allowFrom": []interface{}{"ou_default_legacy"},
	})
	writeJSON(filepath.Join(credentialsDir, "feishu-fly-allowFrom.json"), map[string]interface{}{
		"version":   1,
		"allowFrom": []interface{}{"ou_fly"},
	})
	writeJSON(filepath.Join(credentialsDir, "feishu-pairing.json"), map[string]interface{}{
		"version": 1,
		"requests": []interface{}{
			map[string]interface{}{"id": "ou_pending", "code": "ABCD1234"},
		},
	})

	diagnosis := buildFeishuDMDiagnosis(cfg)
	if diagnosis.PendingPairingCount != 1 {
		t.Fatalf("expected pending pairing count 1, got %d", diagnosis.PendingPairingCount)
	}
	if diagnosis.AuthorizedSenderCount != 3 {
		t.Fatalf("expected authorized sender count 3, got %d", diagnosis.AuthorizedSenderCount)
	}
	if len(diagnosis.AuthorizedSenders) != 2 {
		t.Fatalf("expected 2 authorized sender buckets, got %#v", diagnosis.AuthorizedSenders)
	}

	defaultBucket := diagnosis.AuthorizedSenders[0]
	if defaultBucket.AccountID != "default" {
		t.Fatalf("expected first bucket default, got %#v", defaultBucket)
	}
	if defaultBucket.SenderCount != 2 {
		t.Fatalf("expected default sender count 2, got %#v", defaultBucket)
	}
	if len(defaultBucket.SenderIDs) != 2 || !strings.Contains(strings.Join(defaultBucket.SenderIDs, ","), "ou_default_scoped") || !strings.Contains(strings.Join(defaultBucket.SenderIDs, ","), "ou_default_legacy") {
		t.Fatalf("unexpected default sender ids: %#v", defaultBucket.SenderIDs)
	}
	if len(defaultBucket.SourceFiles) != 2 {
		t.Fatalf("expected default source files to include scoped and legacy stores, got %#v", defaultBucket.SourceFiles)
	}

	flyBucket := diagnosis.AuthorizedSenders[1]
	if flyBucket.AccountID != "fly" || flyBucket.SenderCount != 1 {
		t.Fatalf("unexpected fly bucket: %#v", flyBucket)
	}
	if len(flyBucket.SenderIDs) != 1 || flyBucket.SenderIDs[0] != "ou_fly" {
		t.Fatalf("unexpected fly sender ids: %#v", flyBucket.SenderIDs)
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

func TestPatchModelsJSONForAgentSkipsExternalNestedAgentDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	external := filepath.Join(t.TempDir(), "isolated", "agent")
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
				map[string]interface{}{"id": "work", "agentDir": external},
			},
		},
	})
	modelsPath := filepath.Join(external, "models.json")
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
	if got, _ := compat["supportsDeveloperRole"].(bool); !got {
		t.Fatalf("expected external agentDir models.json to remain untouched")
	}
}

func TestNormalizeFeishuChannelConfigMirrorsDefaultAccountToTopLevel(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"defaultAccount": "main",
		"accounts": map[string]interface{}{
			"main": map[string]interface{}{
				"appId":     "cli_main",
				"appSecret": "secret_main",
			},
			"backup": map[string]interface{}{
				"appId": "cli_backup",
			},
		},
	}

	got := normalizeFeishuChannelConfig(input)

	if got["appId"] != "cli_main" {
		t.Fatalf("expected top-level appId mirrored from default account, got %#v", got["appId"])
	}
	if got["appSecret"] != "secret_main" {
		t.Fatalf("expected top-level appSecret mirrored from default account, got %#v", got["appSecret"])
	}
}

func TestNormalizeFeishuChannelConfigSeedsDefaultAccountFromTopLevel(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"appId":          "cli_simple",
		"appSecret":      "secret_simple",
		"defaultAccount": "default",
	}

	got := normalizeFeishuChannelConfig(input)

	accounts, _ := got["accounts"].(map[string]interface{})
	entry, _ := accounts["default"].(map[string]interface{})
	if entry == nil {
		t.Fatalf("expected default account entry to be created, got %#v", got["accounts"])
	}
	if entry["appId"] != "cli_simple" {
		t.Fatalf("expected default account appId to inherit top-level value, got %#v", entry["appId"])
	}
	if entry["appSecret"] != "secret_simple" {
		t.Fatalf("expected default account appSecret to inherit top-level value, got %#v", entry["appSecret"])
	}
	if enabled, _ := entry["enabled"].(bool); !enabled {
		t.Fatalf("expected seeded default account to be enabled, got %#v", entry["enabled"])
	}
}

func TestNormalizeFeishuChannelConfigCreatesDefaultAccountFromTopLevelWhenMissing(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"appId":     "cli_simple",
		"appSecret": "secret_simple",
	}

	got := normalizeFeishuChannelConfig(input)

	if got["defaultAccount"] != "default" {
		t.Fatalf("expected missing default account to migrate to default, got %#v", got["defaultAccount"])
	}
	accounts, _ := got["accounts"].(map[string]interface{})
	entry, _ := accounts["default"].(map[string]interface{})
	if entry == nil {
		t.Fatalf("expected default account entry to be created, got %#v", got["accounts"])
	}
	if entry["appId"] != "cli_simple" || entry["appSecret"] != "secret_simple" {
		t.Fatalf("expected top-level credentials to seed default account, got %#v", entry)
	}
	if enabled, _ := entry["enabled"].(bool); !enabled {
		t.Fatalf("expected migrated default account to be enabled, got %#v", entry["enabled"])
	}
}

func TestNormalizeFeishuChannelConfigPreservesAccountMetadataAndForcesDefaultEnabled(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"defaultAccount": "main",
		"accounts": map[string]interface{}{
			"main": map[string]interface{}{
				"appId":      "cli_main",
				"appSecret":  "secret_main",
				"botName":    "主机器人",
				"enabled":    false,
				"tenantName": "Team A",
			},
			"backup": map[string]interface{}{
				"appId":      "cli_backup",
				"appSecret":  "secret_backup",
				"botName":    "备用机器人",
				"enabled":    false,
				"tenantName": "Team B",
			},
		},
	}

	got := normalizeFeishuChannelConfig(input)
	accounts, _ := got["accounts"].(map[string]interface{})
	mainEntry, _ := accounts["main"].(map[string]interface{})
	backupEntry, _ := accounts["backup"].(map[string]interface{})
	if mainEntry == nil || backupEntry == nil {
		t.Fatalf("expected both accounts to be preserved, got %#v", got["accounts"])
	}
	if mainEntry["botName"] != "主机器人" || mainEntry["tenantName"] != "Team A" {
		t.Fatalf("expected default account metadata to be preserved, got %#v", mainEntry)
	}
	if enabled, _ := mainEntry["enabled"].(bool); !enabled {
		t.Fatalf("expected default account to be forced enabled, got %#v", mainEntry["enabled"])
	}
	if backupEntry["botName"] != "备用机器人" || backupEntry["tenantName"] != "Team B" {
		t.Fatalf("expected backup account metadata to be preserved, got %#v", backupEntry)
	}
	if enabled, _ := backupEntry["enabled"].(bool); enabled {
		t.Fatalf("expected explicit disabled backup account to stay disabled, got %#v", backupEntry["enabled"])
	}
	if got["appId"] != "cli_main" || got["appSecret"] != "secret_main" {
		t.Fatalf("expected top-level credentials to mirror default account, got appId=%#v appSecret=%#v", got["appId"], got["appSecret"])
	}
}

func TestNormalizeFeishuChannelConfigInfersMissingDefaultFromTopLevelMirror(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"appId":     "cli_main",
		"appSecret": "secret_main",
		"accounts": map[string]interface{}{
			"backup": map[string]interface{}{
				"appId":     "cli_backup",
				"appSecret": "secret_backup",
			},
			"main": map[string]interface{}{
				"appId":     "cli_main",
				"appSecret": "secret_main",
			},
		},
	}

	got := normalizeFeishuChannelConfig(input)

	if got["defaultAccount"] != "main" {
		t.Fatalf("expected defaultAccount to be inferred from top-level mirror, got %#v", got["defaultAccount"])
	}
	if got["appId"] != "cli_main" || got["appSecret"] != "secret_main" {
		t.Fatalf("expected top-level credentials to stay mirrored to main, got appId=%#v appSecret=%#v", got["appId"], got["appSecret"])
	}
}

func TestNormalizeFeishuChannelConfigDoesNotInferDefaultFromPartialMirror(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"appId": "cli_shared",
		"accounts": map[string]interface{}{
			"a": map[string]interface{}{"appId": "cli_shared", "appSecret": "secret_a"},
			"b": map[string]interface{}{"appId": "cli_shared", "appSecret": "secret_b"},
		},
	}

	got := normalizeFeishuChannelConfig(input)

	if got["defaultAccount"] != "a" {
		t.Fatalf("expected safe fallback to first sorted account, got %#v", got["defaultAccount"])
	}
	if got["appSecret"] != "secret_a" {
		t.Fatalf("expected top-level mirror to be rewritten from resolved default account, got %#v", got["appSecret"])
	}
}

func TestNormalizeFeishuChannelConfigInfersMissingDefaultFromUniqueEnabledAccount(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"accounts": map[string]interface{}{
			"default": map[string]interface{}{
				"appId":     "cli_default",
				"appSecret": "secret_default",
				"enabled":   0,
			},
			"work": map[string]interface{}{
				"appId":     "cli_work",
				"appSecret": "secret_work",
				"enabled":   "1",
			},
		},
	}

	got := normalizeFeishuChannelConfig(input)

	if got["defaultAccount"] != "work" {
		t.Fatalf("expected unique enabled account to become default, got %#v", got["defaultAccount"])
	}
	if got["appId"] != "cli_work" || got["appSecret"] != "secret_work" {
		t.Fatalf("expected top-level mirror to follow inferred enabled default, got appId=%#v appSecret=%#v", got["appId"], got["appSecret"])
	}
}

func TestNormalizeFeishuChannelConfigIgnoresMetadataOnlyAccountsForDefaultInference(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"accounts": map[string]interface{}{
			"backup": map[string]interface{}{
				"botName": "备用机器人",
			},
			"main": map[string]interface{}{
				"appId":     "cli_main",
				"appSecret": "secret_main",
			},
		},
	}

	got := normalizeFeishuChannelConfig(input)

	if got["defaultAccount"] != "main" {
		t.Fatalf("expected metadata-only account to be ignored for default inference, got %#v", got["defaultAccount"])
	}
	if got["appId"] != "cli_main" || got["appSecret"] != "secret_main" {
		t.Fatalf("expected top-level mirror to follow runnable account, got appId=%#v appSecret=%#v", got["appId"], got["appSecret"])
	}
}

func TestNormalizeFeishuChannelConfigFallsBackFromExplicitMetadataOnlyDefault(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"defaultAccount": "backup",
		"accounts": map[string]interface{}{
			"backup": map[string]interface{}{
				"botName": "备用机器人",
			},
			"main": map[string]interface{}{
				"appId":     "cli_main",
				"appSecret": "secret_main",
				"enabled":   true,
			},
		},
	}

	got := normalizeFeishuChannelConfig(input)

	if got["defaultAccount"] != "main" {
		t.Fatalf("expected explicit metadata-only default to fall back to runnable account, got %#v", got["defaultAccount"])
	}
	if got["appId"] != "cli_main" || got["appSecret"] != "secret_main" {
		t.Fatalf("expected top-level mirror to follow runnable fallback account, got appId=%#v appSecret=%#v", got["appId"], got["appSecret"])
	}
}

func TestNormalizeFeishuChannelConfigDoesNotMaterializeMissingDefaultWithoutTopLevelSeed(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"defaultAccount": "ghost",
		"accounts": map[string]interface{}{
			"backup": map[string]interface{}{
				"botName": "备用机器人",
			},
		},
	}

	got := normalizeFeishuChannelConfig(input)

	if got["defaultAccount"] != "backup" {
		t.Fatalf("expected missing explicit default to fall back to existing account, got %#v", got["defaultAccount"])
	}
	accounts := got["accounts"].(map[string]interface{})
	if _, exists := accounts["ghost"]; exists {
		t.Fatalf("expected missing explicit default to remain absent, got %#v", accounts["ghost"])
	}
}

func TestNormalizeFeishuChannelConfigClearsMissingDefaultWithoutAccountsOrSeed(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"defaultAccount": "ghost",
	}

	got := normalizeFeishuChannelConfig(input)

	if _, ok := got["defaultAccount"]; ok {
		t.Fatalf("expected missing explicit default to be cleared when no accounts or top-level seed exist, got %#v", got["defaultAccount"])
	}
	accounts, _ := got["accounts"].(map[string]interface{})
	if len(accounts) != 0 {
		t.Fatalf("expected no accounts to be materialized, got %#v", accounts)
	}
}

func TestNormalizeFeishuChannelConfigParsesNumericEnabledFlags(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"defaultAccount": "main",
		"accounts": map[string]interface{}{
			"main": map[string]interface{}{
				"appId":     "cli_main",
				"appSecret": "secret_main",
				"enabled":   1,
			},
			"backup": map[string]interface{}{
				"appId":     "cli_backup",
				"appSecret": "secret_backup",
				"enabled":   0,
			},
		},
	}

	got := normalizeFeishuChannelConfig(input)
	accounts, _ := got["accounts"].(map[string]interface{})
	mainEntry, _ := accounts["main"].(map[string]interface{})
	backupEntry, _ := accounts["backup"].(map[string]interface{})
	if enabled, _ := mainEntry["enabled"].(bool); !enabled {
		t.Fatalf("expected numeric enabled=1 to normalize to true, got %#v", mainEntry["enabled"])
	}
	if enabled, _ := backupEntry["enabled"].(bool); enabled {
		t.Fatalf("expected numeric enabled=0 to normalize to false, got %#v", backupEntry["enabled"])
	}
}

func TestNormalizeFeishuChannelConfigParsesMentionAndAllowList(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"requireMention": "false",
		"groupPolicy":    "allowlist",
		"groupAllowFrom": "oc_123， oc_456 , ",
	}

	got := normalizeFeishuChannelConfig(input)

	if got["requireMention"] != false {
		t.Fatalf("expected requireMention to normalize to bool false, got %#v", got["requireMention"])
	}
	groupAllowFrom, _ := got["groupAllowFrom"].([]interface{})
	if len(groupAllowFrom) != 2 {
		t.Fatalf("expected two allowlist entries, got %#v", got["groupAllowFrom"])
	}
	if groupAllowFrom[0] != "oc_123" || groupAllowFrom[1] != "oc_456" {
		t.Fatalf("unexpected normalized allowlist: %#v", groupAllowFrom)
	}
}

func TestNormalizeFeishuChannelConfigDropsAllowlistOutsideAllowlistPolicy(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"groupPolicy":    "open",
		"groupAllowFrom": "oc_123, oc_456",
	}

	got := normalizeFeishuChannelConfig(input)

	if got["groupPolicy"] != "open" {
		t.Fatalf("expected groupPolicy to stay open, got %#v", got["groupPolicy"])
	}
	if _, exists := got["groupAllowFrom"]; exists {
		t.Fatalf("expected groupAllowFrom to be removed when policy is not allowlist, got %#v", got["groupAllowFrom"])
	}
}

func TestNormalizeFeishuChannelConfigDropsLegacyDMScope(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"dmScope":  "user",
		"dmPolicy": "pairing",
	}

	got := normalizeFeishuChannelConfig(input)

	if _, exists := got["dmScope"]; exists {
		t.Fatalf("expected legacy dmScope to be removed, got %#v", got["dmScope"])
	}
	if got["dmPolicy"] != "pairing" {
		t.Fatalf("expected dmPolicy to be preserved, got %#v", got["dmPolicy"])
	}
}

func TestNormalizeFeishuChannelConfigToleratesLegacyRequireMentionOpen(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"requireMention": "open",
	}

	err := normalizeFeishuChannelConfigInPlace(input)
	if err != nil {
		t.Fatalf("expected legacy requireMention=open to be tolerated, got %v", err)
	}
	if _, exists := input["requireMention"]; exists {
		t.Fatalf("expected legacy requireMention=open to be dropped, got %#v", input["requireMention"])
	}
}

func TestNormalizeWeComChannelConfigCallbackMode(t *testing.T) {
	t.Parallel()

	body := map[string]interface{}{
		"token":          "  token-123  ",
		"encodingAesKey": "  abcdef  ",
		"webhookPath":    " /wecom/callback ",
	}

	got := normalizeWeComChannelConfig(body)
	if got["connectionMode"] != "callback" {
		t.Fatalf("expected callback mode, got %#v", got["connectionMode"])
	}
	if got["encodingAESKey"] != "abcdef" {
		t.Fatalf("expected normalized encodingAESKey, got %#v", got["encodingAESKey"])
	}
	if _, exists := got["encodingAesKey"]; exists {
		t.Fatalf("expected legacy encodingAesKey to be removed, got %#v", got)
	}
	if got["webhookPath"] != "/wecom/callback" {
		t.Fatalf("expected trimmed webhookPath, got %#v", got["webhookPath"])
	}
}

func TestNormalizeWeComChannelConfigLongPollingMode(t *testing.T) {
	t.Parallel()

	body := map[string]interface{}{
		"connectionMode": "long_polling",
		"botId":          " bot-001 ",
		"secret":         " sec-001 ",
	}

	got := normalizeWeComChannelConfig(body)
	if got["connectionMode"] != "long-polling" {
		t.Fatalf("expected long-polling mode, got %#v", got["connectionMode"])
	}
	if got["botId"] != "bot-001" || got["secret"] != "sec-001" {
		t.Fatalf("expected trimmed bot credentials, got %#v", got)
	}
	if got["dmPolicy"] != "open" {
		t.Fatalf("expected long-polling mode to default dmPolicy=open, got %#v", got["dmPolicy"])
	}
}

func TestSaveChannelRejectsQQWhenPluginMissing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	r := gin.New()
	r.PUT("/openclaw/channels/:id", SaveChannel(cfg, nil))

	body := []byte(`{"enabled":true,"wsUrl":"ws://127.0.0.1:3001"}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/qq", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestSaveChannelRejectsQQBotWhenPluginMissing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	r := gin.New()
	r.PUT("/openclaw/channels/:id", SaveChannel(cfg, nil))

	body := []byte(`{"enabled":true,"appId":"123","clientSecret":"secret"}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/qqbot", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestSaveChannelRejectsWeComWhenPluginMissing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	r := gin.New()
	r.PUT("/openclaw/channels/:id", SaveChannel(cfg, nil))

	body := []byte(`{"enabled":true,"token":"abc","encodingAESKey":"def","webhookPath":"/wecom"}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/wecom", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestToggleChannelRejectsQQWhenPluginMissing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	r := gin.New()
	r.PUT("/openclaw/channels/toggle", ToggleChannel(cfg, nil, nil))

	body := []byte(`{"channelId":"qq","enabled":true}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestToggleChannelRejectsQQBotWhenPluginMissing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	r := gin.New()
	r.PUT("/openclaw/channels/toggle", ToggleChannel(cfg, nil, nil))

	body := []byte(`{"channelId":"qqbot","enabled":true}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestToggleChannelRejectsWeComWhenPluginMissing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	r := gin.New()
	r.PUT("/openclaw/channels/toggle", ToggleChannel(cfg, nil, nil))

	body := []byte(`{"channelId":"wecom","enabled":true}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestResolveWeComBotEntryID(t *testing.T) {
	tests := []struct {
		name    string
		entries map[string]interface{}
		want    string
	}{
		{"empty entries returns canonical id", map[string]interface{}{}, "wecom-openclaw-plugin"},
		{"new official entry present", map[string]interface{}{"wecom-openclaw-plugin": map[string]interface{}{}}, "wecom-openclaw-plugin"},
		{"legacy entry present", map[string]interface{}{"wecom": map[string]interface{}{}}, "wecom"},
		{"new entry takes precedence", map[string]interface{}{"wecom-openclaw-plugin": map[string]interface{}{}, "wecom": map[string]interface{}{}}, "wecom-openclaw-plugin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveWecomBotEntryID(tt.entries)
			if got != tt.want {
				t.Errorf("resolveWecomBotEntryID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToggleChannelSupportsOpenClawWeixinLabel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "extensions", "openclaw-weixin"), 0755); err != nil {
		t.Fatalf("mkdir openclaw-weixin extension: %v", err)
	}
	cfg := &config.Config{OpenClawDir: dir}
	r := gin.New()
	r.POST("/openclaw/toggle-channel", ToggleChannel(cfg, nil, nil))

	body := []byte(`{"channelId":"openclaw-weixin","enabled":true}`)
	req := httptest.NewRequest(http.MethodPost, "/openclaw/toggle-channel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "微信（ClawBot）") {
		t.Fatalf("expected response to mention openclaw-weixin label, got %s", w.Body.String())
	}
}

func TestSaveChannelQQReturnsMessageWithoutProcessManager(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "extensions", "qq"), 0755); err != nil {
		t.Fatalf("mkdir qq extension: %v", err)
	}
	cfg := &config.Config{OpenClawDir: dir}
	r := gin.New()
	r.PUT("/openclaw/channels/:id", SaveChannel(cfg, nil))

	body := []byte(`{"enabled":true,"wsUrl":"ws://127.0.0.1:3001","notifications":{"antiRecall":false}}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/qq", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("expected ok response, got %#v", resp)
	}
}

func TestSaveChannelToleratesLegacyFeishuRequireMentionOpen(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	r := gin.New()
	r.PUT("/openclaw/channels/:id", SaveChannel(cfg, nil))

	body := []byte(`{"enabled":true,"requireMention":"open","groupPolicy":"open"}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/feishu", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestSaveChannelRequestsGatewayRestartForTelegramWhenRunning(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	procMgr := process.NewManager(cfg)
	procMgrStatus := process.Status{Running: true, PID: 1234, StartedAt: time.Now()}

	v := reflect.ValueOf(procMgr).Elem().FieldByName("status")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(reflect.ValueOf(procMgrStatus))

	r := gin.New()
	r.PUT("/openclaw/channels/:id", SaveChannel(cfg, procMgr))

	body := []byte(`{"enabled":true,"token":"123456:abc","dmPolicy":"open"}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// handler calls procMgr.Restart(); in CI there is no openclaw binary so it
	// returns a restartWarning instead of restarted=true — both are acceptable
	// because the handler did attempt the restart.
	restarted, _ := resp["restarted"].(bool)
	_, hasWarning := resp["restartWarning"]
	if !restarted && !hasWarning {
		t.Fatalf("expected restarted=true or restartWarning in response, got %#v", resp)
	}
	if saved, err := cfg.ReadOpenClawJSON(); err != nil {
		t.Fatalf("read openclaw config: %v", err)
	} else if channels, _ := saved["channels"].(map[string]interface{}); channels == nil || channels["telegram"] == nil {
		t.Fatalf("expected telegram config to be saved, got %#v", saved)
	} else if telegram, _ := channels["telegram"].(map[string]interface{}); strings.TrimSpace(fmt.Sprint(telegram["botToken"])) != "123456:abc" {
		t.Fatalf("expected telegram botToken to be normalized, got %#v", telegram)
	} else if allowFrom, _ := channels["telegram"].(map[string]interface{})["allowFrom"].([]interface{}); len(allowFrom) == 0 || fmt.Sprint(allowFrom[len(allowFrom)-1]) != "*" {
		t.Fatalf("expected telegram allowFrom wildcard, got %#v", channels["telegram"])
	}
}

// --- Feishu plugin ID resolution tests ---

func TestResolveFeishuEntryID(t *testing.T) {
	entries := map[string]interface{}{
		"openclaw-lark": map[string]interface{}{"enabled": false},
		"feishu":        map[string]interface{}{"enabled": true},
	}
	tests := []struct {
		name           string
		entries        map[string]interface{}
		candidates     []string
		requireEnabled bool
		want           string
	}{
		{"empty entries enabled", map[string]interface{}{}, feishuAllPluginIDs, true, ""},
		{"empty entries present", map[string]interface{}{}, feishuAllPluginIDs, false, ""},
		{"find enabled community", entries, feishuAllPluginIDs, true, "feishu"},
		{"find enabled official only", entries, feishuOfficialPluginIDs, true, ""},
		{"find present official", entries, feishuOfficialPluginIDs, false, "openclaw-lark"},
		{"enabled wins over present", entries, feishuAllPluginIDs, false, "feishu"},
		{"all disabled returns first present", map[string]interface{}{
			"openclaw-lark": map[string]interface{}{"enabled": false},
			"feishu":        map[string]interface{}{"enabled": false},
		}, feishuAllPluginIDs, false, "openclaw-lark"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFeishuEntryID(tt.entries, tt.candidates, tt.requireEnabled)
			if got != tt.want {
				t.Errorf("resolveFeishuEntryID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanupLegacyFeishuPluginIDs(t *testing.T) {
	plugins := map[string]interface{}{
		"allow": []interface{}{"feishu-openclaw-plugin", canonicalFeishuOfficialPluginID},
		"entries": map[string]interface{}{
			"feishu-openclaw-plugin":         map[string]interface{}{"enabled": true},
			canonicalFeishuOfficialPluginID:  map[string]interface{}{"enabled": true},
			canonicalFeishuCommunityPluginID: map[string]interface{}{"enabled": true},
		},
	}
	got := cleanupLegacyFeishuPluginIDs(plugins)
	allow, _ := got["allow"].([]interface{})
	entries, _ := got["entries"].(map[string]interface{})
	if len(allow) != 1 || strings.TrimSpace(fmt.Sprint(allow[0])) != canonicalFeishuOfficialPluginID {
		t.Fatalf("expected legacy ID removed from allow, got %#v", allow)
	}
	if _, ok := entries["feishu-openclaw-plugin"]; ok {
		t.Fatalf("expected legacy entry removed, got %#v", entries["feishu-openclaw-plugin"])
	}
}

func TestResolveFeishuToggleEntryID(t *testing.T) {
	tests := []struct {
		name    string
		plugins map[string]interface{}
		entries map[string]interface{}
		want    string
	}{
		{"prefer allow official", map[string]interface{}{"allow": []interface{}{canonicalFeishuOfficialPluginID}}, map[string]interface{}{}, canonicalFeishuOfficialPluginID},
		{"prefer allow community", map[string]interface{}{"allow": []interface{}{canonicalFeishuCommunityPluginID}}, map[string]interface{}{}, canonicalFeishuCommunityPluginID},
		{"community entry stays community", map[string]interface{}{}, map[string]interface{}{
			canonicalFeishuCommunityPluginID: map[string]interface{}{"enabled": true},
		}, canonicalFeishuCommunityPluginID},
		{"default fallback community", map[string]interface{}{}, map[string]interface{}{}, canonicalFeishuCommunityPluginID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFeishuToggleEntryID(tt.plugins, tt.entries)
			if got != tt.want {
				t.Fatalf("resolveFeishuToggleEntryID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePreferredOfficialFeishuID(t *testing.T) {
	tests := []struct {
		name    string
		plugins map[string]interface{}
		entries map[string]interface{}
		want    string
	}{
		{"prefer allow canonical official", map[string]interface{}{"allow": []interface{}{canonicalFeishuOfficialPluginID}}, map[string]interface{}{}, canonicalFeishuOfficialPluginID},
		{"prefer installs canonical official", map[string]interface{}{"installs": map[string]interface{}{canonicalFeishuOfficialPluginID: true}}, map[string]interface{}{}, canonicalFeishuOfficialPluginID},
		{"default fallback canonical official", map[string]interface{}{}, map[string]interface{}{}, canonicalFeishuOfficialPluginID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePreferredOfficialFeishuID(tt.plugins, tt.entries)
			if got != tt.want {
				t.Fatalf("resolvePreferredOfficialFeishuID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToggleChannelFeishuHealsEntriesAndAllow(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	initial := map[string]interface{}{
		"plugins": map[string]interface{}{
			"allow": []interface{}{"feishu-openclaw-plugin"},
			"entries": map[string]interface{}{
				"feishu-openclaw-plugin": map[string]interface{}{"enabled": true},
			},
		},
	}
	if err := cfg.WriteOpenClawJSON(initial); err != nil {
		t.Fatalf("seed openclaw config: %v", err)
	}

	r := gin.New()
	r.PUT("/openclaw/channels/toggle", ToggleChannel(cfg, nil, nil))
	body := []byte(`{"channelId":"feishu","enabled":true}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	saved, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("read openclaw config: %v", err)
	}
	plugins, _ := saved["plugins"].(map[string]interface{})
	entries, _ := plugins["entries"].(map[string]interface{})
	allow, _ := plugins["allow"].([]interface{})
	if got := strings.TrimSpace(fmt.Sprint(allow[0])); got != canonicalFeishuCommunityPluginID || len(allow) != 1 {
		t.Fatalf("expected allow=[%s], got %#v", canonicalFeishuCommunityPluginID, allow)
	}
	if _, ok := entries["feishu-openclaw-plugin"]; ok {
		t.Fatalf("expected legacy official entry removed, got %#v", entries["feishu-openclaw-plugin"])
	}
	if entry, _ := entries[canonicalFeishuCommunityPluginID].(map[string]interface{}); entry == nil || entry["enabled"] != true {
		t.Fatalf("expected %s enabled entry, got %#v", canonicalFeishuCommunityPluginID, entries[canonicalFeishuCommunityPluginID])
	}
}

func TestSwitchFeishuVariantUsesCanonicalIDsAndAllow(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("switch to official removes legacy and uses canonical official", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &config.Config{OpenClawDir: dir}
		initial := map[string]interface{}{
			"plugins": map[string]interface{}{
				"allow": []interface{}{"feishu-openclaw-plugin"},
				"entries": map[string]interface{}{
					"feishu-openclaw-plugin":         map[string]interface{}{"enabled": true},
					canonicalFeishuCommunityPluginID: map[string]interface{}{"enabled": true},
				},
			},
		}
		if err := cfg.WriteOpenClawJSON(initial); err != nil {
			t.Fatalf("seed openclaw config: %v", err)
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
			t.Fatalf("read openclaw config: %v", err)
		}
		plugins, _ := saved["plugins"].(map[string]interface{})
		entries, _ := plugins["entries"].(map[string]interface{})
		allow, _ := plugins["allow"].([]interface{})
		if len(allow) != 1 || strings.TrimSpace(fmt.Sprint(allow[0])) != canonicalFeishuOfficialPluginID {
			t.Fatalf("expected allow=[%s], got %#v", canonicalFeishuOfficialPluginID, allow)
		}
		if _, ok := entries["feishu-openclaw-plugin"]; ok {
			t.Fatalf("expected legacy official entry removed, got %#v", entries["feishu-openclaw-plugin"])
		}
		if entry, _ := entries[canonicalFeishuOfficialPluginID].(map[string]interface{}); entry == nil || entry["enabled"] != true {
			t.Fatalf("expected %s enabled, got %#v", canonicalFeishuOfficialPluginID, entries[canonicalFeishuOfficialPluginID])
		}
		if entry, _ := entries[canonicalFeishuCommunityPluginID].(map[string]interface{}); entry == nil || entry["enabled"] != false {
			t.Fatalf("expected %s disabled, got %#v", canonicalFeishuCommunityPluginID, entries[canonicalFeishuCommunityPluginID])
		}
	})

	t.Run("switch to community uses community canonical id", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &config.Config{OpenClawDir: dir}
		initial := map[string]interface{}{
			"plugins": map[string]interface{}{
				"allow": []interface{}{canonicalFeishuOfficialPluginID},
				"entries": map[string]interface{}{
					canonicalFeishuOfficialPluginID: map[string]interface{}{"enabled": true},
				},
			},
		}
		if err := cfg.WriteOpenClawJSON(initial); err != nil {
			t.Fatalf("seed openclaw config: %v", err)
		}
		r := gin.New()
		r.POST("/openclaw/feishu-variant", SwitchFeishuVariant(cfg, nil))
		body := []byte(`{"variant":"clawteam"}`)
		req := httptest.NewRequest(http.MethodPost, "/openclaw/feishu-variant", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
		}
		saved, err := cfg.ReadOpenClawJSON()
		if err != nil {
			t.Fatalf("read openclaw config: %v", err)
		}
		plugins, _ := saved["plugins"].(map[string]interface{})
		entries, _ := plugins["entries"].(map[string]interface{})
		allow, _ := plugins["allow"].([]interface{})
		if len(allow) != 1 || strings.TrimSpace(fmt.Sprint(allow[0])) != canonicalFeishuCommunityPluginID {
			t.Fatalf("expected allow=[%s], got %#v", canonicalFeishuCommunityPluginID, allow)
		}
		if entry, _ := entries[canonicalFeishuCommunityPluginID].(map[string]interface{}); entry == nil || entry["enabled"] != true {
			t.Fatalf("expected %s enabled, got %#v", canonicalFeishuCommunityPluginID, entries[canonicalFeishuCommunityPluginID])
		}
		if entry, _ := entries[canonicalFeishuOfficialPluginID].(map[string]interface{}); entry == nil || entry["enabled"] != false {
			t.Fatalf("expected %s disabled, got %#v", canonicalFeishuOfficialPluginID, entries[canonicalFeishuOfficialPluginID])
		}
	})
}

func TestToggleChannelFeishuDisableKeepsAllowButDisablesEntries(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	initial := map[string]interface{}{
		"plugins": map[string]interface{}{
			"allow": []interface{}{canonicalFeishuOfficialPluginID, "diagnostics-otel"},
			"entries": map[string]interface{}{
				canonicalFeishuOfficialPluginID:  map[string]interface{}{"enabled": true},
				canonicalFeishuCommunityPluginID: map[string]interface{}{"enabled": true},
			},
		},
	}
	if err := cfg.WriteOpenClawJSON(initial); err != nil {
		t.Fatalf("seed openclaw config: %v", err)
	}

	r := gin.New()
	r.PUT("/openclaw/channels/toggle", ToggleChannel(cfg, nil, nil))
	body := []byte(`{"channelId":"feishu","enabled":false}`)
	req := httptest.NewRequest(http.MethodPut, "/openclaw/channels/toggle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	saved, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("read openclaw config: %v", err)
	}
	plugins, _ := saved["plugins"].(map[string]interface{})
	entries, _ := plugins["entries"].(map[string]interface{})
	allow, _ := plugins["allow"].([]interface{})
	if len(allow) != 2 || strings.TrimSpace(fmt.Sprint(allow[0])) != canonicalFeishuOfficialPluginID || strings.TrimSpace(fmt.Sprint(allow[1])) != "diagnostics-otel" {
		t.Fatalf("expected allow preserved, got %#v", allow)
	}
	for _, pluginID := range feishuAllPluginIDs {
		if entry, _ := entries[pluginID].(map[string]interface{}); entry != nil && entry["enabled"] != false {
			t.Fatalf("expected %s disabled, got %#v", pluginID, entry)
		}
	}
}
