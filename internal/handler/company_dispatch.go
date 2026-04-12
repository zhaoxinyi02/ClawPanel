package handler

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

type companyDispatchStatus string

const (
	companyDispatchPending   companyDispatchStatus = "pending"
	companyDispatchRunning   companyDispatchStatus = "running"
	companyDispatchSucceeded companyDispatchStatus = "succeeded"
	companyDispatchFailed    companyDispatchStatus = "failed"
	companyDispatchSkipped   companyDispatchStatus = "skipped"
)

type companyAgentDispatchRequest struct {
	TeamID            string   `json:"teamId"`
	ManagerAgentID    string   `json:"managerAgentId"`
	Channel           string   `json:"channel,omitempty"`
	AccountID         string   `json:"accountId,omitempty"`
	SessionKind       string   `json:"sessionKind,omitempty"`
	RoutedAgentID     string   `json:"routedAgentId,omitempty"`
	WorkspaceID       string   `json:"workspaceId,omitempty"`
	ExternalChatID    string   `json:"externalChatId,omitempty"`
	ExternalMessageID string   `json:"externalMessageId,omitempty"`
	UserGoal          string   `json:"userGoal"`
	Targets           []string `json:"targets"`
	Mode              string   `json:"mode"`
}

type companyAgentDispatchAck struct {
	AgentID        string                `json:"agentId"`
	AgentName      string                `json:"agentName,omitempty"`
	Status         companyDispatchStatus `json:"status"`
	AckText        string                `json:"ackText,omitempty"`
	ErrorText      string                `json:"errorText,omitempty"`
	Callable       bool                  `json:"callable"`
	Speakable      bool                  `json:"speakable"`
	Reason         string                `json:"reason,omitempty"`
	DispatchTaskID string                `json:"dispatchTaskId,omitempty"`
}

type companyAgentDispatchResult struct {
	TeamID           string                    `json:"teamId"`
	ManagerAgentID   string                    `json:"managerAgentId"`
	Mode             string                    `json:"mode"`
	RequestedTargets []string                  `json:"requestedTargets"`
	Acks             []companyAgentDispatchAck `json:"acks"`
	Summary          string                    `json:"summary,omitempty"`
}

type companyDispatchabilityView struct {
	AgentID      string `json:"agentId"`
	Dispatchable bool   `json:"dispatchable"`
	Speakable    bool   `json:"speakable"`
	Reason       string `json:"reason,omitempty"`
}

func createAgentDispatchTask(req companyAgentDispatchRequest, agentID string) string {
	return fmt.Sprintf("dispatch-%d-%s", time.Now().UnixMilli(), strings.TrimSpace(agentID))
}

func evaluateDispatchability(snapshot *companyTeamRuntimeSnapshot, targetAgentID string) companyDispatchabilityView {
	view := companyDispatchabilityView{AgentID: strings.TrimSpace(targetAgentID)}
	if snapshot == nil {
		view.Reason = "runtime snapshot missing"
		return view
	}
	for _, item := range snapshot.CallableAgents {
		if strings.TrimSpace(item.AgentID) != view.AgentID {
			continue
		}
		view.Dispatchable = item.CallableNow
		view.Speakable = item.CallableNow
		view.Reason = item.Reason
		return view
	}
	view.Reason = "agent not found in callable snapshot"
	return view
}

func runDispatchedAgentTask(cfg *config.Config, req companyAgentDispatchRequest, agentID string, agentName string) (companyAgentDispatchAck, error) {
	ack := companyAgentDispatchAck{
		AgentID:        strings.TrimSpace(agentID),
		AgentName:      strings.TrimSpace(agentName),
		Status:         companyDispatchRunning,
		Callable:       true,
		Speakable:      true,
		DispatchTaskID: createAgentDispatchTask(req, agentID),
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "reply_ok"
	}
	prompt := strings.Join([]string{
		"你收到来自 AI 公司主管的真实调度请求。",
		fmt.Sprintf("原始用户请求：%s", strings.TrimSpace(req.UserGoal)),
		fmt.Sprintf("当前模式：%s", mode),
		"你必须只代表你自己回复，不能代替其他智能体发言。",
		"请输出简洁确认结果。若 mode=reply_ok，则仅回复 OK；若 mode=checkin，则回复已收到并报到；不要添加额外解释。",
	}, "\n")
	runtimeSession := companyRuntimeSession(ack.DispatchTaskID, req.ExternalMessageID, agentID)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	reply, actualSessionID, err := runPanelChatMessage(ctx, cfg, runtimeSession, prompt)
	if strings.TrimSpace(actualSessionID) != "" {
		runtimeSession.OpenClawSessionID = strings.TrimSpace(actualSessionID)
	}
	if strings.TrimSpace(reply) == "" || err != nil {
		if recovered := companyRecoverPanelChatReply(cfg, runtimeSession, prompt); strings.TrimSpace(recovered) != "" {
			reply = recovered
			err = nil
		}
	}
	if err != nil {
		ack.Status = companyDispatchFailed
		ack.ErrorText = err.Error()
		return ack, err
	}
	ack.Status = companyDispatchSucceeded
	ack.AckText = sanitizePanelChatContent(reply)
	if strings.TrimSpace(ack.AckText) == "" {
		ack.Status = companyDispatchFailed
		ack.ErrorText = "empty ack"
		return ack, fmt.Errorf("empty ack")
	}
	return ack, nil
}

