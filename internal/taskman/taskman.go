package taskman

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusPending  TaskStatus = "pending"
	StatusRunning  TaskStatus = "running"
	StatusSuccess  TaskStatus = "success"
	StatusFailed   TaskStatus = "failed"
	StatusCanceled TaskStatus = "canceled"
)

// Task 安装任务
type Task struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"` // install_software, install_openclaw, install_napcat, install_wechat
	Status    TaskStatus `json:"status"`
	Progress  int        `json:"progress"` // 0-100
	Log       []string   `json:"log"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	cancel    func()
	mu        sync.Mutex
}

// Manager 任务管理器
type Manager struct {
	tasks map[string]*Task
	hub   *websocket.Hub
	mu    sync.RWMutex
}

// NewManager 创建任务管理器
func NewManager(hub *websocket.Hub) *Manager {
	return &Manager{
		tasks: make(map[string]*Task),
		hub:   hub,
	}
}

// CreateTask 创建新任务
func (m *Manager) CreateTask(name, taskType string) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("task-%d", time.Now().UnixMilli())
	task := &Task{
		ID:        id,
		Name:      name,
		Type:      taskType,
		Status:    StatusPending,
		Progress:  0,
		Log:       []string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.tasks[id] = task
	m.broadcastTaskUpdate(task)
	return task
}

// GetTask 获取任务
func (m *Manager) GetTask(id string) *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[id]
}

// GetAllTasks 获取所有任务
func (m *Manager) GetAllTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result
}

// GetRecentTasks 获取最近的任务（最多50个）
func (m *Manager) GetRecentTasks() []*Task {
	tasks := m.GetAllTasks()
	// Sort by created time desc
	for i := 0; i < len(tasks); i++ {
		for j := i + 1; j < len(tasks); j++ {
			if tasks[j].CreatedAt.After(tasks[i].CreatedAt) {
				tasks[i], tasks[j] = tasks[j], tasks[i]
			}
		}
	}
	if len(tasks) > 50 {
		tasks = tasks[:50]
	}
	return tasks
}

// HasRunningTask 检查是否有正在运行的同类型任务
func (m *Manager) HasRunningTask(taskType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tasks {
		if t.Type == taskType && t.Status == StatusRunning {
			return true
		}
	}
	return false
}

// AppendLog 追加日志
func (t *Task) AppendLog(line string) {
	t.mu.Lock()
	t.Log = append(t.Log, line)
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
}

// SetProgress 设置进度
func (t *Task) SetProgress(p int) {
	t.mu.Lock()
	t.Progress = p
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
}

// SetStatus 设置状态
func (t *Task) SetStatus(s TaskStatus) {
	t.mu.Lock()
	t.Status = s
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
}

// RunCommand 运行命令并实时推送输出
func (m *Manager) RunCommand(task *Task, name string, args ...string) error {
	task.SetStatus(StatusRunning)
	m.broadcastTaskUpdate(task)

	cmd := exec.Command(name, args...)
	env := cmd.Environ()

	if runtime.GOOS == "windows" {
		if os.Getenv("USERPROFILE") == "" {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				env = append(env, "USERPROFILE="+home)
			}
		}
	} else {
		home := os.Getenv("HOME")
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		if home == "" {
			if runtime.GOOS == "darwin" {
				home = "/var/root"
			} else {
				home = "/root"
			}
		}
		if os.Getenv("HOME") == "" && home != "" {
			env = append(env, "HOME="+home)
		}
	}

	if os.Getenv("PATH") == "" {
		if runtime.GOOS == "windows" {
			env = append(env, "PATH=C:\\Windows\\System32;C:\\Windows;C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\")
		} else {
			env = append(env, "PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin")
		}
	}

	env = append(env,
		"DEBIAN_FRONTEND=noninteractive",
		"LANG=en_US.UTF-8",
	)

	cmd.Env = dedupeEnv(env)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*64), 1024*64)
	for scanner.Scan() {
		line := scanner.Text()
		task.AppendLog(line)
		m.broadcastTaskLog(task, line)
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func dedupeEnv(env []string) []string {
	seen := make(map[string]int)
	out := make([]string, 0, len(env))

	for _, kv := range env {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			out = append(out, kv)
			continue
		}
		key := kv[:eq]
		if idx, ok := seen[key]; ok {
			out[idx] = kv
			continue
		}
		seen[key] = len(out)
		out = append(out, kv)
	}

	return out
}

// RunScript 运行脚本并实时推送输出（Windows 用 PowerShell，其他平台用 bash）
func (m *Manager) RunScript(task *Task, script string) error {
	if runtime.GOOS == "windows" {
		return m.RunCommand(task, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePowerShellCommand(strings.Join([]string{
			`[Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false)`,
			`[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)`,
			`$OutputEncoding = [System.Text.UTF8Encoding]::new($false)`,
			script,
		}, "\n")))
	}
	return m.RunCommand(task, "bash", "-c", script)
}

func encodePowerShellCommand(script string) string {
	utf16Data := utf16.Encode([]rune(script))
	bytes := make([]byte, 0, len(utf16Data)*2)
	for _, r := range utf16Data {
		bytes = append(bytes, byte(r), byte(r>>8))
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

// RunScriptWithSudo 使用 sudo 运行脚本
func (m *Manager) RunScriptWithSudo(task *Task, sudoPass, script string) error {
	// Use base64 encoding to avoid all shell escaping issues (backslash, single/double quotes, etc.)
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	fullScript := fmt.Sprintf("echo '%s' | sudo -S bash -c \"$(echo %s | base64 -d)\"", sudoPass, encoded)
	return m.RunCommand(task, "bash", "-c", fullScript)
}

// broadcastTaskUpdate 广播任务状态更新
func (m *Manager) broadcastTaskUpdate(task *Task) {
	task.mu.Lock()
	msg := map[string]interface{}{
		"type": "task_update",
		"task": map[string]interface{}{
			"id":        task.ID,
			"name":      task.Name,
			"type":      task.Type,
			"status":    task.Status,
			"progress":  task.Progress,
			"error":     task.Error,
			"createdAt": task.CreatedAt.Format(time.RFC3339),
			"updatedAt": task.UpdatedAt.Format(time.RFC3339),
			"logCount":  len(task.Log),
		},
	}
	task.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	m.hub.Broadcast(data)
}

// broadcastTaskLog 广播任务日志行
func (m *Manager) broadcastTaskLog(task *Task, line string) {
	msg := map[string]interface{}{
		"type":   "task_log",
		"taskId": task.ID,
		"line":   line,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	m.hub.Broadcast(data)
}

// FinishTask 完成任务
func (m *Manager) FinishTask(task *Task, err error) {
	if err != nil {
		task.SetStatus(StatusFailed)
		task.mu.Lock()
		task.Error = err.Error()
		task.mu.Unlock()
		task.AppendLog(fmt.Sprintf("❌ 失败: %v", err))
		log.Printf("[TaskMan] 任务 %s (%s) 失败: %v", task.ID, task.Name, err)
	} else {
		task.SetStatus(StatusSuccess)
		task.SetProgress(100)
		task.AppendLog("✅ 完成")
		log.Printf("[TaskMan] 任务 %s (%s) 完成", task.ID, task.Name)
	}
	m.broadcastTaskUpdate(task)
}
