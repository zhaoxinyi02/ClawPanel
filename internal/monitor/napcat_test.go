package monitor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

func newTestMonitor() *NapCatMonitor {
	return NewNapCatMonitor(&config.Config{DataDir: os.TempDir()}, websocket.NewHub(), nil)
}

func TestNewNapCatMonitorDefaults(t *testing.T) {
	monitor := newTestMonitor()
	status := monitor.GetStatus()

	if status.Status != "offline" {
		t.Fatalf("status = %q, want offline", status.Status)
	}
	if !status.AutoReconnect {
		t.Fatal("AutoReconnect should default to true")
	}
	if status.MaxReconnect != 10 {
		t.Fatalf("MaxReconnect = %d, want 10", status.MaxReconnect)
	}
	if monitor.maxLogs != 100 {
		t.Fatalf("maxLogs = %d, want 100", monitor.maxLogs)
	}
	if monitor.checkInterval != 30*time.Second {
		t.Fatalf("checkInterval = %v, want 30s", monitor.checkInterval)
	}
}

func TestPauseResumeAndSetters(t *testing.T) {
	monitor := newTestMonitor()
	monitor.SetAutoReconnect(false)
	monitor.SetMaxReconnect(3)

	monitor.mu.Lock()
	monitor.offlineCount = 2
	monitor.loginFailCount = 4
	monitor.status.ReconnectCount = 5
	monitor.mu.Unlock()

	monitor.Pause()
	if !monitor.IsPaused() {
		t.Fatal("Pause() should mark monitor paused")
	}

	monitor.Resume()
	if monitor.IsPaused() {
		t.Fatal("Resume() should clear paused state")
	}
	status := monitor.GetStatus()
	if status.ReconnectCount != 0 {
		t.Fatalf("ReconnectCount = %d, want 0 after Resume()", status.ReconnectCount)
	}
	if status.AutoReconnect {
		t.Fatal("SetAutoReconnect(false) should persist")
	}
	if status.MaxReconnect != 3 {
		t.Fatalf("MaxReconnect = %d, want 3", status.MaxReconnect)
	}
	if monitor.offlineCount != 0 || monitor.loginFailCount != 0 {
		t.Fatalf("resume should reset counters, offline=%d loginFail=%d", monitor.offlineCount, monitor.loginFailCount)
	}
}

func TestAddLogKeepsMostRecentEntriesAndGetLogsReturnsCopy(t *testing.T) {
	monitor := newTestMonitor()
	monitor.maxLogs = 2

	monitor.addLog(ReconnectLog{Reason: "first"})
	monitor.addLog(ReconnectLog{Reason: "second"})
	monitor.addLog(ReconnectLog{Reason: "third"})

	logs := monitor.GetLogs()
	if len(logs) != 2 {
		t.Fatalf("log count = %d, want 2", len(logs))
	}
	if logs[0].Reason != "second" || logs[1].Reason != "third" {
		t.Fatalf("unexpected log rotation result: %+v", logs)
	}

	logs[0].Reason = "mutated"
	fresh := monitor.GetLogs()
	if fresh[0].Reason != "second" {
		t.Fatalf("GetLogs() should return a copy, got %+v", fresh)
	}
}

func TestDecodeUTF16LE(t *testing.T) {
	want := "你好😀"
	u16 := utf16.Encode([]rune(want))
	data := []byte{0xFF, 0xFE}
	for _, value := range u16 {
		data = append(data, byte(value), byte(value>>8))
	}
	got := decodeUTF16LE(data)
	if got != want {
		t.Fatalf("decodeUTF16LE() = %q, want %q", got, want)
	}
}

func TestFindNapCatInnerDirAndReadTokenFromLogs(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "versions", "v1", "napcat")
	logsDir := filepath.Join(inner, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(inner, "napcat.mjs"), []byte("export {}"), 0644); err != nil {
		t.Fatalf("WriteFile(napcat.mjs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(logsDir, "20240102.log"), []byte("[WebUi] WebUi Token: live-token\n"), 0644); err != nil {
		t.Fatalf("WriteFile(log) error = %v", err)
	}

	if got := findNapCatInnerDir(root); got != inner {
		t.Fatalf("findNapCatInnerDir() = %q, want %q", got, inner)
	}
	if got := readTokenFromNapCatLogs(inner); got != "live-token" {
		t.Fatalf("readTokenFromNapCatLogs() = %q, want live-token", got)
	}
}

func TestNapCatBoolAcceptsStringAndNumber(t *testing.T) {
	cases := []struct {
		name  string
		input interface{}
		want  bool
	}{
		{name: "bool true", input: true, want: true},
		{name: "string true", input: "true", want: true},
		{name: "string one", input: "1", want: true},
		{name: "string online", input: "online", want: true},
		{name: "number one", input: float64(1), want: true},
		{name: "zero", input: 0, want: false},
	}
	for _, tc := range cases {
		if got := napCatBool(tc.input); got != tc.want {
			t.Fatalf("%s: napCatBool(%v) = %v, want %v", tc.name, tc.input, got, tc.want)
		}
	}
}

func TestNapCatStringNormalizesNumericIDs(t *testing.T) {
	if got := napCatString(float64(123456)); got != "123456" {
		t.Fatalf("napCatString(float64) = %q, want 123456", got)
	}
	if got := napCatString("  abc  "); got != "abc" {
		t.Fatalf("napCatString(string) = %q, want abc", got)
	}
}
