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

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/taskman"
	_ "modernc.org/sqlite"
)

func TestBuildHermesActionScript(t *testing.T) {
	t.Parallel()

	name, script, ok := buildHermesActionScript("doctor")
	if !ok {
		t.Fatalf("expected doctor action to be supported")
	}
	if name == "" || script == "" {
		t.Fatalf("expected non-empty task name and script")
	}

	if _, _, ok := buildHermesActionScript("unknown-action"); ok {
		t.Fatalf("unexpected support for unknown Hermes action")
	}
}

func TestSaveHermesConfigWritesFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)

	r := gin.New()
	r.PUT("/hermes/config", SaveHermesConfig())

	body, _ := json.Marshal(map[string]interface{}{
		"configYaml": "model:\n  provider: openrouter\n",
		"envFile":    "OPENROUTER_API_KEY=test\n",
	})
	req := httptest.NewRequest(http.MethodPut, "/hermes/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	configPath := filepath.Join(home, "config.yaml")
	envPath := filepath.Join(home, ".env")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config.yaml to be written: %v", err)
	}
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("expected .env to be written: %v", err)
	}
}

func TestSaveHermesConfigRejectsInvalidYAML(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)

	r := gin.New()
	r.PUT("/hermes/config", SaveHermesConfig())

	body, _ := json.Marshal(map[string]interface{}{
		"configYaml": "model: [\n",
	})
	req := httptest.NewRequest(http.MethodPut, "/hermes/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestResolveHermesLogSelectionOnlyAllowsKnownFiles(t *testing.T) {
	t.Parallel()

	files := []HermesLogFileSummary{
		{Name: "gateway.log", Path: "/tmp/hermes/logs/gateway.log"},
		{Name: "hermes.log", Path: "/tmp/hermes/logs/hermes.log"},
	}

	if got := resolveHermesLogSelection(files, "gateway.log"); got != "/tmp/hermes/logs/gateway.log" {
		t.Fatalf("expected gateway.log match, got %q", got)
	}
	if got := resolveHermesLogSelection(files, "/tmp/hermes/logs/hermes.log"); got != "/tmp/hermes/logs/hermes.log" {
		t.Fatalf("expected hermes.log path match, got %q", got)
	}
	if got := resolveHermesLogSelection(files, "/etc/passwd"); got != "" {
		t.Fatalf("expected unknown path to be rejected, got %q", got)
	}
}

func TestCollectHermesTasksFiltersTaskTypes(t *testing.T) {
	t.Parallel()

	tm := taskman.NewManager(nil)
	tm.CreateTask("Install Hermes", "install_hermes")
	tm.CreateTask("Hermes Doctor", "hermes_doctor")
	tm.CreateTask("Install OpenClaw", "install_openclaw")

	tasks, summary := collectHermesTasks(tm)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 Hermes tasks, got %d", len(tasks))
	}
	if summary.Total != 2 {
		t.Fatalf("expected total=2, got %d", summary.Total)
	}
}

func TestBuildHermesHealthSnapshotIncludesChecks(t *testing.T) {
	t.Parallel()

	status := HermesStatus{
		Installed:      true,
		Configured:     false,
		Running:        false,
		GatewayRunning: false,
		PythonVersion:  "Python 3.11.9",
		BinaryPath:     "/usr/local/bin/hermes",
		ConfigPath:     "/root/.hermes/config.yaml",
	}
	data := HermesDataSummary{
		StateDirExists:   true,
		LogsDirExists:    true,
		LogFileCount:     0,
		SQLiteStoreCount: 1,
	}
	health := buildHermesHealthSnapshot(status, data, nil, HermesTaskSummary{Total: 2, ByStatus: map[string]int{"running": 1, "success": 1}})
	if len(health.Checks) == 0 {
		t.Fatalf("expected non-empty health checks")
	}
	if health.Summary.Total != 2 {
		t.Fatalf("expected summary total=2, got %d", health.Summary.Total)
	}
}

