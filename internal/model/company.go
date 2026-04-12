package model

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type CompanyTeam struct {
	ID                    string             `json:"id"`
	Name                  string             `json:"name"`
	Description           string             `json:"description,omitempty"`
	ManagerAgentID        string             `json:"managerAgentId,omitempty"`
	DefaultSummaryAgentID string             `json:"defaultSummaryAgentId,omitempty"`
	Status                string             `json:"status"`
	CreatedAt             int64              `json:"createdAt"`
	UpdatedAt             int64              `json:"updatedAt"`
	Agents                []CompanyTeamAgent `json:"agents,omitempty"`
}

type CompanyTeamAgent struct {
	ID        int64  `json:"id"`
	TeamID    string `json:"teamId"`
	AgentID   string `json:"agentId"`
	RoleType  string `json:"roleType"`
	DutyLabel string `json:"dutyLabel,omitempty"`
	SortOrder int    `json:"sortOrder"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
	AgentName string `json:"agentName,omitempty"`
}

type CompanyTask struct {
	ID                  string            `json:"id"`
	TeamID              string            `json:"teamId,omitempty"`
	Title               string            `json:"title"`
	Goal                string            `json:"goal"`
	Status              string            `json:"status"`
	ManagerAgentID      string            `json:"managerAgentId,omitempty"`
	SummaryAgentID      string            `json:"summaryAgentId,omitempty"`
	SourceType          string            `json:"sourceType"`
	SourceChannelType   string            `json:"sourceChannelType,omitempty"`
	SourceChannelID     string            `json:"sourceChannelId,omitempty"`
	SourceThreadID      string            `json:"sourceThreadId,omitempty"`
	SourceMessageID     string            `json:"sourceMessageId,omitempty"`
	SourceRefID         string            `json:"sourceRefId,omitempty"`
	DeliveryType        string            `json:"deliveryType"`
	DeliveryChannelType string            `json:"deliveryChannelType,omitempty"`
	DeliveryChannelID   string            `json:"deliveryChannelId,omitempty"`
	DeliveryThreadID    string            `json:"deliveryThreadId,omitempty"`
	DeliveryTargetID    string            `json:"deliveryTargetId,omitempty"`
	PanelSessionID      string            `json:"panelSessionId,omitempty"`
	ResultText          string            `json:"resultText,omitempty"`
	ReviewResult        string            `json:"reviewResult,omitempty"`
	ReviewComment       string            `json:"reviewComment,omitempty"`
	CreatedAt           int64             `json:"createdAt"`
	UpdatedAt           int64             `json:"updatedAt"`
	Steps               []CompanyTaskStep `json:"steps,omitempty"`
	Events              []CompanyEvent    `json:"events,omitempty"`
}

type CompanyTaskStep struct {
	ID            string                 `json:"id"`
	TaskID        string                 `json:"taskId"`
	StepKey       string                 `json:"stepKey"`
	Title         string                 `json:"title"`
	Instruction   string                 `json:"instruction"`
	WorkerAgentID string                 `json:"workerAgentId,omitempty"`
	Status        string                 `json:"status"`
	OrderIndex    int                    `json:"orderIndex"`
	Input         map[string]interface{} `json:"input,omitempty"`
	OutputText    string                 `json:"outputText,omitempty"`
	ErrorText     string                 `json:"errorText,omitempty"`
	CreatedAt     int64                  `json:"createdAt"`
	UpdatedAt     int64                  `json:"updatedAt"`
}

type CompanyEvent struct {
	ID        int64                  `json:"id"`
	TaskID    string                 `json:"taskId"`
	StepID    string                 `json:"stepId,omitempty"`
	EventType string                 `json:"eventType"`
	Message   string                 `json:"message"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	CreatedAt int64                  `json:"createdAt"`
}

