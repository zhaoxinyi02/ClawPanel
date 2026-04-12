package handler

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

// --- S1: Types + Cache ---

type skillHubCatalog struct {
	Total       int                 `json:"total"`
	GeneratedAt string              `json:"generated_at"`
	Featured    []string            `json:"featured"`
	Categories  map[string][]string `json:"categories"`
	Skills      []skillHubSkillItem `json:"skills"`
}

type skillHubSkillItem struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	DescriptionZh string   `json:"description_zh"`
	Version       string   `json:"version"`
	Homepage      string   `json:"homepage"`
	Tags          []string `json:"tags"`
	Downloads     int      `json:"downloads"`
	Stars         int      `json:"stars"`
	Installs      int      `json:"installs"`
	UpdatedAt     int64    `json:"updated_at"`
	Score         float64  `json:"score"`
	Owner         string   `json:"owner"`
}

// trimmed item for API response (keep homepage so UI can link to a real detail page)
type skillHubSkillTrimmed struct {
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	DescriptionZh  string   `json:"description_zh"`
	Version        string   `json:"version"`
	Homepage       string   `json:"homepage,omitempty"`
	Installed      bool     `json:"installed,omitempty"`
	InstallState   string   `json:"installState,omitempty"`
	InstallMessage string   `json:"installMessage,omitempty"`
	LastInstallAt  int64    `json:"lastInstallAt,omitempty"`
	Tags           []string `json:"tags"`
	Downloads      int      `json:"downloads"`
	Stars          int      `json:"stars"`
	UpdatedAt      int64    `json:"updated_at"`
	Score          float64  `json:"score"`
	Owner          string   `json:"owner"`
}

type skillHubInstallRecord struct {
	State     string `json:"state"`
	Message   string `json:"message,omitempty"`
	UpdatedAt int64  `json:"updatedAt"`
}

var (
	skillHubCache           *skillHubCatalog
	skillHubCacheTime       time.Time
	skillHubCacheMu         sync.Mutex
	skillHubLastGoodURL     string
	skillHubNextRetryTime   time.Time
	skillHubLastErr         string
	skillHubRefreshInFlight bool
	skillHubRefreshDone     chan struct{}
)

const (
	skillHubCacheTTL           = 1 * time.Hour
	skillHubBootstrapURL       = "https://cloudcache.tencentcs.com/qcloud/tea/app/data/skills.805f4f80.json"
	skillHubHomepage           = "https://skillhub.tencent.com/"
	skillHubMaxBodyBytes       = 16 << 20 // 16MB
	skillHubFetchTimeout       = 25 * time.Second
	skillHubRetryBackoff       = 5 * time.Minute
	skillHubCDNBase            = "https://cloudcache.tencentcs.com/qcloud/tea/app/data/"
	skillHubDefaultInstallKit  = "https://skillhub-1388575217.cos.ap-guangzhou.myqcloud.com/install/latest.tar.gz"
	skillHubInstallGuideURL    = "https://skillhub-1388575217.cos.ap-guangzhou.myqcloud.com/install/skillhub.md"
	skillHubInstallTimeout     = 5 * time.Minute
	skillHubInstallMaxBytes    = 32 << 20 // 32MB
	skillHubCommandOutputLimit = 4096
)

var skillHubInstallShellURL = "https://skillhub-1388575217.cos.ap-guangzhou.myqcloud.com/install/install.sh"

var skillHubJSONHashRe = regexp.MustCompile(`skills\.([0-9a-f]+)\.json`)

var skillHubHTTPClient = &http.Client{Timeout: skillHubFetchTimeout}
var skillHubInstallHTTPClient = &http.Client{Timeout: skillHubInstallTimeout}
var skillHubInstallKitURL = skillHubDefaultInstallKit
var skillHubBinaryCandidatePaths = []string{"/usr/local/bin/skillhub", "/opt/homebrew/bin/skillhub"}

// --- S2: URL Discovery + Handler ---