func collectDispatchAcks(result *companyAgentDispatchResult) string {
	if result == nil {
		return "未生成调度结果。"
	}
	lines := []string{fmt.Sprintf("已向 %d 个成员发起真实协作请求。", len(result.Acks))}
	for _, ack := range result.Acks {
		name := strings.TrimSpace(ack.AgentName)
		if name == "" {
			name = ack.AgentID
		}
		switch ack.Status {
		case companyDispatchSucceeded:
			lines = append(lines, fmt.Sprintf("- %s：%s", name, strings.TrimSpace(ack.AckText)))
		case companyDispatchSkipped:
			lines = append(lines, fmt.Sprintf("- %s：未调度（%s）", name, strings.TrimSpace(ack.Reason)))
		default:
			reason := strings.TrimSpace(ack.ErrorText)
			if reason == "" {
				reason = strings.TrimSpace(ack.Reason)
			}
			lines = append(lines, fmt.Sprintf("- %s：失败（%s）", name, reason))
		}
	}
	return strings.Join(lines, "\n")
}

func dispatchAgents(db *sql.DB, cfg *config.Config, req companyAgentDispatchRequest) (*companyAgentDispatchResult, error) {
	managerID := strings.TrimSpace(req.ManagerAgentID)
	if managerID == "" {
		managerID = strings.TrimSpace(req.RoutedAgentID)
	}
	resolution, err := resolveTeamByAgentScope(db, cfg, managerID, req.TeamID)
	if err != nil {
		return nil, err
	}
	req.TeamID = resolution.ResolvedTeamID
	scope := companyRuntimeScope{
		Channel:        strings.TrimSpace(req.Channel),
		AccountID:      strings.TrimSpace(req.AccountID),
		WorkspaceID:    strings.TrimSpace(req.WorkspaceID),
		SessionKind:    strings.TrimSpace(req.SessionKind),
		RoutedAgentID:  strings.TrimSpace(req.RoutedAgentID),
		ManagerAgentID: managerID,
	}
	snapshot, err := buildTeamRuntimeSnapshot(db, cfg, req.TeamID, scope)
	if err != nil {
		return nil, err
	}
	memberNames := map[string]string{}
	for _, member := range snapshot.TeamMembers {
		memberNames[member.AgentID] = member.AgentName
	}
	requested := make([]string, 0, len(req.Targets))
	seen := map[string]struct{}{}
	for _, target := range req.Targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		requested = append(requested, target)
	}
	result := &companyAgentDispatchResult{TeamID: req.TeamID, ManagerAgentID: managerID, Mode: strings.TrimSpace(req.Mode), RequestedTargets: requested, Acks: make([]companyAgentDispatchAck, 0, len(requested))}
	for _, target := range requested {
		dispatchability := evaluateDispatchability(snapshot, target)
		if !dispatchability.Dispatchable {
			result.Acks = append(result.Acks, companyAgentDispatchAck{AgentID: target, AgentName: memberNames[target], Status: companyDispatchSkipped, Callable: false, Speakable: false, Reason: dispatchability.Reason})
			continue
		}
		ack, err := runDispatchedAgentTask(cfg, req, target, memberNames[target])
		if err != nil {
			ack.Reason = dispatchability.Reason
			result.Acks = append(result.Acks, ack)
			continue
		}
		ack.Reason = dispatchability.Reason
		result.Acks = append(result.Acks, ack)
	}
	sort.Slice(result.Acks, func(i, j int) bool { return result.Acks[i].AgentID < result.Acks[j].AgentID })
	result.Summary = collectDispatchAcks(result)
	return result, nil
}

func handleManagerMultiAgentReplyRequest(db *sql.DB, cfg *config.Config, req companyAgentDispatchRequest) (*companyAgentDispatchResult, error) {
	return dispatchAgents(db, cfg, req)
}
