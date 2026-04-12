package handler

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
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
	"github.com/zhaoxinyi02/ClawPanel/internal/model"
)

const panelChatDefaultTitle = "新对话"

var errPanelChatTimeout = errors.New("panel chat timeout")
var errPanelChatCanceled = errors.New("panel chat canceled")

var panelChatTimestampPrefixRe = regexp.MustCompile(`^\[[^\]]+\]\s*`)

const panelChatScannerMaxTokenSize = 16 * 1024 * 1024

var panelChatActiveRuns sync.Map
var panelChatSessionBusy sync.Map
var panelChatSessionLocks sync.Map
var panelChatSessionsFileMu sync.Mutex

type panelChatActiveRun struct {
	cancel context.CancelFunc
	pid    int
}

type panelChatSession struct {
	ID                string `json:"id"`
	OpenClawSessionID string `json:"openclawSessionId"`
	AgentID           string `json:"agentId"`
	ChatType          string `json:"chatType"`
	Title             string `json:"title"`
	TargetID          string `json:"targetId,omitempty"`
	TargetName        string `json:"targetName,omitempty"`
	CreatedAt         int64  `json:"createdAt"`
	UpdatedAt         int64  `json:"updatedAt"`
	Processing        bool   `json:"processing,omitempty"`
	CurrentAgentID    string `json:"currentAgentId,omitempty"`
	CurrentAgentName  string `json:"currentAgentName,omitempty"`
	SummaryAgentID    string `json:"summaryAgentId,omitempty"`
	MessageCount      int    `json:"messageCount"`
	LastMessage       string `json:"lastMessage,omitempty"`
	ParticipantCount  int    `json:"participantCount,omitempty"`
}

