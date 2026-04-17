package handler

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/taskman"
	_ "modernc.org/sqlite"
)

const (
	hermesDocsURL = "https://hermes-agent.nousresearch.com/docs/"
	hermesRepoURL = "https://github.com/NousResearch/hermes-agent"
)

func firstHermesConfig(cfgs ...*config.Config) *config.Config {
	for _, cfg := range cfgs {
		if cfg != nil {
			return cfg
		}
	}
	return nil
}

var hermesPlatformSpecs = []hermesPlatformSpec{
	{ID: "telegram", Label: "Telegram", ConfigPaths: []string{"gateway.telegram"}, RequiredEnv: []string{"TELEGRAM_BOT_TOKEN"}, OptionalEnv: []string{"TELEGRAM_ALLOWED_USERS", "TELEGRAM_HOME_CHAT"}},
	{ID: "discord", Label: "Discord", ConfigPaths: []string{"gateway.discord"}, RequiredEnv: []string{"DISCORD_BOT_TOKEN"}, OptionalEnv: []string{"DISCORD_ALLOWED_USERS", "DISCORD_HOME_CHANNEL"}},
	{ID: "slack", Label: "Slack", ConfigPaths: []string{"gateway.slack"}, RequiredEnv: []string{"SLACK_BOT_TOKEN", "SLACK_APP_TOKEN"}, OptionalEnv: []string{"SLACK_ALLOWED_USERS", "SLACK_HOME_CHANNEL"}},
	{ID: "weixin", Label: "Weixin / WeChat", ConfigPaths: []string{"gateway.weixin"}, RequiredEnv: []string{"WEIXIN_ACCOUNT_ID", "WEIXIN_TOKEN"}, OptionalEnv: []string{"WEIXIN_HOME_CHANNEL", "WEIXIN_HOME_CHANNEL_NAME", "WEIXIN_ALLOWED_USERS"}},
	{ID: "wecom", Label: "WeCom", ConfigPaths: []string{"gateway.wecom"}, RequiredEnv: []string{"WECOM_BOT_ID", "WECOM_SECRET"}, OptionalEnv: []string{"WECOM_HOME_CHANNEL", "WECOM_HOME_CHANNEL_NAME", "WECOM_ALLOWED_USERS"}},
	{ID: "whatsapp", Label: "WhatsApp", ConfigPaths: []string{"gateway.whatsapp"}, RequiredEnv: []string{"WHATSAPP_ENABLED"}, OptionalEnv: []string{"WHATSAPP_ALLOWED_USERS", "WHATSAPP_HOME_CONTACT"}},
	{ID: "signal", Label: "Signal", ConfigPaths: []string{"gateway.signal"}, RequiredEnv: []string{"SIGNAL_ENABLED"}, OptionalEnv: []string{"SIGNAL_ALLOWED_USERS", "SIGNAL_HOME_CONTACT"}},
	{ID: "webhook", Label: "Webhooks", ConfigPaths: []string{"gateway.webhook", "gateway.webhooks", "webhooks"}, RequiredEnv: []string{"WEBHOOK_ENABLED"}, OptionalEnv: []string{"WEBHOOK_PORT", "WEBHOOK_SECRET"}},
	{ID: "feishu", Label: "Feishu / Lark", ConfigPaths: []string{"gateway.feishu"}, RequiredEnv: []string{"FEISHU_ENABLED"}, OptionalEnv: []string{"FEISHU_APP_ID", "FEISHU_APP_SECRET"}},
	{ID: "dingtalk", Label: "DingTalk", ConfigPaths: []string{"gateway.dingtalk"}, RequiredEnv: []string{"DINGTALK_ENABLED"}, OptionalEnv: []string{"DINGTALK_APP_KEY", "DINGTALK_APP_SECRET"}},
	{ID: "matrix", Label: "Matrix", ConfigPaths: []string{"gateway.matrix"}, RequiredEnv: []string{"MATRIX_ENABLED"}, OptionalEnv: []string{"MATRIX_HOMESERVER_URL", "MATRIX_ACCESS_TOKEN"}},
	{ID: "sms", Label: "SMS", ConfigPaths: []string{"gateway.sms"}, RequiredEnv: []string{"SMS_ENABLED"}, OptionalEnv: []string{"TWILIO_ACCOUNT_SID", "TWILIO_AUTH_TOKEN", "TWILIO_FROM_NUMBER"}},
}

type HermesStatus struct {
	Installed      bool   `json:"installed"`
	Configured     bool   `json:"configured"`
	Running        bool   `json:"running"`
	GatewayRunning bool   `json:"gatewayRunning"`
	Version        string `json:"version,omitempty"`
	BinaryPath     string `json:"binaryPath,omitempty"`
	HomeDir        string `json:"homeDir"`
	ConfigPath     string `json:"configPath"`
	EnvPath        string `json:"envPath"`
	StateDir       string `json:"stateDir"`
	SessionsDir    string `json:"sessionsDir"`
	StateDBPath    string `json:"stateDbPath"`
	AuthPath       string `json:"authPath"`
	PythonVersion  string `json:"pythonVersion,omitempty"`
	DocsURL        string `json:"docsUrl"`
	RepoURL        string `json:"repoUrl"`
}

type HermesConfigFiles struct {
	ConfigYAML string `json:"configYaml"`
	EnvFile    string `json:"envFile"`
}

type HermesConfigState struct {
	Status HermesStatus      `json:"status"`
	Files  HermesConfigFiles `json:"files"`
	Exists gin.H             `json:"exists"`
}

type HermesActionSpec struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

type HermesHealthCheck struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
	FixHint string `json:"fixHint,omitempty"`
}

type HermesHealthSnapshot struct {
	Summary HermesTaskSummary   `json:"summary"`
	Checks  []HermesHealthCheck `json:"checks"`
}

type HermesDoctorSnapshot struct {
	UpdatedAt   string                  `json:"updatedAt"`
	FixApplied  bool                    `json:"fixApplied"`
	TaskID      string                  `json:"taskId,omitempty"`
	TaskStatus  string                  `json:"taskStatus,omitempty"`
	Error       string                  `json:"error,omitempty"`
	RawLines    []string                `json:"rawLines,omitempty"`
	StatusLines []string                `json:"statusLines,omitempty"`
	Health      HermesHealthSnapshot    `json:"health"`
	Platforms   HermesPlatformsSnapshot `json:"platforms"`
	Warnings    []string                `json:"warnings,omitempty"`
}

type HermesLogFileSummary struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modifiedAt"`
}

type HermesDataSummary struct {
	HomeDir                  string   `json:"homeDir"`
	InstallDir               string   `json:"installDir"`
	LogsDir                  string   `json:"logsDir"`
	SkillsDir                string   `json:"skillsDir"`
	ProfilesDir              string   `json:"profilesDir"`
	StateDir                 string   `json:"stateDir"`
	SessionsDir              string   `json:"sessionsDir"`
	StateDBPath              string   `json:"stateDbPath"`
	AuthPath                 string   `json:"authPath"`
	InstallDirExists         bool     `json:"installDirExists"`
	LogsDirExists            bool     `json:"logsDirExists"`
	SkillsDirExists          bool     `json:"skillsDirExists"`
	ProfilesDirExists        bool     `json:"profilesDirExists"`
	StateDirExists           bool     `json:"stateDirExists"`
	SessionsDirExists        bool     `json:"sessionsDirExists"`
	StateDBExists            bool     `json:"stateDbExists"`
	AuthPathExists           bool     `json:"authPathExists"`
	SkillCount               int      `json:"skillCount"`
	ProfileCount             int      `json:"profileCount"`
	LogFileCount             int      `json:"logFileCount"`
	SQLiteStoreCount         int      `json:"sqliteStoreCount"`
	SessionArtifactCount     int      `json:"sessionArtifactCount"`
	DetectedSQLiteStores     []string `json:"detectedSqliteStores"`
	DetectedSessionArtifacts []string `json:"detectedSessionArtifacts"`
}

type HermesTaskSummary struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"byStatus"`
}

type HermesSessionMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

type HermesSessionItem struct {
	ID             string                 `json:"id"`
	Title          string                 `json:"title"`
	Path           string                 `json:"path"`
	UpdatedAt      string                 `json:"updatedAt,omitempty"`
	MessageCount   int                    `json:"messageCount"`
	RecentMessages []HermesSessionMessage `json:"recentMessages,omitempty"`
}

type HermesStorageSnapshot struct {
	SQLiteStores         []string                `json:"sqliteStores"`
	ConversationFiles    []string                `json:"conversationFiles"`
	SessionArtifacts     []string                `json:"sessionArtifacts"`
	ConversationCount    int                     `json:"conversationCount"`
	SessionArtifactCount int                     `json:"sessionArtifactCount"`
	PreviewSessions      []HermesSessionItem     `json:"previewSessions"`
	DB                   HermesSessionDBSnapshot `json:"db"`
	Usage                HermesUsageSummary      `json:"usage"`
}

type HermesUsageSummary struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	TotalTokens  int64 `json:"totalTokens"`
	Rows         int   `json:"rows"`
}

type HermesDBTableSummary struct {
	Name     string   `json:"name"`
	Columns  []string `json:"columns"`
	RowCount int64    `json:"rowCount"`
}

type HermesSessionDBSnapshot struct {
	Path                   string                 `json:"path"`
	Exists                 bool                   `json:"exists"`
	Tables                 []HermesDBTableSummary `json:"tables"`
	SessionTableCandidates []string               `json:"sessionTableCandidates"`
	Usage                  HermesUsageSummary     `json:"usage"`
}

type HermesPlatformStatus struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Configured     bool     `json:"configured"`
	Enabled        bool     `json:"enabled"`
	ConfigPath     string   `json:"configPath,omitempty"`
	PresentEnvKeys []string `json:"presentEnvKeys,omitempty"`
	MissingEnvKeys []string `json:"missingEnvKeys,omitempty"`
	RuntimeStatus  string   `json:"runtimeStatus,omitempty"`
	LastEvidence   string   `json:"lastEvidence,omitempty"`
	LastError      string   `json:"lastError,omitempty"`
	Detail         string   `json:"detail,omitempty"`
}

type HermesPlatformsSnapshot struct {
	Platforms       []HermesPlatformStatus `json:"platforms"`
	ConfiguredCount int                    `json:"configuredCount"`
	EnabledCount    int                    `json:"enabledCount"`
}

type HermesPlatformDetail struct {
	Status      HermesPlatformStatus   `json:"status"`
	Config      map[string]interface{} `json:"config"`
	Environment map[string]string      `json:"environment"`
}

type HermesOverview struct {
	Status      HermesStatus            `json:"status"`
	Config      HermesConfigState       `json:"config"`
	Actions     []HermesActionSpec      `json:"actions"`
	Data        HermesDataSummary       `json:"data"`
	Health      HermesHealthSnapshot    `json:"health"`
	Storage     HermesStorageSnapshot   `json:"storage"`
	Platforms   HermesPlatformsSnapshot `json:"platforms"`
	Doctor      *HermesDoctorSnapshot   `json:"doctor,omitempty"`
	LogFiles    []HermesLogFileSummary  `json:"logFiles"`
	RecentTasks []*taskman.Task         `json:"recentTasks"`
	TaskSummary HermesTaskSummary       `json:"taskSummary"`
	Warnings    []string                `json:"warnings"`
}

type HermesStructuredConfig struct {
	Model       map[string]interface{} `json:"model"`
	Gateway     map[string]interface{} `json:"gateway"`
	Session     map[string]interface{} `json:"session"`
	Tools       map[string]interface{} `json:"tools"`
	Memory      map[string]interface{} `json:"memory"`
	Personality map[string]interface{} `json:"personality"`
	Profiles    map[string]interface{} `json:"profiles"`
	Raw         map[string]interface{} `json:"raw"`
}

type HermesProfileFile struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Exists     bool   `json:"exists"`
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modifiedAt,omitempty"`
	Content    string `json:"content,omitempty"`
}

