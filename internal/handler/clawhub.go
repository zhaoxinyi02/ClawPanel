package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

var clawHubHTTPClient = &http.Client{Timeout: 20 * time.Second}
var clawHubSlugRegexp = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

const (
	defaultClawHubRegistryBase           = "https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel-Plugins/main"
	defaultClawHubRegistryFallback       = "https://gitee.com/zxy000006/ClawPanel-Plugins/raw/main"
	defaultClawHubPublicBase             = "https://github.com/zhaoxinyi02/ClawPanel-Plugins/tree/main"
	maxClawHubArchiveBytes         int64 = 32 << 20
	maxClawHubArchiveEntries             = 512
	maxClawHubEntryBytes           int64 = 8 << 20
	maxClawHubExtractedBytes       int64 = 64 << 20
)

type clawHubRegistryConfig struct {
	RequestBase         string
	FallbackRequestBase string
	PublicBase          string
}

type customSkillRegistry struct {
	GeneratedAt string                    `json:"generated_at"`
	Skills      []customSkillRegistryItem `json:"skills"`
}

type customSkillRegistryItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      string   `json:"author,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	UpdatedAt   int64    `json:"updated_at,omitempty"`
	Path        string   `json:"path,omitempty"`
	Files       []string `json:"files,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Requires    struct {
		Env     []string `json:"env,omitempty"`
		Bins    []string `json:"bins,omitempty"`
		AnyBins []string `json:"anyBins,omitempty"`
	} `json:"requires,omitempty"`
}

type clawHubSkillItem struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Version          string `json:"version,omitempty"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	UpdatedAt        int64  `json:"updatedAt,omitempty"`
}

type clawHubLockFile struct {
	Version int                         `json:"version"`
	Skills  map[string]clawHubLockEntry `json:"skills"`
}

type clawHubLockEntry struct {
	Version     string `json:"version,omitempty"`
	InstalledAt int64  `json:"installedAt"`
}

type clawHubOriginFile struct {
	Registry         string `json:"registry"`
	Slug             string `json:"slug"`
	InstalledVersion string `json:"installedVersion"`
	InstalledAt      int64  `json:"installedAt"`
}

type clawHubSearchResponse struct {
	Total   int `json:"total"`
	Results []struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"displayName"`
		Summary     string `json:"summary"`
		Version     string `json:"version"`
		UpdatedAt   int64  `json:"updatedAt"`
	} `json:"results"`
}

type clawHubExploreResponse struct {
	Total int `json:"total"`
	Items []struct {
		Slug          string `json:"slug"`
		DisplayName   string `json:"displayName"`
		Summary       string `json:"summary"`
		UpdatedAt     int64  `json:"updatedAt"`
		LatestVersion *struct {
			Version string `json:"version"`
		} `json:"latestVersion"`
	} `json:"items"`
}

