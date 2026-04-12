package handler

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

const (
	openClawTaskRecentWindow = 5 * time.Minute
)

var openClawTaskActiveStatuses = map[string]bool{
	"queued":  true,
	"running": true,
}

var openClawTaskFailureStatuses = map[string]bool{
	"failed":    true,
	"timed_out": true,
	"lost":      true,
}

type openClawTaskRecord struct {
	TaskID          string `json:"taskId"`
	Runtime         string `json:"runtime"`
	SourceID        string `json:"sourceId,omitempty"`
	OwnerKey        string `json:"ownerKey"`
	ScopeKind       string `json:"scopeKind"`
	ChildSessionKey string `json:"childSessionKey,omitempty"`
	ParentTaskID    string `json:"parentTaskId,omitempty"`
	AgentID         string `json:"agentId,omitempty"`
	RunID           string `json:"runId,omitempty"`
	Label           string `json:"label,omitempty"`
	Task            string `json:"task"`
	Status          string `json:"status"`
	DeliveryStatus  string `json:"deliveryStatus"`
	NotifyPolicy    string `json:"notifyPolicy"`
	CreatedAt       int64  `json:"createdAt"`
	StartedAt       *int64 `json:"startedAt,omitempty"`
	EndedAt         *int64 `json:"endedAt,omitempty"`
	LastEventAt     *int64 `json:"lastEventAt,omitempty"`
	CleanupAfter    *int64 `json:"cleanupAfter,omitempty"`
	Error           string `json:"error,omitempty"`
	ProgressSummary string `json:"progressSummary,omitempty"`
	TerminalSummary string `json:"terminalSummary,omitempty"`
	TerminalOutcome string `json:"terminalOutcome,omitempty"`
	ParentFlowID    string `json:"parentFlowId,omitempty"`
}

type openClawTaskPressureSummary struct {
	Total     int                 `json:"total"`
	Active    int                 `json:"active"`
	Failures  int                 `json:"failures"`
	Visible   int                 `json:"visible"`
	ByStatus  map[string]int      `json:"byStatus"`
	ByRuntime map[string]int      `json:"byRuntime"`
	FocusTask *openClawTaskRecord `json:"focusTask,omitempty"`
	UpdatedAt int64               `json:"updatedAt"`
}

func openClawTaskDBPath(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return filepath.Join(cfg.OpenClawDir, "tasks", "runs.sqlite")
}

func openClawTaskReferenceAt(task openClawTaskRecord) int64 {
	if openClawTaskActiveStatuses[task.Status] {
		if task.LastEventAt != nil && *task.LastEventAt > 0 {
			return *task.LastEventAt
		}
		if task.StartedAt != nil && *task.StartedAt > 0 {
			return *task.StartedAt
		}
		return task.CreatedAt
	}
	if task.EndedAt != nil && *task.EndedAt > 0 {
		return *task.EndedAt
	}
	if task.LastEventAt != nil && *task.LastEventAt > 0 {
		return *task.LastEventAt
	}
	if task.StartedAt != nil && *task.StartedAt > 0 {
		return *task.StartedAt
	}
	return task.CreatedAt
}

func isOpenClawTaskExpired(task openClawTaskRecord, now time.Time) bool {
	return task.CleanupAfter != nil && *task.CleanupAfter > 0 && *task.CleanupAfter <= now.UnixMilli()
}

func isOpenClawTaskRecentTerminal(task openClawTaskRecord, now time.Time) bool {
	if openClawTaskActiveStatuses[task.Status] {
		return false
	}
	referenceAt := openClawTaskReferenceAt(task)
	if referenceAt <= 0 {
		return false
	}
	return now.UnixMilli()-referenceAt <= openClawTaskRecentWindow.Milliseconds()
}