type HermesPersonalityState struct {
	SoulPath    string              `json:"soulPath"`
	SoulExists  bool                `json:"soulExists"`
	SoulContent string              `json:"soulContent"`
	Profiles    []HermesProfileFile `json:"profiles"`
}

type HermesSessionPreview struct {
	Platform     string `json:"platform"`
	ChatType     string `json:"chatType"`
	ChatID       string `json:"chatId"`
	UserID       string `json:"userId,omitempty"`
	GroupPerUser bool   `json:"groupPerUser"`
	UsedFallback bool   `json:"usedFallback"`
	SessionKey   string `json:"sessionKey"`
	HomeTarget   string `json:"homeTarget,omitempty"`
	Reason       string `json:"reason"`
}

func previewHermesSession(config HermesConfigState, platform string, chatType string, chatID string, userID string) HermesSessionPreview {
	raw := parseHermesYAMLFile(config.Files.ConfigYAML)
	groupPerUser := false
	if value, ok := raw["group_sessions_per_user"]; ok {
		groupPerUser = truthyHermesValue(value)
	}
	if !groupPerUser {
		if value := hermesNestedValue(raw, "session.group_sessions_per_user"); value != nil {
			groupPerUser = truthyHermesValue(value)
		}
	}
	if !groupPerUser {
		if value := hermesNestedValue(raw, "sessions.group_sessions_per_user"); value != nil {
			groupPerUser = truthyHermesValue(value)
		}
	}

	envMap := parseHermesEnvFile(config.Files.EnvFile)
	homeTarget := ""
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "telegram":
		homeTarget = envMap["TELEGRAM_HOME_CHAT"]
	case "discord":
		homeTarget = envMap["DISCORD_HOME_CHANNEL"]
	case "slack":
		homeTarget = envMap["SLACK_HOME_CHANNEL"]
	case "whatsapp":
		homeTarget = envMap["WHATSAPP_HOME_CONTACT"]
	case "signal":
		homeTarget = envMap["SIGNAL_HOME_CONTACT"]
	}

	platform = strings.TrimSpace(strings.ToLower(platform))
	chatType = strings.TrimSpace(strings.ToLower(chatType))
	chatID = strings.TrimSpace(chatID)
	userID = strings.TrimSpace(userID)
	if platform == "" {
		platform = "unknown"
	}
	if chatType == "" {
		chatType = "direct"
	}
	if chatID == "" {
		chatID = "unknown"
	}

	preview := HermesSessionPreview{
		Platform:     platform,
		ChatType:     chatType,
		ChatID:       chatID,
		UserID:       userID,
		GroupPerUser: groupPerUser,
		HomeTarget:   homeTarget,
	}

	if chatType == "group" || chatType == "channel" {
		if groupPerUser && userID != "" {
			preview.SessionKey = strings.Join([]string{platform, chatType, chatID, "user", userID}, ":")
			preview.Reason = "group_sessions_per_user=true，且提供了 userId，因此按群/频道 + 用户拆分会话"
			return preview
		}
		preview.SessionKey = strings.Join([]string{platform, chatType, chatID}, ":")
		preview.UsedFallback = groupPerUser && userID == ""
		preview.Reason = ternary(preview.UsedFallback, "group_sessions_per_user=true，但缺少 userId，回退为共享群会话", "按群/频道共享会话")
		return preview
	}

	preview.SessionKey = strings.Join([]string{platform, chatType, chatID}, ":")
	preview.Reason = "私聊默认按平台 + 会话目标拆分"
	return preview
}

type hermesPlatformSpec struct {
	ID          string
	Label       string
	ConfigPaths []string
	RequiredEnv []string
	OptionalEnv []string
}

func hermesHomeDir(cfgs ...*config.Config) string {
	if custom := strings.TrimSpace(os.Getenv("HERMES_HOME")); custom != "" {
		return custom
	}

	candidates := make([]string, 0, 8)
	appendCandidate := func(home string) {
		home = strings.TrimSpace(home)
		if home == "" {
			return
		}
		candidates = append(candidates, filepath.Join(home, ".hermes"))
	}

	if cfg := firstHermesConfig(cfgs...); cfg != nil {
		appendCandidate(filepath.Dir(strings.TrimSpace(cfg.OpenClawDir)))
	}

	home, _ := os.UserHomeDir()
	appendCandidate(home)
	appendCandidate(os.Getenv("HOME"))
	if runtime.GOOS == "windows" {
		appendCandidate(os.Getenv("USERPROFILE"))
	} else {
		appendCandidate("/root")
	}

	if runtime.GOOS == "darwin" {
		if entries, err := os.ReadDir("/Users"); err == nil {
			for _, entry := range entries {
				if entry.IsDir() && entry.Name() != "Shared" {
					appendCandidate(filepath.Join("/Users", entry.Name()))
				}
			}
		}
	}

	seen := map[string]struct{}{}
	bestPath := ""
	bestScore := -1
	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		if cleaned == "" || cleaned == "." {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		score := 0
		for _, marker := range []string{"config.yaml", ".env", "state.db", "state", "sessions"} {
			if _, err := os.Stat(filepath.Join(cleaned, marker)); err == nil {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestPath = cleaned
		}
	}

	if bestPath != "" {
		return bestPath
	}
	return ".hermes"
}

func detectHermesBinaryPath(cfgs ...*config.Config) string {
	if runtime.GOOS == "windows" {
		if out := detectCmd("where", "hermes"); out != "" {
			return firstNonEmptyLine(out)
		}
	} else {
		cmd := exec.Command("sh", "-lc", "command -v hermes 2>/dev/null || true")
		cmd.Env = config.BuildExecEnv()
		if out, err := cmd.Output(); err == nil {
			if path := firstNonEmptyLine(string(out)); path != "" {
				return path
			}
		}
	}

	home := hermesHomeDir(cfgs...)
	candidates := []string{
		filepath.Join(filepath.Dir(home), "bin", hermesExecutableName()),
		filepath.Join(filepath.Dir(home), ".local", "bin", hermesExecutableName()),
		filepath.Join(filepath.Dir(home), "hermes-agent", "venv", "bin", hermesExecutableName()),
		filepath.Join("/usr/local/bin", hermesExecutableName()),
		filepath.Join("/usr/bin", hermesExecutableName()),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func hermesExecutableName() string {
	if runtime.GOOS == "windows" {
		return "hermes.exe"
	}
	return "hermes"
}

func firstNonEmptyLine(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func detectHermesVersion(cfgs ...*config.Config) string {
	if version := firstNonEmptyLine(detectCmd("hermes", "--version")); version != "" {
		return version
	}
	if binaryPath := detectHermesBinaryPath(cfgs...); binaryPath != "" {
		cmd := exec.Command(binaryPath, "--version")
		cmd.Env = config.BuildExecEnv()
		if out, err := cmd.Output(); err == nil {
			return firstNonEmptyLine(string(out))
		}
	}
	return ""
}

func detectHermesProcessState() (running bool, gatewayRunning bool) {
	if runtime.GOOS == "windows" {
		return false, false
	}
	out := detectCmd("ps", "-axo", "pid=,command=")
	return parseHermesProcessState(out)
}

func parseHermesProcessState(raw string) (running bool, gatewayRunning bool) {
	if strings.TrimSpace(raw) == "" {
		return false, false
	}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		// Ignore process inspection commands and temporary Hermes snapshots to avoid false positives.
		if strings.Contains(lower, "pgrep -af hermes") || strings.Contains(lower, "ps -axo") || strings.Contains(lower, "grep hermes") || strings.Contains(lower, "egrep hermes") {
			continue
		}
		if strings.Contains(lower, "hermes-snap-") || strings.Contains(lower, "__hermes_") {
			continue
		}
		if strings.Contains(lower, "-m hermes_cli.main gateway run") || strings.Contains(lower, "hermes gateway") || strings.Contains(lower, "gateway start") || strings.Contains(lower, "gateway setup") {
			running = true
			gatewayRunning = true
			continue
		}
		if strings.Contains(lower, "-m hermes_cli.main") || strings.Contains(lower, " hermes ") || strings.HasSuffix(lower, " hermes") || strings.Contains(lower, "/hermes ") || strings.HasSuffix(lower, "/hermes") {
			running = true
		}
	}
	return running, gatewayRunning
}

func detectHermesStatus(cfgs ...*config.Config) HermesStatus {
	homeDir := hermesHomeDir(cfgs...)
	configPath := filepath.Join(homeDir, "config.yaml")
	envPath := filepath.Join(homeDir, ".env")
	stateDir := filepath.Join(homeDir, "state")
	sessionsDir := filepath.Join(homeDir, "sessions")
	stateDBPath := filepath.Join(homeDir, "state.db")
	authPath := filepath.Join(homeDir, "auth.json")
	binaryPath := detectHermesBinaryPath(cfgs...)
	version := detectHermesVersion(cfgs...)
	running, gatewayRunning := detectHermesProcessState()

	configured := false
	for _, path := range []string{configPath, envPath, stateDir, sessionsDir, stateDBPath, filepath.Join(homeDir, "skills")} {
		if info, err := os.Stat(path); err == nil {
			if info.IsDir() || info.Size() >= 0 {
				configured = true
				break
			}
		}
	}

	return HermesStatus{
		Installed:      binaryPath != "" || version != "",
		Configured:     configured,
		Running:        running,
		GatewayRunning: gatewayRunning,
		Version:        version,
		BinaryPath:     binaryPath,
		HomeDir:        homeDir,
		ConfigPath:     configPath,
		EnvPath:        envPath,
		StateDir:       stateDir,
		SessionsDir:    sessionsDir,
		StateDBPath:    stateDBPath,
		AuthPath:       authPath,
		PythonVersion:  detectPythonVersion(),
		DocsURL:        hermesDocsURL,
		RepoURL:        hermesRepoURL,
	}
}

func readTextFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func buildHermesConfigState(cfgs ...*config.Config) HermesConfigState {
	status := detectHermesStatus(cfgs...)
	return HermesConfigState{
		Status: status,
		Files: HermesConfigFiles{
			ConfigYAML: readTextFile(status.ConfigPath),
			EnvFile:    readTextFile(status.EnvPath),
		},
		Exists: gin.H{
			"homeDir":    fileOrDirExists(status.HomeDir),
			"configPath": fileOrDirExists(status.ConfigPath),
			"envPath":    fileOrDirExists(status.EnvPath),
			"stateDir":   fileOrDirExists(status.StateDir),
		},
	}
}

func buildHermesStructuredConfig(raw map[string]interface{}) HermesStructuredConfig {
	session := map[string]interface{}{}
	for _, key := range []string{"group_sessions_per_user", "session_search", "session_history_limit", "session_summary_tokens"} {
		if value, ok := raw[key]; ok {
			session[key] = value
		}
	}
	if nested := hermesNestedMap(raw, "session"); len(nested) > 0 {
		for key, value := range nested {
			session[key] = value
		}
	}
	if nested := hermesNestedMap(raw, "sessions"); len(nested) > 0 {
		for key, value := range nested {
			session[key] = value
		}
	}

	return HermesStructuredConfig{
		Model:       hermesNestedMap(raw, "model"),
		Gateway:     hermesNestedMap(raw, "gateway"),
		Session:     session,
		Tools:       hermesNestedMap(raw, "tools"),
		Memory:      hermesNestedMap(raw, "memory"),
		Personality: hermesNestedMap(raw, "personality"),
		Profiles:    hermesNestedMap(raw, "profiles"),
		Raw:         raw,
	}
}

func parseHermesYAMLFileWithErr(raw string) (map[string]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]interface{}{}, nil
	}
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	if parsed == nil {
		parsed = map[string]interface{}{}
	}
	return parsed, nil
}

func parseHermesEnvFile(raw string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "export ")
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			result[key] = value
		}
	}
	return result
}

