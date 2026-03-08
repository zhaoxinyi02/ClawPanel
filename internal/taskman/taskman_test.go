package taskman

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

func newTestManager() *Manager {
	return NewManager(websocket.NewHub())
}

func TestCreateTaskStoresPendingTask(t *testing.T) {
	manager := newTestManager()

	task := manager.CreateTask("Install OpenClaw", "install_openclaw")
	if task == nil {
		t.Fatal("CreateTask() returned nil")
	}
	if task.Status != StatusPending {
		t.Fatalf("status = %q, want %q", task.Status, StatusPending)
	}
	if got := manager.GetTask(task.ID); got != task {
		t.Fatal("GetTask() did not return stored task")
	}
}

func TestGetRecentTasksOrdersNewestFirstAndLimitsToFifty(t *testing.T) {
	manager := newTestManager()
	now := time.Now()

	for i := 0; i < 55; i++ {
		id := "task-" + time.Unix(0, int64(i)).Format("150405.000000000")
		manager.tasks[id] = &Task{
			ID:        id,
			Name:      id,
			Type:      "test",
			Status:    StatusPending,
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
			UpdatedAt: now.Add(time.Duration(i) * time.Minute),
			Log:       []string{},
		}
	}

	recent := manager.GetRecentTasks()
	if len(recent) != 50 {
		t.Fatalf("recent task count = %d, want 50", len(recent))
	}
	for i := 1; i < len(recent); i++ {
		if recent[i].CreatedAt.After(recent[i-1].CreatedAt) {
			t.Fatalf("tasks not sorted newest-first at index %d", i)
		}
	}
}

func TestHasRunningTaskMatchesTaskType(t *testing.T) {
	manager := newTestManager()
	manager.tasks["running"] = &Task{ID: "running", Type: "install", Status: StatusRunning}
	manager.tasks["done"] = &Task{ID: "done", Type: "install", Status: StatusSuccess}

	if !manager.HasRunningTask("install") {
		t.Fatal("HasRunningTask() should detect running task of matching type")
	}
	if manager.HasRunningTask("other") {
		t.Fatal("HasRunningTask() should ignore non-matching task types")
	}
}

func TestTaskMutationHelpers(t *testing.T) {
	task := &Task{Log: []string{}}

	task.AppendLog("hello")
	task.SetProgress(42)
	task.SetStatus(StatusRunning)

	if len(task.Log) != 1 || task.Log[0] != "hello" {
		t.Fatalf("unexpected logs: %+v", task.Log)
	}
	if task.Progress != 42 {
		t.Fatalf("progress = %d, want 42", task.Progress)
	}
	if task.Status != StatusRunning {
		t.Fatalf("status = %q, want %q", task.Status, StatusRunning)
	}
}

func TestDedupeEnvKeepsLastValueForDuplicateKeys(t *testing.T) {
	env := []string{
		"HOME=/tmp/one",
		"PATH=/bin",
		"HOME=/tmp/two",
		"PLAIN",
	}

	deduped := dedupeEnv(env)
	if len(deduped) != 3 {
		t.Fatalf("dedupeEnv() len = %d, want 3", len(deduped))
	}
	if deduped[0] != "HOME=/tmp/two" {
		t.Fatalf("HOME entry = %q, want last value", deduped[0])
	}
	if deduped[1] != "PATH=/bin" {
		t.Fatalf("PATH entry = %q, want PATH=/bin", deduped[1])
	}
	if deduped[2] != "PLAIN" {
		t.Fatalf("plain entry = %q, want PLAIN", deduped[2])
	}
}

func TestFinishTaskSuccessUpdatesTaskState(t *testing.T) {
	manager := newTestManager()
	task := manager.CreateTask("Task", "type")

	manager.FinishTask(task, nil)

	if task.Status != StatusSuccess {
		t.Fatalf("status = %q, want %q", task.Status, StatusSuccess)
	}
	if task.Progress != 100 {
		t.Fatalf("progress = %d, want 100", task.Progress)
	}
	if len(task.Log) == 0 || task.Log[len(task.Log)-1] != "✅ 完成" {
		t.Fatalf("unexpected logs: %+v", task.Log)
	}
}

func TestFinishTaskFailureCapturesError(t *testing.T) {
	manager := newTestManager()
	task := manager.CreateTask("Task", "type")

	manager.FinishTask(task, errors.New("boom"))

	if task.Status != StatusFailed {
		t.Fatalf("status = %q, want %q", task.Status, StatusFailed)
	}
	if task.Error != "boom" {
		t.Fatalf("error = %q, want boom", task.Error)
	}
	if len(task.Log) == 0 || !strings.Contains(task.Log[len(task.Log)-1], "boom") {
		t.Fatalf("unexpected logs: %+v", task.Log)
	}
}