type panelChatParticipantView struct {
	AgentID           string `json:"agentId"`
	Name              string `json:"name,omitempty"`
	RoleType          string `json:"roleType"`
	OrderIndex        int    `json:"orderIndex"`
	AutoReply         bool   `json:"autoReply"`
	IsSummary         bool   `json:"isSummary"`
	Enabled           bool   `json:"enabled"`
	OpenClawSessionID string `json:"openclawSessionId,omitempty"`
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

func panelChatSessionsPath(cfg *config.Config) string {
	return filepath.Join(cfg.DataDir, "panel-chat", "sessions.json")
}

func panelChatMessagesPath(cfg *config.Config, sessionID string) string {
	return filepath.Join(cfg.DataDir, "panel-chat", "messages", sessionID+".json")
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func loadPanelChatMessages(cfg *config.Config, sessionID string) ([]map[string]interface{}, error) {
	path := panelChatMessagesPath(cfg, sessionID)
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

func savePanelChatMessages(cfg *config.Config, sessionID string, messages []map[string]interface{}) error {
	path := panelChatMessagesPath(cfg, sessionID)
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0o644)
}

func loadPanelChatSessionsUnlocked(cfg *config.Config) ([]panelChatSession, error) {
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
	normalized := make([]panelChatSession, 0, len(sessions))
	for i := range sessions {
		sessions[i].ChatType = normalizePanelChatType(sessions[i].ChatType)
		if strings.TrimSpace(sessions[i].AgentID) == "" {
			sessions[i].AgentID = loadDefaultAgentID(cfg)
		}
		if strings.TrimSpace(sessions[i].OpenClawSessionID) == "" {
			sessions[i].OpenClawSessionID = sessions[i].ID
		}
		if strings.TrimSpace(sessions[i].Title) == "" {
			sessions[i].Title = panelChatDefaultTitle
		}
		normalized = append(normalized, sessions[i])
	}
	return normalized, nil
}

func loadPanelChatSessions(cfg *config.Config) ([]panelChatSession, error) {
	return loadPanelChatSessionsUnlocked(cfg)
}

func savePanelChatSessionsUnlocked(cfg *config.Config, sessions []panelChatSession) error {
	path := panelChatSessionsPath(cfg)
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0o644)
}

func savePanelChatSessions(cfg *config.Config, sessions []panelChatSession) error {
	return savePanelChatSessionsUnlocked(cfg, sessions)
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
	value := strings.ToLower(strings.TrimSpace(chatType))
	if value == "group" {
		return "group"
	}
	return "direct"
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

func panelChatSessionLock(sessionID string) *sync.Mutex {
	lock, _ := panelChatSessionLocks.LoadOrStore(sessionID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func panelChatTryAcquireBusy(sessionID string) bool {
	_, loaded := panelChatSessionBusy.LoadOrStore(sessionID, struct{}{})
	return !loaded
}

func panelChatReleaseBusy(sessionID string) {
	panelChatSessionBusy.Delete(sessionID)
}

func ensurePanelChatRuntime(cfg *config.Config, session panelChatSession) (stateDir, configPath, workDir string, err error) {
	root := panelChatRuntimeRoot(cfg, session.ID)
	stateDir = filepath.Join(root, "state")
	workDir = root
	configPath = filepath.Join(stateDir, "openclaw.json")
	if err = os.MkdirAll(stateDir, 0o755); err != nil {
		return "", "", "", err
	}
	sourceConfigPath := filepath.Join(cfg.OpenClawDir, "openclaw.json")
	if err = copyFile(sourceConfigPath, configPath); err != nil {
		return "", "", "", err
	}
	srcAgentDir := filepath.Join(cfg.OpenClawDir, "agents", session.AgentID)
	scopedAgentID := panelChatScopedAgentID(session.ID, session.AgentID)
	dstAgentDir := filepath.Join(stateDir, "agents", session.AgentID)
	if err = copyDirWithoutSessions(srcAgentDir, dstAgentDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", "", err
	}
	_ = os.MkdirAll(filepath.Join(dstAgentDir, "sessions"), 0o755)
	if err = os.MkdirAll(filepath.Join(workDir, "openclaw-work"), 0o755); err != nil {
		return "", "", "", err
	}
	srcWorkspace := resolveAgentWorkspacePath(cfg, session.AgentID)
	runtimeWorkspace := filepath.ToSlash(filepath.Join("openclaw-work", scopedAgentID))
	dstWorkspace := filepath.Join(workDir, "openclaw-work", scopedAgentID)
	if strings.TrimSpace(srcWorkspace) == "" {
		if err = os.MkdirAll(dstWorkspace, 0o755); err != nil {
			return "", "", "", err
		}
	} else {
		runtimeWorkspace = filepath.ToSlash(srcWorkspace)
	}
	if err = rewritePanelChatRuntimeConfig(cfg, sourceConfigPath, configPath, session, runtimeWorkspace); err != nil {
		return "", "", "", err
	}
	return stateDir, configPath, workDir, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func syncFileIfNewer(src, dst string, srcInfo os.FileInfo) error {
	if dstInfo, err := os.Stat(dst); err == nil {
		if dstInfo.Size() == srcInfo.Size() && !srcInfo.ModTime().After(dstInfo.ModTime()) {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, time.Now(), srcInfo.ModTime())
}

func syncDirPreferSource(src, dst string) error {
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
		return syncFileIfNewer(path, target, info)
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

func rewritePanelChatRuntimeConfig(cfg *config.Config, srcConfigPath, dstConfigPath string, session panelChatSession, runtimeWorkspace string) error {
	data, err := os.ReadFile(srcConfigPath)
	if err != nil {
		return err
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	agentsMap := readMap(obj["agents"])
	list := materializeAgentList(cfg, obj)
	newList := make([]interface{}, 0, 1)
	for _, agent := range list {
		if strings.TrimSpace(toString(agent["id"])) != session.AgentID {
			continue
		}
		cloned := map[string]interface{}{}
		for k, v := range agent {
			cloned[k] = v
		}
		cloned["id"] = session.AgentID
		if strings.TrimSpace(runtimeWorkspace) != "" {
			cloned["workspace"] = filepath.ToSlash(runtimeWorkspace)
		} else {
			cloned["workspace"] = filepath.ToSlash(filepath.Join("openclaw-work", panelChatScopedAgentID(session.ID, session.AgentID)))
		}
		cloned["default"] = true
		newList = append(newList, cloned)
		break
	}
	if len(newList) == 0 && strings.TrimSpace(session.AgentID) != "" {
		workspace := strings.TrimSpace(runtimeWorkspace)
		if workspace == "" {
			workspace = filepath.ToSlash(filepath.Join("openclaw-work", panelChatScopedAgentID(session.ID, session.AgentID)))
		}
		newList = append(newList, map[string]interface{}{
			"id":        session.AgentID,
			"workspace": filepath.ToSlash(workspace),
			"default":   true,
		})
	}
	agentsMap["list"] = newList
	obj["agents"] = agentsMap
	delete(obj, "channels")
	delete(obj, "plugins")
	encoded, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dstConfigPath, append(encoded, '\n'), 0o644)
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

func loadPanelChatAgentNameMap(cfg *config.Config) map[string]string {
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig == nil {
		return map[string]string{}
	}
	list := materializeAgentList(cfg, ocConfig)
	nameMap := make(map[string]string, len(list))
	for _, item := range list {
		id := strings.TrimSpace(toString(item["id"]))
		if id == "" {
			continue
		}
		name := strings.TrimSpace(toString(item["name"]))
		identity := readMap(item["identity"])
		if name == "" {
			name = strings.TrimSpace(toString(identity["name"]))
		}
		if name == "" {
			name = id
		}
		nameMap[id] = name
	}
	return nameMap
}

func normalizePanelChatParticipantInput(agentIDs []string, primaryAgentID, summaryAgentID string) []model.PanelChatParticipant {
	seen := map[string]struct{}{}
	items := make([]model.PanelChatParticipant, 0, len(agentIDs)+1)
	appendItem := func(agentID, roleType string, isSummary bool) {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			return
		}
		if _, ok := seen[agentID]; ok {
			return
		}
		seen[agentID] = struct{}{}
		items = append(items, model.PanelChatParticipant{AgentID: agentID, RoleType: roleType, AutoReply: true, Enabled: true, IsSummary: isSummary})
	}
	for _, agentID := range agentIDs {
		appendItem(agentID, "assistant", false)
	}
	if len(items) == 0 {
		appendItem(primaryAgentID, "assistant", false)
	}
	summaryAgentID = strings.TrimSpace(summaryAgentID)
	if summaryAgentID != "" {
		if _, ok := seen[summaryAgentID]; ok {
			for i := range items {
				if items[i].AgentID == summaryAgentID {
					items[i].IsSummary = true
					items[i].RoleType = "summary"
				}
			}
		} else {
			appendItem(summaryAgentID, "summary", true)
		}
	}
	return items
}

func clonePanelChatParticipants(items []panelChatParticipantView) []model.PanelChatParticipant {
	cloned := make([]model.PanelChatParticipant, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, model.PanelChatParticipant{
			AgentID:           item.AgentID,
			RoleType:          item.RoleType,
			OrderIndex:        item.OrderIndex,
			AutoReply:         item.AutoReply,
			IsSummary:         item.IsSummary,
			Enabled:           item.Enabled,
			OpenClawSessionID: item.OpenClawSessionID,
		})
	}
	return cloned
}

func persistPanelChatParticipantSessions(db *sql.DB, sessionID string, updates map[string]string) error {
	for agentID, openClawSessionID := range updates {
		if strings.TrimSpace(agentID) == "" || strings.TrimSpace(openClawSessionID) == "" {
			continue
		}
		if err := model.UpdatePanelChatParticipantOpenClawSession(db, sessionID, agentID, openClawSessionID); err != nil {
			return err
		}
	}
	return nil
}

func validatePanelChatParticipants(cfg *config.Config, primaryAgentID string, participantIDs []string, summaryAgentID string) error {
	_, knownSet := loadAgentIDs(cfg)
	validateKnown := func(agentID string, field string) error {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			return nil
		}
		if err := validateAgentID(agentID); err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		if _, ok := knownSet[agentID]; !ok {
			return fmt.Errorf("%s: agent %s 不存在", field, agentID)
		}
		return nil
	}
	if err := validateKnown(primaryAgentID, "agentId"); err != nil {
		return err
	}
	participantSet := map[string]struct{}{}
	for _, agentID := range participantIDs {
		agentID = strings.TrimSpace(agentID)
		if err := validateKnown(agentID, "agentIds"); err != nil {
			return err
		}
		if agentID != "" {
			participantSet[agentID] = struct{}{}
		}
	}
	if err := validateKnown(summaryAgentID, "summaryAgentId"); err != nil {
		return err
	}
	if strings.TrimSpace(summaryAgentID) != "" {
		if _, ok := participantSet[strings.TrimSpace(summaryAgentID)]; !ok {
			return fmt.Errorf("summaryAgentId 必须包含在 agentIds 中")
		}
	}
	return nil
}

func loadPanelChatParticipants(db *sql.DB, cfg *config.Config, session panelChatSession) ([]panelChatParticipantView, error) {
	items, err := model.ListPanelChatParticipants(db, session.ID)
	if err != nil {
		return nil, err
	}
	nameMap := loadPanelChatAgentNameMap(cfg)
	views := make([]panelChatParticipantView, 0, len(items))
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		views = append(views, panelChatParticipantView{
			AgentID:           item.AgentID,
			Name:              nameMap[item.AgentID],
			RoleType:          item.RoleType,
			OrderIndex:        item.OrderIndex,
			AutoReply:         item.AutoReply,
			IsSummary:         item.IsSummary,
			Enabled:           item.Enabled,
			OpenClawSessionID: item.OpenClawSessionID,
		})
	}
	if len(views) == 0 && strings.TrimSpace(session.AgentID) != "" {
		views = append(views, panelChatParticipantView{AgentID: session.AgentID, Name: nameMap[session.AgentID], RoleType: "assistant", OrderIndex: 0, AutoReply: true, Enabled: true})
	}
	return views, nil
}

func decoratePanelChatSession(db *sql.DB, cfg *config.Config, session *panelChatSession) {
	if session == nil {
		return
	}
	participants, err := loadPanelChatParticipants(db, cfg, *session)
	if err != nil {
		if strings.TrimSpace(session.AgentID) != "" {
			session.ParticipantCount = 1
		}
		return
	}
	session.ParticipantCount = len(participants)
	if session.ChatType == "group" {
		for _, item := range participants {
			if item.IsSummary {
				session.SummaryAgentID = item.AgentID
				break
			}
		}
	}
}

func panelChatSessionFile(cfg *config.Config, agentID, openclawSessionID string) string {
	return filepath.Join(resolveAgentSessionsDir(cfg, agentID), openclawSessionID+".jsonl")
}

func panelChatRuntimeSessionFile(cfg *config.Config, session panelChatSession) string {
	return filepath.Join(panelChatRuntimeRoot(cfg, session.ID), "state", "agents", session.AgentID, "sessions", session.OpenClawSessionID+".jsonl")
}

func panelChatSessionFileCandidates(cfg *config.Config, session panelChatSession) []string {
	paths := make([]string, 0, 2)
	addPath := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		for _, existing := range paths {
			if existing == p {
				return
			}
		}
		paths = append(paths, p)
	}
	addPath(panelChatSessionFile(cfg, session.AgentID, session.OpenClawSessionID))
	addPath(panelChatRuntimeSessionFile(cfg, session))
	return paths
}

func readPanelChatSessionMessagesWithFallback(cfg *config.Config, session panelChatSession, limit int) ([]map[string]interface{}, error) {
	var lastErr error
	for _, filePath := range panelChatSessionFileCandidates(cfg, session) {
		messages, err := readSessionMessages(filePath, limit)
		if err == nil {
			return messages, nil
		}
		lastErr = err
	}
	return nil, lastErr
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
	localMessages, err := loadPanelChatMessages(cfg, session.ID)
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
			"id":         entry["id"],
			"role":       role,
			"senderType": map[string]string{"user": "user", "assistant": "agent"}[role],
			"content":    content,
			"timestamp":  ts,
		}
		if role == "assistant" {
			message["agentId"] = session.AgentID
			message["messageType"] = "chat"
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
	args = append(args, "agent", "--agent", session.AgentID, "--to", panelChatVirtualTarget(session), "--session-id", session.OpenClawSessionID, "--message", message, "--json")
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
	} else if cfg.OpenClawWork != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("OPENCLAW_WORK_DIR=%s", cfg.OpenClawWork))
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

func buildGroupChatPrompt(agentName string, participant panelChatParticipantView, transcript []map[string]interface{}, latestUserMessage string) string {
	lines := []string{
		fmt.Sprintf("你正在参与一个多 AI 角色群聊。你的身份是：%s。", strings.TrimSpace(agentName)),
		"请只以你自己的身份回复，不要伪造其他角色的发言，不要输出多余前缀。",
	}
	if participant.IsSummary {
		lines = append(lines, "你是本轮总结 AI。请基于对话和其他 AI 的回复，给出面向用户的最终汇总。")
	} else {
		lines = append(lines, "请结合已有聊天上下文，直接补充你的观点或答案。")
	}
	lines = append(lines, "", "以下是共享消息流（按时间顺序，最近消息在后）：")
	start := 0
	if len(transcript) > 12 {
		start = len(transcript) - 12
	}
	for _, item := range transcript[start:] {
		role := strings.TrimSpace(toString(item["role"]))
		senderType := strings.TrimSpace(toString(item["senderType"]))
		content := strings.TrimSpace(toString(item["content"]))
		if content == "" {
			continue
		}
		speaker := "用户"
		if senderType == "agent" || role == "assistant" {
			agentID := strings.TrimSpace(toString(item["agentId"]))
			agentNameInMsg := strings.TrimSpace(toString(item["agentName"]))
			if agentNameInMsg != "" && agentID != "" {
				speaker = fmt.Sprintf("AI %s(%s)", agentNameInMsg, agentID)
			} else if agentID != "" {
				speaker = fmt.Sprintf("AI %s", agentID)
			} else {
				speaker = "AI"
			}
		} else if senderType == "system" || role == "system" {
			speaker = "系统"
		}
		lines = append(lines, fmt.Sprintf("- %s：%s", speaker, content))
	}
	if strings.TrimSpace(latestUserMessage) != "" {
		lines = append(lines, "", "用户当前最新输入：", latestUserMessage)
	}
	return strings.Join(lines, "\n")
}

func executeGroupPanelChat(ctx context.Context, cfg *config.Config, session panelChatSession, participants []panelChatParticipantView, existingMessages []map[string]interface{}, userMessage string) ([]map[string]interface{}, string, map[string]string, error) {
	nameMap := loadPanelChatAgentNameMap(cfg)
	messages := append([]map[string]interface{}{}, existingMessages...)
	sessionIDs := map[string]string{}
	userTimestamp := time.Now().UTC().Format(time.RFC3339)
	userEntry := map[string]interface{}{
		"id":          fmt.Sprintf("user-%d", time.Now().UnixNano()),
		"role":        "user",
		"senderType":  "user",
		"messageType": "chat",
		"content":     strings.TrimSpace(userMessage),
		"timestamp":   userTimestamp,
	}
	messages = append(messages, userEntry)
	lastReply := ""
	for _, participant := range participants {
		if !participant.Enabled || !participant.AutoReply {
			continue
		}
		agentName := participant.Name
		if strings.TrimSpace(agentName) == "" {
			agentName = nameMap[participant.AgentID]
		}
		_, _ = updatePanelChatSessionState(cfg, session.ID, func(item *panelChatSession) {
			item.Processing = true
			item.CurrentAgentID = participant.AgentID
			item.CurrentAgentName = agentName
			item.UpdatedAt = time.Now().UnixMilli()
		})
		prompt := buildGroupChatPrompt(agentName, participant, messages, userMessage)
		runtimeSession := session
		runtimeSession.AgentID = participant.AgentID
		runtimeSession.OpenClawSessionID = strings.TrimSpace(participant.OpenClawSessionID)
		if runtimeSession.OpenClawSessionID == "" {
			runtimeSession.OpenClawSessionID = fmt.Sprintf("%s-%s", session.ID, participant.AgentID)
		}
		reply, actualSessionID, err := runPanelChatMessage(ctx, cfg, runtimeSession, prompt)
		if strings.TrimSpace(reply) == "" || err != nil {
			if rawMessages, historyErr := readPanelChatSessionMessagesWithFallback(cfg, runtimeSession, 200); historyErr == nil {
				recovered := extractLatestAssistantReply(rawMessages, prompt)
				if strings.TrimSpace(recovered) != "" {
					reply = recovered
					if errors.Is(err, errPanelChatTimeout) {
						err = nil
					}
				}
			}
		}
		if strings.TrimSpace(actualSessionID) != "" {
			sessionIDs[participant.AgentID] = strings.TrimSpace(actualSessionID)
		}
		if err != nil {
			return messages, lastReply, sessionIDs, err
		}
		reply = sanitizePanelChatContent(reply)
		if strings.TrimSpace(reply) == "" {
			continue
		}
		lastReply = reply
		messageType := "chat"
		if participant.IsSummary {
			messageType = "summary"
		}
		messages = append(messages, map[string]interface{}{
			"id":          fmt.Sprintf("agent-%s-%d", participant.AgentID, time.Now().UnixNano()),
			"role":        "assistant",
			"senderType":  "agent",
			"agentId":     participant.AgentID,
			"agentName":   agentName,
			"messageType": messageType,
			"content":     reply,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
	}
	return messages, lastReply, sessionIDs, nil
}

func CancelPanelChatMessage(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("id"))
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "session id required"})
			return
		}
		if _, ok := panelChatSessionBusy.Load(sessionID); !ok {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "当前会话没有正在进行的请求"})
			return
		}
		if active, ok := panelChatActiveRuns.Load(sessionID); ok {
			active.(panelChatActiveRun).cancel()
		}
		lock := panelChatSessionLock(sessionID)
		lock.Lock()
		defer lock.Unlock()
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

