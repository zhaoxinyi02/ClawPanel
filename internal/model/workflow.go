package model

import (
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type WorkflowSettings struct {
	Enabled         bool   `json:"enabled"`
	ProviderID      string `json:"providerId"`
	ModelID         string `json:"modelId"`
	ApprovalMode    int    `json:"approvalMode"`
	ProgressMode    string `json:"progressMode"`
	Tone            string `json:"tone"`
	AutoCreateRuns  bool   `json:"autoCreateRuns"`
	PushProgress    bool   `json:"pushProgress"`
	ComplexityGuard string `json:"complexityGuard"`
}

type WorkflowTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Status      string                 `json:"status"`
	TriggerMode string                 `json:"triggerMode"`
	Settings    map[string]interface{} `json:"settings"`
	Definition  map[string]interface{} `json:"definition"`
	CreatedAt   int64                  `json:"createdAt"`
	UpdatedAt   int64                  `json:"updatedAt"`
}

type WorkflowRun struct {
	ID             string                 `json:"id"`
	ShortID        string                 `json:"shortId"`
	TemplateID     string                 `json:"templateId"`
	Name           string                 `json:"name"`
	Status         string                 `json:"status"`
	ChannelID      string                 `json:"channelId"`
	ConversationID string                 `json:"conversationId"`
	UserID         string                 `json:"userId"`
	SourceMessage  string                 `json:"sourceMessage"`
	Settings       map[string]interface{} `json:"settings"`
	Context        map[string]interface{} `json:"context"`
	LastMessage    string                 `json:"lastMessage"`
	CreatedAt      int64                  `json:"createdAt"`
	UpdatedAt      int64                  `json:"updatedAt"`
	Steps          []WorkflowStep         `json:"steps,omitempty"`
}

type WorkflowStep struct {
	ID            string                 `json:"id"`
	RunID         string                 `json:"runId"`
	StepKey       string                 `json:"stepKey"`
	Title         string                 `json:"title"`
	StepType      string                 `json:"stepType"`
	Status        string                 `json:"status"`
	OrderIndex    int                    `json:"orderIndex"`
	NeedsApproval bool                   `json:"needsApproval"`
	Input         map[string]interface{} `json:"input"`
	OutputText    string                 `json:"outputText"`
	ErrorText     string                 `json:"errorText"`
	CreatedAt     int64                  `json:"createdAt"`
	UpdatedAt     int64                  `json:"updatedAt"`
}

type WorkflowEvent struct {
	ID        int64                  `json:"id"`
	RunID     string                 `json:"runId"`
	StepID    string                 `json:"stepId"`
	EventType string                 `json:"eventType"`
	Message   string                 `json:"message"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt int64                  `json:"createdAt"`
}

func GetWorkflowSettings(db *sql.DB) (*WorkflowSettings, error) {
	value, err := GetSetting(db, "workflow.settings")
	if err != nil {
		if err == sql.ErrNoRows {
			return &WorkflowSettings{Enabled: false, ApprovalMode: 2, ProgressMode: "concise", Tone: "professional", AutoCreateRuns: true, PushProgress: false, ComplexityGuard: "balanced"}, nil
		}
		return nil, err
	}
	var settings WorkflowSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, err
	}
	if settings.ProgressMode == "" {
		settings.ProgressMode = "concise"
	}
	if settings.Tone == "" {
		settings.Tone = "professional"
	}
	return &settings, nil

}

func SaveWorkflowSettings(db *sql.DB, settings *WorkflowSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return SetSetting(db, "workflow.settings", string(data))
}

func ListWorkflowTemplates(db *sql.DB) ([]WorkflowTemplate, error) {
	rows, err := db.Query(`SELECT id, name, description, category, status, trigger_mode, settings_json, definition_json, created_at, updated_at FROM workflow_templates ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []WorkflowTemplate
	for rows.Next() {
		var item WorkflowTemplate
		var settingsJSON, definitionJSON string
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Category, &item.Status, &item.TriggerMode, &settingsJSON, &definitionJSON, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(settingsJSON), &item.Settings)
		json.Unmarshal([]byte(definitionJSON), &item.Definition)
		items = append(items, item)
	}
	if items == nil {
		items = []WorkflowTemplate{}
	}
	return items, nil
}

func GetWorkflowTemplate(db *sql.DB, id string) (*WorkflowTemplate, error) {
	var item WorkflowTemplate
	var settingsJSON, definitionJSON string
	err := db.QueryRow(`SELECT id, name, description, category, status, trigger_mode, settings_json, definition_json, created_at, updated_at FROM workflow_templates WHERE id = ?`, id).
		Scan(&item.ID, &item.Name, &item.Description, &item.Category, &item.Status, &item.TriggerMode, &settingsJSON, &definitionJSON, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(settingsJSON), &item.Settings)
	json.Unmarshal([]byte(definitionJSON), &item.Definition)
	return &item, nil
}

