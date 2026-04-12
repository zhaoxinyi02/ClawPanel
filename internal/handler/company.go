package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/model"
)

const companySourcePanelChat = "panel_chat"
const companySourcePanelManual = "panel_manual"

const companyDeliveryWriteBackPanelSession = "write_back_panel_session"
const companyDeliveryNotifyOnly = "notify_only"

var companyPlanExtractJSONRe = regexp.MustCompile(`(?s)\{.*\}|\[.*\]`)

type companyPlanStep struct {
	ID                string   `json:"id,omitempty"`
	Title             string   `json:"title"`
	Instruction       string   `json:"instruction,omitempty"`
	WorkerAgentID     string   `json:"workerAgentId"`
	RequiredSkills    []string `json:"requiredSkills,omitempty"`
	Goal              string   `json:"goal,omitempty"`
	InputArtifacts    []string `json:"inputArtifacts,omitempty"`
	ExpectedArtifacts []string `json:"expectedArtifacts,omitempty"`
	DependsOn         []string `json:"dependsOn,omitempty"`
	Parallelizable    bool     `json:"parallelizable,omitempty"`
	Retryable         bool     `json:"retryable,omitempty"`
	MaxRetries        int      `json:"maxRetries,omitempty"`
	AssignmentReason  string   `json:"assignmentReason,omitempty"`
	Critical          bool     `json:"critical,omitempty"`
}

type companyTaskPlan struct {
	TaskType string            `json:"taskType,omitempty"`
	Summary  string            `json:"summary,omitempty"`
	Steps    []companyPlanStep `json:"steps"`
}

type companyAgentCapability struct {
	AgentID         string
	Name            string
	Role            string
	Skills          []string
	Languages       []string
	Tools           []string
	Channels        []string
	PriorityWeight  int
	Enabled         bool
	ChannelBindable bool
}

type companyCapabilityDescriptor struct {
	Skills         []string
	Languages      []string
	Tools          []string
	PriorityWeight int
}

type companyStepTemplate struct {
	Key               string
	Title             string
	Goal              string
	RequiredSkills    []string
	ExpectedArtifacts []string
	DependsOn         []string
	Parallelizable    bool
	Retryable         bool
	MaxRetries        int
	Critical          bool
}

var companyRoleProfiles = map[string]companyCapabilityDescriptor{
	"manager":      {Skills: []string{"planning", "delegation", "synthesis", "review"}, Languages: []string{"zh", "en"}, Tools: []string{"read", "write"}, PriorityWeight: 5},
	"planner":      {Skills: []string{"planning", "analysis", "decomposition"}, Languages: []string{"zh", "en"}, Tools: []string{"read", "write"}, PriorityWeight: 6},
	"coding":       {Skills: []string{"code_generation", "bug_fix", "implementation", "testing"}, Languages: []string{"c", "cpp", "go", "python", "ts", "js", "java", "rust", "zh", "en"}, Tools: []string{"write", "exec", "read"}, PriorityWeight: 9},
	"frontend_dev": {Skills: []string{"code_generation", "implementation", "frontend", "ui", "testing"}, Languages: []string{"ts", "js", "html", "css", "react", "vue", "zh", "en"}, Tools: []string{"write", "exec", "read"}, PriorityWeight: 8},
	"backend_dev":  {Skills: []string{"code_generation", "implementation", "backend", "api", "testing"}, Languages: []string{"go", "python", "java", "ts", "sql", "zh", "en"}, Tools: []string{"write", "exec", "read"}, PriorityWeight: 8},
	"writer":       {Skills: []string{"writing", "documentation", "delivery_notes", "summary"}, Languages: []string{"zh", "en"}, Tools: []string{"write", "read"}, PriorityWeight: 7},
	"translator":   {Skills: []string{"translation", "writing", "localization"}, Languages: []string{"zh", "en", "jp"}, Tools: []string{"write", "read"}, PriorityWeight: 7},
	"researcher":   {Skills: []string{"research", "analysis", "fact_gathering"}, Languages: []string{"zh", "en"}, Tools: []string{"read", "write"}, PriorityWeight: 7},
	"reviewer":     {Skills: []string{"review", "qa", "validation", "testing"}, Languages: []string{"zh", "en", "c", "cpp", "go", "python", "ts", "js"}, Tools: []string{"read", "exec", "write"}, PriorityWeight: 8},
	"tester":       {Skills: []string{"testing", "validation", "qa"}, Languages: []string{"zh", "en", "c", "cpp", "go", "python", "ts", "js"}, Tools: []string{"read", "exec", "write"}, PriorityWeight: 8},
	"generalist":   {Skills: []string{"general", "research", "analysis"}, Languages: []string{"zh", "en"}, Tools: []string{"read", "write"}, PriorityWeight: 4},
}

var companyRoleAliases = map[string][]string{
	"manager":      {"company_manager", "manager", "主管", "经理"},
	"planner":      {"planner", "规划", "调度"},
	"coding":       {"coding", "engineer", "dev", "开发", "工程师", "程序"},
	"frontend_dev": {"frontend", "前端", "ui"},
	"backend_dev":  {"backend", "后端", "api", "服务端"},
	"writer":       {"writer", "文档", "策划", "写作", "说明"},
	"translator":   {"translator", "翻译", "本地化"},
	"researcher":   {"researcher", "research", "调研", "研究"},
	"reviewer":     {"reviewer", "审校", "review", "评审"},
	"tester":       {"tester", "test", "测试", "qa"},
}