// discoverSkillHubJSONURL fetches the SkillHub homepage and extracts the
// current JSON data URL from embedded script/asset references.
func discoverSkillHubJSONURL() (string, error) {
	resp, err := skillHubHTTPClient.Get(skillHubHomepage)
	if err != nil {
		return "", fmt.Errorf("fetch skillhub homepage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("skillhub homepage returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // 512KB max for HTML
	if err != nil {
		return "", fmt.Errorf("read skillhub homepage: %w", err)
	}
	matches := skillHubJSONHashRe.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("no skills JSON hash found in homepage")
	}
	// use the last match (usually in main JS bundle near bottom)
	filename := matches[len(matches)-1][0]
	return skillHubCDNBase + filename, nil
}

// resolveSkillHubJSONURLs returns candidate URLs in priority order without
// mutating the last-good state. lastGoodURL is only updated after a successful
// JSON fetch and decode.
func resolveSkillHubJSONURLs(lastGoodURL string) []string {
	urls := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	appendURL := func(url string) {
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		urls = append(urls, url)
	}
	url, err := discoverSkillHubJSONURL()
	if err == nil && url != "" {
		appendURL(url)
	}
	appendURL(lastGoodURL)
	appendURL(skillHubBootstrapURL)
	return urls
}

func fetchSkillHubCatalog(url string) (*skillHubCatalog, error) {
	resp, err := skillHubHTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch skillhub JSON: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skillhub JSON returned %d", resp.StatusCode)
	}

	reader := io.LimitReader(resp.Body, skillHubMaxBodyBytes)
	var catalog skillHubCatalog
	dec := json.NewDecoder(reader)
	if err := dec.Decode(&catalog); err != nil {
		return nil, fmt.Errorf("parse skillhub JSON: %w", err)
	}
	if catalog.Skills == nil {
		return nil, fmt.Errorf("skillhub JSON missing skills list")
	}
	if catalog.Featured == nil {
		catalog.Featured = []string{}
	}
	if catalog.Categories == nil {
		catalog.Categories = map[string][]string{}
	}
	return &catalog, nil
}

func refreshSkillHubCatalog(lastGoodURL string) (*skillHubCatalog, string, error) {
	var lastErr error
	for _, jsonURL := range resolveSkillHubJSONURLs(lastGoodURL) {
		catalog, err := fetchSkillHubCatalog(jsonURL)
		if err != nil {
			lastErr = err
			continue
		}
		return catalog, jsonURL, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to resolve skillhub catalog URL")
	}
	return nil, "", lastErr
}

func loadSkillHubCatalog() (*skillHubCatalog, error) {
	for {
		now := time.Now()

		skillHubCacheMu.Lock()
		if skillHubCache != nil && now.Sub(skillHubCacheTime) < skillHubCacheTTL {
			cached := skillHubCache
			skillHubCacheMu.Unlock()
			return cached, nil
		}
		if skillHubCache != nil && !skillHubNextRetryTime.IsZero() && now.Before(skillHubNextRetryTime) {
			cached := skillHubCache
			skillHubCacheMu.Unlock()
			return cached, nil
		}
		if skillHubRefreshInFlight {
			waitCh := skillHubRefreshDone
			cached := skillHubCache
			skillHubCacheMu.Unlock()

			if cached != nil {
				return cached, nil
			}
			if waitCh != nil {
				<-waitCh
			}

			skillHubCacheMu.Lock()
			cached = skillHubCache
			lastErr := skillHubLastErr
			skillHubCacheMu.Unlock()
			if cached != nil {
				return cached, nil
			}
			if lastErr != "" {
				return nil, fmt.Errorf("%s", lastErr)
			}
			continue
		}

		staleCache := skillHubCache
		lastGoodURL := skillHubLastGoodURL
		doneCh := make(chan struct{})
		skillHubRefreshInFlight = true
		skillHubRefreshDone = doneCh
		skillHubCacheMu.Unlock()

		catalog, jsonURL, err := refreshSkillHubCatalog(lastGoodURL)

		skillHubCacheMu.Lock()
		skillHubRefreshInFlight = false
		close(doneCh)
		skillHubRefreshDone = nil

		if err == nil {
			skillHubLastGoodURL = jsonURL
			skillHubCache = catalog
			skillHubCacheTime = time.Now()
			skillHubNextRetryTime = time.Time{}
			skillHubLastErr = ""
			skillHubCacheMu.Unlock()
			return catalog, nil
		}

		skillHubLastErr = err.Error()
		if staleCache != nil {
			skillHubNextRetryTime = time.Now().Add(skillHubRetryBackoff)
			cached := skillHubCache
			if cached == nil {
				cached = staleCache
			}
			skillHubCacheMu.Unlock()
			return cached, nil
		}
		skillHubNextRetryTime = time.Time{}
		skillHubCacheMu.Unlock()
		return nil, err
	}
}

