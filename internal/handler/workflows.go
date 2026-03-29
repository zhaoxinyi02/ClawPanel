package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/model"
	ws "github.com/zhaoxinyi02/ClawPanel/internal/websocket"
)

const workflowTaskType = "workflow_run"

var workflowShortIDPattern = regexp.MustCompile(`#([A-Za-z0-9]{4,12})`)

type workflowRuntime struct {
	db      *sql.DB
	cfg     *config.Config
	hub     *ws.Hub
	mu      sync.Mutex
	running map[string]bool
}

func NewWorkflowRuntime(db *sql.DB, cfg *config.Config, hub *ws.Hub) *workflowRuntime {
	return &workflowRuntime{db: db, cfg: cfg, hub: hub, running: map[string]bool{}}
}

func (rt *workflowRuntime) GetSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := model.GetWorkflowSettings(rt.db)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "settings": settings})
	}
}

func (rt *workflowRuntime) SaveSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req model.WorkflowSettings
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := model.SaveWorkflowSettings(rt.db, &req); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func (rt *workflowRuntime) ListTemplates() gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := model.ListWorkflowTemplates(rt.db)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "templates": items})
	}
}

func (rt *workflowRuntime) SaveTemplate() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Template model.WorkflowTemplate `json:"template"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if strings.TrimSpace(req.Template.ID) == "" {
			req.Template.ID = model.NewWorkflowTemplateID()
		}
		if req.Template.Status == "" {
			req.Template.Status = "ready"
		}
		if req.Template.TriggerMode == "" {
			req.Template.TriggerMode = "manual"
		}
		req.Template.Definition = model.NormalizeWorkflowTemplateDefinition(req.Template.Definition)
		if err := model.UpsertWorkflowTemplate(rt.db, &req.Template); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "template": req.Template})
	}
}

func (rt *workflowRuntime) DeleteTemplate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := model.DeleteWorkflowTemplate(rt.db, c.Param("id")); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func (rt *workflowRuntime) GenerateTemplate() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Prompt   string                 `json:"prompt"`
			Category string                 `json:"category"`
			Settings map[string]interface{} `json:"settings"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "prompt required"})
			return
		}
		generated, raw, err := rt.generateTemplateByAI(req.Prompt, req.Category, req.Settings)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "template": generated, "raw": raw})
	}
}

func (rt *workflowRuntime) ListRuns() gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := model.ListWorkflowRuns(rt.db, strings.TrimSpace(c.Query("status")))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "runs": items})
	}
}

func (rt *workflowRuntime) GetRun() gin.HandlerFunc {
	return func(c *gin.Context) {
		run, err := model.GetWorkflowRun(rt.db, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		events, _ := model.ListWorkflowEvents(rt.db, run.ID)
		c.JSON(http.StatusOK, gin.H{"ok": true, "run": run, "events": events})
	}
}

func (rt *workflowRuntime) InterceptInbound() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader("X-Workflow-Token"))
		if token == "" || token != strings.TrimSpace(rt.cfg.AdminToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "unauthorized"})
			return
		}
		var req struct {
			ChannelID      string                 `json:"channelId"`
			ConversationID string                 `json:"conversationId"`
			UserID         string                 `json:"userId"`
			Text           string                 `json:"text"`
			Context        map[string]interface{} `json:"context"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		handled, reply, reason := rt.interceptInboundMessage(strings.TrimSpace(req.ChannelID), strings.TrimSpace(req.ConversationID), strings.TrimSpace(req.UserID), strings.TrimSpace(req.Text), req.Context)
		c.JSON(http.StatusOK, gin.H{"ok": true, "handled": handled, "reply": reply, "reason": reason})
	}
}

func (rt *workflowRuntime) StartRunFromTemplate() gin.HandlerFunc {
	return func(c *gin.Context) {
		tpl, err := model.GetWorkflowTemplate(rt.db, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		var req struct {
			Input          string                 `json:"input"`
			ChannelID      string                 `json:"channelId"`
			ConversationID string                 `json:"conversationId"`
			UserID         string                 `json:"userId"`
			SourceMessage  string                 `json:"sourceMessage"`
			Context        map[string]interface{} `json:"context"`
		}
		_ = c.ShouldBindJSON(&req)
		run, steps, err := rt.instantiateRun(tpl, req.Input, req.ChannelID, req.ConversationID, req.UserID, req.SourceMessage, req.Context)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := model.CreateWorkflowRun(rt.db, run, steps); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		run.Steps = steps
		rt.logWorkflowActivity(run, "workflow.run.started", fmt.Sprintf("工作流 %s %s 已启动", run.Name, run.ShortID), run.SourceMessage)
		rt.broadcastWorkflowTaskUpdate(run)
		rt.broadcastWorkflowTaskLog(run.ID, fmt.Sprintf("🚀 %s %s 已启动", run.Name, run.ShortID))
		go rt.executeRun(run.ID)
		c.JSON(http.StatusOK, gin.H{"ok": true, "run": run})
	}
}

func (rt *workflowRuntime) ControlRun() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Action string `json:"action"`
			Reply  string `json:"reply"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		run, err := model.GetWorkflowRun(rt.db, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		updated, action, err := rt.applyRunControl(run, strings.TrimSpace(req.Action), req.Reply, "api")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "run": updated, "action": action})
	}
}

func (rt *workflowRuntime) DeleteRun() gin.HandlerFunc {
	return func(c *gin.Context) {
		run, _ := model.GetWorkflowRun(rt.db, c.Param("id"))
		if err := model.DeleteWorkflowRun(rt.db, c.Param("id")); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if run != nil {
			rt.logWorkflowActivity(run, "workflow.run.deleted", fmt.Sprintf("工作流 %s %s 已删除", run.Name, run.ShortID), run.SourceMessage)
			if rt.hub != nil {
				data, _ := json.Marshal(map[string]interface{}{
					"type": "task_update",
					"task": map[string]interface{}{
						"id":        workflowTaskID(run.ID),
						"name":      fmt.Sprintf("%s %s", run.Name, run.ShortID),
						"type":      workflowTaskType,
						"status":    "canceled",
						"progress":  0,
						"error":     "deleted",
						"createdAt": time.UnixMilli(run.CreatedAt).Format(time.RFC3339),
						"updatedAt": time.Now().Format(time.RFC3339),
						"logCount":  0,
					},
				})
				rt.hub.Broadcast(data)
			}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func (rt *workflowRuntime) ResendArtifacts() gin.HandlerFunc {
	return func(c *gin.Context) {
		run, err := model.GetWorkflowRun(rt.db, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		var req struct {
			StepKey  string `json:"stepKey"`
			FileName string `json:"fileName"`
		}
		_ = c.ShouldBindJSON(&req)
		artifacts := toArtifactSlice(run.Context["artifactFiles"])
		selected := make([]map[string]interface{}, 0, 1)
		if strings.TrimSpace(req.StepKey) == "" && strings.TrimSpace(req.FileName) == "" {
			selected = selectedArtifactsForDelivery(run)
		} else {
			for _, item := range artifacts {
				if strings.TrimSpace(req.StepKey) != "" && strings.TrimSpace(toStringLocal(item["stepKey"])) == strings.TrimSpace(req.StepKey) {
					selected = append(selected, item)
					break
				}
				if strings.TrimSpace(req.FileName) != "" && strings.TrimSpace(toStringLocal(item["fileName"])) == strings.TrimSpace(req.FileName) {
					selected = append(selected, item)
					break
				}
			}
		}
		if len(selected) == 0 {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "artifact not found"})
			return
		}
		updated, sent, failed, err := rt.sendArtifactsToOrigin(run, selected, true)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "run": updated, "sent": sent, "failed": failed})
	}
}

func (rt *workflowRuntime) HandleInboundReply(channelID, conversationID, userID, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if handled, reply, _ := rt.interceptWaitingRuns(channelID, conversationID, userID, text, true, nil); handled {
		if reply != "" {
			_ = rt.sendWorkflowAck(channelID, conversationID, userID, reply)
		}
		return
	}
	if channelID != "qq" {
		rt.handleAutoCreateRun(channelID, conversationID, userID, text)
	}
}

func (rt *workflowRuntime) interceptInboundMessage(channelID, conversationID, userID, text string, extra map[string]interface{}) (bool, string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return false, "", "empty"
	}
	if handled, reply, reason := rt.interceptWaitingRuns(channelID, conversationID, userID, text, false, extra); handled {
		return true, reply, reason
	}
	return rt.tryInterceptInbound(channelID, conversationID, userID, text, extra)
}

func (rt *workflowRuntime) interceptWaitingRuns(channelID, conversationID, userID, text string, notify bool, extra map[string]interface{}) (bool, string, string) {
	runs, err := model.ListWorkflowRuns(rt.db, "")
	if err != nil {
		return false, "", "list_failed"
	}
	candidates := make([]model.WorkflowRun, 0, 4)
	mentionedShortID := extractMentionedWorkflowShortID(text)
	for _, run := range runs {
		if run.Status != "waiting_for_user" && run.Status != "waiting_for_approval" && run.Status != "paused" {
			continue
		}
		if mentionedShortID != "" && strings.EqualFold(run.ShortID, mentionedShortID) {
			candidates = append(candidates, run)
			continue
		}
		if mentionedShortID == "" && run.ChannelID == channelID && run.ConversationID == conversationID {
			candidates = append(candidates, run)
		}
	}
	if len(candidates) == 0 {
		return false, "", "no_waiting_run"
	}
	if len(candidates) > 1 {
		clarify := rt.generateAmbiguousReply(candidates)
		if notify {
			if err := rt.sendWorkflowAck(channelID, conversationID, userID, clarify); err != nil {
				rt.logWorkflowActivity(&model.WorkflowRun{Name: "工作流", ShortID: "", ChannelID: channelID, ConversationID: conversationID}, "workflow.reply.ambiguous_notify_failed", "工作流歧义澄清发送失败", err.Error())
			} else {
				rt.logWorkflowActivity(&model.WorkflowRun{Name: "工作流", ShortID: "", ChannelID: channelID, ConversationID: conversationID}, "workflow.reply.ambiguous_notify", fmt.Sprintf("检测到 %d 个等待中的工作流，已请求用户澄清", len(candidates)), clarify)
			}
		} else {
			rt.logWorkflowActivity(&model.WorkflowRun{Name: "工作流", ShortID: "", ChannelID: channelID, ConversationID: conversationID}, "workflow.reply.ambiguous_notify", fmt.Sprintf("检测到 %d 个等待中的工作流，已请求用户澄清", len(candidates)), clarify)
		}
		rt.logWorkflowActivity(&model.WorkflowRun{Name: "工作流", ShortID: "", ChannelID: channelID, ConversationID: conversationID}, "workflow.reply.ambiguous", fmt.Sprintf("检测到 %d 个等待中的工作流，用户回复未能唯一匹配", len(candidates)), text)
		return true, clarify, "ambiguous"
	}
	run, err := model.GetWorkflowRun(rt.db, candidates[0].ID)
	if err != nil {
		return false, "", "run_load_failed"
	}
	if len(extra) > 0 {
		if run.Context == nil {
			run.Context = map[string]interface{}{}
		}
		for k, v := range extra {
			run.Context[k] = v
		}
		_ = model.UpdateWorkflowRun(rt.db, run)
	}
	if _, action, err := rt.applyRunControl(run, "", text, "inbound"); err == nil {
		rt.logWorkflowActivity(run, "workflow.reply.matched", fmt.Sprintf("原会话回复已命中工作流 %s %s", run.Name, run.ShortID), fmt.Sprintf("action=%s\nreply=%s", action, text))
		return true, rt.generateMatchedReply(run, action), action
	}
	return false, "", "apply_failed"
}