var companyTaskTypeTemplates = map[string][]companyStepTemplate{
	"code_generation": {
		{Key: "implement", Title: "实现核心交付", Goal: "根据需求完成核心代码或可执行方案，输出可直接交付的代码/实现结果。", RequiredSkills: []string{"code_generation", "implementation"}, ExpectedArtifacts: []string{"code"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
		{Key: "document", Title: "编写交付说明", Goal: "基于核心交付结果编写说明、使用方法或交付文档，不要重复实现代码。", RequiredSkills: []string{"writing", "documentation"}, ExpectedArtifacts: []string{"documentation"}, DependsOn: []string{"implement"}, Parallelizable: true, Retryable: true, MaxRetries: 1, Critical: false},
		{Key: "review", Title: "功能与题意审查", Goal: "基于核心交付结果进行验证，检查是否符合题意、是否可运行、是否有明显缺陷，并给出明确结论。", RequiredSkills: []string{"validation", "testing"}, ExpectedArtifacts: []string{"review_report"}, DependsOn: []string{"implement"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
	},
	"bug_fix": {
		{Key: "diagnose", Title: "定位并修复问题", Goal: "定位 bug 根因并完成修复，输出修复说明和关键变更。", RequiredSkills: []string{"bug_fix", "implementation"}, ExpectedArtifacts: []string{"fix"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
		{Key: "validate", Title: "回归验证", Goal: "基于修复结果验证问题已解决，检查是否引入新问题。", RequiredSkills: []string{"validation", "testing"}, ExpectedArtifacts: []string{"review_report"}, DependsOn: []string{"diagnose"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
		{Key: "document", Title: "输出修复说明", Goal: "基于修复和验证结果编写面向用户的修复说明。", RequiredSkills: []string{"writing", "documentation"}, ExpectedArtifacts: []string{"documentation"}, DependsOn: []string{"diagnose", "validate"}, Parallelizable: true, Retryable: true, MaxRetries: 1, Critical: false},
	},
	"writing": {
		{Key: "draft", Title: "撰写初稿", Goal: "根据需求撰写结构清晰的正文初稿。", RequiredSkills: []string{"writing"}, ExpectedArtifacts: []string{"draft"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
		{Key: "review", Title: "审校文稿", Goal: "基于初稿检查逻辑、结构、一致性与表达准确性。", RequiredSkills: []string{"review", "validation"}, ExpectedArtifacts: []string{"review_report"}, DependsOn: []string{"draft"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
	},
	"research": {
		{Key: "research", Title: "资料整理与分析", Goal: "围绕任务目标提炼事实、结论和建议，输出结构化研究结论。", RequiredSkills: []string{"research", "analysis"}, ExpectedArtifacts: []string{"research_notes"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
		{Key: "summary", Title: "整理研究摘要", Goal: "基于研究结论整理对外可读的摘要与建议。", RequiredSkills: []string{"writing", "summary"}, ExpectedArtifacts: []string{"documentation"}, DependsOn: []string{"research"}, Parallelizable: true, Retryable: true, MaxRetries: 1, Critical: false},
		{Key: "review", Title: "审校研究结果", Goal: "检查研究结论是否自洽、是否遗漏关键点。", RequiredSkills: []string{"validation", "review"}, ExpectedArtifacts: []string{"review_report"}, DependsOn: []string{"research"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
	},
	"mixed": {
		{Key: "analyze", Title: "提炼执行规格", Goal: "先提炼任务边界、关键约束与交付规格。", RequiredSkills: []string{"analysis", "planning"}, ExpectedArtifacts: []string{"spec"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
		{Key: "implement", Title: "完成核心交付", Goal: "基于规格完成主要交付结果。", RequiredSkills: []string{"implementation", "code_generation"}, ExpectedArtifacts: []string{"result"}, DependsOn: []string{"analyze"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
		{Key: "review", Title: "审查交付结果", Goal: "检查主要交付结果是否符合规格。", RequiredSkills: []string{"validation", "review"}, ExpectedArtifacts: []string{"review_report"}, DependsOn: []string{"implement"}, Parallelizable: false, Retryable: true, MaxRetries: 1, Critical: true},
	},
}

type companyCreateTaskRequest struct {
	TeamID              string   `json:"teamId"`
	Title               string   `json:"title"`
	Goal                string   `json:"goal"`
	ManagerAgentID      string   `json:"managerAgentId"`
	WorkerAgentIDs      []string `json:"workerAgentIds"`
	SummaryAgentID      string   `json:"summaryAgentId"`
	SourceType          string   `json:"sourceType"`
	SourceChannelType   string   `json:"sourceChannelType"`
	SourceChannelID     string   `json:"sourceChannelId"`
	SourceThreadID      string   `json:"sourceThreadId"`
	SourceMessageID     string   `json:"sourceMessageId"`
	SourceRefID         string   `json:"sourceRefId"`
	DeliveryType        string   `json:"deliveryType"`
	DeliveryChannelType string   `json:"deliveryChannelType"`
	DeliveryChannelID   string   `json:"deliveryChannelId"`
	DeliveryThreadID    string   `json:"deliveryThreadId"`
	DeliveryTargetID    string   `json:"deliveryTargetId"`
	PanelSessionID      string   `json:"panelSessionId"`
}

type companySaveTeamRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	ManagerAgentID string   `json:"managerAgentId"`
	WorkerAgentIDs []string `json:"workerAgentIds"`
	Status         string   `json:"status"`
}

type companyOverview struct {
	TeamCount      int                 `json:"teamCount"`
	TaskCount      int                 `json:"taskCount"`
	RunningCount   int                 `json:"runningCount"`
	CompletedCount int                 `json:"completedCount"`
	RecentTasks    []model.CompanyTask `json:"recentTasks"`
}

type companyChannelPlaceholder struct {
	ChannelType   string `json:"channelType"`
	Label         string `json:"label"`
	SourceReady   bool   `json:"sourceReady"`
	DeliveryReady bool   `json:"deliveryReady"`
	Status        string `json:"status"`
	Note          string `json:"note,omitempty"`
}

func companyAgentNameMap(cfg *config.Config) map[string]string {
	return loadPanelChatAgentNameMap(cfg)
}

func companyKnownChannels() []companyChannelPlaceholder {
	return []companyChannelPlaceholder{
		{ChannelType: "panel_chat", Label: "面板聊天", SourceReady: true, DeliveryReady: true, Status: "active", Note: "当前已接入为任务入口与回写目标"},
		{ChannelType: "panel_manual", Label: "面板手工创建", SourceReady: true, DeliveryReady: false, Status: "active", Note: "当前任务中心可直接创建"},
		{ChannelType: "qq", Label: "QQ (NapCat)", SourceReady: false, DeliveryReady: false, Status: "reserved", Note: "预留 source/delivery 字段与接口"},
		{ChannelType: "qqbot", Label: "QQ 官方机器人", SourceReady: false, DeliveryReady: false, Status: "reserved", Note: "预留 source/delivery 字段与接口"},
		{ChannelType: "wechat", Label: "微信", SourceReady: false, DeliveryReady: false, Status: "reserved", Note: "预留 source/delivery 字段与接口"},
		{ChannelType: "feishu", Label: "飞书 / Lark", SourceReady: false, DeliveryReady: false, Status: "reserved", Note: "预留 source/delivery 字段与接口"},
		{ChannelType: "wecom", Label: "企业微信（机器人）", SourceReady: false, DeliveryReady: false, Status: "reserved", Note: "预留 source/delivery 字段与接口"},
		{ChannelType: "wecom-app", Label: "企业微信（自建应用）", SourceReady: false, DeliveryReady: false, Status: "reserved", Note: "预留 source/delivery 字段与接口"},
		{ChannelType: "api", Label: "API", SourceReady: false, DeliveryReady: false, Status: "reserved", Note: "预留统一 API source / response delivery"},
		{ChannelType: "webhook", Label: "Webhook", SourceReady: false, DeliveryReady: false, Status: "reserved", Note: "预留 webhook source / notify delivery"},
	}
}

func companyAgentCatalog(cfg *config.Config) []map[string]interface{} {
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig == nil {
		ocConfig = map[string]interface{}{}
	}
	return materializeAgentList(cfg, ocConfig)
}

func companyAgentDisplayName(item map[string]interface{}) string {
	if item == nil {
		return ""
	}
	if name := strings.TrimSpace(toString(item["name"])); name != "" {
		return name
	}
	if identity, ok := item["identity"].(map[string]interface{}); ok && identity != nil {
		if name := strings.TrimSpace(toString(identity["name"])); name != "" {
			return name
		}
	}
	return strings.TrimSpace(toString(item["id"]))
}

func companyDefaultManagerAgentID(cfg *config.Config) string {
	list := companyAgentCatalog(cfg)
	fallback := strings.TrimSpace(loadDefaultAgentID(cfg))
	bestID := strings.TrimSpace(fallback)
	bestScore := -1
	for _, item := range list {
		id := strings.TrimSpace(toString(item["id"]))
		if id == "" {
			continue
		}
		name := strings.ToLower(companyAgentDisplayName(item))
		lowerID := strings.ToLower(id)
		score := 0
		switch {
		case lowerID == "company_manager":
			score = 100
		case strings.HasPrefix(lowerID, "company_manager"):
			score = 95
		case strings.Contains(name, "ai company manager"):
			score = 90
		case strings.Contains(name, "company manager"):
			score = 85
		case strings.Contains(name, "ai公司manager") || strings.Contains(name, "ai公司主管") || strings.Contains(name, "公司主管") || strings.Contains(name, "公司经理"):
			score = 80
		case fallback != "" && id == fallback:
			score = 40
		case asBool(item["default"]):
			score = 20
		default:
			score = 10
		}
		if score > bestScore {
			bestScore = score
			bestID = id
		}
	}
	if strings.TrimSpace(bestID) != "" {
		return bestID
	}
	if fallback != "" {
		return fallback
	}
	return "main"
}

func companyNormalizeWorkerAgentIDs(managerID string, raw []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" || item == managerID {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		items = append(items, item)
	}
	return items
}

func companyCatalogWorkerAgentIDs(cfg *config.Config, managerID string) []string {
	workers := make([]string, 0)
	for _, item := range companyAgentCatalog(cfg) {
		if id := strings.TrimSpace(toString(item["id"])); id != "" && id != managerID {
			workers = append(workers, id)
		}
	}
	return companyNormalizeWorkerAgentIDs(managerID, workers)
}

func companyNormalizedTeamAgents(cfg *config.Config, managerID string, existing []model.CompanyTeamAgent, requestedWorkers []string) []model.CompanyTeamAgent {
	managerID = strings.TrimSpace(managerID)
	if managerID == "" {
		managerID = companyDefaultManagerAgentID(cfg)
	}
	byID := map[string]model.CompanyTeamAgent{}
	for _, item := range existing {
		id := strings.TrimSpace(item.AgentID)
		if id == "" {
			continue
		}
		item.AgentID = id
		byID[id] = item
	}
	workers := companyNormalizeWorkerAgentIDs(managerID, requestedWorkers)
	if len(workers) == 0 {
		for _, item := range existing {
			id := strings.TrimSpace(item.AgentID)
			if id == "" || id == managerID {
				continue
			}
			workers = append(workers, id)
		}
	}
	workers = companyNormalizeWorkerAgentIDs(managerID, workers)
	nameMap := companyAgentNameMap(cfg)
	manager := byID[managerID]
	manager.AgentID = managerID
	manager.RoleType = "manager"
	manager.Enabled = true
	if strings.TrimSpace(manager.DutyLabel) == "" {
		manager.DutyLabel = "调度 / 汇总"
	}
	if strings.TrimSpace(manager.AgentName) == "" {
		manager.AgentName = nameMap[managerID]
	}
	items := []model.CompanyTeamAgent{manager}
	for i, workerID := range workers {
		worker := byID[workerID]
		worker.AgentID = workerID
		worker.RoleType = "worker"
		worker.SortOrder = i + 1
		if worker.ID == 0 && !worker.Enabled {
			worker.Enabled = true
		}
		if strings.TrimSpace(worker.DutyLabel) == "" {
			worker.DutyLabel = "执行"
		}
		if strings.TrimSpace(worker.AgentName) == "" {
			worker.AgentName = nameMap[workerID]
		}
		items = append(items, worker)
	}
	return items
}

func companyNormalizeTeam(cfg *config.Config, team *model.CompanyTeam) *model.CompanyTeam {
	if team == nil {
		return nil
	}
	managerID := strings.TrimSpace(team.ManagerAgentID)
	if managerID == "" {
		for _, item := range team.Agents {
			if strings.EqualFold(strings.TrimSpace(item.RoleType), "manager") && strings.TrimSpace(item.AgentID) != "" {
				managerID = strings.TrimSpace(item.AgentID)
				break
			}
		}
	}
	if managerID == "" {
		managerID = companyDefaultManagerAgentID(cfg)
	}
	team.ManagerAgentID = managerID
	if strings.TrimSpace(team.DefaultSummaryAgentID) == "" {
		team.DefaultSummaryAgentID = managerID
	}
	if strings.TrimSpace(team.Status) == "" {
		team.Status = "active"
	}
	team.Agents = companyNormalizedTeamAgents(cfg, managerID, team.Agents, nil)
	return team
}

func normalizeCompanySourceType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == companySourcePanelChat {
		return companySourcePanelChat
	}
	return companySourcePanelManual
}

func normalizeCompanyDeliveryType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == companyDeliveryWriteBackPanelSession {
		return companyDeliveryWriteBackPanelSession
	}
	return companyDeliveryNotifyOnly
}

func loadCompanyTaskBundle(db *sql.DB, id string) (*model.CompanyTask, error) {
	task, err := model.GetCompanyTask(db, id)
	if err != nil {
		return nil, err
	}
	steps, _ := model.ListCompanyTaskSteps(db, id)
	events, _ := model.ListCompanyEvents(db, id)
	task.Steps = steps
	task.Events = events
	return task, nil
}

func ensureDefaultCompanyTeam(db *sql.DB, cfg *config.Config) (*model.CompanyTeam, error) {
	teams, err := model.ListCompanyTeams(db)
	if err != nil {
		return nil, err
	}
	if len(teams) > 0 {
		team := teams[0]
		agents, _ := model.ListCompanyTeamAgents(db, team.ID)
		team.Agents = agents
		return companyNormalizeTeam(cfg, &team), nil
	}
	defaultID := companyDefaultManagerAgentID(cfg)
	team := &model.CompanyTeam{
		ID:                    "default",
		Name:                  "默认团队",
		Description:           "基于当前智能体配置自动生成的执行团队",
		ManagerAgentID:        defaultID,
		DefaultSummaryAgentID: defaultID,
		Status:                "active",
	}
	if err := model.UpsertCompanyTeam(db, team); err != nil {
		return nil, err
	}
	teamAgents := companyNormalizedTeamAgents(cfg, defaultID, nil, companyCatalogWorkerAgentIDs(cfg, defaultID))
	if err := model.ReplaceCompanyTeamAgents(db, team.ID, teamAgents); err != nil {
		return nil, err
	}
	team.Agents = teamAgents
	return companyNormalizeTeam(cfg, team), nil
}

func resolveCompanyTeam(db *sql.DB, cfg *config.Config, teamID string) (*model.CompanyTeam, error) {
	if strings.TrimSpace(teamID) == "" || strings.TrimSpace(teamID) == "default" {
		return ensureDefaultCompanyTeam(db, cfg)
	}
	team, err := model.GetCompanyTeam(db, teamID)
	if err != nil {
		return nil, err
	}
	agents, err := model.ListCompanyTeamAgents(db, team.ID)
	if err != nil {
		return nil, err
	}
	team.Agents = agents
	return companyNormalizeTeam(cfg, team), nil
}

func companyParticipantsFromTeam(cfg *config.Config, team *model.CompanyTeam, req companyCreateTaskRequest) (string, []string, string) {
	manager := strings.TrimSpace(req.ManagerAgentID)
	if manager == "" {
		manager = strings.TrimSpace(team.ManagerAgentID)
	}
	if manager == "" {
		manager = companyDefaultManagerAgentID(cfg)
	}
	summary := strings.TrimSpace(req.SummaryAgentID)
	if summary == "" {
		summary = strings.TrimSpace(team.DefaultSummaryAgentID)
		if summary == "" {
			summary = manager
		}
	}
	workers := make([]string, 0)
	seen := map[string]struct{}{}
	for _, item := range req.WorkerAgentIDs {
		item = strings.TrimSpace(item)
		if item == "" || item == manager {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		workers = append(workers, item)
	}
	if len(workers) == 0 {
		for _, item := range team.Agents {
			if !item.Enabled || item.AgentID == manager {
				continue
			}
			if _, ok := seen[item.AgentID]; ok {
				continue
			}
			seen[item.AgentID] = struct{}{}
			workers = append(workers, item.AgentID)
		}
	}
	return manager, workers, summary
}

func companyInferTaskType(title, goal string) string {
	text := strings.ToLower(strings.TrimSpace(title + "\n" + goal))
	switch {
	case strings.Contains(text, "bug") || strings.Contains(text, "修复") || strings.Contains(text, "报错") || strings.Contains(text, "错误"):
		return "bug_fix"
	case strings.Contains(text, "文案") || strings.Contains(text, "报告") || strings.Contains(text, "说明") || strings.Contains(text, "润色"):
		return "writing"
	case strings.Contains(text, "调研") || strings.Contains(text, "研究") || strings.Contains(text, "research"):
		return "research"
	case strings.Contains(text, "代码") || strings.Contains(text, "c语言") || strings.Contains(text, "程序") || strings.Contains(text, "编译") || strings.Contains(text, "实现"):
		return "code_generation"
	default:
		return "mixed"
	}
}

func companyNeedsWriter(taskType, goal string) bool {
	text := strings.ToLower(goal)
	if taskType == "writing" {
		return true
	}
	return strings.Contains(text, "报告") || strings.Contains(text, "说明") || strings.Contains(text, "文档") || strings.Contains(text, "readme") || strings.Contains(text, "总结")
}

func companyInferRole(agentID, name string) string {
	text := strings.ToLower(agentID + " " + name)
	for role, aliases := range companyRoleAliases {
		for _, alias := range aliases {
			if alias != "" && strings.Contains(text, strings.ToLower(alias)) {
				return role
			}
		}
	}
	return "generalist"
}

func companyCapabilityProfile(role string) (skills, languages, tools []string, priority int) {
	if profile, ok := companyRoleProfiles[role]; ok {
		return profile.Skills, profile.Languages, profile.Tools, profile.PriorityWeight
	}
	profile := companyRoleProfiles["generalist"]
	return profile.Skills, profile.Languages, profile.Tools, profile.PriorityWeight
}

func companyCapabilityRegistry(cfg *config.Config) map[string]companyAgentCapability {
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig == nil {
		ocConfig = map[string]interface{}{}
	}
	list := companyAgentCatalog(cfg)
	agentsCfg := ensureAgentsConfig(ocConfig)
	bindings := getBindingsFromConfig(ocConfig, agentsCfg)
	channelsByAgent := map[string]map[string]struct{}{}
	for _, item := range bindings {
		agentID := strings.TrimSpace(toString(item["agentId"]))
		if agentID == "" {
			continue
		}
		match, _ := item["match"].(map[string]interface{})
		channel := strings.TrimSpace(toString(match["channel"]))
		if channel == "" {
			continue
		}
		if channelsByAgent[agentID] == nil {
			channelsByAgent[agentID] = map[string]struct{}{}
		}
		channelsByAgent[agentID][channel] = struct{}{}
	}
	registry := map[string]companyAgentCapability{}
	for _, item := range list {
		agentID := strings.TrimSpace(toString(item["id"]))
		if agentID == "" {
			continue
		}
		name := companyAgentDisplayName(item)
		role := companyInferRole(agentID, name)
		skills, languages, tools, priority := companyCapabilityProfile(role)
		channels := make([]string, 0, len(channelsByAgent[agentID]))
		for channel := range channelsByAgent[agentID] {
			channels = append(channels, channel)
		}
		registry[agentID] = companyAgentCapability{
			AgentID:         agentID,
			Name:            name,
			Role:            role,
			Skills:          skills,
			Languages:       languages,
			Tools:           tools,
			Channels:        channels,
			PriorityWeight:  priority,
			Enabled:         true,
			ChannelBindable: len(channels) > 0,
		}
	}
	return registry
}

func companyStepLikelyLanguage(goal string) string {
	text := strings.ToLower(goal)
	switch {
	case strings.Contains(text, "c++"):
		return "cpp"
	case strings.Contains(text, "c语言"), strings.Contains(text, " c "):
		return "c"
	case strings.Contains(text, "python"):
		return "python"
	case strings.Contains(text, "go") || strings.Contains(text, "golang"):
		return "go"
	case strings.Contains(text, "typescript") || strings.Contains(text, "ts"):
		return "ts"
	default:
		return ""
	}
}

func companyContains(list []string, value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	for _, item := range list {
		if strings.TrimSpace(strings.ToLower(item)) == value {
			return true
		}
	}
	return false
}

func companyInt64(v interface{}) int64 {
	switch value := v.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	case string:
		parsed, _ := json.Number(strings.TrimSpace(value)).Int64()
		return parsed
	default:
		return 0
	}
}

func companySelectWorker(step *companyPlanStep, registry map[string]companyAgentCapability, candidates []string, exclude ...string) (string, string) {
	excluded := map[string]struct{}{}
	for _, item := range exclude {
		item = strings.TrimSpace(item)
		if item != "" {
			excluded[item] = struct{}{}
		}
	}
	bestID := ""
	bestReason := ""
	bestScore := -1
	language := companyStepLikelyLanguage(step.Goal)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, blocked := excluded[candidate]; blocked {
			continue
		}
		cap, ok := registry[candidate]
		if !ok {
			role := companyInferRole(candidate, candidate)
			skills, languages, tools, priority := companyCapabilityProfile(role)
			cap = companyAgentCapability{AgentID: candidate, Name: candidate, Role: role, Skills: skills, Languages: languages, Tools: tools, PriorityWeight: priority, Enabled: true}
		}
		if !cap.Enabled {
			continue
		}
		score := 0
		reasons := make([]string, 0)
		for _, skill := range step.RequiredSkills {
			if companyContains(cap.Skills, skill) {
				score += 30
				reasons = append(reasons, "skill:"+skill)
			}
		}
		if language != "" && companyContains(cap.Languages, language) {
			score += 20
			reasons = append(reasons, "lang:"+language)
		}
		if companyContains(step.ExpectedArtifacts, "code") && companyContains(cap.Tools, "exec") {
			score += 10
			reasons = append(reasons, "tool:exec")
		}
		if companyContains(step.ExpectedArtifacts, "documentation") && companyContains(cap.Tools, "write") {
			score += 6
			reasons = append(reasons, "tool:write")
		}
		score += cap.PriorityWeight
		reasons = append(reasons, fmt.Sprintf("priority:%d", cap.PriorityWeight))
		if score > bestScore {
			bestScore = score
			bestID = candidate
			bestReason = strings.Join(reasons, ",")
		}
	}
	return bestID, bestReason
}

func companyBuildSOPPlan(task *model.CompanyTask, workers []string, registry map[string]companyAgentCapability) companyTaskPlan {
	taskType := companyInferTaskType(task.Title, task.Goal)
	plan := companyTaskPlan{TaskType: taskType, Summary: strings.TrimSpace(task.Title + "：" + task.Goal)}
	templates := companyTaskTypeTemplates[taskType]
	if len(templates) == 0 {
		templates = companyTaskTypeTemplates["mixed"]
	}
	buildStep := func(id, title, goal string, required []string, expected []string, dependsOn []string, parallelizable bool, critical bool) companyPlanStep {
		step := companyPlanStep{
			ID:                id,
			Title:             title,
			Goal:              goal,
			RequiredSkills:    required,
			ExpectedArtifacts: expected,
			InputArtifacts:    dependsOn,
			DependsOn:         dependsOn,
			Parallelizable:    parallelizable,
			Retryable:         true,
			MaxRetries:        1,
			Critical:          critical,
		}
		step.WorkerAgentID, step.AssignmentReason = companySelectWorker(&step, registry, workers)
		return step
	}
	for _, tpl := range templates {
		if tpl.Key == "document" && !companyNeedsWriter(taskType, task.Goal) {
			continue
		}
		plan.Steps = append(plan.Steps, buildStep(tpl.Key, tpl.Title, tpl.Goal, tpl.RequiredSkills, tpl.ExpectedArtifacts, tpl.DependsOn, tpl.Parallelizable, tpl.Critical))
	}
	if len(plan.Steps) == 0 {
		plan.Steps = append(plan.Steps, buildStep("general", "执行主任务", task.Goal, []string{"general"}, []string{"result"}, nil, false, true))
	}
	return plan
}

func parseCompanyPlan(raw string) companyTaskPlan {
	match := companyPlanExtractJSONRe.FindString(strings.TrimSpace(raw))
	if match == "" {
		return companyTaskPlan{}
	}
	var wrapped companyTaskPlan
	if strings.HasPrefix(strings.TrimSpace(match), "{") {
		if err := json.Unmarshal([]byte(match), &wrapped); err == nil && len(wrapped.Steps) > 0 {
			return wrapped
		}
		var legacy struct {
			Steps []companyPlanStep `json:"steps"`
		}
		if err := json.Unmarshal([]byte(match), &legacy); err == nil && len(legacy.Steps) > 0 {
			return companyTaskPlan{Steps: legacy.Steps}
		}
	}
	var plain []companyPlanStep
	if err := json.Unmarshal([]byte(match), &plain); err == nil && len(plain) > 0 {
		return companyTaskPlan{Steps: plain}
	}
	return companyTaskPlan{}
}

func companyInferStepSkills(title, goal string) []string {
	text := strings.ToLower(strings.TrimSpace(title + "\n" + goal))
	skills := make([]string, 0, 3)
	appendSkill := func(items ...string) {
		for _, item := range items {
			if !companyContains(skills, item) {
				skills = append(skills, item)
			}
		}
	}
	switch {
	case strings.Contains(text, "审") || strings.Contains(text, "校验") || strings.Contains(text, "review") || strings.Contains(text, "验收"):
		appendSkill("review", "validation")
	case strings.Contains(text, "文档") || strings.Contains(text, "说明") || strings.Contains(text, "报告") || strings.Contains(text, "总结"):
		appendSkill("writing", "documentation")
	case strings.Contains(text, "调研") || strings.Contains(text, "研究"):
		appendSkill("research", "analysis")
	default:
		appendSkill("code_generation", "implementation")
	}
	if lang := companyStepLikelyLanguage(goal); lang != "" {
		appendSkill(lang)
	}
	return skills
}

func companyInferExpectedArtifacts(skills []string) []string {
	if companyContains(skills, "review") || companyContains(skills, "validation") {
		return []string{"review_report"}
	}
	if companyContains(skills, "writing") || companyContains(skills, "documentation") {
		return []string{"documentation"}
	}
	if companyContains(skills, "research") {
		return []string{"research_notes"}
	}
	return []string{"code"}
}

func companyNormalizePlan(task *model.CompanyTask, plan companyTaskPlan, workers []string, registry map[string]companyAgentCapability) companyTaskPlan {
	if plan.TaskType == "" {
		plan.TaskType = companyInferTaskType(task.Title, task.Goal)
	}
	if strings.TrimSpace(plan.Summary) == "" {
		plan.Summary = strings.TrimSpace(task.Title + "：" + task.Goal)
	}
	if len(plan.Steps) == 0 {
		return companyBuildSOPPlan(task, workers, registry)
	}
	allowed := map[string]struct{}{}
	for _, worker := range workers {
		allowed[strings.TrimSpace(worker)] = struct{}{}
	}
	seen := map[string]struct{}{}
	for i := range plan.Steps {
		if strings.TrimSpace(plan.Steps[i].ID) == "" {
			plan.Steps[i].ID = fmt.Sprintf("step_%d", i+1)
		}
		plan.Steps[i].ID = strings.TrimSpace(plan.Steps[i].ID)
		if _, ok := seen[plan.Steps[i].ID]; ok {
			plan.Steps[i].ID = fmt.Sprintf("%s_%d", plan.Steps[i].ID, i+1)
		}
		seen[plan.Steps[i].ID] = struct{}{}
		if strings.TrimSpace(plan.Steps[i].Title) == "" {
			plan.Steps[i].Title = fmt.Sprintf("步骤 %d", i+1)
		}
		if strings.TrimSpace(plan.Steps[i].Goal) == "" {
			plan.Steps[i].Goal = strings.TrimSpace(plan.Steps[i].Instruction)
		}
		if strings.TrimSpace(plan.Steps[i].Goal) == "" {
			plan.Steps[i].Goal = plan.Steps[i].Title
		}
		if len(plan.Steps[i].RequiredSkills) == 0 {
			plan.Steps[i].RequiredSkills = companyInferStepSkills(plan.Steps[i].Title, plan.Steps[i].Goal)
		}
		if len(plan.Steps[i].ExpectedArtifacts) == 0 {
			plan.Steps[i].ExpectedArtifacts = companyInferExpectedArtifacts(plan.Steps[i].RequiredSkills)
		}
		if plan.Steps[i].MaxRetries <= 0 {
			plan.Steps[i].MaxRetries = 1
		}
		plan.Steps[i].Retryable = true
		if plan.Steps[i].Critical == false {
			plan.Steps[i].Critical = companyContains(plan.Steps[i].ExpectedArtifacts, "code") || companyContains(plan.Steps[i].ExpectedArtifacts, "review_report")
		}
		if _, ok := allowed[strings.TrimSpace(plan.Steps[i].WorkerAgentID)]; !ok {
			plan.Steps[i].WorkerAgentID = ""
		}
		selected, reason := companySelectWorker(&plan.Steps[i], registry, workers)
		if selected != "" {
			plan.Steps[i].WorkerAgentID = selected
			plan.Steps[i].AssignmentReason = reason
		}
	}
	for i := range plan.Steps {
		deps := make([]string, 0, len(plan.Steps[i].DependsOn))
		for _, dep := range plan.Steps[i].DependsOn {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			if _, ok := seen[dep]; ok {
				deps = append(deps, dep)
			}
		}
		plan.Steps[i].DependsOn = deps
		if len(plan.Steps[i].DependsOn) > 0 {
			plan.Steps[i].InputArtifacts = append([]string{}, plan.Steps[i].DependsOn...)
			plan.Steps[i].Parallelizable = false
		}
	}
	if plan.TaskType == "code_generation" || plan.TaskType == "bug_fix" || plan.TaskType == "mixed" {
		implID := ""
		for i := range plan.Steps {
			if companyContains(plan.Steps[i].ExpectedArtifacts, "code") {
				implID = plan.Steps[i].ID
				break
			}
		}
		for i := range plan.Steps {
			if companyContains(plan.Steps[i].RequiredSkills, "review") && implID != "" && len(plan.Steps[i].DependsOn) == 0 {
				plan.Steps[i].DependsOn = []string{implID}
				plan.Steps[i].InputArtifacts = []string{implID}
				plan.Steps[i].Parallelizable = false
			}
			if companyContains(plan.Steps[i].RequiredSkills, "writing") && implID != "" && len(plan.Steps[i].DependsOn) == 0 {
				plan.Steps[i].DependsOn = []string{implID}
				plan.Steps[i].InputArtifacts = []string{implID}
			}
		}
	}
	for i := range plan.Steps {
		if strings.TrimSpace(plan.Steps[i].WorkerAgentID) == "" {
			selected, reason := companySelectWorker(&plan.Steps[i], registry, workers)
			plan.Steps[i].WorkerAgentID = selected
			plan.Steps[i].AssignmentReason = reason
		}
	}
	allSame := true
	firstGoal := strings.TrimSpace(plan.Steps[0].Goal)
	for i := range plan.Steps {
		if strings.TrimSpace(plan.Steps[i].Goal) != firstGoal {
			allSame = false
			break
		}
	}
	if allSame && len(plan.Steps) > 1 {
		return companyBuildSOPPlan(task, workers, registry)
	}
	return plan
}

func companyBuildPlanStepInput(plan companyTaskPlan, step companyPlanStep) map[string]interface{} {
	return map[string]interface{}{
		"taskType":          plan.TaskType,
		"summary":           plan.Summary,
		"goal":              step.Goal,
		"requiredSkills":    step.RequiredSkills,
		"inputArtifacts":    step.InputArtifacts,
		"expectedArtifacts": step.ExpectedArtifacts,
		"dependsOn":         step.DependsOn,
		"parallelizable":    step.Parallelizable,
		"retryable":         step.Retryable,
		"maxRetries":        step.MaxRetries,
		"assignmentReason":  step.AssignmentReason,
		"critical":          step.Critical,
		"attempt":           0,
	}
}

func companyStringList(value interface{}) []string {
	items := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				items = append(items, trimmed)
			}
		}
	case []interface{}:
		for _, item := range typed {
			if trimmed := strings.TrimSpace(toString(item)); trimmed != "" {
				items = append(items, trimmed)
			}
		}
	}
	return items
}

func companyPlanStepFromModel(step model.CompanyTaskStep) companyPlanStep {
	meta := companyPlanStep{
		ID:               strings.TrimSpace(step.StepKey),
		Title:            strings.TrimSpace(step.Title),
		Instruction:      strings.TrimSpace(step.Instruction),
		WorkerAgentID:    strings.TrimSpace(step.WorkerAgentID),
		Goal:             strings.TrimSpace(toString(step.Input["goal"])),
		Parallelizable:   asBool(step.Input["parallelizable"]),
		Retryable:        step.Input["retryable"] == nil || asBool(step.Input["retryable"]),
		MaxRetries:       int(companyInt64(step.Input["maxRetries"])),
		AssignmentReason: strings.TrimSpace(toString(step.Input["assignmentReason"])),
		Critical:         asBool(step.Input["critical"]),
	}
	meta.RequiredSkills = companyStringList(step.Input["requiredSkills"])
	meta.DependsOn = companyStringList(step.Input["dependsOn"])
	meta.ExpectedArtifacts = companyStringList(step.Input["expectedArtifacts"])
	meta.InputArtifacts = companyStringList(step.Input["inputArtifacts"])
	if meta.MaxRetries <= 0 {
		meta.MaxRetries = 1
	}
	return meta
}

func companyStepAttempt(step model.CompanyTaskStep) int {
	value := int(companyInt64(step.Input["attempt"]))
	if value < 0 {
		return 0
	}
	return value
}

func companySetStepAttempt(step *model.CompanyTaskStep, attempt int) {
	if step.Input == nil {
		step.Input = map[string]interface{}{}
	}
	step.Input["attempt"] = attempt
}

func companyResolveDependencyOutputs(steps []model.CompanyTaskStep, dependsOn []string) map[string]string {
	outputs := map[string]string{}
	for _, dep := range dependsOn {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		for _, step := range steps {
			if strings.TrimSpace(step.StepKey) == dep && strings.TrimSpace(step.OutputText) != "" {
				outputs[dep] = strings.TrimSpace(step.OutputText)
				break
			}
		}
	}
	return outputs
}

func companyFormatCollaborationSnapshot(snapshot *companyTaskCollaborationSnapshot) string {
	if snapshot == nil {
		return ""
	}
	formatAgentList := func(items []companyTeamAgentView) string {
		if len(items) == 0 {
			return "无"
		}
		parts := make([]string, 0, len(items))
		for _, item := range items {
			label := item.AgentName
			if label == "" {
				label = item.AgentID
			}
			if item.RoleType != "" {
				label += fmt.Sprintf("(%s)", item.RoleType)
			}
			parts = append(parts, label)
		}
		return strings.Join(parts, "、")
	}
	formatVisible := func(items []companyVisibleAgentView) string {
		if len(items) == 0 {
			return "无"
		}
		parts := make([]string, 0, len(items))
		for _, item := range items {
			if !item.VisibleInChannel {
				continue
			}
			label := item.AgentName
			if label == "" {
				label = item.AgentID
			}
			if item.VisibleForAccount {
				label += "[当前账号可见]"
			} else {
				label += "[当前账号不可见]"
			}
			parts = append(parts, label)
		}
		if len(parts) == 0 {
			return "无"
		}
		return strings.Join(parts, "、")
	}
	formatCallable := func(items []companyCallableAgentView) string {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			if !item.CallableNow {
				continue
			}
			label := item.AgentName
			if label == "" {
				label = item.AgentID
			}
			parts = append(parts, label)
		}
		if len(parts) == 0 {
			return "无"
		}
		return strings.Join(parts, "、")
	}
	return strings.Join([]string{
		"协作成员快照：",
		fmt.Sprintf("- 团队已注册成员：%s", formatAgentList(snapshot.TeamMembers)),
		fmt.Sprintf("- 当前渠道可见成员：%s", formatVisible(snapshot.VisibleAgents)),
		fmt.Sprintf("- 当前会话可直接调用成员：%s", formatCallable(snapshot.CallableAgents)),
		fmt.Sprintf("- 当前任务参与成员：%s", strings.Join(snapshot.ActiveStepAgents, "、")),
		fmt.Sprintf("- 你的上游协作者：%s", strings.Join(snapshot.UpstreamAgents, "、")),
		fmt.Sprintf("- 你的下游协作者：%s", strings.Join(snapshot.DownstreamAgents, "、")),
		"注意：团队中存在但当前不可调用的成员，不代表不存在，只是当前会话未直接授权。",
	}, "\n")
}

func companyPlanningPrompt(task *model.CompanyTask, workers []string, registry map[string]companyAgentCapability) string {
	capLines := make([]string, 0, len(workers))
	for _, worker := range workers {
		cap := registry[worker]
		capLines = append(capLines, fmt.Sprintf("- %s | role=%s | skills=%s | languages=%s | tools=%s", worker, cap.Role, strings.Join(cap.Skills, ","), strings.Join(cap.Languages, ","), strings.Join(cap.Tools, ",")))
	}
	return strings.Join([]string{
		"你是 AI 公司 Manager，负责按能力拆任务、分配 Worker、定义依赖关系。",
		fmt.Sprintf("任务标题：%s", task.Title),
		fmt.Sprintf("任务目标：%s", task.Goal),
		fmt.Sprintf("推断任务类型：%s", companyInferTaskType(task.Title, task.Goal)),
		"可用 Worker 能力：",
		strings.Join(capLines, "\n"),
		"要求：",
		"1. 优先最少且必要步骤，不要把相同 goal 广播给所有 worker。",
		"2. 校验/测试/审查类步骤必须依赖上游实现产物，不能直接基于原始 goal 空审。",
		"3. 文档/翻译类步骤只能整理和解释上游产物，不要重复实现类工作。",
		"4. 输出 JSON：{\"taskType\":\"...\",\"summary\":\"...\",\"steps\":[{\"id\":\"...\",\"title\":\"...\",\"goal\":\"...\",\"requiredSkills\":[\"...\"],\"inputArtifacts\":[\"...\"],\"expectedArtifacts\":[\"...\"],\"dependsOn\":[\"...\"],\"parallelizable\":true,\"retryable\":true,\"maxRetries\":1,\"workerAgentId\":\"可选，留空则由系统分配\"}]}",
		"5. 最多输出 4 个步骤。不要输出解释文本。",
	}, "\n")
}

func companyRuntimeSession(taskID, panelSessionID, agentID string) panelChatSession {
	sid := strings.TrimSpace(panelSessionID)
	if sid == "" {
		sid = fmt.Sprintf("company-%s", taskID)
	}
	return panelChatSession{ID: sid, OpenClawSessionID: fmt.Sprintf("%s-%s", taskID, agentID), AgentID: agentID, ChatType: "direct"}
}

func companyLikelyToolLeadIn(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	if strings.HasSuffix(trimmed, ":") {
		return true
	}
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "现在创建") || strings.Contains(lower, "我来") || strings.Contains(lower, "让我")
}

func companyRecoverExchangeOutput(messages []map[string]interface{}, userMessage string) string {
	start := -1
	for i := len(messages) - 1; i >= 0; i-- {
		role := strings.TrimSpace(toString(messages[i]["role"]))
		content := strings.TrimSpace(toString(messages[i]["content"]))
		if role == "user" && content == strings.TrimSpace(userMessage) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	bestAssistant := ""
	bestToolResult := ""
	for i := start + 1; i < len(messages); i++ {
		role := strings.TrimSpace(toString(messages[i]["role"]))
		content := sanitizePanelChatContent(toString(messages[i]["content"]))
		if strings.TrimSpace(content) == "" {
			continue
		}
		switch role {
		case "assistant":
			if companyLikelyToolLeadIn(content) {
				if bestAssistant == "" {
					bestAssistant = content
				}
				continue
			}
			bestAssistant = content
		case "toolResult":
			if len(strings.TrimSpace(content)) > len(strings.TrimSpace(bestToolResult)) {
				bestToolResult = content
			}
		}
	}
	if strings.TrimSpace(bestAssistant) != "" && !companyLikelyToolLeadIn(bestAssistant) {
		return bestAssistant
	}
	if strings.TrimSpace(bestToolResult) != "" {
		return bestToolResult
	}
	return bestAssistant
}

func companyRecoverPanelChatReply(cfg *config.Config, session panelChatSession, prompt string) string {
	rawMessages, historyErr := readSessionMessages(panelChatRuntimeSessionFile(cfg, session), 400)
	if historyErr != nil {
		return ""
	}
	if recovered := companyRecoverExchangeOutput(rawMessages, prompt); strings.TrimSpace(recovered) != "" {
		return sanitizePanelChatContent(recovered)
	}
	return sanitizePanelChatContent(extractLatestAssistantReply(rawMessages, prompt))
}

func companyBestEffortSummary(task *model.CompanyTask, steps []model.CompanyTaskStep) (string, string, string) {
	meaningful := make([]model.CompanyTaskStep, 0, len(steps))
	for _, step := range steps {
		text := strings.TrimSpace(step.OutputText)
		if text == "" {
			continue
		}
		meaningful = append(meaningful, step)
	}
	if len(meaningful) == 0 {
		return "revise", "all worker outputs are empty", "本次任务未产出有效结果，请重新执行。"
	}
	review := "approved"
	comment := "fallback summary from worker outputs"
	parts := make([]string, 0, len(meaningful))
	for _, step := range meaningful {
		parts = append(parts, fmt.Sprintf("[%s] %s\n%s", step.WorkerAgentID, step.Title, strings.TrimSpace(step.OutputText)))
	}
	if reviewStep := func() *model.CompanyTaskStep {
		for i := range meaningful {
			meta := companyPlanStepFromModel(meaningful[i])
			if companyContains(meta.ExpectedArtifacts, "review_report") || companyContains(meta.RequiredSkills, "validation") || companyContains(meta.RequiredSkills, "review") {
				return &meaningful[i]
			}
		}
		return nil
	}(); reviewStep != nil {
		text := strings.TrimSpace(reviewStep.OutputText)
		if strings.Contains(text, "验收通过") || strings.Contains(text, "可直接交付") || strings.Contains(text, "任务已100%完成") {
			review = "approved"
			comment = "review fallback accepted"
		} else {
			review = "revise"
			comment = "review fallback requires revision"
		}
	}
	summary := strings.Join(parts, "\n\n")
	if len(meaningful) == 1 {
		summary = strings.TrimSpace(meaningful[0].OutputText)
	}
	return review, comment, summary
}

func companyManagerPlan(ctx *gin.Context, db *sql.DB, cfg *config.Config, task *model.CompanyTask, workers []string, registry map[string]companyAgentCapability) companyTaskPlan {
	prompt := companyPlanningPrompt(task, workers, registry)
	sources := retrievePanelChatKnowledge(db, cfg, task.ManagerAgentID, task.PanelSessionID, task.Goal)
	if len(sources) > 0 {
		prompt = injectPanelChatKnowledge(prompt, sources)
	}
	runtimeSession := companyRuntimeSession(task.ID, task.PanelSessionID, task.ManagerAgentID)
	reply, actualSessionID, err := runPanelChatMessage(ctx.Request.Context(), cfg, runtimeSession, prompt)
	if strings.TrimSpace(actualSessionID) != "" {
		runtimeSession.OpenClawSessionID = strings.TrimSpace(actualSessionID)
	}
	if strings.TrimSpace(reply) == "" || err != nil {
		if recovered := companyRecoverPanelChatReply(cfg, runtimeSession, prompt); strings.TrimSpace(recovered) != "" {
			reply = recovered
			if errors.Is(err, errPanelChatTimeout) {
				err = nil
			}
		}
	}
	if err != nil {
		return companyBuildSOPPlan(task, workers, registry)
	}
	plan := parseCompanyPlan(reply)
	plan = companyNormalizePlan(task, plan, workers, registry)
	if len(plan.Steps) == 0 {
		return companyBuildSOPPlan(task, workers, registry)
	}
	return plan
}

func companyWorkerExecute(ctx *gin.Context, db *sql.DB, cfg *config.Config, task *model.CompanyTask, steps []model.CompanyTaskStep, step *model.CompanyTaskStep, dependencyOutputs map[string]string) (string, error) {
	meta := companyPlanStepFromModel(*step)
	collabScope := companyRuntimeScope{SessionKind: "company_task", ManagerAgentID: task.ManagerAgentID}
	collabSnapshot, _ := buildTaskCollaborationSnapshot(db, cfg, task, steps, step, collabScope)
	lines := []string{
		"你是 AI 公司 Worker，请只完成当前被分配的单个步骤。",
		fmt.Sprintf("任务类型：%s", strings.TrimSpace(toString(step.Input["taskType"]))),
		fmt.Sprintf("总任务摘要：%s", strings.TrimSpace(toString(step.Input["summary"]))),
		fmt.Sprintf("当前步骤：%s", step.Title),
		fmt.Sprintf("当前步骤目标：%s", meta.Goal),
		fmt.Sprintf("预期产物：%s", strings.Join(meta.ExpectedArtifacts, ", ")),
	}
	if collabText := companyFormatCollaborationSnapshot(collabSnapshot); strings.TrimSpace(collabText) != "" {
		lines = append(lines, collabText)
	}
	if len(dependencyOutputs) > 0 {
		lines = append(lines, "你只能基于以下上游产物继续，不要假设未提供的信息：")
		for _, dep := range meta.DependsOn {
			if output := strings.TrimSpace(dependencyOutputs[dep]); output != "" {
				lines = append(lines, fmt.Sprintf("- 上游产物 %s:\n%s", dep, output))
			}
		}
	} else {
		lines = append(lines, fmt.Sprintf("原始任务背景：%s", task.Goal))
	}
	lines = append(lines,
		"只输出当前步骤对应的可交付结果。",
		"不要继续替其他角色撰写文档或审查，不要把任务再拆给别人。",
	)
	prompt := strings.Join(lines, "\n")
	sources := retrievePanelChatKnowledge(db, cfg, step.WorkerAgentID, task.PanelSessionID, task.Goal+"\n"+step.Instruction)
	if len(sources) > 0 {
		prompt = injectPanelChatKnowledge(prompt, sources)
	}
	runtimeSession := companyRuntimeSession(task.ID, task.PanelSessionID, step.WorkerAgentID)
	reply, actualSessionID, err := runPanelChatMessage(ctx.Request.Context(), cfg, runtimeSession, prompt)
	if strings.TrimSpace(actualSessionID) != "" {
		runtimeSession.OpenClawSessionID = strings.TrimSpace(actualSessionID)
	}
	if strings.TrimSpace(reply) == "" || err != nil {
		if recovered := companyRecoverPanelChatReply(cfg, runtimeSession, prompt); strings.TrimSpace(recovered) != "" {
			reply = recovered
			if errors.Is(err, errPanelChatTimeout) {
				err = nil
			}
		}
	}
	if err != nil {
		return "", err
	}
	return sanitizePanelChatContent(reply), nil
}

func companyManagerSummarize(ctx *gin.Context, db *sql.DB, cfg *config.Config, task *model.CompanyTask, steps []model.CompanyTaskStep) (string, string, string) {
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		parts = append(parts, fmt.Sprintf("[%s] %s\n%s", step.WorkerAgentID, step.Title, strings.TrimSpace(step.OutputText)))
	}
	prompt := strings.Join([]string{
		"你是 AI 公司 Manager，负责审核 Worker 结果并输出最终总结。",
		fmt.Sprintf("原始任务：%s", task.Goal),
		"请按以下格式输出：",
		"REVIEW: approved 或 revise",
		"COMMENT: 一句话说明",
		"SUMMARY:",
		"面向用户的最终结果",
		"以下是 Worker 输出：",
		strings.Join(parts, "\n\n"),
	}, "\n")
	sources := retrievePanelChatKnowledge(db, cfg, task.SummaryAgentID, task.PanelSessionID, task.Goal)
	if len(sources) > 0 {
		prompt = injectPanelChatKnowledge(prompt, sources)
	}
	runtimeSession := companyRuntimeSession(task.ID, task.PanelSessionID, task.SummaryAgentID)
	reply, actualSessionID, err := runPanelChatMessage(ctx.Request.Context(), cfg, runtimeSession, prompt)
	if strings.TrimSpace(actualSessionID) != "" {
		runtimeSession.OpenClawSessionID = strings.TrimSpace(actualSessionID)
	}
	if strings.TrimSpace(reply) == "" || err != nil {
		if recovered := companyRecoverPanelChatReply(cfg, runtimeSession, prompt); strings.TrimSpace(recovered) != "" {
			reply = recovered
			if errors.Is(err, errPanelChatTimeout) {
				err = nil
			}
		}
	}
	if err != nil || strings.TrimSpace(reply) == "" {
		return companyBestEffortSummary(task, steps)
	}
	text := sanitizePanelChatContent(reply)
	review := "approved"
	comment := "manager reviewed"
	summary := text
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "review:") {
			value := strings.TrimSpace(trimmed[len("review:"):])
			if value != "" {
				review = value
			}
		}
		if strings.HasPrefix(lower, "comment:") {
			value := strings.TrimSpace(trimmed[len("comment:"):])
			if value != "" {
				comment = value
			}
		}
	}
	if idx := strings.Index(strings.ToUpper(text), "SUMMARY:"); idx >= 0 {
		summary = strings.TrimSpace(text[idx+len("SUMMARY:"):])
	}
	if summary == "" {
		summary = text
	}
	return review, comment, summary
}

func companyAllDependenciesCompleted(step model.CompanyTaskStep, statuses map[string]string) bool {
	for _, dep := range companyPlanStepFromModel(step).DependsOn {
		if statuses[strings.TrimSpace(dep)] != "completed" {
			return false
		}
	}
	return true
}

func companyHasFailedDependency(step model.CompanyTaskStep, statuses map[string]string) bool {
	for _, dep := range companyPlanStepFromModel(step).DependsOn {
		status := statuses[strings.TrimSpace(dep)]
		if status == "failed" || status == "blocked" {
			return true
		}
	}
	return false
}

func companyFindAlternativeWorker(step model.CompanyTaskStep, registry map[string]companyAgentCapability, workers []string) (string, string) {
	meta := companyPlanStepFromModel(step)
	return companySelectWorker(&meta, registry, workers, step.WorkerAgentID)
}

type companyStepExecResult struct {
	Index          int
	Output         string
	Err            error
	FallbackWorker string
	FallbackReason string
}

func companyFinalizeTaskStatus(task *model.CompanyTask, steps []model.CompanyTaskStep, review, comment, summary string) {
	successCount := 0
	failedCount := 0
	blockedCount := 0
	criticalSuccess := false
	for _, step := range steps {
		meta := companyPlanStepFromModel(step)
		switch step.Status {
		case "completed":
			successCount++
			if meta.Critical && strings.TrimSpace(step.OutputText) != "" {
				criticalSuccess = true
			}
		case "failed":
			failedCount++
		case "blocked":
			blockedCount++
		}
	}
	if successCount == 0 {
		task.Status = "failed"
		if strings.TrimSpace(review) == "approved" {
			review = "revise"
		}
		if strings.TrimSpace(comment) == "" {
			comment = "all worker steps failed"
		}
	} else if failedCount > 0 || blockedCount > 0 || strings.TrimSpace(review) != "approved" || !criticalSuccess {
		task.Status = "completed_with_errors"
		if !criticalSuccess {
			review = "revise"
			if strings.TrimSpace(comment) == "" {
				comment = "critical deliverable missing"
			}
		}
	} else {
		task.Status = "completed"
	}
	task.ReviewResult = review
	task.ReviewComment = comment
	task.ResultText = summary
}

func executeCompanyTask(ctx context.Context, db *sql.DB, cfg *config.Config, taskID string) {
	task, err := model.GetCompanyTask(db, taskID)
	if err != nil || task == nil {
		return
	}
	team, err := resolveCompanyTeam(db, cfg, task.TeamID)
	if err != nil {
		task.Status = "failed"
		task.ReviewResult = "revise"
		task.ReviewComment = "team resolve failed"
		task.ResultText = err.Error()
		_ = model.UpdateCompanyTask(db, task)
		_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, EventType: "task_failed", Message: "任务执行失败：无法加载团队", Payload: map[string]interface{}{"error": err.Error()}})
		return
	}

	task.Status = "running"
	_ = model.UpdateCompanyTask(db, task)
	_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, EventType: "task_started", Message: "任务已进入后台执行"})

	manager, workers, _ := companyParticipantsFromTeam(cfg, team, companyCreateTaskRequest{
		ManagerAgentID: task.ManagerAgentID,
		SummaryAgentID: task.SummaryAgentID,
	})
	if manager != "" {
		task.ManagerAgentID = manager
	}
	if len(workers) == 0 {
		for _, item := range team.Agents {
			if strings.TrimSpace(item.AgentID) == "" || strings.EqualFold(item.RoleType, "manager") {
				continue
			}
			workers = append(workers, strings.TrimSpace(item.AgentID))
		}
	}
	if len(workers) == 0 {
		task.Status = "failed"
		task.ReviewResult = "revise"
		task.ReviewComment = "no worker agents available"
		task.ResultText = "no worker agents available"
		_ = model.UpdateCompanyTask(db, task)
		_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, EventType: "task_failed", Message: "任务执行失败：没有可用 Worker", Payload: map[string]interface{}{"error": "no worker agents available"}})
		return
	}

	registry := companyCapabilityRegistry(cfg)
	plan := companyManagerPlan(&gin.Context{Request: (&http.Request{}).WithContext(ctx)}, db, cfg, task, workers, registry)
	steps := make([]model.CompanyTaskStep, 0, len(plan.Steps))
	for i, item := range plan.Steps {
		instruction := strings.TrimSpace(item.Instruction)
		if instruction == "" {
			instruction = strings.TrimSpace(item.Goal)
		}
		stepKey := strings.TrimSpace(item.ID)
		if stepKey == "" {
			stepKey = fmt.Sprintf("step_%d", i+1)
		}
		steps = append(steps, model.CompanyTaskStep{
			ID:            fmt.Sprintf("%s-%s", task.ID, stepKey),
			TaskID:        task.ID,
			StepKey:       stepKey,
			Title:         item.Title,
			Instruction:   instruction,
			WorkerAgentID: item.WorkerAgentID,
			Status:        "pending",
			OrderIndex:    i,
			Input:         companyBuildPlanStepInput(plan, item),
		})
	}
	if err := model.ReplaceCompanyTaskSteps(db, task.ID, steps); err != nil {
		task.Status = "failed"
		task.ReviewResult = "revise"
		task.ReviewComment = "step persistence failed"
		task.ResultText = err.Error()
		_ = model.UpdateCompanyTask(db, task)
		_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, EventType: "task_failed", Message: "任务执行失败：无法保存步骤", Payload: map[string]interface{}{"error": err.Error()}})
		return
	}
	_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, EventType: "task_planned", Message: fmt.Sprintf("Manager 已拆解 %d 个步骤", len(steps)), Payload: map[string]interface{}{"stepCount": len(steps), "taskType": plan.TaskType, "summary": plan.Summary}})

	statuses := map[string]string{}
	for _, step := range steps {
		statuses[step.StepKey] = step.Status
	}
	for {
		ready := make([]int, 0)
		pending := 0
		for i := range steps {
			statuses[steps[i].StepKey] = steps[i].Status
			if steps[i].Status == "completed" || steps[i].Status == "failed" || steps[i].Status == "blocked" {
				continue
			}
			pending++
			if companyHasFailedDependency(steps[i], statuses) {
				steps[i].Status = "blocked"
				steps[i].ErrorText = "blocked by failed dependency"
				_ = model.UpdateCompanyTaskStep(db, &steps[i])
				_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, StepID: steps[i].ID, EventType: "step_blocked", Message: fmt.Sprintf("%s 被阻塞：依赖步骤失败", steps[i].Title)})
				statuses[steps[i].StepKey] = steps[i].Status
				continue
			}
			if companyAllDependenciesCompleted(steps[i], statuses) {
				ready = append(ready, i)
			}
		}
		if pending == 0 || len(ready) == 0 {
			break
		}
		batch := ready
		for _, idx := range ready {
			if !companyPlanStepFromModel(steps[idx]).Parallelizable {
				batch = []int{idx}
				break
			}
		}
		results := make(chan companyStepExecResult, len(batch))
		var wg sync.WaitGroup
		for _, idx := range batch {
			steps[idx].Status = "running"
			_ = model.UpdateCompanyTaskStep(db, &steps[idx])
			_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, StepID: steps[idx].ID, EventType: "step_started", Message: fmt.Sprintf("%s 开始执行：%s", steps[idx].WorkerAgentID, steps[idx].Title), Payload: map[string]interface{}{"dependsOn": companyPlanStepFromModel(steps[idx]).DependsOn}})
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				bgGin := &gin.Context{Request: (&http.Request{}).WithContext(ctx)}
				deps := companyPlanStepFromModel(steps[index]).DependsOn
				depOutputs := companyResolveDependencyOutputs(steps, deps)
				output, execErr := companyWorkerExecute(bgGin, db, cfg, task, steps, &steps[index], depOutputs)
				results <- companyStepExecResult{Index: index, Output: output, Err: execErr}
			}(idx)
		}
		wg.Wait()
		close(results)
		for result := range results {
			step := &steps[result.Index]
			meta := companyPlanStepFromModel(*step)
			if result.Err != nil {
				attempt := companyStepAttempt(*step)
				if meta.Retryable && attempt < meta.MaxRetries {
					companySetStepAttempt(step, attempt+1)
					step.Status = "pending"
					step.ErrorText = result.Err.Error()
					_ = model.UpdateCompanyTaskStep(db, step)
					_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, StepID: step.ID, EventType: "step_retry_scheduled", Message: fmt.Sprintf("%s 计划重试，第 %d 次", step.Title, attempt+1), Payload: map[string]interface{}{"error": result.Err.Error()}})
					continue
				}
				if alt, reason := companyFindAlternativeWorker(*step, registry, workers); alt != "" {
					step.WorkerAgentID = alt
					if step.Input == nil {
						step.Input = map[string]interface{}{}
					}
					step.Input["assignmentReason"] = reason
					companySetStepAttempt(step, 0)
					step.Status = "pending"
					step.ErrorText = result.Err.Error()
					_ = model.UpdateCompanyTaskStep(db, step)
					_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, StepID: step.ID, EventType: "task_replanned", Message: fmt.Sprintf("Manager 将 %s 改派给 %s", step.Title, alt), Payload: map[string]interface{}{"previousError": result.Err.Error(), "assignmentReason": reason}})
					continue
				}
				step.Status = "failed"
				step.ErrorText = result.Err.Error()
				_ = model.UpdateCompanyTaskStep(db, step)
				_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, StepID: step.ID, EventType: "step_failed", Message: fmt.Sprintf("%s 执行失败", step.WorkerAgentID), Payload: map[string]interface{}{"error": result.Err.Error()}})
				continue
			}
			output := sanitizePanelChatContent(result.Output)
			if strings.TrimSpace(output) == "" {
				step.Status = "failed"
				step.ErrorText = "empty worker output"
				_ = model.UpdateCompanyTaskStep(db, step)
				_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, StepID: step.ID, EventType: "step_failed", Message: fmt.Sprintf("%s 执行失败：无有效输出", step.WorkerAgentID), Payload: map[string]interface{}{"error": "empty worker output"}})
				continue
			}
			step.Status = "completed"
			step.OutputText = output
			step.ErrorText = ""
			_ = model.UpdateCompanyTaskStep(db, step)
			_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, StepID: step.ID, EventType: "step_completed", Message: fmt.Sprintf("%s 已完成：%s", step.WorkerAgentID, step.Title), Payload: map[string]interface{}{"expectedArtifacts": meta.ExpectedArtifacts}})
		}
	}

	bgGin := &gin.Context{Request: (&http.Request{}).WithContext(ctx)}
	review, comment, summaryText := companyManagerSummarize(bgGin, db, cfg, task, steps)
	companyFinalizeTaskStatus(task, steps, review, comment, summaryText)
	if err := model.UpdateCompanyTask(db, task); err != nil {
		return
	}
	_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, EventType: "task_reviewed", Message: fmt.Sprintf("Manager 已完成审核：%s", task.ReviewResult), Payload: map[string]interface{}{"reviewComment": task.ReviewComment}})

	events, _ := model.ListCompanyEvents(db, task.ID)
	if task.DeliveryType == companyDeliveryWriteBackPanelSession {
		_, _ = companyWriteBackToPanelSession(cfg, task, events)
	}
}