func trimSkillHubSkills(skills []skillHubSkillItem, installState map[string]skillHubInstallRecord, installedDirs map[string]struct{}) []skillHubSkillTrimmed {
	out := make([]skillHubSkillTrimmed, len(skills))
	for i, s := range skills {
		record := installState[s.Slug]
		_, installed := installedDirs[s.Slug]
		if installed {
			record.State = "installed"
			record.Message = ""
		}
		out[i] = skillHubSkillTrimmed{
			Slug:           s.Slug,
			Name:           s.Name,
			Description:    s.Description,
			DescriptionZh:  s.DescriptionZh,
			Version:        s.Version,
			Homepage:       strings.TrimSpace(s.Homepage),
			Installed:      installed,
			InstallState:   record.State,
			InstallMessage: record.Message,
			LastInstallAt:  record.UpdatedAt,
			Tags:           s.Tags,
			Downloads:      s.Downloads,
			Stars:          s.Stars,
			UpdatedAt:      s.UpdatedAt,
			Score:          s.Score,
			Owner:          s.Owner,
		}
	}
	return out
}

// GetSkillHubCatalog returns the SkillHub catalog data.
func GetSkillHubCatalog(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		catalog, err := loadSkillHubCatalog()
		if err != nil {
			errMsg := err.Error()
			// sanitize internal URLs from error message
			if strings.Contains(errMsg, "cloudcache.tencentcs.com") {
				errMsg = "failed to load SkillHub data from upstream"
			}
			c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": errMsg})
			return
		}

		agentID, err := resolveRequestedAgentID(cfg, c.Query("agentId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		workdir, installTarget, err := resolveSkillInstallBase(cfg, agentID, c.Query("installTarget"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		installState := map[string]skillHubInstallRecord{}
		installedDirs := map[string]struct{}{}
		if state, err := readSkillHubInstallState(workdir); err == nil {
			installState = state
		}
		if dirs, err := listInstalledSkillDirs(workdir); err == nil {
			installedDirs = dirs
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":            true,
			"agentId":       agentID,
			"installTarget": installTarget,
			"total":         catalog.Total,
			"generatedAt":   catalog.GeneratedAt,
			"featured":      catalog.Featured,
			"categories":    catalog.Categories,
			"skills":        trimSkillHubSkills(catalog.Skills, installState, installedDirs),
		})
	}
}

func skillHubStateFilePath(workdir string) string {
	return filepath.Join(workdir, ".skillhub", "install-state.json")
}

func ensureSkillHubStatePath(workdir string) error {
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
		{value: filepath.Join(workdir, ".skillhub"), finalIsDir: true},
		{value: skillHubStateFilePath(workdir), finalIsDir: false},
	} {
		if err := ensureExistingPathChainSafe(path.value, path.finalIsDir); err != nil {
			return err
		}
		if info, err := os.Lstat(path.value); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to use symlinked SkillHub state path %s", path.value)
		}
		realPath, err := resolveExistingRealPath(path.value)
		if err != nil {
			return fmt.Errorf("resolve SkillHub state path: %w", err)
		}
		if !pathWithinBase(realWorkdir, realPath) {
			return fmt.Errorf("SkillHub state path escapes workspace")
		}
	}
	return nil
}

