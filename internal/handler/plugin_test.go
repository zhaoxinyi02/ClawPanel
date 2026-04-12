package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/plugin"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
	"github.com/zhaoxinyi02/ClawPanel/internal/taskman"
	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

type fakeOpenClawRuntime struct {
	status           process.Status
	gatewayListening bool
	startCalls       int
	restartCalls     int
	startErr         error
	restartErr       error
}

func (f *fakeOpenClawRuntime) GetStatus() process.Status { return f.status }
func (f *fakeOpenClawRuntime) GatewayListening() bool    { return f.gatewayListening }
func (f *fakeOpenClawRuntime) Start() error {
	f.startCalls++
	return f.startErr
}
func (f *fakeOpenClawRuntime) Restart() error {
	f.restartCalls++
	return f.restartErr
}

func TestInstallPluginRequiresTaskManager(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/plugins/install", InstallPlugin(&plugin.Manager{}, nil, nil))

	req := httptest.NewRequest(http.MethodPost, "/plugins/install", strings.NewReader(`{"pluginId":"feishu"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when task manager is nil, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "task manager not initialized") {
		t.Fatalf("expected nil task manager error, got %s", w.Body.String())
	}
}

func TestUninstallPluginRequiresTaskManager(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.DELETE("/plugins/:id", UninstallPlugin(&plugin.Manager{}, nil, nil))

	req := httptest.NewRequest(http.MethodDelete, "/plugins/feishu", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when task manager is nil, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "task manager not initialized") {
		t.Fatalf("expected nil task manager error, got %s", w.Body.String())
	}
}

func TestUpdatePluginVersionRequiresTaskManager(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/plugins/:id/update", UpdatePluginVersion(&plugin.Manager{}, nil, nil))

	req := httptest.NewRequest(http.MethodPost, "/plugins/feishu/update", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when task manager is nil, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "task manager not initialized") {
		t.Fatalf("expected nil task manager error, got %s", w.Body.String())
	}
}

func TestInstallPluginRejectsDuplicatePendingTask(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tm := taskman.NewManager(websocket.NewHub())
	_ = tm.CreateTask("Install plugin feishu", "install_plugin_feishu")

	r := gin.New()
	r.POST("/plugins/install", InstallPlugin(&plugin.Manager{}, tm, nil))

	req := httptest.NewRequest(http.MethodPost, "/plugins/install", strings.NewReader(`{"pluginId":"feishu"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 duplicate-task response, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "already running") {
		t.Fatalf("expected duplicate install guard to trigger, got %s", w.Body.String())
	}
}

func TestRestoreOpenClawAfterPluginMutationRestartsRunningRuntime(t *testing.T) {
	t.Parallel()

	rt := &fakeOpenClawRuntime{
		status: process.Status{Running: true, PID: 1234},
	}

	if err := restoreOpenClawAfterPluginMutation(rt, true, nil); err != nil {
		t.Fatalf("expected restart path to succeed, got %v", err)
	}
	if rt.restartCalls != 1 {
		t.Fatalf("expected exactly one restart call, got %d", rt.restartCalls)
	}
	if rt.startCalls != 0 {
		t.Fatalf("did not expect start call when runtime was already running, got %d", rt.startCalls)
	}
}

func TestRestoreOpenClawAfterPluginMutationStartsOfflineRuntime(t *testing.T) {
	t.Parallel()

	rt := &fakeOpenClawRuntime{}

	if err := restoreOpenClawAfterPluginMutation(rt, true, nil); err != nil {
		t.Fatalf("expected start path to succeed, got %v", err)
	}
	if rt.startCalls != 1 {
		t.Fatalf("expected exactly one start call, got %d", rt.startCalls)
	}
	if rt.restartCalls != 0 {
		t.Fatalf("did not expect restart call for offline runtime, got %d", rt.restartCalls)
	}
}

func TestRestoreOpenClawAfterPluginMutationReturnsRestartError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	rt := &fakeOpenClawRuntime{
		status:     process.Status{Running: true, PID: 1},
		restartErr: wantErr,
	}

	if err := restoreOpenClawAfterPluginMutation(rt, true, nil); !errors.Is(err, wantErr) {
		t.Fatalf("expected restart error %v, got %v", wantErr, err)
	}
}