func renderHermesEnvFile(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+values[key])
	}
	return strings.Join(lines, "\n") + "\n"
}

func parseHermesYAMLFile(raw string) map[string]interface{} {
	parsed, err := parseHermesYAMLFileWithErr(raw)
	if err != nil {
		return map[string]interface{}{}
	}
	return parsed
}

func cloneHermesMap(raw map[string]interface{}) map[string]interface{} {
	if raw == nil {
		return map[string]interface{}{}
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return map[string]interface{}{}
	}
	var clone map[string]interface{}
	if err := json.Unmarshal(buf, &clone); err != nil {
		return map[string]interface{}{}
	}
	return clone
}

func hermesNestedValue(raw map[string]interface{}, path string) interface{} {
	current := any(raw)
	for _, part := range strings.Split(path, ".") {
		next, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = next[part]
	}
	return current
}

func hermesNestedMap(raw map[string]interface{}, path string) map[string]interface{} {
	if raw == nil {
		return map[string]interface{}{}
	}
	if path == "" {
		return cloneHermesMap(raw)
	}
	value := hermesNestedValue(raw, path)
	if mapped, ok := value.(map[string]interface{}); ok {
		return cloneHermesMap(mapped)
	}
	return map[string]interface{}{}
}

func setHermesNestedValue(raw map[string]interface{}, path string, value interface{}) {
	if raw == nil || strings.TrimSpace(path) == "" {
		return
	}
	parts := strings.Split(path, ".")
	cur := raw
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := cur[part].(map[string]interface{})
		if !ok || next == nil {
			next = map[string]interface{}{}
			cur[part] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = value
}

func truthyHermesValue(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		value := strings.ToLower(strings.TrimSpace(t))
		return value == "true" || value == "1" || value == "yes" || value == "on"
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	default:
		return false
	}
}

func hermesInstallDir(homeDir string) string {
	return filepath.Join(homeDir, "hermes-agent")
}

func hermesLogsDir(homeDir string) string {
	return filepath.Join(homeDir, "logs")
}

func hermesSkillsDir(homeDir string) string {
	return filepath.Join(homeDir, "skills")
}

func hermesProfilesDir(homeDir string) string {
	return filepath.Join(homeDir, "profiles")
}

func hermesSoulPath(homeDir string) string {
	return filepath.Join(homeDir, "SOUL.md")
}

func detectHermesDataSummary(status HermesStatus) HermesDataSummary {
	summary := HermesDataSummary{
		HomeDir:           status.HomeDir,
		InstallDir:        hermesInstallDir(status.HomeDir),
		LogsDir:           hermesLogsDir(status.HomeDir),
		SkillsDir:         hermesSkillsDir(status.HomeDir),
		ProfilesDir:       hermesProfilesDir(status.HomeDir),
		StateDir:          status.StateDir,
		SessionsDir:       status.SessionsDir,
		StateDBPath:       status.StateDBPath,
		AuthPath:          status.AuthPath,
		InstallDirExists:  fileOrDirExists(hermesInstallDir(status.HomeDir)),
		LogsDirExists:     fileOrDirExists(hermesLogsDir(status.HomeDir)),
		SkillsDirExists:   fileOrDirExists(hermesSkillsDir(status.HomeDir)),
		ProfilesDirExists: fileOrDirExists(hermesProfilesDir(status.HomeDir)),
		StateDirExists:    fileOrDirExists(status.StateDir),
		SessionsDirExists: fileOrDirExists(status.SessionsDir),
		StateDBExists:     fileOrDirExists(status.StateDBPath),
		AuthPathExists:    fileOrDirExists(status.AuthPath),
	}

	if entries, err := os.ReadDir(summary.SkillsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				summary.SkillCount++
			}
		}
	}
	if entries, err := os.ReadDir(summary.ProfilesDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				summary.ProfileCount++
			}
		}
	}

	logFiles := listHermesLogFiles(status)
	summary.LogFileCount = len(logFiles)

	_ = filepath.WalkDir(status.HomeDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := strings.ToLower(d.Name())
			switch base {
			case ".git", "venv", ".venv", "node_modules", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}

		lowerName := strings.ToLower(d.Name())
		lowerPath := strings.ToLower(path)
		switch filepath.Ext(lowerName) {
		case ".db", ".sqlite", ".sqlite3":
			if strings.Contains(lowerName, "state") || strings.Contains(lowerName, "session") || strings.Contains(lowerName, "history") || strings.Contains(lowerName, "memory") {
				summary.SQLiteStoreCount++
				if len(summary.DetectedSQLiteStores) < 12 {
					summary.DetectedSQLiteStores = append(summary.DetectedSQLiteStores, path)
				}
			}
		case ".jsonl":
			if strings.Contains(lowerPath, "session") || strings.Contains(lowerPath, "history") || strings.Contains(lowerPath, "conversation") {
				summary.SessionArtifactCount++
				if len(summary.DetectedSessionArtifacts) < 12 {
					summary.DetectedSessionArtifacts = append(summary.DetectedSessionArtifacts, path)
				}
			}
		}
		return nil
	})

	return summary
}

func listHermesLogFiles(status HermesStatus) []HermesLogFileSummary {
	logsDir := hermesLogsDir(status.HomeDir)
	candidates := make([]HermesLogFileSummary, 0)
	addFiles := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			lowerName := strings.ToLower(name)
			if !strings.HasSuffix(lowerName, ".log") && !strings.HasSuffix(lowerName, ".txt") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			candidates = append(candidates, HermesLogFileSummary{
				Name:       name,
				Path:       filepath.Join(dir, name),
				Size:       info.Size(),
				ModifiedAt: info.ModTime().Format(time.RFC3339),
			})
		}
	}

	addFiles(logsDir)
	if len(candidates) == 0 {
		addFiles(status.HomeDir)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ModifiedAt > candidates[j].ModifiedAt
	})
	return candidates
}

func resolveHermesLogSelection(files []HermesLogFileSummary, selected string) string {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		if len(files) > 0 {
			return files[0].Path
		}
		return ""
	}
	for _, file := range files {
		if file.Path == selected || file.Name == selected {
			return file.Path
		}
	}
	return ""
}

func readLastLines(filePath string, limit int) []string {
	if limit <= 0 {
		limit = 120
	}
	f, err := os.Open(filePath)
	if err != nil {
		return []string{}
	}
	defer f.Close()

	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) > limit {
			lines = lines[len(lines)-limit:]
		}
	}
	return lines
}

func detectHermesWarnings(status HermesStatus, data HermesDataSummary) []string {
	warnings := make([]string, 0, 8)
	switch {
	case !status.Installed:
		warnings = append(warnings, "Hermes 未安装")
	case !status.Configured:
		warnings = append(warnings, "Hermes 已安装但尚未初始化配置")
	}
	if status.Installed && !status.Running {
		warnings = append(warnings, "Hermes 当前未运行")
	}
	if status.Installed && status.Running && !status.GatewayRunning {
		warnings = append(warnings, "Hermes 主进程已运行，但未检测到消息网关")
	}
	if status.Installed && strings.TrimSpace(status.PythonVersion) == "" {
		warnings = append(warnings, "未检测到 Python 运行时")
	}
	if data.LogsDirExists && data.LogFileCount == 0 {
		warnings = append(warnings, "检测到 Hermes 日志目录，但暂未发现日志文件")
	}
	return warnings
}

