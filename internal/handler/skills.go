package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

type skillInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Version      string                 `json:"version,omitempty"`
	Enabled      bool                   `json:"enabled"`
	Path         string                 `json:"path"`
	SkillKey     string                 `json:"skillKey,omitempty"`
	Source       string                 `json:"source,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Requires     map[string]interface{} `json:"requires,omitempty"`
	ConfigSchema []skillConfigField     `json:"configSchema,omitempty"`
}

type skillConfigField struct {
	Key          string              `json:"key"`
	Label        string              `json:"label,omitempty"`
	Type         string              `json:"type,omitempty"`
	Placeholder  string              `json:"placeholder,omitempty"`
	Help         string              `json:"help,omitempty"`
	Required     bool                `json:"required,omitempty"`
	Options      []skillConfigOption `json:"options,omitempty"`
	DefaultValue interface{}         `json:"defaultValue,omitempty"`
}

type skillConfigOption struct {
	Label string      `json:"label,omitempty"`
	Value interface{} `json:"value"`
}

type pluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source,omitempty"`
	Path        string `json:"path"`
}

type skillDiscoveryRoot struct {
	Dir    string
	Source string
}

type pluginDiscoveryCandidate struct {
	ID     string
	Path   string
	Source string
}

const (
	maxSkillScanDepth = 3
	maxSkillFileSize  = 2 * 1024 * 1024
	maxSkillsPerRoot  = 256
)

