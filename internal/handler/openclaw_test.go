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