func checkHermesState(status HermesStatus, configState HermesConfigState, data HermesDataSummary, platforms HermesPlatformsSnapshot, doctor *HermesDoctorSnapshot) ([]ConfigIssue, int) {
	issues := make([]ConfigIssue, 0, 16)
	checked := 0

	checked++
	if !status.Installed {
		issues = append(issues, ConfigIssue{
			ID:          "hermes-not-installed",
			Severity:    "error",
			Component:   "hermes",
			Title:       "Hermes 未安装",
			Description: "当前未检测到 Hermes 可执行文件，请先安装 Hermes Agent",
			Fixable:     false,
			CurrentVal:  "(未检测到 hermes 命令)",
			ExpectedVal: "hermes 命令可用",
			FilePath:    status.BinaryPath,
		})
	}

	checked++
	if status.Installed && !status.Configured {
		issues = append(issues, ConfigIssue{
			ID:          "hermes-no-config",
			Severity:    "warning",
			Component:   "hermes",
			Title:       "Hermes 尚未初始化配置",
			Description: "Hermes 已安装，但尚未检测到 config.yaml / .env / state 目录",
			Fixable:     true,
			CurrentVal:  "(未初始化)",
			ExpectedVal: status.ConfigPath,
			FilePath:    status.HomeDir,
		})
	}

	checked++
	if status.Installed && strings.TrimSpace(status.PythonVersion) == "" {
		issues = append(issues, ConfigIssue{
			ID:          "hermes-no-python",
			Severity:    "error",
			Component:   "hermes",
			Title:       "未检测到 Python 运行时",
			Description: "Hermes 安装和运行依赖 Python 3.11+",
			Fixable:     false,
			CurrentVal:  "(未检测到)",
			ExpectedVal: "Python 3.11+",
		})
	}

	checked++
	if configState.Files.ConfigYAML != "" {
		if _, err := parseHermesYAMLFileWithErr(configState.Files.ConfigYAML); err != nil {
			issues = append(issues, ConfigIssue{
				ID:          "hermes-invalid-yaml",
				Severity:    "error",
				Component:   "hermes",
				Title:       "Hermes config.yaml 不是有效 YAML",
				Description: err.Error(),
				Fixable:     false,
				FilePath:    status.ConfigPath,
			})
		}
	}

	checked++
	if status.Installed && !data.StateDirExists {
		issues = append(issues, ConfigIssue{
			ID:          "hermes-missing-state-dir",
			Severity:    "warning",
			Component:   "hermes",
			Title:       "Hermes state 目录不存在",
			Description: "未检测到 Hermes state 目录，后续会话和状态持久化可能不可用",
			Fixable:     true,
			FilePath:    status.StateDir,
		})
	}

	checked++
	if status.Installed && !data.LogsDirExists {
		issues = append(issues, ConfigIssue{
			ID:          "hermes-missing-logs-dir",
			Severity:    "warning",
			Component:   "hermes",
			Title:       "Hermes 日志目录不存在",
			Description: "未检测到 Hermes 日志目录，无法提供日志回溯",
			Fixable:     true,
			FilePath:    hermesLogsDir(status.HomeDir),
		})
	}

	checked++
	if status.Installed && platforms.EnabledCount > 0 && !status.GatewayRunning {
		issues = append(issues, ConfigIssue{
			ID:          "hermes-gateway-stopped",
			Severity:    "error",
			Component:   "hermes",
			Title:       "Hermes 平台已启用但 Gateway 未运行",
			Description: "检测到至少一个 Hermes 平台已启用，但当前未检测到 Hermes Gateway 进程",
			Fixable:     true,
			CurrentVal:  "gateway stopped",
			ExpectedVal: "gateway running",
		})
	}

	checked++
	if doctor == nil {
		issues = append(issues, ConfigIssue{
			ID:          "hermes-doctor-missing",
			Severity:    "info",
			Component:   "hermes",
			Title:       "尚未生成 Hermes 诊断快照",
			Description: "建议先执行一次 Hermes doctor，生成后端可消费的结构化诊断结果",
			Fixable:     true,
		})
	}

	for _, platform := range platforms.Platforms {
		checked++
		if platform.Configured && len(platform.MissingEnvKeys) > 0 {
			issues = append(issues, ConfigIssue{
				ID:          "hermes-platform-" + platform.ID + "-missing-env",
				Severity:    "error",
				Component:   "hermes",
				Title:       fmt.Sprintf("Hermes 平台 %s 缺少必要环境变量", platform.Label),
				Description: "缺失变量: " + strings.Join(platform.MissingEnvKeys, ", "),
				Fixable:     false,
				FilePath:    status.EnvPath,
			})
		}
		checked++
		if platform.RuntimeStatus == "error" && platform.LastError != "" {
			issues = append(issues, ConfigIssue{
				ID:          "hermes-platform-" + platform.ID + "-runtime-error",
				Severity:    "warning",
				Component:   "hermes",
				Title:       fmt.Sprintf("Hermes 平台 %s 检测到运行异常", platform.Label),
				Description: platform.LastError,
				Fixable:     false,
			})
		}
	}

	return issues, checked
}

func analyzeHermesPlatformEvidence(platformID string, status HermesStatus) (runtimeStatus string, lastEvidence string, lastError string) {
	keywords := map[string][]string{
		"telegram": {"telegram", "tg"},
		"discord":  {"discord"},
		"slack":    {"slack"},
		"weixin":   {"weixin", "wechat", "we chat", "wx"},
		"wecom":    {"wecom", "work wechat", "qywx", "enterprise wechat"},
		"whatsapp": {"whatsapp"},
		"signal":   {"signal"},
		"webhook":  {"webhook"},
		"feishu":   {"feishu", "lark"},
		"dingtalk": {"dingtalk", "dingtalk"},
		"matrix":   {"matrix"},
		"sms":      {"sms", "twilio"},
	}
	logFiles := listHermesLogFiles(status)
	matchers := keywords[platformID]
	if len(matchers) == 0 {
		matchers = []string{platformID}
	}
	isMatch := func(line string) bool {
		lower := strings.ToLower(line)
		for _, keyword := range matchers {
			if strings.Contains(lower, keyword) {
				return true
			}
		}
		return false
	}
	successWords := []string{"connected", "ready", "started", "authenticated", "listening", "online"}
	errorWords := []string{"error", "failed", "unauthorized", "invalid", "missing", "disconnect", "exception"}

	for _, file := range logFiles {
		lines := readLastLines(file.Path, 200)
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" || !isMatch(line) {
				continue
			}
			lower := strings.ToLower(line)
			for _, word := range errorWords {
				if strings.Contains(lower, word) {
					return "error", line, line
				}
			}
			for _, word := range successWords {
				if strings.Contains(lower, word) {
					return "healthy", line, ""
				}
			}
			if lastEvidence == "" {
				lastEvidence = line
			}
		}
	}

	if status.GatewayRunning {
		if lastEvidence != "" {
			return "warning", lastEvidence, ""
		}
		return "warning", "Gateway 运行中，但未在最近日志中发现该平台明确活动线索", ""
	}
	return "inactive", "", ""
}

func detectHermesPlatforms(configState HermesConfigState) HermesPlatformsSnapshot {
	status := configState.Status
	envMap := parseHermesEnvFile(configState.Files.EnvFile)
	yamlMap := parseHermesYAMLFile(configState.Files.ConfigYAML)
	result := HermesPlatformsSnapshot{Platforms: make([]HermesPlatformStatus, 0, len(hermesPlatformSpecs))}

	for _, spec := range hermesPlatformSpecs {
		item := HermesPlatformStatus{
			ID:         spec.ID,
			Label:      spec.Label,
			ConfigPath: strings.Join(spec.ConfigPaths, " | "),
		}

		for _, key := range spec.RequiredEnv {
			if value := strings.TrimSpace(envMap[key]); value != "" {
				item.PresentEnvKeys = append(item.PresentEnvKeys, key)
			} else {
				item.MissingEnvKeys = append(item.MissingEnvKeys, key)
			}
		}
		for _, key := range spec.OptionalEnv {
			if value := strings.TrimSpace(envMap[key]); value != "" {
				item.PresentEnvKeys = append(item.PresentEnvKeys, key)
			}
		}

		configConfigured := false
		configEnabled := false
		for _, path := range spec.ConfigPaths {
			value := hermesNestedValue(yamlMap, path)
			if value == nil {
				continue
			}
			configConfigured = true
			if truthyHermesValue(value) {
				configEnabled = true
				break
			}
			if valueMap, ok := value.(map[string]interface{}); ok {
				if len(valueMap) > 0 {
					configEnabled = true
					if enabledRaw, exists := valueMap["enabled"]; exists {
						configEnabled = truthyHermesValue(enabledRaw)
					}
					break
				}
			}
		}

		item.Configured = configConfigured || len(item.PresentEnvKeys) > 0
		item.Enabled = configEnabled || (len(item.PresentEnvKeys) > 0 && len(item.MissingEnvKeys) == 0)
		item.RuntimeStatus, item.LastEvidence, item.LastError = analyzeHermesPlatformEvidence(spec.ID, status)
		if !item.Configured && (item.LastEvidence != "" || item.LastError != "") {
			item.Configured = true
		}
		if !item.Enabled && status.GatewayRunning && (item.RuntimeStatus == "healthy" || item.RuntimeStatus == "warning" || item.RuntimeStatus == "error") {
			item.Enabled = true
		}
		item.Detail = strings.TrimSpace(strings.Join([]string{
			ternary(configConfigured, "检测到 config.yaml 配置。", ""),
			ternary(len(item.PresentEnvKeys) > 0, "环境变量: "+strings.Join(item.PresentEnvKeys, ", "), ""),
			ternary(len(item.MissingEnvKeys) > 0, "缺失: "+strings.Join(item.MissingEnvKeys, ", "), ""),
			ternary(item.LastError != "", "错误线索: "+item.LastError, ""),
			ternary(item.LastError == "" && item.LastEvidence != "", "最近线索: "+item.LastEvidence, ""),
		}, " "))

		if item.Configured {
			result.ConfiguredCount++
		}
		if item.Enabled {
			result.EnabledCount++
		}
		result.Platforms = append(result.Platforms, item)
	}

	return result
}