func UpsertWorkflowTemplate(db *sql.DB, item *WorkflowTemplate) error {
	settingsJSON, _ := json.Marshal(item.Settings)
	definitionJSON, _ := json.Marshal(item.Definition)
	if item.CreatedAt == 0 {
		item.CreatedAt = time.Now().UnixMilli()
	}
	item.UpdatedAt = time.Now().UnixMilli()
	_, err := db.Exec(`INSERT INTO workflow_templates (id, name, description, category, status, trigger_mode, settings_json, definition_json, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET name=excluded.name, description=excluded.description, category=excluded.category, status=excluded.status, trigger_mode=excluded.trigger_mode, settings_json=excluded.settings_json, definition_json=excluded.definition_json, updated_at=excluded.updated_at`,
		item.ID, item.Name, item.Description, item.Category, item.Status, item.TriggerMode, string(settingsJSON), string(definitionJSON), item.CreatedAt, item.UpdatedAt)
	return err
}

func DeleteWorkflowTemplate(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM workflow_templates WHERE id = ?`, id)
	return err
}

func ListWorkflowRuns(db *sql.DB, status string) ([]WorkflowRun, error) {
	query := `SELECT id, short_id, template_id, name, status, channel_id, conversation_id, user_id, source_message, settings_json, context_json, last_message, created_at, updated_at FROM workflow_runs`
	args := []interface{}{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []WorkflowRun
	for rows.Next() {
		item, err := scanWorkflowRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if items == nil {
		items = []WorkflowRun{}
	}
	return items, nil
}

func GetWorkflowRun(db *sql.DB, id string) (*WorkflowRun, error) {
	row := db.QueryRow(`SELECT id, short_id, template_id, name, status, channel_id, conversation_id, user_id, source_message, settings_json, context_json, last_message, created_at, updated_at FROM workflow_runs WHERE id = ?`, id)
	item, err := scanWorkflowRun(row)
	if err != nil {
		return nil, err
	}
	steps, err := ListWorkflowSteps(db, id)
	if err != nil {
		return nil, err
	}
	item.Steps = steps
	return item, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanWorkflowRun(s scanner) (*WorkflowRun, error) {
	var item WorkflowRun
	var settingsJSON, contextJSON string
	if err := s.Scan(&item.ID, &item.ShortID, &item.TemplateID, &item.Name, &item.Status, &item.ChannelID, &item.ConversationID, &item.UserID, &item.SourceMessage, &settingsJSON, &contextJSON, &item.LastMessage, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(settingsJSON), &item.Settings)
	json.Unmarshal([]byte(contextJSON), &item.Context)
	return &item, nil
}

func CreateWorkflowRun(db *sql.DB, run *WorkflowRun, steps []WorkflowStep) error {
	now := time.Now().UnixMilli()
	if run.CreatedAt == 0 {
		run.CreatedAt = now
	}
	run.UpdatedAt = now
	settingsJSON, _ := json.Marshal(run.Settings)
	contextJSON, _ := json.Marshal(run.Context)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO workflow_runs (id, short_id, template_id, name, status, channel_id, conversation_id, user_id, source_message, settings_json, context_json, last_message, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.ShortID, run.TemplateID, run.Name, run.Status, run.ChannelID, run.ConversationID, run.UserID, run.SourceMessage, string(settingsJSON), string(contextJSON), run.LastMessage, run.CreatedAt, run.UpdatedAt)
	if err != nil {
		return err
	}
	for _, step := range steps {
		inputJSON, _ := json.Marshal(step.Input)
		if step.CreatedAt == 0 {
			step.CreatedAt = now
		}
		step.UpdatedAt = now
		_, err = tx.Exec(`INSERT INTO workflow_steps (id, run_id, step_key, title, step_type, status, order_index, needs_approval, input_json, output_text, error_text, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			step.ID, step.RunID, step.StepKey, step.Title, step.StepType, step.Status, step.OrderIndex, boolToInt(step.NeedsApproval), string(inputJSON), step.OutputText, step.ErrorText, step.CreatedAt, step.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func UpdateWorkflowRun(db *sql.DB, run *WorkflowRun) error {
	run.UpdatedAt = time.Now().UnixMilli()
	settingsJSON, _ := json.Marshal(run.Settings)
	contextJSON, _ := json.Marshal(run.Context)
	_, err := db.Exec(`UPDATE workflow_runs SET status = ?, settings_json = ?, context_json = ?, last_message = ?, updated_at = ? WHERE id = ?`, run.Status, string(settingsJSON), string(contextJSON), run.LastMessage, run.UpdatedAt, run.ID)
	return err
}

func DeleteWorkflowRun(db *sql.DB, runID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM workflow_events WHERE run_id = ?`, runID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM workflow_steps WHERE run_id = ?`, runID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM workflow_runs WHERE id = ?`, runID); err != nil {
		return err
	}
	return tx.Commit()
}

func ListWorkflowSteps(db *sql.DB, runID string) ([]WorkflowStep, error) {
	rows, err := db.Query(`SELECT id, run_id, step_key, title, step_type, status, order_index, needs_approval, input_json, output_text, error_text, created_at, updated_at FROM workflow_steps WHERE run_id = ? ORDER BY order_index ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []WorkflowStep
	for rows.Next() {
		var item WorkflowStep
		var inputJSON string
		var needsApproval int
		if err := rows.Scan(&item.ID, &item.RunID, &item.StepKey, &item.Title, &item.StepType, &item.Status, &item.OrderIndex, &needsApproval, &inputJSON, &item.OutputText, &item.ErrorText, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.NeedsApproval = needsApproval == 1
		json.Unmarshal([]byte(inputJSON), &item.Input)
		items = append(items, item)
	}
	if items == nil {
		items = []WorkflowStep{}
	}
	return items, nil
}

func UpdateWorkflowStep(db *sql.DB, step *WorkflowStep) error {
	step.UpdatedAt = time.Now().UnixMilli()
	inputJSON, _ := json.Marshal(step.Input)
	_, err := db.Exec(`UPDATE workflow_steps SET status = ?, needs_approval = ?, input_json = ?, output_text = ?, error_text = ?, updated_at = ? WHERE id = ?`, step.Status, boolToInt(step.NeedsApproval), string(inputJSON), step.OutputText, step.ErrorText, step.UpdatedAt, step.ID)
	return err
}

func AddWorkflowEvent(db *sql.DB, event *WorkflowEvent) error {
	if event.CreatedAt == 0 {
		event.CreatedAt = time.Now().UnixMilli()
	}
	payloadJSON, _ := json.Marshal(event.Payload)
	_, err := db.Exec(`INSERT INTO workflow_events (run_id, step_id, event_type, message, payload_json, created_at) VALUES (?, ?, ?, ?, ?, ?)`, event.RunID, event.StepID, event.EventType, event.Message, string(payloadJSON), event.CreatedAt)
	return err
}

func ListWorkflowEvents(db *sql.DB, runID string) ([]WorkflowEvent, error) {
	rows, err := db.Query(`SELECT id, run_id, step_id, event_type, message, payload_json, created_at FROM workflow_events WHERE run_id = ? ORDER BY created_at ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []WorkflowEvent
	for rows.Next() {
		var item WorkflowEvent
		var payloadJSON string
		if err := rows.Scan(&item.ID, &item.RunID, &item.StepID, &item.EventType, &item.Message, &payloadJSON, &item.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(payloadJSON), &item.Payload)
		items = append(items, item)
	}
	if items == nil {
		items = []WorkflowEvent{}
	}
	return items, nil
}

func GenerateWorkflowShortID(id string) string {
	if id == "" {
		return "#FLOW"
	}
	sum := sha1.Sum([]byte(id))
	alphabet := []rune("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
	buf := make([]rune, 0, 4)
	for i := 0; i < 4; i++ {
		buf = append(buf, alphabet[int(sum[i])%len(alphabet)])
	}
	return "#" + string(buf)
}

func stringsToUpper(v string) string {
	buf := []rune(v)
	for i, r := range buf {
		if r >= 'a' && r <= 'z' {
			buf[i] = r - 32
		}
	}
	return string(buf)
}

func NormalizeWorkflowTemplateDefinition(def map[string]interface{}) map[string]interface{} {
	if def == nil {
		def = map[string]interface{}{}
	}
	switch nodes := def["nodes"].(type) {
	case []interface{}:
		if len(nodes) == 0 {
			def["nodes"] = []map[string]interface{}{}
		}
	case []map[string]interface{}:
		if len(nodes) == 0 {
			def["nodes"] = []map[string]interface{}{}
		}
	default:
		def["nodes"] = []map[string]interface{}{}
	}
	switch edges := def["edges"].(type) {
	case []interface{}:
		if len(edges) == 0 {
			def["edges"] = []map[string]interface{}{}
		}
	case []map[string]interface{}:
		if len(edges) == 0 {
			def["edges"] = []map[string]interface{}{}
		}
	default:
		def["edges"] = []map[string]interface{}{}
	}
	return def
}

func ExtractWorkflowNodes(def map[string]interface{}) []map[string]interface{} {
	nodes := make([]map[string]interface{}, 0)
	switch raw := def["nodes"].(type) {
	case []interface{}:
		nodes = make([]map[string]interface{}, 0, len(raw))
		for _, item := range raw {
			if node, ok := item.(map[string]interface{}); ok {
				nodes = append(nodes, node)
			}
		}
	case []map[string]interface{}:
		nodes = append(nodes, raw...)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return toInt(nodes[i]["order"]) < toInt(nodes[j]["order"])
	})
	return nodes
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func NewWorkflowTemplateID() string { return fmt.Sprintf("wf_%d", time.Now().UnixNano()) }
func NewWorkflowRunID() string      { return fmt.Sprintf("run_%d", time.Now().UnixNano()) }
func NewWorkflowStepID() string     { return fmt.Sprintf("step_%d", time.Now().UnixNano()) }