func UpsertCompanyTeam(db *sql.DB, team *CompanyTeam) error {
	now := time.Now().UnixMilli()
	if team.CreatedAt == 0 {
		team.CreatedAt = now
	}
	team.UpdatedAt = now
	if team.Status == "" {
		team.Status = "active"
	}
	legacyManagerIDColumn, err := companyTeamHasColumn(db, "manager_id")
	if err != nil {
		return err
	}
	if legacyManagerIDColumn {
		_, err = db.Exec(`INSERT INTO company_teams (id, name, description, manager_id, manager_agent_id, default_summary_agent_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name, description=excluded.description, manager_id=excluded.manager_id, manager_agent_id=excluded.manager_agent_id, default_summary_agent_id=excluded.default_summary_agent_id, status=excluded.status, updated_at=excluded.updated_at`, team.ID, team.Name, team.Description, team.ManagerAgentID, team.ManagerAgentID, team.DefaultSummaryAgentID, team.Status, team.CreatedAt, team.UpdatedAt)
		return err
	}
	_, err = db.Exec(`INSERT INTO company_teams (id, name, description, manager_agent_id, default_summary_agent_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name, description=excluded.description, manager_agent_id=excluded.manager_agent_id, default_summary_agent_id=excluded.default_summary_agent_id, status=excluded.status, updated_at=excluded.updated_at`, team.ID, team.Name, team.Description, team.ManagerAgentID, team.DefaultSummaryAgentID, team.Status, team.CreatedAt, team.UpdatedAt)
	return err
}