func (rt *workflowRuntime) tryInterceptInbound(channelID, conversationID, userID, text string, extra map[string]interface{}) (bool, string, string) {
	settings, err := model.GetWorkflowSettings(rt.db)
	if err != nil || settings == nil {
		return false, "", "settings_unavailable"
	}
	if !settings.Enabled || !settings.AutoCreateRuns {
		return false, "", "workflow_disabled"
	}
	complex, reason := rt.isComplexTask(text, settings)
	if !complex {
		return false, "", "not_complex"
	}
	if existing := rt.findRecentAutoCreatedRun(channelID, conversationID, text, 15*time.Second); existing != nil {
		return true, fmt.Sprintf("这个任务已经由 %s 接管了，我继续按当前工作流推进。", existing.ShortID), "deduplicated"
	}
	ack := rt.generateImmediateAck(text)
	if ack == "" {
		ack = "收到，我先按工作流帮你拆开推进，关键进展我会及时同步。"
	}
	if channelID == "qq" {
		if err := rt.sendWorkflowAck(channelID, conversationID, userID, ack); err != nil {
			rt.logWorkflowActivity(&model.WorkflowRun{Name: "工作流", ChannelID: channelID, ConversationID: conversationID}, "workflow.ack.failed", "工作流即时确认发送失败", err.Error())
		} else {
			rt.logWorkflowActivity(&model.WorkflowRun{Name: "工作流", ChannelID: channelID, ConversationID: conversationID}, "workflow.ack.sent", "复杂任务已即时确认，开始创建工作流", ack)
		}
	} else {
		rt.logWorkflowActivity(&model.WorkflowRun{Name: "工作流", ChannelID: channelID, ConversationID: conversationID}, "workflow.ack.sent", "复杂任务已即时确认，开始创建工作流", ack)
	}
	go rt.createAutoWorkflowRun(channelID, conversationID, userID, text, reason, extra)
	return true, ack, reason
}

func (rt *workflowRuntime) handleAutoCreateRun(channelID, conversationID, userID, text string) {
	_, _, _ = rt.tryInterceptInbound(channelID, conversationID, userID, text, nil)
}

func (rt *workflowRuntime) createAutoWorkflowRun(channelID, conversationID, userID, text, reason string, extra map[string]interface{}) {
	tpl, _, err := rt.generateTemplateByAI(text, "自动任务", map[string]interface{}{
		"approvalMode":  2,
		"progressMode":  "detailed",
		"tone":          "professional",
		"autoGenerated": true,
	})
	if err != nil || tpl == nil {
		tpl = rt.fallbackGeneratedTemplate(text, "自动任务", map[string]interface{}{
			"approvalMode":  2,
			"progressMode":  "detailed",
			"tone":          "professional",
			"autoGenerated": true,
		})
	}
	if tpl.Name == "" {
		tpl.Name = "自动创建工作流"
	}
	ctx := map[string]interface{}{
		"autoCreated":        true,
		"complexityReason":   reason,
		"originConversation": conversationID,
	}
	for k, v := range extra {
		ctx[k] = v
	}
	run, steps, err := rt.instantiateRun(tpl, text, channelID, conversationID, userID, text, ctx)
	if err != nil {
		rt.logWorkflowActivity(&model.WorkflowRun{Name: tpl.Name, ChannelID: channelID, ConversationID: conversationID}, "workflow.run.auto_create_failed", "自动创建工作流失败", err.Error())
		return
	}
	if err := model.CreateWorkflowRun(rt.db, run, steps); err != nil {
		rt.logWorkflowActivity(run, "workflow.run.auto_create_failed", fmt.Sprintf("自动创建工作流 %s 失败", tpl.Name), err.Error())
		return
	}
	run.Steps = steps
	rt.logWorkflowActivity(run, "workflow.run.auto_created", fmt.Sprintf("已从原会话自动创建工作流 %s %s", run.Name, run.ShortID), text)
	rt.broadcastWorkflowTaskUpdate(run)
	rt.broadcastWorkflowTaskLog(run.ID, fmt.Sprintf("🚀 已从原会话自动创建 %s %s", run.Name, run.ShortID))
	go rt.executeRun(run.ID)
}

func (rt *workflowRuntime) shouldPushProgress() bool {
	settings, err := model.GetWorkflowSettings(rt.db)
	if err != nil || settings == nil {
		return false
	}
	return settings.PushProgress
}

func (rt *workflowRuntime) pushRunMessageToOrigin(run *model.WorkflowRun, message string) {
	if run == nil || !rt.shouldPushProgress() {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" || strings.TrimSpace(run.ChannelID) == "" {
		return
	}
	conversationID := run.ConversationID
	if run.ChannelID == "wecom" && run.Context != nil {
		if responseURL := strings.TrimSpace(toStringLocal(run.Context["responseUrl"])); responseURL != "" {
			conversationID = responseURL
		}
	}
	if err := rt.sendWorkflowAck(run.ChannelID, conversationID, run.UserID, message); err != nil {
		rt.updateOriginPushState(run, false, err.Error(), message)
		rt.logWorkflowActivity(run, "workflow.origin_push.failed", fmt.Sprintf("工作流 %s %s 回写原通道失败", run.Name, run.ShortID), err.Error())
	} else {
		rt.updateOriginPushState(run, true, "", message)
		rt.logWorkflowActivity(run, "workflow.origin_push.sent", fmt.Sprintf("工作流 %s %s 已回写原通道", run.Name, run.ShortID), message)
	}
}

func (rt *workflowRuntime) renderFinalArtifactSummary(run *model.WorkflowRun) string {
	if run == nil || run.Context == nil {
		return ""
	}
	files := toArtifactSlice(run.Context["finalFiles"])
	if len(files) == 0 {
		files = toArtifactSlice(run.Context["artifactFiles"])
	}
	if len(files) == 0 {
		return ""
	}
	lines := []string{"已为你生成以下结果文件："}
	for i, item := range files {
		name := strings.TrimSpace(toStringLocal(item["artifactName"]))
		if name == "" {
			name = strings.TrimSpace(toStringLocal(item["fileName"]))
		}
		lines = append(lines, fmt.Sprintf("%d. %s（%s）", i+1, name, toStringLocal(item["relativePath"])))
	}
	if relDir := strings.TrimSpace(toStringLocal(run.Context["workflowDirRelative"])); relDir != "" {
		lines = append(lines, fmt.Sprintf("工作区目录：%s", filepath.ToSlash(relDir)))
	}
	return strings.Join(lines, "\n")
}

func (rt *workflowRuntime) updateOriginPushState(run *model.WorkflowRun, success bool, errorText, message string) {
	if run == nil {
		return
	}
	if run.Context == nil {
		run.Context = map[string]interface{}{}
	}
	status := "failed"
	if success {
		status = "sent"
	}
	run.Context["originPushStatus"] = status
	run.Context["originPushMessage"] = strings.TrimSpace(message)
	run.Context["originPushUpdatedAt"] = time.Now().UnixMilli()
	if success {
		delete(run.Context, "originPushError")
	} else {
		run.Context["originPushError"] = strings.TrimSpace(errorText)
	}
	_ = model.UpdateWorkflowRun(rt.db, run)
}

func (rt *workflowRuntime) isComplexTask(text string, settings *model.WorkflowSettings) (bool, string) {
	prompt := fmt.Sprintf("请判断下面这条用户任务是否适合转为多步骤工作流执行。只输出 JSON：{\"complex\":true|false,\"reason\":\"...\"}。判断维度：是否多阶段、是否需要规划/审批/等待用户/多产物协同。复杂度阈值：%s。用户任务：%s", settings.ComplexityGuard, text)
	out, _, err := rt.callWorkflowModel([]map[string]string{{"role": "system", "content": "你是工作流复杂度分类器，只输出 JSON。"}, {"role": "user", "content": prompt}})
	if err == nil {
		var result struct {
			Complex bool   `json:"complex"`
			Reason  string `json:"reason"`
		}
		if json.Unmarshal([]byte(stripCodeFence(strings.TrimSpace(out))), &result) == nil {
			return result.Complex, strings.TrimSpace(result.Reason)
		}
	}
	lower := strings.ToLower(text)
	keywords := []string{"然后", "再", "最后", "审批", "确认", "规划", "流程", "方案", "复盘", "多步", "多个", "先", "总结", "整理", "输出", "活动", "运营"}
	hits := 0
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			hits++
		}
	}
	return hits >= 2 || len([]rune(text)) > 30, "fallback heuristic"
}

func (rt *workflowRuntime) generateImmediateAck(text string) string {
	prompt := fmt.Sprintf("用户发来一个复杂任务：%s。请生成一条立即回复给用户的话，要求自然、简短、像真人助手，表达你已经接手并会按工作流推进。不要使用模板口吻。", text)
	out, _, err := rt.callWorkflowModel([]map[string]string{{"role": "system", "content": "你是工作流即时确认助手，输出一句简短中文。"}, {"role": "user", "content": prompt}})
	if err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	return "收到，我先按工作流帮你拆开推进，关键进展我会及时同步。"
}

func (rt *workflowRuntime) sendWorkflowAck(channelID, conversationID, userID, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	switch channelID {
	case "qq":
		if strings.HasPrefix(conversationID, "qq:group:") {
			groupID := strings.TrimPrefix(conversationID, "qq:group:")
			_, err := onebotApiCallSafe("POST", "/send_group_msg", map[string]interface{}{"group_id": groupID, "message": message})
			return err
		}
		privateID := userID
		if privateID == "" && strings.HasPrefix(conversationID, "qq:private:") {
			privateID = strings.TrimPrefix(conversationID, "qq:private:")
		}
		if privateID == "" {
			return fmt.Errorf("missing QQ private target")
		}
		_, err := onebotApiCallSafe("POST", "/send_private_msg", map[string]interface{}{"user_id": privateID, "message": message})
		return err
	case "feishu":
		chatID := strings.TrimSpace(conversationID)
		if chatID == "" {
			return fmt.Errorf("missing Feishu chat id")
		}
		return rt.sendFeishuText(chatID, message)
	case "wecom":
		responseURL := ""
		if strings.TrimSpace(conversationID) != "" {
			responseURL = strings.TrimSpace(conversationID)
		}
		if strings.HasPrefix(responseURL, "wecom:") || !strings.HasPrefix(responseURL, "http") {
			responseURL = ""
		}
		return rt.sendWecomText(responseURL, message)
	case "wechat":
		target := strings.TrimSpace(conversationID)
		if target == "" {
			target = strings.TrimSpace(userID)
		}
		if target == "" {
			return fmt.Errorf("missing WeChat target")
		}
		_, err := wechatBridgeRequest(rt.cfg, http.MethodPost, "/send/text", map[string]interface{}{
			"to":      target,
			"content": message,
			"isRoom":  strings.HasSuffix(target, "@chatroom"),
		})
		return err
	default:
		return fmt.Errorf("channel not supported yet: %s", channelID)
	}
}