func readSkillHubInstallState(workdir string) (map[string]skillHubInstallRecord, error) {
	state := make(map[string]skillHubInstallRecord)
	if workdir == "" {
		return state, nil
	}
	if err := ensureSkillHubStatePath(workdir); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(skillHubStateFilePath(workdir))
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return nil, fmt.Errorf("read SkillHub install state: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse SkillHub install state: %w", err)
	}
	return state, nil
}

func writeSkillHubInstallState(workdir string, state map[string]skillHubInstallRecord) error {
	if err := ensureSkillHubStatePath(workdir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(workdir, ".skillhub"), 0o755); err != nil {
		return fmt.Errorf("create SkillHub state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal SkillHub install state: %w", err)
	}
	if err := os.WriteFile(skillHubStateFilePath(workdir), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write SkillHub install state: %w", err)
	}
	return nil
}

func recordSkillHubInstallState(workdir, slug, state, message string) error {
	records, err := readSkillHubInstallState(workdir)
	if err != nil {
		return err
	}
	if state == "installed" {
		message = ""
	}
	records[slug] = skillHubInstallRecord{
		State:     state,
		Message:   strings.TrimSpace(message),
		UpdatedAt: time.Now().UnixMilli(),
	}
	return writeSkillHubInstallState(workdir, records)
}

func listInstalledSkillDirs(workdir string) (map[string]struct{}, error) {
	installed := make(map[string]struct{})
	if workdir == "" {
		return installed, nil
	}
	if err := ensureClawHubStatePath(workdir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(workdir, "skills"))
	if err != nil {
		if os.IsNotExist(err) {
			return installed, nil
		}
		return nil, fmt.Errorf("read workspace skills dir: %w", err)
	}
	discoverable := listDiscoverableSkillKeys(filepath.Join(workdir, "skills"), "workspace")
	for _, entry := range entries {
		if entry.IsDir() {
			if _, ok := discoverable[entry.Name()]; !ok {
				continue
			}
			installed[entry.Name()] = struct{}{}
		}
	}
	return installed, nil
}

// GetSkillHubStatus reports whether the official SkillHub CLI is available locally.
func GetSkillHubStatus(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		binPath, err := resolveSkillHubBinary()
		resp := gin.H{
			"ok":                  true,
			"installed":           err == nil,
			"installGuideURL":     skillHubInstallGuideURL,
			"skillInstallCommand": "skillhub install <slug>",
		}
		if err == nil {
			resp["binPath"] = binPath
		} else {
			resp["error"] = err.Error()
			if pythonErr := detectSkillHubPythonDependency(); pythonErr != nil {
				resp["missingPython"] = true
				resp["installHint"] = pythonErr.Error()
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}

// InstallSkillHubCLI installs the official SkillHub CLI from Tencent's published kit.
func InstallSkillHubCLI(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if binPath, err := resolveSkillHubBinary(); err == nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "installed": true, "binPath": binPath})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), skillHubInstallTimeout)
		defer cancel()

		binPath, output, err := installSkillHubCLI(ctx)
		if err != nil {
			resp := gin.H{"ok": false, "error": err.Error(), "output": output}
			if pythonErr := detectSkillHubPythonDependency(); pythonErr != nil {
				resp["missingPython"] = true
				resp["installHint"] = pythonErr.Error()
			}
			c.JSON(http.StatusBadGateway, resp)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "installed": true, "binPath": binPath, "output": output})
	}
}

func detectSkillHubPythonDependency() error {
	_, _, err := detectSkillHubPython()
	if err != nil {
		return err
	}
	return nil
}