func findHermesPlatformSpec(id string) *hermesPlatformSpec {
	id = strings.TrimSpace(strings.ToLower(id))
	for i := range hermesPlatformSpecs {
		if hermesPlatformSpecs[i].ID == id {
			return &hermesPlatformSpecs[i]
		}
	}
	return nil
}

func buildHermesPlatformDetail(configState HermesConfigState, id string) (*HermesPlatformDetail, bool) {
	spec := findHermesPlatformSpec(id)
	if spec == nil {
		return nil, false
	}
	platforms := detectHermesPlatforms(configState)
	var status HermesPlatformStatus
	found := false
	for _, item := range platforms.Platforms {
		if item.ID == spec.ID {
			status = item
			found = true
			break
		}
	}
	if !found {
		return nil, false
	}

	rawConfig := parseHermesYAMLFile(configState.Files.ConfigYAML)
	envMap := parseHermesEnvFile(configState.Files.EnvFile)
	configBlock := map[string]interface{}{}
	for _, path := range spec.ConfigPaths {
		value := hermesNestedValue(rawConfig, path)
		if block, ok := value.(map[string]interface{}); ok {
			configBlock = cloneHermesMap(block)
			break
		}
		if value != nil {
			configBlock = map[string]interface{}{"value": value}
			break
		}
	}

	envValues := map[string]string{}
	for _, key := range append(append([]string{}, spec.RequiredEnv...), spec.OptionalEnv...) {
		if value, ok := envMap[key]; ok {
			envValues[key] = value
		}
	}

	return &HermesPlatformDetail{
		Status:      status,
		Config:      configBlock,
		Environment: envValues,
	}, true
}

func ensureHermesBootstrap(status HermesStatus) error {
	if err := os.MkdirAll(status.HomeDir, 0755); err != nil {
		return err
	}
	for _, dir := range []string{status.StateDir, hermesLogsDir(status.HomeDir), hermesSkillsDir(status.HomeDir), hermesProfilesDir(status.HomeDir)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if !fileOrDirExists(status.ConfigPath) {
		raw, err := yaml.Marshal(map[string]interface{}{
			"model":       map[string]interface{}{},
			"gateway":     map[string]interface{}{},
			"tools":       map[string]interface{}{},
			"memory":      map[string]interface{}{},
			"personality": map[string]interface{}{},
			"profiles":    map[string]interface{}{},
		})
		if err != nil {
			return err
		}
		if err := os.WriteFile(status.ConfigPath, raw, 0644); err != nil {
			return err
		}
	}
	if !fileOrDirExists(status.EnvPath) {
		if err := os.WriteFile(status.EnvPath, []byte("# Hermes environment variables\n"), 0600); err != nil {
			return err
		}
	}
	return nil
}

func fixHermesIssue(id string, cfg *config.Config) error {
	status := detectHermesStatus(cfg)
	switch id {
	case "hermes-no-config":
		return ensureHermesBootstrap(status)
	case "hermes-missing-state-dir":
		return os.MkdirAll(status.StateDir, 0755)
	case "hermes-missing-logs-dir":
		return os.MkdirAll(hermesLogsDir(status.HomeDir), 0755)
	case "hermes-gateway-stopped":
		_, err := runHermesCommandLines(90*time.Second, "gateway", "restart")
		return err
	case "hermes-doctor-missing":
		_, err := runHermesDoctorAndPersistSnapshot(cfg, false)
		return err
	default:
		return fmt.Errorf("unsupported Hermes fix: %s", id)
	}
}

func buildHermesHealthSnapshot(status HermesStatus, data HermesDataSummary, logs []HermesLogFileSummary, taskSummary HermesTaskSummary) HermesHealthSnapshot {
	checks := []HermesHealthCheck{
		{
			ID:      "install",
			Title:   "安装状态",
			Status:  boolToHealth(status.Installed),
			Detail:  valueOr(status.BinaryPath, "未检测到 hermes 命令"),
			FixHint: "先通过软件安装入口或官方 install.sh 安装 Hermes",
		},
		{
			ID:      "config",
			Title:   "配置状态",
			Status:  boolToHealth(status.Configured),
			Detail:  valueOr(status.ConfigPath, "未发现 config.yaml / .env / state"),
			FixHint: "运行 hermes setup 初始化配置",
		},
		{
			ID:      "runtime",
			Title:   "运行状态",
			Status:  boolToHealth(status.Running),
			Detail:  ternary(status.GatewayRunning, "检测到 Hermes Gateway 运行中", ternary(status.Running, "检测到 Hermes 进程运行中", "未检测到 Hermes 进程")),
			FixHint: "可通过 Hermes 动作执行 gateway start / restart",
		},
		{
			ID:      "python",
			Title:   "Python 环境",
			Status:  boolToHealth(strings.TrimSpace(status.PythonVersion) != ""),
			Detail:  valueOr(status.PythonVersion, "未检测到 Python"),
			FixHint: "Hermes 安装与运行依赖 Python 3.11+",
		},
		{
			ID:      "logs",
			Title:   "日志输出",
			Status:  ternaryStatus(data.LogsDirExists && len(logs) > 0, "healthy", ternaryStatus(data.LogsDirExists, "warning", "error")),
			Detail:  ternary(data.LogsDirExists, valueOr(joinTopNames(logs, 3), "日志目录存在但暂无日志文件"), "未检测到日志目录"),
			FixHint: "执行 Hermes 动作后再观察日志目录是否有输出",
		},
		{
			ID:      "storage",
			Title:   "存储快照",
			Status:  ternaryStatus(data.SQLiteStoreCount > 0 || data.SessionArtifactCount > 0, "healthy", ternaryStatus(data.StateDirExists, "warning", "error")),
			Detail:  formatHermesStorageDetail(data),
			FixHint: "先让 Hermes 完成一次真实对话或运行一次 gateway，再观察状态目录",
		},
	}
	return HermesHealthSnapshot{
		Summary: taskSummary,
		Checks:  checks,
	}
}

func boolToHealth(v bool) string {
	if v {
		return "healthy"
	}
	return "error"
}

func ternaryStatus(cond bool, a string, b string) string {
	if cond {
		return a
	}
	return b
}

func ternary[T any](cond bool, a T, b T) T {
	if cond {
		return a
	}
	return b
}

func valueOr(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func joinTopNames(files []HermesLogFileSummary, limit int) string {
	names := make([]string, 0, limit)
	for _, file := range files {
		if len(names) >= limit {
			break
		}
		names = append(names, file.Name)
	}
	return strings.Join(names, ", ")
}

func formatHermesStorageDetail(data HermesDataSummary) string {
	return strings.TrimSpace(strings.Join([]string{
		ternary(data.StateDirExists, "state 目录已存在。", "state 目录不存在。"),
		ternary(data.StateDBExists, "state.db 已存在。", "state.db 不存在。"),
		ternary(data.SessionsDirExists, "sessions 目录已存在。", "sessions 目录不存在。"),
		"SQLite: " + strconv.Itoa(data.SQLiteStoreCount),
		"会话产物: " + strconv.Itoa(data.SessionArtifactCount),
		"技能目录: " + strconv.Itoa(data.SkillCount),
	}, " "))
}

func openHermesStateDB(status HermesStatus) (*sql.DB, error) {
	if !fileOrDirExists(status.StateDBPath) {
		return nil, os.ErrNotExist
	}
	return sql.Open("sqlite", status.StateDBPath+"?_pragma=busy_timeout(5000)&mode=ro")
}

func inspectHermesSessionDB(status HermesStatus) HermesSessionDBSnapshot {
	snapshot := HermesSessionDBSnapshot{
		Path:   status.StateDBPath,
		Exists: fileOrDirExists(status.StateDBPath),
		Tables: []HermesDBTableSummary{},
	}
	if !snapshot.Exists {
		return snapshot
	}

	db, err := openHermesStateDB(status)
	if err != nil {
		return snapshot
	}
	defer db.Close()

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		return snapshot
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		table := HermesDBTableSummary{Name: name, Columns: []string{}}
		columnRows, err := db.Query(`PRAGMA table_info("` + strings.ReplaceAll(name, `"`, `""`) + `")`)
		if err == nil {
			for columnRows.Next() {
				var cid int
				var colName string
				var colType string
				var notNull int
				var defaultValue interface{}
				var pk int
				if err := columnRows.Scan(&cid, &colName, &colType, &notNull, &defaultValue, &pk); err == nil {
					table.Columns = append(table.Columns, colName)
				}
			}
			columnRows.Close()
		}

		countQuery := `SELECT COUNT(*) FROM "` + strings.ReplaceAll(name, `"`, `""`) + `"`
		_ = db.QueryRow(countQuery).Scan(&table.RowCount)
		snapshot.Tables = append(snapshot.Tables, table)

		lowerName := strings.ToLower(name)
		if strings.Contains(lowerName, "session") || strings.Contains(lowerName, "conversation") || strings.Contains(lowerName, "history") {
			snapshot.SessionTableCandidates = append(snapshot.SessionTableCandidates, name)
		}

		columnSet := map[string]struct{}{}
		for _, col := range table.Columns {
			columnSet[strings.ToLower(col)] = struct{}{}
		}
		hasInput := hasHermesColumn(columnSet, "input_tokens", "prompt_tokens", "input")
		hasOutput := hasHermesColumn(columnSet, "output_tokens", "completion_tokens", "output")
		hasTotal := hasHermesColumn(columnSet, "total_tokens", "tokens_total")
		if hasInput || hasOutput || hasTotal {
			sums := hermesTokenSumsForTable(db, name, columnSet)
			snapshot.Usage.InputTokens += sums.InputTokens
			snapshot.Usage.OutputTokens += sums.OutputTokens
			snapshot.Usage.TotalTokens += sums.TotalTokens
			snapshot.Usage.Rows += sums.Rows
		}
	}
	return snapshot
}

func hasHermesColumn(columns map[string]struct{}, names ...string) bool {
	for _, name := range names {
		if _, ok := columns[name]; ok {
			return true
		}
	}
	return false
}

func pickHermesColumn(columns map[string]struct{}, names ...string) string {
	for _, name := range names {
		if _, ok := columns[name]; ok {
			return name
		}
	}
	return ""
}

func hermesTokenSumsForTable(db *sql.DB, table string, columns map[string]struct{}) HermesUsageSummary {
	inputCol := pickHermesColumn(columns, "input_tokens", "prompt_tokens", "input")
	outputCol := pickHermesColumn(columns, "output_tokens", "completion_tokens", "output")
	totalCol := pickHermesColumn(columns, "total_tokens", "tokens_total")
	if inputCol == "" && outputCol == "" && totalCol == "" {
		return HermesUsageSummary{}
	}

	selectParts := []string{}
	if inputCol != "" {
		selectParts = append(selectParts, `COALESCE(SUM("`+inputCol+`"),0)`)
	} else {
		selectParts = append(selectParts, `0`)
	}
	if outputCol != "" {
		selectParts = append(selectParts, `COALESCE(SUM("`+outputCol+`"),0)`)
	} else {
		selectParts = append(selectParts, `0`)
	}
	if totalCol != "" {
		selectParts = append(selectParts, `COALESCE(SUM("`+totalCol+`"),0)`)
	} else {
		selectParts = append(selectParts, `0`)
	}
	selectParts = append(selectParts, `COUNT(*)`)

	query := `SELECT ` + strings.Join(selectParts, ", ") + ` FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`
	var summary HermesUsageSummary
	_ = db.QueryRow(query).Scan(&summary.InputTokens, &summary.OutputTokens, &summary.TotalTokens, &summary.Rows)
	if summary.TotalTokens == 0 {
		summary.TotalTokens = summary.InputTokens + summary.OutputTokens
	}
	return summary
}

func hermesDoctorSnapshotPath(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return filepath.Join(cfg.DataDir, "hermes-doctor-snapshot.json")
}

func readHermesDoctorSnapshot(cfg *config.Config) *HermesDoctorSnapshot {
	path := hermesDoctorSnapshotPath(cfg)
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var snapshot HermesDoctorSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil
	}
	return &snapshot
}