func workflowArtifactFileServerURL(absPath string) string {
	absPath = strings.TrimSpace(absPath)
	if absPath == "" {
		return ""
	}
	return "http://172.17.0.1:18790/file?path=" + url.QueryEscape(absPath)
}

func qqPrivateTargetUserID(userID, conversationID string) string {
	userID = strings.TrimSpace(userID)
	if userID != "" {
		return userID
	}
	conversationID = strings.TrimSpace(conversationID)
	if strings.HasPrefix(conversationID, "qq:private:") {
		return strings.TrimSpace(strings.TrimPrefix(conversationID, "qq:private:"))
	}
	return ""
}

func qqGroupTargetID(conversationID string) string {
	conversationID = strings.TrimSpace(conversationID)
	if strings.HasPrefix(conversationID, "qq:group:") {
		return strings.TrimSpace(strings.TrimPrefix(conversationID, "qq:group:"))
	}
	return ""
}

func isOneBotCallSuccessful(result map[string]interface{}) bool {
	if len(result) == 0 {
		return false
	}
	if status := strings.ToLower(strings.TrimSpace(toStringLocal(result["status"]))); status == "ok" {
		return true
	}
	switch code := result["retcode"].(type) {
	case float64:
		return code == 0
	case int:
		return code == 0
	case int64:
		return code == 0
	case json.Number:
		return code == "0"
	case string:
		return strings.TrimSpace(code) == "0"
	}
	return false
}

func oneBotResultError(result map[string]interface{}) error {
	if isOneBotCallSuccessful(result) {
		return nil
	}
	message := strings.TrimSpace(toStringLocal(result["msg"]))
	wording := strings.TrimSpace(toStringLocal(result["wording"]))
	if message == "" {
		message = wording
	}
	if message == "" {
		message = strings.TrimSpace(mustJSON(result))
	}
	return fmt.Errorf("onebot api failed: %s", message)
}

func (rt *workflowRuntime) sendQQPrivateArtifact(userID, absPath, fileName string) error {
	userID = strings.TrimSpace(userID)
	absPath = strings.TrimSpace(absPath)
	fileName = strings.TrimSpace(fileName)
	if userID == "" {
		return fmt.Errorf("missing QQ private target")
	}
	if absPath == "" {
		return fmt.Errorf("artifact path missing")
	}
	if fileName == "" {
		fileName = filepath.Base(absPath)
	}
	if _, err := os.Stat(absPath); err != nil {
		return err
	}
	result, err := onebotApiCallSafe("POST", "/upload_private_file", map[string]interface{}{"user_id": userID, "file": absPath, "name": fileName})
	if err == nil && isOneBotCallSuccessful(result) {
		return nil
	}
	fileURL := workflowArtifactFileServerURL(absPath)
	if fileURL == "" {
		if err != nil {
			return err
		}
		return oneBotResultError(result)
	}
	result, retryErr := onebotApiCallSafe("POST", "/upload_private_file", map[string]interface{}{"user_id": userID, "file": fileURL, "name": fileName})
	if retryErr != nil {
		if err != nil {
			return fmt.Errorf("upload local failed: %v; upload url failed: %w", err, retryErr)
		}
		return retryErr
	}
	return oneBotResultError(result)
}

func (rt *workflowRuntime) sendQQGroupArtifact(groupID, absPath, fileName string) error {
	groupID = strings.TrimSpace(groupID)
	absPath = strings.TrimSpace(absPath)
	fileName = strings.TrimSpace(fileName)
	if groupID == "" {
		return fmt.Errorf("missing QQ group target")
	}
	if absPath == "" {
		return fmt.Errorf("artifact path missing")
	}
	if fileName == "" {
		fileName = filepath.Base(absPath)
	}
	if _, err := os.Stat(absPath); err != nil {
		return err
	}
	result, err := onebotApiCallSafe("POST", "/upload_group_file", map[string]interface{}{"group_id": groupID, "file": absPath, "name": fileName, "folder": ""})
	if err == nil && isOneBotCallSuccessful(result) {
		return nil
	}
	fileURL := workflowArtifactFileServerURL(absPath)
	if fileURL == "" {
		if err != nil {
			return err
		}
		return oneBotResultError(result)
	}
	result, retryErr := onebotApiCallSafe("POST", "/upload_group_file", map[string]interface{}{"group_id": groupID, "file": fileURL, "name": fileName, "folder": ""})
	if retryErr != nil {
		if err != nil {
			return fmt.Errorf("upload local failed: %v; upload url failed: %w", err, retryErr)
		}
		return retryErr
	}
	return oneBotResultError(result)
}

func selectedArtifactsForDelivery(run *model.WorkflowRun) []map[string]interface{} {
	if run == nil || run.Context == nil {
		return nil
	}
	files := toArtifactSlice(run.Context["finalFiles"])
	if len(files) == 0 {
		files = toArtifactSlice(run.Context["artifactFiles"])
	}
	if len(files) == 0 {
		return nil
	}
	selected := make([]map[string]interface{}, 0, len(files))
	for _, item := range files {
		if toBoolLocal(item["sendToUser"]) {
			selected = append(selected, item)
		}
	}
	if len(selected) == 0 {
		selected = files
	}
	return selected
}