func companyAppendPanelChatMessages(cfg *config.Config, sessionID string, additions ...map[string]interface{}) ([]map[string]interface{}, error) {
	messages, err := loadPanelChatMessages(cfg, sessionID)
	if err != nil {
		return nil, err
	}
	messages = append(messages, additions...)
	if err := savePanelChatMessages(cfg, sessionID, messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func companyWriteBackToPanelSession(cfg *config.Config, task *model.CompanyTask, events []model.CompanyEvent) ([]map[string]interface{}, error) {
	if strings.TrimSpace(task.PanelSessionID) == "" {
		return nil, nil
	}
	nameMap := companyAgentNameMap(cfg)
	systemMsgs := make([]map[string]interface{}, 0, len(events)+1)
	for _, event := range events {
		systemMsgs = append(systemMsgs, map[string]interface{}{
			"id":          fmt.Sprintf("company-%s-%d", task.ID, time.Now().UnixNano()),
			"role":        "system",
			"senderType":  "system",
			"messageType": event.EventType,
			"content":     event.Message,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"taskId":      task.ID,
		})
	}
	systemMsgs = append(systemMsgs, map[string]interface{}{
		"id":          fmt.Sprintf("company-summary-%d", time.Now().UnixNano()),
		"role":        "assistant",
		"senderType":  "agent",
		"agentId":     task.SummaryAgentID,
		"agentName":   nameMap[task.SummaryAgentID],
		"messageType": "task_summary",
		"content":     task.ResultText,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"taskId":      task.ID,
	})
	messages, err := companyAppendPanelChatMessages(cfg, task.PanelSessionID, systemMsgs...)
	if err != nil {
		return nil, err
	}
	_, _ = updatePanelChatSessionState(cfg, task.PanelSessionID, func(item *panelChatSession) {
		item.UpdatedAt = time.Now().UnixMilli()
		item.Processing = false
		item.CurrentAgentID = ""
		item.CurrentAgentName = ""
		item.MessageCount = len(messages)
		item.LastMessage = task.Title
	})
	return messages, nil
}

func SaveCompanyTeam(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req companySaveTeamRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid payload"})
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "team name required"})
			return
		}
		managerID := strings.TrimSpace(req.ManagerAgentID)
		if managerID == "" {
			managerID = companyDefaultManagerAgentID(cfg)
		}
		workerIDs := companyNormalizeWorkerAgentIDs(managerID, req.WorkerAgentIDs)
		if len(workerIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "at least one worker agent required"})
			return
		}
		teamID := strings.TrimSpace(c.Param("id"))
		if teamID == "" {
			teamID = fmt.Sprintf("company-team-%d", time.Now().UnixMilli())
		}
		status := strings.TrimSpace(req.Status)
		if status == "" {
			status = "active"
		}
		team := &model.CompanyTeam{
			ID:                    teamID,
			Name:                  name,
			Description:           strings.TrimSpace(req.Description),
			ManagerAgentID:        managerID,
			DefaultSummaryAgentID: managerID,
			Status:                status,
		}
		if existing, err := model.GetCompanyTeam(db, teamID); err == nil && existing != nil {
			team.CreatedAt = existing.CreatedAt
		}
		if err := model.UpsertCompanyTeam(db, team); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		agents := companyNormalizedTeamAgents(cfg, managerID, nil, workerIDs)
		if err := model.ReplaceCompanyTeamAgents(db, team.ID, agents); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		team.Agents = agents
		c.JSON(http.StatusOK, gin.H{"ok": true, "team": companyNormalizeTeam(cfg, team)})
	}
}

