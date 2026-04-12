package handler

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/model"
)

type companyRuntimeScope struct {
	Channel         string `json:"channel,omitempty"`
	AccountID       string `json:"accountId,omitempty"`
	WorkspaceID     string `json:"workspaceId,omitempty"`
	SessionKind     string `json:"sessionKind,omitempty"`
	RoutedAgentID   string `json:"routedAgentId,omitempty"`
	ManagerAgentID  string `json:"managerAgentId,omitempty"`
	AllowDelegation bool   `json:"allowDelegation,omitempty"`
}

type companyTeamAgentView struct {
	TeamID    string   `json:"teamId"`
	AgentID   string   `json:"agentId"`
	AgentName string   `json:"agentName"`
	RoleType  string   `json:"roleType"`
	Enabled   bool     `json:"enabled"`
	Workspace string   `json:"workspace,omitempty"`
	Skills    []string `json:"skills,omitempty"`
}

type companyBindingMatchView struct {
	Channel   string `json:"channel"`
	AccountID string `json:"accountId,omitempty"`
	MatchType string `json:"matchType"`
}

type companyVisibleAgentView struct {
	TeamID            string                    `json:"teamId"`
	AgentID           string                    `json:"agentId"`
	AgentName         string                    `json:"agentName"`
	RoleType          string                    `json:"roleType"`
	Channel           string                    `json:"channel"`
	VisibleInChannel  bool                      `json:"visibleInChannel"`
	VisibleForAccount bool                      `json:"visibleForAccount"`
	MatchedBindings   []companyBindingMatchView `json:"matchedBindings,omitempty"`
	Reason            string                    `json:"reason,omitempty"`
}

type companyCallableAgentView struct {
	TeamID      string   `json:"teamId"`
	AgentID     string   `json:"agentId"`
	AgentName   string   `json:"agentName"`
	RoleType    string   `json:"roleType"`
	CallableNow bool     `json:"callableNow"`
	Reason      string   `json:"reason,omitempty"`
	Constraints []string `json:"constraints,omitempty"`
	SessionKind string   `json:"sessionKind,omitempty"`
	WorkspaceID string   `json:"workspaceId,omitempty"`
}

type companyTeamRuntimeSnapshot struct {
	TeamID         string                     `json:"teamId"`
	Channel        string                     `json:"channel,omitempty"`
	AccountID      string                     `json:"accountId,omitempty"`
	WorkspaceID    string                     `json:"workspaceId,omitempty"`
	SessionKind    string                     `json:"sessionKind,omitempty"`
	RoutedAgentID  string                     `json:"routedAgentId,omitempty"`
	ManagerAgentID string                     `json:"managerAgentId,omitempty"`
	TeamMembers    []companyTeamAgentView     `json:"teamMembers"`
	VisibleAgents  []companyVisibleAgentView  `json:"visibleAgents"`
	CallableAgents []companyCallableAgentView `json:"callableAgents"`
}

type companyTaskCollaborationSnapshot struct {
	TeamID           string                     `json:"teamId"`
	TaskID           string                     `json:"taskId"`
	CurrentAgentID   string                     `json:"currentAgentId"`
	ManagerAgentID   string                     `json:"managerAgentId,omitempty"`
	Channel          string                     `json:"channel,omitempty"`
	AccountID        string                     `json:"accountId,omitempty"`
	SessionKind      string                     `json:"sessionKind,omitempty"`
	TeamMembers      []companyTeamAgentView     `json:"teamMembers"`
	VisibleAgents    []companyVisibleAgentView  `json:"visibleAgents"`
	CallableAgents   []companyCallableAgentView `json:"callableAgents"`
	ActiveStepAgents []string                   `json:"activeStepAgents,omitempty"`
	UpstreamAgents   []string                   `json:"upstreamAgents,omitempty"`
	DownstreamAgents []string                   `json:"downstreamAgents,omitempty"`
}