func runHermesCommandLines(timeout time.Duration, args ...string) ([]string, error) {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "hermes", args...)
	cmd.Env = config.BuildExecEnv()
	out, err := cmd.CombinedOutput()
	lines := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		return lines, fmt.Errorf("命令超时: hermes %s", strings.Join(args, " "))
	}
	if err != nil {
		return lines, err
	}
	return lines, nil
}

func runHermesDoctorAndPersistSnapshot(cfg *config.Config, fix bool) (*HermesDoctorSnapshot, error) {
	args := []string{"doctor"}
	if fix {
		args = append(args, "--fix")
	}
	lines, err := runHermesCommandLines(120*time.Second, args...)
	status := detectHermesStatus(cfg)
	data := detectHermesDataSummary(status)
	logFiles := listHermesLogFiles(status)
	platforms := detectHermesPlatforms(buildHermesConfigState(cfg))
	snapshot := buildHermesDoctorSnapshot(status, data, logFiles, platforms, HermesTaskSummary{ByStatus: map[string]int{}}, nil, fix)
	snapshot.RawLines = lines
	if err != nil {
		snapshot.Error = err.Error()
	}
	writeHermesDoctorSnapshot(cfg, snapshot)
	return &snapshot, err
}

func listHermesProfiles(status HermesStatus) []HermesProfileFile {
	entries, err := os.ReadDir(hermesProfilesDir(status.HomeDir))
	if err != nil {
		return []HermesProfileFile{}
	}
	result := make([]HermesProfileFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".txt") && !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, HermesProfileFile{
			Name:       name,
			Path:       filepath.Join(hermesProfilesDir(status.HomeDir), name),
			Exists:     true,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Format(time.RFC3339),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func sanitizeHermesProfileName(name string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	if name == "." || name == "" {
		return ""
	}
	return name
}

func buildHermesPersonalityState(status HermesStatus) HermesPersonalityState {
	soulPath := hermesSoulPath(status.HomeDir)
	state := HermesPersonalityState{
		SoulPath:    soulPath,
		SoulExists:  fileOrDirExists(soulPath),
		SoulContent: readTextFile(soulPath),
		Profiles:    listHermesProfiles(status),
	}
	return state
}

func writeHermesDoctorSnapshot(cfg *config.Config, snapshot HermesDoctorSnapshot) {
	path := hermesDoctorSnapshotPath(cfg)
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0644)
}

func buildHermesDoctorSnapshot(status HermesStatus, data HermesDataSummary, logs []HermesLogFileSummary, platforms HermesPlatformsSnapshot, taskSummary HermesTaskSummary, task *taskman.Task, fixApplied bool) HermesDoctorSnapshot {
	taskID := ""
	taskStatus := ""
	taskErr := ""
	rawLines := []string{}
	if task != nil {
		taskID = task.ID
		taskStatus = string(task.Status)
		taskErr = task.Error
		rawLines = append(rawLines, task.Log...)
	}
	statusLines := []string{}
	if out := detectCmd("hermes", "status"); strings.TrimSpace(out) != "" {
		for _, line := range strings.Split(out, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				statusLines = append(statusLines, trimmed)
			}
		}
	}
	health := buildHermesHealthSnapshot(status, data, logs, taskSummary)
	return HermesDoctorSnapshot{
		UpdatedAt:   time.Now().Format(time.RFC3339),
		FixApplied:  fixApplied,
		TaskID:      taskID,
		TaskStatus:  taskStatus,
		Error:       taskErr,
		RawLines:    rawLines,
		StatusLines: statusLines,
		Health:      health,
		Platforms:   platforms,
		Warnings:    detectHermesWarnings(status, data),
	}
}

func scanHermesStorage(status HermesStatus) HermesStorageSnapshot {
	dataSummary := detectHermesDataSummary(status)
	dbSnapshot := inspectHermesSessionDB(status)
	result := HermesStorageSnapshot{
		SQLiteStores:      append([]string(nil), dataSummary.DetectedSQLiteStores...),
		SessionArtifacts:  append([]string(nil), dataSummary.DetectedSessionArtifacts...),
		ConversationFiles: []string{},
		PreviewSessions:   []HermesSessionItem{},
		DB:                dbSnapshot,
		Usage:             dbSnapshot.Usage,
	}
	if dbSnapshot.Exists && dbSnapshot.Path != "" {
		hasStateDB := false
		for _, path := range result.SQLiteStores {
			if path == dbSnapshot.Path {
				hasStateDB = true
				break
			}
		}
		if !hasStateDB {
			result.SQLiteStores = append(result.SQLiteStores, dbSnapshot.Path)
		}
	}
	seenConversations := map[string]struct{}{}
	seenArtifacts := map[string]struct{}{}
	_ = filepath.WalkDir(status.HomeDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := strings.ToLower(d.Name())
			switch base {
			case ".git", "venv", ".venv", "node_modules", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}

		lowerPath := strings.ToLower(path)
		lowerName := strings.ToLower(d.Name())
		if strings.HasSuffix(lowerName, ".jsonl") && (strings.Contains(lowerPath, "conversation") || strings.Contains(lowerPath, "history")) {
			if _, ok := seenConversations[path]; !ok {
				seenConversations[path] = struct{}{}
				result.ConversationFiles = append(result.ConversationFiles, path)
			}
		}
		if strings.HasSuffix(lowerName, ".jsonl") && (strings.Contains(lowerPath, "session") || strings.Contains(lowerPath, "history") || strings.Contains(lowerPath, "conversation")) {
			if _, ok := seenArtifacts[path]; !ok {
				seenArtifacts[path] = struct{}{}
				result.SessionArtifacts = append(result.SessionArtifacts, path)
			}
		}
		return nil
	})

	sort.Strings(result.SQLiteStores)
	sort.Strings(result.ConversationFiles)
	sort.Strings(result.SessionArtifacts)
	result.ConversationCount = len(result.ConversationFiles)
	result.SessionArtifactCount = len(result.SessionArtifacts)

	previewTargets := result.ConversationFiles
	if len(previewTargets) == 0 {
		previewTargets = result.SessionArtifacts
	}
	for _, path := range previewTargets {
		if len(result.PreviewSessions) >= 10 {
			break
		}
		if session := buildHermesSessionPreview(path); session != nil {
			result.PreviewSessions = append(result.PreviewSessions, *session)
		}
	}
	return result
}

func hermesSessionID(path string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(path))
}

func buildHermesSessionPreview(path string) *HermesSessionItem {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	item := &HermesSessionItem{
		ID:        hermesSessionID(path),
		Title:     filepath.Base(path),
		Path:      path,
		UpdatedAt: info.ModTime().Format(time.RFC3339),
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		msg := parseHermesSessionMessage(raw)
		if msg == nil {
			continue
		}
		item.MessageCount++
		item.RecentMessages = append(item.RecentMessages, *msg)
		if len(item.RecentMessages) > 5 {
			item.RecentMessages = item.RecentMessages[len(item.RecentMessages)-5:]
		}
		if item.Title == filepath.Base(path) && msg.Content != "" {
			item.Title = truncateHermesTitle(msg.Content)
		}
	}
	return item
}