func extractLatestAssistantReply(messages []map[string]interface{}, userMessage string) string {
	exchange := extractLatestPanelChatExchange(messages, userMessage)
	for i := len(exchange) - 1; i >= 0; i-- {
		role, _ := exchange[i]["role"].(string)
		if role != "assistant" {
			continue
		}
		content := sanitizePanelChatContent(toString(exchange[i]["content"]))
		if strings.TrimSpace(content) != "" {
			return content
		}
	}
	return ""
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
	panelChatSessionsFileMu.Lock()
	defer panelChatSessionsFileMu.Unlock()
	sessions, err := loadPanelChatSessionsUnlocked(cfg)
	if err != nil {
		return nil, err
	}
	idx, session := findPanelChatSession(sessions, sessionID)
	if session == nil {
		return nil, fmt.Errorf("会话不存在")
	}
	mutate(&sessions[idx])
	sortPanelChatSessions(sessions)
	if err := savePanelChatSessionsUnlocked(cfg, sessions); err != nil {
		return nil, err
	}
	for i := range sessions {
		if sessions[i].ID == sessionID {
			return &sessions[i], nil
		}
	}
	return &sessions[0], nil
}

func ListPanelChatSessions(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := loadPanelChatSessions(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		for i := range sessions {
			decoratePanelChatSession(db, cfg, &sessions[i])
		}
		sortPanelChatSessions(sessions)
		c.JSON(http.StatusOK, gin.H{"ok": true, "sessions": sessions})
	}
}