// InstallSkillHubSkill runs the official `skillhub install <slug>` command inside the selected workspace.
func InstallSkillHubSkill(cfg *config.Config) gin.HandlerFunc {
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
		if err := ensureClawHubInstallTarget(workdir, skillDir); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
			return
		}
		if info, err := os.Stat(skillDir); err == nil {
			if info.IsDir() {
				resp := gin.H{"ok": false, "error": fmt.Sprintf("skill %s is already installed", slug), "installState": "installed", "installTarget": installTarget}
				if stateErr := recordSkillHubInstallState(workdir, slug, "installed", ""); stateErr != nil {
					resp["warning"] = stateErr.Error()
				}
				c.JSON(http.StatusConflict, resp)
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("install target exists and is not a directory: %s", skillDir)})
			return
		} else if !os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": fmt.Sprintf("check install target: %v", err)})
			return
		}
		if err := os.MkdirAll(filepath.Join(workdir, "skills"), 0o755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": fmt.Sprintf("create workspace directories: %v", err)})
			return
		}
		binPath, err := resolveSkillHubBinary()
		if err != nil {
			c.JSON(http.StatusPreconditionFailed, gin.H{"ok": false, "error": err.Error(), "needsCLI": true})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), skillHubInstallTimeout)
		defer cancel()

		output, err := runCommandCaptureWithEnv(ctx, workdir, skillHubInstallCommandEnv(), binPath, "install", slug)
		if err != nil {
			wrappedErr := wrapSkillHubCommandError("skillhub install", err, output)
			resp := gin.H{"ok": false, "error": wrappedErr.Error(), "output": output, "installState": "failed"}
			if stateErr := recordSkillHubInstallState(workdir, slug, "failed", wrappedErr.Error()); stateErr != nil {
				resp["warning"] = stateErr.Error()
			}
			c.JSON(http.StatusBadGateway, resp)
			return
		}
		resp := gin.H{"ok": true, "agentId": agentID, "installTarget": installTarget, "skillId": slug, "output": output, "installState": "installed"}
		if stateErr := recordSkillHubInstallState(workdir, slug, "installed", ""); stateErr != nil {
			resp["warning"] = stateErr.Error()
		}
		c.JSON(http.StatusOK, resp)
	}
}

func resolveSkillHubBinary() (string, error) {
	seen := make(map[string]struct{}, 8)
	candidates := make([]string, 0, 8)
	if envPath := strings.TrimSpace(os.Getenv("SKILLHUB_BIN")); envPath != "" {
		candidates = append(candidates, envPath)
	}
	candidates = append(candidates, "skillhub")
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		candidates = append(candidates,
			filepath.Join(home, ".local", "bin", "skillhub"),
			filepath.Join(home, "bin", "skillhub"),
		)
	}
	candidates = append(candidates, skillHubBinaryCandidatePaths...)

	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("SkillHub CLI not found; install SkillHub CLI first")
}

func installSkillHubCLI(ctx context.Context) (string, string, error) {
	// Unix 系统优先使用官方 install.sh 脚本
	if runtime.GOOS != "windows" {
		binPath, output, err := installSkillHubCLIViaShell(ctx)
		if err == nil {
			return binPath, output, nil
		}
		shellErr := err

		// install.sh 失败时回退到 tar.gz 安装包方式
		binPath, output, err = installSkillHubCLIViaKit(ctx)
		if err == nil {
			return binPath, output, nil
		}
		return "", "", fmt.Errorf("install via shell: %v; install via kit: %v", shellErr, err)
	}

	// Windows 只支持 tar.gz 安装包方式
	return installSkillHubCLIViaKit(ctx)
}

// installSkillHubCLIViaShell 通过官方 install.sh 脚本安装（Unix only）
func installSkillHubCLIViaShell(ctx context.Context) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, skillHubInstallShellURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create install.sh request: %w", err)
	}
	resp, err := skillHubInstallHTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("download install.sh: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("install.sh returned %d", resp.StatusCode)
	}

	scriptBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", "", fmt.Errorf("read install.sh: %w", err)
	}

	tempFile, err := os.CreateTemp("", "skillhub-install-*.sh")
	if err != nil {
		return "", "", fmt.Errorf("create temp install script: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(scriptBytes); err != nil {
		tempFile.Close()
		return "", "", fmt.Errorf("write install script: %w", err)
	}
	tempFile.Close()

	if err := os.Chmod(tempPath, 0o755); err != nil {
		return "", "", fmt.Errorf("chmod install script: %w", err)
	}

	output, err := runCommandCapture(ctx, "", "bash", tempPath, "--no-skills")
	if err != nil {
		return "", "", wrapSkillHubCommandError("install.sh", err, output)
	}

	binPath, resolveErr := resolveSkillHubBinary()
	if resolveErr != nil {
		return "", output, resolveErr
	}
	return binPath, output, nil
}

