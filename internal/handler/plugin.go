package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/plugin"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
	"github.com/zhaoxinyi02/ClawPanel/internal/taskman"
)

// GetPluginList returns all installed plugins + registry plugins
func GetPluginList(pm *plugin.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		installed := pm.ListInstalled()
		reg := pm.GetRegistry()
		if len(reg.Plugins) == 0 {
			if fetched, err := pm.FetchRegistry(); err == nil && fetched != nil {
				reg = fetched
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":        true,
			"installed": installed,
			"registry":  reg.Plugins,
		})
	}
}

// GetInstalledPlugins returns only installed plugins
func GetInstalledPlugins(pm *plugin.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"plugins": pm.ListInstalled(),
		})
	}
}

// GetPluginDetail returns a single plugin's details
func GetPluginDetail(pm *plugin.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		p := pm.GetPlugin(id)
		if p == nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "插件未安装"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "plugin": p})
	}
}

// RefreshPluginRegistry fetches the latest registry
func RefreshPluginRegistry(pm *plugin.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		reg, err := pm.FetchRegistry()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":       true,
			"registry": reg,
		})
	}
}

// InstallPlugin installs a plugin from registry or custom URL
func InstallPlugin(pm *plugin.Manager, tm *taskman.Manager, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			PluginID string `json:"pluginId"`
			Source   string `json:"source,omitempty"` // custom git/archive URL
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.PluginID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "pluginId required"})
			return
		}

		conflicts := pm.CheckConflicts(req.PluginID)
		if len(conflicts) > 0 {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": conflicts[0], "conflicts": conflicts})
			return
		}
		if tm == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "任务管理器未初始化"})
			return
		}
		if tm.HasRunningTask("install_plugin_" + req.PluginID) {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "该插件已有安装任务正在进行中"})
			return
		}

		task := tm.CreateTask("安装插件 "+req.PluginID, "install_plugin_"+req.PluginID)
		tm.StartTask(task)
		wasRunning := procMgr != nil && (procMgr.GetStatus().Running || procMgr.GatewayListening())
		go func() {
			task.AppendLog("🚀 开始安装插件 " + req.PluginID)
			err := pm.InstallWithProgress(req.PluginID, req.Source, task.AppendLog)
			if err == nil {
				err = restoreOpenClawAfterPluginMutation(procMgr, wasRunning, task.AppendLog)
			}
			tm.FinishTask(task, err)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID, "message": "插件安装任务已创建，请在消息中心查看进度"})
	}
}

// UninstallPlugin removes a plugin
func UninstallPlugin(pm *plugin.Manager, tm *taskman.Manager, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cleanupConfig := true
		if raw := strings.TrimSpace(c.Query("cleanupConfig")); raw != "" {
			cleanupConfig = raw != "false" && raw != "0"
		}
		if tm == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "任务管理器未初始化"})
			return
		}
		if tm.HasRunningTask("uninstall_plugin_" + id) {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "该插件已有卸载任务正在进行中"})
			return
		}

		task := tm.CreateTask("卸载插件 "+id, "uninstall_plugin_"+id)
		tm.StartTask(task)
		wasRunning := procMgr != nil && (procMgr.GetStatus().Running || procMgr.GatewayListening())
		go func() {
			if cleanupConfig {
				task.AppendLog("🧹 卸载后将一并清理对应通道配置")
			} else {
				task.AppendLog("📦 卸载后将保留通道配置")
			}
			err := pm.UninstallWithProgress(id, cleanupConfig, task.AppendLog)
			if err == nil {
				err = restoreOpenClawAfterPluginMutation(procMgr, wasRunning, task.AppendLog)
			}
			tm.FinishTask(task, err)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID, "message": "插件卸载任务已创建，请在消息中心查看进度"})
	}
}

// TogglePlugin enables or disables a plugin
func TogglePlugin(pm *plugin.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		var err error
		if req.Enabled {
			err = pm.Enable(id)
		} else {
			err = pm.Disable(id)
		}
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetPluginConfig returns a plugin's configuration and schema
func GetPluginConfig(pm *plugin.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cfg, schema, err := pm.GetConfig(id)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":     true,
			"config": cfg,
			"schema": schema,
		})
	}
}

// UpdatePluginConfig updates a plugin's configuration
func UpdatePluginConfig(pm *plugin.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var cfg map[string]interface{}
		if err := c.ShouldBindJSON(&cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		if err := pm.UpdateConfig(id, cfg); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetPluginLogs returns a plugin's log output
func GetPluginLogs(pm *plugin.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		logs, err := pm.GetPluginLogs(id)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "logs": logs})
	}
}

// UpdatePlugin updates a plugin to the latest version
func UpdatePluginVersion(pm *plugin.Manager, tm *taskman.Manager, procMgr *process.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if tm == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "任务管理器未初始化"})
			return
		}
		if tm.HasRunningTask("update_plugin_" + id) {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "该插件已有更新任务正在进行中"})
			return
		}

		task := tm.CreateTask("更新插件 "+id, "update_plugin_"+id)
		tm.StartTask(task)
		wasRunning := procMgr != nil && (procMgr.GetStatus().Running || procMgr.GatewayListening())
		go func() {
			task.AppendLog("🔄 开始更新插件 " + id)
			err := pm.Update(id)
			if err == nil {
				err = restoreOpenClawAfterPluginMutation(procMgr, wasRunning, task.AppendLog)
			}
			tm.FinishTask(task, err)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID, "message": "插件更新任务已创建，请在消息中心查看进度"})
	}
}

func restoreOpenClawAfterPluginMutation(procMgr *process.Manager, wasRunning bool, logf func(string)) error {
	if procMgr == nil || !wasRunning {
		return nil
	}

	status := procMgr.GetStatus()
	gatewayRunning := procMgr.GatewayListening()
	if status.Running || gatewayRunning {
		if logf != nil {
			logf("✅ OpenClaw 运行环境已在线")
		}
		return nil
	}

	if logf != nil {
		logf("🔄 插件变更后正在恢复 OpenClaw 运行环境...")
	}
	if err := procMgr.Start(); err != nil {
		return err
	}
	if logf != nil {
		logf("✅ OpenClaw 运行环境已恢复")
	}
	return nil
}