type clawHubSkillResponse struct {
	Skill *struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"displayName"`
		Summary     string `json:"summary"`
		Moderation  *struct {
			IsMalwareBlocked bool `json:"isMalwareBlocked"`
		} `json:"moderation"`
	} `json:"skill"`
	LatestVersion *struct {
		Version string `json:"version"`
	} `json:"latestVersion"`
}

// SearchClawHub searches the official ClawHub API and annotates installed status for the selected install target.
func SearchClawHub(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := strings.TrimSpace(c.Query("q"))
		agentID, err := resolveRequestedAgentID(cfg, c.Query("agentId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		limit := 30
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				if parsed > 100 {
					parsed = 100
				}
				limit = parsed
			}
		}
		page := 1
		if raw := strings.TrimSpace(c.Query("page")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				page = parsed
			}
		}

		workdir, installTarget, err := resolveSkillInstallBase(cfg, agentID, c.Query("installTarget"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := ensureClawHubStatePath(workdir); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		installed := readInstalledClawHubState(workdir)
		registry, err := resolveClawHubRegistryConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		items, total, err := fetchClawHubItems(registry, query, limit, page)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
			return
		}
		for i := range items {
			if state, ok := installed[items[i].ID]; ok {
				items[i].Installed = true
				items[i].InstalledVersion = state.Version
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":            true,
			"agentId":       agentID,
			"installTarget": installTarget,
			"registryBase":  registry.PublicBase,
			"skills":        items,
			"page":          page,
			"limit":         limit,
			"total":         total,
		})
	}
}

// UninstallSkill removes a skill from the workspace skills directory and cleans up lock file entries.
func UninstallSkill(cfg *config.Config) gin.HandlerFunc {
	type reqBody struct {
		SkillID       string `json:"skillId"`
		AgentID       string `json:"agentId"`
		InstallTarget string `json:"installTarget,omitempty"`
	}
	return func(c *gin.Context) {
		var req reqBody
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		slug := sanitizeClawHubSlug(req.SkillID)
		if slug == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid skillId"})
			return
		}
		agentID, err := resolveRequestedAgentID(cfg, req.AgentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		workdir, installTarget, err := resolveSkillInstallBase(cfg, agentID, req.InstallTarget)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		skillDir := filepath.Join(workdir, "skills", slug)

		// Validate paths are safe before removal
		if err := ensureClawHubStatePath(workdir); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		realWorkdir, err := resolveExistingRealPath(workdir)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("resolve workspace path: %s", err)})
			return
		}

		info, statErr := os.Lstat(skillDir)
		if statErr != nil || !info.IsDir() {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": fmt.Sprintf("skill %s is not installed in target", slug)})
			return
		}
		if info.Mode()&os.ModeSymlink != 0 {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "refusing to remove symlinked skill directory"})
			return
		}
		realSkillDir, err := resolveExistingRealPath(skillDir)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("resolve skill path: %s", err)})
			return
		}
		if !pathWithinBase(realWorkdir, realSkillDir) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "skill path escapes workspace"})
			return
		}

		// Remove skill directory
		if err := os.RemoveAll(skillDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": fmt.Sprintf("failed to remove skill: %s", err)})
			return
		}

		// Clean up lock file entry
		removeClawHubLockEntry(workdir, slug)

		c.JSON(http.StatusOK, gin.H{
			"ok":            true,
			"agentId":       agentID,
			"installTarget": installTarget,
			"skillId":       slug,
		})
	}
}

// CheckSkillDeps checks whether a skill's declared requirements (env vars, binaries) are satisfied.
func CheckSkillDeps(_ *config.Config) gin.HandlerFunc {
	type reqBody struct {
		Env     []string `json:"env"`
		Bins    []string `json:"bins"`
		AnyBins []string `json:"anyBins"`
	}
	type depResult struct {
		Name  string `json:"name"`
		Found bool   `json:"found"`
	}
	return func(c *gin.Context) {
		var req reqBody
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		envResults := make([]depResult, 0, len(req.Env))
		for _, e := range req.Env {
			_, found := os.LookupEnv(e)
			envResults = append(envResults, depResult{Name: e, Found: found})
		}
		binResults := make([]depResult, 0, len(req.Bins))
		for _, b := range req.Bins {
			_, err := exec.LookPath(b)
			binResults = append(binResults, depResult{Name: b, Found: err == nil})
		}
		anyBinOK := len(req.AnyBins) == 0
		anyBinResults := make([]depResult, 0, len(req.AnyBins))
		for _, b := range req.AnyBins {
			_, err := exec.LookPath(b)
			found := err == nil
			if found {
				anyBinOK = true
			}
			anyBinResults = append(anyBinResults, depResult{Name: b, Found: found})
		}
		allOK := anyBinOK
		for _, r := range envResults {
			if !r.Found {
				allOK = false
				break
			}
		}
		if allOK {
			for _, r := range binResults {
				if !r.Found {
					allOK = false
					break
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":      true,
			"allMet":  allOK,
			"env":     envResults,
			"bins":    binResults,
			"anyBins": anyBinResults,
		})
	}
}

// InstallClawHubSkill installs a public ClawHub skill into the selected agent workspace.
func InstallClawHubSkill(cfg *config.Config) gin.HandlerFunc {
	type reqBody struct {
		SkillID       string `json:"skillId"`
		AgentID       string `json:"agentId"`
		InstallTarget string `json:"installTarget,omitempty"`
		Version       string `json:"version,omitempty"`
	}
	return func(c *gin.Context) {
		var req reqBody
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		slug := sanitizeClawHubSlug(req.SkillID)
		if slug == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid skillId"})
			return
		}
		agentID, err := resolveRequestedAgentID(cfg, req.AgentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		workdir, installTarget, err := resolveSkillInstallBase(cfg, agentID, req.InstallTarget)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		registry, err := resolveClawHubRegistryConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		skillDir := filepath.Join(workdir, "skills", slug)
		if err := ensureClawHubInstallTarget(workdir, skillDir); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if info, err := os.Stat(skillDir); err == nil && info.IsDir() {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": fmt.Sprintf("skill %s is already installed", slug)})
			return
		}

		meta, err := fetchClawHubSkill(registry, slug)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
			return
		}
		version := strings.TrimSpace(req.Version)
		if version == "" && meta.LatestVersion != nil {
			version = strings.TrimSpace(meta.LatestVersion.Version)
		}
		if version == "" {
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": fmt.Sprintf("unable to resolve latest version for %s", slug)})
			return
		}

		if usesCustomSkillRegistry(registry) {
			if err := downloadCustomSkillFiles(registry, slug, skillDir); err != nil {
				_ = os.RemoveAll(skillDir)
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
		} else {
			archiveBytes, err := downloadClawHubArchive(registry.RequestBase, slug, version)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
				return
			}
			if err := extractClawHubArchive(skillDir, archiveBytes); err != nil {
				_ = os.RemoveAll(skillDir)
				c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
				return
			}
		}

		installedAt := time.Now().UnixMilli()
		if err := writeClawHubOrigin(skillDir, registry.PublicBase, slug, version, installedAt); err != nil {
			_ = os.RemoveAll(skillDir)
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := updateClawHubLock(workdir, slug, version, installedAt); err != nil {
			_ = os.RemoveAll(skillDir)
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := alignPathOwnershipToParent(skillDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if err := alignPathOwnershipToParent(filepath.Join(workdir, ".clawhub")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":            true,
			"agentId":       agentID,
			"installTarget": installTarget,
			"skillId":       slug,
			"version":       version,
			"path":          skillDir,
		})
	}
}

func fetchClawHubItems(registry clawHubRegistryConfig, query string, limit, page int) ([]clawHubSkillItem, int, error) {
	if !usesCustomSkillRegistry(registry) {
		return fetchLegacyClawHubItems(registry.RequestBase, query, limit, page)
	}
	catalog, err := fetchCustomSkillRegistry(registry)
	if err != nil {
		return nil, 0, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	items := make([]clawHubSkillItem, 0, len(catalog.Skills))
	for _, skill := range catalog.Skills {
		if q != "" {
			haystack := strings.ToLower(strings.Join([]string{skill.ID, skill.Name, skill.Description, strings.Join(skill.Tags, " ")}, " "))
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		items = append(items, clawHubSkillItem{
			ID:          skill.ID,
			Name:        firstNonEmpty(skill.Name, skill.ID),
			Description: skill.Description,
			Version:     skill.Version,
			UpdatedAt:   skill.UpdatedAt,
		})
	}
	total := len(items)
	if limit <= 0 {
		limit = 30
	}
	start := (page - 1) * limit
	if start >= total {
		return []clawHubSkillItem{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return items[start:end], total, nil
}

func fetchClawHubSkill(registry clawHubRegistryConfig, slug string) (*clawHubSkillResponse, error) {
	if !usesCustomSkillRegistry(registry) {
		return fetchLegacyClawHubSkill(registry.RequestBase, slug)
	}
	catalog, err := fetchCustomSkillRegistry(registry)
	if err != nil {
		return nil, err
	}
	for _, skill := range catalog.Skills {
		if skill.ID != slug {
			continue
		}
		payload := &clawHubSkillResponse{}
		payload.Skill = &struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"displayName"`
			Summary     string `json:"summary"`
			Moderation  *struct {
				IsMalwareBlocked bool `json:"isMalwareBlocked"`
			} `json:"moderation"`
		}{Slug: skill.ID, DisplayName: skill.Name, Summary: skill.Description}
		payload.LatestVersion = &struct {
			Version string `json:"version"`
		}{Version: skill.Version}
		return payload, nil
	}
	return nil, fmt.Errorf("skill %s not found in registry", slug)
}