// installSkillHubCLIViaKit 通过 tar.gz 安装包方式安装
func installSkillHubCLIViaKit(ctx context.Context) (string, string, error) {
	tempDir, err := os.MkdirTemp("", "clawpanel-skillhub-install-*")
	if err != nil {
		return "", "", fmt.Errorf("create temporary installer workspace: %w", err)
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "skillhub-latest.tar.gz")
	if err := downloadSkillHubInstallKit(ctx, archivePath); err != nil {
		return "", "", err
	}
	installerPath, err := extractSkillHubInstallKit(tempDir, archivePath)
	if err != nil {
		return "", "", err
	}
	output, err := installSkillHubCLIFromKit(filepath.Dir(installerPath))
	if err != nil {
		return "", "", wrapSkillHubCommandError("install SkillHub CLI", err, "")
	}
	binPath, err := resolveSkillHubBinary()
	if err != nil {
		return "", output, err
	}
	return binPath, output, nil
}

func installSkillHubCLIFromKit(cliSourceDir string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	pythonCmd, pythonArgs, err := detectSkillHubPython()
	if err != nil {
		return "", err
	}

	installBase := filepath.Join(homeDir, ".skillhub")
	binDir := filepath.Join(homeDir, ".local", "bin")
	cliTarget := filepath.Join(installBase, "skills_store_cli.py")
	upgradeTarget := filepath.Join(installBase, "skills_upgrade.py")
	versionTarget := filepath.Join(installBase, "version.json")
	metadataTarget := filepath.Join(installBase, "metadata.json")
	indexTarget := filepath.Join(installBase, "skills_index.local.json")
	configTarget := filepath.Join(installBase, "config.json")

	if err := os.MkdirAll(installBase, 0o755); err != nil {
		return "", fmt.Errorf("create SkillHub home: %w", err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("create SkillHub bin dir: %w", err)
	}

	copyTargets := map[string]string{
		filepath.Join(cliSourceDir, "skills_store_cli.py"):     cliTarget,
		filepath.Join(cliSourceDir, "skills_upgrade.py"):       upgradeTarget,
		filepath.Join(cliSourceDir, "version.json"):            versionTarget,
		filepath.Join(cliSourceDir, "metadata.json"):           metadataTarget,
		filepath.Join(cliSourceDir, "skills_index.local.json"): indexTarget,
	}
	for src, dst := range copyTargets {
		if err := copySkillHubInstallFile(src, dst); err != nil {
			return "", err
		}
	}
	if err := os.Chmod(cliTarget, 0o755); err != nil {
		return "", fmt.Errorf("mark SkillHub CLI executable: %w", err)
	}
	if _, err := os.Stat(configTarget); os.IsNotExist(err) {
		configData := []byte("{\n  \"self_update_url\": \"https://skillhub-1388575217.cos.ap-guangzhou.myqcloud.com/version.json\"\n}\n")
		if err := os.WriteFile(configTarget, configData, 0o644); err != nil {
			return "", fmt.Errorf("create SkillHub config: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("check SkillHub config: %w", err)
	}

	wrapperPath, legacyWrapperPath, err := writeSkillHubWrappers(binDir, cliTarget, pythonCmd, pythonArgs)
	if err != nil {
		return "", err
	}

	output := strings.Join([]string{
		"Install complete.",
		"installed cli",
		"  mode: cli",
		"  cli: " + wrapperPath,
		"  index: " + indexTarget,
		"",
		"Quick check:",
		"  skillhub search calendar",
		"  legacy: " + legacyWrapperPath,
	}, "\n")
	return output, nil
}

func detectSkillHubPython() (string, []string, error) {
	candidates := [][]string{
		{"python3"},
		{"python"},
	}
	if runtime.GOOS == "windows" {
		candidates = append([][]string{{"py", "-3"}}, candidates...)
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate[0]); err == nil {
			return candidate[0], candidate[1:], nil
		}
	}
	if runtime.GOOS == "windows" {
		return "", nil, fmt.Errorf("未检测到 Python 运行时；SkillHub CLI 依赖 Python。请先安装 Python 3 或 py launcher 后再试")
	}
	return "", nil, fmt.Errorf("未检测到 Python 3 运行时；SkillHub CLI 依赖 Python。请先安装 python3 后再试")
}

func copySkillHubInstallFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open SkillHub kit file %s: %w", filepath.Base(srcPath), err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create SkillHub target file %s: %w", filepath.Base(dstPath), err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy SkillHub file %s: %w", filepath.Base(dstPath), err)
	}
	return nil
}