func listHermesSessions(status HermesStatus, limit int) []HermesSessionItem {
	storage := scanHermesStorage(status)
	targets := storage.ConversationFiles
	if len(targets) == 0 {
		targets = storage.SessionArtifacts
	}
	result := make([]HermesSessionItem, 0, len(targets))
	for _, path := range targets {
		if session := buildHermesSessionPreview(path); session != nil {
			result = append(result, *session)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt > result[j].UpdatedAt
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func resolveHermesSessionPath(status HermesStatus, id string) string {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(id))
	if err != nil {
		return ""
	}
	path := filepath.Clean(string(raw))
	if !filepath.IsAbs(path) {
		return ""
	}
	home := filepath.Clean(status.HomeDir)
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return ""
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

func readHermesSessionMessages(path string, limit int) []HermesSessionMessage {
	if limit <= 0 {
		limit = 200
	}
	f, err := os.Open(path)
	if err != nil {
		return []HermesSessionMessage{}
	}
	defer f.Close()

	messages := make([]HermesSessionMessage, 0, limit)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		msg := parseHermesSessionMessage(raw)
		if msg == nil {
			continue
		}
		messages = append(messages, *msg)
		if len(messages) > limit {
			messages = messages[len(messages)-limit:]
		}
	}
	return messages
}

func parseHermesSessionMessage(raw map[string]interface{}) *HermesSessionMessage {
	role := strings.TrimSpace(anyToString(raw["role"]))
	if role == "" {
		role = strings.TrimSpace(anyToString(raw["speaker"]))
	}
	content := strings.TrimSpace(anyToString(raw["content"]))
	if content == "" {
		if payload, ok := raw["message"].(map[string]interface{}); ok {
			role = valueOr(strings.TrimSpace(anyToString(payload["role"])), role)
			content = strings.TrimSpace(anyToString(payload["content"]))
		}
	}
	if content == "" {
		return nil
	}
	return &HermesSessionMessage{
		Role:      valueOr(role, "assistant"),
		Content:   content,
		Timestamp: strings.TrimSpace(anyToString(raw["timestamp"])),
	}
}

func truncateHermesTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "Conversation"
	}
	runes := []rune(content)
	if len(runes) > 48 {
		return string(runes[:48]) + "..."
	}
	return content
}

func anyToString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func isHermesTask(task *taskman.Task) bool {
	if task == nil {
		return false
	}
	if task.Type == "install_hermes" {
		return true
	}
	return strings.HasPrefix(task.Type, "hermes_")
}

func collectHermesTasks(tm *taskman.Manager) ([]*taskman.Task, HermesTaskSummary) {
	summary := HermesTaskSummary{ByStatus: map[string]int{}}
	if tm == nil {
		return []*taskman.Task{}, summary
	}
	all := tm.GetAllTasks()
	result := make([]*taskman.Task, 0, len(all))
	for _, task := range all {
		if !isHermesTask(task) {
			continue
		}
		result = append(result, task)
		summary.Total++
		summary.ByStatus[string(task.Status)]++
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	if len(result) > 50 {
		result = result[:50]
	}
	return result, summary
}

func fileOrDirExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func hermesActionCatalog() []HermesActionSpec {
	return []HermesActionSpec{
		{ID: "setup", Label: "Hermes Setup", Description: "运行 Hermes 初始化向导", Command: "hermes setup"},
		{ID: "doctor", Label: "Hermes Doctor", Description: "诊断 Hermes 运行环境与依赖", Command: "hermes doctor"},
		{ID: "update", Label: "Hermes Update", Description: "更新 Hermes 到最新版本", Command: "hermes update"},
		{ID: "gateway-install", Label: "Hermes Gateway Install", Description: "安装 Hermes 消息网关依赖", Command: "hermes gateway install"},
		{ID: "gateway-start", Label: "Hermes Gateway Start", Description: "启动 Hermes 消息网关", Command: "hermes gateway start"},
		{ID: "gateway-stop", Label: "Hermes Gateway Stop", Description: "停止 Hermes 消息网关", Command: "hermes gateway stop"},
		{ID: "gateway-restart", Label: "Hermes Gateway Restart", Description: "重启 Hermes 消息网关", Command: "hermes gateway restart"},
		{ID: "claw-migrate", Label: "Migrate From OpenClaw", Description: "从 OpenClaw 迁移配置与数据到 Hermes", Command: "hermes claw migrate"},
	}
}

func buildHermesActionScript(action string) (taskName string, script string, ok bool) {
	switch action {
	case "setup":
		return "Hermes Setup", `set -e
export HOME="${HOME:-$(cd ~ && pwd)}"
export PATH="$HOME/.local/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
hermes setup`, true
	case "doctor":
		return "Hermes Doctor", `set -e
export HOME="${HOME:-$(cd ~ && pwd)}"
export PATH="$HOME/.local/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
hermes doctor`, true
	case "update":
		return "Hermes Update", `set -e
export HOME="${HOME:-$(cd ~ && pwd)}"
export PATH="$HOME/.local/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
hermes update`, true
	case "gateway-install":
		return "Hermes Gateway Install", `set -e
export HOME="${HOME:-$(cd ~ && pwd)}"
export PATH="$HOME/.local/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
hermes gateway install`, true
	case "gateway-start":
		return "Hermes Gateway Start", `set -e
export HOME="${HOME:-$(cd ~ && pwd)}"
export PATH="$HOME/.local/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
hermes gateway start`, true
	case "gateway-stop":
		return "Hermes Gateway Stop", `set -e
export HOME="${HOME:-$(cd ~ && pwd)}"
export PATH="$HOME/.local/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
hermes gateway stop`, true
	case "gateway-restart":
		return "Hermes Gateway Restart", `set -e
export HOME="${HOME:-$(cd ~ && pwd)}"
export PATH="$HOME/.local/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
hermes gateway restart`, true
	case "claw-migrate":
		return "Hermes OpenClaw Migration", `set -e
export HOME="${HOME:-$(cd ~ && pwd)}"
export PATH="$HOME/.local/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
hermes claw migrate`, true
	default:
		return "", "", false
	}
}

func GetHermesStatus(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": detectHermesStatus(cfgs...)})
	}
}

func GetHermesPersonality(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		c.JSON(http.StatusOK, gin.H{"ok": true, "personality": buildHermesPersonalityState(status)})
	}
}

func SaveHermesPersonality(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SoulContent string `json:"soulContent"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		status := detectHermesStatus(cfgs...)
		if err := os.MkdirAll(status.HomeDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "创建 Hermes home 目录失败: " + err.Error()})
			return
		}
		if err := os.WriteFile(hermesSoulPath(status.HomeDir), []byte(req.SoulContent), 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "写入 SOUL.md 失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "personality": buildHermesPersonalityState(status)})
	}
}

func GetHermesProfiles(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		c.JSON(http.StatusOK, gin.H{"ok": true, "profiles": listHermesProfiles(status)})
	}
}

func GetHermesProfileDetail(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		name := sanitizeHermesProfileName(c.Param("name"))
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "profile name required"})
			return
		}
		path := filepath.Join(hermesProfilesDir(status.HomeDir), name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "Hermes profile not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "profile": HermesProfileFile{
			Name:       name,
			Path:       path,
			Exists:     true,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Format(time.RFC3339),
			Content:    readTextFile(path),
		}})
	}
}

func SaveHermesProfileDetail(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		name := sanitizeHermesProfileName(c.Param("name"))
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "profile name required"})
			return
		}
		var req struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		dir := hermesProfilesDir(status.HomeDir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "创建 profiles 目录失败: " + err.Error()})
			return
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "写入 profile 失败: " + err.Error()})
			return
		}
		info, _ := os.Stat(path)
		c.JSON(http.StatusOK, gin.H{"ok": true, "profile": HermesProfileFile{
			Name:       name,
			Path:       path,
			Exists:     true,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Format(time.RFC3339),
			Content:    readTextFile(path),
		}})
	}
}

func CheckHermes(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfg)
		configState := buildHermesConfigState(cfg)
		data := detectHermesDataSummary(status)
		platforms := detectHermesPlatforms(configState)
		doctor := readHermesDoctorSnapshot(cfg)
		issues, checked := checkHermesState(status, configState, data, platforms, doctor)
		problems := 0
		for _, issue := range issues {
			if issue.Severity == "error" || issue.Severity == "warning" {
				problems++
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":       true,
			"issues":   issues,
			"checked":  checked,
			"problems": problems,
		})
	}
}

func FixHermes(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			IssueIDs []string `json:"issueIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		fixed := []string{}
		failed := []string{}
		for _, issueID := range req.IssueIDs {
			if err := fixHermesIssue(strings.TrimSpace(issueID), cfg); err != nil {
				failed = append(failed, issueID+": "+err.Error())
			} else {
				fixed = append(fixed, issueID)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":     true,
			"fixed":  fixed,
			"failed": failed,
		})
	}
}

func GetHermesStructuredConfig(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		configState := buildHermesConfigState(cfgs...)
		raw := parseHermesYAMLFile(configState.Files.ConfigYAML)
		c.JSON(http.StatusOK, gin.H{"ok": true, "config": buildHermesStructuredConfig(raw)})
	}
}

func SaveHermesStructuredConfig(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Model       map[string]interface{} `json:"model"`
			Gateway     map[string]interface{} `json:"gateway"`
			Session     map[string]interface{} `json:"session"`
			Tools       map[string]interface{} `json:"tools"`
			Memory      map[string]interface{} `json:"memory"`
			Personality map[string]interface{} `json:"personality"`
			Profiles    map[string]interface{} `json:"profiles"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		state := detectHermesStatus(cfgs...)
		if err := os.MkdirAll(state.HomeDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "创建 Hermes 配置目录失败: " + err.Error()})
			return
		}

		raw := parseHermesYAMLFile(readTextFile(state.ConfigPath))
		if raw == nil {
			raw = map[string]interface{}{}
		}
		if req.Model != nil {
			setHermesNestedValue(raw, "model", req.Model)
		}
		if req.Gateway != nil {
			setHermesNestedValue(raw, "gateway", req.Gateway)
		}
		if req.Session != nil {
			for key, value := range req.Session {
				if key == "group_sessions_per_user" || key == "session_search" || key == "session_history_limit" || key == "session_summary_tokens" {
					raw[key] = value
				} else {
					setHermesNestedValue(raw, "session."+key, value)
				}
			}
		}
		if req.Tools != nil {
			setHermesNestedValue(raw, "tools", req.Tools)
		}
		if req.Memory != nil {
			setHermesNestedValue(raw, "memory", req.Memory)
		}
		if req.Personality != nil {
			setHermesNestedValue(raw, "personality", req.Personality)
		}
		if req.Profiles != nil {
			setHermesNestedValue(raw, "profiles", req.Profiles)
		}

		marshaled, err := yaml.Marshal(raw)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "序列化 Hermes 配置失败: " + err.Error()})
			return
		}
		if err := os.WriteFile(state.ConfigPath, marshaled, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "写入 config.yaml 失败: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "config": buildHermesStructuredConfig(parseHermesYAMLFile(readTextFile(state.ConfigPath)))})
	}
}

func GetHermesConfig(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "config": buildHermesConfigState(cfgs...)})
	}
}

func SaveHermesConfig(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ConfigYAML *string `json:"configYaml"`
			EnvFile    *string `json:"envFile"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		state := detectHermesStatus(cfgs...)
		if err := os.MkdirAll(state.HomeDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "创建 Hermes 配置目录失败: " + err.Error()})
			return
		}

		if req.ConfigYAML != nil {
			trimmed := strings.TrimSpace(*req.ConfigYAML)
			if trimmed != "" {
				var parsed any
				if err := yaml.Unmarshal([]byte(trimmed), &parsed); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "config.yaml 不是有效 YAML: " + err.Error()})
					return
				}
			}
			if err := os.WriteFile(state.ConfigPath, []byte(*req.ConfigYAML), 0644); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "写入 config.yaml 失败: " + err.Error()})
				return
			}
		}

		if req.EnvFile != nil {
			if err := os.WriteFile(state.EnvPath, []byte(*req.EnvFile), 0600); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "写入 .env 失败: " + err.Error()})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "config": buildHermesConfigState(cfgs...)})
	}
}