type companyTeamResolution struct {
	ResolvedTeamID   string   `json:"resolvedTeamId"`
	CandidateTeamIDs []string `json:"candidateTeamIds,omitempty"`
}

func companyAgentCatalogIndex(cfg *config.Config) map[string]map[string]interface{} {
	items := companyAgentCatalog(cfg)
	indexed := make(map[string]map[string]interface{}, len(items))
	for _, item := range items {
		id := strings.TrimSpace(toString(item["id"]))
		if id == "" {
			continue
		}
		indexed[id] = item
	}
	return indexed
}

func companyBindingsSnapshot(cfg *config.Config) ([]map[string]interface{}, map[string]string) {
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig == nil {
		ocConfig = map[string]interface{}{}
	}
	agentsCfg := ensureAgentsConfig(ocConfig)
	return getBindingsFromConfig(ocConfig, agentsCfg), collectChannelDefaultAccounts(ocConfig)
}

func listTeamAgents(db *sql.DB, cfg *config.Config, teamID string) ([]companyTeamAgentView, error) {
	team, err := resolveCompanyTeam(db, cfg, teamID)
	if err != nil {
		return nil, err
	}
	catalogByID := companyAgentCatalogIndex(cfg)
	capabilities := companyCapabilityRegistry(cfg)
	out := make([]companyTeamAgentView, 0, len(team.Agents))
	for _, item := range team.Agents {
		agentID := strings.TrimSpace(item.AgentID)
		if agentID == "" {
			continue
		}
		view := companyTeamAgentView{
			TeamID:    team.ID,
			AgentID:   agentID,
			AgentName: strings.TrimSpace(item.AgentName),
			RoleType:  strings.TrimSpace(item.RoleType),
			Enabled:   item.Enabled,
		}
		if meta, ok := catalogByID[agentID]; ok {
			if view.AgentName == "" {
				view.AgentName = companyAgentDisplayName(meta)
			}
			if workspace := strings.TrimSpace(toString(meta["workspace"])); workspace != "" {
				view.Workspace = workspace
			}
		}
		if cap, ok := capabilities[agentID]; ok && len(cap.Skills) > 0 {
			view.Skills = append([]string{}, cap.Skills...)
		}
		if view.AgentName == "" {
			view.AgentName = agentID
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RoleType != out[j].RoleType {
			return out[i].RoleType < out[j].RoleType
		}
		if out[i].AgentName != out[j].AgentName {
			return out[i].AgentName < out[j].AgentName
		}
		return out[i].AgentID < out[j].AgentID
	})
	return out, nil
}

func companyBindingMatchesChannel(binding map[string]interface{}, channel string) (companyBindingMatchView, bool, string) {
	view := companyBindingMatchView{}
	if binding == nil {
		return view, false, "binding missing"
	}
	if enabled, ok := binding["enabled"].(bool); ok && !enabled {
		return view, false, "binding disabled"
	}
	if bindingType(binding) != "route" {
		return view, false, "binding type is not route"
	}
	matchRaw, _ := binding["match"].(map[string]interface{})
	match, err := normalizeBindingMatchForPreview(matchRaw)
	if err != nil {
		return view, false, err.Error()
	}
	if strings.TrimSpace(match.Channel) != channel {
		return view, false, "channel mismatch"
	}
	view = companyBindingMatchView{
		Channel:   match.Channel,
		AccountID: strings.TrimSpace(match.AccountID),
		MatchType: routePreviewMatchTier(match),
	}
	return view, true, ""
}

func companyBindingVisibleForAccount(match routePreviewMatch, accountID, defaultAccountID string) (bool, string) {
	if strings.TrimSpace(accountID) == "" {
		return true, "account scope not provided"
	}
	switch routePreviewMatchTier(match) {
	case "binding.channel":
		if strings.TrimSpace(defaultAccountID) != "" {
			return true, "channel fallback binding"
		}
		return true, "channel-level binding"
	case "binding.account.wildcard":
		return true, "account wildcard binding"
	case "binding.account":
		if strings.TrimSpace(match.AccountID) == strings.TrimSpace(accountID) {
			return true, "exact account binding"
		}
		return false, "account mismatch"
	default:
		return false, fmt.Sprintf("requires extra scope for %s", routePreviewMatchTier(match))
	}
}

