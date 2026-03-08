package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/eventlog"
	"github.com/zhaoxinyi02/ClawPanel/internal/handler"
	"github.com/zhaoxinyi02/ClawPanel/internal/middleware"
	"github.com/zhaoxinyi02/ClawPanel/internal/model"
	"github.com/zhaoxinyi02/ClawPanel/internal/monitor"
	"github.com/zhaoxinyi02/ClawPanel/internal/plugin"
	"github.com/zhaoxinyi02/ClawPanel/internal/process"
	"github.com/zhaoxinyi02/ClawPanel/internal/taskman"
	"github.com/zhaoxinyi02/ClawPanel/internal/update"
	"github.com/zhaoxinyi02/ClawPanel/internal/updater"
	"github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

// Version is set by ldflags at build time: -X main.Version=...
var Version = "dev"

//go:embed all:frontend/dist
var frontendFS embed.FS

//go:embed docs/FAQ.md
var faqMD []byte

func main() {
	// 独立更新子进程模式：仅运行更新服务HTTP服务器
	// 由主进程 fork 出来，主进程被 systemctl stop 杀死后子进程继续存活
	if len(os.Args) >= 6 && os.Args[1] == "--updater-standalone" {
		version := os.Args[2]     // e.g. "5.0.11"
		dataDir := os.Args[3]     // e.g. "/home/xxx/ClawPanel/data"
		panelPort := os.Args[4]   // e.g. "19527"
		openClawDir := os.Args[5] // e.g. "/home/xxx/openclaw/config"
		port := 0
		fmt.Sscanf(panelPort, "%d", &port)
		if port == 0 {
			port = 19527
		}
		log.Printf("[Updater-Standalone] 独立更新子进程启动: version=%s dataDir=%s panelPort=%d openClawDir=%s", version, dataDir, port, openClawDir)
		srv := updater.NewServer(version, dataDir, openClawDir, port)
		srv.RunStandalone()
		return
	}

	// Check if running as a Windows service
	if runAsService() {
		return
	}

	// Running as a normal process — use signal-based shutdown
	stopCh := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[ClawPanel] 正在关闭...")
		close(stopCh)
	}()

	runServer(stopCh)
}