func usesCustomSkillRegistry(registry clawHubRegistryConfig) bool {
	base := strings.ToLower(strings.TrimSpace(registry.RequestBase))
	return strings.Contains(base, "clawpanel-plugins") || strings.Contains(base, "/raw/") || strings.Contains(base, "raw.githubusercontent.com")
}

func fetchLegacyClawHubItems(registryBase, query string, limit, page int) ([]clawHubSkillItem, int, error) {
	offset := (page - 1) * limit
	if strings.TrimSpace(query) == "" {
		resp, err := clawHubHTTPClient.Get(registryBase + "/api/v1/skills?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(offset) + "&sort=downloads")
		if err != nil {
			return nil, 0, fmt.Errorf("search clawhub: request failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return nil, 0, fmt.Errorf("search clawhub: %s", strings.TrimSpace(string(body)))
		}
		var payload clawHubExploreResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, 0, fmt.Errorf("decode clawhub explore response: %w", err)
		}
		items := make([]clawHubSkillItem, 0, len(payload.Items))
		for _, item := range payload.Items {
			version := ""
			if item.LatestVersion != nil {
				version = strings.TrimSpace(item.LatestVersion.Version)
			}
			items = append(items, clawHubSkillItem{ID: item.Slug, Name: firstNonEmpty(item.DisplayName, item.Slug), Description: item.Summary, Version: version, UpdatedAt: item.UpdatedAt})
		}
		return items, payload.Total, nil
	}
	values := url.Values{}
	values.Set("q", query)
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	resp, err := clawHubHTTPClient.Get(registryBase + "/api/v1/search?" + values.Encode())
	if err != nil {
		return nil, 0, fmt.Errorf("search clawhub: request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, 0, fmt.Errorf("search clawhub: %s", strings.TrimSpace(string(body)))
	}
	var payload clawHubSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, fmt.Errorf("decode clawhub search response: %w", err)
	}
	items := make([]clawHubSkillItem, 0, len(payload.Results))
	for _, item := range payload.Results {
		items = append(items, clawHubSkillItem{ID: item.Slug, Name: firstNonEmpty(item.DisplayName, item.Slug), Description: item.Summary, Version: item.Version, UpdatedAt: item.UpdatedAt})
	}
	return items, payload.Total, nil
}

func fetchLegacyClawHubSkill(registryBase, slug string) (*clawHubSkillResponse, error) {
	resp, err := clawHubHTTPClient.Get(registryBase + "/api/v1/skills/" + url.PathEscape(slug))
	if err != nil {
		return nil, fmt.Errorf("get clawhub skill %s: request failed", slug)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("get clawhub skill %s: %s", slug, strings.TrimSpace(string(body)))
	}
	var payload clawHubSkillResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode clawhub skill %s: %w", slug, err)
	}
	return &payload, nil
}