func writeSkillHubWrappers(binDir, cliTarget, pythonCmd string, pythonArgs []string) (string, string, error) {
	if runtime.GOOS == "windows" {
		return writeSkillHubWindowsWrappers(binDir, cliTarget, pythonCmd, pythonArgs)
	}
	return writeSkillHubUnixWrappers(binDir, cliTarget, pythonCmd, pythonArgs)
}

func writeSkillHubUnixWrappers(binDir, cliTarget, pythonCmd string, pythonArgs []string) (string, string, error) {
	wrapperPath := filepath.Join(binDir, "skillhub")
	legacyPath := filepath.Join(binDir, "oc-skills")
	pythonInvocation := strings.Join(append([]string{shellQuote(pythonCmd)}, shellQuoteSlice(pythonArgs)...), " ")
	wrapperContent := strings.Join([]string{
		"#!/bin/sh",
		"set -eu",
		"",
		"CLI=" + shellQuote(cliTarget),
		"if [ ! -f \"$CLI\" ]; then",
		"  echo \"Error: CLI not found at $CLI\" >&2",
		"  exit 1",
		"fi",
		"",
		"exec " + pythonInvocation + " \"$CLI\" \"$@\"",
		"",
	}, "\n")
	legacyContent := strings.Join([]string{
		"#!/bin/sh",
		"set -eu",
		"exec " + shellQuote(wrapperPath) + " \"$@\"",
		"",
	}, "\n")
	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0o755); err != nil {
		return "", "", fmt.Errorf("write SkillHub wrapper: %w", err)
	}
	if err := os.WriteFile(legacyPath, []byte(legacyContent), 0o755); err != nil {
		return "", "", fmt.Errorf("write SkillHub legacy wrapper: %w", err)
	}
	return wrapperPath, legacyPath, nil
}

func writeSkillHubWindowsWrappers(binDir, cliTarget, pythonCmd string, pythonArgs []string) (string, string, error) {
	wrapperPath := filepath.Join(binDir, "skillhub.cmd")
	legacyPath := filepath.Join(binDir, "oc-skills.cmd")
	pythonInvocation := strings.Join(append([]string{cmdQuote(pythonCmd)}, cmdQuoteSlice(pythonArgs)...), " ")
	cliValue := cmdSetValue(cliTarget)
	wrapperContent := strings.Join([]string{
		"@echo off",
		"setlocal",
		"set \"CLI=" + cliValue + "\"",
		"if not exist \"%CLI%\" (",
		"  >&2 echo Error: CLI not found at %CLI%",
		"  exit /b 1",
		")",
		pythonInvocation + " \"%CLI%\" %*",
		"",
	}, "\r\n")
	legacyContent := strings.Join([]string{
		"@echo off",
		"setlocal",
		"call \"" + cmdSetValue(wrapperPath) + "\" %*",
		"",
	}, "\r\n")
	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0o644); err != nil {
		return "", "", fmt.Errorf("write SkillHub wrapper: %w", err)
	}
	if err := os.WriteFile(legacyPath, []byte(legacyContent), 0o644); err != nil {
		return "", "", fmt.Errorf("write SkillHub legacy wrapper: %w", err)
	}
	return wrapperPath, legacyPath, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func shellQuoteSlice(values []string) []string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, shellQuote(value))
	}
	return quoted
}

func cmdQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func cmdQuoteSlice(values []string) []string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, cmdQuote(value))
	}
	return quoted
}