var skillFrontmatterRegexp = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n?`)

// GetSkills returns installed skills and plugins from OpenClaw.
func GetSkills(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID, err := resolveRequestedAgentID(cfg, c.Query("agentId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}

		skillsCfg := asMapAny(ocConfig["skills"])
		skillEntries := asMapAny(skillsCfg["entries"])
		legacyBlocklist := readLegacySkillBlocklist(skillsCfg)
		pluginEntries := asMapAny(asMapAny(ocConfig["plugins"])["entries"])
		pluginInstalls := asMapAny(asMapAny(ocConfig["plugins"])["installs"])

		plugins := discoverPlugins(cfg, pluginEntries, pluginInstalls)
		sort.Slice(plugins, func(i, j int) bool {
			return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name)
		})

		roots := resolveSkillDiscoveryRoots(cfg, ocConfig, plugins, agentID)
		skills := discoverSkills(roots, skillEntries, legacyBlocklist, resolveBundledSkillAllowlist(ocConfig))

		c.JSON(http.StatusOK, gin.H{
			"ok":        true,
			"agentId":   agentID,
			"workspace": resolveSkillsWorkspace(cfg, agentID),
			"skills":    skills,
			"plugins":   plugins,
		})
	}
}

// ToggleSkill toggles a skill by writing skills.entries.<key>.enabled.
func ToggleSkill(cfg *config.Config) gin.HandlerFunc {
	type reqBody struct {
		Enabled bool     `json:"enabled"`
		Aliases []string `json:"aliases,omitempty"`
	}
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.Param("id"))
		if key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "missing skill id"})
			return
		}
		var req reqBody
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		ocConfig, _ := cfg.ReadOpenClawJSON()
		if ocConfig == nil {
			ocConfig = map[string]interface{}{}
		}
		skillsCfg := asMapAny(ocConfig["skills"])
		entries := asMapAny(skillsCfg["entries"])
		entry := asMapAny(entries[key])
		entry["enabled"] = req.Enabled
		entries[key] = entry
		skillsCfg["entries"] = entries
		removeLegacyBlocklistEntries(skillsCfg, append([]string{key}, req.Aliases...)...)
		ocConfig["skills"] = skillsCfg
		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetSkillConfig returns the current values for a skill's declared requires.config keys.
func GetSkillConfig(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		requested := strings.TrimSpace(c.Param("id"))
		if requested == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "missing skill id"})
			return
		}
		agentID, err := resolveRequestedAgentID(cfg, c.Query("agentId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		skill, ocConfig, err := resolveSkillConfigTarget(cfg, agentID, requested)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
			return
		}

		configKeys := asStringSlice(skill.Requires["config"])
		values := make(map[string]interface{}, len(configKeys))
		for _, key := range configKeys {
			value, ok, err := getNestedConfigValue(ocConfig, key)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
			if ok {
				values[key] = value
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":         true,
			"agentId":    agentID,
			"skillId":    skill.ID,
			"skillKey":   skill.SkillKey,
			"configKeys": configKeys,
			"values":     values,
		})
	}
}

// UpdateSkillConfig updates the current values for a skill's declared requires.config keys.
func UpdateSkillConfig(cfg *config.Config) gin.HandlerFunc {
	type reqBody struct {
		AgentID string                 `json:"agentId"`
		Values  map[string]interface{} `json:"values"`
	}
	return func(c *gin.Context) {
		requested := strings.TrimSpace(c.Param("id"))
		if requested == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "missing skill id"})
			return
		}

		var req reqBody
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if len(req.Values) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "missing config values"})
			return
		}

		agentID, err := resolveRequestedAgentID(cfg, req.AgentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		skill, ocConfig, err := resolveSkillConfigTarget(cfg, agentID, requested)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
			return
		}

		configKeys := asStringSlice(skill.Requires["config"])
		if len(configKeys) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "skill has no configurable keys"})
			return
		}
		allowed := make(map[string]struct{}, len(configKeys))
		for _, key := range configKeys {
			allowed[key] = struct{}{}
		}
		for key := range req.Values {
			if _, ok := allowed[key]; !ok {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("config key %q is not declared by skill %q", key, skill.Name)})
				return
			}
		}

		for _, key := range configKeys {
			value, ok := req.Values[key]
			if !ok {
				continue
			}
			if value == nil {
				if err := deleteNestedConfigValue(ocConfig, key); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
					return
				}
				continue
			}
			if err := setNestedConfigValue(ocConfig, key, value); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
		}

		if err := cfg.WriteOpenClawJSON(ocConfig); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		values := make(map[string]interface{}, len(configKeys))
		for _, key := range configKeys {
			value, ok, err := getNestedConfigValue(ocConfig, key)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
			if ok {
				values[key] = value
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":         true,
			"agentId":    agentID,
			"skillId":    skill.ID,
			"skillKey":   skill.SkillKey,
			"configKeys": configKeys,
			"values":     values,
			"updated":    len(req.Values),
		})
	}
}

// CopySkill copies an installed skill into another agent workspace or the global managed root.
func CopySkill(cfg *config.Config) gin.HandlerFunc {
	type reqBody struct {
		SourceAgentID string `json:"sourceAgentId"`
		TargetAgentID string `json:"targetAgentId"`
		InstallTarget string `json:"installTarget,omitempty"`
	}
	return func(c *gin.Context) {
		requested := strings.TrimSpace(c.Param("id"))
		if requested == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "missing skill id"})
			return
		}

		var req reqBody
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		sourceAgentID, err := resolveRequestedAgentID(cfg, req.SourceAgentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		targetAgentID := req.TargetAgentID
		if normalizeSkillInstallTarget(req.InstallTarget) != "global" {
			targetAgentID, err = resolveRequestedAgentID(cfg, req.TargetAgentID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
		}

		skill, _, err := resolveSkillConfigTarget(cfg, sourceAgentID, requested)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if strings.TrimSpace(skill.Path) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "skill path not available"})
			return
		}
		if err := ensureExistingPathChainSafe(skill.Path, true); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}

		installBase, installTarget, err := resolveSkillInstallBase(cfg, targetAgentID, req.InstallTarget)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		destID := strings.TrimSpace(skill.ID)
		if destID == "" {
			destID = strings.TrimSpace(skill.SkillKey)
		}
		if destID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "skill id not available"})
			return
		}
		destDir := filepath.Join(installBase, "skills", destID)
		if err := ensureClawHubInstallTarget(installBase, destDir); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if _, err := os.Stat(destDir); err == nil {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": fmt.Sprintf("skill %s is already installed", destID)})
			return
		} else if !os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := copySkillDirectory(skill.Path, destDir); err != nil {
			_ = os.RemoveAll(destDir)
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":            true,
			"skillId":       destID,
			"skillKey":      skill.SkillKey,
			"sourceAgentId": sourceAgentID,
			"targetAgentId": targetAgentID,
			"installTarget": installTarget,
		})
	}
}

// GetCronJobs returns cron jobs from openclaw.json cron.jobs.
func GetCronJobs(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		oc, _ := cfg.ReadOpenClawJSON()
		jobs := make([]interface{}, 0)
		if oc != nil {
			if cronCfg, ok := oc["cron"].(map[string]interface{}); ok {
				if arr, ok := cronCfg["jobs"].([]interface{}); ok {
					jobs = arr
				}
			}
		}
		if len(jobs) == 0 {
			if raw, err := os.ReadFile(filepath.Join(cfg.OpenClawDir, "cron", "jobs.json")); err == nil {
				var saved map[string]interface{}
				if json.Unmarshal(raw, &saved) == nil {
					if arr, ok := saved["jobs"].([]interface{}); ok {
						jobs = arr
					}
				}
			}
		}
		// T12: Normalize legacy delivery fields → canonical delivery on read
		for i, raw := range jobs {
			job, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			// If job already has delivery.mode, keep it and normalize webhook fields
			// to canonical delivery.to (legacy delivery.url remains read-compatible).
			if del, ok := job["delivery"].(map[string]interface{}); ok {
				if m, hasMode := del["mode"]; hasMode {
					mode := strings.ToLower(strings.TrimSpace(toString(m)))
					if mode != "" {
						del["mode"] = mode
						if mode == "webhook" {
							target := strings.TrimSpace(toString(del["to"]))
							if target == "" {
								target = strings.TrimSpace(toString(del["url"]))
							}
							if target != "" {
								del["to"] = target
							}
							delete(del, "url")
						}
						job["delivery"] = del
						jobs[i] = job
						continue
					}
				}
				// Incomplete delivery object (no mode) — remove so legacy promotion runs
				delete(job, "delivery")
			}
			// Promote from legacy payload.deliver / payload.channel / payload.to
			payload, _ := job["payload"].(map[string]interface{})
			if payload == nil {
				continue
			}
			deliver, _ := payload["deliver"].(bool)
			channel, _ := payload["channel"].(string)
			to, _ := payload["to"].(string)
			bestEffort, _ := payload["bestEffortDeliver"].(bool)
			if deliver || channel != "" || to != "" {
				del := map[string]interface{}{"mode": "announce"}
				if channel != "" {
					del["channel"] = channel
				}
				if to != "" {
					del["to"] = to
				}
				if bestEffort {
					del["bestEffort"] = true
				}
				job["delivery"] = del
				jobs[i] = job
			}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "jobs": jobs})
	}
}

// SaveCronJobs replaces cron.jobs in openclaw.json.
func SaveCronJobs(cfg *config.Config) gin.HandlerFunc {
	type reqBody struct {
		Jobs []map[string]interface{} `json:"jobs"`
	}
	return func(c *gin.Context) {
		var req reqBody
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		openClawPath := filepath.Join(cfg.OpenClawDir, "openclaw.json")
		originalOpenClawJSON, err := os.ReadFile(openClawPath)
		if err != nil && !os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		oc, _ := cfg.ReadOpenClawJSON()
		if oc == nil {
			oc = map[string]interface{}{}
		}
		if err := validateCronJobsSessionTargets(cfg, req.Jobs); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		cronCfg := asMapAny(oc["cron"])
		list := make([]interface{}, 0, len(req.Jobs))
		for _, job := range req.Jobs {
			copyJob := make(map[string]interface{}, len(job))
			for k, v := range job {
				copyJob[k] = v
			}
			// Normalise missing/empty sessionTarget to the official default ("main").
			existingTarget, _ := copyJob["sessionTarget"].(string)
			if strings.TrimSpace(existingTarget) == "" {
				copyJob["sessionTarget"] = "main"
			}
			list = append(list, copyJob)
		}
		cronCfg["jobs"] = list
		oc["cron"] = cronCfg
		if err := cfg.WriteOpenClawJSON(oc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := writeCronJobsFile(cfg, list); err != nil {
			if restoreErr := restoreFile(openClawPath, originalOpenClawJSON); restoreErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error() + "; 回滚 openclaw.json 失败: " + restoreErr.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func scanPluginDir(path, id string, pluginEntries, installs map[string]interface{}) (pluginInfo, bool) {
	pj := filepath.Join(path, "openclaw.plugin.json")
	b, err := os.ReadFile(pj)
	if err != nil {
		return pluginInfo{}, false
	}
	var meta map[string]interface{}
	if err := json.Unmarshal(b, &meta); err != nil {
		return pluginInfo{}, false
	}
	entry := asMapAny(pluginEntries[id])
	enabled := true
	if v, ok := entry["enabled"].(bool); ok {
		enabled = v
	} else if v, ok := installs[id].(bool); ok {
		enabled = v
	}
	install := asMapAny(installs[id])
	version := trimmedString(meta["version"], trimmedString(install["version"], ""))
	return pluginInfo{
		ID:          id,
		Name:        trimmedString(meta["name"], id),
		Description: trimmedString(meta["description"], ""),
		Version:     version,
		Enabled:     enabled,
		Path:        path,
	}, true
}

func discoverPlugins(cfg *config.Config, pluginEntries, pluginInstalls map[string]interface{}) []pluginInfo {
	candidates := collectPluginDiscoveryCandidates(cfg, pluginInstalls)
	plugins := make([]pluginInfo, 0, len(candidates))
	seenPaths := map[string]bool{}
	seenIDs := map[string]bool{}
	for _, candidate := range candidates {
		candidate.Path = filepath.Clean(candidate.Path)
		if candidate.Path == "" || seenPaths[candidate.Path] {
			continue
		}
		seenPaths[candidate.Path] = true
		id := strings.TrimSpace(candidate.ID)
		if id == "" {
			id = filepath.Base(candidate.Path)
		}
		plugin, ok := scanPluginDir(candidate.Path, id, pluginEntries, pluginInstalls)
		if !ok || seenIDs[plugin.ID] {
			continue
		}
		plugin.Source = candidate.Source
		seenIDs[plugin.ID] = true
		plugins = append(plugins, plugin)
	}
	for pluginID, raw := range pluginEntries {
		if seenIDs[pluginID] {
			continue
		}
		entry := asMapAny(raw)
		install := asMapAny(pluginInstalls[pluginID])
		enabled := true
		if value, ok := entry["enabled"].(bool); ok {
			enabled = value
		}
		plugins = append(plugins, pluginInfo{
			ID:          pluginID,
			Name:        trimmedString(entry["name"], pluginID),
			Description: trimmedString(entry["description"], ""),
			Version:     trimmedString(install["version"], ""),
			Enabled:     enabled,
			Source:      "config",
			Path:        trimmedString(install["installPath"], ""),
		})
	}
	return plugins
}

func collectPluginDiscoveryCandidates(cfg *config.Config, pluginInstalls map[string]interface{}) []pluginDiscoveryCandidate {
	candidates := make([]pluginDiscoveryCandidate, 0)
	added := map[string]bool{}
	addCandidate := func(id, path, source string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if !filepath.IsAbs(path) {
			path = expandSkillPath(filepath.Dir(cfg.OpenClawDir), path)
		}
		path = filepath.Clean(path)
		if added[path] {
			return
		}
		added[path] = true
		candidates = append(candidates, pluginDiscoveryCandidate{ID: id, Path: path, Source: source})
	}

	for pluginID, raw := range pluginInstalls {
		install := asMapAny(raw)
		installPath := trimmedString(install["installPath"], "")
		addCandidate(pluginID, installPath, normalizePluginSource(trimmedString(install["source"], ""), installPath))
	}

	roots := []string{
		filepath.Join(cfg.OpenClawDir, "extensions"),
		filepath.Join(cfg.OpenClawDir, "plugins"),
		filepath.Join(cfg.OpenClawDir, "node_modules"),
	}
	if appRoot := strings.TrimSpace(cfg.OpenClawApp); appRoot != "" {
		roots = append(roots,
			filepath.Join(appRoot, "extensions"),
			filepath.Join(appRoot, "plugins"),
			filepath.Join(appRoot, "node_modules"),
		)
	}
	roots = uniqueStrings(roots)
	for _, root := range roots {
		if root == "." || root == "" {
			continue
		}
		for _, candidate := range listPluginCandidatesUnderRoot(root) {
			addCandidate(candidate.ID, candidate.Path, candidate.Source)
		}
	}
	return candidates
}

func listPluginCandidatesUnderRoot(root string) []pluginDiscoveryCandidate {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	candidates := make([]pluginDiscoveryCandidate, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		child := filepath.Join(root, name)
		if strings.HasPrefix(name, "@") {
			scopedEntries, err := os.ReadDir(child)
			if err != nil {
				continue
			}
			for _, scoped := range scopedEntries {
				if !scoped.IsDir() {
					continue
				}
				candidates = append(candidates, pluginDiscoveryCandidate{
					ID:     name + "/" + scoped.Name(),
					Path:   filepath.Join(child, scoped.Name()),
					Source: pluginCandidateSource(root),
				})
			}
			continue
		}
		candidates = append(candidates, pluginDiscoveryCandidate{ID: name, Path: child, Source: pluginCandidateSource(root)})
	}
	return candidates
}

func pluginCandidateSource(root string) string {
	root = filepath.Clean(root)
	switch filepath.Base(root) {
	case "node_modules":
		return "config-ext"
	default:
		return "installed"
	}
}

func normalizePluginSource(source, path string) string {
	switch strings.TrimSpace(source) {
	case "config", "config-ext", "installed":
		return strings.TrimSpace(source)
	}
	if filepath.Base(filepath.Dir(path)) == "node_modules" || filepath.Base(path) == "node_modules" {
		return "config-ext"
	}
	return "installed"
}

func resolveSkillDiscoveryRoots(cfg *config.Config, ocConfig map[string]interface{}, plugins []pluginInfo, agentID string) []skillDiscoveryRoot {
	workspace := resolveSkillsWorkspace(cfg, agentID)
	baseDir := filepath.Dir(cfg.OpenClawDir)
	roots := make([]skillDiscoveryRoot, 0, 8)

	for _, dir := range uniqueStrings(resolveExtraSkillDirs(cfg, ocConfig, workspace)) {
		roots = append(roots, skillDiscoveryRoot{Dir: dir, Source: "extra-dir"})
	}
	roots = append(roots, resolvePluginSkillDirs(baseDir, plugins)...)

	// Respect skills.allowBundled: false — skip bundled root when explicitly disabled.
	allowBundled := true
	skillsCfg := asMapAny(ocConfig["skills"])
	if v, ok := skillsCfg["allowBundled"].(bool); ok {
		allowBundled = v
	}
	if allowBundled {
		bundled := filepath.Join(resolveBundledSkillsBase(cfg), "skills")
		roots = append(roots, skillDiscoveryRoot{Dir: bundled, Source: "app-skill"})
	}

	managed := filepath.Join(cfg.OpenClawDir, "skills")
	roots = append(roots, skillDiscoveryRoot{Dir: managed, Source: "managed"})
	globalAgent := expandSkillPath(baseDir, "~/.agents/skills")
	roots = append(roots, skillDiscoveryRoot{Dir: globalAgent, Source: "global-agent"})
	if workspace != "" {
		roots = append(roots,
			skillDiscoveryRoot{Dir: filepath.Join(workspace, ".agents", "skills"), Source: "workspace-agent"},
			skillDiscoveryRoot{Dir: filepath.Join(workspace, "skills"), Source: "workspace"},
		)
	}
	return roots
}

func resolveSkillsWorkspace(cfg *config.Config, agentID string) string {
	if workspace := resolveAgentWorkspacePath(cfg, agentID); workspace != "" {
		return workspace
	}
	if cfg.OpenClawWork != "" {
		return cfg.OpenClawWork
	}
	return filepath.Join(filepath.Dir(cfg.OpenClawDir), "work")
}

func normalizeSkillInstallTarget(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "global") {
		return "global"
	}
	return "agent"
}

func resolveSkillInstallBase(cfg *config.Config, agentID, installTarget string) (string, string, error) {
	normalized := normalizeSkillInstallTarget(installTarget)
	if normalized == "global" {
		base := filepath.Clean(strings.TrimSpace(cfg.OpenClawDir))
		if base == "" || base == "." {
			return "", normalized, fmt.Errorf("openclaw dir not configured")
		}
		return base, normalized, nil
	}
	workspace := resolveSkillsWorkspace(cfg, agentID)
	if workspace == "" {
		return "", normalized, fmt.Errorf("workspace not configured")
	}
	return workspace, normalized, nil
}

func resolveRequestedAgentID(cfg *config.Config, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		requested = loadDefaultAgentID(cfg)
		if requested == "" {
			requested = "main"
		}
	}
	if strings.Contains(requested, "/") || strings.Contains(requested, "\\") || strings.Contains(requested, "..") {
		return "", fmt.Errorf("invalid agentId %q", requested)
	}
	_, agentSet := loadAgentIDs(cfg)
	if len(agentSet) == 0 {
		agentSet = map[string]struct{}{"main": {}}
	}
	if _, ok := agentSet[requested]; !ok {
		return "", fmt.Errorf("agentId %q 不存在于当前 Agent 列表", requested)
	}
	return requested, nil
}

func resolveBundledSkillsBase(cfg *config.Config) string {
	if strings.TrimSpace(cfg.OpenClawApp) != "" {
		return cfg.OpenClawApp
	}
	return filepath.Join(filepath.Dir(cfg.OpenClawDir), "app")
}

func resolveExtraSkillDirs(cfg *config.Config, ocConfig map[string]interface{}, workspace string) []string {
	baseDir := filepath.Dir(cfg.OpenClawDir)
	roots := make([]string, 0)
	for _, raw := range asStringSlice(asMapAny(asMapAny(ocConfig["skills"])["load"])["extraDirs"]) {
		if dir := expandSkillPath(baseDir, raw); dir != "" {
			roots = append(roots, dir)
		}
	}
	if workspace != "" {
		for _, raw := range asStringSlice(asMapAny(asMapAny(ocConfig["skill"])["load"])["extraDirs"]) {
			if dir := expandSkillPath(workspace, raw); dir != "" {
				roots = append(roots, dir)
			}
		}
	}
	return roots
}

func resolvePluginSkillDirs(baseDir string, plugins []pluginInfo) []skillDiscoveryRoot {
	roots := make([]skillDiscoveryRoot, 0)
	seen := map[string]bool{}
	for _, plugin := range plugins {
		if !plugin.Enabled {
			continue
		}
		manifestPath := filepath.Join(plugin.Path, "openclaw.plugin.json")
		manifestBytes, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest map[string]interface{}
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			continue
		}
		skillDirs := asStringSlice(manifest["skills"])
		if len(skillDirs) == 0 {
			fallback := filepath.Join(plugin.Path, "skills")
			if info, err := os.Stat(fallback); err == nil && info.IsDir() {
				skillDirs = []string{"skills"}
			}
		}
		for _, raw := range skillDirs {
			raw = strings.TrimSpace(raw)
			if raw == "" || filepath.IsAbs(raw) {
				continue
			}
			resolved := filepath.Clean(filepath.Join(plugin.Path, raw))
			rel, err := filepath.Rel(plugin.Path, resolved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				continue
			}
			if resolved == "" || seen[resolved] {
				continue
			}
			seen[resolved] = true
			roots = append(roots, skillDiscoveryRoot{Dir: resolved, Source: "plugin-skill"})
		}
		_ = baseDir
	}
	return roots
}

func resolveBundledSkillAllowlist(ocConfig map[string]interface{}) map[string]struct{} {
	allowlist := asStringSlice(asMapAny(ocConfig["skills"])["allowBundled"])
	if len(allowlist) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(allowlist))
	for _, item := range allowlist {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		set[item] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func discoverSkills(roots []skillDiscoveryRoot, skillEntries map[string]interface{}, legacyBlocklist map[string]bool, bundledAllowlist map[string]struct{}) []skillInfo {
	skills := make([]skillInfo, 0)
	positions := map[string]int{}
	seenRoots := map[string]bool{}
	for _, root := range roots {
		if root.Dir == "" {
			continue
		}
		resolved := filepath.Clean(root.Dir)
		if seenRoots[resolved] {
			continue
		}
		seenRoots[resolved] = true
		scanSkillRoot(resolved, root.Source, &skills, positions, skillEntries, legacyBlocklist)
	}
	if len(bundledAllowlist) > 0 {
		filtered := make([]skillInfo, 0, len(skills))
		for _, skill := range skills {
			if skill.Source == "app-skill" {
				if _, ok := bundledAllowlist[skill.SkillKey]; !ok {
					if _, ok := bundledAllowlist[skill.Name]; !ok {
						continue
					}
				}
			}
			filtered = append(filtered, skill)
		}
		skills = filtered
	}
	sort.Slice(skills, func(i, j int) bool {
		left := strings.ToLower(skills[i].Name)
		right := strings.ToLower(skills[j].Name)
		if left == right {
			return skills[i].SkillKey < skills[j].SkillKey
		}
		return left < right
	})
	return skills
}

func listDiscoverableSkillKeys(root, source string) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, skill := range discoverSkills([]skillDiscoveryRoot{{Dir: root, Source: source}}, nil, nil, nil) {
		for _, key := range []string{
			strings.TrimSpace(skill.ID),
			strings.TrimSpace(skill.SkillKey),
			strings.TrimSpace(filepath.Base(skill.Path)),
		} {
			if key == "" {
				continue
			}
			keys[key] = struct{}{}
		}
	}
	return keys
}

func scanSkillRoot(root, source string, skills *[]skillInfo, positions map[string]int, skillEntries map[string]interface{}, legacyBlocklist map[string]bool) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return
	}
	// Resolve the canonical real path of the root once so the recursive scanner
	// can guard every child against symlink-based path escapes.
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = filepath.Clean(root)
	}
	count := 0
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err == nil {
		if skill, ok := parseSkillInfo(root, source, skillEntries, legacyBlocklist); ok {
			upsertSkill(skills, positions, skill)
			count++
		}
	}
	if count >= maxSkillsPerRoot {
		return
	}
	scanSkillDirRecursive(root, realRoot, source, 0, &count, skills, positions, skillEntries, legacyBlocklist)
}

func scanSkillDirRecursive(dir, realRoot, source string, depth int, count *int, skills *[]skillInfo, positions map[string]int, skillEntries map[string]interface{}, legacyBlocklist map[string]bool) {
	if depth >= maxSkillScanDepth || *count >= maxSkillsPerRoot {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	childNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if shouldSkipSkillDir(name) {
			continue
		}
		childNames = append(childNames, name)
	}
	sort.Strings(childNames)
	for _, name := range childNames {
		if *count >= maxSkillsPerRoot {
			return
		}
		child := filepath.Join(dir, name)
		// Symlink escape guard: resolve child to its real path and ensure it
		// remains within the declared scan root. This prevents a crafted
		// symlink from traversing outside the intended directory tree.
		realChild, err := filepath.EvalSymlinks(child)
		if err != nil {
			continue // broken or inaccessible symlink – skip
		}
		rel, err := filepath.Rel(realRoot, realChild)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue // resolved path escapes the declared scan root – skip
		}
		if _, err := os.Stat(filepath.Join(child, "SKILL.md")); err == nil {
			if skill, ok := parseSkillInfo(child, source, skillEntries, legacyBlocklist); ok {
				upsertSkill(skills, positions, skill)
				*count++
			}
			continue
		}
		scanSkillDirRecursive(child, realRoot, source, depth+1, count, skills, positions, skillEntries, legacyBlocklist)
	}
}

func parseSkillInfo(skillPath, source string, skillEntries map[string]interface{}, legacyBlocklist map[string]bool) (skillInfo, bool) {
	id := filepath.Base(skillPath)
	mdPath := filepath.Join(skillPath, "SKILL.md")
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil || len(mdBytes) > maxSkillFileSize {
		return skillInfo{}, false
	}
	body := string(mdBytes)
	frontmatter := map[string]interface{}{}
	if match := skillFrontmatterRegexp.FindStringSubmatch(body); len(match) == 2 {
		_ = yaml.Unmarshal([]byte(match[1]), &frontmatter)
		body = body[len(match[0]):]
	}

	pkg := map[string]interface{}{}
	if packageBytes, err := os.ReadFile(filepath.Join(skillPath, "package.json")); err == nil {
		_ = json.Unmarshal(packageBytes, &pkg)
	}

	manifest := map[string]interface{}{}
	if manifestBytes, err := os.ReadFile(filepath.Join(skillPath, "skill.json")); err == nil && len(manifestBytes) <= maxSkillFileSize {
		_ = json.Unmarshal(manifestBytes, &manifest)
	}

	openClawMeta := resolveOpenClawMetadata(frontmatter)
	skillKey := trimmedString(openClawMeta["skillKey"], id)
	if skillKey == "" {
		skillKey = id
	}

	requiresMap := asMapAny(openClawMeta["requires"])
	requires := map[string]interface{}{}
	if env := asStringSlice(requiresMap["env"]); len(env) > 0 {
		requires["env"] = env
	}
	if bins := asStringSlice(requiresMap["bins"]); len(bins) > 0 {
		requires["bins"] = bins
	}
	if anyBins := asStringSlice(requiresMap["anyBins"]); len(anyBins) > 0 {
		requires["anyBins"] = anyBins
	}
	configSchema := normalizeSkillConfigSchema(manifest["config"])
	configKeys := asStringSlice(requiresMap["config"])
	if len(configSchema) > 0 {
		configKeys = uniqueStrings(append(configKeys, skillConfigKeys(configSchema)...))
	}
	if len(configKeys) > 0 {
		requires["config"] = configKeys
	}

	metadata := map[string]interface{}{}
	if len(openClawMeta) > 0 {
		metadata["openclaw"] = openClawMeta
	}
	if manifestMeta := compactSkillManifestMetadata(manifest); len(manifestMeta) > 0 {
		metadata["skill"] = manifestMeta
	}

	enabled := resolveSkillEnabled(skillEntries, legacyBlocklist, skillKey, id)
	skill := skillInfo{
		ID:           id,
		Name:         resolveSkillName(frontmatter, manifest, pkg, id),
		Description:  resolveSkillDescription(frontmatter, manifest, pkg, body),
		Version:      resolveLocalSkillVersion(pkg, manifest, skillPath),
		Enabled:      enabled,
		Path:         skillPath,
		SkillKey:     skillKey,
		Source:       source,
		ConfigSchema: configSchema,
	}
	if len(metadata) > 0 {
		skill.Metadata = metadata
	}
	if len(requires) > 0 {
		skill.Requires = requires
	}
	return skill, true
}

func compactSkillManifestMetadata(manifest map[string]interface{}) map[string]interface{} {
	if len(manifest) == 0 {
		return nil
	}
	out := map[string]interface{}{}
	for _, key := range []string{"name", "displayName", "description", "version", "author", "entry", "language"} {
		if value := trimmedString(manifest[key], ""); value != "" {
			out[key] = value
		}
	}
	if tags := asStringSlice(manifest["tags"]); len(tags) > 0 {
		out["tags"] = tags
	}
	return out
}

func normalizeSkillConfigSchema(value interface{}) []skillConfigField {
	items, ok := value.([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]skillConfigField, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		fieldMap := asMapAny(item)
		key := trimmedString(fieldMap["key"], "")
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		field := skillConfigField{
			Key:         key,
			Label:       trimmedString(fieldMap["label"], ""),
			Type:        normalizeSkillConfigType(trimmedString(fieldMap["type"], "text")),
			Placeholder: trimmedString(fieldMap["placeholder"], ""),
			Help:        trimmedString(fieldMap["help"], ""),
			Required:    skillManifestBool(fieldMap["required"]),
			Options:     normalizeSkillConfigOptions(fieldMap["options"]),
		}
		if defaultValue, ok := fieldMap["defaultValue"]; ok {
			field.DefaultValue = defaultValue
		} else if defaultValue, ok := fieldMap["default"]; ok {
			field.DefaultValue = defaultValue
		}
		out = append(out, field)
	}
	return out
}

func normalizeSkillConfigType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "text", "password", "textarea", "select", "toggle", "number":
		return value
	default:
		return "text"
	}
}

func normalizeSkillConfigOptions(value interface{}) []skillConfigOption {
	switch typed := value.(type) {
	case []interface{}:
		out := make([]skillConfigOption, 0, len(typed))
		for _, item := range typed {
			switch option := item.(type) {
			case map[string]interface{}:
				if rawValue, ok := option["value"]; ok {
					out = append(out, skillConfigOption{Label: trimmedString(option["label"], trimmedString(rawValue, "")), Value: rawValue})
					continue
				}
			case string:
				option = strings.TrimSpace(option)
				if option != "" {
					out = append(out, skillConfigOption{Label: option, Value: option})
				}
				continue
			}
			text := trimmedString(item, "")
			if text != "" {
				out = append(out, skillConfigOption{Label: text, Value: text})
			}
		}
		return out
	default:
		return nil
	}
}

func skillConfigKeys(fields []skillConfigField) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Key != "" {
			out = append(out, field.Key)
		}
	}
	return out
}

func resolveSkillName(frontmatter, manifest, pkg map[string]interface{}, fallback string) string {
	for _, value := range []interface{}{frontmatter["name"], manifest["displayName"], manifest["name"], pkg["name"]} {
		if name := trimmedString(value, ""); name != "" {
			return name
		}
	}
	return fallback
}

func resolveOpenClawMetadata(frontmatter map[string]interface{}) map[string]interface{} {
	metadata := asMapAny(frontmatter["metadata"])
	for _, key := range []string{"openclaw", "clawdbot", "clawdis"} {
		if meta := asMapAny(metadata[key]); len(meta) > 0 {
			return meta
		}
	}
	for _, key := range []string{"openclaw", "clawdbot", "clawdis"} {
		if meta := asMapAny(frontmatter[key]); len(meta) > 0 {
			return meta
		}
	}
	return map[string]interface{}{}
}

func resolveSkillDescription(frontmatter, manifest, pkg map[string]interface{}, body string) string {
	if desc := trimmedString(frontmatter["description"], ""); desc != "" {
		return desc
	}
	if desc := trimmedString(manifest["description"], ""); desc != "" {
		return desc
	}
	if desc := trimmedString(pkg["description"], ""); desc != "" {
		return desc
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}

func resolveLocalSkillVersion(pkg, manifest map[string]interface{}, skillPath string) string {
	if version := trimmedString(pkg["version"], ""); version != "" {
		return version
	}
	if version := trimmedString(manifest["version"], ""); version != "" {
		return version
	}
	raw, err := os.ReadFile(filepath.Join(skillPath, ".clawhub", "origin.json"))
	if err != nil {
		return ""
	}
	var origin clawHubOriginFile
	if err := json.Unmarshal(raw, &origin); err != nil {
		return ""
	}
	return strings.TrimSpace(origin.InstalledVersion)
}

func resolveSkillEnabled(skillEntries map[string]interface{}, legacyBlocklist map[string]bool, keys ...string) bool {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if entry := asMapAny(skillEntries[key]); len(entry) > 0 {
			if enabled, ok := entry["enabled"].(bool); ok {
				return enabled
			}
		}
	}
	for _, key := range keys {
		if key != "" && legacyBlocklist[key] {
			return false
		}
	}
	return true
}

func upsertSkill(skills *[]skillInfo, positions map[string]int, skill skillInfo) {
	key := skill.SkillKey
	if key == "" {
		key = skill.ID
	}
	if idx, ok := positions[key]; ok {
		(*skills)[idx] = skill
		return
	}
	positions[key] = len(*skills)
	*skills = append(*skills, skill)
}

func shouldSkipSkillDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "dist", "build", "coverage", "vendor":
		return true
	default:
		return false
	}
}

func readLegacySkillBlocklist(skillsCfg map[string]interface{}) map[string]bool {
	blocked := map[string]bool{}
	for _, value := range asStringSlice(skillsCfg["blocklist"]) {
		blocked[value] = true
	}
	return blocked
}

func removeLegacyBlocklistEntries(skillsCfg map[string]interface{}, keys ...string) {
	blocklist := asStringSlice(skillsCfg["blocklist"])
	if len(blocklist) == 0 {
		return
	}
	removeSet := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			removeSet[key] = struct{}{}
		}
	}
	filtered := make([]string, 0, len(blocklist))
	for _, item := range blocklist {
		if _, ok := removeSet[item]; !ok {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		delete(skillsCfg, "blocklist")
		return
	}
	values := make([]interface{}, 0, len(filtered))
	for _, item := range filtered {
		values = append(values, item)
	}
	skillsCfg["blocklist"] = values
}

func validateCronJobsSessionTargets(cfg *config.Config, jobs []map[string]interface{}) error {
	_, agentSet := loadAgentIDs(cfg)
	if len(agentSet) == 0 {
		agentSet = map[string]struct{}{"main": {}}
	}
	for _, job := range jobs {
		if rawAgentID, exists := job["agentId"]; exists {
			switch v := rawAgentID.(type) {
			case nil:
				delete(job, "agentId")
			case string:
				agentID := strings.TrimSpace(v)
				if agentID == "" {
					delete(job, "agentId")
				} else {
					if _, ok := agentSet[agentID]; !ok {
						return fmt.Errorf("agentId %q 不存在于当前 Agent 列表", agentID)
					}
					job["agentId"] = agentID
				}
			default:
				return fmt.Errorf("agentId 必须是字符串")
			}
		}
		// Use type assertion to safely handle nil (missing key) and non-string values.
		target, _ := job["sessionTarget"].(string)
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		// Official semantics: sessionTarget is an execution mode ("main" or "isolated"),
		// not an agent ID. Accept the two official values unconditionally.
		if target == "main" || target == "isolated" {
			// T10: validate sessionTarget↔payload.kind constraint
			if payloadMap, ok := job["payload"].(map[string]interface{}); ok {
				payloadKind, _ := payloadMap["kind"].(string)
				if payloadKind == "" {
					// Auto-fill missing payload.kind from sessionTarget
					if target == "main" {
						payloadMap["kind"] = "systemEvent"
					} else {
						payloadMap["kind"] = "agentTurn"
					}
				} else {
					if target == "main" && payloadKind != "systemEvent" {
						return fmt.Errorf("sessionTarget=main 只允许 payload.kind=systemEvent，当前为 %q", payloadKind)
					}
					if target == "isolated" && payloadKind != "agentTurn" {
						return fmt.Errorf("sessionTarget=isolated 只允许 payload.kind=agentTurn，当前为 %q", payloadKind)
					}
				}
			}
			// T10: non-default agent must not use main session
			if target == "main" {
				if agentID, _ := job["agentId"].(string); agentID != "" && agentID != "main" {
					defaultAgentID := loadDefaultAgentID(cfg)
					if agentID != defaultAgentID {
						return fmt.Errorf("非默认 Agent %q 不能使用 main session", agentID)
					}
				}
			}
			continue
		}
		// Backward-compat: if the value is a known agent ID (legacy behaviour), migrate it
		// into agentId and normalise sessionTarget to the official default ("main").
		if _, isAgent := agentSet[target]; isAgent {
			if _, hasAgentID := job["agentId"]; !hasAgentID {
				job["agentId"] = target
			}
			job["sessionTarget"] = "main"
			continue
		}
		return fmt.Errorf("sessionTarget %q 无效，有效值为 \"main\" 或 \"isolated\"", target)
	}
	// Validate delivery configuration
	for _, job := range jobs {
		if deliveryMap, ok := job["delivery"].(map[string]interface{}); ok {
			mode := strings.ToLower(strings.TrimSpace(toString(deliveryMap["mode"])))
			target := strings.TrimSpace(toString(job["sessionTarget"]))
			if target == "" {
				// SaveCronJobs will normalize empty sessionTarget to "main".
				target = "main"
			}
			switch mode {
			case "":
				return fmt.Errorf("delivery.mode 不能为空")
			case "none":
				deliveryMap["mode"] = "none"
			case "announce":
				if target == "main" {
					// Backward-compat: official runtime strips non-webhook delivery on
					// main session; normalize legacy announce configs to none.
					deliveryMap["mode"] = "none"
					for _, field := range []string{"channel", "to", "accountId", "bestEffort", "url", "failureDestination"} {
						delete(deliveryMap, field)
					}
					continue
				}
				deliveryMap["mode"] = "announce"
			case "webhook":
				target := strings.TrimSpace(toString(deliveryMap["to"]))
				if target == "" {
					target = strings.TrimSpace(toString(deliveryMap["url"]))
				}
				if target == "" {
					return fmt.Errorf("delivery.mode=webhook 时必须指定非空 to")
				}
				targetLower := strings.ToLower(target)
				if !strings.HasPrefix(targetLower, "http://") && !strings.HasPrefix(targetLower, "https://") {
					return fmt.Errorf("webhook to 必须以 http:// 或 https:// 开头")
				}
				for _, blocked := range []string{"://localhost", "://127.0.0.1", "://0.0.0.0", "://[::1]", "://169.254.169.254"} {
					if strings.Contains(targetLower, blocked) {
						return fmt.Errorf("webhook to 不允许指向内部地址: %s", target)
					}
				}
				deliveryMap["mode"] = "webhook"
				deliveryMap["to"] = target
				delete(deliveryMap, "url")
			default:
				return fmt.Errorf("delivery.mode %q 无效，有效值为 none/announce/webhook", mode)
			}
		}
	}
	// T11: validate schedule structure
	for _, job := range jobs {
		if schedMap, ok := job["schedule"].(map[string]interface{}); ok {
			kind, _ := schedMap["kind"].(string)
			switch kind {
			case "cron":
				if expr, _ := schedMap["expr"].(string); strings.TrimSpace(expr) == "" {
					return fmt.Errorf("cron 类型 schedule 必须包含非空 expr")
				}
			case "every":
				// accept either everyMs (number) or every (string)
				hasEveryMs := false
				if v, ok := schedMap["everyMs"]; ok {
					if num, ok := v.(float64); ok && num > 0 {
						hasEveryMs = true
					}
				}
				hasEvery := false
				if v, ok := schedMap["every"].(string); ok && v != "" {
					hasEvery = true
				}
				if !hasEveryMs && !hasEvery {
					return fmt.Errorf("every 类型 schedule 必须包含正数 everyMs 或非空 every")
				}
			case "at":
				// accept either at (ISO string) or atMs (number)
				hasAt := false
				if v, ok := schedMap["at"].(string); ok && v != "" {
					if _, err := time.Parse(time.RFC3339, v); err != nil {
						return fmt.Errorf("schedule.at %q 不是有效的 ISO 8601 时间格式 (RFC3339)", v)
					}
					hasAt = true
				}
				hasAtMs := false
				if v, ok := schedMap["atMs"]; ok {
					if num, ok := v.(float64); ok && num > 0 {
						hasAtMs = true
					}
				}
				if !hasAt && !hasAtMs {
					return fmt.Errorf("at 类型 schedule 必须包含 ISO 8601 字符串 at 或正数 atMs")
				}
			case "":
				// no kind → skip validation
			default:
				return fmt.Errorf("schedule.kind %q 无效，有效值为 cron/every/at", kind)
			}
		}
	}
	return nil
}

func replaceFileAtomically(dest string, raw []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, raw, mode); err != nil {
		return err
	}
	backup := dest + ".bak"
	_ = os.Remove(backup)
	hadDest := false
	if _, err := os.Stat(dest); err == nil {
		hadDest = true
		if err := os.Rename(dest, backup); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	} else if !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		if hadDest {
			_ = os.Rename(backup, dest)
		}
		return err
	}
	if hadDest {
		_ = os.Remove(backup)
	}
	return nil
}

func writeCronJobsFile(cfg *config.Config, jobs []interface{}) error {
	raw, err := json.MarshalIndent(map[string]interface{}{"jobs": jobs}, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write: write to a temp file then rename so a mid-write crash cannot
	// leave a partially-written jobs.json (mirrors openclaw store.ts behaviour).
	dest := filepath.Join(cfg.OpenClawDir, "cron", "jobs.json")
	return replaceFileAtomically(dest, raw, 0644)
}

func expandSkillPath(baseDir, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "~/") || raw == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if raw == "~" {
				return home
			}
			return filepath.Join(home, raw[2:])
		}
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	if baseDir == "" {
		return filepath.Clean(raw)
	}
	return filepath.Clean(filepath.Join(baseDir, raw))
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func resolveSkillConfigTarget(cfg *config.Config, agentID, requested string) (skillInfo, map[string]interface{}, error) {
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig == nil {
		ocConfig = map[string]interface{}{}
	}

	skillsCfg := asMapAny(ocConfig["skills"])
	skillEntries := asMapAny(skillsCfg["entries"])
	legacyBlocklist := readLegacySkillBlocklist(skillsCfg)
	pluginEntries := asMapAny(asMapAny(ocConfig["plugins"])["entries"])
	pluginInstalls := asMapAny(asMapAny(ocConfig["plugins"])["installs"])
	plugins := discoverPlugins(cfg, pluginEntries, pluginInstalls)
	roots := resolveSkillDiscoveryRoots(cfg, ocConfig, plugins, agentID)
	skills := discoverSkills(roots, skillEntries, legacyBlocklist, resolveBundledSkillAllowlist(ocConfig))
	for _, skill := range skills {
		if skill.SkillKey == requested || skill.ID == requested {
			return skill, ocConfig, nil
		}
	}
	return skillInfo{}, ocConfig, fmt.Errorf("skill %q not found", requested)
}

func copySkillDirectory(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create skill parent directory: %w", err)
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := dest
		if rel != "." {
			target = filepath.Join(dest, rel)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to copy symlinked skill entry %s", path)
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if err := copySkillFile(path, target, info.Mode().Perm()); err != nil {
			return err
		}
		return nil
	})
}

func copySkillFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	return nil
}

func configPathSegments(path string) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("invalid empty config key")
	}
	parts := strings.Split(path, ".")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("invalid config key %q", path)
		}
		segments = append(segments, part)
	}
	return segments, nil
}

func getNestedConfigValue(root map[string]interface{}, path string) (interface{}, bool, error) {
	segments, err := configPathSegments(path)
	if err != nil {
		return nil, false, err
	}
	var current interface{} = root
	for _, segment := range segments {
		object, ok := current.(map[string]interface{})
		if !ok || object == nil {
			return nil, false, nil
		}
		next, exists := object[segment]
		if !exists {
			return nil, false, nil
		}
		current = next
	}
	return current, true, nil
}

func setNestedConfigValue(root map[string]interface{}, path string, value interface{}) error {
	segments, err := configPathSegments(path)
	if err != nil {
		return err
	}
	current := root
	for _, segment := range segments[:len(segments)-1] {
		next, exists := current[segment]
		if !exists || next == nil {
			child := map[string]interface{}{}
			current[segment] = child
			current = child
			continue
		}
		child, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("config key %q collides with a non-object at %q", path, segment)
		}
		current = child
	}
	current[segments[len(segments)-1]] = value
	return nil
}

func deleteNestedConfigValue(root map[string]interface{}, path string) error {
	segments, err := configPathSegments(path)
	if err != nil {
		return err
	}
	_, err = deleteNestedConfigSegments(root, segments)
	return err
}

func deleteNestedConfigSegments(current map[string]interface{}, segments []string) (bool, error) {
	if len(segments) == 0 {
		return len(current) == 0, nil
	}
	if len(segments) == 1 {
		delete(current, segments[0])
		return len(current) == 0, nil
	}

	next, exists := current[segments[0]]
	if !exists {
		return len(current) == 0, nil
	}
	child, ok := next.(map[string]interface{})
	if !ok {
		return false, nil
	}
	empty, err := deleteNestedConfigSegments(child, segments[1:])
	if err != nil {
		return false, err
	}
	if empty {
		delete(current, segments[0])
	}
	return len(current) == 0, nil
}

func asMapAny(value interface{}) map[string]interface{} {
	if typed, ok := value.(map[string]interface{}); ok && typed != nil {
		return typed
	}
	return map[string]interface{}{}
}

func asStringSlice(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			value := strings.TrimSpace(fmt.Sprint(item))
			if value != "" {
				out = append(out, value)
			}
		}
		return out
	default:
		return nil
	}
}

func trimmedString(value interface{}, fallback string) string {
	if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
		return text
	}
	return fallback
}

func skillManifestBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}
