package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

const panelChatDefaultTitle = "新对话"

var errPanelChatTimeout = errors.New("panel chat timeout")
var errPanelChatCanceled = errors.New("panel chat canceled")

var panelChatTimestampPrefixRe = regexp.MustCompile(`^\[[^\]]+\]\s*`)

const panelChatScannerMaxTokenSize = 16 * 1024 * 1024

var panelChatActiveRuns sync.Map

type panelChatActiveRun struct {
	cancel context.CancelFunc
	pid    int
}

type panelChatSession struct {
	ID                   string            `json:"id"`
	OpenClawSessionID    string            `json:"openclawSessionId"`
	AgentID              string            `json:"agentId"`
	ChatType             string            `json:"chatType"`
	ParticipantAgentIDs  []string          `json:"participantAgentIds,omitempty"`
	ControllerAgentID    string            `json:"controllerAgentId,omitempty"`
	PreferredAgentID     string            `json:"preferredAgentId,omitempty"`
	GroupAgentSessionIDs map[string]string `json:"groupAgentSessionIds,omitempty"`
	Status               string            `json:"status,omitempty"`
	Title                string            `json:"title"`
	TargetID             string            `json:"targetId,omitempty"`
	TargetName           string            `json:"targetName,omitempty"`
	CreatedAt            int64             `json:"createdAt"`
	UpdatedAt            int64             `json:"updatedAt"`
	Processing           bool              `json:"processing,omitempty"`
	MessageCount         int               `json:"messageCount"`
	LastMessage          string            `json:"lastMessage,omitempty"`
}

type panelChatRunResult struct {
	Payloads []struct {
		Text string `json:"text"`
	} `json:"payloads"`
	Meta struct {
		AgentMeta struct {
			SessionID string `json:"sessionId"`
		} `json:"agentMeta"`
	} `json:"meta"`
}

type panelChatCLIResult struct {
	Status string             `json:"status"`
	Result panelChatRunResult `json:"result"`
}

type panelChatTaskMeta struct {
	ID                  string   `json:"id"`
	SessionID           string   `json:"sessionId"`
	ChatType            string   `json:"chatType"`
	ControllerAgentID   string   `json:"controllerAgentId"`
	PreferredAgentID    string   `json:"preferredAgentId"`
	ParticipantAgentIDs []string `json:"participantAgentIds"`
	Title               string   `json:"title"`
	Status              string   `json:"status"`
	CurrentStage        string   `json:"currentStage"`
	UserMessageID       string   `json:"userMessageId,omitempty"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

type panelChatTaskEvent struct {
	Time          string `json:"time"`
	Type          string `json:"type"`
	AgentID       string `json:"agentId,omitempty"`
	TargetAgentID string `json:"targetAgentId,omitempty"`
	Message       string `json:"message,omitempty"`
}

type panelChatSubtask struct {
	AgentID    string `json:"agentId"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	AssignedAt string `json:"assignedAt,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	InputRef   string `json:"inputRef,omitempty"`
	OutputRef  string `json:"outputRef,omitempty"`
	SummaryRef string `json:"summaryRef,omitempty"`
	Summary    string `json:"summary,omitempty"`
}

type panelChatTaskBundle struct {
	TaskID    string                      `json:"taskId"`
	Meta      panelChatTaskMeta           `json:"meta"`
	Spec      string                      `json:"spec"`
	Result    string                      `json:"result,omitempty"`
	Error     string                      `json:"error,omitempty"`
	Timeline  []panelChatTaskEvent        `json:"timeline,omitempty"`
	Subtasks  map[string]panelChatSubtask `json:"subtasks,omitempty"`
	WorkItems []panelChatWorkItem         `json:"workitems,omitempty"`
	Artifacts map[string]string           `json:"artifacts,omitempty"`
}

type panelChatWorkItem struct {
	ID        string `json:"id"`
	AgentID   string `json:"agentId"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Summary   string `json:"summary,omitempty"`
	OutputRef string `json:"outputRef,omitempty"`
}

func panelChatSessionsPath(cfg *config.Config) string {
	return filepath.Join(cfg.DataDir, "panel-chat", "sessions.json")
}

func panelChatGroupMessagesPath(cfg *config.Config, sessionID string) string {
	return filepath.Join(cfg.DataDir, "panel-chat", "groups", sessionID+".json")
}

func panelChatDirectMessagesPath(cfg *config.Config, sessionID string) string {
	return filepath.Join(cfg.DataDir, "panel-chat", "messages", sessionID+".json")
}

func panelChatTaskDir(cfg *config.Config, taskID string) string {
	return filepath.Join(cfg.DataDir, "panel-chat", "tasks", taskID)
}

func panelChatTaskMetaPath(cfg *config.Config, taskID string) string {
	return filepath.Join(panelChatTaskDir(cfg, taskID), "meta.json")
}

func panelChatTaskSpecPath(cfg *config.Config, taskID string) string {
	return filepath.Join(panelChatTaskDir(cfg, taskID), "spec.md")
}

func panelChatTaskResultPath(cfg *config.Config, taskID string) string {
	return filepath.Join(panelChatTaskDir(cfg, taskID), "result.md")
}

func panelChatTaskTimelinePath(cfg *config.Config, taskID string) string {
	return filepath.Join(panelChatTaskDir(cfg, taskID), "timeline.json")
}

func panelChatTaskSubtaskPath(cfg *config.Config, taskID, agentID string) string {
	return filepath.Join(panelChatTaskDir(cfg, taskID), "subtasks", agentID+".json")
}

func panelChatTaskArtifactPath(cfg *config.Config, taskID, agentID string) string {
	return filepath.Join(panelChatTaskDir(cfg, taskID), "artifacts", agentID+".md")
}

func panelChatTaskWorkItemPath(cfg *config.Config, taskID, workItemID string) string {
	return filepath.Join(panelChatTaskDir(cfg, taskID), "workitems", workItemID+".json")
}

func panelChatTaskSummaryPath(cfg *config.Config, taskID, agentID string) string {
	return filepath.Join(panelChatTaskDir(cfg, taskID), "artifacts", agentID+".summary.md")
}