func cmdSetValue(value string) string {
	return strings.ReplaceAll(value, `"`, `""`)
}

func downloadSkillHubInstallKit(ctx context.Context, targetPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, skillHubInstallKitURL, nil)
	if err != nil {
		return fmt.Errorf("create SkillHub installer request: %w", err)
	}
	resp, err := skillHubInstallHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download SkillHub installer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SkillHub installer returned %d", resp.StatusCode)
	}
	file, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create installer archive: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, io.LimitReader(resp.Body, skillHubInstallMaxBytes+1))
	if err != nil {
		return fmt.Errorf("write installer archive: %w", err)
	}
	if written > skillHubInstallMaxBytes {
		return fmt.Errorf("SkillHub installer archive is too large")
	}
	return nil
}

func extractSkillHubInstallKit(rootDir, archivePath string) (string, error) {
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open installer archive: %w", err)
	}
	defer archiveFile.Close()

	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return "", fmt.Errorf("read installer gzip: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	rootPrefix := rootDir + string(os.PathSeparator)
	var installerPath string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read installer archive: %w", err)
		}

		relPath := filepath.Clean(header.Name)
		if relPath == "." || relPath == "" || relPath == string(os.PathSeparator) {
			continue
		}
		if strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
			return "", fmt.Errorf("installer archive contains invalid path %q", header.Name)
		}

		targetPath := filepath.Join(rootDir, relPath)
		if targetPath != rootDir && !strings.HasPrefix(targetPath, rootPrefix) {
			return "", fmt.Errorf("installer archive escapes destination")
		}

		switch header.Typeflag {
		case tar.TypeDir:
			mode := os.FileMode(header.Mode)
			if mode == 0 {
				mode = 0o755
			}
			if err := os.MkdirAll(targetPath, mode); err != nil {
				return "", fmt.Errorf("create installer directory: %w", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return "", fmt.Errorf("create installer file parent: %w", err)
			}
			mode := os.FileMode(header.Mode)
			if mode == 0 {
				mode = 0o644
			}
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return "", fmt.Errorf("create installer file: %w", err)
			}
			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return "", fmt.Errorf("extract installer file: %w", err)
			}
			if err := file.Close(); err != nil {
				return "", fmt.Errorf("close installer file: %w", err)
			}
			unixPath := filepath.ToSlash(relPath)
			if unixPath == "cli/install.sh" || strings.HasSuffix(unixPath, "/cli/install.sh") {
				installerPath = targetPath
			}
		case tar.TypeSymlink, tar.TypeLink:
			return "", fmt.Errorf("installer archive contains unsupported linked file %q", header.Name)
		default:
			return "", fmt.Errorf("installer archive contains unsupported entry %q", header.Name)
		}
	}

	if installerPath == "" {
		return "", fmt.Errorf("SkillHub installer archive missing cli/install.sh")
	}
	return installerPath, nil
}

func runCommandCapture(ctx context.Context, dir, name string, args ...string) (string, error) {
	return runCommandCaptureWithEnv(ctx, dir, nil, name, args...)
}

func runCommandCaptureWithEnv(ctx context.Context, dir string, extraEnv []string, name string, args ...string) (string, error) {
	cmdName := name
	cmdArgs := args
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".cmd" || ext == ".bat" {
			cmdName = "cmd.exe"
			cmdArgs = append([]string{"/c", name}, args...)
		}
	}
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	output, err := cmd.CombinedOutput()
	return trimSkillHubCommandOutput(output), err
}

func skillHubInstallCommandEnv() []string {
	return []string{
		"PYTHONHTTPSVERIFY=0",
		"SSL_NO_VERIFY=1",
	}
}

func trimSkillHubCommandOutput(output []byte) string {
	trimmed := strings.TrimSpace(string(output))
	if len(trimmed) <= skillHubCommandOutputLimit {
		return trimmed
	}
	return trimmed[:skillHubCommandOutputLimit] + "..."
}

func wrapSkillHubCommandError(action string, err error, output string) error {
	output = strings.TrimSpace(output)
	if output == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, output)
}