func listVisibleAgents(db *sql.DB, cfg *config.Config, teamID, channel, accountID string) ([]companyVisibleAgentView, error) {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return nil, fmt.Errorf("channel required")
	}
	members, err := listTeamAgents(db, cfg, teamID)
	if err != nil {
		return nil, err
	}
	bindings, defaultAccounts := companyBindingsSnapshot(cfg)
	defaultAccountID := strings.TrimSpace(defaultAccounts[channel])
	memberByID := map[string]companyTeamAgentView{}
	for _, member := range members {
		memberByID[member.AgentID] = member
	}
	matchedByAgent := map[string][]companyBindingMatchView{}
	visibleForAccount := map[string]bool{}
	reasons := map[string]string{}
	for _, raw := range bindings {
		agentID := strings.TrimSpace(extractBindingAgentID(raw))
		if _, ok := memberByID[agentID]; !ok {
			continue
		}
		binding := deepCloneMap(raw)
		normalizeBindingTopLevel(binding)
		matchView, ok, why := companyBindingMatchesChannel(binding, channel)
		if !ok {
			if reasons[agentID] == "" && strings.Contains(why, "requires extra") {
				reasons[agentID] = why
			}
			continue
		}
		matchedByAgent[agentID] = append(matchedByAgent[agentID], matchView)
		matchRaw, _ := binding["match"].(map[string]interface{})
		match, err := normalizeBindingMatchForPreview(matchRaw)
		if err != nil {
			if reasons[agentID] == "" {
				reasons[agentID] = err.Error()
			}
			continue
		}
		visible, reason := companyBindingVisibleForAccount(match, accountID, defaultAccountID)
		if visible {
			visibleForAccount[agentID] = true
		} else if reasons[agentID] == "" {
			reasons[agentID] = reason
		}
	}
	out := make([]companyVisibleAgentView, 0, len(members))
	for _, member := range members {
		reason := reasons[member.AgentID]
		if len(matchedByAgent[member.AgentID]) == 0 && reason == "" {
			reason = "当前渠道下无显式 binding"
		}
		out = append(out, companyVisibleAgentView{
			TeamID:            member.TeamID,
			AgentID:           member.AgentID,
			AgentName:         member.AgentName,
			RoleType:          member.RoleType,
			Channel:           channel,
			VisibleInChannel:  len(matchedByAgent[member.AgentID]) > 0,
			VisibleForAccount: visibleForAccount[member.AgentID],
			MatchedBindings:   matchedByAgent[member.AgentID],
			Reason:            reason,
		})
	}
	return out, nil
}