func (rt *workflowRuntime) updateArtifactDeliveryState(run *model.WorkflowRun, selected []map[string]interface{}, sent []string, failed []string, mode, target string) {
	if run == nil {
		return
	}
	if run.Context == nil {
		run.Context = map[string]interface{}{}
	}
	deliveryMap := map[string]map[string]interface{}{}
	for _, item := range toArtifactSlice(run.Context["artifactFiles"]) {
		key := strings.TrimSpace(toStringLocal(item["stepKey"]))
		if key == "" {
			key = strings.TrimSpace(toStringLocal(item["fileName"]))
		}
		deliveryMap[key] = item
	}
	failedSet := map[string]string{}
	for _, line := range failed {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			failedSet[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	now := time.Now().UnixMilli()
	for _, item := range selected {
		key := strings.TrimSpace(toStringLocal(item["stepKey"]))
		if key == "" {
			key = strings.TrimSpace(toStringLocal(item["fileName"]))
		}
		record, ok := deliveryMap[key]
		if !ok {
			continue
		}
		name := strings.TrimSpace(toStringLocal(record["artifactName"]))
		if name == "" {
			name = strings.TrimSpace(toStringLocal(record["fileName"]))
		}
		record["deliveryChannel"] = "qq"
		record["deliveryMode"] = mode
		record["deliveryTarget"] = target
		record["deliveryUpdatedAt"] = now
		if errText, hasFailed := failedSet[name]; hasFailed {
			record["deliveryStatus"] = "failed"
			record["deliveryError"] = errText
		} else {
			record["deliveryStatus"] = "sent"
			delete(record, "deliveryError")
		}
	}
	artifacts := make([]map[string]interface{}, 0, len(deliveryMap))
	for _, item := range toArtifactSlice(run.Context["artifactFiles"]) {
		key := strings.TrimSpace(toStringLocal(item["stepKey"]))
		if key == "" {
			key = strings.TrimSpace(toStringLocal(item["fileName"]))
		}
		if updated, ok := deliveryMap[key]; ok {
			artifacts = append(artifacts, updated)
		} else {
			artifacts = append(artifacts, item)
		}
	}
	run.Context["artifactFiles"] = artifacts
	run.Context["finalFiles"] = selectedArtifactsForDelivery(run)
	status := "skipped"
	if len(selected) > 0 {
		if len(failed) == 0 {
			status = "sent"
		} else if len(sent) > 0 {
			status = "partial"
		} else {
			status = "failed"
		}
	}
	run.Context["artifactDeliveryStatus"] = status
	run.Context["artifactDeliveryChannel"] = "qq"
	run.Context["artifactDeliveryMode"] = mode
	run.Context["artifactDeliveryTarget"] = target
	run.Context["artifactDeliveryUpdatedAt"] = now
	run.Context["artifactDeliverySentFiles"] = sent
	run.Context["artifactDeliveryFailedFiles"] = failed
	_ = model.UpdateWorkflowRun(rt.db, run)
}

func renderArtifactDeliveryNotice(run *model.WorkflowRun, sent []string, failed []string, mode string) string {
	targetLabel := "QQ 私聊"
	if mode == "group" {
		targetLabel = "QQ 群聊"
	}
	runLabel := "工作流"
	if run != nil {
		name := strings.TrimSpace(run.Name)
		if name != "" && strings.TrimSpace(run.ShortID) != "" {
			runLabel = fmt.Sprintf("%s %s", name, run.ShortID)
		} else if name != "" {
			runLabel = name
		} else if strings.TrimSpace(run.ShortID) != "" {
			runLabel = strings.TrimSpace(run.ShortID)
		}
	}
	lines := make([]string, 0, 3)
	if len(sent) > 0 {
		lines = append(lines, fmt.Sprintf("%s 已把 %d 个结果文件发到%s：", runLabel, len(sent), targetLabel))
		for i, name := range sent {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, name))
		}
	}
	if len(failed) > 0 {
		lines = append(lines, fmt.Sprintf("另有 %d 个文件回传失败，可在工作流中心查看原因。", len(failed)))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (rt *workflowRuntime) resolveArtifactDeliveryTarget(run *model.WorkflowRun) (string, string) {
	if run == nil {
		return "", ""
	}
	if groupID := qqGroupTargetID(run.ConversationID); groupID != "" {
		return "group", groupID
	}
	if userID := qqPrivateTargetUserID(run.UserID, run.ConversationID); userID != "" {
		return "private", userID
	}
	return "", ""
}

func (rt *workflowRuntime) sendArtifactsToOrigin(run *model.WorkflowRun, selected []map[string]interface{}, sendNotice bool) (*model.WorkflowRun, []string, []string, error) {
	if run == nil || run.Context == nil {
		return run, nil, nil, fmt.Errorf("workflow run not found")
	}
	if strings.TrimSpace(run.ChannelID) != "qq" {
		return run, nil, nil, fmt.Errorf("channel not supported for artifact delivery")
	}
	if len(selected) == 0 {
		return run, nil, nil, fmt.Errorf("no artifacts selected")
	}
	targetMode, targetID := rt.resolveArtifactDeliveryTarget(run)
	if targetMode == "" || targetID == "" {
		return run, nil, nil, fmt.Errorf("missing QQ delivery target")
	}
	sent := make([]string, 0, len(selected))
	failed := make([]string, 0)
	for _, item := range selected {
		absPath := strings.TrimSpace(toStringLocal(item["absolutePath"]))
		fileName := strings.TrimSpace(toStringLocal(item["fileName"]))
		name := strings.TrimSpace(toStringLocal(item["artifactName"]))
		if name == "" {
			name = fileName
		}
		var err error
		if targetMode == "group" {
			err = rt.sendQQGroupArtifact(targetID, absPath, fileName)
		} else {
			err = rt.sendQQPrivateArtifact(targetID, absPath, fileName)
		}
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		sent = append(sent, name)
	}
	rt.updateArtifactDeliveryState(run, selected, sent, failed, targetMode, targetID)
	updated, _ := model.GetWorkflowRun(rt.db, run.ID)
	if updated == nil {
		updated = run
	}
	if len(sent) > 0 {
		targetLabel := "QQ 私聊"
		if targetMode == "group" {
			targetLabel = "QQ 群聊"
		}
		rt.logWorkflowActivity(updated, "workflow.artifact.sent", fmt.Sprintf("工作流 %s %s 已向 %s 回传 %d 个文件", updated.Name, updated.ShortID, targetLabel, len(sent)), strings.Join(sent, "\n"))
	}
	if len(failed) > 0 {
		rt.logWorkflowActivity(updated, "workflow.artifact.send_failed", fmt.Sprintf("工作流 %s %s 有 %d 个文件回传失败", updated.Name, updated.ShortID, len(failed)), strings.Join(failed, "\n"))
	}
	if sendNotice {
		if notice := renderArtifactDeliveryNotice(updated, sent, failed, targetMode); notice != "" {
			if err := rt.sendWorkflowAck(updated.ChannelID, updated.ConversationID, updated.UserID, notice); err != nil {
				rt.logWorkflowActivity(updated, "workflow.artifact.notice_failed", fmt.Sprintf("工作流 %s %s 文件回传提示发送失败", updated.Name, updated.ShortID), err.Error())
			} else {
				rt.logWorkflowActivity(updated, "workflow.artifact.notice_sent", fmt.Sprintf("工作流 %s %s 已发送文件回传提示", updated.Name, updated.ShortID), notice)
			}
		}
	}
	return updated, sent, failed, nil
}

func (rt *workflowRuntime) deliverFinalArtifactsToOrigin(run *model.WorkflowRun) {
	if run == nil || run.Context == nil {
		return
	}
	if strings.TrimSpace(run.ChannelID) != "qq" {
		return
	}
	selected := selectedArtifactsForDelivery(run)
	if len(selected) == 0 {
		return
	}
	_, _, _, _ = rt.sendArtifactsToOrigin(run, selected, true)
}

func (rt *workflowRuntime) sendWecomText(responseURL, text string) error {
	responseURL = strings.TrimSpace(responseURL)
	if responseURL == "" {
		return fmt.Errorf("wecom response_url missing")
	}
	body, _ := json.Marshal(map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": text,
		},
	})
	req, err := http.NewRequest("POST", responseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wecom response_url send failed: http %d %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func (rt *workflowRuntime) sendFeishuText(chatID, text string) error {
	ocConfig, err := rt.cfg.ReadOpenClawJSON()
	if err != nil {
		return err
	}
	channels, _ := ocConfig["channels"].(map[string]interface{})
	feishu, _ := channels["feishu"].(map[string]interface{})
	appID := strings.TrimSpace(toStringLocal(feishu["appId"]))
	appSecret := strings.TrimSpace(toStringLocal(feishu["appSecret"]))
	if appID == "" || appSecret == "" {
		return fmt.Errorf("feishu app credentials not configured")
	}
	tenantToken, err := rt.fetchFeishuTenantToken(appID, appSecret)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]interface{}{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    fmt.Sprintf("{\"text\":%q}", text),
	})
	req, err := http.NewRequest("POST", "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tenantToken)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu send failed: http %d %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var result map[string]interface{}
	_ = json.Unmarshal(data, &result)
	if code, ok := result["code"].(float64); ok && code != 0 {
		return fmt.Errorf("feishu send failed: %v", result["msg"])
	}
	return nil
}

func (rt *workflowRuntime) fetchFeishuTenantToken(appID, appSecret string) (string, error) {
	body, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": appSecret})
	req, err := http.NewRequest("POST", "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("feishu token failed: http %d %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if code, ok := result["code"].(float64); ok && code != 0 {
		return "", fmt.Errorf("feishu token failed: %v", result["msg"])
	}
	token := strings.TrimSpace(toStringLocal(result["tenant_access_token"]))
	if token == "" {
		return "", fmt.Errorf("feishu tenant access token missing")
	}
	return token, nil
}

func (rt *workflowRuntime) generateAmbiguousReply(candidates []model.WorkflowRun) string {
	if len(candidates) == 0 {
		return "我这边没有找到可继续的工作流。"
	}
	items := make([]string, 0, len(candidates))
	limit := len(candidates)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		run := candidates[i]
		name := strings.TrimSpace(run.Name)
		if name == "" {
			name = "未命名工作流"
		}
		items = append(items, fmt.Sprintf("%s %s（%s）", run.ShortID, name, run.Status))
	}
	return "我这里同时挂着多个等待中的工作流，请直接回复对应编号继续，例如“继续 #A7K2”。当前可选：\n" + strings.Join(items, "\n")
}

func (rt *workflowRuntime) generateMatchedReply(run *model.WorkflowRun, action string) string {
	if run == nil {
		return "收到，我来继续处理。"
	}
	name := strings.TrimSpace(run.Name)
	if name == "" {
		name = "工作流"
	}
	switch action {
	case "pause":
		return fmt.Sprintf("好的，先把 %s %s 暂停。", name, run.ShortID)
	case "cancel":
		return fmt.Sprintf("好的，%s %s 我先取消掉。", name, run.ShortID)
	case "show_error":
		return fmt.Sprintf("我正在整理 %s %s 的错误信息。", name, run.ShortID)
	default:
		return fmt.Sprintf("收到，我继续推进 %s %s。", name, run.ShortID)
	}
}

func (rt *workflowRuntime) applyRunControl(run *model.WorkflowRun, action, reply, source string) (*model.WorkflowRun, string, error) {
	if run == nil {
		return nil, "", fmt.Errorf("workflow run not found")
	}
	resolved := strings.TrimSpace(action)
	if reply != "" {
		resolved = rt.classifyUserIntent(run, reply)
	}
	if resolved == "" {
		resolved = "ignore"
	}
	switch resolved {
	case "pause":
		run.Status = "paused"
		run.LastMessage = reply
	case "cancel":
		run.Status = "cancelled"
		run.LastMessage = reply
	case "show_error":
		// no status change
	case "retry", "resume", "modify_input", "approve", "continue":
		run.Status = "running"
		if run.Context == nil {
			run.Context = map[string]interface{}{}
		}
		if reply != "" {
			run.Context["latestUserReply"] = reply
		}
	case "ignore":
		return run, resolved, nil
	default:
		run.Status = "waiting_for_user"
	}
	if err := model.UpdateWorkflowRun(rt.db, run); err != nil {
		return nil, resolved, err
	}
	rt.broadcastWorkflowTaskUpdate(run)
	_ = model.AddWorkflowEvent(rt.db, &model.WorkflowEvent{RunID: run.ID, EventType: "user_intent", Message: resolved, Payload: map[string]interface{}{"reply": reply, "source": source}})
	rt.logWorkflowActivity(run, "workflow.run.control", fmt.Sprintf("工作流 %s %s 收到控制：%s", run.Name, run.ShortID, resolved), reply)
	rt.broadcastWorkflowTaskLog(run.ID, fmt.Sprintf("💬 收到控制指令：%s", resolved))
	if resolved == "retry" || resolved == "resume" || resolved == "modify_input" || resolved == "approve" || resolved == "continue" {
		steps, _ := model.ListWorkflowSteps(rt.db, run.ID)
		for i := range steps {
			if steps[i].Status == "blocked" || steps[i].Status == "failed" {
				steps[i].ErrorText = ""
				if reply != "" {
					if steps[i].Input == nil {
						steps[i].Input = map[string]interface{}{}
					}
					steps[i].Input["userReply"] = reply
				}
				switch steps[i].StepType {
				case "approval":
					steps[i].Status = "completed"
					if reply != "" {
						steps[i].OutputText = reply
					} else {
						steps[i].OutputText = "approved"
					}
				case "wait_user", "input":
					if strings.TrimSpace(reply) != "" {
						steps[i].Status = "completed"
						steps[i].OutputText = reply
					} else {
						steps[i].Status = "pending"
					}
				default:
					steps[i].Status = "pending"
				}
				_ = model.UpdateWorkflowStep(rt.db, &steps[i])
				break
			}
		}
		go rt.executeRun(run.ID)
	}
	updated, _ := model.GetWorkflowRun(rt.db, run.ID)
	return updated, resolved, nil
}

func extractMentionedWorkflowShortID(text string) string {
	matches := workflowShortIDPattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return "#" + strings.ToUpper(matches[1])
}

func (rt *workflowRuntime) instantiateRun(tpl *model.WorkflowTemplate, input, channelID, conversationID, userID, sourceMessage string, extra map[string]interface{}) (*model.WorkflowRun, []model.WorkflowStep, error) {
	runID := model.NewWorkflowRunID()
	shortID := model.GenerateWorkflowShortID(runID)
	workflowDir, workflowDirRel, err := rt.ensureWorkflowRunDir(channelID, userID, shortID)
	if err != nil {
		return nil, nil, err
	}
	ctx := map[string]interface{}{
		"initialInput":        input,
		"artifacts":           map[string]interface{}{},
		"timeline":            []interface{}{},
		"workflowDir":         workflowDir,
		"workflowDirRelative": workflowDirRel,
		"artifactFiles":       []map[string]interface{}{},
		"finalFiles":          []map[string]interface{}{},
	}
	for k, v := range extra {
		ctx[k] = v
	}
	run := &model.WorkflowRun{ID: runID, ShortID: shortID, TemplateID: tpl.ID, Name: tpl.Name, Status: "running", ChannelID: channelID, ConversationID: conversationID, UserID: userID, SourceMessage: sourceMessage, Settings: tpl.Settings, Context: ctx}
	nodes := model.ExtractWorkflowNodes(tpl.Definition)
	steps := make([]model.WorkflowStep, 0, len(nodes))
	for i, node := range nodes {
		stepType := strings.TrimSpace(toStringLocal(node["type"]))
		if stepType == "" {
			stepType = "ai_task"
		}
		step := model.WorkflowStep{ID: model.NewWorkflowStepID(), RunID: runID, StepKey: strings.TrimSpace(toStringLocal(node["id"])), Title: strings.TrimSpace(toStringLocal(node["title"])), StepType: stepType, Status: "pending", OrderIndex: i, Input: node}
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return nil, nil, fmt.Errorf("workflow 模板没有节点")
	}
	return run, steps, nil
}

func (rt *workflowRuntime) workflowWorkspaceRoot() string {
	root := strings.TrimSpace(rt.cfg.OpenClawWork)
	if root == "" {
		root = filepath.Join(filepath.Dir(rt.cfg.OpenClawDir), "work")
	}
	_ = os.MkdirAll(root, 0755)
	return root
}

func sanitizeWorkflowPathSegment(value, fallback string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "#", ""))
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_' || r == '.':
			return r
		default:
			return -1
		}
	}, value)
	if value == "" {
		return fallback
	}
	return value
}

func sanitizeArtifactFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "artifact.md"
	}
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "..", "-")
	return name
}

func defaultArtifactFileName(step *model.WorkflowStep) string {
	ext := ".md"
	if format := strings.ToLower(strings.TrimSpace(toStringLocal(step.Input["outputFormat"]))); format == "json" {
		ext = ".json"
	} else if format == "text" || format == "txt" {
		ext = ".txt"
	}
	name := sanitizeWorkflowPathSegment(step.Title, step.StepKey)
	if name == "" {
		name = sanitizeWorkflowPathSegment(step.StepKey, "step")
	}
	return fmt.Sprintf("%02d-%s%s", step.OrderIndex+1, name, ext)
}

func rawOutputFileSetting(step *model.WorkflowStep) interface{} {
	if step == nil || step.Input == nil {
		return nil
	}
	return step.Input["outputFile"]
}

func explicitArtifactFileName(step *model.WorkflowStep) string {
	if step == nil || step.Input == nil {
		return ""
	}
	if s, ok := step.Input["outputFile"].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func (rt *workflowRuntime) ensureWorkflowRunDir(channelID, userID, shortID string) (string, string, error) {
	root := rt.workflowWorkspaceRoot()
	rel := filepath.Join("workflows", sanitizeWorkflowPathSegment(channelID, "unknown-channel"), sanitizeWorkflowPathSegment(userID, "unknown-user"), sanitizeWorkflowPathSegment(shortID, "workflow"))
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", "", err
	}
	return abs, rel, nil
}

func stepShouldPersistArtifact(step *model.WorkflowStep) bool {
	if step == nil {
		return false
	}
	switch raw := rawOutputFileSetting(step).(type) {
	case bool:
		return raw
	case string:
		if strings.TrimSpace(raw) != "" {
			return true
		}
	}
	switch step.StepType {
	case "ai_plan", "ai_task", "publish", "summary", "analyze":
		return true
	default:
		return false
	}
}

func (rt *workflowRuntime) persistStepArtifact(run *model.WorkflowRun, step *model.WorkflowStep) error {
	if run == nil || step == nil || !stepShouldPersistArtifact(step) || strings.TrimSpace(step.OutputText) == "" {
		return nil
	}
	if run.Context == nil {
		run.Context = map[string]interface{}{}
	}
	dir := strings.TrimSpace(toStringLocal(run.Context["workflowDir"]))
	relDir := strings.TrimSpace(toStringLocal(run.Context["workflowDirRelative"]))
	if dir == "" {
		var err error
		dir, relDir, err = rt.ensureWorkflowRunDir(run.ChannelID, run.UserID, run.ShortID)
		if err != nil {
			return err
		}
		run.Context["workflowDir"] = dir
		run.Context["workflowDirRelative"] = relDir
	}
	fileName := explicitArtifactFileName(step)
	if fileName == "" {
		fileName = defaultArtifactFileName(step)
	}
	fileName = sanitizeArtifactFileName(fileName)
	absPath := filepath.Join(dir, fileName)
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(absPath, []byte(step.OutputText), 0644); err != nil {
		return err
	}
	artifactName := strings.TrimSpace(toStringLocal(step.Input["artifactName"]))
	if artifactName == "" {
		artifactName = step.Title
	}
	relPath := filepath.ToSlash(filepath.Join(relDir, fileName))
	record := map[string]interface{}{
		"stepKey":       step.StepKey,
		"title":         step.Title,
		"artifactName":  artifactName,
		"fileName":      fileName,
		"relativePath":  relPath,
		"absolutePath":  absPath,
		"isFinalOutput": toBoolLocal(step.Input["isFinalOutput"]),
		"sendToUser":    toBoolLocal(step.Input["sendToUser"]),
		"updatedAt":     time.Now().UnixMilli(),
	}
	files := toArtifactSlice(run.Context["artifactFiles"])
	replaced := false
	for i := range files {
		if strings.TrimSpace(toStringLocal(files[i]["stepKey"])) == step.StepKey {
			files[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		files = append(files, record)
	}
	sort.Slice(files, func(i, j int) bool {
		return strings.TrimSpace(toStringLocal(files[i]["fileName"])) < strings.TrimSpace(toStringLocal(files[j]["fileName"]))
	})
	run.Context["artifactFiles"] = files
	finalFiles := make([]map[string]interface{}, 0)
	for _, item := range files {
		if toBoolLocal(item["isFinalOutput"]) {
			finalFiles = append(finalFiles, item)
		}
	}
	if len(finalFiles) == 0 && len(files) > 0 {
		finalFiles = append(finalFiles, files[len(files)-1])
	}
	run.Context["finalFiles"] = finalFiles
	return model.UpdateWorkflowRun(rt.db, run)
}

func toArtifactSlice(v interface{}) []map[string]interface{} {
	items := []map[string]interface{}{}
	switch raw := v.(type) {
	case []map[string]interface{}:
		items = append(items, raw...)
	case []interface{}:
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				items = append(items, m)
			}
		}
	}
	return items
}

func toBoolLocal(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(strings.TrimSpace(b), "true") || strings.EqualFold(strings.TrimSpace(b), "yes")
	case float64:
		return b != 0
	case int:
		return b != 0
	default:
		return false
	}
}

func (rt *workflowRuntime) executeRun(runID string) {
	rt.mu.Lock()
	if rt.running[runID] {
		rt.mu.Unlock()
		return
	}
	rt.running[runID] = true
	rt.mu.Unlock()
	defer func() {
		rt.mu.Lock()
		delete(rt.running, runID)
		rt.mu.Unlock()
	}()

	run, err := model.GetWorkflowRun(rt.db, runID)
	if err != nil {
		return
	}
	rt.broadcastWorkflowTaskUpdate(run)
	for idx := range run.Steps {
		step := run.Steps[idx]
		if step.Status == "completed" || step.Status == "skipped" {
			continue
		}
		if step.Status == "blocked" && (run.Status == "waiting_for_user" || run.Status == "waiting_for_approval" || run.Status == "paused") {
			return
		}
		step.Status = "in_progress"
		_ = model.UpdateWorkflowStep(rt.db, &step)
		progressMsg := rt.generateProgressMessage(run, &step, "start")
		run.LastMessage = progressMsg
		run.Status = "running"
		_ = model.UpdateWorkflowRun(rt.db, run)
		_ = model.AddWorkflowEvent(rt.db, &model.WorkflowEvent{RunID: run.ID, StepID: step.ID, EventType: "progress", Message: progressMsg, Payload: map[string]interface{}{"phase": "start"}})
		rt.logWorkflowActivity(run, "workflow.step.started", fmt.Sprintf("工作流 %s %s 开始步骤：%s", run.Name, run.ShortID, step.Title), progressMsg)
		rt.broadcastWorkflowTaskUpdate(run)
		rt.broadcastWorkflowTaskLog(run.ID, progressMsg)
		rt.pushRunMessageToOrigin(run, progressMsg)

		switch step.StepType {
		case "approval":
			step.Status = "blocked"
			step.NeedsApproval = true
			_ = model.UpdateWorkflowStep(rt.db, &step)
			run.Status = "waiting_for_approval"
			run.LastMessage = rt.generateProgressMessage(run, &step, "approval")
			_ = model.UpdateWorkflowRun(rt.db, run)
			_ = model.AddWorkflowEvent(rt.db, &model.WorkflowEvent{RunID: run.ID, StepID: step.ID, EventType: "waiting_for_approval", Message: run.LastMessage})
			rt.logWorkflowActivity(run, "workflow.step.waiting_approval", fmt.Sprintf("工作流 %s %s 等待审批：%s", run.Name, run.ShortID, step.Title), run.LastMessage)
			rt.broadcastWorkflowTaskUpdate(run)
			rt.broadcastWorkflowTaskLog(run.ID, run.LastMessage)
			rt.pushRunMessageToOrigin(run, run.LastMessage)
			return
		case "wait_user", "input":
			userReply := strings.TrimSpace(toStringLocal(step.Input["userReply"]))
			if userReply == "" && run.Context != nil {
				userReply = strings.TrimSpace(toStringLocal(run.Context["latestUserReply"]))
			}
			if userReply != "" {
				step.Status = "completed"
				step.OutputText = userReply
				if step.Input == nil {
					step.Input = map[string]interface{}{}
				}
				step.Input["userReply"] = userReply
				_ = model.UpdateWorkflowStep(rt.db, &step)
				if run.Context == nil {
					run.Context = map[string]interface{}{}
				}
				if artifacts, ok := run.Context["artifacts"].(map[string]interface{}); ok {
					artifacts[step.StepKey] = userReply
				}
				run.Context["latestUserReply"] = ""
				break
			}
			step.Status = "blocked"
			_ = model.UpdateWorkflowStep(rt.db, &step)
			run.Status = "waiting_for_user"
			run.LastMessage = rt.generateProgressMessage(run, &step, "wait_user")
			_ = model.UpdateWorkflowRun(rt.db, run)
			_ = model.AddWorkflowEvent(rt.db, &model.WorkflowEvent{RunID: run.ID, StepID: step.ID, EventType: "waiting_for_user", Message: run.LastMessage})
			rt.logWorkflowActivity(run, "workflow.step.waiting_user", fmt.Sprintf("工作流 %s %s 等待用户输入：%s", run.Name, run.ShortID, step.Title), run.LastMessage)
			rt.broadcastWorkflowTaskUpdate(run)
			rt.broadcastWorkflowTaskLog(run.ID, run.LastMessage)
			rt.pushRunMessageToOrigin(run, run.LastMessage)
			return
		case "end":
			step.Status = "completed"
			step.OutputText = "Workflow completed"
			_ = model.UpdateWorkflowStep(rt.db, &step)
		default:
			result, aiErr := rt.executeAIStep(run, &step)
			if aiErr != nil {
				step.Status = "failed"
				step.ErrorText = aiErr.Error()
				_ = model.UpdateWorkflowStep(rt.db, &step)
				run.Status = "waiting_for_user"
				run.LastMessage = rt.generateFailureMessage(run, &step, aiErr)
				_ = model.UpdateWorkflowRun(rt.db, run)
				_ = model.AddWorkflowEvent(rt.db, &model.WorkflowEvent{RunID: run.ID, StepID: step.ID, EventType: "failed", Message: run.LastMessage, Payload: map[string]interface{}{"error": aiErr.Error()}})
				rt.logWorkflowActivity(run, "workflow.step.failed", fmt.Sprintf("工作流 %s %s 步骤失败：%s", run.Name, run.ShortID, step.Title), aiErr.Error())
				rt.broadcastWorkflowTaskUpdate(run)
				rt.broadcastWorkflowTaskLog(run.ID, run.LastMessage)
				rt.broadcastWorkflowTaskLog(run.ID, fmt.Sprintf("❌ %s", aiErr.Error()))
				rt.pushRunMessageToOrigin(run, run.LastMessage)
				return
			}
			step.Status = "completed"
			step.OutputText = result
			_ = model.UpdateWorkflowStep(rt.db, &step)
			if artifacts, ok := run.Context["artifacts"].(map[string]interface{}); ok {
				artifacts[step.StepKey] = result
			}
		}
		_ = rt.persistStepArtifact(run, &step)
		progressMsg = rt.generateProgressMessage(run, &step, "completed")
		run.LastMessage = progressMsg
		_ = model.UpdateWorkflowRun(rt.db, run)
		_ = model.AddWorkflowEvent(rt.db, &model.WorkflowEvent{RunID: run.ID, StepID: step.ID, EventType: "progress", Message: progressMsg, Payload: map[string]interface{}{"phase": "completed"}})
		rt.logWorkflowActivity(run, "workflow.step.completed", fmt.Sprintf("工作流 %s %s 完成步骤：%s", run.Name, run.ShortID, step.Title), progressMsg)
		rt.broadcastWorkflowTaskUpdate(run)
		rt.broadcastWorkflowTaskLog(run.ID, progressMsg)
		rt.pushRunMessageToOrigin(run, progressMsg)
		run, _ = model.GetWorkflowRun(rt.db, run.ID)
	}
	run.Status = "completed"
	run.LastMessage = rt.generateRunCompletionMessage(run)
	if artifactSummary := rt.renderFinalArtifactSummary(run); artifactSummary != "" {
		run.LastMessage = strings.TrimSpace(run.LastMessage + "\n\n" + artifactSummary)
	}
	_ = model.UpdateWorkflowRun(rt.db, run)
	_ = model.AddWorkflowEvent(rt.db, &model.WorkflowEvent{RunID: run.ID, EventType: "completed", Message: run.LastMessage})
	rt.logWorkflowActivity(run, "workflow.run.completed", fmt.Sprintf("工作流 %s %s 已完成", run.Name, run.ShortID), run.LastMessage)
	rt.broadcastWorkflowTaskUpdate(run)
	rt.broadcastWorkflowTaskLog(run.ID, run.LastMessage)
	rt.pushRunMessageToOrigin(run, run.LastMessage)
	rt.deliverFinalArtifactsToOrigin(run)
}