func loadPanelChatGroupMessages(cfg *config.Config, sessionID string) ([]map[string]interface{}, error) {
	path := panelChatGroupMessagesPath(cfg, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}
	var messages []map[string]interface{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return []map[string]interface{}{}, nil
	}
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func loadPanelChatDirectMessages(cfg *config.Config, sessionID string) ([]map[string]interface{}, error) {
	path := panelChatDirectMessagesPath(cfg, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}
	var messages []map[string]interface{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return []map[string]interface{}{}, nil
	}
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func savePanelChatGroupMessages(cfg *config.Config, sessionID string, messages []map[string]interface{}) error {
	path := panelChatGroupMessagesPath(cfg, sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func savePanelChatDirectMessages(cfg *config.Config, sessionID string, messages []map[string]interface{}) error {
	path := panelChatDirectMessagesPath(cfg, sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func createPanelChatTask(cfg *config.Config, session panelChatSession, userMessage string) (string, error) {
	taskID := fmt.Sprintf("task-%d", time.Now().UnixMilli())
	if err := os.MkdirAll(panelChatTaskDir(cfg, taskID), 0o755); err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	meta := panelChatTaskMeta{
		ID:                  taskID,
		SessionID:           session.ID,
		ChatType:            session.ChatType,
		ControllerAgentID:   session.ControllerAgentID,
		PreferredAgentID:    session.PreferredAgentID,
		ParticipantAgentIDs: append([]string{}, session.ParticipantAgentIDs...),
		Title:               buildPanelChatTitle(userMessage),
		Status:              "running",
		CurrentStage:        "user",
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := writePanelChatTaskMeta(cfg, taskID, meta); err != nil {
		return "", err
	}
	if err := writePanelChatTaskSpec(cfg, taskID, session, userMessage); err != nil {
		return "", err
	}
	_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: now, Type: "task_created", AgentID: session.ControllerAgentID, Message: "任务已创建"})
	return taskID, nil
}

func writePanelChatTaskMeta(cfg *config.Config, taskID string, meta panelChatTaskMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(panelChatTaskMetaPath(cfg, taskID), append(data, '\n'), 0o644)
}

func writePanelChatTaskSpec(cfg *config.Config, taskID string, session panelChatSession, userMessage string) error {
	content := fmt.Sprintf("# Task Spec\n\n## User Request\n%s\n\n## Objective\n完成当前群聊任务，并由主控智能体给出最终汇总。\n\n## Participants\n- 主控：%s\n- 优先处理：%s\n- 参与智能体：%s\n", strings.TrimSpace(userMessage), strings.TrimSpace(session.ControllerAgentID), strings.TrimSpace(session.PreferredAgentID), strings.Join(session.ParticipantAgentIDs, ", "))
	return os.WriteFile(panelChatTaskSpecPath(cfg, taskID), []byte(content), 0o644)
}

func writePanelChatTaskResult(cfg *config.Config, taskID, result string) error {
	return os.WriteFile(panelChatTaskResultPath(cfg, taskID), []byte(strings.TrimSpace(result)+"\n"), 0o644)
}

func writePanelChatTaskFailure(cfg *config.Config, taskID, message string) error {
	path := filepath.Join(panelChatTaskDir(cfg, taskID), "error.md")
	return os.WriteFile(path, []byte(strings.TrimSpace(message)+"\n"), 0o644)
}

func appendPanelChatTaskEvent(cfg *config.Config, taskID string, event panelChatTaskEvent) error {
	path := panelChatTaskTimelinePath(cfg, taskID)
	var events []panelChatTaskEvent
	if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		_ = json.Unmarshal(data, &events)
	}
	events = append(events, event)
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writePanelChatSubtask(cfg *config.Config, taskID string, subtask panelChatSubtask) error {
	path := panelChatTaskSubtaskPath(cfg, taskID, subtask.AgentID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(subtask, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writePanelChatWorkItem(cfg *config.Config, taskID string, item panelChatWorkItem) error {
	path := panelChatTaskWorkItemPath(cfg, taskID, item.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writePanelChatArtifact(cfg *config.Config, taskID, agentID, content string) error {
	path := panelChatTaskArtifactPath(cfg, taskID, agentID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644)
}

func writePanelChatSummary(cfg *config.Config, taskID, agentID, content string) error {
	path := panelChatTaskSummaryPath(cfg, taskID, agentID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644)
}

func summarizePanelChatArtifact(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	out := make([]string, 0, 6)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "###") {
			out = append(out, line)
		} else if len(out) < 2 {
			out = append(out, line)
		}
		if len(out) >= 6 {
			break
		}
	}
	if len(out) == 0 {
		runes := []rune(content)
		if len(runes) > 220 {
			return string(runes[:220]) + "..."
		}
		return content
	}
	return strings.Join(out, "\n")
}

func loadLatestPanelChatTask(cfg *config.Config, sessionID string) (*panelChatTaskBundle, error) {
	base := filepath.Join(cfg.DataDir, "panel-chat", "tasks")
	entries, err := os.ReadDir(base)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	type candidate struct {
		meta panelChatTaskMeta
		dir  string
	}
	var matches []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(base, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta panelChatTaskMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		if strings.TrimSpace(meta.SessionID) != strings.TrimSpace(sessionID) {
			continue
		}
		matches = append(matches, candidate{meta: meta, dir: filepath.Join(base, entry.Name())})
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].meta.CreatedAt > matches[j].meta.CreatedAt })
	chosen := matches[0]
	bundle := &panelChatTaskBundle{TaskID: chosen.meta.ID, Meta: chosen.meta, Subtasks: map[string]panelChatSubtask{}, Artifacts: map[string]string{}}
	if data, err := os.ReadFile(filepath.Join(chosen.dir, "spec.md")); err == nil {
		bundle.Spec = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(chosen.dir, "result.md")); err == nil {
		bundle.Result = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(chosen.dir, "error.md")); err == nil {
		bundle.Error = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(chosen.dir, "timeline.json")); err == nil {
		_ = json.Unmarshal(data, &bundle.Timeline)
	}
	if entries, err := os.ReadDir(filepath.Join(chosen.dir, "subtasks")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(chosen.dir, "subtasks", entry.Name()))
			if err != nil {
				continue
			}
			var sub panelChatSubtask
			if err := json.Unmarshal(data, &sub); err != nil {
				continue
			}
			bundle.Subtasks[sub.AgentID] = sub
		}
	}
	if entries, err := os.ReadDir(filepath.Join(chosen.dir, "artifacts")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(chosen.dir, "artifacts", entry.Name()))
			if err != nil {
				continue
			}
			agentID := strings.TrimSuffix(entry.Name(), ".md")
			bundle.Artifacts[agentID] = string(data)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(chosen.dir, "artifacts")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".summary.md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(chosen.dir, "artifacts", entry.Name()))
			if err != nil {
				continue
			}
			agentID := strings.TrimSuffix(entry.Name(), ".summary.md")
			if sub, ok := bundle.Subtasks[agentID]; ok {
				sub.Summary = strings.TrimSpace(string(data))
				bundle.Subtasks[agentID] = sub
			}
		}
	}
	if entries, err := os.ReadDir(filepath.Join(chosen.dir, "workitems")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(chosen.dir, "workitems", entry.Name()))
			if err != nil {
				continue
			}
			var item panelChatWorkItem
			if err := json.Unmarshal(data, &item); err != nil {
				continue
			}
			bundle.WorkItems = append(bundle.WorkItems, item)
		}
		sort.Slice(bundle.WorkItems, func(i, j int) bool { return bundle.WorkItems[i].ID < bundle.WorkItems[j].ID })
	}
	return bundle, nil
}

func loadPanelChatSessions(cfg *config.Config) ([]panelChatSession, error) {
	path := panelChatSessionsPath(cfg)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []panelChatSession{}, nil
		}
		return nil, err
	}
	var sessions []panelChatSession
	if len(strings.TrimSpace(string(data))) == 0 {
		return []panelChatSession{}, nil
	}
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func savePanelChatSessions(cfg *config.Config, sessions []panelChatSession) error {
	path := panelChatSessionsPath(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func sortPanelChatSessions(sessions []panelChatSession) {
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].UpdatedAt == sessions[j].UpdatedAt {
			return sessions[i].CreatedAt > sessions[j].CreatedAt
		}
		return sessions[i].UpdatedAt > sessions[j].UpdatedAt
	})
}

func findPanelChatSession(sessions []panelChatSession, id string) (int, *panelChatSession) {
	for i := range sessions {
		if sessions[i].ID == id {
			return i, &sessions[i]
		}
	}
	return -1, nil
}

func normalizePanelChatType(chatType string) string {
	return "direct"
}

func normalizePanelChatAgentIDs(agentIDs []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(agentIDs))
	for _, raw := range agentIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func panelChatVirtualTarget(session panelChatSession) string {
	seed := strings.TrimSpace(session.ID) + ":" + strings.TrimSpace(session.AgentID) + ":" + strings.TrimSpace(session.OpenClawSessionID)
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	value := h.Sum64()%90000000000000 + 10000000000000
	return fmt.Sprintf("+%d", value)
}

func panelChatScopedAgentID(sessionID, agentID string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(sessionID + ":" + agentID))
	return fmt.Sprintf("pc_%s_%x", strings.TrimSpace(agentID), h.Sum64())
}

func panelChatRuntimeRoot(cfg *config.Config, sessionID string) string {
	return filepath.Join(cfg.DataDir, "panel-chat", "runtime", sessionID)
}

func ensurePanelChatRuntime(cfg *config.Config, session panelChatSession) (stateDir, configPath, workDir string, err error) {
	root := panelChatRuntimeRoot(cfg, session.ID)
	stateDir = filepath.Join(root, "state")
	workDir = root
	configPath = filepath.Join(stateDir, "openclaw.json")
	if err = os.MkdirAll(stateDir, 0o755); err != nil {
		return "", "", "", err
	}
	if err = os.MkdirAll(filepath.Join(workDir, "openclaw-work"), 0o755); err != nil {
		return "", "", "", err
	}
	sourceConfigPath := filepath.Join(cfg.OpenClawDir, "openclaw.json")
	if err = copyFileIfMissing(sourceConfigPath, configPath); err != nil {
		return "", "", "", err
	}
	agentIDs := []string{session.AgentID}
	if session.ChatType == "group" {
		agentIDs = append(agentIDs, session.ParticipantAgentIDs...)
	}
	agentIDs = normalizePanelChatAgentIDs(agentIDs)
	for _, agentID := range agentIDs {
		srcAgentDir := filepath.Join(cfg.OpenClawDir, "agents", agentID)
		scopedAgentID := panelChatScopedAgentID(session.ID, agentID)
		dstAgentDir := filepath.Join(stateDir, "agents", scopedAgentID)
		if err = copyDirWithoutSessions(srcAgentDir, dstAgentDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", "", err
		}
		_ = os.MkdirAll(filepath.Join(dstAgentDir, "sessions"), 0o755)
		srcWorkspace := filepath.Join(cfg.OpenClawWork, agentID)
		dstWorkspace := filepath.Join(workDir, "openclaw-work", scopedAgentID)
		if err = copyDirIfMissing(srcWorkspace, dstWorkspace); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", "", err
		}
	}
	if err = rewritePanelChatRuntimeConfig(sourceConfigPath, configPath, session, agentIDs); err != nil {
		return "", "", "", err
	}
	return stateDir, configPath, workDir, nil
}

func copyFileIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func copyDirIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrNotExist
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func copyDirWithoutSessions(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrNotExist
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "sessions" || strings.HasPrefix(rel, "sessions"+string(os.PathSeparator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func rewritePanelChatRuntimeConfig(srcConfigPath, dstConfigPath string, session panelChatSession, agentIDs []string) error {
	data, err := os.ReadFile(srcConfigPath)
	if err != nil {
		return err
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	agentsMap := readMap(obj["agents"])
	list, _ := agentsMap["list"].([]interface{})
	newList := make([]interface{}, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		for _, raw := range list {
			agent, ok := raw.(map[string]interface{})
			if !ok || strings.TrimSpace(toString(agent["id"])) != agentID {
				continue
			}
			cloned := map[string]interface{}{}
			for k, v := range agent {
				cloned[k] = v
			}
			scopedID := panelChatScopedAgentID(session.ID, agentID)
			cloned["id"] = scopedID
			cloned["workspace"] = filepath.ToSlash(filepath.Join("openclaw-work", scopedID))
			if strings.TrimSpace(agentID) == strings.TrimSpace(session.ControllerAgentID) || (session.ControllerAgentID == "" && strings.TrimSpace(agentID) == strings.TrimSpace(session.AgentID)) {
				cloned["default"] = true
			} else {
				cloned["default"] = false
			}
			newList = append(newList, cloned)
			break
		}
	}
	agentsMap["list"] = newList
	obj["agents"] = agentsMap
	encoded, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dstConfigPath, append(encoded, '\n'), 0o644)
}

func containsPanelChatAgent(agentIDs []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, id := range agentIDs {
		if strings.TrimSpace(id) == target {
			return true
		}
	}
	return false
}

func clonePanelChatSessionMap(raw map[string]string) map[string]string {
	if len(raw) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = v
	}
	return out
}

func buildPanelChatTitle(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return panelChatDefaultTitle
	}
	runes := []rune(input)
	if len(runes) > 24 {
		return strings.TrimSpace(string(runes[:24])) + "..."
	}
	return input
}

func panelChatSessionFile(cfg *config.Config, agentID, openclawSessionID string) string {
	return resolveAgentPath(cfg, agentID, "sessions", openclawSessionID+".jsonl")
}

func panelChatRuntimeSessionFile(cfg *config.Config, session panelChatSession) string {
	return filepath.Join(panelChatRuntimeRoot(cfg, session.ID), "state", "agents", panelChatScopedAgentID(session.ID, session.AgentID), "sessions", session.OpenClawSessionID+".jsonl")
}

func sanitizePanelChatContent(content string) string {
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, "<think>", "")
	content = strings.ReplaceAll(content, "</think>", "")
	content = strings.TrimPrefix(content, "[[reply_to_current]]")
	content = panelChatTimestampPrefixRe.ReplaceAllString(content, "")
	return strings.TrimSpace(content)
}