func sortOpenClawTasksByReferenceDesc(tasks []openClawTaskRecord) {
	sort.Slice(tasks, func(i, j int) bool {
		left := openClawTaskReferenceAt(tasks[i])
		right := openClawTaskReferenceAt(tasks[j])
		if left == right {
			return tasks[i].CreatedAt > tasks[j].CreatedAt
		}
		return left > right
	})
}

func taskStatusTitle(task openClawTaskRecord) string {
	if strings.TrimSpace(task.Label) != "" {
		return strings.TrimSpace(task.Label)
	}
	if strings.TrimSpace(task.Task) != "" {
		return strings.TrimSpace(task.Task)
	}
	return "Background task"
}

func taskStatusDetail(task openClawTaskRecord) string {
	if openClawTaskActiveStatuses[task.Status] {
		return strings.TrimSpace(task.ProgressSummary)
	}
	if strings.TrimSpace(task.Error) != "" {
		return strings.TrimSpace(task.Error)
	}
	return strings.TrimSpace(task.TerminalSummary)
}

func summarizeOpenClawTasks(tasks []openClawTaskRecord, now time.Time) openClawTaskPressureSummary {
	summary := openClawTaskPressureSummary{
		ByStatus: map[string]int{
			"queued":    0,
			"running":   0,
			"succeeded": 0,
			"failed":    0,
			"timed_out": 0,
			"cancelled": 0,
			"lost":      0,
		},
		ByRuntime: map[string]int{
			"subagent": 0,
			"acp":      0,
			"cli":      0,
			"cron":     0,
		},
		UpdatedAt: now.UnixMilli(),
	}

	visibleCandidates := make([]openClawTaskRecord, 0, len(tasks))
	active := make([]openClawTaskRecord, 0, len(tasks))
	recentTerminal := make([]openClawTaskRecord, 0, len(tasks))
	for _, task := range tasks {
		if isOpenClawTaskExpired(task, now) {
			continue
		}
		visibleCandidates = append(visibleCandidates, task)
		summary.Total++
		summary.ByStatus[task.Status]++
		if _, ok := summary.ByRuntime[task.Runtime]; ok {
			summary.ByRuntime[task.Runtime]++
		} else {
			summary.ByRuntime[task.Runtime] = 1
		}
		if openClawTaskActiveStatuses[task.Status] {
			summary.Active++
			active = append(active, task)
		} else if isOpenClawTaskRecentTerminal(task, now) {
			recentTerminal = append(recentTerminal, task)
		}
	}
	sortOpenClawTasksByReferenceDesc(active)
	sortOpenClawTasksByReferenceDesc(recentTerminal)
	visible := recentTerminal
	if len(active) > 0 {
		visible = append(append([]openClawTaskRecord{}, active...), recentTerminal...)
	}
	summary.Visible = len(visible)
	if len(active) == 0 {
		for _, task := range recentTerminal {
			if openClawTaskFailureStatuses[task.Status] {
				summary.Failures++
			}
		}
	}
	if len(visible) > 0 {
		focus := visible[0]
		for _, task := range visible {
			if openClawTaskFailureStatuses[task.Status] {
				focus = task
				break
			}
		}
		summary.FocusTask = &focus
	}
	return summary
}

