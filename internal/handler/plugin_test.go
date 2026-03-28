package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/plugin"
	"github.com/zhaoxinyi02/ClawPanel/internal/taskman"
	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

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
	if !strings.Contains(w.Body.String(), "任务管理器未初始化") {
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
	if !strings.Contains(w.Body.String(), "任务管理器未初始化") {
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
	if !strings.Contains(w.Body.String(), "任务管理器未初始化") {
		t.Fatalf("expected nil task manager error, got %s", w.Body.String())
	}
}

func TestInstallPluginRejectsDuplicatePendingTask(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tm := taskman.NewManager(websocket.NewHub())
	_ = tm.CreateTask("安装插件 feishu", "install_plugin_feishu")

	r := gin.New()
	r.POST("/plugins/install", InstallPlugin(&plugin.Manager{}, tm, nil))

	req := httptest.NewRequest(http.MethodPost, "/plugins/install", strings.NewReader(`{"pluginId":"feishu"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 duplicate-task response, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "该插件已有安装任务正在进行中") {
		t.Fatalf("expected duplicate install guard to trigger, got %s", w.Body.String())
	}
}
