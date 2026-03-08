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
		// OpenClaw 的技能目录在不同版本中存在差异：
		// - 旧版常见为 ~/work/skills
		// - 新版常见为 ~/.openclaw/workspace/skills
		// 因此这里同时扫描多种候选路径，避免“技能中心扫不到已下载 skills”。
		for _, candidate := range buildSkillScanCandidates(cfg) {
			scanSkillDir(candidate.Path, candidate.Source, false, blockSet, &skills, seen)
		}

		// App skills
		appDir := cfg.OpenClawApp
		candidates := []string{}
		if appDir != "" {
			candidates = append(candidates, filepath.Join(appDir, "skills"))
		}
		candidates = append(candidates,
			filepath.Join(filepath.Dir(cfg.OpenClawDir), "app", "skills"),
		)
		for _, candidate := range candidates {
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

type skillScanCandidate struct {
	Path   string
	Source string
}

func buildSkillScanCandidates(cfg *config.Config) []skillScanCandidate {
	parentDir := filepath.Dir(cfg.OpenClawDir)
	workDir := strings.TrimSpace(cfg.OpenClawWork)
	if workDir == "" {
		workDir = filepath.Join(parentDir, "work")
	}

	raw := []skillScanCandidate{
		// Legacy/global skill directory
		{Path: filepath.Join(cfg.OpenClawDir, "skills"), Source: "skill"},
		// OpenClaw 2026+ default workspace
		{Path: filepath.Join(cfg.OpenClawDir, "workspace", "skills"), Source: "workspace"},
		// Backward-compatible work dir
		{Path: filepath.Join(workDir, "skills"), Source: "workspace"},
		// Some deployments use OPENCLAW_WORK as root and keep workspace/skills below it
		{Path: filepath.Join(workDir, "workspace", "skills"), Source: "workspace"},
		// Extra fallback for custom layouts (e.g. /home/user/workspace/skills)
		{Path: filepath.Join(parentDir, "workspace", "skills"), Source: "workspace"},
	}

	seenPath := map[string]bool{}
	out := make([]skillScanCandidate, 0, len(raw))
	for _, c := range raw {
		p := strings.TrimSpace(c.Path)
		if p == "" || seenPath[p] {
			continue
		}
		seenPath[p] = true
		out = append(out, skillScanCandidate{Path: p, Source: c.Source})
	}
	return out
}
