package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/model"
)

var panelChatKnowledgeTokenRe = regexp.MustCompile(`[\p{Han}A-Za-z0-9_\-]{2,}`)

type panelChatKnowledgeSource struct {
	Path    string `json:"path"`
	Title   string `json:"title,omitempty"`
	Excerpt string `json:"excerpt"`
	Score   int    `json:"score"`
}

func panelChatTextPreviewExtensions() map[string]bool {
	return map[string]bool{
		".txt": true, ".md": true, ".log": true, ".json": true, ".jsonl": true,
		".js": true, ".ts": true, ".py": true, ".sh": true, ".yaml": true,
		".yml": true, ".xml": true, ".html": true, ".css": true, ".csv": true,
		".ini": true, ".conf": true, ".toml": true, ".env": true,
	}
}

func normalizeKnowledgeBindingPaths(paths []string) []model.PanelChatKnowledgeBinding {
	seen := map[string]struct{}{}
	items := make([]model.PanelChatKnowledgeBinding, 0, len(paths))
	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		items = append(items, model.PanelChatKnowledgeBinding{Path: path, Title: filepath.Base(path)})
	}
	return items
}

func validateKnowledgeBindings(cfg *config.Config, items []model.PanelChatKnowledgeBinding) error {
	for _, item := range items {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		if _, _, err := resolvePanelChatKnowledgePath(cfg, item.Path); err != nil {
			return fmt.Errorf("知识文件不存在或不可访问: %s", item.Path)
		}
	}
	return nil
}

func panelChatKnowledgeRoots(cfg *config.Config) []string {
	roots := []string{getWorkspaceDir(cfg)}
	if strings.TrimSpace(cfg.OpenClawWork) != "" {
		roots = append(roots, cfg.OpenClawWork)
	}
	if strings.TrimSpace(cfg.OpenClawDir) != "" {
		roots = append(roots, cfg.OpenClawDir)
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(roots))
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil || abs == "" {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		result = append(result, abs)
	}
	return result
}

func resolvePanelChatKnowledgePath(cfg *config.Config, raw string) (string, string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", "", fmt.Errorf("empty path")
	}
	roots := panelChatKnowledgeRoots(cfg)
	try := make([]string, 0, len(roots)+1)
	if filepath.IsAbs(path) {
		try = append(try, path)
	} else {
		for _, root := range roots {
			try = append(try, filepath.Join(root, path))
		}
	}
	for _, candidate := range try {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		allowed := false
		for _, root := range roots {
			if strings.HasPrefix(abs, root) {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || info.IsDir() {
			continue
		}
		display := abs
		for _, root := range roots {
			prefix := root + string(os.PathSeparator)
			if strings.HasPrefix(abs, prefix) {
				display = strings.TrimPrefix(abs, prefix)
				break
			}
		}
		return abs, filepath.ToSlash(display), nil
	}
	return "", "", fmt.Errorf("file not found")
}

func extractPanelChatKnowledgeTokens(query string) []string {
	matches := panelChatKnowledgeTokenRe.FindAllString(strings.ToLower(query), -1)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, item := range matches {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func scorePanelChatKnowledgeChunk(fileName, chunk string, tokens []string) int {
	text := strings.ToLower(chunk)
	name := strings.ToLower(fileName)
	score := 0
	for _, token := range tokens {
		if strings.Contains(name, token) {
			score += 8
		}
		count := strings.Count(text, token)
		if count > 0 {
			score += count * 3
		}
	}
	if score == 0 && len(tokens) == 0 && strings.TrimSpace(chunk) != "" {
		score = 1
	}
	return score
}

func splitPanelChatKnowledgeChunks(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	blocks := strings.Split(content, "\n\n")
	chunks := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		runes := []rune(block)
		for len(runes) > 1200 {
			chunks = append(chunks, string(runes[:1200]))
			runes = runes[1200:]
		}
		if len(runes) > 0 {
			chunks = append(chunks, string(runes))
		}
	}
	if len(chunks) == 0 && strings.TrimSpace(content) != "" {
		chunks = append(chunks, strings.TrimSpace(content))
	}
	return chunks
}

func retrievePanelChatKnowledge(db *sql.DB, cfg *config.Config, agentID, sessionID, query string) []panelChatKnowledgeSource {
	bindings, _ := model.ListPanelChatAgentKnowledgeBindings(db, agentID)
	shared, _ := model.ListPanelChatSessionSharedContexts(db, sessionID)
	all := make([]model.PanelChatKnowledgeBinding, 0, len(bindings)+len(shared))
	all = append(all, bindings...)
	all = append(all, shared...)
	if len(all) == 0 {
		return nil
	}
	tokens := extractPanelChatKnowledgeTokens(query)
	results := make([]panelChatKnowledgeSource, 0)
	for _, item := range all {
		fullPath, displayPath, err := resolvePanelChatKnowledgePath(cfg, item.Path)
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(fullPath))
		if !panelChatTextPreviewExtensions()[ext] {
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		if len(data) > 256*1024 {
			data = data[:256*1024]
		}
		chunks := splitPanelChatKnowledgeChunks(string(data))
		best := panelChatKnowledgeSource{}
		for _, chunk := range chunks {
			score := scorePanelChatKnowledgeChunk(filepath.Base(fullPath), chunk, tokens)
			if score <= best.Score {
				continue
			}
			excerpt := strings.TrimSpace(chunk)
			runes := []rune(excerpt)
			if len(runes) > 400 {
				excerpt = string(runes[:400]) + "..."
			}
			best = panelChatKnowledgeSource{Path: displayPath, Title: item.Title, Excerpt: excerpt, Score: score}
		}
		if best.Score > 0 {
			results = append(results, best)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Path < results[j].Path
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > 3 {
		results = results[:3]
	}
	return results
}

func injectPanelChatKnowledge(prompt string, sources []panelChatKnowledgeSource) string {
	if len(sources) == 0 {
		return prompt
	}
	lines := []string{prompt, "", "以下是检索到的参考资料，请优先结合这些资料回答，并在内容中保持与资料一致："}
	for _, item := range sources {
		label := item.Path
		if strings.TrimSpace(item.Title) != "" {
			label = item.Title + " (" + item.Path + ")"
		}
		lines = append(lines, fmt.Sprintf("- 来源：%s\n%s", label, item.Excerpt))
	}
	return strings.Join(lines, "\n\n")
}

func GetPanelChatAgentKnowledgeBindings(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := model.ListPanelChatAgentKnowledgeBindings(db, strings.TrimSpace(c.Param("agentId")))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "bindings": items})
	}
}

func SavePanelChatAgentKnowledgeBindings(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Paths []string `json:"paths"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		agentID := strings.TrimSpace(c.Param("agentId"))
		items := normalizeKnowledgeBindingPaths(req.Paths)
		if err := validateKnowledgeBindings(cfg, items); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := model.ReplacePanelChatAgentKnowledgeBindings(db, agentID, items); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "bindings": items})
	}
}

func GetPanelChatSessionSharedContexts(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := model.ListPanelChatSessionSharedContexts(db, strings.TrimSpace(c.Param("id")))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "sharedContexts": items})
	}
}

func SavePanelChatSessionSharedContexts(db *sql.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Paths []string `json:"paths"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		sessionID := strings.TrimSpace(c.Param("id"))
		items := normalizeKnowledgeBindingPaths(req.Paths)
		if err := validateKnowledgeBindings(cfg, items); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := model.ReplacePanelChatSessionSharedContexts(db, sessionID, items); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "sharedContexts": items})
	}
}