func (rt *workflowRuntime) findRecentAutoCreatedRun(channelID, conversationID, sourceMessage string, window time.Duration) *model.WorkflowRun {
	runs, err := model.ListWorkflowRuns(rt.db, "")
	if err != nil {
		return nil
	}
	cutoff := time.Now().Add(-window).UnixMilli()
	trimmedMessage := strings.TrimSpace(sourceMessage)
	for _, item := range runs {
		if item.CreatedAt < cutoff {
			continue
		}
		if item.ChannelID != channelID || item.ConversationID != conversationID {
			continue
		}
		if strings.TrimSpace(item.SourceMessage) != trimmedMessage {
			continue
		}
		if item.Context == nil || item.Context["autoCreated"] != true {
			continue
		}
		run, err := model.GetWorkflowRun(rt.db, item.ID)
		if err == nil {
			return run
		}
	}
	return nil
}

func (rt *workflowRuntime) logWorkflowActivity(run *model.WorkflowRun, eventType, summary, detail string) {
	if rt.db == nil || run == nil {
		return
	}
	event := &model.Event{
		Time:    time.Now().UnixMilli(),
		Source:  "workflow",
		Type:    eventType,
		Summary: summary,
		Detail:  detail,
	}
	id, err := model.AddEvent(rt.db, event)
	if err != nil {
		return
	}
	event.ID = id
	if rt.hub == nil {
		return
	}
	entry := map[string]interface{}{
		"id":      event.ID,
		"time":    event.Time,
		"source":  event.Source,
		"type":    event.Type,
		"summary": event.Summary,
		"detail":  event.Detail,
	}
	if payload, err := json.Marshal(map[string]interface{}{"type": "log-entry", "data": entry}); err == nil {
		rt.hub.Broadcast(payload)
	}
}

