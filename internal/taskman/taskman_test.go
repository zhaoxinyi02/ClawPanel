package taskman

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHasRunningTaskTreatsPendingAsActive(t *testing.T) {
	t.Parallel()

	m := &Manager{
		tasks: map[string]*Task{
			"task-1": {
				Type:   "install_plugin_feishu",
				Status: StatusPending,
			},
		},
	}

	if !m.HasRunningTask("install_plugin_feishu") {
		t.Fatalf("expected pending task to be treated as active")
	}
}

func TestHasRunningTaskIgnoresFinishedTasks(t *testing.T) {
	t.Parallel()

	m := &Manager{
		tasks: map[string]*Task{
			"task-1": {
				Type:   "install_plugin_feishu",
				Status: StatusSuccess,
			},
		},
	}

	if m.HasRunningTask("install_plugin_feishu") {
		t.Fatalf("expected finished task not to be treated as active")
	}
}

func TestCreateTaskGeneratesUniqueIDs(t *testing.T) {
	t.Parallel()

	m := NewManager(nil)
	first := m.CreateTask("first", "install_plugin_feishu")
	second := m.CreateTask("second", "install_plugin_feishu")

	if first.ID == second.ID {
		t.Fatalf("expected unique task ids, got %q", first.ID)
	}
}

func TestGetTaskReturnsCopy(t *testing.T) {
	t.Parallel()

	original := &Task{
		ID:     "task-1",
		Status: StatusPending,
		Log:    []string{"hello"},
	}
	m := &Manager{tasks: map[string]*Task{"task-1": original}}

	got := m.GetTask("task-1")
	got.Status = StatusFailed
	got.Log[0] = "changed"

	if original.Status != StatusPending {
		t.Fatalf("expected GetTask to return an isolated copy")
	}
	if original.Log[0] != "hello" {
		t.Fatalf("expected GetTask log copy, got %#v", original.Log)
	}
}

func TestFinishTaskSetsFailureMessage(t *testing.T) {
	t.Parallel()

	task := &Task{ID: "task-1", Name: "demo", Status: StatusRunning}
	m := NewManager(nil)
	m.FinishTask(task, errors.New("boom"))

	if task.Status != StatusFailed {
		t.Fatalf("expected failed status, got %s", task.Status)
	}
	if task.Error != "boom" {
		t.Fatalf("expected error to be recorded, got %q", task.Error)
	}
}

func TestRunScriptWithSudoPassesPasswordViaStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sudo is not supported on Windows")
	}

	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	passwordPath := filepath.Join(dir, "password.txt")
	scriptPath := filepath.Join(dir, "script.txt")
	sudoPath := filepath.Join(dir, "sudo")
	body := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\n" +
		"IFS= read -r password\n" +
		"printf '%s' \"$password\" > " + shellQuote(passwordPath) + "\n" +
		"cat > " + shellQuote(scriptPath) + "\n"
	if err := os.WriteFile(sudoPath, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake sudo: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+originalPath)

	task := &Task{ID: "task-1", Name: "sudo"}
	m := NewManager(nil)
	password := "pa'ss"
	script := "echo hello\nprintf '%s' world\n"
	if err := m.RunScriptWithSudo(task, password, script); err != nil {
		t.Fatalf("RunScriptWithSudo: %v", err)
	}

	passwordData, err := os.ReadFile(passwordPath)
	if err != nil {
		t.Fatalf("read captured password: %v", err)
	}
	if string(passwordData) != password {
		t.Fatalf("expected password via stdin, got %q", string(passwordData))
	}

	scriptData, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read captured script: %v", err)
	}
	if string(scriptData) != script {
		t.Fatalf("expected script via stdin, got %q", string(scriptData))
	}

	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read captured args: %v", err)
	}
	args := string(argsData)
	if !strings.Contains(args, "-S\nbash\n-s\n") {
		t.Fatalf("expected sudo to receive -S bash -s args, got %q", args)
	}
	if strings.Contains(args, password) {
		t.Fatalf("expected password to stay out of process args, got %q", args)
	}
}

func TestRunScriptFailsWhenOutputLineExceedsScannerBuffer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash script differs on Windows")
	}
	t.Parallel()

	task := &Task{ID: "task-1", Name: "long-output"}
	m := NewManager(nil)
	script := "i=0; while [ \"$i\" -lt 70000 ]; do printf a; i=$((i+1)); done; printf '\\n'"
	err := m.RunScript(task, script)
	if err == nil {
		t.Fatalf("expected oversized output line to return an error")
	}
	if !strings.Contains(err.Error(), "读取命令输出失败") {
		t.Fatalf("expected scanner error to be propagated, got %v", err)
	}
}

func TestRunCommandUsesAugmentedRuntimePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path layout differs on Windows")
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	scriptPath := filepath.Join(binDir, "fake-openclaw-helper")
	script := "#!/bin/sh\nprintf 'ok\\n'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	task := &Task{ID: "task-1", Name: "augmented-path"}
	m := NewManager(nil)
	if err := m.RunCommand(task, "fake-openclaw-helper"); err != nil {
		t.Fatalf("RunCommand should discover helper from augmented PATH: %v", err)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