func CreatePanelChatSession(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title          string   `json:"title"`
			ChatType       string   `json:"chatType"`
			AgentID        string   `json:"agentId"`
			AgentIDs       []string `json:"agentIds"`
			SummaryAgentID string   `json:"summaryAgentId"`
			TargetID       string   `json:"targetId"`
			TargetName     string   `json:"targetName"`
		}
		_ = c.ShouldBindJSON(&req)

		agentID := strings.TrimSpace(req.AgentID)
		if agentID == "" {
			agentID = loadDefaultAgentID(cfg)
		}
		if err := validatePanelChatParticipants(cfg, agentID, req.AgentIDs, req.SummaryAgentID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		participants := normalizePanelChatParticipantInput(req.AgentIDs, agentID, req.SummaryAgentID)
		chatType := normalizePanelChatType(req.ChatType)
		if len(participants) > 1 || strings.TrimSpace(req.SummaryAgentID) != "" {
			chatType = "group"
		}
		if chatType == "group" && len(participants) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "group chat requires at least 2 AI participants"})
			return
		}
		now := time.Now().UnixMilli()
		id := fmt.Sprintf("panel-%d", now)
		if chatType == "group" {
			id = fmt.Sprintf("group-%d", now)
		}
		session := panelChatSession{
			ID:                id,
			OpenClawSessionID: id,
			AgentID:           agentID,
			ChatType:          chatType,
			Title:             buildPanelChatTitle(req.Title),
			TargetID:          strings.TrimSpace(req.TargetID),
			TargetName:        strings.TrimSpace(req.TargetName),
			SummaryAgentID:    strings.TrimSpace(req.SummaryAgentID),
			ParticipantCount:  len(participants),
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		if err := model.ReplacePanelChatParticipants(db, session.ID, participants); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		panelChatSessionsFileMu.Lock()
		sessions, err := loadPanelChatSessionsUnlocked(cfg)
		if err == nil {
			sessions = append(sessions, session)
			sortPanelChatSessions(sessions)
			err = savePanelChatSessionsUnlocked(cfg, sessions)
		}
		panelChatSessionsFileMu.Unlock()
		if err != nil {
			_ = model.DeletePanelChatParticipants(db, session.ID)
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		decoratePanelChatSession(db, cfg, &session)
		c.JSON(http.StatusOK, gin.H{"ok": true, "session": session})
	}
}