func (rt *workflowRuntime) broadcastWorkflowTaskUpdate(run *model.WorkflowRun) {
	if rt.hub == nil || run == nil {
		return
	}
	totalSteps := len(run.Steps)
	completed := 0
	for _, step := range run.Steps {
		switch step.Status {
		case "completed", "skipped":
			completed++
		}
	}
	progress := 0
	if totalSteps > 0 {
		progress = int(float64(completed) / float64(totalSteps) * 100)
	}
	if run.Status == "completed" {
		progress = 100
	}
	status := workflowRunStatusToTaskStatus(run.Status)
	msg := map[string]interface{}{
		"type": "task_update",
		"task": map[string]interface{}{
			"id":        workflowTaskID(run.ID),
			"name":      fmt.Sprintf("%s %s", run.Name, run.ShortID),
			"type":      workflowTaskType,
			"status":    status,
			"progress":  progress,
			"error":     "",
			"createdAt": time.UnixMilli(run.CreatedAt).Format(time.RFC3339),
			"updatedAt": time.UnixMilli(run.UpdatedAt).Format(time.RFC3339),
			"logCount":  0,
		},
	}
	if run.Status == "failed" {
		msg["task"].(map[string]interface{})["error"] = run.LastMessage
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	rt.hub.Broadcast(data)
}

func (rt *workflowRuntime) broadcastWorkflowTaskLog(runID, line string) {
	if rt.hub == nil || strings.TrimSpace(runID) == "" || strings.TrimSpace(line) == "" {
		return
	}
	data, err := json.Marshal(map[string]interface{}{
		"type":   "task_log",
		"taskId": workflowTaskID(runID),
		"line":   line,
	})
	if err != nil {
		return
	}
	rt.hub.Broadcast(data)
}

func workflowTaskID(runID string) string {
	return "workflow-" + runID
}

func workflowRunStatusToTaskStatus(status string) string {
	switch status {
	case "completed":
		return "success"
	case "failed":
		return "failed"
	case "cancelled":
		return "canceled"
	case "paused", "waiting_for_user", "waiting_for_approval", "running":
		return "running"
	default:
		return "pending"
	}
}

func (rt *workflowRuntime) executeAIStep(run *model.WorkflowRun, step *model.WorkflowStep) (string, error) {
	skillPrompt := ""
	skillID := strings.TrimSpace(toStringLocal(step.Input["skill"]))
	if skillID != "" && !strings.EqualFold(skillID, "none") {
		if skillContent, err := rt.loadSkillInstruction(skillID); err == nil && strings.TrimSpace(skillContent) != "" {
			skillPrompt = fmt.Sprintf("\n请优先使用 skill `%s` 的方法完成这个步骤。以下是该 skill 的说明：\n%s\n", skillID, skillContent)
		} else {
			skillPrompt = fmt.Sprintf("\n请优先按 skill `%s` 的能力和约束来完成这个步骤；如果找不到该 skill，请在结果中按该 skill 目标自行完成。\n", skillID)
		}
	}
	executionContext := buildStepExecutionContext(run, step)
	constraint := stepOutputConstraint(step)
	prompt := fmt.Sprintf("你是工作流执行器。当前工作流：%s (%s)。\n步骤标题：%s\n步骤类型：%s\n步骤配置：%s\n当前上下文摘要：%s%s%s\n请直接输出本步骤的执行结果正文，尽量使用 Markdown 组织内容，不要添加额外解释。", run.Name, run.ShortID, step.Title, step.StepType, mustJSON(step.Input), mustJSON(executionContext), skillPrompt, constraint)
	text, _, err := rt.callWorkflowModel([]map[string]string{{"role": "system", "content": "你正在执行一个业务工作流节点，请产出这个节点的结果，输出结果本身，不要包多余解释。"}, {"role": "user", "content": prompt}})
	if err != nil {
		if shouldRetryWithCompactContext(err) {
			compactContext := buildCompactStepExecutionContext(run, step)
			compactPrompt := fmt.Sprintf("你是工作流执行器。当前工作流：%s (%s)。\n步骤标题：%s\n步骤类型：%s\n步骤配置：%s\n精简上下文：%s%s%s\n上一次请求疑似因超时失败，这次请基于精简上下文直接输出本步骤结果正文，尽量精炼但保持可执行。不要添加额外解释。", run.Name, run.ShortID, step.Title, step.StepType, mustJSON(step.Input), mustJSON(compactContext), skillPrompt, constraint)
			text, _, retryErr := rt.callWorkflowModel([]map[string]string{{"role": "system", "content": "你正在执行一个业务工作流节点。由于上一次请求超时，这次请在保留关键信息的前提下更紧凑地输出结果。只输出结果正文。"}, {"role": "user", "content": compactPrompt}})
			if retryErr == nil {
				return strings.TrimSpace(text), nil
			}
			return "", retryErr
		}
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func trimWorkflowContextValue(v interface{}, limit int) interface{} {
	if limit <= 0 {
		limit = 1200
	}
	switch value := v.(type) {
	case string:
		value = strings.TrimSpace(value)
		runes := []rune(value)
		if len(runes) > limit {
			return string(runes[:limit]) + "...(truncated)"
		}
		return value
	case []interface{}:
		maxItems := 8
		out := make([]interface{}, 0, minInt(len(value), maxItems))
		for i, item := range value {
			if i >= maxItems {
				out = append(out, fmt.Sprintf("... %d more items", len(value)-maxItems))
				break
			}
			out = append(out, trimWorkflowContextValue(item, limit/2))
		}
		return out
	case map[string]interface{}:
		out := map[string]interface{}{}
		keys := make([]string, 0, len(value))
		for k := range value {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i >= 24 {
				out["__truncated__"] = fmt.Sprintf("%d more keys", len(keys)-24)
				break
			}
			out[k] = trimWorkflowContextValue(value[k], limit/2)
		}
		return out
	default:
		return v
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func shouldRetryWithCompactContext(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "504") || strings.Contains(text, "timeout") || strings.Contains(text, "deadline exceeded") || strings.Contains(text, "请求超时")
}

func stepOutputConstraint(step *model.WorkflowStep) string {
	if step == nil {
		return ""
	}
	switch step.StepType {
	case "ai_plan":
		return "\n输出要求：请优先给出紧凑、可执行的方案，控制在 6 个以内一级小节，尽量不超过 1200 字。"
	case "summary", "publish":
		return "\n输出要求：请保留交付完整度，但尽量控制篇幅，避免不必要铺垫。"
	case "analyze":
		return "\n输出要求：只保留关键分析结论、依据和建议，避免重复展开。"
	default:
		return ""
	}
}

func buildStepExecutionContext(run *model.WorkflowRun, step *model.WorkflowStep) map[string]interface{} {
	ctx := map[string]interface{}{}
	if run == nil {
		return ctx
	}
	if run.Context != nil {
		for _, key := range []string{"initialInput", "latestUserReply", "workflowDirRelative", "originConversationId", "originChannelId", "sourceMessage"} {
			if val, ok := run.Context[key]; ok {
				ctx[key] = trimWorkflowContextValue(val, 1200)
			}
		}
		if artifacts, ok := run.Context["artifacts"].(map[string]interface{}); ok {
			summary := map[string]interface{}{}
			keys := make([]string, 0, len(artifacts))
			for k := range artifacts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if step != nil && k == step.StepKey {
					continue
				}
				summary[k] = trimWorkflowContextValue(artifacts[k], 800)
			}
			ctx["artifacts"] = summary
		}
		if files := toArtifactSlice(run.Context["artifactFiles"]); len(files) > 0 {
			fileSummaries := make([]map[string]interface{}, 0, minInt(len(files), 8))
			for i, item := range files {
				if i >= 8 {
					break
				}
				fileSummaries = append(fileSummaries, map[string]interface{}{
					"stepKey":      toStringLocal(item["stepKey"]),
					"artifactName": toStringLocal(item["artifactName"]),
					"fileName":     toStringLocal(item["fileName"]),
				})
			}
			ctx["artifactFiles"] = fileSummaries
		}
	}
	if run.Steps != nil {
		stepSummaries := make([]map[string]interface{}, 0, len(run.Steps))
		for _, item := range run.Steps {
			entry := map[string]interface{}{
				"stepKey": item.StepKey,
				"title":   item.Title,
				"type":    item.StepType,
				"status":  item.Status,
			}
			if item.OutputText != "" && (step == nil || item.StepKey != step.StepKey) {
				entry["output"] = trimWorkflowContextValue(item.OutputText, 600)
			}
			if item.ErrorText != "" {
				entry["error"] = trimWorkflowContextValue(item.ErrorText, 300)
			}
			stepSummaries = append(stepSummaries, entry)
		}
		ctx["steps"] = stepSummaries
	}
	return ctx
}

func buildCompactStepExecutionContext(run *model.WorkflowRun, step *model.WorkflowStep) map[string]interface{} {
	ctx := map[string]interface{}{}
	if run == nil {
		return ctx
	}
	if run.Context != nil {
		for _, key := range []string{"initialInput", "latestUserReply", "sourceMessage"} {
			if val, ok := run.Context[key]; ok {
				ctx[key] = trimWorkflowContextValue(val, 500)
			}
		}
		if artifacts, ok := run.Context["artifacts"].(map[string]interface{}); ok {
			keys := make([]string, 0, len(artifacts))
			for k := range artifacts {
				if step != nil && k == step.StepKey {
					continue
				}
				keys = append(keys, k)
			}
			sort.Strings(keys)
			compact := map[string]interface{}{}
			start := 0
			if len(keys) > 3 {
				start = len(keys) - 3
			}
			for _, k := range keys[start:] {
				compact[k] = trimWorkflowContextValue(artifacts[k], 300)
			}
			ctx["recentArtifacts"] = compact
		}
	}
	if run.Steps != nil {
		recent := make([]map[string]interface{}, 0, 4)
		for _, item := range run.Steps {
			if item.Status != "completed" || (step != nil && item.StepKey == step.StepKey) {
				continue
			}
			recent = append(recent, map[string]interface{}{
				"stepKey": item.StepKey,
				"title":   item.Title,
				"output":  trimWorkflowContextValue(item.OutputText, 240),
			})
		}
		if len(recent) > 4 {
			recent = recent[len(recent)-4:]
		}
		ctx["recentSteps"] = recent
	}
	return ctx
}

func (rt *workflowRuntime) loadSkillInstruction(skillID string) (string, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return "", nil
	}
	candidates := []string{
		filepath.Join(rt.cfg.OpenClawDir, "skills", skillID, "SKILL.md"),
		filepath.Join(rt.cfg.OpenClawWork, "skills", skillID, "SKILL.md"),
		filepath.Join(rt.cfg.OpenClawApp, "skills", skillID, "SKILL.md"),
	}
	for _, p := range candidates {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if data, err := os.ReadFile(p); err == nil {
			content := strings.TrimSpace(string(data))
			if len(content) > 6000 {
				content = content[:6000]
			}
			return content, nil
		}
	}
	return "", fmt.Errorf("skill not found: %s", skillID)
}

func (rt *workflowRuntime) generateProgressMessage(run *model.WorkflowRun, step *model.WorkflowStep, phase string) string {
	prompt := fmt.Sprintf("请为用户生成一条自然的工作流进度消息。工作流名称：%s，短 ID：%s，当前步骤：%s，阶段：%s。要求简洁、自然、像真人运营助手，不要死板模板，但必须提到工作流名称和短 ID。", run.Name, run.ShortID, step.Title, phase)
	text, _, err := rt.callWorkflowModel([]map[string]string{{"role": "system", "content": "你是工作流进度播报助手。消息要简洁自然。"}, {"role": "user", "content": prompt}})
	if err == nil && strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}
	return fmt.Sprintf("%s %s：当前步骤 %s（%s）", run.Name, run.ShortID, step.Title, phase)
}

func (rt *workflowRuntime) generateFailureMessage(run *model.WorkflowRun, step *model.WorkflowStep, err error) string {
	prompt := fmt.Sprintf("请为用户生成一条工作流失败后的消息。工作流：%s %s，失败步骤：%s，错误：%s。告诉用户可以重试、查看错误、先放着或取消。不要追加额外寒暄，不要问用户是否要改写消息，不要输出这四项之外的新建议。", run.Name, run.ShortID, step.Title, err.Error())
	text, _, aiErr := rt.callWorkflowModel([]map[string]string{{"role": "system", "content": "你是工作流失败处理助手，消息要友好清晰、简洁克制。严格只输出失败说明和 4 个可选操作，不要添加额外结尾。"}, {"role": "user", "content": prompt}})
	if aiErr == nil && strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}
	return fmt.Sprintf("%s %s：步骤“%s”执行失败。你可以回复“重试”、“查看错误”或“先放着”。", run.Name, run.ShortID, step.Title)
}

func (rt *workflowRuntime) generateRunCompletionMessage(run *model.WorkflowRun) string {
	prompt := fmt.Sprintf("请为用户生成一条工作流完成消息。工作流：%s %s。要简洁、正向，告诉用户已经完成。", run.Name, run.ShortID)
	text, _, err := rt.callWorkflowModel([]map[string]string{{"role": "system", "content": "你是工作流完成通知助手。"}, {"role": "user", "content": prompt}})
	if err == nil && strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}
	return fmt.Sprintf("%s %s：工作流已完成。", run.Name, run.ShortID)
}

func (rt *workflowRuntime) classifyUserIntent(run *model.WorkflowRun, reply string) string {
	prompt := fmt.Sprintf("请判断下面这条用户回复更接近哪种工作流意图，只能从这些标签中选一个输出：retry,show_error,pause,resume,cancel,modify_input,continue,ignore。\n工作流：%s %s\n状态：%s\n用户回复：%s", run.Name, run.ShortID, run.Status, reply)
	text, _, err := rt.callWorkflowModel([]map[string]string{{"role": "system", "content": "你是工作流用户意图分类器，只输出标签本身。"}, {"role": "user", "content": prompt}})
	if err == nil {
		label := strings.ToLower(strings.TrimSpace(text))
		switch label {
		case "retry", "show_error", "pause", "resume", "cancel", "modify_input", "continue", "ignore":
			return label
		}
	}
	lower := strings.ToLower(reply)
	switch {
	case strings.Contains(lower, "重试"):
		return "retry"
	case strings.Contains(lower, "错误"):
		return "show_error"
	case strings.Contains(lower, "先放") || strings.Contains(lower, "暂停"):
		return "pause"
	case strings.Contains(lower, "继续") || strings.Contains(lower, "恢复") || strings.Contains(lower, "同意"):
		return "continue"
	case strings.Contains(lower, "取消"):
		return "cancel"
	default:
		return "modify_input"
	}
}

func (rt *workflowRuntime) generateTemplateByAI(prompt, category string, settings map[string]interface{}) (*model.WorkflowTemplate, string, error) {
	workflowPrompt := fmt.Sprintf("请根据这段需求生成一个工作流模板 JSON。需求：%s。JSON 结构必须是 {name,description,category,status,triggerMode,settings,definition:{nodes,edges}}。nodes 里每个节点至少包含 id,title,type,order。节点还应该尽量补齐 skill,outputFile,artifactName,outputFormat,isFinalOutput,sendToUser。特别注意：outputFile 如果需要落文件，必须是字符串文件名，例如 \"01-活动方案.md\"，不要输出 true/false；如果不需要落文件，就填空字符串或不要这个字段。skill 默认填 none。类型只允许 input,ai_plan,ai_task,approval,wait_user,publish,analyze,summary,end。不要输出 markdown 代码块。", prompt)
	text, _, err := rt.callWorkflowModel([]map[string]string{{"role": "system", "content": "你是工作流模板规划器，只输出合法 JSON。"}, {"role": "user", "content": workflowPrompt}})
	if err != nil {
		fallback := rt.fallbackGeneratedTemplate(prompt, category, settings)
		return fallback, "", nil
	}
	raw := strings.TrimSpace(stripCodeFence(text))
	var tpl model.WorkflowTemplate
	if json.Unmarshal([]byte(raw), &tpl) != nil || tpl.Name == "" {
		fallback := rt.fallbackGeneratedTemplate(prompt, category, settings)
		return fallback, raw, nil
	}
	if tpl.ID == "" {
		tpl.ID = model.NewWorkflowTemplateID()
	}
	if tpl.Category == "" {
		tpl.Category = category
	}
	if tpl.Status == "" {
		tpl.Status = "ready"
	}
	if tpl.TriggerMode == "" {
		tpl.TriggerMode = "manual"
	}
	if len(tpl.Settings) == 0 {
		tpl.Settings = settings
	}
	tpl.Definition = model.NormalizeWorkflowTemplateDefinition(tpl.Definition)
	return &tpl, raw, nil
}