func extractPanelChatPayloads(content interface{}) (string, []map[string]string) {
	if content == nil {
		return "", nil
	}
	if s, ok := content.(string); ok {
		return sanitizePanelChatContent(s), nil
	}
	items, ok := content.([]interface{})
	if !ok {
		return "", nil
	}
	parts := make([]string, 0, len(items))
	images := make([]map[string]string, 0)
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		switch t {
		case "text":
			if text, _ := m["text"].(string); strings.TrimSpace(text) != "" {
				parts = append(parts, sanitizePanelChatContent(text))
			}
		case "image":
			data, _ := m["data"].(string)
			if strings.TrimSpace(data) == "" {
				continue
			}
			mimeType, _ := m["mimeType"].(string)
			if mimeType == "" {
				mimeType, _ = m["mediaType"].(string)
			}
			if mimeType == "" {
				mimeType = "image/png"
			}
			src := data
			if !strings.HasPrefix(src, "data:") {
				src = fmt.Sprintf("data:%s;base64,%s", mimeType, data)
			}
			images = append(images, map[string]string{"src": src, "mimeType": mimeType})
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if len(images) > 0 && strings.HasPrefix(text, "Read image file [") {
		text = ""
	}
	return text, images
}

func readPanelChatMessages(cfg *config.Config, session panelChatSession) ([]map[string]interface{}, error) {
	if session.ChatType == "group" {
		return loadPanelChatGroupMessages(cfg, session.ID)
	}
	localMessages, err := loadPanelChatDirectMessages(cfg, session.ID)
	if err != nil {
		return nil, err
	}
	if len(localMessages) > 0 {
		return localMessages, nil
	}
	filePath := panelChatSessionFile(cfg, session.AgentID, session.OpenClawSessionID)
	if _, err := os.Stat(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	messages := make([]map[string]interface{}, 0, 64)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), panelChatScannerMaxTokenSize)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entryType, _ := entry["type"].(string); entryType != "message" && entryType != "assistant" {
			continue
		}
		msg, ok := entry["message"].(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		content, images := extractPanelChatPayloads(msg["content"])
		if role != "user" && role != "assistant" {
			if role == "toolResult" && len(images) > 0 {
				role = "assistant"
			} else {
				continue
			}
		}
		if content == "" {
			if errMsg, _ := msg["errorMessage"].(string); strings.TrimSpace(errMsg) != "" {
				content = errMsg
			}
		}
		if content == "" && len(images) == 0 {
			continue
		}
		ts, _ := entry["timestamp"].(string)
		message := map[string]interface{}{
			"id":        entry["id"],
			"role":      role,
			"content":   content,
			"timestamp": ts,
		}
		if len(images) > 0 {
			message["images"] = images
		}
		messages = append(messages, message)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(messages) > 400 {
		messages = messages[len(messages)-400:]
	}
	return messages, nil
}

func resolvePanelChatCommand(cfg *config.Config) (string, []string, error) {
	if app := strings.TrimSpace(cfg.OpenClawApp); app != "" {
		entry := filepath.Join(app, "openclaw.mjs")
		if _, err := os.Stat(entry); err == nil {
			nodeCandidates := []string{
				filepath.Join(filepath.Dir(app), "node", "bin", "node"),
				"node",
			}
			for _, candidate := range nodeCandidates {
				candidate = strings.TrimSpace(candidate)
				if candidate == "" {
					continue
				}
				if filepath.IsAbs(candidate) {
					if _, err := os.Stat(candidate); err == nil {
						return candidate, []string{entry}, nil
					}
					continue
				}
				if resolved, err := exec.LookPath(candidate); err == nil {
					return resolved, []string{entry}, nil
				}
			}
		}
	}

	for _, bin := range candidateOpenClawBins(cfg) {
		bin = strings.TrimSpace(bin)
		if bin == "" {
			continue
		}
		if filepath.IsAbs(bin) {
			if _, err := os.Stat(bin); err == nil {
				return bin, nil, nil
			}
			continue
		}
		if resolved, err := exec.LookPath(bin); err == nil {
			return resolved, nil, nil
		}
	}

	return "", nil, fmt.Errorf("未找到可用的 openclaw 命令")
}

func newPanelChatExecCommand(ctx context.Context, cfg *config.Config, session panelChatSession, message string) (*exec.Cmd, error) {
	sessionStateDir, sessionConfigPath, sessionWorkDir, err := ensurePanelChatRuntime(cfg, session)
	if err != nil {
		return nil, err
	}
	bin, prefixArgs, err := resolvePanelChatCommand(cfg)
	if err != nil {
		return nil, err
	}
	args := append([]string{}, prefixArgs...)
	args = append(args, "agent", "--agent", panelChatScopedAgentID(session.ID, session.AgentID), "--to", panelChatVirtualTarget(session), "--session-id", session.OpenClawSessionID, "--message", message, "--json")
	cmd := exec.CommandContext(ctx, bin, args...)
	setPanelChatProcessGroup(cmd)
	cmd.Dir = sessionStateDir
	cmd.Env = append(config.BuildExecEnv(),
		fmt.Sprintf("OPENCLAW_DIR=%s", sessionStateDir),
		fmt.Sprintf("OPENCLAW_STATE_DIR=%s", sessionStateDir),
		fmt.Sprintf("OPENCLAW_CONFIG_PATH=%s", sessionConfigPath),
	)
	if sessionWorkDir != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("OPENCLAW_WORK_DIR=%s", sessionWorkDir))
	}
	if cfg.OpenClawApp != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("OPENCLAW_APP=%s", cfg.OpenClawApp))
	}
	return cmd, nil
}

func extractPanelChatJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if idx := strings.LastIndex(raw, "\n{"); idx >= 0 {
		return strings.TrimSpace(raw[idx+1:])
	}
	if idx := strings.Index(raw, "{"); idx >= 0 {
		return strings.TrimSpace(raw[idx:])
	}
	return raw
}

func runPanelChatMessage(ctx context.Context, cfg *config.Config, session panelChatSession, message string) (string, string, error) {
	baseCtx, baseCancel := context.WithCancel(ctx)
	defer baseCancel()
	ctx, timeoutCancel := context.WithTimeout(baseCtx, 70*time.Second)
	defer timeoutCancel()
	panelChatActiveRuns.Store(session.ID, panelChatActiveRun{cancel: baseCancel})
	defer panelChatActiveRuns.Delete(session.ID)
	cmd, err := newPanelChatExecCommand(ctx, cfg, session, message)
	if err != nil {
		return "", "", err
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	waitCh := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return "", "", err
	}
	panelChatActiveRuns.Store(session.ID, panelChatActiveRun{cancel: baseCancel, pid: cmd.Process.Pid})
	go func() {
		waitCh <- cmd.Wait()
	}()
	var waitErr error
	select {
	case <-ctx.Done():
		killPanelChatProcess(cmd)
		waitErr = <-waitCh
	case waitErr = <-waitCh:
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return "", "", errPanelChatCanceled
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "", "", errPanelChatTimeout
	}
	if waitErr != nil {
		trimmed := strings.TrimSpace(output.String())
		if trimmed == "" {
			trimmed = waitErr.Error()
		}
		return "", "", fmt.Errorf("%s", trimmed)
	}

	jsonText := extractPanelChatJSON(output.String())
	var result panelChatCLIResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return "", "", fmt.Errorf("无法解析 OpenClaw 返回结果")
	}

	parts := make([]string, 0, len(result.Result.Payloads))
	for _, payload := range result.Result.Payloads {
		text := strings.TrimSpace(payload.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n"), strings.TrimSpace(result.Result.Meta.AgentMeta.SessionID), nil
}

func CancelPanelChatMessage(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("id"))
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "session id required"})
			return
		}
		if active, ok := panelChatActiveRuns.Load(sessionID); ok {
			active.(panelChatActiveRun).cancel()
		}
		if _, err := updatePanelChatSessionState(cfg, sessionID, func(item *panelChatSession) {
			item.Processing = false
			item.UpdatedAt = time.Now().UnixMilli()
		}); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "canceled": true})
	}
}

func panelChatEffectiveReply(messages []map[string]interface{}, fallback string) string {
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if role, _ := messages[i]["role"].(string); role == "assistant" {
			if content, _ := messages[i]["content"].(string); strings.TrimSpace(content) != "" {
				return content
			}
		}
	}
	return ""
}