func ListCompanyTeams(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _ = ensureDefaultCompanyTeam(db, cfg)
		teams, err := model.ListCompanyTeams(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		for i := range teams {
			agents, _ := model.ListCompanyTeamAgents(db, teams[i].ID)
			teams[i].Agents = agents
			companyNormalizeTeam(cfg, &teams[i])
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "teams": teams})
	}
}

func GetCompanyTeamDetail(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		team, err := resolveCompanyTeam(db, cfg, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "team": team})
	}
}

func GetCompanyOverview(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _ = ensureDefaultCompanyTeam(db, cfg)
		teams, _ := model.ListCompanyTeams(db)
		tasks, _ := model.ListCompanyTasks(db, 8)
		overview := companyOverview{TeamCount: len(teams), TaskCount: len(tasks), RecentTasks: tasks}
		for _, task := range tasks {
			if task.Status == "running" || task.Status == "pending" {
				overview.RunningCount++
			}
			if task.Status == "completed" {
				overview.CompletedCount++
			}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "overview": overview})
	}
}

func GetCompanyChannels() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "channels": companyKnownChannels()})
	}
}

func GetCompanyCapabilities() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ok":            true,
			"sourceTypes":   []string{"panel_chat", "panel_manual", "qq", "qqbot", "wechat", "feishu", "wecom", "wecom-app", "api", "webhook"},
			"deliveryTypes": []string{"write_back_panel_session", "notify_only", "send_to_qq", "send_to_qqbot", "send_to_wechat", "send_to_feishu", "send_to_wecom", "send_to_wecom_app", "api_response", "webhook_callback"},
			"channels":      companyKnownChannels(),
		})
	}
}