func TestBuildHermesSessionPreviewReadsJsonl(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "conversation-1.jsonl")
	content := `{"role":"user","content":"你好"}
{"role":"assistant","content":"你好，我是 Hermes"}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}

	session := buildHermesSessionPreview(path)
	if session == nil {
		t.Fatalf("expected session preview")
	}
	if session.MessageCount != 2 {
		t.Fatalf("expected messageCount=2, got %d", session.MessageCount)
	}
	if session.Title == "" {
		t.Fatalf("expected derived title")
	}
}

func TestParseHermesEnvFile(t *testing.T) {
	t.Parallel()

	env := parseHermesEnvFile(`
# comment
export TELEGRAM_BOT_TOKEN="abc"
SLACK_APP_TOKEN=xyz
INVALID_LINE
`)
	if env["TELEGRAM_BOT_TOKEN"] != "abc" {
		t.Fatalf("expected TELEGRAM_BOT_TOKEN to be parsed")
	}
	if env["SLACK_APP_TOKEN"] != "xyz" {
		t.Fatalf("expected SLACK_APP_TOKEN to be parsed")
	}
}

func TestDetectHermesPlatforms(t *testing.T) {
	t.Parallel()

	config := HermesConfigState{
		Files: HermesConfigFiles{
			ConfigYAML: "gateway:\n  telegram:\n    enabled: true\n  slack:\n    enabled: false\n",
			EnvFile:    "TELEGRAM_BOT_TOKEN=abc\nSLACK_BOT_TOKEN=x\n",
		},
	}
	platforms := detectHermesPlatforms(config)
	if platforms.ConfiguredCount == 0 {
		t.Fatalf("expected configured platforms")
	}
	foundTelegram := false
	for _, platform := range platforms.Platforms {
		if platform.ID == "telegram" {
			foundTelegram = true
			if !platform.Configured || !platform.Enabled {
				t.Fatalf("expected telegram to be configured and enabled")
			}
		}
	}
	if !foundTelegram {
		t.Fatalf("expected telegram platform in snapshot")
	}
}

func TestDoctorSnapshotReadWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir}
	snapshot := HermesDoctorSnapshot{
		UpdatedAt:  "2026-04-15T00:00:00Z",
		FixApplied: false,
		TaskID:     "task-1",
		TaskStatus: "success",
	}
	writeHermesDoctorSnapshot(cfg, snapshot)
	got := readHermesDoctorSnapshot(cfg)
	if got == nil {
		t.Fatalf("expected snapshot to be readable")
	}
	if got.TaskID != "task-1" {
		t.Fatalf("expected task id to round-trip, got %q", got.TaskID)
	}
}

func TestSaveHermesStructuredConfigWritesYAML(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)

	r := gin.New()
	r.PUT("/hermes/config/structured", SaveHermesStructuredConfig())

	body, _ := json.Marshal(map[string]interface{}{
		"model": map[string]interface{}{
			"provider": "openrouter",
			"name":     "anthropic/claude-sonnet-4-5",
		},
		"gateway": map[string]interface{}{
			"telegram": map[string]interface{}{
				"enabled": true,
			},
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/hermes/config/structured", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	raw, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatalf("expected config.yaml to exist: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "openrouter") || !strings.Contains(text, "telegram") {
		t.Fatalf("expected structured config to be written, got %s", text)
	}
}

func TestAnalyzeHermesPlatformEvidence(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	logsDir := filepath.Join(home, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logsDir, "gateway.log"), []byte("telegram connected successfully\nslack error invalid token\n"), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	status := HermesStatus{HomeDir: home, GatewayRunning: true}
	runtimeStatus, evidence, logErr := analyzeHermesPlatformEvidence("telegram", status)
	if runtimeStatus != "healthy" {
		t.Fatalf("expected healthy telegram status, got %s", runtimeStatus)
	}
	if evidence == "" || logErr != "" {
		t.Fatalf("expected telegram evidence without error, got evidence=%q error=%q", evidence, logErr)
	}

	runtimeStatus, evidence, logErr = analyzeHermesPlatformEvidence("slack", status)
	if runtimeStatus != "error" {
		t.Fatalf("expected slack error status, got %s", runtimeStatus)
	}
	if logErr == "" {
		t.Fatalf("expected slack error line")
	}
}

func TestBuildHermesPlatformDetail(t *testing.T) {
	t.Parallel()

	config := HermesConfigState{
		Status: HermesStatus{HomeDir: "/tmp/hermes"},
		Files: HermesConfigFiles{
			ConfigYAML: "gateway:\n  telegram:\n    enabled: true\n    botName: demo\n",
			EnvFile:    "TELEGRAM_BOT_TOKEN=abc\n",
		},
	}
	detail, ok := buildHermesPlatformDetail(config, "telegram")
	if !ok || detail == nil {
		t.Fatalf("expected telegram platform detail")
	}
	if !detail.Status.Enabled {
		t.Fatalf("expected telegram to be enabled")
	}
	if detail.Environment["TELEGRAM_BOT_TOKEN"] != "abc" {
		t.Fatalf("expected env value to be present")
	}
}

func TestSaveHermesPlatformDetailWritesConfigAndEnv(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("gateway:\n  telegram:\n    enabled: false\n"), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("TELEGRAM_BOT_TOKEN=old\n"), 0600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	r := gin.New()
	r.PUT("/hermes/platforms/:id", SaveHermesPlatformDetail())

	body, _ := json.Marshal(map[string]interface{}{
		"enabled": true,
		"config": map[string]interface{}{
			"enabled": true,
			"botName": "demo",
		},
		"env": map[string]string{
			"TELEGRAM_BOT_TOKEN": "new-token",
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/hermes/platforms/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	rawConfig, _ := os.ReadFile(filepath.Join(home, "config.yaml"))
	if !strings.Contains(string(rawConfig), "botName") {
		t.Fatalf("expected botName in config.yaml, got %s", string(rawConfig))
	}
	rawEnv, _ := os.ReadFile(filepath.Join(home, ".env"))
	if !strings.Contains(string(rawEnv), "new-token") {
		t.Fatalf("expected updated env file, got %s", string(rawEnv))
	}
}

func TestPreviewHermesSession(t *testing.T) {
	t.Parallel()

	config := HermesConfigState{
		Files: HermesConfigFiles{
			ConfigYAML: "group_sessions_per_user: true\n",
			EnvFile:    "TELEGRAM_HOME_CHAT=chat-1\n",
		},
	}
	preview := previewHermesSession(config, "telegram", "group", "g123", "u456")
	if !preview.GroupPerUser {
		t.Fatalf("expected groupPerUser=true")
	}
	if preview.SessionKey != "telegram:group:g123:user:u456" {
		t.Fatalf("unexpected session key: %s", preview.SessionKey)
	}
	if preview.HomeTarget != "chat-1" {
		t.Fatalf("expected home target to be read from env")
	}
}

func TestSaveHermesPersonalityWritesSoulFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)

	r := gin.New()
	r.PUT("/hermes/personality", SaveHermesPersonality())

	body, _ := json.Marshal(map[string]interface{}{
		"soulContent": "# SOUL\n\nYou are Hermes.\n",
	})
	req := httptest.NewRequest(http.MethodPut, "/hermes/personality", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(home, "SOUL.md"))
	if err != nil {
		t.Fatalf("expected SOUL.md to be written: %v", err)
	}
	if !strings.Contains(string(raw), "Hermes") {
		t.Fatalf("unexpected SOUL.md content: %s", string(raw))
	}
}

func TestSaveHermesProfileDetailWritesFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)

	r := gin.New()
	r.PUT("/hermes/profiles/:name", SaveHermesProfileDetail())

	body, _ := json.Marshal(map[string]interface{}{
		"content": "name: deep-work\nsystem: focused\n",
	})
	req := httptest.NewRequest(http.MethodPut, "/hermes/profiles/deep-work.yaml", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(home, "profiles", "deep-work.yaml"))
	if err != nil {
		t.Fatalf("expected profile file to be written: %v", err)
	}
	if !strings.Contains(string(raw), "deep-work") {
		t.Fatalf("unexpected profile file content: %s", string(raw))
	}
}

func TestApplyHermesRoutingMatchesRule(t *testing.T) {
	t.Parallel()

	routing := HermesRoutingConfig{
		DefaultProfile: "default",
		Rules: []HermesRouteRule{
			{
				ID:      "rule-1",
				Name:    "telegram-work",
				Enabled: true,
				Match: HermesRouteMatch{
					Platform: "telegram",
					ChatType: "group",
					ChatID:   "g1",
				},
				ProfileName: "deep-work",
				HomeTarget:  "ops-room",
			},
		},
	}
	base := HermesSessionPreview{Platform: "telegram", ChatType: "group", ChatID: "g1", SessionKey: "telegram:group:g1"}
	preview := applyHermesRouting(routing, base, "deploy status")
	if preview.MatchedRuleID != "rule-1" {
		t.Fatalf("expected routing rule match")
	}
	if preview.RoutedProfile != "deep-work" {
		t.Fatalf("expected routed profile deep-work, got %q", preview.RoutedProfile)
	}
	if preview.RoutedHomeTarget != "ops-room" {
		t.Fatalf("expected routed home target ops-room, got %q", preview.RoutedHomeTarget)
	}
}

func TestSaveHermesRoutingRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir}
	input := HermesRoutingConfig{
		DefaultProfile: "default",
		Rules: []HermesRouteRule{
			{
				Name:        "telegram-support",
				Enabled:     true,
				Priority:    10,
				Match:       HermesRouteMatch{Platform: "telegram", ChatType: "direct"},
				ProfileName: "support",
			},
		},
	}
	if err := saveHermesRoutingConfig(cfg, input); err != nil {
		t.Fatalf("save routing: %v", err)
	}
	got := loadHermesRoutingConfig(cfg)
	if got.DefaultProfile != "default" {
		t.Fatalf("expected default profile default, got %q", got.DefaultProfile)
	}
	if len(got.Rules) != 1 || got.Rules[0].ProfileName != "support" {
		t.Fatalf("unexpected routing rules: %+v", got.Rules)
	}
	if got.Rules[0].ID == "" {
		t.Fatalf("expected generated rule id")
	}
}

func TestInspectHermesSessionDBSummarizesUsage(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dbPath := filepath.Join(home, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE conversations (id TEXT PRIMARY KEY, title TEXT, input_tokens INTEGER, output_tokens INTEGER, total_tokens INTEGER);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO conversations (id, title, input_tokens, output_tokens, total_tokens) VALUES ('a', 'demo', 10, 5, 15), ('b', 'demo2', 20, 8, 28);`); err != nil {
		t.Fatalf("insert rows: %v", err)
	}

	status := HermesStatus{HomeDir: home, StateDBPath: dbPath}
	snapshot := inspectHermesSessionDB(status)
	if !snapshot.Exists {
		t.Fatalf("expected snapshot to exist")
	}
	if snapshot.Usage.TotalTokens != 43 {
		t.Fatalf("expected total tokens 43, got %d", snapshot.Usage.TotalTokens)
	}
	if len(snapshot.SessionTableCandidates) == 0 {
		t.Fatalf("expected session table candidate")
	}
}

func TestScanHermesStorageIncludesDBUsage(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "sessions"), 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "sessions", "s1.jsonl"), []byte(`{"role":"user","content":"hello"}`), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	dbPath := filepath.Join(home, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE session_metrics (id TEXT PRIMARY KEY, total_tokens INTEGER);`); err != nil {
		t.Fatalf("create metrics table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO session_metrics (id, total_tokens) VALUES ('a', 99);`); err != nil {
		t.Fatalf("insert metrics: %v", err)
	}

	status := HermesStatus{HomeDir: home, SessionsDir: filepath.Join(home, "sessions"), StateDBPath: dbPath}
	storage := scanHermesStorage(status)
	if storage.Usage.TotalTokens != 99 {
		t.Fatalf("expected usage total 99, got %d", storage.Usage.TotalTokens)
	}
	if !storage.DB.Exists {
		t.Fatalf("expected db snapshot to exist")
	}
}