func fetchCustomSkillRegistry(registry clawHubRegistryConfig) (*customSkillRegistry, error) {
	var lastErr error
	for _, base := range []string{registry.RequestBase, registry.FallbackRequestBase} {
		base = strings.TrimSpace(strings.TrimRight(base, "/"))
		if base == "" {
			continue
		}
		resp, err := clawHubHTTPClient.Get(base + "/skills/registry.json")
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			lastErr = fmt.Errorf("fetch custom registry: %s", strings.TrimSpace(string(body)))
			continue
		}
		var catalog customSkillRegistry
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxClawHubArchiveBytes)).Decode(&catalog); err != nil {
			resp.Body.Close()
			lastErr = err
			continue
		}
		resp.Body.Close()
		return &catalog, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to load custom skill registry")
	}
	return nil, lastErr
}

func downloadCustomSkillFiles(registry clawHubRegistryConfig, slug, targetDir string) error {
	catalog, err := fetchCustomSkillRegistry(registry)
	if err != nil {
		return err
	}
	var item *customSkillRegistryItem
	for i := range catalog.Skills {
		if catalog.Skills[i].ID == slug {
			item = &catalog.Skills[i]
			break
		}
	}
	if item == nil {
		return fmt.Errorf("skill %s not found in registry", slug)
	}
	files := item.Files
	if len(files) == 0 {
		files = []string{"SKILL.md"}
	}
	pathPrefix := strings.Trim(strings.TrimSpace(item.Path), "/")
	if pathPrefix == "" {
		pathPrefix = "skills/" + item.ID
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	foundSkill := false
	for _, rel := range files {
		rel = filepath.ToSlash(strings.TrimSpace(rel))
		if rel == "" || strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
			return fmt.Errorf("invalid skill file path %s", rel)
		}
		targetPath := filepath.Join(targetDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		var lastErr error
		for _, base := range []string{registry.RequestBase, registry.FallbackRequestBase} {
			base = strings.TrimSpace(strings.TrimRight(base, "/"))
			if base == "" {
				continue
			}
			resp, err := clawHubHTTPClient.Get(base + "/" + pathPrefix + "/" + rel)
			if err != nil {
				lastErr = err
				continue
			}
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
				resp.Body.Close()
				lastErr = fmt.Errorf("download %s: %s", rel, strings.TrimSpace(string(body)))
				continue
			}
			f, err := os.Create(targetPath)
			if err != nil {
				resp.Body.Close()
				return err
			}
			_, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxClawHubEntryBytes+1))
			closeErr := f.Close()
			resp.Body.Close()
			if copyErr != nil {
				lastErr = copyErr
				continue
			}
			if closeErr != nil {
				lastErr = closeErr
				continue
			}
			lastErr = nil
			break
		}
		if lastErr != nil {
			return lastErr
		}
		if rel == "SKILL.md" {
			foundSkill = true
		}
	}
	if !foundSkill {
		return fmt.Errorf("skill %s missing SKILL.md", slug)
	}
	return nil
}