func extractLatestPanelChatExchange(messages []map[string]interface{}, userMessage string) []map[string]interface{} {
	needle := sanitizePanelChatContent(userMessage)
	start := -1
	for i := len(messages) - 1; i >= 0; i-- {
		role, _ := messages[i]["role"].(string)
		content, _ := messages[i]["content"].(string)
		if role == "user" && sanitizePanelChatContent(content) == needle {
			start = i
			break
		}
	}
	if start == -1 {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(messages)-start)
	for i := start; i < len(messages); i++ {
		role, _ := messages[i]["role"].(string)
		if role != "user" && role != "assistant" {
			continue
		}
		content, _ := messages[i]["content"].(string)
		content = sanitizePanelChatContent(content)
		if strings.TrimSpace(content) == "" {
			continue
		}
		cloned := map[string]interface{}{
			"id":        messages[i]["id"],
			"role":      role,
			"content":   content,
			"timestamp": messages[i]["timestamp"],
		}
		result = append(result, cloned)
	}
	return result
}

func mergePanelChatTranscripts(existing, incoming []map[string]interface{}) []map[string]interface{} {
	if len(existing) == 0 {
		return incoming
	}
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	out := make([]map[string]interface{}, 0, len(existing)+len(incoming))
	for _, msg := range existing {
		id, _ := msg["id"].(string)
		if id != "" {
			seen[id] = struct{}{}
		}
		out = append(out, msg)
	}
	for _, msg := range incoming {
		id, _ := msg["id"].(string)
		if id != "" {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
		}
		out = append(out, msg)
	}
	return out
}

func updatePanelChatSessionState(cfg *config.Config, sessionID string, mutate func(*panelChatSession)) (*panelChatSession, error) {
	sessions, err := loadPanelChatSessions(cfg)
	if err != nil {
		return nil, err
	}
	idx, session := findPanelChatSession(sessions, sessionID)
	if session == nil {
		return nil, fmt.Errorf("会话不存在")
	}
	mutate(&sessions[idx])
	sortPanelChatSessions(sessions)
	if err := savePanelChatSessions(cfg, sessions); err != nil {
		return nil, err
	}
	for i := range sessions {
		if sessions[i].ID == sessionID {
			return &sessions[i], nil
		}
	}
	return &sessions[0], nil
}

