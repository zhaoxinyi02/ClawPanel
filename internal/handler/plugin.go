package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/plugin"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
	"github.com/zhaoxinyi02/ClawPanel/internal/taskman"
)

type openClawRuntime interface {
	GetStatus() process.Status
	GatewayListening() bool
	Start() error
	Restart() error
}

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
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "plugin not installed"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "task manager not initialized"})
			return
		}
		if tm.HasRunningTask("install_plugin_" + req.PluginID) {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "an install task for this plugin is already running"})
			return
		}

		task := tm.CreateTask("Install plugin "+req.PluginID, "install_plugin_"+req.PluginID)
		tm.StartTask(task)
		wasRunning := procMgr != nil && (procMgr.GetStatus().Running || procMgr.GatewayListening())
		go func() {
			task.AppendLog("Starting plugin install: " + req.PluginID)
			err := pm.InstallWithProgress(req.PluginID, req.Source, task.AppendLog)
			if err == nil {
				err = restoreOpenClawAfterPluginMutation(procMgr, wasRunning, task.AppendLog)
			}
			tm.FinishTask(task, err)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID, "message": "plugin install task created; check the message center for progress"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "task manager not initialized"})
			return
		}
		if tm.HasRunningTask("uninstall_plugin_" + id) {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "an uninstall task for this plugin is already running"})
			return
		}

		task := tm.CreateTask("Uninstall plugin "+id, "uninstall_plugin_"+id)
		tm.StartTask(task)
		wasRunning := procMgr != nil && (procMgr.GetStatus().Running || procMgr.GatewayListening())
		go func() {
			if cleanupConfig {
				task.AppendLog("Cleanup channel config after uninstall is enabled")
			} else {
				task.AppendLog("Channel config will be preserved after uninstall")
			}
			err := pm.UninstallWithProgress(id, cleanupConfig, task.AppendLog)
			if err == nil {
				err = restoreOpenClawAfterPluginMutation(procMgr, wasRunning, task.AppendLog)
			}
			tm.FinishTask(task, err)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID, "message": "plugin uninstall task created; check the message center for progress"})
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
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid parameters"})
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
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid parameters"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "task manager not initialized"})
			return
		}
		if tm.HasRunningTask("update_plugin_" + id) {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "an update task for this plugin is already running"})
			return
		}

		task := tm.CreateTask("Update plugin "+id, "update_plugin_"+id)
		tm.StartTask(task)
		wasRunning := procMgr != nil && (procMgr.GetStatus().Running || procMgr.GatewayListening())
		go func() {
			task.AppendLog("Starting plugin update: " + id)
			err := pm.Update(id)
			if err == nil {
				err = restoreOpenClawAfterPluginMutation(procMgr, wasRunning, task.AppendLog)
			}
			tm.FinishTask(task, err)
		}()
		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID, "message": "plugin update task created; check the message center for progress"})
	}
}

func restoreOpenClawAfterPluginMutation(procMgr openClawRuntime, wasRunning bool, logf func(string)) error {
	if procMgr == nil || !wasRunning {
		return nil
	}

	status := procMgr.GetStatus()
	gatewayRunning := procMgr.GatewayListening()
	if status.Running || gatewayRunning {
		if logf != nil {
			logf("plugin mutation complete; restarting OpenClaw to load the latest plugin")
		}
		if err := procMgr.Restart(); err != nil {
			return err
		}
		if logf != nil {
			logf("OpenClaw restarted successfully")
		}
		return nil
	}

	if logf != nil {
		logf("plugin mutation complete; restoring the OpenClaw runtime")
	}
	if err := procMgr.Start(); err != nil {
		return err
	}
	if logf != nil {
		logf("OpenClaw runtime restored successfully")
	}
	return nil
}