func downloadClawHubArchive(registryBase, slug, version string) ([]byte, error) {
	values := url.Values{}
	values.Set("slug", slug)
	if strings.TrimSpace(version) != "" {
		values.Set("version", version)
	}
	resp, err := clawHubHTTPClient.Get(registryBase + "/api/v1/download?" + values.Encode())
	if err != nil {
		return nil, fmt.Errorf("download clawhub skill %s: request failed", slug)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download clawhub skill %s: %s", slug, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxClawHubArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read clawhub download %s: %w", slug, err)
	}
	if int64(len(body)) > maxClawHubArchiveBytes {
		return nil, fmt.Errorf("download clawhub skill %s: archive too large", slug)
	}
	return body, nil
}

func extractClawHubArchive(targetDir string, archiveBytes []byte) error {
	reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return fmt.Errorf("open clawhub archive: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}
	if len(reader.File) > maxClawHubArchiveEntries {
		return fmt.Errorf("archive contains too many files")
	}
	wrapperPrefix := detectClawHubArchivePrefix(reader.File)
	var totalWritten int64
	foundSkillFile := false
	for _, file := range reader.File {
		relPath := normalizeClawHubArchivePath(file.Name)
		if relPath == "." || relPath == "" || strings.HasPrefix(relPath, "__MACOSX"+string(os.PathSeparator)) {
			continue
		}
		if wrapperPrefix != "" {
			if relPath == wrapperPrefix {
				continue
			}
			if strings.HasPrefix(relPath, wrapperPrefix+string(os.PathSeparator)) {
				relPath = strings.TrimPrefix(relPath, wrapperPrefix+string(os.PathSeparator))
			}
		}
		if relPath == "" {
			continue
		}
		targetPath := filepath.Join(targetDir, relPath)
		if !strings.HasPrefix(targetPath, targetDir+string(os.PathSeparator)) && targetPath != targetDir {
			return fmt.Errorf("invalid archive path: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("create extracted dir: %w", err)
			}
			continue
		}
		if file.UncompressedSize64 > uint64(maxClawHubEntryBytes) {
			return fmt.Errorf("archive entry too large: %s", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("create extracted parent: %w", err)
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open archive entry %s: %w", file.Name, err)
		}
		mode := file.Mode()
		if mode == 0 {
			mode = 0644
		}
		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("create extracted file %s: %w", file.Name, err)
		}
		limited := &io.LimitedReader{R: rc, N: maxClawHubEntryBytes + 1}
		written, copyErr := io.Copy(dst, limited)
		closeErr := dst.Close()
		_ = rc.Close()
		if written > maxClawHubEntryBytes {
			return fmt.Errorf("archive entry too large: %s", file.Name)
		}
		totalWritten += written
		if totalWritten > maxClawHubExtractedBytes {
			return fmt.Errorf("archive extracted size exceeds limit")
		}
		if copyErr != nil {
			return fmt.Errorf("write extracted file %s: %w", file.Name, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close extracted file %s: %w", file.Name, closeErr)
		}
		if relPath == "SKILL.md" {
			foundSkillFile = true
		}
		if err := os.Chmod(targetPath, mode.Perm()); err != nil {
			return fmt.Errorf("write extracted file %s: %w", file.Name, err)
		}
	}
	if !foundSkillFile {
		return fmt.Errorf("archive does not contain SKILL.md")
	}
	return nil
}

func readInstalledClawHubState(workdir string) map[string]clawHubLockEntry {
	installed := map[string]clawHubLockEntry{}
	lock, err := readClawHubLock(workdir)
	lockEntries := map[string]clawHubLockEntry{}
	if err == nil && lock.Skills != nil {
		lockEntries = lock.Skills
	}
	skillsDir := filepath.Join(workdir, "skills")
	discoverable := listDiscoverableSkillKeys(skillsDir, "workspace")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return installed
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		if _, ok := discoverable[slug]; !ok {
			continue
		}
		state := readClawHubOriginEntry(filepath.Join(skillsDir, slug))
		if lockEntry, ok := lockEntries[slug]; ok {
			if state.Version == "" {
				state.Version = lockEntry.Version
			}
			if state.InstalledAt == 0 {
				state.InstalledAt = lockEntry.InstalledAt
			}
		}
		recordClawHubInstalledState(installed, slug, state)
	}
	return installed
}

func recordClawHubInstalledState(installed map[string]clawHubLockEntry, key string, entry clawHubLockEntry) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	current, exists := installed[key]
	if !exists {
		installed[key] = entry
		return
	}
	if entry.Version != "" {
		current.Version = entry.Version
	}
	if current.InstalledAt == 0 && entry.InstalledAt != 0 {
		current.InstalledAt = entry.InstalledAt
	}
	installed[key] = current
}