func newPanelChatTimelineMessage(sessionID, role, content, agentID, stage string, internal bool) map[string]interface{} {
	msg := map[string]interface{}{
		"id":        fmt.Sprintf("%s-%d", role, time.Now().UnixNano()),
		"role":      role,
		"content":   strings.TrimSpace(content),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if sessionID != "" {
		msg["sessionId"] = sessionID
	}
	if agentID != "" {
		msg["agentId"] = agentID
	}
	if stage != "" {
		msg["stage"] = stage
	}
	if internal {
		msg["internal"] = true
	}
	return msg
}

func buildPanelChatSpecialistPrompt(session panelChatSession, userMessage, targetAgentID string) string {
	controller := strings.TrimSpace(session.ControllerAgentID)
	if controller == "" {
		controller = "main"
	}
	participants := strings.Join(session.ParticipantAgentIDs, ", ")
	scope := "只处理属于你职责的技术或实现部分。"
	if strings.EqualFold(targetAgentID, "coding") {
		scope = "你只处理技术实现部分：规则设计、系统结构、模块划分、数据结构、平衡数值建议。不要负责报告正文、文案润色、最终风险总结。输出尽量控制在 6 个小点以内。"
	}
	return fmt.Sprintf("你正在参与一个多智能体协作群聊。\n你的 agentId 是 %s。\n主控 agentId 是 %s。\n参与智能体有：%s。\n\n用户原始问题：\n%s\n\n职责范围说明：%s\n\n请先判断这件事是否主要属于你的职责范围。\n- 如果主要属于你的职责范围，请给出面向主控智能体的专项处理结果。\n- 如果只有少部分与你相关，请说明你能处理的部分，并明确哪些部分应该交回主控智能体。\n- 如果明显不属于你的职责范围，请直接回复：HANDOFF_TO_MAIN，并简述原因。\n\n额外规则：\n1. 不要长时间停留在泛泛澄清阶段。只要已有信息足够开始，请先给出阶段性成果。\n2. 对于课程设计/软件项目类任务，即使信息不完整，也至少先产出：功能拆解、模块划分、数据结构建议或下一步执行框架。\n3. 你的结果要尽量细化，优先拆成可执行小项，不要只给“大概方向”。\n4. 如果需要补充信息，请把“待补充项”放在最后，并同时给出你已经能确定的结论。\n5. 不要输出 thinking、推理痕迹、XML/HTML 标签或内部协议说明。\n\n注意：\n- 你现在不是直接对用户回复。\n- 你的输出对象是主控智能体。\n- 回答尽量结构化、简洁、专业。", targetAgentID, controller, participants, userMessage, scope)
}

func buildPanelChatControllerPrompt(session panelChatSession, userMessage, preferredAgentID, specialistReport string) string {
	return buildPanelChatControllerPromptWithReview(session, userMessage, preferredAgentID, specialistReport, "")
}

func buildPanelChatControllerPromptWithReview(session panelChatSession, userMessage, preferredAgentID, specialistReport, reviewerReport string) string {
	controller := strings.TrimSpace(session.ControllerAgentID)
	if controller == "" {
		controller = "main"
	}
	participants := strings.Join(session.ParticipantAgentIDs, ", ")
	reportBlock := ""
	if strings.TrimSpace(specialistReport) != "" {
		reportBlock = fmt.Sprintf("\n\n优先处理智能体 %s 的内部回报：\n%s", preferredAgentID, specialistReport)
	}
	reviewBlock := ""
	if strings.TrimSpace(reviewerReport) != "" {
		reviewBlock = fmt.Sprintf("\n\nreviewer 的校验结果：\n%s", reviewerReport)
	}
	return fmt.Sprintf("你是当前群聊的主控智能体 %s。\n参与智能体有：%s。\n用户当前问题：\n%s%s%s\n\n请按以下规则输出最终给用户的答复：\n1. 如果问题简单，可以直接给出结论和方案。\n2. 如果问题主要属于某个领域，结合该智能体的回报进行总结。\n3. 如果 reviewer 已给出校验意见，请优先采用校验后的结论。\n4. 如果问题明显超出当前参与智能体能力范围，明确建议用户创建更合适的智能体。\n5. 对课程设计/项目开发类任务，不要一直卡在补信息。只要题目、核心要求、规模已知，就先给出可执行的阶段成果，例如功能清单、模块设计、数据结构、开发计划或代码框架。\n6. 任务拆解时尽量细化，不要只给笼统大项。优先拆成可执行的小任务，例如：数据结构设计、菜单流程、文件读写、测试用例、报告目录，而不是一句“完成系统设计”。\n7. 如果确实需要用户补充信息，请在答复末尾列出最少必要项，并且先交付你已经可以完成的部分。\n8. 不要暴露内部协作细节、thinking、工具痕迹或标签内容。最终答复应面向用户、清晰、可执行。", controller, participants, userMessage, reportBlock, reviewBlock)
}

func buildPanelChatReviewerPrompt(session panelChatSession, userMessage, specialistReport string) string {
	participants := strings.Join(session.ParticipantAgentIDs, ", ")
	return fmt.Sprintf("你正在参与一个多智能体协作群聊校验环节。\n你的角色是 reviewer。\n参与智能体有：%s。\n\n用户原始问题：\n%s\n\n待校验的执行回报：\n%s\n\n请从以下维度做校验：\n1. 是否回答了用户核心问题\n2. 是否有明显遗漏或越权\n3. 是否需要补充风险提示\n4. 是否把可先执行的部分先推进了，而不是一味停留在澄清阶段\n5. 是否可以交由主控智能体对外总结\n\n请输出简洁、结构化的校验意见，不要输出 thinking 或内部标签。", participants, userMessage, specialistReport)
}

func buildPanelChatWriterPrompt(session panelChatSession, userMessage, specialistReport string) string {
	participants := strings.Join(session.ParticipantAgentIDs, ", ")
	return fmt.Sprintf("你正在参与一个多智能体协作群聊。\n你的角色是 writer。\n参与智能体有：%s。\n\n用户原始问题：\n%s\n\n如果问题涉及报告、说明文档、总结、设计文档、Word 文档、答辩材料、交付说明，请输出面向主控智能体的文档结构化成果。\n\n你必须优先产出：\n1. 报告目录或章节结构\n2. 每章应写什么\n3. 若已有 coding 的专项回报，可将其整理成适合课程设计/说明文档的书写框架\n4. 若任务是课程设计/项目交付，至少给出一份可直接使用的文档骨架\n\nCoding 的专项结果如下：\n%s\n\n硬性规则：\n- 只输出可直接整合进最终文档/报告的内容\n- 不要输出 thinking、内部标签、闲聊口吻\n- 不要重复解释自己的职责\n- 如果需要文档，请务必输出正文骨架，不要只说“可以由 writer 补强”\n- 只有在任务完全与文档无关时，才回复：WRITER_NOT_NEEDED", participants, userMessage, specialistReport)
}

func buildPanelChatReworkPrompt(session panelChatSession, userMessage, priorReport, reviewerReport, agentID string) string {
	return fmt.Sprintf("你正在处理一轮返工任务。\n当前 agentId: %s\n用户原始问题：\n%s\n\n你上一轮产出：\n%s\n\nreviewer 的返工意见：\n%s\n\n请根据 reviewer 意见重新输出一版更可交付的结果。\n要求：\n- 直接输出修订后的结果\n- 不要解释太多过程\n- 不要输出 thinking 或内部标签", agentID, userMessage, priorReport, reviewerReport)
}

func panelChatNeedsWriter(userMessage string) bool {
	text := strings.ToLower(userMessage)
	keywords := []string{"报告", "文档", "说明", "word", "总结", "答辩", "设计书", "设计报告", "report", "doc", "文案", "礼包", "活动说明", "公告", "活动文案", "shop copy"}
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func buildPanelChatWriterFallback(userMessage, specialistReport string) string {
	return fmt.Sprintf("## 报告结构建议\n\n### 1. 课题名称\n- 直接写本次课程设计题目\n\n### 2. 设计目的\n- 说明本系统要解决的问题\n- 说明训练的 C 语言知识点：结构体、函数、文件操作、菜单程序\n\n### 3. 总体设计\n- 系统总体功能说明\n- 模块划分\n- 程序流程\n\n### 4. 详细设计\n- 数据结构设计\n- 各功能模块说明\n- 文件存储设计\n- 核心算法说明\n\n### 5. 程序清单\n- 附完整源代码\n- 说明关键函数作用\n\n### 6. 运行结果及分析\n- 菜单界面\n- 新增/查询/修改/删除结果\n- 功能测试分析\n\n### 7. 总结\n- 本次课设完成情况\n- 学到的知识点\n- 存在问题与改进方向\n\n---\n\n### 可直接整合进报告的写作建议\n- 先按 Coding 的技术方案写“总体设计”和“详细设计”\n- 再补“运行结果”和“总结”\n- 若学校有固定模板，再对标题和封面格式做适配\n\n用户原始任务：%s\n\nCoding 专项结果摘要：\n%s", userMessage, specialistReport)
}

func extractPanelChatUserRequest(spec string) string {
	marker := "## User Request\n"
	idx := strings.Index(spec, marker)
	if idx == -1 {
		return strings.TrimSpace(spec)
	}
	rest := spec[idx+len(marker):]
	if end := strings.Index(rest, "\n\n## "); end >= 0 {
		return strings.TrimSpace(rest[:end])
	}
	return strings.TrimSpace(rest)
}

func splitPanelChatComplexTask(userMessage string) map[string][]string {
	parts := []string{}
	for _, seg := range regexp.MustCompile(`[\n；;]+`).Split(userMessage, -1) {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		parts = append(parts, seg)
	}
	if len(parts) <= 1 {
		return nil
	}
	out := map[string][]string{}
	for _, part := range parts {
		lower := strings.ToLower(part)
		switch {
		case strings.Contains(lower, "文案") || strings.Contains(lower, "新闻稿") || strings.Contains(lower, "报告") || strings.Contains(lower, "结构") || strings.Contains(lower, "宣传"):
			out["writer"] = append(out["writer"], part)
		case strings.Contains(lower, "风险") || strings.Contains(lower, "回滚") || strings.Contains(lower, "验收") || strings.Contains(lower, "校验") || strings.Contains(lower, "检查"):
			out["reviewer"] = append(out["reviewer"], part)
		default:
			out["coding"] = append(out["coding"], part)
		}
	}
	return out
}

func joinPanelChatTaskParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return "- " + strings.Join(parts, "\n- ")
}

func buildPanelChatWorkItems(userMessage string) []panelChatWorkItem {
	buckets := splitPanelChatComplexTask(userMessage)
	if len(buckets) == 0 {
		return nil
	}
	items := make([]panelChatWorkItem, 0, 8)
	seq := 1
	for agentID, parts := range buckets {
		for _, part := range parts {
			items = append(items, panelChatWorkItem{
				ID:      fmt.Sprintf("subtask-%02d", seq),
				AgentID: agentID,
				Title:   part,
				Status:  "pending",
			})
			seq++
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func buildPanelChatWorkItemPrompt(session panelChatSession, item panelChatWorkItem) string {
	switch item.AgentID {
	case "writer":
		return buildPanelChatWriterPrompt(session, item.Title, "")
	case "reviewer":
		return buildPanelChatReviewerPrompt(session, item.Title, "")
	default:
		return buildPanelChatSpecialistPrompt(session, item.Title, item.AgentID)
	}
}

func panelChatReviewerRequestsRework(report string) bool {
	text := strings.ToLower(report)
	keywords := []string{"返工", "需返工", "建议返工", "不可交付", "需要重写", "需要重做", "not ready", "rework"}
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func runPanelChatGroupWorkflow(ctx context.Context, cfg *config.Config, session panelChatSession, taskID, userMessage, preferredAgentID string) ([]map[string]interface{}, string, map[string]string, error) {
	messages, err := loadPanelChatGroupMessages(cfg, session.ID)
	if err != nil {
		return nil, "", nil, err
	}
	controller := strings.TrimSpace(session.ControllerAgentID)
	if controller == "" {
		controller = session.AgentID
	}
	if controller == "" {
		controller = "main"
	}
	participants := normalizePanelChatAgentIDs(session.ParticipantAgentIDs)
	if len(participants) == 0 {
		participants = []string{controller}
	}
	if !containsPanelChatAgent(participants, controller) {
		participants = append([]string{controller}, participants...)
		participants = normalizePanelChatAgentIDs(participants)
	}
	preferred := strings.TrimSpace(preferredAgentID)
	if preferred == "" || !containsPanelChatAgent(participants, preferred) {
		preferred = controller
	}
	groupSessionIDs := clonePanelChatSessionMap(session.GroupAgentSessionIDs)
	taskBuckets := splitPanelChatComplexTask(userMessage)
	messages = append(messages, newPanelChatTimelineMessage(session.ID, "user", userMessage, "", "user", false))
	if workItems := buildPanelChatWorkItems(userMessage); len(workItems) >= 4 {
		messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", fmt.Sprintf("任务较复杂，已拆分为 %d 个独立子任务，分别交由对应智能体处理。", len(workItems)), controller, "plan", true))
		summaryBlocks := make([]string, 0, len(workItems))
		for _, item := range workItems {
			if taskID != "" {
				item.Status = "running"
				_ = writePanelChatWorkItem(cfg, taskID, item)
				_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "subtask_dispatched", AgentID: controller, TargetAgentID: item.AgentID, Message: item.Title})
			}
			sessionID := strings.TrimSpace(groupSessionIDs[item.AgentID])
			if sessionID == "" {
				sessionID = fmt.Sprintf("%s-%s", session.ID, item.AgentID)
			}
			report, actualSessionID, err := runPanelChatMessage(ctx, cfg, panelChatSession{ID: session.ID, AgentID: item.AgentID, OpenClawSessionID: sessionID, ControllerAgentID: controller, ParticipantAgentIDs: participants, ChatType: session.ChatType}, buildPanelChatWorkItemPrompt(session, item))
			if err != nil {
				if taskID != "" {
					item.Status = "failed"
					item.Summary = err.Error()
					_ = writePanelChatWorkItem(cfg, taskID, item)
					_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "subtask_failed", AgentID: item.AgentID, Message: err.Error()})
				}
				return messages, "", groupSessionIDs, err
			}
			if strings.TrimSpace(actualSessionID) != "" {
				groupSessionIDs[item.AgentID] = strings.TrimSpace(actualSessionID)
			}
			report = strings.TrimSpace(report)
			summary := summarizePanelChatArtifact(report)
			if report != "" {
				messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", report, item.AgentID, "report", true))
			}
			if taskID != "" {
				item.Status = "done"
				item.Summary = summary
				item.OutputRef = filepath.Join("artifacts", item.ID+"-"+item.AgentID+".md")
				_ = writePanelChatWorkItem(cfg, taskID, item)
				_ = writePanelChatArtifact(cfg, taskID, item.ID+"-"+item.AgentID, report)
				_ = writePanelChatSummary(cfg, taskID, item.ID+"-"+item.AgentID, summary)
				_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: item.AgentID, Role: "executor", Status: "done", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: item.OutputRef, Summary: summary, SummaryRef: filepath.Join("artifacts", item.ID+"-"+item.AgentID+".summary.md")})
			}
			if summary != "" {
				summaryBlocks = append(summaryBlocks, fmt.Sprintf("[%s]\n%s", item.Title, summary))
			}
		}
		controllerSessionID := strings.TrimSpace(groupSessionIDs[controller])
		if controllerSessionID == "" {
			controllerSessionID = fmt.Sprintf("%s-%s", session.ID, controller)
		}
		finalPrompt := fmt.Sprintf("你是主控智能体 %s。\n用户原始任务：\n%s\n\n以下是各独立子任务的摘要，请只基于这些摘要生成最终交付结果，不要重新展开全部细节：\n\n%s", controller, userMessage, strings.Join(summaryBlocks, "\n\n"))
		finalReply, actualSessionID, err := runPanelChatMessage(ctx, cfg, panelChatSession{ID: session.ID, AgentID: controller, OpenClawSessionID: controllerSessionID}, finalPrompt)
		if err != nil {
			return messages, "", groupSessionIDs, err
		}
		if strings.TrimSpace(actualSessionID) != "" {
			groupSessionIDs[controller] = strings.TrimSpace(actualSessionID)
		}
		finalReply = strings.TrimSpace(finalReply)
		if finalReply != "" {
			messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", finalReply, controller, "final", false))
			if taskID != "" {
				_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: controller, Role: "controller", Status: "done", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: "result.md", Summary: "主控基于子任务摘要完成汇总"})
				_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "task_done", AgentID: controller, Message: "主控基于子任务摘要完成汇总"})
			}
		}
		return messages, finalReply, groupSessionIDs, nil
	}
	var specialistReport string
	var writerReport string
	var reviewerReport string
	if preferred != controller {
		preferredTask := userMessage
		if len(taskBuckets[preferred]) > 0 {
			preferredTask = joinPanelChatTaskParts(taskBuckets[preferred])
		}
		dispatch := fmt.Sprintf("收到，我会先请 %s 从其职责范围判断并处理相关部分，然后由 %s 统一汇总结论。", preferred, controller)
		messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", dispatch, controller, "plan", true))
		if taskID != "" {
			_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: preferred, Role: "executor", Status: "running", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", Summary: "已分派 coding 专项任务"})
			_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "subtask_dispatched", AgentID: controller, TargetAgentID: preferred, Message: dispatch})
		}
		specialistSessionID := strings.TrimSpace(groupSessionIDs[preferred])
		if specialistSessionID == "" {
			specialistSessionID = fmt.Sprintf("%s-%s", session.ID, preferred)
		}
		report, actualSessionID, err := runPanelChatMessage(ctx, cfg, panelChatSession{ID: session.ID, AgentID: preferred, OpenClawSessionID: specialistSessionID}, buildPanelChatSpecialistPrompt(session, preferredTask, preferred))
		if err != nil {
			return messages, "", groupSessionIDs, err
		}
		if strings.TrimSpace(actualSessionID) != "" {
			groupSessionIDs[preferred] = strings.TrimSpace(actualSessionID)
		}
		specialistReport = strings.TrimSpace(report)
		if specialistReport != "" {
			messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", specialistReport, preferred, "report", true))
			if taskID != "" {
				_ = writePanelChatArtifact(cfg, taskID, preferred, specialistReport)
				_ = writePanelChatSummary(cfg, taskID, preferred, summarizePanelChatArtifact(specialistReport))
				_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: preferred, Role: "executor", Status: "done", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: filepath.Join("artifacts", preferred+".md"), Summary: "专项处理结果已提交"})
				_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "subtask_done", AgentID: preferred, Message: "专项处理结果已提交"})
			}
		}
	}
	writerID := ""
	for _, id := range participants {
		if id == "writer" {
			writerID = id
			break
		}
	}
	if writerID != "" && panelChatNeedsWriter(userMessage) && strings.TrimSpace(writerReport) == "" {
		writerTask := userMessage
		if len(taskBuckets[writerID]) > 0 {
			writerTask = joinPanelChatTaskParts(taskBuckets[writerID])
		}
		writerReport = buildPanelChatWriterFallback(writerTask, specialistReport)
		messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", writerReport, writerID, "report", true))
		if taskID != "" {
			_ = writePanelChatArtifact(cfg, taskID, writerID, writerReport)
			_ = writePanelChatSummary(cfg, taskID, writerID, summarizePanelChatArtifact(writerReport))
			_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: writerID, Role: "writer", Status: "done", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: filepath.Join("artifacts", writerID+".md"), Summary: "writer 已生成报告结构成果"})
			_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "subtask_done", AgentID: writerID, Message: "writer 已生成报告结构成果"})
		}
	}
	reviewerID := ""
	for _, id := range participants {
		if id == "reviewer" {
			reviewerID = id
			break
		}
	}
	if reviewerID != "" && reviewerID != controller && strings.TrimSpace(specialistReport) != "" {
		reviewerSessionID := strings.TrimSpace(groupSessionIDs[reviewerID])
		if reviewerSessionID == "" {
			reviewerSessionID = fmt.Sprintf("%s-%s", session.ID, reviewerID)
		}
		combinedReport := strings.TrimSpace(strings.TrimSpace(summarizePanelChatArtifact(specialistReport)) + "\n\n" + strings.TrimSpace(summarizePanelChatArtifact(writerReport)))
		review, actualSessionID, err := runPanelChatMessage(ctx, cfg, panelChatSession{ID: session.ID, AgentID: reviewerID, OpenClawSessionID: reviewerSessionID}, buildPanelChatReviewerPrompt(session, userMessage, combinedReport))
		if err != nil {
			if strings.Contains(err.Error(), "Unknown agent id") {
				messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", fmt.Sprintf("reviewer 未配置，跳过校验环节，由 %s 直接汇总。", controller), controller, "review", true))
			} else {
				return messages, "", groupSessionIDs, err
			}
		} else {
			if strings.TrimSpace(actualSessionID) != "" {
				groupSessionIDs[reviewerID] = strings.TrimSpace(actualSessionID)
			}
			reviewerReport = strings.TrimSpace(review)
			if reviewerReport != "" {
				messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", reviewerReport, reviewerID, "review", true))
				if taskID != "" {
					_ = writePanelChatArtifact(cfg, taskID, reviewerID, reviewerReport)
					_ = writePanelChatSummary(cfg, taskID, reviewerID, summarizePanelChatArtifact(reviewerReport))
					_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: reviewerID, Role: "reviewer", Status: "done", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: filepath.Join("artifacts", reviewerID+".md"), Summary: "reviewer 已完成复核"})
					_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "review_done", AgentID: reviewerID, Message: "reviewer 已完成复核"})
					if panelChatReviewerRequestsRework(reviewerReport) {
						_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "review_returned", AgentID: reviewerID, Message: "reviewer 建议返工"})
						if preferred != controller {
							_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: preferred, Role: "executor", Status: "returned", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: filepath.Join("artifacts", preferred+".md"), Summary: "reviewer 建议 coding 返工"})
						}
						if writerID != "" && strings.TrimSpace(writerReport) != "" {
							_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: writerID, Role: "writer", Status: "returned", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: filepath.Join("artifacts", writerID+".md"), Summary: "reviewer 建议 writer 返工"})
						}
					}
				}
			}
		}
	}
	if panelChatReviewerRequestsRework(reviewerReport) {
		if preferred != controller && strings.TrimSpace(specialistReport) != "" {
			rework, actualSessionID, err := runPanelChatMessage(ctx, cfg, panelChatSession{ID: session.ID, AgentID: preferred, OpenClawSessionID: groupSessionIDs[preferred]}, buildPanelChatReworkPrompt(session, userMessage, specialistReport, reviewerReport, preferred))
			if err == nil && strings.TrimSpace(rework) != "" {
				if strings.TrimSpace(actualSessionID) != "" {
					groupSessionIDs[preferred] = strings.TrimSpace(actualSessionID)
				}
				specialistReport = strings.TrimSpace(rework)
				messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", specialistReport, preferred, "report", true))
				if taskID != "" {
					_ = writePanelChatArtifact(cfg, taskID, preferred, specialistReport)
					_ = writePanelChatSummary(cfg, taskID, preferred, summarizePanelChatArtifact(specialistReport))
					_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: preferred, Role: "executor", Status: "done", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: filepath.Join("artifacts", preferred+".md"), Summary: "coding 已根据 reviewer 意见返工完成"})
					_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "subtask_redone", AgentID: preferred, Message: "coding 已完成返工"})
				}
			}
		}
		if writerID != "" && strings.TrimSpace(writerReport) != "" {
			rework, actualSessionID, err := runPanelChatMessage(ctx, cfg, panelChatSession{ID: session.ID, AgentID: writerID, OpenClawSessionID: groupSessionIDs[writerID]}, buildPanelChatReworkPrompt(session, userMessage, writerReport, reviewerReport, writerID))
			if err == nil && strings.TrimSpace(rework) != "" {
				if strings.TrimSpace(actualSessionID) != "" {
					groupSessionIDs[writerID] = strings.TrimSpace(actualSessionID)
				}
				writerReport = strings.TrimSpace(rework)
				messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", writerReport, writerID, "report", true))
				if taskID != "" {
					_ = writePanelChatArtifact(cfg, taskID, writerID, writerReport)
					_ = writePanelChatSummary(cfg, taskID, writerID, summarizePanelChatArtifact(writerReport))
					_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: writerID, Role: "writer", Status: "done", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: filepath.Join("artifacts", writerID+".md"), Summary: "writer 已根据 reviewer 意见返工完成"})
					_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "subtask_redone", AgentID: writerID, Message: "writer 已完成返工"})
				}
			}
		}
	}
	controllerSessionID := strings.TrimSpace(groupSessionIDs[controller])
	if controllerSessionID == "" {
		controllerSessionID = session.OpenClawSessionID
	}
	if controllerSessionID == "" {
		controllerSessionID = fmt.Sprintf("%s-%s", session.ID, controller)
	}
	combinedReport := strings.TrimSpace(strings.TrimSpace(summarizePanelChatArtifact(specialistReport)) + "\n\n" + strings.TrimSpace(summarizePanelChatArtifact(writerReport)))
	finalReply, actualSessionID, err := runPanelChatMessage(ctx, cfg, panelChatSession{ID: session.ID, AgentID: controller, OpenClawSessionID: controllerSessionID}, buildPanelChatControllerPromptWithReview(session, userMessage, preferred, combinedReport, summarizePanelChatArtifact(reviewerReport)))
	if err != nil {
		return messages, "", groupSessionIDs, err
	}
	if strings.TrimSpace(actualSessionID) != "" {
		groupSessionIDs[controller] = strings.TrimSpace(actualSessionID)
	}
	finalReply = strings.TrimSpace(finalReply)
	if finalReply != "" {
		messages = append(messages, newPanelChatTimelineMessage(session.ID, "assistant", finalReply, controller, "final", false))
		if taskID != "" {
			_ = writePanelChatSubtask(cfg, taskID, panelChatSubtask{AgentID: controller, Role: "controller", Status: "done", AssignedAt: time.Now().UTC().Format(time.RFC3339), StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339), InputRef: "spec.md", OutputRef: "result.md", Summary: "主控智能体已完成最终汇总"})
			_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "task_done", AgentID: controller, Message: "主控智能体已完成最终汇总"})
		}
	}
	return messages, finalReply, groupSessionIDs, nil
}