func listCallableAgents(db *sql.DB, cfg *config.Config, teamID string, scope companyRuntimeScope) ([]companyCallableAgentView, error) {
	members, err := listTeamAgents(db, cfg, teamID)
	if err != nil {
		return nil, err
	}
	visibleByAgent := map[string]companyVisibleAgentView{}
	if strings.TrimSpace(scope.Channel) != "" {
		visible, err := listVisibleAgents(db, cfg, teamID, scope.Channel, scope.AccountID)
		if err == nil {
			for _, item := range visible {
				visibleByAgent[item.AgentID] = item
			}
		}
	}
	catalogByID := companyAgentCatalogIndex(cfg)
	out := make([]companyCallableAgentView, 0, len(members))
	for _, member := range members {
		view := companyCallableAgentView{
			TeamID:      member.TeamID,
			AgentID:     member.AgentID,
			AgentName:   member.AgentName,
			RoleType:    member.RoleType,
			SessionKind: strings.TrimSpace(scope.SessionKind),
			WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		}
		if !member.Enabled {
			view.CallableNow = false
			view.Reason = "团队中已禁用"
			out = append(out, view)
			continue
		}
		if _, ok := catalogByID[member.AgentID]; !ok {
			view.CallableNow = false
			view.Reason = "agent registry 中不存在该成员"
			out = append(out, view)
			continue
		}
		switch strings.TrimSpace(scope.SessionKind) {
		case "company_task":
			view.CallableNow = true
			view.Reason = "当前 company_task 内部调度作用域可调用"
		case "external_chat":
			if strings.TrimSpace(scope.RoutedAgentID) == member.AgentID {
				view.CallableNow = true
				view.Reason = "当前外部会话直接命中的响应 agent"
			} else if visible, ok := visibleByAgent[member.AgentID]; ok && visible.VisibleInChannel {
				if member.RoleType == "worker" {
					view.CallableNow = true
					if visible.VisibleForAccount {
						view.Reason = "同团队 worker，且在当前渠道/账号下可见，可用于外部会话协作调度"
					} else {
						view.Reason = "同团队 worker，在当前渠道下可见，可用于外部会话协作调度；但当前账号未命中其显式 binding，实际执行可能回落到渠道级规则"
						view.Constraints = []string{"channel_visible_only", "account_not_explicitly_matched"}
					}
				} else {
					view.CallableNow = false
					view.Reason = "当前 external_chat 只开放对同团队 worker 的协作调度，不将其他 manager 角色视为直接可调用"
					view.Constraints = []string{"manager_to_manager_not_enabled"}
				}
			} else {
				view.CallableNow = false
				view.Reason = "成员存在，但当前渠道下不可见，或没有可用于外部会话调度的显式可见性"
				view.Constraints = []string{"not_visible_in_channel"}
			}
		case "panel_chat":
			if strings.TrimSpace(scope.RoutedAgentID) == member.AgentID {
				view.CallableNow = true
				view.Reason = "当前 panel_chat 直接命中的会话 agent"
			} else {
				view.CallableNow = false
				view.Reason = "panel_chat 当前未显式暴露 agent-to-agent delegation 作用域"
				view.Constraints = []string{"delegation_not_established"}
			}
		default:
			view.CallableNow = false
			view.Reason = "当前会话缺少足够的 runtime scope 信息，暂做保守不可调用处理"
			view.Constraints = []string{"insufficient_runtime_scope"}
		}
		out = append(out, view)
	}
	return out, nil
}

func buildTeamRuntimeSnapshot(db *sql.DB, cfg *config.Config, teamID string, scope companyRuntimeScope) (*companyTeamRuntimeSnapshot, error) {
	members, err := listTeamAgents(db, cfg, teamID)
	if err != nil {
		return nil, err
	}
	visible := make([]companyVisibleAgentView, 0)
	if strings.TrimSpace(scope.Channel) != "" {
		visible, err = listVisibleAgents(db, cfg, teamID, scope.Channel, scope.AccountID)
		if err != nil {
			return nil, err
		}
	}
	callable, err := listCallableAgents(db, cfg, teamID, scope)
	if err != nil {
		return nil, err
	}
	return &companyTeamRuntimeSnapshot{
		TeamID:         teamID,
		Channel:        strings.TrimSpace(scope.Channel),
		AccountID:      strings.TrimSpace(scope.AccountID),
		WorkspaceID:    strings.TrimSpace(scope.WorkspaceID),
		SessionKind:    strings.TrimSpace(scope.SessionKind),
		RoutedAgentID:  strings.TrimSpace(scope.RoutedAgentID),
		ManagerAgentID: strings.TrimSpace(scope.ManagerAgentID),
		TeamMembers:    members,
		VisibleAgents:  visible,
		CallableAgents: callable,
	}, nil
}