func readOpenClawTasks(cfg *config.Config, limit int) ([]openClawTaskRecord, error) {
	dbPath := openClawTaskDBPath(cfg)
	if strings.TrimSpace(dbPath) == "" {
		return []openClawTaskRecord{}, nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		return []openClawTaskRecord{}, nil
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	query := `
SELECT
  task_id, runtime, source_id, owner_key, scope_kind, child_session_key, parent_task_id,
  agent_id, run_id, label, task, status, delivery_status, notify_policy, created_at,
  started_at, ended_at, last_event_at, cleanup_after, error, progress_summary,
  terminal_summary, terminal_outcome, parent_flow_id
FROM task_runs
ORDER BY created_at DESC
`
	if limit > 0 {
		query += " LIMIT ?"
	}
	var rows *sql.Rows
	if limit > 0 {
		rows, err = db.Query(query, limit)
	} else {
		rows, err = db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]openClawTaskRecord, 0)
	for rows.Next() {
		var task openClawTaskRecord
		var startedAt, endedAt, lastEventAt, cleanupAfter sql.NullInt64
		var sourceID, childSessionKey, parentTaskID, agentID, runID, label sql.NullString
		var errorText, progressSummary, terminalSummary, terminalOutcome, parentFlowID sql.NullString
		if err := rows.Scan(
			&task.TaskID,
			&task.Runtime,
			&sourceID,
			&task.OwnerKey,
			&task.ScopeKind,
			&childSessionKey,
			&parentTaskID,
			&agentID,
			&runID,
			&label,
			&task.Task,
			&task.Status,
			&task.DeliveryStatus,
			&task.NotifyPolicy,
			&task.CreatedAt,
			&startedAt,
			&endedAt,
			&lastEventAt,
			&cleanupAfter,
			&errorText,
			&progressSummary,
			&terminalSummary,
			&terminalOutcome,
			&parentFlowID,
		); err != nil {
			return nil, err
		}
		task.SourceID = strings.TrimSpace(sourceID.String)
		task.ChildSessionKey = strings.TrimSpace(childSessionKey.String)
		task.ParentTaskID = strings.TrimSpace(parentTaskID.String)
		task.AgentID = strings.TrimSpace(agentID.String)
		task.RunID = strings.TrimSpace(runID.String)
		task.Label = strings.TrimSpace(label.String)
		task.Error = strings.TrimSpace(errorText.String)
		task.ProgressSummary = strings.TrimSpace(progressSummary.String)
		task.TerminalSummary = strings.TrimSpace(terminalSummary.String)
		task.TerminalOutcome = strings.TrimSpace(terminalOutcome.String)
		task.ParentFlowID = strings.TrimSpace(parentFlowID.String)
		if startedAt.Valid {
			task.StartedAt = &startedAt.Int64
		}
		if endedAt.Valid {
			task.EndedAt = &endedAt.Int64
		}
		if lastEventAt.Valid {
			task.LastEventAt = &lastEventAt.Int64
		}
		if cleanupAfter.Valid {
			task.CleanupAfter = &cleanupAfter.Int64
		}
		records = append(records, task)
	}
	return records, rows.Err()
}

func GetOpenClawTasks(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		records, err := readOpenClawTasks(cfg, 200)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		now := time.Now()
		summary := summarizeOpenClawTasks(records, now)
		visible := make([]openClawTaskRecord, 0, len(records))
		active := make([]openClawTaskRecord, 0, len(records))
		recentTerminal := make([]openClawTaskRecord, 0, len(records))
		for _, task := range records {
			if isOpenClawTaskExpired(task, now) {
				continue
			}
			if openClawTaskActiveStatuses[task.Status] {
				active = append(active, task)
			} else if isOpenClawTaskRecentTerminal(task, now) {
				recentTerminal = append(recentTerminal, task)
			}
		}
		sortOpenClawTasksByReferenceDesc(active)
		sortOpenClawTasksByReferenceDesc(recentTerminal)
		if len(active) > 0 {
			visible = append(visible, active...)
		}
		visible = append(visible, recentTerminal...)
		c.JSON(http.StatusOK, gin.H{
			"ok":           true,
			"tasks":        visible,
			"allTasks":     records,
			"taskPressure": summary,
		})
	}
}

func GetOpenClawTaskDetail(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := strings.TrimSpace(c.Param("id"))
		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "task id required"})
			return
		}
		records, err := readOpenClawTasks(cfg, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		for _, task := range records {
			if task.TaskID == taskID {
				c.JSON(http.StatusOK, gin.H{
					"ok":     true,
					"task":   task,
					"title":  taskStatusTitle(task),
					"detail": taskStatusDetail(task),
				})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "task not found"})
	}
}