func ListPanelChatSessions(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := loadPanelChatSessions(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		sortPanelChatSessions(sessions)
		c.JSON(http.StatusOK, gin.H{"ok": true, "sessions": sessions})
	}
}

func CreatePanelChatSession(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title               string   `json:"title"`
			ChatType            string   `json:"chatType"`
			AgentID             string   `json:"agentId"`
			ParticipantAgentIDs []string `json:"participantAgentIds"`
			ControllerAgentID   string   `json:"controllerAgentId"`
			PreferredAgentID    string   `json:"preferredAgentId"`
			TargetID            string   `json:"targetId"`
			TargetName          string   `json:"targetName"`
		}
		_ = c.ShouldBindJSON(&req)

		agentID := strings.TrimSpace(req.AgentID)
		if agentID == "" {
			agentID = loadDefaultAgentID(cfg)
		}
		chatType := normalizePanelChatType(req.ChatType)
		participantAgentIDs := normalizePanelChatAgentIDs(req.ParticipantAgentIDs)
		controllerAgentID := strings.TrimSpace(req.ControllerAgentID)
		preferredAgentID := strings.TrimSpace(req.PreferredAgentID)
		if chatType == "group" {
			if controllerAgentID == "" {
				controllerAgentID = "main"
			}
			if !containsPanelChatAgent(participantAgentIDs, controllerAgentID) {
				participantAgentIDs = append([]string{controllerAgentID}, participantAgentIDs...)
				participantAgentIDs = normalizePanelChatAgentIDs(participantAgentIDs)
			}
			if len(participantAgentIDs) == 0 {
				participantAgentIDs = []string{"main"}
			}
			if preferredAgentID == "" || !containsPanelChatAgent(participantAgentIDs, preferredAgentID) {
				preferredAgentID = controllerAgentID
			}
			agentID = controllerAgentID
		}
		now := time.Now().UnixMilli()
		id := fmt.Sprintf("panel-%d", now)
		session := panelChatSession{
			ID:                   id,
			OpenClawSessionID:    id,
			AgentID:              agentID,
			ChatType:             chatType,
			ParticipantAgentIDs:  participantAgentIDs,
			ControllerAgentID:    controllerAgentID,
			PreferredAgentID:     preferredAgentID,
			GroupAgentSessionIDs: map[string]string{},
			Status:               "idle",
			Title:                buildPanelChatTitle(req.Title),
			TargetID:             strings.TrimSpace(req.TargetID),
			TargetName:           strings.TrimSpace(req.TargetName),
			CreatedAt:            now,
			UpdatedAt:            now,
		}

		sessions, err := loadPanelChatSessions(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		sessions = append(sessions, session)
		sortPanelChatSessions(sessions)
		if err := savePanelChatSessions(cfg, sessions); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "session": session})
	}
}

