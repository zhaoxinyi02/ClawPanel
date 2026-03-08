package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

type skillInfo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Enabled     bool        `json:"enabled"`
	Source      string      `json:"source"`
	Path        string      `json:"path,omitempty"`
	Metadata    interface{} `json:"metadata,omitempty"`
	Requires    interface{} `json:"requires,omitempty"`
}

type pluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source"`
	InstalledAt string `json:"installedAt,omitempty"`
	Path        string `json:"path,omitempty"`
}

// GetSkills 获取技能和插件列表
func GetSkills(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}

		// 获取 blocklist
		var blocklist []string
		if skills, ok := ocConfig["skills"].(map[string]interface{}); ok {
			if bl, ok := skills["blocklist"].([]interface{}); ok {
				for _, v := range bl {
					if s, ok := v.(string); ok {
						blocklist = append(blocklist, s)
					}
				}
			}
		}
		blockSet := make(map[string]bool)
		for _, b := range blocklist {
			blockSet[b] = true
		}

		// 获取插件配置
		pluginEntries := map[string]interface{}{}
		pluginInstalls := map[string]interface{}{}
		if plugins, ok := ocConfig["plugins"].(map[string]interface{}); ok {
			if e, ok := plugins["entries"].(map[string]interface{}); ok {
				pluginEntries = e
			}
			if i, ok := plugins["installs"].(map[string]interface{}); ok {
				pluginInstalls = i
			}
		}

		var skills []skillInfo
		var pluginsList []pluginInfo
		seen := make(map[string]bool)
		pluginSeen := make(map[string]bool)

		// --- 扫描插件 ---
		extDir := filepath.Join(cfg.OpenClawDir, "extensions")
		scanPluginDir(extDir, pluginEntries, pluginInstalls, &pluginsList, pluginSeen, "installed")

		// 扫描 config/extensions (OpenClawDir 本身就是 config 目录)
		configExtDir := filepath.Join(cfg.OpenClawDir, "extensions")
		if configExtDir != extDir {
			scanPluginDir(configExtDir, pluginEntries, pluginInstalls, &pluginsList, pluginSeen, "config-ext")
		}

		// 添加 plugins.entries 中未扫描到的
		for id, entry := range pluginEntries {
			if pluginSeen[id] {
				continue
			}
			pluginSeen[id] = true
			e, _ := entry.(map[string]interface{})
			enabled := true
			if e != nil {
				if v, ok := e["enabled"].(bool); ok {
					enabled = v
				}
			}
			inst, _ := pluginInstalls[id].(map[string]interface{})
			ver := ""
			installedAt := ""
			if inst != nil {
				ver, _ = inst["version"].(string)
				installedAt, _ = inst["installedAt"].(string)
			}
			pluginsList = append(pluginsList, pluginInfo{
				ID: id, Name: id, Enabled: enabled,
				Source: "config", Version: ver, InstalledAt: installedAt,
			})
		}

		// --- 扫描技能 ---
		for _, candidate := range resolveWorkspaceSkillDirs(cfg, ocConfig) {
			scanSkillDir(candidate, "workspace", false, blockSet, &skills, seen)
		}
		for _, candidate := range resolveOpenClawSkillDirs(cfg) {
			scanSkillDir(candidate, "skill", false, blockSet, &skills, seen)
		}
		for _, candidate := range resolveAppSkillDirs(cfg) {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				scanSkillDir(candidate, "app-skill", true, blockSet, &skills, seen)
				break
			}
		}

		if skills == nil {
			skills = []skillInfo{}
		}
		if pluginsList == nil {
			pluginsList = []pluginInfo{}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "skills": skills, "plugins": pluginsList})
	}
}

func resolveOpenClawSkillDirs(cfg *config.Config) []string {
	return uniqueSkillDirs(filepath.Join(cfg.OpenClawDir, "skills"))
}