func companyTeamHasColumn(db *sql.DB, column string) (bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM pragma_table_info('company_teams') WHERE name = ? LIMIT 1`, column).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.EqualFold(name, column), nil
}

func ReplaceCompanyTeamAgents(db *sql.DB, teamID string, items []CompanyTeamAgent) error {
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM company_team_agents WHERE team_id = ?`, teamID); err != nil {
		return err
	}
	for i, item := range items {
		if item.CreatedAt == 0 {
			item.CreatedAt = now
		}
		item.UpdatedAt = now
		if item.RoleType == "" {
			item.RoleType = "worker"
		}
		if _, err := tx.Exec(`INSERT INTO company_team_agents (team_id, agent_id, role_type, duty_label, sort_order, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, teamID, item.AgentID, item.RoleType, item.DutyLabel, i, boolToInt(item.Enabled), item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ListCompanyTeams(db *sql.DB) ([]CompanyTeam, error) {
	rows, err := db.Query(`SELECT id, name, description, manager_agent_id, default_summary_agent_id, status, created_at, updated_at FROM company_teams ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CompanyTeam, 0)
	for rows.Next() {
		var item CompanyTeam
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.ManagerAgentID, &item.DefaultSummaryAgentID, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func GetCompanyTeam(db *sql.DB, id string) (*CompanyTeam, error) {
	var item CompanyTeam
	err := db.QueryRow(`SELECT id, name, description, manager_agent_id, default_summary_agent_id, status, created_at, updated_at FROM company_teams WHERE id = ?`, id).Scan(&item.ID, &item.Name, &item.Description, &item.ManagerAgentID, &item.DefaultSummaryAgentID, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func ListCompanyTeamAgents(db *sql.DB, teamID string) ([]CompanyTeamAgent, error) {
	rows, err := db.Query(`SELECT id, team_id, agent_id, role_type, duty_label, sort_order, enabled, created_at, updated_at FROM company_team_agents WHERE team_id = ? ORDER BY sort_order ASC, id ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CompanyTeamAgent, 0)
	for rows.Next() {
		var item CompanyTeamAgent
		var enabled int
		if err := rows.Scan(&item.ID, &item.TeamID, &item.AgentID, &item.RoleType, &item.DutyLabel, &item.SortOrder, &enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled == 1
		items = append(items, item)
	}
	return items, nil
}

func CreateCompanyTask(db *sql.DB, task *CompanyTask) error {
	now := time.Now().UnixMilli()
	if task.CreatedAt == 0 {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	_, err := db.Exec(`INSERT INTO company_tasks (id, team_id, title, goal, status, manager_agent_id, summary_agent_id, source_type, source_channel_type, source_channel_id, source_thread_id, source_message_id, source_ref_id, delivery_type, delivery_channel_type, delivery_channel_id, delivery_thread_id, delivery_target_id, panel_session_id, result_text, review_result, review_comment, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, task.ID, task.TeamID, task.Title, task.Goal, task.Status, task.ManagerAgentID, task.SummaryAgentID, task.SourceType, task.SourceChannelType, task.SourceChannelID, task.SourceThreadID, task.SourceMessageID, task.SourceRefID, task.DeliveryType, task.DeliveryChannelType, task.DeliveryChannelID, task.DeliveryThreadID, task.DeliveryTargetID, task.PanelSessionID, task.ResultText, task.ReviewResult, task.ReviewComment, task.CreatedAt, task.UpdatedAt)
	return err
}

func UpdateCompanyTask(db *sql.DB, task *CompanyTask) error {
	task.UpdatedAt = time.Now().UnixMilli()
	_, err := db.Exec(`UPDATE company_tasks SET team_id = ?, title = ?, goal = ?, status = ?, manager_agent_id = ?, summary_agent_id = ?, source_type = ?, source_channel_type = ?, source_channel_id = ?, source_thread_id = ?, source_message_id = ?, source_ref_id = ?, delivery_type = ?, delivery_channel_type = ?, delivery_channel_id = ?, delivery_thread_id = ?, delivery_target_id = ?, panel_session_id = ?, result_text = ?, review_result = ?, review_comment = ?, updated_at = ? WHERE id = ?`, task.TeamID, task.Title, task.Goal, task.Status, task.ManagerAgentID, task.SummaryAgentID, task.SourceType, task.SourceChannelType, task.SourceChannelID, task.SourceThreadID, task.SourceMessageID, task.SourceRefID, task.DeliveryType, task.DeliveryChannelType, task.DeliveryChannelID, task.DeliveryThreadID, task.DeliveryTargetID, task.PanelSessionID, task.ResultText, task.ReviewResult, task.ReviewComment, task.UpdatedAt, task.ID)
	return err
}

func ListCompanyTasks(db *sql.DB, limit int) ([]CompanyTask, error) {
	query := `SELECT id, team_id, title, goal, status, manager_agent_id, summary_agent_id, source_type, source_ref_id, delivery_type, delivery_target_id, panel_session_id, result_text, review_result, review_comment, created_at, updated_at FROM company_tasks ORDER BY updated_at DESC, created_at DESC`
	args := []interface{}{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.Query(strings.Replace(query, `SELECT id, team_id, title, goal, status, manager_agent_id, summary_agent_id, source_type, source_ref_id, delivery_type, delivery_target_id, panel_session_id, result_text, review_result, review_comment, created_at, updated_at`, `SELECT id, team_id, title, goal, status, manager_agent_id, summary_agent_id, source_type, source_channel_type, source_channel_id, source_thread_id, source_message_id, source_ref_id, delivery_type, delivery_channel_type, delivery_channel_id, delivery_thread_id, delivery_target_id, panel_session_id, result_text, review_result, review_comment, created_at, updated_at`, 1), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CompanyTask, 0)
	for rows.Next() {
		var item CompanyTask
		if err := rows.Scan(&item.ID, &item.TeamID, &item.Title, &item.Goal, &item.Status, &item.ManagerAgentID, &item.SummaryAgentID, &item.SourceType, &item.SourceChannelType, &item.SourceChannelID, &item.SourceThreadID, &item.SourceMessageID, &item.SourceRefID, &item.DeliveryType, &item.DeliveryChannelType, &item.DeliveryChannelID, &item.DeliveryThreadID, &item.DeliveryTargetID, &item.PanelSessionID, &item.ResultText, &item.ReviewResult, &item.ReviewComment, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func GetCompanyTask(db *sql.DB, id string) (*CompanyTask, error) {
	var item CompanyTask
	err := db.QueryRow(`SELECT id, team_id, title, goal, status, manager_agent_id, summary_agent_id, source_type, source_channel_type, source_channel_id, source_thread_id, source_message_id, source_ref_id, delivery_type, delivery_channel_type, delivery_channel_id, delivery_thread_id, delivery_target_id, panel_session_id, result_text, review_result, review_comment, created_at, updated_at FROM company_tasks WHERE id = ?`, id).Scan(&item.ID, &item.TeamID, &item.Title, &item.Goal, &item.Status, &item.ManagerAgentID, &item.SummaryAgentID, &item.SourceType, &item.SourceChannelType, &item.SourceChannelID, &item.SourceThreadID, &item.SourceMessageID, &item.SourceRefID, &item.DeliveryType, &item.DeliveryChannelType, &item.DeliveryChannelID, &item.DeliveryThreadID, &item.DeliveryTargetID, &item.PanelSessionID, &item.ResultText, &item.ReviewResult, &item.ReviewComment, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func ReplaceCompanyTaskSteps(db *sql.DB, taskID string, steps []CompanyTaskStep) error {
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM company_task_steps WHERE task_id = ?`, taskID); err != nil {
		return err
	}
	for i, step := range steps {
		if step.CreatedAt == 0 {
			step.CreatedAt = now
		}
		step.UpdatedAt = now
		inputJSON, _ := json.Marshal(step.Input)
		if _, err := tx.Exec(`INSERT INTO company_task_steps (id, task_id, step_key, title, instruction, worker_agent_id, status, order_index, input_json, output_text, error_text, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, step.ID, taskID, step.StepKey, step.Title, step.Instruction, step.WorkerAgentID, step.Status, i, string(inputJSON), step.OutputText, step.ErrorText, step.CreatedAt, step.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func UpdateCompanyTaskStep(db *sql.DB, step *CompanyTaskStep) error {
	step.UpdatedAt = time.Now().UnixMilli()
	inputJSON, _ := json.Marshal(step.Input)
	_, err := db.Exec(`UPDATE company_task_steps SET title = ?, instruction = ?, worker_agent_id = ?, status = ?, input_json = ?, output_text = ?, error_text = ?, updated_at = ? WHERE id = ?`, step.Title, step.Instruction, step.WorkerAgentID, step.Status, string(inputJSON), step.OutputText, step.ErrorText, step.UpdatedAt, step.ID)
	return err
}

func ListCompanyTaskSteps(db *sql.DB, taskID string) ([]CompanyTaskStep, error) {
	rows, err := db.Query(`SELECT id, task_id, step_key, title, instruction, worker_agent_id, status, order_index, input_json, output_text, error_text, created_at, updated_at FROM company_task_steps WHERE task_id = ? ORDER BY order_index ASC, created_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CompanyTaskStep, 0)
	for rows.Next() {
		var item CompanyTaskStep
		var inputJSON string
		if err := rows.Scan(&item.ID, &item.TaskID, &item.StepKey, &item.Title, &item.Instruction, &item.WorkerAgentID, &item.Status, &item.OrderIndex, &inputJSON, &item.OutputText, &item.ErrorText, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(inputJSON), &item.Input)
		items = append(items, item)
	}
	return items, nil
}

func AddCompanyEvent(db *sql.DB, event *CompanyEvent) error {
	if event.CreatedAt == 0 {
		event.CreatedAt = time.Now().UnixMilli()
	}
	payloadJSON, _ := json.Marshal(event.Payload)
	_, err := db.Exec(`INSERT INTO company_execution_events (task_id, step_id, event_type, message, payload_json, created_at) VALUES (?, ?, ?, ?, ?, ?)`, event.TaskID, event.StepID, event.EventType, event.Message, string(payloadJSON), event.CreatedAt)
	return err
}

func ListCompanyEvents(db *sql.DB, taskID string) ([]CompanyEvent, error) {
	rows, err := db.Query(`SELECT id, task_id, step_id, event_type, message, payload_json, created_at FROM company_execution_events WHERE task_id = ? ORDER BY created_at ASC, id ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CompanyEvent, 0)
	for rows.Next() {
		var item CompanyEvent
		var payloadJSON string
		if err := rows.Scan(&item.ID, &item.TaskID, &item.StepID, &item.EventType, &item.Message, &payloadJSON, &item.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(payloadJSON), &item.Payload)
		items = append(items, item)
	}
	return items, nil
}