func GetPanelChatSessionDetail(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := loadPanelChatSessions(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		_, session := findPanelChatSession(sessions, c.Param("id"))
		if session == nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "会话不存在"})
			return
		}
		messages, err := readPanelChatMessages(cfg, *session)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "session": session, "messages": messages})
	}
}

func GetPanelChatLatestTask(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		bundle, err := loadLatestPanelChatTask(cfg, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if bundle == nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "task": nil})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "task": bundle})
	}
}

func ControlPanelChatTask(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			AgentID       string `json:"agentId"`
			Action        string `json:"action"`
			TargetAgentID string `json:"targetAgentId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid request"})
			return
		}
		bundle, err := loadLatestPanelChatTask(cfg, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if bundle == nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "task not found"})
			return
		}
		agentID := strings.TrimSpace(req.AgentID)
		if agentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "agent id required"})
			return
		}
		action := strings.TrimSpace(req.Action)
		now := time.Now().UTC().Format(time.RFC3339)
		path := panelChatTaskSubtaskPath(cfg, bundle.TaskID, agentID)
		var sub panelChatSubtask
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, &sub); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
		} else {
			sub = panelChatSubtask{AgentID: agentID, Role: "executor", Status: "pending", AssignedAt: now, InputRef: "spec.md", Summary: "子任务对象由主控补建"}
		}
		switch action {
		case "retry":
			sub.Status = "running"
			sub.StartedAt = now
			sub.Summary = "主控已请求重新执行"
			_ = appendPanelChatTaskEvent(cfg, bundle.TaskID, panelChatTaskEvent{Time: now, Type: "subtask_retry_requested", AgentID: bundle.Meta.ControllerAgentID, TargetAgentID: agentID, Message: fmt.Sprintf("已请求 %s 重试", agentID)})
			userMessage := extractPanelChatUserRequest(bundle.Spec)
			session := panelChatSession{ID: bundle.Meta.SessionID, AgentID: agentID, ControllerAgentID: bundle.Meta.ControllerAgentID, PreferredAgentID: bundle.Meta.PreferredAgentID, ParticipantAgentIDs: bundle.Meta.ParticipantAgentIDs, ChatType: bundle.Meta.ChatType}
			prompt := userMessage
			switch agentID {
			case "coding":
				prompt = buildPanelChatSpecialistPrompt(session, userMessage, agentID)
			case "writer":
				prompt = buildPanelChatWriterPrompt(session, userMessage, bundle.Artifacts["coding"])
			case "reviewer":
				combined := strings.TrimSpace(strings.TrimSpace(bundle.Artifacts["coding"]) + "\n\n" + strings.TrimSpace(bundle.Artifacts["writer"]))
				prompt = buildPanelChatReviewerPrompt(session, userMessage, combined)
			case "main":
				combined := strings.TrimSpace(strings.TrimSpace(bundle.Artifacts["coding"]) + "\n\n" + strings.TrimSpace(bundle.Artifacts["writer"]))
				prompt = buildPanelChatControllerPromptWithReview(session, userMessage, bundle.Meta.PreferredAgentID, combined, bundle.Artifacts["reviewer"])
			}
			report, _, execErr := runPanelChatMessage(c.Request.Context(), cfg, panelChatSession{ID: bundle.Meta.SessionID, AgentID: agentID, OpenClawSessionID: fmt.Sprintf("%s-%s", bundle.Meta.SessionID, agentID)}, prompt)
			if execErr != nil {
				sub.Status = "failed"
				sub.FinishedAt = now
				sub.Summary = execErr.Error()
				_ = appendPanelChatTaskEvent(cfg, bundle.TaskID, panelChatTaskEvent{Time: now, Type: "subtask_retry_failed", AgentID: agentID, Message: execErr.Error()})
			} else {
				sub.Status = "done"
				sub.FinishedAt = time.Now().UTC().Format(time.RFC3339)
				sub.Summary = "主控重试后已完成"
				_ = writePanelChatArtifact(cfg, bundle.TaskID, agentID, report)
				_ = appendPanelChatTaskEvent(cfg, bundle.TaskID, panelChatTaskEvent{Time: now, Type: "subtask_redone", AgentID: agentID, Message: fmt.Sprintf("%s 已完成重试", agentID)})
			}
		case "abort":
			sub.Status = "aborted"
			sub.FinishedAt = now
			sub.Summary = "主控已终止该子任务"
			_ = appendPanelChatTaskEvent(cfg, bundle.TaskID, panelChatTaskEvent{Time: now, Type: "subtask_aborted", AgentID: bundle.Meta.ControllerAgentID, TargetAgentID: agentID, Message: fmt.Sprintf("已终止 %s", agentID)})
		case "reassign":
			target := strings.TrimSpace(req.TargetAgentID)
			if target == "" {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "target agent id required"})
				return
			}
			sub.Status = "returned"
			sub.Summary = fmt.Sprintf("已改派给 %s", target)
			targetSubtask := panelChatSubtask{AgentID: target, Role: "executor", Status: "running", AssignedAt: now, StartedAt: now, InputRef: "spec.md", Summary: fmt.Sprintf("从 %s 改派而来", agentID)}
			_ = writePanelChatSubtask(cfg, bundle.TaskID, targetSubtask)
			_ = appendPanelChatTaskEvent(cfg, bundle.TaskID, panelChatTaskEvent{Time: now, Type: "subtask_reassigned", AgentID: bundle.Meta.ControllerAgentID, TargetAgentID: target, Message: fmt.Sprintf("已将 %s 改派给 %s", agentID, target)})
			userMessage := extractPanelChatUserRequest(bundle.Spec)
			session := panelChatSession{ID: bundle.Meta.SessionID, AgentID: target, ControllerAgentID: bundle.Meta.ControllerAgentID, PreferredAgentID: target, ParticipantAgentIDs: bundle.Meta.ParticipantAgentIDs, ChatType: bundle.Meta.ChatType}
			prompt := buildPanelChatSpecialistPrompt(session, userMessage, target)
			if target == "writer" {
				prompt = buildPanelChatWriterPrompt(session, userMessage, bundle.Artifacts["coding"])
			} else if target == "reviewer" {
				combined := strings.TrimSpace(strings.TrimSpace(bundle.Artifacts["coding"]) + "\n\n" + strings.TrimSpace(bundle.Artifacts["writer"]))
				prompt = buildPanelChatReviewerPrompt(session, userMessage, combined)
			}
			report, _, execErr := runPanelChatMessage(c.Request.Context(), cfg, panelChatSession{ID: bundle.Meta.SessionID, AgentID: target, OpenClawSessionID: fmt.Sprintf("%s-%s", bundle.Meta.SessionID, target)}, prompt)
			if execErr != nil {
				targetSubtask.Status = "failed"
				targetSubtask.FinishedAt = now
				targetSubtask.Summary = execErr.Error()
				_ = appendPanelChatTaskEvent(cfg, bundle.TaskID, panelChatTaskEvent{Time: now, Type: "subtask_reassign_failed", AgentID: target, Message: execErr.Error()})
			} else {
				targetSubtask.Status = "done"
				targetSubtask.FinishedAt = time.Now().UTC().Format(time.RFC3339)
				targetSubtask.OutputRef = filepath.Join("artifacts", target+".md")
				targetSubtask.Summary = fmt.Sprintf("%s 已完成改派任务", target)
				_ = writePanelChatArtifact(cfg, bundle.TaskID, target, report)
				_ = appendPanelChatTaskEvent(cfg, bundle.TaskID, panelChatTaskEvent{Time: now, Type: "subtask_reassigned_done", AgentID: target, Message: fmt.Sprintf("%s 已完成改派任务", target)})
			}
			_ = writePanelChatSubtask(cfg, bundle.TaskID, targetSubtask)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "unsupported action"})
			return
		}
		if err := writePanelChatSubtask(cfg, bundle.TaskID, sub); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func RenamePanelChatSession(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "title required"})
			return
		}
		title := buildPanelChatTitle(req.Title)
		session, err := updatePanelChatSessionState(cfg, c.Param("id"), func(item *panelChatSession) {
			item.Title = title
			item.UpdatedAt = time.Now().UnixMilli()
		})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "session": session})
	}
}

func SendPanelChatMessage(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Message          string `json:"message"`
			PreferredAgentID string `json:"preferredAgentId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Message) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "message required"})
			return
		}
		if !cfg.OpenClawInstalled() {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "OpenClaw 未安装或未配置"})
			return
		}

		sessions, err := loadPanelChatSessions(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		_, session := findPanelChatSession(sessions, c.Param("id"))
		if session == nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "会话不存在"})
			return
		}
		if _, err := updatePanelChatSessionState(cfg, session.ID, func(item *panelChatSession) {
			item.Processing = true
			if item.ChatType == "group" {
				item.Status = "dispatching"
			}
			item.UpdatedAt = time.Now().UnixMilli()
			item.LastMessage = strings.TrimSpace(req.Message)
			if strings.TrimSpace(req.PreferredAgentID) != "" {
				item.PreferredAgentID = strings.TrimSpace(req.PreferredAgentID)
			}
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		var (
			reply           string
			actualSessionID string
			runErr          error
			messages        []map[string]interface{}
			groupSessionIDs map[string]string
			taskID          string
		)
		if session.ChatType == "group" {
			taskID, err = createPanelChatTask(cfg, *session, strings.TrimSpace(req.Message))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
			messages, reply, groupSessionIDs, runErr = runPanelChatGroupWorkflow(c.Request.Context(), cfg, *session, taskID, strings.TrimSpace(req.Message), strings.TrimSpace(req.PreferredAgentID))
		} else {
			reply, actualSessionID, runErr = runPanelChatMessage(c.Request.Context(), cfg, *session, strings.TrimSpace(req.Message))
		}
		_, _ = updatePanelChatSessionState(cfg, session.ID, func(item *panelChatSession) {
			item.Processing = false
			if item.ChatType == "group" {
				item.Status = "done"
			} else {
				item.Status = "idle"
			}
			if strings.TrimSpace(actualSessionID) != "" {
				item.OpenClawSessionID = strings.TrimSpace(actualSessionID)
			}
			if len(groupSessionIDs) > 0 {
				item.GroupAgentSessionIDs = groupSessionIDs
			}
			if strings.TrimSpace(req.PreferredAgentID) != "" {
				item.PreferredAgentID = strings.TrimSpace(req.PreferredAgentID)
			}
		})
		timedOut := false
		if runErr != nil {
			if taskID != "" {
				meta := panelChatTaskMeta{
					ID:                  taskID,
					SessionID:           session.ID,
					ChatType:            session.ChatType,
					ControllerAgentID:   session.ControllerAgentID,
					PreferredAgentID:    session.PreferredAgentID,
					ParticipantAgentIDs: append([]string{}, session.ParticipantAgentIDs...),
					Title:               buildPanelChatTitle(req.Message),
					Status:              "failed",
					CurrentStage:        "failed",
					CreatedAt:           time.Now().UTC().Format(time.RFC3339),
					UpdatedAt:           time.Now().UTC().Format(time.RFC3339),
				}
				_ = writePanelChatTaskMeta(cfg, taskID, meta)
				_ = appendPanelChatTaskEvent(cfg, taskID, panelChatTaskEvent{Time: time.Now().UTC().Format(time.RFC3339), Type: "task_failed", AgentID: session.ControllerAgentID, Message: runErr.Error()})
				_ = writePanelChatTaskFailure(cfg, taskID, runErr.Error())
			}
			if errors.Is(runErr, errPanelChatCanceled) {
				c.JSON(http.StatusOK, gin.H{"ok": false, "canceled": true})
				return
			}
			if errors.Is(runErr, errPanelChatTimeout) {
				timedOut = true
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": runErr.Error()})
				return
			}
		}
		if session.ChatType == "group" {
			if err := savePanelChatGroupMessages(cfg, session.ID, messages); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
		} else {
			existingMessages, readErr := loadPanelChatDirectMessages(cfg, session.ID)
			if readErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": readErr.Error()})
				return
			}
			if strings.TrimSpace(actualSessionID) != "" {
				session.OpenClawSessionID = strings.TrimSpace(actualSessionID)
			}
			rawMessages, historyErr := readSessionMessages(panelChatRuntimeSessionFile(cfg, *session), 400)
			if historyErr == nil {
				latestExchange := extractLatestPanelChatExchange(rawMessages, strings.TrimSpace(req.Message))
				if len(latestExchange) > 0 {
					existingMessages = mergePanelChatTranscripts(existingMessages, latestExchange)
				}
			}
			if len(existingMessages) == 0 && strings.TrimSpace(reply) != "" {
				userTime := time.Now().UTC()
				assistantTime := userTime.Add(1200 * time.Millisecond)
				existingMessages = append(existingMessages,
					map[string]interface{}{"id": fmt.Sprintf("user-%d", time.Now().UnixNano()), "role": "user", "content": strings.TrimSpace(req.Message), "timestamp": userTime.Format(time.RFC3339)},
					map[string]interface{}{"id": fmt.Sprintf("assistant-%d", time.Now().UnixNano()+1), "role": "assistant", "content": strings.TrimSpace(reply), "timestamp": assistantTime.Format(time.RFC3339)},
				)
			}
			if err := savePanelChatDirectMessages(cfg, session.ID, existingMessages); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
			messages = existingMessages
		}

		reply = panelChatEffectiveReply(messages, reply)
		if taskID != "" && !timedOut {
			meta := panelChatTaskMeta{
				ID:                  taskID,
				SessionID:           session.ID,
				ChatType:            session.ChatType,
				ControllerAgentID:   session.ControllerAgentID,
				PreferredAgentID:    session.PreferredAgentID,
				ParticipantAgentIDs: append([]string{}, session.ParticipantAgentIDs...),
				Title:               buildPanelChatTitle(req.Message),
				Status:              "done",
				CurrentStage:        "final",
				CreatedAt:           time.Now().UTC().Format(time.RFC3339),
				UpdatedAt:           time.Now().UTC().Format(time.RFC3339),
			}
			_ = writePanelChatTaskMeta(cfg, taskID, meta)
			if strings.TrimSpace(reply) != "" && !strings.Contains(strings.ToLower(reply), "timed out") && !strings.Contains(strings.ToLower(reply), "connection error") {
				_ = writePanelChatTaskResult(cfg, taskID, reply)
			}
		}
		if len(messages) == 0 && strings.TrimSpace(reply) != "" {
			userTime := time.Now().UTC()
			assistantTime := userTime.Add(1200 * time.Millisecond)
			messages = append(messages,
				map[string]interface{}{"id": fmt.Sprintf("user-%d", time.Now().UnixNano()), "role": "user", "content": strings.TrimSpace(req.Message), "timestamp": userTime.Format(time.RFC3339)},
				map[string]interface{}{"id": fmt.Sprintf("assistant-%d", time.Now().UnixNano()+1), "role": "assistant", "content": strings.TrimSpace(reply), "timestamp": assistantTime.Format(time.RFC3339)},
			)
		}
		updated, err := updatePanelChatSessionState(cfg, session.ID, func(item *panelChatSession) {
			item.UpdatedAt = time.Now().UnixMilli()
			item.Processing = false
			item.MessageCount = len(messages)
			item.LastMessage = strings.TrimSpace(req.Message)
			if item.Title == panelChatDefaultTitle && len(messages) > 0 {
				item.Title = buildPanelChatTitle(req.Message)
			}
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":         true,
			"reply":      reply,
			"session":    updated,
			"messages":   messages,
			"processing": timedOut,
		})
	}
}

func DeletePanelChatSession(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := loadPanelChatSessions(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		idx, session := findPanelChatSession(sessions, c.Param("id"))
		if session == nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "会话不存在"})
			return
		}
		sessions = append(sessions[:idx], sessions[idx+1:]...)
		if err := savePanelChatSessions(cfg, sessions); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if session.ChatType == "group" {
			_ = os.Remove(panelChatGroupMessagesPath(cfg, session.ID))
		} else {
			_ = os.Remove(panelChatDirectMessagesPath(cfg, session.ID))
			_ = os.Remove(panelChatSessionFile(cfg, session.AgentID, session.OpenClawSessionID))
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