func GetPanelChatSessionDetail(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
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
		participants, err := loadPanelChatParticipants(db, cfg, *session)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		decoratePanelChatSession(db, cfg, session)
		c.JSON(http.StatusOK, gin.H{"ok": true, "session": session, "messages": messages, "participants": participants})
	}
}

func RenamePanelChatSession(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("id"))
		lock := panelChatSessionLock(sessionID)
		lock.Lock()
		defer lock.Unlock()
		if _, busy := panelChatSessionBusy.Load(sessionID); busy {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "当前会话正在处理中，无法重命名"})
			return
		}
		var req struct {
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "title required"})
			return
		}
		title := buildPanelChatTitle(req.Title)
		session, err := updatePanelChatSessionState(cfg, sessionID, func(item *panelChatSession) {
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

func SendPanelChatMessage(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Message string `json:"message"`
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
		if !panelChatTryAcquireBusy(session.ID) {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "当前会话正在处理中，请稍后重试或先取消当前请求"})
			return
		}
		defer panelChatReleaseBusy(session.ID)
		if _, err := updatePanelChatSessionState(cfg, session.ID, func(item *panelChatSession) {
			item.Processing = true
			item.CurrentAgentID = ""
			item.CurrentAgentName = ""
			item.UpdatedAt = time.Now().UnixMilli()
			item.LastMessage = strings.TrimSpace(req.Message)
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		participants, err := loadPanelChatParticipants(db, cfg, *session)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		existingMessages, readErr := loadPanelChatMessages(cfg, session.ID)
		if readErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": readErr.Error()})
			return
		}
		reply := ""
		actualSessionID := ""
		participantSessionUpdates := map[string]string{}
		var runErr error
		if session.ChatType == "group" || len(participants) > 1 {
			existingMessages, reply, participantSessionUpdates, runErr = executeGroupPanelChat(c.Request.Context(), cfg, *session, participants, existingMessages, strings.TrimSpace(req.Message))
		} else {
			reply, actualSessionID, runErr = runPanelChatMessage(c.Request.Context(), cfg, *session, strings.TrimSpace(req.Message))
			if strings.TrimSpace(actualSessionID) != "" {
				session.OpenClawSessionID = strings.TrimSpace(actualSessionID)
			}
		}
		_, _ = updatePanelChatSessionState(cfg, session.ID, func(item *panelChatSession) {
			item.Processing = false
			item.CurrentAgentID = ""
			item.CurrentAgentName = ""
			if strings.TrimSpace(actualSessionID) != "" {
				item.OpenClawSessionID = strings.TrimSpace(actualSessionID)
			}
		})
		if session.ChatType != "group" && len(participants) <= 1 {
			rawMessages, historyErr := readPanelChatSessionMessagesWithFallback(cfg, *session, 400)
			if historyErr == nil {
				latestExchange := extractLatestPanelChatExchange(rawMessages, strings.TrimSpace(req.Message))
				if len(latestExchange) > 0 {
					existingMessages = mergePanelChatTranscripts(existingMessages, latestExchange)
				}
			}
		}
		if len(existingMessages) == 0 && strings.TrimSpace(reply) != "" {
			userTime := time.Now().UTC()
			assistantTime := userTime.Add(1200 * time.Millisecond)
			existingMessages = append(existingMessages,
				map[string]interface{}{"id": fmt.Sprintf("user-%d", time.Now().UnixNano()), "role": "user", "senderType": "user", "messageType": "chat", "content": strings.TrimSpace(req.Message), "timestamp": userTime.Format(time.RFC3339)},
				map[string]interface{}{"id": fmt.Sprintf("assistant-%d", time.Now().UnixNano()+1), "role": "assistant", "senderType": "agent", "agentId": session.AgentID, "messageType": "chat", "content": strings.TrimSpace(reply), "timestamp": assistantTime.Format(time.RFC3339)},
			)
		}
		if err := savePanelChatMessages(cfg, session.ID, existingMessages); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if len(participantSessionUpdates) > 0 {
			if err := persistPanelChatParticipantSessions(db, session.ID, participantSessionUpdates); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
		}

		reply = panelChatEffectiveReply(existingMessages, reply)
		updated, err := updatePanelChatSessionState(cfg, session.ID, func(item *panelChatSession) {
			item.UpdatedAt = time.Now().UnixMilli()
			item.Processing = false
			item.MessageCount = len(existingMessages)
			item.LastMessage = strings.TrimSpace(req.Message)
			if item.Title == panelChatDefaultTitle && len(existingMessages) > 0 {
				item.Title = buildPanelChatTitle(req.Message)
			}
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		decoratePanelChatSession(db, cfg, updated)
		participants, _ = loadPanelChatParticipants(db, cfg, *updated)
		if runErr != nil {
			if errors.Is(runErr, errPanelChatCanceled) {
				c.JSON(http.StatusOK, gin.H{"ok": false, "canceled": true, "session": updated, "messages": existingMessages, "participants": participants})
				return
			}
			if !errors.Is(runErr, errPanelChatTimeout) {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": runErr.Error(), "session": updated, "messages": existingMessages, "participants": participants})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":           true,
			"reply":        reply,
			"session":      updated,
			"messages":     existingMessages,
			"participants": participants,
			"processing":   errors.Is(runErr, errPanelChatTimeout),
		})
	}
}

func DeletePanelChatSession(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("id"))
		lock := panelChatSessionLock(sessionID)
		lock.Lock()
		defer lock.Unlock()
		panelChatSessionsFileMu.Lock()
		sessions, err := loadPanelChatSessionsUnlocked(cfg)
		if err != nil {
			panelChatSessionsFileMu.Unlock()
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		idx, session := findPanelChatSession(sessions, sessionID)
		if session == nil {
			panelChatSessionsFileMu.Unlock()
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "会话不存在"})
			return
		}
		if _, busy := panelChatSessionBusy.Load(session.ID); busy {
			panelChatSessionsFileMu.Unlock()
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "当前会话正在处理中，无法删除"})
			return
		}
		currentParticipants, err := loadPanelChatParticipants(db, cfg, *session)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		participantBackup := clonePanelChatParticipants(currentParticipants)
		sessions = append(sessions[:idx], sessions[idx+1:]...)
		if err := model.DeletePanelChatParticipants(db, session.ID); err != nil {
			panelChatSessionsFileMu.Unlock()
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := savePanelChatSessionsUnlocked(cfg, sessions); err != nil {
			panelChatSessionsFileMu.Unlock()
			if len(participantBackup) == 0 {
				participantBackup = normalizePanelChatParticipantInput([]string{session.AgentID}, session.AgentID, session.SummaryAgentID)
			}
			_ = model.ReplacePanelChatParticipants(db, session.ID, participantBackup)
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		panelChatSessionsFileMu.Unlock()
		_ = os.Remove(panelChatMessagesPath(cfg, session.ID))
		_ = os.Remove(panelChatSessionFile(cfg, session.AgentID, session.OpenClawSessionID))
		_ = os.RemoveAll(panelChatRuntimeRoot(cfg, session.ID))
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