// runServer starts the ClawPanel server and blocks until stopCh is closed.
func runServer(stopCh chan struct{}) {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[ClawPanel] 配置加载失败: %v", err)
	}

	// 写入内嵌 FAQ 文档到数据目录（供 AI 助手读取）
	faqDir := filepath.Join(cfg.DataDir, "docs")
	os.MkdirAll(faqDir, 0755)
	os.WriteFile(filepath.Join(faqDir, "FAQ.md"), faqMD, 0644)

	// 初始化数据库
	db, err := model.InitDB(cfg.DataDir)
	if err != nil {
		log.Fatalf("[ClawPanel] 数据库初始化失败: %v", err)
	}
	defer db.Close()

	// 初始化进程管理器
	procMgr := process.NewManager(cfg)

	// 自动启动 OpenClaw（如果已安装且配置存在）
	if cfg.OpenClawConfigExists() {
		if procMgr.GatewayListening() {
			log.Println("[ClawPanel] 检测到 OpenClaw 网关已在运行，跳过自动启动")
		} else if err := procMgr.Start(); err != nil {
			log.Printf("[ClawPanel] OpenClaw 自动启动失败: %v", err)
		} else {
			log.Println("[ClawPanel] OpenClaw 已自动启动")
		}
	}

	// 初始化 WebSocket Hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// 初始化任务管理器
	taskMgr := taskman.NewManager(wsHub)

	// 初始化系统事件日志
	sysLog := eventlog.NewSystemLogger(db, wsHub)
	sysLog.Log("system", "panel.start", "ClawPanel 管理面板已启动")

	readQQChannelState := func() (bool, string) {
		enabled, token, err := cfg.ReadQQChannelState()
		if err != nil {
			return false, ""
		}
		return enabled, token
	}

	// 检查 QQ 通道是否启用，读取 accessToken 用于 WS 认证
	qqEnabled, _ := readQQChannelState()

	// 启动 OneBot11 事件监听器 (仅当 QQ 通道启用时)
	var evListener *eventlog.Listener
	if qqEnabled {
		evListener = eventlog.NewListener(db, wsHub, "ws://127.0.0.1:3001", func() string {
			_, token := readQQChannelState()
			return token
		})
		evListener.Start()
		defer evListener.Stop()
	}

	// 启动 NapCat 连接监控 (仅当 QQ 通道启用时)
	var napcatMon *monitor.NapCatMonitor
	if qqEnabled {
		napcatMon = monitor.NewNapCatMonitor(cfg, wsHub, sysLog)
		napcatMon.Start()
		defer napcatMon.Stop()
	} else {
		// 创建空监控器供 API 使用
		napcatMon = monitor.NewNapCatMonitor(cfg, wsHub, sysLog)
	}

	// 初始化插件管理器
	pluginMgr := plugin.NewManager(cfg)

	// 初始化面板自检更新器
	panelUpdater := update.NewUpdater(Version, cfg.DataDir)
	workflowRuntime := handler.NewWorkflowRuntime(db, cfg, wsHub)
	if err := handler.EnsureWorkflowDefaults(db); err != nil {
		log.Printf("[ClawPanel] 工作流模板初始化失败: %v", err)
	}
	if evListener != nil {
		evListener.SetInboundHandler(workflowRuntime.HandleInboundReply)
	}

	// 启动独立更新服务（进程隔离，独立端口）
	updaterSrv := updater.NewServer(Version, cfg.DataDir, cfg.OpenClawDir, cfg.Port)
	updaterSrv.Start()

	// 设置 Gin 模式
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())

	// API 路由组
	api := r.Group("/api")
	{
		// 公开路由
		api.POST("/auth/login", handler.Login(db, cfg))
		api.POST("/workflows/intercept", workflowRuntime.InterceptInbound())

		// 需要认证的路由
		auth := api.Group("")
		auth.Use(middleware.Auth(cfg))
		{
			// 认证
			auth.POST("/auth/change-password", handler.ChangePassword(db, cfg))

			// 状态总览
			auth.GET("/status", handler.GetStatus(db, cfg, procMgr, napcatMon))

			// OpenClaw 配置
			auth.GET("/openclaw/config", handler.GetOpenClawConfig(cfg))
			auth.PUT("/openclaw/config", handler.SaveOpenClawConfig(cfg))
			auth.GET("/openclaw/feishu-dm-diagnosis", handler.GetFeishuDMDiagnosis(cfg))
			auth.GET("/openclaw/agents", handler.GetOpenClawAgents(cfg))
			auth.POST("/openclaw/agents", handler.CreateOpenClawAgent(cfg))
			auth.PUT("/openclaw/agents/:id", handler.UpdateOpenClawAgent(cfg))
			auth.DELETE("/openclaw/agents/:id", handler.DeleteOpenClawAgent(cfg))
			auth.GET("/openclaw/agents/:id/core-files", handler.GetOpenClawAgentCoreFiles(cfg))
			auth.PUT("/openclaw/agents/:id/core-files", handler.SaveOpenClawAgentCoreFile(cfg))
			auth.GET("/openclaw/bindings", handler.GetOpenClawBindings(cfg))
			auth.PUT("/openclaw/bindings", handler.SaveOpenClawBindings(cfg))
			auth.POST("/openclaw/route/preview", handler.PreviewOpenClawRoute(cfg))
			auth.GET("/openclaw/models", handler.GetModels(cfg))
			auth.PUT("/openclaw/models", handler.SaveModels(cfg))
			auth.GET("/openclaw/channels", handler.GetChannels(cfg))
			auth.PUT("/openclaw/channels/:id", handler.SaveChannel(cfg, procMgr))
			auth.PUT("/openclaw/plugins/:id", handler.SavePlugin(cfg))
			auth.POST("/openclaw/toggle-channel", handler.ToggleChannel(cfg, procMgr, napcatMon, sysLog))
			auth.GET("/openclaw/qq-channel/state", handler.GetQQChannelState(cfg))
			auth.POST("/openclaw/qq-channel/setup", handler.SetupQQChannel(cfg, taskMgr, procMgr))
			auth.POST("/openclaw/qq-channel/repair", handler.RepairQQChannel(cfg, taskMgr, procMgr))
			auth.POST("/openclaw/qq-channel/cleanup", handler.CleanupQQChannel(cfg, taskMgr, procMgr))
			auth.POST("/openclaw/feishu-variant", handler.SwitchFeishuVariant(cfg, procMgr, sysLog))

			// 进程管理
			auth.POST("/process/start", handler.StartProcess(procMgr, sysLog))
			auth.POST("/process/stop", handler.StopProcess(procMgr, sysLog))
			auth.POST("/process/restart", handler.RestartProcess(procMgr, sysLog))
			auth.GET("/process/status", handler.ProcessStatus(procMgr))

			// 系统信息
			auth.GET("/system/env", handler.GetSystemEnv(cfg))
			auth.GET("/system/version", handler.GetVersion(cfg))
			auth.POST("/system/backup", handler.Backup(cfg))
			auth.GET("/system/backups", handler.ListBackups(cfg))
			auth.POST("/system/restore", handler.Restore(cfg))
			auth.POST("/system/restart-gateway", handler.RestartGateway(cfg, procMgr))
			auth.POST("/system/restart-panel", handler.RestartPanel())
			auth.GET("/system/restart-gateway-status", handler.RestartGatewayStatus(cfg))

			// 技能 & 插件
			auth.GET("/system/skills", handler.GetSkills(cfg))
			auth.PUT("/system/skills/:id/toggle", handler.ToggleSkill(cfg))

			// 定时任务
			auth.GET("/system/cron", handler.GetCronJobs(cfg))
			auth.PUT("/system/cron", handler.SaveCronJobs(cfg))

			// 文档管理
			auth.GET("/system/docs", handler.GetDocs(cfg))
			auth.PUT("/system/docs", handler.SaveDoc(cfg))
			auth.GET("/system/identity-docs", handler.GetIdentityDocs(cfg))
			auth.PUT("/system/identity-docs", handler.SaveIdentityDoc(cfg))

			// 模型健康检查
			auth.POST("/system/model-health", handler.ModelHealthCheck())

			// AI 助手
			auth.POST("/system/ai-chat", handler.AIChat(cfg))
			auth.GET("/workflows/settings", workflowRuntime.GetSettings())
			auth.PUT("/workflows/settings", workflowRuntime.SaveSettings())
			auth.GET("/workflows/templates", workflowRuntime.ListTemplates())
			auth.POST("/workflows/templates", workflowRuntime.SaveTemplate())
			auth.DELETE("/workflows/templates/:id", workflowRuntime.DeleteTemplate())
			auth.POST("/workflows/templates/generate", workflowRuntime.GenerateTemplate())
			auth.GET("/workflows/runs", workflowRuntime.ListRuns())
			auth.GET("/workflows/runs/:id", workflowRuntime.GetRun())
			auth.POST("/workflows/templates/:id/run", workflowRuntime.StartRunFromTemplate())
			auth.POST("/workflows/runs/:id/control", workflowRuntime.ControlRun())
			auth.POST("/workflows/runs/:id/artifacts/resend", workflowRuntime.ResendArtifacts())
			auth.DELETE("/workflows/runs/:id", workflowRuntime.DeleteRun())

			// OpenClaw 更新
			auth.POST("/system/check-update", handler.CheckUpdate(cfg))
			auth.POST("/system/do-update", handler.DoUpdate(cfg))
			auth.GET("/system/update-status", handler.UpdateStatus(cfg))

			// ClawPanel 面板自检更新
			auth.GET("/panel/version", handler.GetPanelVersion(Version))
			auth.GET("/panel/check-update", handler.CheckPanelUpdate(panelUpdater))
			auth.POST("/panel/do-update", handler.DoPanelUpdate(panelUpdater))
			auth.GET("/panel/update-progress", handler.PanelUpdateProgress(panelUpdater))
			auth.GET("/panel/update-popup", handler.GetUpdatePopup(panelUpdater))
			auth.POST("/panel/update-popup/shown", handler.MarkUpdatePopupShown(panelUpdater))

			// 独立更新工具
			auth.POST("/panel/update-token", handler.GenerateUpdateToken(cfg, cfg.Port))
			auth.GET("/panel/update-history", handler.GetUpdateHistory(cfg))

			// 事件日志
			auth.GET("/events", handler.GetEvents(db))
			auth.POST("/events/clear", handler.ClearEvents(db))

			// Admin 配置
			auth.GET("/admin/config", handler.GetAdminConfig(cfg))
			auth.PUT("/admin/config", handler.SaveAdminConfig(cfg))
			auth.PUT("/admin/config/:section", handler.SaveAdminSection(cfg))

			// Admin Token & Sudo Password
			auth.GET("/system/admin-token", handler.GetAdminToken(cfg))
			auth.GET("/system/sudo-password", handler.GetSudoPassword(cfg))
			auth.PUT("/system/sudo-password", handler.SetSudoPassword(cfg))

			// ClawHub 同步
			auth.POST("/system/clawhub-sync", handler.ClawHubSync(cfg))

			// Bot 操作
			auth.GET("/bot/groups", handler.GetBotGroups(cfg))
			auth.GET("/bot/friends", handler.GetBotFriends(cfg))
			auth.POST("/bot/send", handler.BotSend(cfg))
			auth.POST("/bot/reconnect", handler.BotReconnect(cfg))

			// 请求审批
			auth.GET("/requests", handler.GetRequests(cfg))
			auth.POST("/requests/:flag/approve", handler.ApproveRequest(cfg))
			auth.POST("/requests/:flag/reject", handler.RejectRequest(cfg))

			// NapCat QQ 登录
			auth.POST("/napcat/login-status", handler.NapcatLoginStatus(cfg))
			auth.POST("/napcat/qrcode", handler.NapcatGetQRCode(cfg))
			auth.POST("/napcat/qrcode/refresh", handler.NapcatRefreshQRCode(cfg))
			auth.GET("/napcat/quick-login-list", handler.NapcatQuickLoginList(cfg))
			auth.POST("/napcat/quick-login", handler.NapcatQuickLogin(cfg))
			auth.POST("/napcat/password-login", handler.NapcatPasswordLogin(cfg))
			auth.GET("/napcat/login-info", handler.NapcatLoginInfo(cfg))
			auth.POST("/napcat/logout", handler.NapcatLogout(cfg))
			auth.POST("/napcat/restart", handler.RestartNapcat(cfg))
			auth.GET("/napcat/status", handler.GetNapCatStatus(napcatMon))
			auth.GET("/napcat/reconnect-logs", handler.GetNapCatReconnectLogs(napcatMon))
			auth.POST("/napcat/reconnect", handler.NapCatReconnect(napcatMon))
			auth.PUT("/napcat/monitor-config", handler.NapCatMonitorConfig(napcatMon))
			auth.POST("/napcat/diagnose", handler.DiagnoseNapCat(cfg, procMgr))

			// 系统诊断
			auth.GET("/system/diagnose", handler.SystemDiagnose(cfg))

			// WeChat
			auth.GET("/wechat/status", handler.WechatStatus(cfg))
			auth.GET("/wechat/login-url", handler.WechatLoginUrl(cfg))
			auth.POST("/wechat/send", handler.WechatSend(cfg))
			auth.POST("/wechat/send-file", handler.WechatSendFile(cfg))
			auth.GET("/wechat/config", handler.WechatGetConfig(cfg))
			auth.PUT("/wechat/config", handler.WechatUpdateConfig(cfg))

			// 工作区
			auth.GET("/workspace/files", handler.WorkspaceFiles(cfg))
			auth.GET("/workspace/stats", handler.WorkspaceStats(cfg))
			auth.GET("/workspace/config", handler.WorkspaceConfig(cfg))
			auth.PUT("/workspace/config", handler.WorkspaceUpdateConfig(cfg))
			auth.POST("/workspace/upload", handler.WorkspaceUpload(cfg))
			auth.POST("/workspace/mkdir", handler.WorkspaceMkdir(cfg))
			auth.POST("/workspace/delete", handler.WorkspaceDelete(cfg))
			auth.POST("/workspace/clean", handler.WorkspaceClean(cfg))
			auth.GET("/workspace/notes", handler.WorkspaceNotes(cfg))
			auth.PUT("/workspace/notes", handler.WorkspaceSetNote(cfg))

			// 会话管理
			auth.GET("/sessions", handler.GetSessions(cfg))
			auth.GET("/sessions/:id", handler.GetSessionDetail(cfg))
			auth.DELETE("/sessions/:id", handler.DeleteSession(cfg))

			// 配置检测 & 修复
			auth.GET("/openclaw/config/check", handler.CheckConfig(cfg))
			auth.POST("/openclaw/config/fix", handler.FixConfig(cfg))

			// 插件中心
			auth.GET("/plugins/list", handler.GetPluginList(pluginMgr))
			auth.GET("/plugins/installed", handler.GetInstalledPlugins(pluginMgr))
			auth.GET("/plugins/:id", handler.GetPluginDetail(pluginMgr))
			auth.POST("/plugins/registry/refresh", handler.RefreshPluginRegistry(pluginMgr))
			auth.POST("/plugins/install", handler.InstallPlugin(pluginMgr, taskMgr))
			auth.DELETE("/plugins/:id", handler.UninstallPlugin(pluginMgr, taskMgr))
			auth.PUT("/plugins/:id/toggle", handler.TogglePlugin(pluginMgr))
			auth.GET("/plugins/:id/config", handler.GetPluginConfig(pluginMgr))
			auth.PUT("/plugins/:id/config", handler.UpdatePluginConfig(pluginMgr))
			auth.GET("/plugins/:id/logs", handler.GetPluginLogs(pluginMgr))
			auth.POST("/plugins/:id/update", handler.UpdatePluginVersion(pluginMgr))

			// 软件环境 & 安装任务
			auth.GET("/software/list", handler.GetSoftwareList(cfg))
			auth.GET("/software/openclaw-instances", handler.DetectOpenClawInstances(cfg))
			auth.POST("/software/install", handler.InstallSoftware(cfg, taskMgr))
			auth.GET("/tasks", handler.GetTasks(taskMgr))
			auth.GET("/tasks/:id", handler.GetTaskDetail(taskMgr))

			// WebSocket 实时日志
			auth.GET("/ws/logs", wsHub.HandleWebSocket())
		}

		// 工作区下载和预览（支持 token query param）
		api.GET("/workspace/download", handler.WorkspaceDownload(cfg))
		api.GET("/workspace/preview", handler.WorkspacePreview(cfg))

		// 外部日志接口（无需认证）
		api.POST("/events/log", handler.PostEvent(db, wsHub))
	}

	// WebSocket 路由（前端连接 /ws?token=...，需通过 JWT 验证）
	r.GET("/ws", wsHub.HandleWebSocket(func(token string) bool {
		return middleware.ValidateToken(token, cfg.JWTSecret)
	}))

	// 内嵌前端静态资源
	frontendDist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatalf("[ClawPanel] 前端资源加载失败: %v", err)
	}
	// SPA fallback: 所有非 API 路由返回 index.html
	staticFS := http.FS(frontendDist)
	r.NoRoute(func(c *gin.Context) {
		urlPath := c.Request.URL.Path

		// 尝试提供静态文件（仅当路径包含扩展名时，如 .js .css .png）
		if strings.Contains(urlPath, ".") {
			f, err := staticFS.Open(urlPath)
			if err == nil {
				defer f.Close()
				stat, _ := f.Stat()
				if !stat.IsDir() {
					http.ServeContent(c.Writer, c.Request, urlPath, stat.ModTime(), f)
					return
				}
			}
		}

		// SPA fallback: 所有其他路由返回 index.html
		indexData, err := frontendDist.Open("index.html")
		if err != nil {
			c.String(404, "Not Found")
			return
		}
		defer indexData.Close()
		stat, _ := indexData.Stat()
		c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), indexData.(io.ReadSeeker))
	})

	// 启动日志收集（将 OpenClaw 进程日志推送到 WebSocket）
	go procMgr.StreamLogs(wsHub)

	// 启动服务器
	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	log.Printf("[ClawPanel] v%s 启动中 → http://%s", Version, addr)
	log.Printf("[ClawPanel] 数据目录: %s", cfg.DataDir)
	log.Printf("[ClawPanel] OpenClaw 目录: %s", cfg.OpenClawDir)

	srv := &http.Server{Addr: addr, Handler: r}

	// 优雅关闭：监听 stopCh
	go func() {
		<-stopCh
		procMgr.StopAll()
		srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[ClawPanel] 服务器启动失败: %v", err)
	}
}