func ListCompanyTasks(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tasks, err := model.ListCompanyTasks(db, 100)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "tasks": tasks})
	}
}

func CreateCompanyTask(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req companyCreateTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Goal) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "goal required"})
			return
		}
		req.SourceType = normalizeCompanySourceType(req.SourceType)
		req.DeliveryType = normalizeCompanyDeliveryType(req.DeliveryType)
		if req.SourceType == companySourcePanelChat && strings.TrimSpace(req.PanelSessionID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "panel session required for panel_chat source"})
			return
		}
		if req.DeliveryType == companyDeliveryWriteBackPanelSession && strings.TrimSpace(req.PanelSessionID) == "" && strings.TrimSpace(req.DeliveryTargetID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "delivery target required"})
			return
		}
		if strings.TrimSpace(req.PanelSessionID) != "" {
			sessions, err := loadPanelChatSessions(cfg)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
			if _, session := findPanelChatSession(sessions, strings.TrimSpace(req.PanelSessionID)); session == nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "panel session not found"})
				return
			}
		}
		team, err := resolveCompanyTeam(db, cfg, req.TeamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		manager, workers, summary := companyParticipantsFromTeam(cfg, team, req)
		if len(workers) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "no worker agents available"})
			return
		}
		task := &model.CompanyTask{
			ID:                  fmt.Sprintf("company-task-%d", time.Now().UnixMilli()),
			TeamID:              team.ID,
			Title:               buildPanelChatTitle(req.Title),
			Goal:                strings.TrimSpace(req.Goal),
			Status:              "pending",
			ManagerAgentID:      manager,
			SummaryAgentID:      summary,
			SourceType:          req.SourceType,
			SourceChannelType:   strings.TrimSpace(req.SourceChannelType),
			SourceChannelID:     strings.TrimSpace(req.SourceChannelID),
			SourceThreadID:      strings.TrimSpace(req.SourceThreadID),
			SourceMessageID:     strings.TrimSpace(req.SourceMessageID),
			SourceRefID:         strings.TrimSpace(req.SourceRefID),
			DeliveryType:        req.DeliveryType,
			DeliveryChannelType: strings.TrimSpace(req.DeliveryChannelType),
			DeliveryChannelID:   strings.TrimSpace(req.DeliveryChannelID),
			DeliveryThreadID:    strings.TrimSpace(req.DeliveryThreadID),
			DeliveryTargetID:    strings.TrimSpace(req.DeliveryTargetID),
			PanelSessionID:      strings.TrimSpace(req.PanelSessionID),
		}
		if task.Title == panelChatDefaultTitle {
			task.Title = buildPanelChatTitle(task.Goal)
		}
		if task.DeliveryType == companyDeliveryWriteBackPanelSession && task.DeliveryTargetID == "" {
			task.DeliveryTargetID = task.PanelSessionID
		}
		if err := model.CreateCompanyTask(db, task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		_ = model.AddCompanyEvent(db, &model.CompanyEvent{TaskID: task.ID, EventType: "task_created", Message: "任务已创建", Payload: map[string]interface{}{"sourceType": task.SourceType, "deliveryType": task.DeliveryType}})
		bundle, _ := loadCompanyTaskBundle(db, task.ID)
		go executeCompanyTask(context.Background(), db, cfg, task.ID)
		c.JSON(http.StatusOK, gin.H{"ok": true, "task": bundle, "queued": true, "message": "任务已创建，正在后台执行"})
	}
}

func GetCompanyTaskDetail(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		item, err := loadCompanyTaskBundle(db, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "task": item})
	}
}

func GetCompanyTaskSteps(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		steps, err := model.ListCompanyTaskSteps(db, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "steps": steps})
	}
}

func GetCompanyTaskEvents(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		events, err := model.ListCompanyEvents(db, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "events": events})
	}
}

func GetCompanyTaskResult(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		task, err := model.GetCompanyTask(db, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "result": task.ResultText, "reviewResult": task.ReviewResult, "reviewComment": task.ReviewComment})
	}
}