func resolveWorkspaceSkillDirs(cfg *config.Config, ocConfig map[string]interface{}) []string {
	parentDir := filepath.Dir(cfg.OpenClawDir)
	roots := []string{}

	if cfg.OpenClawWork != "" {
		roots = append(roots, cfg.OpenClawWork)
	} else {
		roots = append(roots, filepath.Join(parentDir, "work"))
	}

	if workspace, ok := ocConfig["workspace"].(map[string]interface{}); ok {
		if path := normalizeSkillRootPath(parentDir, toString(workspace["path"])); path != "" {
			roots = append(roots, path)
		}
	}

	if agents, ok := ocConfig["agents"].(map[string]interface{}); ok {
		if list, ok := agents["list"].([]interface{}); ok {
			for _, raw := range list {
				item, _ := raw.(map[string]interface{})
				if item == nil {
					continue
				}
				if path := normalizeSkillRootPath(parentDir, toString(item["workspace"])); path != "" {
					roots = append(roots, path)
				}
			}
		}
	}

	skillDirs := make([]string, 0, len(roots))
	for _, root := range roots {
		if root == "" {
			continue
		}
		skillDirs = append(skillDirs, filepath.Join(root, "skills"))
	}
	return uniqueSkillDirs(skillDirs...)
}

func resolveAppSkillDirs(cfg *config.Config) []string {
	parentDir := filepath.Dir(cfg.OpenClawDir)
	candidates := []string{}
	if cfg.OpenClawApp != "" {
		candidates = append(candidates, filepath.Join(cfg.OpenClawApp, "skills"))
	}
	candidates = append(candidates, filepath.Join(parentDir, "app", "skills"))
	return uniqueSkillDirs(candidates...)
}

func normalizeSkillRootPath(parentDir, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	path := filepath.Clean(raw)
	if !filepath.IsAbs(path) {
		path = filepath.Join(parentDir, path)
	}
	return filepath.Clean(path)
}

func uniqueSkillDirs(paths ...string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		p = filepath.Clean(strings.TrimSpace(p))
		if p == "." || p == "" || seen[p] {
			continue
		}
		seen[p] = true
		result = append(result, p)
	}
	return result
}

// ToggleSkill 切换技能启用/禁用
func ToggleSkill(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "无法读取配置"})
			return
		}

		skillsCfg, _ := ocConfig["skills"].(map[string]interface{})
		if skillsCfg == nil {
			skillsCfg = map[string]interface{}{}
		}

		var blocklist []string
		if bl, ok := skillsCfg["blocklist"].([]interface{}); ok {
			for _, v := range bl {
				if s, ok := v.(string); ok {
					blocklist = append(blocklist, s)
				}
			}
		}

		if req.Enabled {
			// 从 blocklist 移除
			var newList []string
			for _, s := range blocklist {
				if s != id {
					newList = append(newList, s)
				}
			}
			blocklist = newList
		} else {
			// 添加到 blocklist
			found := false
			for _, s := range blocklist {
				if s == id {
					found = true
					break
				}
			}
			if !found {
				blocklist = append(blocklist, id)
			}
		}

		// 转换为 []interface{}
		blInterface := make([]interface{}, len(blocklist))
		for i, s := range blocklist {
			blInterface[i] = s
		}
		skillsCfg["blocklist"] = blInterface
		ocConfig["skills"] = skillsCfg

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetCronJobs 获取定时任务
func GetCronJobs(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cronPath := filepath.Join(cfg.OpenClawDir, "cron", "jobs.json")
		data, err := os.ReadFile(cronPath)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "jobs": []interface{}{}})
			return
		}
		var cronData map[string]interface{}
		json.Unmarshal(data, &cronData)
		jobs, _ := cronData["jobs"].([]interface{})
		if jobs == nil {
			jobs = []interface{}{}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "jobs": jobs})
	}
}

