package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

// ModelHealthCheck 模型健康检查
func ModelHealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BaseURL string `json:"baseUrl"`
			APIKey  string `json:"apiKey"`
			APIType string `json:"apiType"`
			ModelID string `json:"modelId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}
		if req.BaseURL == "" || req.APIKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "baseUrl and apiKey required"})
			return
		}

		testModel := req.ModelID
		if testModel == "" {
			testModel = "gpt-4o"
		}

		baseURL := strings.TrimRight(req.BaseURL, "/")
		var url string
		var body []byte
		headers := map[string]string{"Content-Type": "application/json"}

		switch req.APIType {
		case "anthropic", "anthropic-messages":
			headers["x-api-key"] = req.APIKey
			headers["anthropic-version"] = "2023-06-01"
			url = baseURL + "/messages"
			body, _ = json.Marshal(map[string]interface{}{
				"model": testModel, "max_tokens": 5,
				"messages": []map[string]string{{"role": "user", "content": "hi"}},
			})
		case "google-genai", "google-generative-ai":
			url = fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, testModel, req.APIKey)
			body, _ = json.Marshal(map[string]interface{}{
				"contents":         []map[string]interface{}{{"parts": []map[string]string{{"text": "hi"}}}},
				"generationConfig": map[string]int{"maxOutputTokens": 5},
			})
		default:
			headers["Authorization"] = "Bearer " + req.APIKey
			url = baseURL + "/chat/completions"
			body, _ = json.Marshal(map[string]interface{}{
				"model": testModel, "max_tokens": 5,
				"messages": []map[string]string{{"role": "user", "content": "hi"}},
			})
		}

		client := &http.Client{Timeout: 15 * time.Second}
		httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		for k, v := range headers {
			httpReq.Header.Set(k, v)
		}

		start := time.Now()
		resp, err := client.Do(httpReq)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "healthy": false, "error": err.Error()})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.JSON(http.StatusOK, gin.H{"ok": true, "healthy": true, "status": resp.StatusCode, "latencyMs": latency})
		} else {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			var data map[string]interface{}
			json.Unmarshal(respBody, &data)
			errMsg := ""
			if e, ok := data["error"].(map[string]interface{}); ok {
				errMsg, _ = e["message"].(string)
			}
			if errMsg == "" {
				if m, ok := data["message"].(string); ok {
					errMsg = m
				}
			}
			if errMsg == "" {
				errMsg = string(respBody)
				if len(errMsg) > 200 {
					errMsg = errMsg[:200]
				}
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "healthy": false, "status": resp.StatusCode, "error": errMsg})
		}
	}
}

// AIChat AI 助手对话
func AIChat(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Messages   []map[string]string `json:"messages"`
			ProviderID string              `json:"providerId"`
			ModelID    string              `json:"modelId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Messages) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "messages required"})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "OpenClaw 配置未找到"})
			return
		}
		normalizeOpenClawCompatConfig(ocConfig)

		models, _ := ocConfig["models"].(map[string]interface{})
		providers, _ := models["providers"].(map[string]interface{})

		pid := req.ProviderID
		mid := req.ModelID

		// 如果未指定，使用默认模型
		if pid == "" || mid == "" {
			if agents, ok := ocConfig["agents"].(map[string]interface{}); ok {
				if defaults, ok := agents["defaults"].(map[string]interface{}); ok {
					if model, ok := defaults["model"].(map[string]interface{}); ok {
						if primary, ok := model["primary"].(string); ok {
							parts := strings.SplitN(primary, "/", 2)
							if len(parts) >= 2 {
								if pid == "" {
									pid = parts[0]
								}
								if mid == "" {
									mid = parts[1]
								}
							}
						}
					}
				}
			}
		}

		provider, ok := providers[pid].(map[string]interface{})
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("Provider \"%s\" not found", pid)})
			return
		}

		baseURL, _ := provider["baseUrl"].(string)
		apiKey, _ := provider["apiKey"].(string)
		apiType, _ := provider["api"].(string)
		if apiType == "" {
			apiType = "openai-completions"
		}

		if baseURL == "" || apiKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "Provider missing baseUrl or apiKey"})
			return
		}

		baseURL = strings.TrimRight(baseURL, "/")

		// 读取 FAQ 文档注入到系统提示词
		faqContent := ""
		// 尝试从多个可能的位置读取 FAQ
		faqPaths := []string{
			filepath.Join(cfg.DataDir, "docs", "FAQ.md"),
			"/opt/clawpanel/docs/FAQ.md",
		}
		// 也尝试从可执行文件同级目录读取
		if exe, err := os.Executable(); err == nil {
			faqPaths = append(faqPaths, filepath.Join(filepath.Dir(exe), "docs", "FAQ.md"))
		}
		for _, p := range faqPaths {
			if data, err := os.ReadFile(p); err == nil {
				faqContent = string(data)
				break
			}
		}

		// 系统提示词
		faqSection := ""
		if faqContent != "" {
			faqSection = fmt.Sprintf(`

以下是 ClawPanel 常见问题解答文档，请根据此文档优先回答用户问题：

%s`, faqContent)
		}

		systemPrompt := fmt.Sprintf(`你是 ClawPanel 管理后台的 AI 助手。ClawPanel 是一个开源的 OpenClaw 智能助手管理面板。

当前使用的模型: %s/%s (API: %s)

你的职责：
1. 帮助用户理解和使用 ClawPanel 管理后台的各项功能
2. 解答关于 OpenClaw 配置、技能、插件、通道管理等问题
3. 帮助排查错误和问题
4. 提供操作建议和最佳实践

项目信息：
- GitHub: https://github.com/zhaoxinyi02/ClawPanel
- 技术栈: Go + React + TailwindCSS (v5.0.2)
- 支持的通道: QQ, 微信, Telegram, Discord 等 20+
- 技能系统: 支持 65+ 内置技能和自定义技能
- 面板默认端口: 19527，默认密码: clawpanel
- 面板数据目录: ~/.clawpanel/
- 面板支持一键自检更新（国内加速服务器）

回答要简洁、准确、友好。使用 Markdown 格式。优先根据 FAQ 文档回答，如果 FAQ 中没有相关内容再根据你的知识回答。%s`, pid, mid, apiType, faqSection)

		// 构建完整消息列表
		fullMessages := []map[string]string{{"role": "system", "content": systemPrompt}}
		fullMessages = append(fullMessages, req.Messages...)

		// 根据 API 类型构建请求
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
			for _, m := range fullMessages {
				if m["role"] == "system" {
					sysContent = m["content"]
				} else {
					nonSys = append(nonSys, m)
				}
			}
			body, _ = json.Marshal(map[string]interface{}{
				"model": mid, "max_tokens": 2048,
				"system": sysContent, "messages": nonSys,
			})
		case "google-genai", "google-generative-ai":
			url = fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, mid, apiKey)
			var contents []map[string]interface{}
			sysContent := ""
			for _, m := range fullMessages {
				if m["role"] == "system" {
					sysContent = m["content"]
					continue
				}
				role := "user"
				if m["role"] == "assistant" {
					role = "model"
				}
				contents = append(contents, map[string]interface{}{
					"role":  role,
					"parts": []map[string]string{{"text": m["content"]}},
				})
			}
			body, _ = json.Marshal(map[string]interface{}{
				"systemInstruction": map[string]interface{}{
					"parts": []map[string]string{{"text": sysContent}},
				},
				"contents":         contents,
				"generationConfig": map[string]int{"maxOutputTokens": 2048},
			})
		default:
			headers["Authorization"] = "Bearer " + apiKey
			url = baseURL + "/chat/completions"
			body, _ = json.Marshal(map[string]interface{}{
				"model": mid, "max_tokens": 2048, "messages": fullMessages,
			})
		}

		client := &http.Client{Timeout: 60 * time.Second}
		httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		for k, v := range headers {
			httpReq.Header.Set(k, v)
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var data map[string]interface{}
		if err := json.Unmarshal(respBody, &data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "Invalid response from model"})
			return
		}

		if resp.StatusCode >= 400 {
			errMsg := ""
			if e, ok := data["error"].(map[string]interface{}); ok {
				errMsg, _ = e["message"].(string)
			}
			if errMsg == "" {
				errMsg, _ = data["message"].(string)
			}
			if errMsg == "" {
				errMsg = "API error"
			}
			c.JSON(resp.StatusCode, gin.H{"ok": false, "error": errMsg})
			return
		}

		// 提取回复
		var reply string
		switch apiType {
		case "anthropic", "anthropic-messages":
			if content, ok := data["content"].([]interface{}); ok && len(content) > 0 {
				if first, ok := content[0].(map[string]interface{}); ok {
					reply, _ = first["text"].(string)
				}
			}
		case "google-genai", "google-generative-ai":
			if candidates, ok := data["candidates"].([]interface{}); ok && len(candidates) > 0 {
				if cand, ok := candidates[0].(map[string]interface{}); ok {
					if content, ok := cand["content"].(map[string]interface{}); ok {
						if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
							if part, ok := parts[0].(map[string]interface{}); ok {
								reply, _ = part["text"].(string)
							}
						}
					}
				}
			}
		default:
			if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if msg, ok := choice["message"].(map[string]interface{}); ok {
						reply, _ = msg["content"].(string)
					}
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "reply": reply})
	}
}