func resolveTeamByAgentScope(db *sql.DB, cfg *config.Config, agentID string, hintedTeamID string) (*companyTeamResolution, error) {
	hintedTeamID = strings.TrimSpace(hintedTeamID)
	if hintedTeamID != "" {
		return &companyTeamResolution{ResolvedTeamID: hintedTeamID, CandidateTeamIDs: []string{hintedTeamID}}, nil
	}
	teams, err := model.ListCompanyTeams(db)
	if err != nil {
		return nil, err
	}
	candidates := make([]string, 0)
	managerCandidates := make([]string, 0)
	for _, team := range teams {
		agents, err := model.ListCompanyTeamAgents(db, team.ID)
		if err != nil {
			continue
		}
		for _, item := range agents {
			if strings.TrimSpace(item.AgentID) != strings.TrimSpace(agentID) {
				continue
			}
			candidates = append(candidates, team.ID)
			if strings.EqualFold(strings.TrimSpace(item.RoleType), "manager") {
				managerCandidates = append(managerCandidates, team.ID)
			}
			break
		}
	}
	resolved := ""
	if len(managerCandidates) > 0 {
		resolved = managerCandidates[0]
	} else if len(candidates) > 0 {
		resolved = candidates[0]
	}
	if resolved == "" {
		return nil, sql.ErrNoRows
	}
	return &companyTeamResolution{ResolvedTeamID: resolved, CandidateTeamIDs: candidates}, nil
}

func buildTaskCollaborationSnapshot(db *sql.DB, cfg *config.Config, task *model.CompanyTask, steps []model.CompanyTaskStep, currentStep *model.CompanyTaskStep, scope companyRuntimeScope) (*companyTaskCollaborationSnapshot, error) {
	if task == nil || currentStep == nil {
		return nil, nil
	}
	snapshot, err := buildTeamRuntimeSnapshot(db, cfg, task.TeamID, scope)
	if err != nil {
		return nil, err
	}
	active := map[string]struct{}{}
	for _, step := range steps {
		if agentID := strings.TrimSpace(step.WorkerAgentID); agentID != "" {
			active[agentID] = struct{}{}
		}
	}
	upstream := map[string]struct{}{}
	downstream := map[string]struct{}{}
	currentMeta := companyPlanStepFromModel(*currentStep)
	for _, dep := range currentMeta.DependsOn {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		for _, step := range steps {
			if strings.TrimSpace(step.StepKey) == dep {
				if agentID := strings.TrimSpace(step.WorkerAgentID); agentID != "" && agentID != currentStep.WorkerAgentID {
					upstream[agentID] = struct{}{}
				}
			}
		}
	}
	for _, step := range steps {
		meta := companyPlanStepFromModel(step)
		for _, dep := range meta.DependsOn {
			if strings.TrimSpace(dep) == strings.TrimSpace(currentStep.StepKey) {
				if agentID := strings.TrimSpace(step.WorkerAgentID); agentID != "" && agentID != currentStep.WorkerAgentID {
					downstream[agentID] = struct{}{}
				}
			}
		}
	}
	toSortedList := func(items map[string]struct{}) []string {
		out := make([]string, 0, len(items))
		for item := range items {
			out = append(out, item)
		}
		sort.Strings(out)
		return out
	}
	return &companyTaskCollaborationSnapshot{
		TeamID:           task.TeamID,
		TaskID:           task.ID,
		CurrentAgentID:   strings.TrimSpace(currentStep.WorkerAgentID),
		ManagerAgentID:   strings.TrimSpace(task.ManagerAgentID),
		Channel:          snapshot.Channel,
		AccountID:        snapshot.AccountID,
		SessionKind:      snapshot.SessionKind,
		TeamMembers:      snapshot.TeamMembers,
		VisibleAgents:    snapshot.VisibleAgents,
		CallableAgents:   snapshot.CallableAgents,
		ActiveStepAgents: toSortedList(active),
		UpstreamAgents:   toSortedList(upstream),
		DownstreamAgents: toSortedList(downstream),
	}, nil
}