// SaveCronJobs 保存定时任务
func SaveCronJobs(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Jobs []interface{} `json:"jobs"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "参数错误"})
			return
		}

		agentIDs, agentSet := loadAgentIDs(cfg)
		if len(agentIDs) == 0 {
			agentSet = map[string]struct{}{"main": {}}
		}
		defaultAgent := loadDefaultAgentID(cfg)
		if defaultAgent == "" {
			defaultAgent = "main"
		}
		for i := range req.Jobs {
			job, ok := req.Jobs[i].(map[string]interface{})
			if !ok {
				continue
			}
			target := strings.TrimSpace(toString(job["sessionTarget"]))
			if target == "" {
				target = defaultAgent
				job["sessionTarget"] = target
			}
			if _, ok := agentSet[target]; !ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"ok":    false,
					"error": fmt.Sprintf("jobs[%d] sessionTarget 不存在: %s", i, target),
				})
				return
			}
		}

		cronDir := filepath.Join(cfg.OpenClawDir, "cron")
		os.MkdirAll(cronDir, 0755)
		cronPath := filepath.Join(cronDir, "jobs.json")

		existing := map[string]interface{}{"version": float64(1)}
		if data, err := os.ReadFile(cronPath); err == nil {
			json.Unmarshal(data, &existing)
		}
		existing["jobs"] = req.Jobs

		data, _ := json.MarshalIndent(existing, "", "  ")
		if err := os.WriteFile(cronPath, data, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// --- 辅助函数 ---

func scanPluginDir(dir string, entries, installs map[string]interface{}, result *[]pluginInfo, seen map[string]bool, source string) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range dirEntries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if seen[name] {
			continue
		}
		seen[name] = true

		extPath := filepath.Join(dir, name)
		var pkgInfo map[string]interface{}
		if data, err := os.ReadFile(filepath.Join(extPath, "package.json")); err == nil {
			json.Unmarshal(data, &pkgInfo)
		}
		var pluginJSON map[string]interface{}
		if data, err := os.ReadFile(filepath.Join(extPath, "openclaw.plugin.json")); err == nil {
			json.Unmarshal(data, &pluginJSON)
		}

		pName := name
		pDesc := ""
		if pluginJSON != nil {
			if n, ok := pluginJSON["name"].(string); ok && n != "" {
				pName = n
			}
			if d, ok := pluginJSON["description"].(string); ok {
				pDesc = d
			}
		}
		if pDesc == "" && pkgInfo != nil {
			if d, ok := pkgInfo["description"].(string); ok {
				pDesc = d
			}
		}

		enabled := true
		if entry, ok := entries[name].(map[string]interface{}); ok {
			if v, ok := entry["enabled"].(bool); ok {
				enabled = v
			}
		}

		ver := ""
		installedAt := ""
		if inst, ok := installs[name].(map[string]interface{}); ok {
			ver, _ = inst["version"].(string)
			installedAt, _ = inst["installedAt"].(string)
		}
		if ver == "" && pkgInfo != nil {
			ver, _ = pkgInfo["version"].(string)
		}

		*result = append(*result, pluginInfo{
			ID: name, Name: pName, Description: pDesc,
			Version: ver, Enabled: enabled, Source: source,
			InstalledAt: installedAt, Path: extPath,
		})
	}
}

func scanSkillDir(dir, source string, requireSkillMd bool, blockSet map[string]bool, result *[]skillInfo, seen map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if seen[name] {
			continue
		}

		skillPath := filepath.Join(dir, name)
		skillMdPath := filepath.Join(skillPath, "SKILL.md")

		if requireSkillMd {
			if _, err := os.Stat(skillMdPath); err != nil {
				continue
			}
		}

		seen[name] = true

		var pkgInfo map[string]interface{}
		if data, err := os.ReadFile(filepath.Join(skillPath, "package.json")); err == nil {
			json.Unmarshal(data, &pkgInfo)
		}

		sName := name
		sDesc := ""
		var metadata interface{}
		var requires interface{}

		if pkgInfo != nil {
			if n, ok := pkgInfo["name"].(string); ok && n != "" {
				sName = n
			}
			if d, ok := pkgInfo["description"].(string); ok {
				sDesc = d
			}
		}

		// 解析 SKILL.md frontmatter
		if mdData, err := os.ReadFile(skillMdPath); err == nil {
			mdContent := string(mdData)
			re := regexp.MustCompile(`(?s)^---\n(.*?)\n---`)
			if match := re.FindStringSubmatch(mdContent); len(match) > 1 {
				fm := match[1]
				if nameMatch := regexp.MustCompile(`(?m)^name:\s*(.+)$`).FindStringSubmatch(fm); len(nameMatch) > 1 {
					sName = strings.TrimSpace(nameMatch[1])
				}
				if descMatch := regexp.MustCompile(`(?m)^description:\s*["']?(.+?)["']?$`).FindStringSubmatch(fm); len(descMatch) > 1 {
					sDesc = strings.TrimSpace(descMatch[1])
				}
			}
			if sDesc == "" {
				for _, line := range strings.Split(mdContent, "\n") {
					line = strings.TrimSpace(line)
					if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "---") {
						if len(line) > 200 {
							line = line[:200]
						}
						sDesc = line
						break
					}
				}
			}
		}

		ver := ""
		if pkgInfo != nil {
			ver, _ = pkgInfo["version"].(string)
		}

		*result = append(*result, skillInfo{
			ID: name, Name: sName, Description: sDesc,
			Version: ver, Enabled: !blockSet[name],
			Source: source, Path: skillPath,
			Metadata: metadata, Requires: requires,
		})
	}
}