func (rt *workflowRuntime) fallbackGeneratedTemplate(prompt, category string, settings map[string]interface{}) *model.WorkflowTemplate {
	return &model.WorkflowTemplate{
		ID:          model.NewWorkflowTemplateID(),
		Name:        "AI 生成工作流",
		Description: prompt,
		Category:    category,
		Status:      "ready",
		TriggerMode: "manual",
		Settings:    settings,
		Definition: map[string]interface{}{
			"nodes": []map[string]interface{}{
				{"id": "plan", "title": "拆解任务", "type": "ai_plan", "order": 1, "skill": "none", "outputFile": "01-task-plan.md", "artifactName": "任务拆解", "outputFormat": "markdown"},
				{"id": "draft", "title": "生成初稿", "type": "ai_task", "order": 2, "skill": "none", "outputFile": "02-first-draft.md", "artifactName": "执行初稿", "outputFormat": "markdown"},
				{"id": "review", "title": "关键节点确认", "type": "approval", "order": 3, "skill": "none"},
				{"id": "execute", "title": "执行输出", "type": "publish", "order": 4, "skill": "none", "outputFile": "03-deliverable.md", "artifactName": "执行产出", "outputFormat": "markdown", "isFinalOutput": true},
				{"id": "summary", "title": "总结结果", "type": "summary", "order": 5, "skill": "none", "outputFile": "04-summary.md", "artifactName": "结果总结", "outputFormat": "markdown", "isFinalOutput": true, "sendToUser": true},
				{"id": "end", "title": "完成", "type": "end", "order": 6},
			},
			"edges": []map[string]interface{}{},
		},
	}
}

func (rt *workflowRuntime) callWorkflowModel(messages []map[string]string) (string, map[string]string, error) {
	settings, err := model.GetWorkflowSettings(rt.db)
	if err != nil {
		return "", nil, err
	}
	ocConfig, _ := rt.cfg.ReadOpenClawJSON()
	if ocConfig == nil {
		return "", nil, fmt.Errorf("OpenClaw 配置未找到")
	}
	models, _ := ocConfig["models"].(map[string]interface{})
	providers, _ := models["providers"].(map[string]interface{})
	pid := strings.TrimSpace(settings.ProviderID)
	mid := strings.TrimSpace(settings.ModelID)
	if pid == "" || mid == "" {
		if agents, ok := ocConfig["agents"].(map[string]interface{}); ok {
			if defaults, ok := agents["defaults"].(map[string]interface{}); ok {
				if modelCfg, ok := defaults["model"].(map[string]interface{}); ok {
					if primary, ok := modelCfg["primary"].(string); ok {
						parts := strings.SplitN(primary, "/", 2)
						if len(parts) == 2 {
							pid, mid = parts[0], parts[1]
						}
					}
				}
			}
		}
	}
	provider, ok := providers[pid].(map[string]interface{})
	if !ok {
		return "", nil, fmt.Errorf("workflow provider not found: %s", pid)
	}
	baseURL, _ := provider["baseUrl"].(string)
	apiKey, _ := provider["apiKey"].(string)
	apiType, _ := provider["api"].(string)
	if apiType == "" {
		apiType = "openai-completions"
	}
	if baseURL == "" || apiKey == "" || mid == "" {
		return "", nil, fmt.Errorf("workflow AI 模型未配置完整")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	var url string
	var body []byte
	headers := map[string]string{"Content-Type": "application/json"}
	switch apiType {
	case "anthropic", "anthropic-messages":
		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"
		url = baseURL + "/messages"
		var nonSys []map[string]string
		sysContent := ""
		for _, m := range messages {
			if m["role"] == "system" {
				sysContent = m["content"]
			} else {
				nonSys = append(nonSys, m)
			}
		}
		body, _ = json.Marshal(map[string]interface{}{"model": mid, "max_tokens": 2048, "system": sysContent, "messages": nonSys})
	case "google-genai", "google-generative-ai":
		url = fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, mid, apiKey)
		contents := []map[string]interface{}{}
		sysContent := ""
		for _, m := range messages {
			if m["role"] == "system" {
				sysContent = m["content"]
				continue
			}
			role := "user"
			if m["role"] == "assistant" {
				role = "model"
			}
			contents = append(contents, map[string]interface{}{"role": role, "parts": []map[string]string{{"text": m["content"]}}})
		}
		payload := map[string]interface{}{"contents": contents, "generationConfig": map[string]int{"maxOutputTokens": 2048}}
		if sysContent != "" {
			payload["systemInstruction"] = map[string]interface{}{"parts": []map[string]string{{"text": sysContent}}}
		}
		body, _ = json.Marshal(payload)
	default:
		headers["Authorization"] = "Bearer " + apiKey
		if apiType == "openai-responses" || apiType == "openai-response" || apiType == "openai" || strings.Contains(strings.ToLower(baseURL), "/v1") {
			url = baseURL + "/responses"
			input := make([]map[string]interface{}, 0, len(messages))
			for _, m := range messages {
				input = append(input, map[string]interface{}{
					"role":    m["role"],
					"content": []map[string]string{{"type": "input_text", "text": m["content"]}},
				})
			}
			body, _ = json.Marshal(map[string]interface{}{"model": mid, "input": input, "max_output_tokens": 2048})
			apiType = "openai-responses"
		} else {
			url = baseURL + "/chat/completions"
			body, _ = json.Marshal(map[string]interface{}{"model": mid, "messages": messages, "max_tokens": 2048})
		}
	}
	client := &http.Client{Timeout: 120 * time.Second}
	var respBody []byte
	var resp *http.Response
	var doErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, doErr = client.Do(req)
		if doErr != nil {
			if attempt == 0 && (strings.Contains(strings.ToLower(doErr.Error()), "timeout") || strings.Contains(strings.ToLower(doErr.Error()), "deadline exceeded")) {
				time.Sleep(1200 * time.Millisecond)
				continue
			}
			return "", nil, doErr
		}
		respBody, _ = io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		resp.Body.Close()
		if resp.StatusCode >= 500 && attempt == 0 {
			time.Sleep(1200 * time.Millisecond)
			continue
		}
		break
	}
	if doErr != nil {
		return "", nil, doErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if apiType == "openai-completions" && strings.Contains(string(respBody), "/v1/responses") {
			headers["Authorization"] = "Bearer " + apiKey
			url = baseURL + "/responses"
			input := make([]map[string]interface{}, 0, len(messages))
			for _, m := range messages {
				input = append(input, map[string]interface{}{
					"role":    m["role"],
					"content": []map[string]string{{"type": "input_text", "text": m["content"]}},
				})
			}
			body, _ = json.Marshal(map[string]interface{}{"model": mid, "input": input, "max_output_tokens": 2048})
			req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, err = client.Do(req)
			if err != nil {
				return "", nil, err
			}
			respBody, _ = io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				apiType = "openai-responses"
			} else {
				return "", nil, fmt.Errorf("workflow AI 请求失败: %s", strings.TrimSpace(string(respBody)))
			}
		} else {
			return "", nil, fmt.Errorf("workflow AI 请求失败: %s", strings.TrimSpace(string(respBody)))
		}
	}
	text, err := extractAIText(apiType, respBody)
	if err != nil {
		return "", nil, err
	}
	return text, map[string]string{"providerId": pid, "modelId": mid}, nil
}

func extractAIText(apiType string, body []byte) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	switch apiType {
	case "anthropic", "anthropic-messages":
		if arr, ok := data["content"].([]interface{}); ok {
			parts := []string{}
			for _, item := range arr {
				if block, ok := item.(map[string]interface{}); ok {
					if text, ok := block["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
			return strings.Join(parts, "\n"), nil
		}
	case "google-genai", "google-generative-ai":
		if cands, ok := data["candidates"].([]interface{}); ok && len(cands) > 0 {
			if cand, ok := cands[0].(map[string]interface{}); ok {
				if content, ok := cand["content"].(map[string]interface{}); ok {
					if parts, ok := content["parts"].([]interface{}); ok {
						chunks := []string{}
						for _, part := range parts {
							if block, ok := part.(map[string]interface{}); ok {
								if text, ok := block["text"].(string); ok {
									chunks = append(chunks, text)
								}
							}
						}
						return strings.Join(chunks, "\n"), nil
					}
				}
			}
		}
	case "openai-responses":
		if output, ok := data["output"].([]interface{}); ok {
			parts := []string{}
			for _, item := range output {
				block, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if content, ok := block["content"].([]interface{}); ok {
					for _, c := range content {
						segment, ok := c.(map[string]interface{})
						if !ok {
							continue
						}
						if text, ok := segment["text"].(string); ok && strings.TrimSpace(text) != "" {
							parts = append(parts, text)
						}
					}
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n"), nil
			}
		}
		if text, ok := data["output_text"].(string); ok && strings.TrimSpace(text) != "" {
			return text, nil
		}
	default:
		if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if message, ok := choice["message"].(map[string]interface{}); ok {
					if text, ok := message["content"].(string); ok {
						return text, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("无法解析 workflow AI 返回")
}

func stripCodeFence(text string) string {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func mustJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func toStringLocal(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func GetDefaultWorkflowTemplates() []model.WorkflowTemplate {
	now := time.Now().UnixMilli()
	return []model.WorkflowTemplate{
		{
			ID:          "workflow_example_quickstart",
			Name:        "示例模板：活动方案快速演示",
			Description: "一个用于演示工作流 1.0 的示例模板，包含收集需求、生成计划、等待确认、输出最终结果与文件回传。",
			Category:    "示例模板",
			Status:      "ready",
			TriggerMode: "manual",
			Settings:    map[string]interface{}{"approvalMode": 2, "progressMode": "detailed", "tone": "professional"},
			Definition: map[string]interface{}{"nodes": []map[string]interface{}{
				{"id": "collect", "title": "收集任务需求", "type": "input", "order": 1, "skill": "none"},
				{"id": "plan", "title": "生成执行计划", "type": "ai_plan", "order": 2, "skill": "none", "outputFile": "01-执行计划.md", "artifactName": "执行计划", "outputFormat": "markdown"},
				{"id": "confirm", "title": "等待用户确认方向", "type": "wait_user", "order": 3, "skill": "none"},
				{"id": "deliver", "title": "整理最终交付结果", "type": "summary", "order": 4, "skill": "none", "outputFile": "02-最终结果.md", "artifactName": "最终结果", "outputFormat": "markdown", "isFinalOutput": true, "sendToUser": true},
				{"id": "end", "title": "结束", "type": "end", "order": 5},
			}, "edges": []map[string]interface{}{}},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

func EnsureWorkflowDefaults(db *sql.DB) error {
	if legacy, err := model.GetWorkflowTemplate(db, "workflow_event_ops"); err == nil && legacy != nil && strings.TrimSpace(legacy.Name) == "活动运营流程" {
		_ = model.DeleteWorkflowTemplate(db, legacy.ID)
	}
	items, err := model.ListWorkflowTemplates(db)
	if err != nil {
		return err
	}
	if len(items) > 0 {
		return nil
	}
	for _, item := range GetDefaultWorkflowTemplates() {
		copy := item
		if err := model.UpsertWorkflowTemplate(db, &copy); err != nil {
			return err
		}
	}
	return nil
}