func readClawHubLock(workdir string) (*clawHubLockFile, error) {
	raw, err := os.ReadFile(filepath.Join(workdir, ".clawhub", "lock.json"))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Version int                       `json:"version"`
		Skills  map[string]map[string]any `json:"skills"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	lock := &clawHubLockFile{Version: payload.Version, Skills: map[string]clawHubLockEntry{}}
	if lock.Version == 0 {
		lock.Version = 1
	}
	for slug, entry := range payload.Skills {
		lockEntry := clawHubLockEntry{}
		if version := strings.TrimSpace(fmt.Sprint(entry["version"])); version != "" && version != "<nil>" {
			lockEntry.Version = version
		}
		switch v := entry["installedAt"].(type) {
		case float64:
			lockEntry.InstalledAt = int64(v)
		case int64:
			lockEntry.InstalledAt = v
		case int:
			lockEntry.InstalledAt = int64(v)
		case string:
			if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
				lockEntry.InstalledAt = parsed
			}
		}
		lock.Skills[slug] = lockEntry
	}
	return lock, nil
}

func updateClawHubLock(workdir, slug, version string, installedAt int64) error {
	lock, err := readClawHubLock(workdir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		lock = &clawHubLockFile{Version: 1, Skills: map[string]clawHubLockEntry{}}
	}
	if lock.Skills == nil {
		lock.Skills = map[string]clawHubLockEntry{}
	}
	lock.Skills[slug] = clawHubLockEntry{Version: version, InstalledAt: installedAt}
	if err := os.MkdirAll(filepath.Join(workdir, ".clawhub"), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workdir, ".clawhub", "lock.json"), raw, 0644)
}

func removeClawHubLockEntry(workdir, slug string) {
	lock, err := readClawHubLock(workdir)
	if err != nil || lock == nil || lock.Skills == nil {
		return
	}
	if _, ok := lock.Skills[slug]; !ok {
		return
	}
	delete(lock.Skills, slug)
	raw, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(workdir, ".clawhub", "lock.json"), raw, 0644)
}

func writeClawHubOrigin(skillDir, registryBase, slug, version string, installedAt int64) error {
	originDir := filepath.Join(skillDir, ".clawhub")
	if err := os.MkdirAll(originDir, 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(clawHubOriginFile{
		Registry:         registryBase,
		Slug:             slug,
		InstalledVersion: version,
		InstalledAt:      installedAt,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(originDir, "origin.json"), raw, 0644)
}

func alignPathOwnershipToParent(targetPath string) error {
	uid, gid, ok, err := parentOwnership(filepath.Dir(targetPath))
	if err != nil || !ok {
		return err
	}
	return filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}

func readClawHubOriginEntry(skillDir string) clawHubLockEntry {
	raw, err := os.ReadFile(filepath.Join(skillDir, ".clawhub", "origin.json"))
	if err != nil {
		return clawHubLockEntry{}
	}
	var payload clawHubOriginFile
	if err := json.Unmarshal(raw, &payload); err != nil {
		return clawHubLockEntry{}
	}
	return clawHubLockEntry{
		Version:     strings.TrimSpace(payload.InstalledVersion),
		InstalledAt: payload.InstalledAt,
	}
}

func resolveClawHubRegistryConfig() (clawHubRegistryConfig, error) {
	rawRegistry := strings.TrimRight(strings.TrimSpace(os.Getenv("CLAWHUB_REGISTRY")), "/")
	rawSite := strings.TrimRight(strings.TrimSpace(os.Getenv("CLAWHUB_SITE")), "/")
	fallbackRaw := strings.TrimRight(strings.TrimSpace(os.Getenv("CLAWHUB_REGISTRY_FALLBACK")), "/")

	requestRaw := firstNonEmpty(rawRegistry, rawSite, defaultClawHubRegistryBase)
	requestSource := "default ClawHub registry"
	switch {
	case rawRegistry != "":
		requestSource = "CLAWHUB_REGISTRY"
	case rawSite != "":
		requestSource = "CLAWHUB_SITE"
	}
	requestURL, err := parseClawHubBaseURL(requestRaw, requestSource)
	if err != nil {
		return clawHubRegistryConfig{}, err
	}
	publicRaw := rawSite
	publicSource := "CLAWHUB_SITE"
	if publicRaw == "" {
		publicRaw = defaultClawHubPublicBase
		publicSource = requestSource
	}
	publicURL, err := parseClawHubBaseURL(publicRaw, publicSource)
	if err != nil {
		return clawHubRegistryConfig{}, err
	}
	publicURL.User = nil
	return clawHubRegistryConfig{
		RequestBase:         requestURL.String(),
		FallbackRequestBase: firstNonEmpty(fallbackRaw, defaultClawHubRegistryFallback),
		PublicBase:          firstNonEmpty(publicURL.String(), defaultClawHubPublicBase),
	}, nil
}

func parseClawHubBaseURL(raw, source string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", source, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("%s must be an absolute http(s) URL", source)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%s must use http or https", source)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

func sanitizeClawHubSlug(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." {
		return ""
	}
	if strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return ""
	}
	if filepath.Base(value) != value || filepath.Clean(value) != value {
		return ""
	}
	if !clawHubSlugRegexp.MatchString(value) {
		return ""
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func ensureClawHubInstallTarget(workdir, skillDir string) error {
	if err := ensureClawHubStatePath(workdir); err != nil {
		return err
	}
	for _, path := range []struct {
		value      string
		finalIsDir bool
	}{
		{value: workdir, finalIsDir: true},
		{value: filepath.Join(workdir, "skills"), finalIsDir: true},
		{value: skillDir, finalIsDir: true},
	} {
		if err := ensureExistingPathChainSafe(path.value, path.finalIsDir); err != nil {
			return err
		}
	}
	realWorkdir, err := resolveExistingRealPath(workdir)
	if err != nil {
		return fmt.Errorf("resolve workspace path: %w", err)
	}
	for _, path := range []string{filepath.Join(workdir, "skills"), skillDir} {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to install into symlinked path %s", path)
		}
	}
	for _, path := range []string{filepath.Join(workdir, "skills"), skillDir} {
		realPath, err := resolveExistingRealPath(path)
		if err != nil {
			return fmt.Errorf("resolve install path: %w", err)
		}
		if !pathWithinBase(realWorkdir, realPath) {
			return fmt.Errorf("install path escapes workspace")
		}
	}
	return nil
}

func ensureClawHubStatePath(workdir string) error {
	if err := ensureExistingPathChainSafe(workdir, true); err != nil {
		return err
	}
	realWorkdir, err := resolveExistingRealPath(workdir)
	if err != nil {
		return fmt.Errorf("resolve workspace path: %w", err)
	}
	for _, path := range []struct {
		value      string
		finalIsDir bool
	}{
		{value: filepath.Join(workdir, "skills"), finalIsDir: true},
		{value: filepath.Join(workdir, ".clawhub"), finalIsDir: true},
		{value: filepath.Join(workdir, ".clawhub", "lock.json"), finalIsDir: false},
	} {
		if err := ensureExistingPathChainSafe(path.value, path.finalIsDir); err != nil {
			return err
		}
	}
	for _, path := range []string{filepath.Join(workdir, "skills"), filepath.Join(workdir, ".clawhub"), filepath.Join(workdir, ".clawhub", "lock.json")} {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to use symlinked ClawHub state path %s", path)
		}
		realPath, err := resolveExistingRealPath(path)
		if err != nil {
			return fmt.Errorf("resolve ClawHub state path: %w", err)
		}
		if !pathWithinBase(realWorkdir, realPath) {
			return fmt.Errorf("ClawHub state path escapes workspace")
		}
	}
	return nil
}

func ensureExistingPathChainSafe(path string, finalIsDir bool) error {
	absPath := normalizeTrustedSystemAliasPath(path)
	volume := filepath.VolumeName(absPath)
	current := volume + string(os.PathSeparator)
	trimmed := absPath
	if volume != "" {
		trimmed = strings.TrimPrefix(trimmed, volume)
	}
	trimmed = strings.TrimPrefix(trimmed, string(os.PathSeparator))
	if info, err := os.Lstat(current); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to use symlinked path %s", current)
	}
	segments := strings.Split(trimmed, string(os.PathSeparator))
	for index, segment := range segments {
		if segment == "" {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to use symlinked path %s", current)
		}
		if index < len(segments)-1 && !info.IsDir() {
			return fmt.Errorf("path component is not a directory: %s", current)
		}
		if index == len(segments)-1 && finalIsDir && !info.IsDir() {
			return fmt.Errorf("path component is not a directory: %s", current)
		}
	}
	return nil
}

func normalizeTrustedSystemAliasPath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	for _, alias := range []string{"/var", "/tmp"} {
		if absPath != alias && !strings.HasPrefix(absPath, alias+string(os.PathSeparator)) {
			continue
		}
		info, err := os.Lstat(alias)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := filepath.EvalSymlinks(alias)
		if err != nil || strings.TrimSpace(target) == "" {
			continue
		}
		suffix := strings.TrimPrefix(absPath, alias)
		if suffix == "" {
			return filepath.Clean(target)
		}
		return filepath.Clean(target + suffix)
	}
	return absPath
}

func resolveExistingRealPath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "" {
		return "", fmt.Errorf("empty path")
	}
	trailing := make([]string, 0)
	current := cleaned
	for {
		if _, err := os.Lstat(current); err == nil {
			real, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(trailing) - 1; i >= 0; i-- {
				real = filepath.Join(real, trailing[i])
			}
			return filepath.Clean(real), nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing parent for %s", path)
		}
		trailing = append(trailing, filepath.Base(current))
		current = parent
	}
}

func pathWithinBase(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func detectClawHubArchivePrefix(files []*zip.File) string {
	prefix := ""
	for _, file := range files {
		relPath := normalizeClawHubArchivePath(file.Name)
		if relPath == "" || strings.HasPrefix(relPath, "__MACOSX"+string(os.PathSeparator)) {
			continue
		}
		first, rest, found := strings.Cut(relPath, string(os.PathSeparator))
		if !found {
			return ""
		}
		if rest == "" {
			return ""
		}
		if prefix == "" {
			prefix = first
			continue
		}
		if prefix != first {
			return ""
		}
	}
	return prefix
}

func normalizeClawHubArchivePath(name string) string {
	relPath := filepath.Clean(strings.ReplaceAll(name, "\\", "/"))
	relPath = strings.TrimPrefix(relPath, "/")
	for strings.HasPrefix(relPath, "../") {
		relPath = strings.TrimPrefix(relPath, "../")
	}
	return relPath
}
