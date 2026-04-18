package taskman

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
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
	tasks  map[string]*Task
	hub    *websocket.Hub
	nextID uint64
	mu     sync.RWMutex
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

	m.nextID++
	id := fmt.Sprintf("task-%d", m.nextID)
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
	task := m.tasks[id]
	m.mu.RUnlock()
	return cloneTask(task)
}

// GetAllTasks 获取所有任务
func (m *Manager) GetAllTasks() []*Task {
	m.mu.RLock()
	tasks := make([]*Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	m.mu.RUnlock()

	result := make([]*Task, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, cloneTask(t))
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
		if t.Type != taskType {
			continue
		}
		t.mu.Lock()
		status := t.Status
		t.mu.Unlock()
		if status == StatusPending || status == StatusRunning {
			return true
		}
	}
	return false
}

// StartTask marks a task as running and broadcasts the state change.
func (m *Manager) StartTask(task *Task) {
	if task == nil {
		return
	}
	task.SetStatus(StatusRunning)
	m.broadcastTaskUpdate(task)
}

func cloneTask(task *Task) *Task {
	if task == nil {
		return nil
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	cp := Task{
		ID:        task.ID,
		Name:      task.Name,
		Type:      task.Type,
		Status:    task.Status,
		Progress:  task.Progress,
		Log:       append([]string(nil), task.Log...),
		Error:     task.Error,
		CreatedAt: task.CreatedAt,
		UpdatedAt: task.UpdatedAt,
	}
	return &cp
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

func (t *Task) SetFailure(message string) {
	t.mu.Lock()
	t.Status = StatusFailed
	t.Error = message
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
}

func (t *Task) SetSuccess() {
	t.mu.Lock()
	t.Status = StatusSuccess
	t.Progress = 100
	t.Error = ""
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
}

// RunCommand 运行命令并实时推送输出
func (m *Manager) RunCommand(task *Task, name string, args ...string) error {
	return m.runCommandWithInput(task, nil, name, args...)
}

func (m *Manager) runCommandWithInput(task *Task, input io.Reader, name string, args ...string) error {
	task.SetStatus(StatusRunning)
	m.broadcastTaskUpdate(task)

	cmd := exec.Command(resolveCommandPath(name), args...)
	env := config.BuildExecEnv()

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

	env = append(env,
		"DEBIAN_FRONTEND=noninteractive",
		"LANG=en_US.UTF-8",
	)

	cmd.Env = dedupeEnv(env)
	cmd.Stdin = input

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
	if err := scanner.Err(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("读取命令输出失败: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func resolveCommandPath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	if filepath.IsAbs(name) || strings.ContainsRune(name, os.PathSeparator) {
		return name
	}
	if resolved, err := exec.LookPath(name); err == nil && resolved != "" {
		return resolved
	}
	if resolved := lookPathInPath(name, config.BuildAugmentedPath(os.Getenv("PATH"))); resolved != "" {
		return resolved
	}
	return name
}

func lookPathInPath(name, pathValue string) string {
	if strings.TrimSpace(pathValue) == "" {
		return ""
	}
	exts := []string{""}
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		for _, ext := range strings.Split(os.Getenv("PATHEXT"), ";") {
			ext = strings.TrimSpace(ext)
			if ext != "" {
				exts = append(exts, ext)
			}
		}
	}
	for _, dir := range filepath.SplitList(pathValue) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		for _, ext := range exts {
			candidate := filepath.Join(dir, name) + ext
			info, err := os.Stat(candidate)
			if err != nil || info.IsDir() {
				continue
			}
			if runtime.GOOS == "windows" || info.Mode()&0o111 != 0 {
				return candidate
			}
		}
	}
	return ""
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
	if runtime.GOOS == "windows" {
		return fmt.Errorf("Windows 不支持 sudo 提权执行")
	}
	return m.runCommandWithInput(task, strings.NewReader(sudoPass+"\n"+script), "sudo", "-S", "bash", "-s")
}

// broadcastTaskUpdate 广播任务状态更新
func (m *Manager) broadcastTaskUpdate(task *Task) {
	if m == nil || m.hub == nil || task == nil {
		return
	}
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
	if m == nil || m.hub == nil || task == nil {
		return
	}
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
		task.SetFailure(err.Error())
		task.AppendLog(fmt.Sprintf("❌ 失败: %v", err))
		log.Printf("[TaskMan] 任务 %s (%s) 失败: %v", task.ID, task.Name, err)
	} else {
		task.SetSuccess()
		task.AppendLog("✅ 完成")
		log.Printf("[TaskMan] 任务 %s (%s) 完成", task.ID, task.Name)
	}
	m.broadcastTaskUpdate(task)
}