func GetHermesActions() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "actions": hermesActionCatalog()})
	}
}

func GetHermesOverview(cfg *config.Config, tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfg)
		configState := buildHermesConfigState(cfg)
		data := detectHermesDataSummary(status)
		storage := scanHermesStorage(status)
		platforms := detectHermesPlatforms(configState)
		logFiles := listHermesLogFiles(status)
		tasks, taskSummary := collectHermesTasks(tm)
		health := buildHermesHealthSnapshot(status, data, logFiles, taskSummary)
		doctor := readHermesDoctorSnapshot(cfg)
		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"overview": HermesOverview{
				Status:      status,
				Config:      configState,
				Actions:     hermesActionCatalog(),
				Data:        data,
				Health:      health,
				Storage:     storage,
				Platforms:   platforms,
				Doctor:      doctor,
				LogFiles:    logFiles,
				RecentTasks: tasks,
				TaskSummary: taskSummary,
				Warnings:    detectHermesWarnings(status, data),
			},
		})
	}
}

func GetHermesLogs(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		files := listHermesLogFiles(status)
		selectedPath := resolveHermesLogSelection(files, c.Query("path"))
		lines := 120
		if raw := strings.TrimSpace(c.Query("lines")); raw != "" {
			if n, err := parseHermesInt(raw); err == nil && n > 0 && n <= 2000 {
				lines = n
			}
		}
		content := []string{}
		if selectedPath != "" {
			content = readLastLines(selectedPath, lines)
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":           true,
			"files":        files,
			"selectedPath": selectedPath,
			"lines":        content,
		})
	}
}

func parseHermesInt(raw string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(raw))
}

func GetHermesStorage(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		c.JSON(http.StatusOK, gin.H{"ok": true, "storage": scanHermesStorage(status)})
	}
}

func GetHermesUsage(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		storage := scanHermesStorage(status)
		c.JSON(http.StatusOK, gin.H{"ok": true, "usage": storage.Usage, "db": storage.DB})
	}
}

func GetHermesSessions(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		limit := 100
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			if n, err := parseHermesInt(raw); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "sessions": listHermesSessions(status, limit)})
	}
}

func GetHermesSessionDetail(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		path := resolveHermesSessionPath(status, c.Param("id"))
		if path == "" {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "Hermes session not found"})
			return
		}
		limit := 200
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			if n, err := parseHermesInt(raw); err == nil && n > 0 && n <= 2000 {
				limit = n
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":       true,
			"id":       c.Param("id"),
			"path":     path,
			"messages": readHermesSessionMessages(path, limit),
		})
	}
}

func PreviewHermesSession(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Platform string `json:"platform"`
			ChatType string `json:"chatType"`
			ChatID   string `json:"chatId"`
			UserID   string `json:"userId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		preview := previewHermesSession(buildHermesConfigState(cfgs...), req.Platform, req.ChatType, req.ChatID, req.UserID)
		c.JSON(http.StatusOK, gin.H{"ok": true, "preview": preview})
	}
}

func GetHermesHealth(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := detectHermesStatus(cfgs...)
		data := detectHermesDataSummary(status)
		logFiles := listHermesLogFiles(status)
		health := buildHermesHealthSnapshot(status, data, logFiles, HermesTaskSummary{ByStatus: map[string]int{}})
		c.JSON(http.StatusOK, gin.H{"ok": true, "health": health})
	}
}

func GetHermesPlatforms(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		configState := buildHermesConfigState(cfgs...)
		c.JSON(http.StatusOK, gin.H{"ok": true, "platforms": detectHermesPlatforms(configState)})
	}
}

func GetHermesPlatformDetail(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		configState := buildHermesConfigState(cfgs...)
		detail, ok := buildHermesPlatformDetail(configState, c.Param("id"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "Hermes platform not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "platform": detail})
	}
}

func SaveHermesPlatformDetail(cfgs ...*config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		spec := findHermesPlatformSpec(c.Param("id"))
		if spec == nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "Hermes platform not found"})
			return
		}

		var req struct {
			Enabled *bool                  `json:"enabled"`
			Config  map[string]interface{} `json:"config"`
			Env     map[string]string      `json:"env"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		state := detectHermesStatus(cfgs...)
		if err := os.MkdirAll(state.HomeDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "创建 Hermes 配置目录失败: " + err.Error()})
			return
		}

		rawConfig := parseHermesYAMLFile(readTextFile(state.ConfigPath))
		if rawConfig == nil {
			rawConfig = map[string]interface{}{}
		}
		targetPath := spec.ConfigPaths[0]

		var nextBlock interface{}
		if req.Config != nil {
			nextBlock = cloneHermesMap(req.Config)
		} else {
			current := hermesNestedValue(rawConfig, targetPath)
			if block, ok := current.(map[string]interface{}); ok {
				nextBlock = cloneHermesMap(block)
			} else if current != nil {
				nextBlock = current
			} else {
				nextBlock = map[string]interface{}{}
			}
		}

		if req.Enabled != nil {
			if block, ok := nextBlock.(map[string]interface{}); ok {
				block["enabled"] = *req.Enabled
				nextBlock = block
			} else {
				nextBlock = map[string]interface{}{"enabled": *req.Enabled}
			}
		}
		setHermesNestedValue(rawConfig, targetPath, nextBlock)

		marshaled, err := yaml.Marshal(rawConfig)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "序列化 Hermes 平台配置失败: " + err.Error()})
			return
		}
		if err := os.WriteFile(state.ConfigPath, marshaled, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "写入 config.yaml 失败: " + err.Error()})
			return
		}

		if req.Env != nil {
			envMap := parseHermesEnvFile(readTextFile(state.EnvPath))
			for key, value := range req.Env {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				if strings.TrimSpace(value) == "" {
					delete(envMap, key)
				} else {
					envMap[key] = value
				}
			}
			if err := os.WriteFile(state.EnvPath, []byte(renderHermesEnvFile(envMap)), 0600); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "写入 .env 失败: " + err.Error()})
				return
			}
		}

		configState := buildHermesConfigState(cfgs...)
		detail, ok := buildHermesPlatformDetail(configState, spec.ID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "平台保存后重新读取失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "platform": detail})
	}
}

func GetHermesDoctorSnapshot(cfg *config.Config, tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if snapshot := readHermesDoctorSnapshot(cfg); snapshot != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "snapshot": snapshot})
			return
		}
		status := detectHermesStatus(cfg)
		data := detectHermesDataSummary(status)
		logFiles := listHermesLogFiles(status)
		platforms := detectHermesPlatforms(buildHermesConfigState(cfg))
		_, summary := collectHermesTasks(tm)
		snapshot := buildHermesDoctorSnapshot(status, data, logFiles, platforms, summary, nil, false)
		c.JSON(http.StatusOK, gin.H{"ok": true, "snapshot": snapshot})
	}
}

func GetHermesTasks(tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tasks, summary := collectHermesTasks(tm)
		c.JSON(http.StatusOK, gin.H{"ok": true, "tasks": tasks, "summary": summary})
	}
}

func GetHermesTaskDetail(tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "task id required"})
			return
		}
		task := tm.GetTask(id)
		if !isHermesTask(task) {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "Hermes task not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "task": task})
	}
}

func RunHermesDoctor(cfg *config.Config, tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Fix bool `json:"fix"`
		}
		_ = c.ShouldBindJSON(&req)
		if runtime.GOOS == "windows" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "Hermes doctor 暂不支持原生 Windows，请在 WSL2 或 Linux 环境中使用"})
			return
		}
		taskType := "hermes_doctor"
		if req.Fix {
			taskType = "hermes_doctor_fix"
		}
		if tm.HasRunningTask(taskType) {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "Hermes doctor 正在执行中"})
			return
		}
		command := "hermes doctor"
		if req.Fix {
			command = "hermes doctor --fix"
		}
		task := tm.CreateTask("Hermes Doctor", taskType)
		go func() {
			script := "set -e\nexport HOME=\"${HOME:-$(cd ~ && pwd)}\"\nexport PATH=\"$HOME/.local/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH\"\n" + command
			err := tm.RunScript(task, script)
			tm.FinishTask(task, err)

			cloned := tm.GetTask(task.ID)
			status := detectHermesStatus(cfg)
			data := detectHermesDataSummary(status)
			logFiles := listHermesLogFiles(status)
			platforms := detectHermesPlatforms(buildHermesConfigState(cfg))
			_, summary := collectHermesTasks(tm)
			snapshot := buildHermesDoctorSnapshot(status, data, logFiles, platforms, summary, cloned, req.Fix)
			writeHermesDoctorSnapshot(cfg, snapshot)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID})
	}
}

func RunHermesAction(tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Action string `json:"action"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Action) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "action required"})
			return
		}
		action := strings.TrimSpace(req.Action)
		taskName, script, ok := buildHermesActionScript(action)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "unsupported action: " + action})
			return
		}
		if runtime.GOOS == "windows" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "Hermes 动作暂不支持原生 Windows，请在 WSL2 或 Linux 环境中使用"})
			return
		}
		if tm.HasRunningTask("hermes_" + action) {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "该 Hermes 动作正在执行中"})
			return
		}

		task := tm.CreateTask(taskName, "hermes_"+action)
		go func() {
			err := tm.RunScript(task, script)
			tm.FinishTask(task, err)
		}()

		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID})
	}
}
