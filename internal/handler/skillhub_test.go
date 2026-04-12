package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestLoadSkillHubCatalogDoesNotPoisonLastGoodURLWhenDiscoveredURLFails(t *testing.T) {
	restore := resetSkillHubTestState(t)
	defer restore()

	badDynamicURL := skillHubCDNBase + "skills.deadbeef.json"
	skillHubHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case skillHubHomepage:
			return newHTTPResponse(http.StatusOK, `<script src="/assets/main.js"></script>skills.deadbeef.json`), nil
		case badDynamicURL:
			return newHTTPResponse(http.StatusNotFound, `not found`), nil
		case skillHubBootstrapURL:
			return newHTTPResponse(http.StatusOK, validSkillHubCatalogJSON("bootstrap-skill")), nil
		default:
			return nil, errors.New("unexpected url: " + req.URL.String())
		}
	})}

	catalog, err := loadSkillHubCatalog()
	if err != nil {
		t.Fatalf("loadSkillHubCatalog error: %v", err)
	}
	if catalog == nil || len(catalog.Skills) != 1 || catalog.Skills[0].Slug != "bootstrap-skill" {
		t.Fatalf("expected bootstrap catalog, got %#v", catalog)
	}
	if skillHubLastGoodURL != skillHubBootstrapURL {
		t.Fatalf("expected lastGoodURL %q, got %q", skillHubBootstrapURL, skillHubLastGoodURL)
	}
}

func TestLoadSkillHubCatalogReturnsStaleCacheWhenRefreshFails(t *testing.T) {
	restore := resetSkillHubTestState(t)
	defer restore()

	stale := &skillHubCatalog{
		Total:       1,
		GeneratedAt: "2026-03-01T13:44:23Z",
		Featured:    []string{"cached-skill"},
		Categories:  map[string][]string{"AI 智能": {"ai"}},
		Skills:      []skillHubSkillItem{{Slug: "cached-skill", Name: "Cached Skill"}},
	}
	skillHubCache = stale
	skillHubCacheTime = time.Now().Add(-2 * skillHubCacheTTL)
	skillHubLastGoodURL = skillHubCDNBase + "skills.cached.json"

	skillHubHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case skillHubHomepage:
			return nil, errors.New("homepage unavailable")
		case skillHubLastGoodURL, skillHubBootstrapURL:
			return nil, errors.New("upstream unavailable")
		default:
			return nil, errors.New("unexpected url: " + req.URL.String())
		}
	})}

	got, err := loadSkillHubCatalog()
	if err != nil {
		t.Fatalf("expected stale cache fallback, got error: %v", err)
	}
	if got != stale {
		t.Fatalf("expected stale cache pointer, got %#v", got)
	}
}

func TestLoadSkillHubCatalogErrorsWithoutAnySuccessfulCatalog(t *testing.T) {
	restore := resetSkillHubTestState(t)
	defer restore()

	skillHubHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case skillHubHomepage:
			return nil, errors.New("homepage unavailable")
		case skillHubBootstrapURL:
			return nil, errors.New("bootstrap unavailable")
		default:
			return nil, errors.New("unexpected url: " + req.URL.String())
		}
	})}

	if _, err := loadSkillHubCatalog(); err == nil {
		t.Fatalf("expected error when no cache and upstream unavailable")
	}
}

func TestLoadSkillHubCatalogSkipsRefreshDuringRetryBackoff(t *testing.T) {
	restore := resetSkillHubTestState(t)
	defer restore()

	stale := &skillHubCatalog{
		Total:       1,
		GeneratedAt: "2026-03-01T13:44:23Z",
		Featured:    []string{"cached-skill"},
		Categories:  map[string][]string{"AI 智能": {"ai"}},
		Skills:      []skillHubSkillItem{{Slug: "cached-skill", Name: "Cached Skill"}},
	}
	skillHubCache = stale
	skillHubCacheTime = time.Now().Add(-2 * skillHubCacheTTL)
	skillHubNextRetryTime = time.Now().Add(2 * time.Minute)

	calls := 0
	skillHubHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("network should not be used during retry backoff")
	})}

	got, err := loadSkillHubCatalog()
	if err != nil {
		t.Fatalf("expected stale cache during retry backoff, got error: %v", err)
	}
	if got != stale {
		t.Fatalf("expected stale cache pointer, got %#v", got)
	}
	if calls != 0 {
		t.Fatalf("expected no upstream calls during retry backoff, got %d", calls)
	}
}

func TestLoadSkillHubCatalogRetriesWithoutCacheEvenIfRetryWindowIsSet(t *testing.T) {
	restore := resetSkillHubTestState(t)
	defer restore()

	skillHubNextRetryTime = time.Now().Add(2 * time.Minute)
	skillHubLastErr = "temporary failure"

	calls := 0
	skillHubHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		switch req.URL.String() {
		case skillHubHomepage:
			return nil, errors.New("homepage unavailable")
		case skillHubBootstrapURL:
			return newHTTPResponse(http.StatusOK, validSkillHubCatalogJSON("recovered-skill")), nil
		default:
			return nil, errors.New("unexpected url: " + req.URL.String())
		}
	})}

	got, err := loadSkillHubCatalog()
	if err != nil {
		t.Fatalf("expected successful retry without cache, got error: %v", err)
	}
	if got == nil || len(got.Skills) != 1 || got.Skills[0].Slug != "recovered-skill" {
		t.Fatalf("expected recovered catalog, got %#v", got)
	}
	if calls == 0 {
		t.Fatalf("expected upstream retry when no cache exists")
	}
}

func TestLoadSkillHubCatalogReturnsStaleImmediatelyWhileRefreshInFlight(t *testing.T) {
	restore := resetSkillHubTestState(t)
	defer restore()

	stale := &skillHubCatalog{
		Total:       1,
		GeneratedAt: "2026-03-01T13:44:23Z",
		Featured:    []string{"cached-skill"},
		Categories:  map[string][]string{"AI 智能": {"ai"}},
		Skills:      []skillHubSkillItem{{Slug: "cached-skill", Name: "Cached Skill"}},
	}
	skillHubCache = stale
	skillHubCacheTime = time.Now().Add(-2 * skillHubCacheTTL)

	homepageStarted := make(chan struct{})
	releaseHomepage := make(chan struct{})
	skillHubHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case skillHubHomepage:
			select {
			case <-homepageStarted:
			default:
				close(homepageStarted)
			}
			<-releaseHomepage
			return nil, errors.New("homepage unavailable")
		case skillHubBootstrapURL:
			return nil, errors.New("bootstrap unavailable")
		default:
			return nil, errors.New("unexpected url: " + req.URL.String())
		}
	})}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = loadSkillHubCatalog()
	}()

	select {
	case <-homepageStarted:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for refresh to start")
	}

	start := time.Now()
	got, err := loadSkillHubCatalog()
	if err != nil {
		t.Fatalf("expected stale cache while refresh in flight, got error: %v", err)
	}
	if got != stale {
		t.Fatalf("expected stale cache pointer, got %#v", got)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("expected stale response without waiting for refresh, took %v", elapsed)
	}

	close(releaseHomepage)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for refresh goroutine to exit")
	}
}

func TestGetSkillHubStatusReportsInstalledBinary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := resetSkillHubTestState(t)
	defer restore()

	root := t.TempDir()
	binPath := filepath.Join(root, testExecutableBase("skillhub"))
	writeFakeSkillHubCommand(t, binPath, filepath.Join(root, "skillhub-status.log"), false)
	t.Setenv("SKILLHUB_BIN", binPath)
	t.Setenv("PATH", root)
	skillHubBinaryCandidatePaths = nil

	r := gin.New()
	r.GET("/system/skillhub/status", GetSkillHubStatus(&config.Config{}))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/skillhub/status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK        bool   `json:"ok"`
		Installed bool   `json:"installed"`
		BinPath   string `json:"binPath"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || !resp.Installed || resp.BinPath != binPath {
		t.Fatalf("unexpected status response: %+v", resp)
	}
}

func TestInstallSkillHubCLIInstallsFromOfficialKit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := resetSkillHubTestState(t)
	defer restore()

	root := t.TempDir()
	home := filepath.Join(root, "home")
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	setTestHomeEnv(t, home)
	prependTestPath(t, binDir)
	skillHubBinaryCandidatePaths = nil

	archiveBytes := buildSkillHubInstallArchive(t, "bundle/cli/install.sh", `#!/bin/sh
set -eu
mkdir -p "$HOME/.local/bin"
cat > "$HOME/.local/bin/skillhub" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod +x "$HOME/.local/bin/skillhub"
printf 'installed cli'
`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest.tar.gz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archiveBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	skillHubInstallKitURL = server.URL + "/latest.tar.gz"
	// 禁用 install.sh 回退，确保测试只走 tar.gz 路径
	skillHubInstallShellURL = server.URL + "/install.sh"
	skillHubInstallHTTPClient = server.Client()

	r := gin.New()
	r.POST("/system/skillhub/install-cli", InstallSkillHubCLI(&config.Config{}))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/skillhub/install-cli", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK        bool   `json:"ok"`
		Installed bool   `json:"installed"`
		BinPath   string `json:"binPath"`
		Output    string `json:"output"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || !resp.Installed {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if filepath.Base(resp.BinPath) != testExecutableBase("skillhub") {
		t.Fatalf("expected skillhub binary path, got %q", resp.BinPath)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", testExecutableBase("skillhub"))); err != nil {
		t.Fatalf("expected installed binary: %v", err)
	}
	if !strings.Contains(resp.Output, "installed cli") {
		t.Fatalf("expected installer output, got %q", resp.Output)
	}
}

func TestInstallSkillHubSkillRunsCommandInSelectedWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := resetSkillHubTestState(t)
	defer restore()

	root := resolvedTempDir(t)
	workspace := filepath.Join(root, "workspace")
	openClawDir := filepath.Join(root, "openclaw")
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	logPath := filepath.Join(root, "skillhub-call.log")
	binPath := filepath.Join(root, testExecutableBase("skillhub"))
	writeFakeSkillHubCommand(t, binPath, logPath, true)
	t.Setenv("SKILLHUB_BIN", binPath)
	prependTestPath(t, binDir)
	skillHubBinaryCandidatePaths = nil

	r := gin.New()
	r.POST("/system/skillhub/install", InstallSkillHubSkill(cfg))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/skillhub/install", strings.NewReader(`{"skillId":"demo-skill","agentId":"main"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"skillId":"demo-skill"`) {
		t.Fatalf("expected installed skill response, got %s", body)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read skillhub log: %v", err)
	}
	logText := string(logged)
	if !strings.Contains(logText, "pwd="+workspace) {
		t.Fatalf("expected workspace cwd, got %q", logText)
	}
	if !strings.Contains(logText, "args=install demo-skill") {
		t.Fatalf("expected install args, got %q", logText)
	}
	if !strings.Contains(logText, "pythonhttpsverify=0") || !strings.Contains(logText, "sslnoverify=1") {
		t.Fatalf("expected SSL bypass envs, got %q", logText)
	}
}

func TestInstallSkillHubSkillRejectsExistingWorkspaceTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := resetSkillHubTestState(t)
	defer restore()

	root := resolvedTempDir(t)
	workspace := filepath.Join(root, "workspace")
	openClawDir := filepath.Join(root, "openclaw")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	if err := os.MkdirAll(filepath.Join(workspace, "skills", "demo-skill"), 0o755); err != nil {
		t.Fatalf("create existing skill dir: %v", err)
	}
	binPath := filepath.Join(root, testExecutableBase("skillhub"))
	writeFakeSkillHubCommand(t, binPath, filepath.Join(root, "unused.log"), false)
	t.Setenv("SKILLHUB_BIN", binPath)
	skillHubBinaryCandidatePaths = nil

	r := gin.New()
	r.POST("/system/skillhub/install", InstallSkillHubSkill(cfg))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/skillhub/install", strings.NewReader(`{"skillId":"demo-skill","agentId":"main"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "already installed") {
		t.Fatalf("expected friendly conflict message, got %s", w.Body.String())
	}
}

func TestInstallSkillHubSkillGlobalTargetRunsInManagedRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := resetSkillHubTestState(t)
	defer restore()

	root := resolvedTempDir(t)
	workspace := filepath.Join(root, "workspace")
	openClawDir := filepath.Join(root, "openclaw")
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}
	logPath := filepath.Join(root, "skillhub-global.log")
	binPath := filepath.Join(root, testExecutableBase("skillhub"))
	writeFakeSkillHubCommand(t, binPath, logPath, false)
	t.Setenv("SKILLHUB_BIN", binPath)
	prependTestPath(t, binDir)
	skillHubBinaryCandidatePaths = nil

	r := gin.New()
	r.POST("/system/skillhub/install", InstallSkillHubSkill(cfg))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/system/skillhub/install", strings.NewReader(`{"skillId":"demo-skill","agentId":"main","installTarget":"global"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read skillhub log: %v", err)
	}
	logText := string(logged)
	if !strings.Contains(logText, "pwd="+openClawDir) {
		t.Fatalf("expected managed cwd, got %q", logText)
	}
	if _, err := os.Stat(filepath.Join(openClawDir, "skills", "demo-skill")); err != nil {
		t.Fatalf("expected managed skill dir, got %v", err)
	}
}

func TestGetSkillHubCatalogMergesWorkspaceInstallState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := resetSkillHubTestState(t)
	defer restore()

	root := resolvedTempDir(t)
	workspace := filepath.Join(root, "workspace")
	openClawDir := filepath.Join(root, "openclaw")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}

	skillHubCache = &skillHubCatalog{
		Total:       2,
		GeneratedAt: "2026-03-01T13:44:23Z",
		Featured:    []string{"installed-skill"},
		Categories:  map[string][]string{"AI 智能": {"ai"}},
		Skills: []skillHubSkillItem{
			{Slug: "installed-skill", Name: "Installed Skill"},
			{Slug: "failed-skill", Name: "Failed Skill"},
		},
	}
	skillHubCacheTime = time.Now()
	writeSkillFixture(t, filepath.Join(workspace, "skills", "installed-skill"), "Installed Skill", "Installed skill", "")
	writeJSON(t, filepath.Join(workspace, ".skillhub", "install-state.json"), map[string]skillHubInstallRecord{
		"failed-skill": {State: "failed", Message: "SSL certificate verify failed", UpdatedAt: 1772065840450},
	})

	r := gin.New()
	r.GET("/system/skillhub/catalog", GetSkillHubCatalog(cfg))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/skillhub/catalog?agentId=main", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK     bool `json:"ok"`
		Skills []struct {
			Slug           string `json:"slug"`
			Installed      bool   `json:"installed"`
			InstallState   string `json:"installState"`
			InstallMessage string `json:"installMessage"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %+v", resp.Skills)
	}
	if !resp.Skills[0].Installed || resp.Skills[0].InstallState != "installed" {
		t.Fatalf("expected installed skill state, got %+v", resp.Skills[0])
	}
	if resp.Skills[1].InstallState != "failed" || !strings.Contains(resp.Skills[1].InstallMessage, "SSL certificate verify failed") {
		t.Fatalf("expected failed install state, got %+v", resp.Skills[1])
	}
}

func TestGetSkillHubCatalogGlobalTargetUsesManagedState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := resetSkillHubTestState(t)
	defer restore()

	root := resolvedTempDir(t)
	workspace := filepath.Join(root, "workspace")
	openClawDir := filepath.Join(root, "openclaw")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}

	skillHubCache = &skillHubCatalog{
		Total:       1,
		GeneratedAt: "2026-03-01T13:44:23Z",
		Featured:    []string{"managed-skill"},
		Categories:  map[string][]string{"AI 智能": {"ai"}},
		Skills: []skillHubSkillItem{
			{Slug: "managed-skill", Name: "Managed Skill"},
		},
	}
	skillHubCacheTime = time.Now()
	writeSkillFixture(t, filepath.Join(openClawDir, "skills", "managed-skill"), "Managed Skill", "Managed skill", "")
	writeJSON(t, filepath.Join(openClawDir, ".skillhub", "install-state.json"), map[string]skillHubInstallRecord{
		"managed-skill": {State: "installed", UpdatedAt: 1772065840450},
	})

	r := gin.New()
	r.GET("/system/skillhub/catalog", GetSkillHubCatalog(cfg))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/skillhub/catalog?agentId=main&installTarget=global", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"installTarget":"global"`) || !strings.Contains(w.Body.String(), `"installed":true`) {
		t.Fatalf("expected global managed install state, got %s", w.Body.String())
	}
}

func TestGetSkillHubCatalogIgnoresNonDiscoverableDir(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := resetSkillHubTestState(t)
	defer restore()

	root := resolvedTempDir(t)
	workspace := filepath.Join(root, "workspace")
	openClawDir := filepath.Join(root, "openclaw")
	cfg := &config.Config{OpenClawDir: openClawDir, OpenClawWork: workspace}

	skillHubCache = &skillHubCatalog{
		Total:       1,
		GeneratedAt: "2026-03-01T13:44:23Z",
		Featured:    []string{"broken-skill"},
		Categories:  map[string][]string{"AI 智能": {"ai"}},
		Skills: []skillHubSkillItem{
			{Slug: "broken-skill", Name: "Broken Skill"},
		},
	}
	skillHubCacheTime = time.Now()
	if err := os.MkdirAll(filepath.Join(workspace, "skills", "broken-skill"), 0o755); err != nil {
		t.Fatalf("create invalid dir: %v", err)
	}

	r := gin.New()
	r.GET("/system/skillhub/catalog", GetSkillHubCatalog(cfg))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/skillhub/catalog?agentId=main", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"installed":true`) {
		t.Fatalf("expected invalid dir to remain not installed, got %s", w.Body.String())
	}
}

func resetSkillHubTestState(t *testing.T) func() {
	t.Helper()
	origClient := skillHubHTTPClient
	origCache := skillHubCache
	origCacheTime := skillHubCacheTime
	origLastGood := skillHubLastGoodURL
	origNextRetry := skillHubNextRetryTime
	origLastErr := skillHubLastErr
	origRefreshInFlight := skillHubRefreshInFlight
	origRefreshDone := skillHubRefreshDone
	origInstallClient := skillHubInstallHTTPClient
	origInstallKitURL := skillHubInstallKitURL
	origInstallShellURL := skillHubInstallShellURL
	origBinaryCandidates := append([]string(nil), skillHubBinaryCandidatePaths...)

	skillHubCache = nil
	skillHubCacheTime = time.Time{}
	skillHubLastGoodURL = ""
	skillHubNextRetryTime = time.Time{}
	skillHubLastErr = ""
	skillHubRefreshInFlight = false
	skillHubRefreshDone = nil
	skillHubInstallHTTPClient = &http.Client{Timeout: skillHubInstallTimeout}
	skillHubInstallKitURL = skillHubDefaultInstallKit
	skillHubBinaryCandidatePaths = []string{"/usr/local/bin/skillhub", "/opt/homebrew/bin/skillhub"}

	return func() {
		skillHubHTTPClient = origClient
		skillHubCache = origCache
		skillHubCacheTime = origCacheTime
		skillHubLastGoodURL = origLastGood
		skillHubNextRetryTime = origNextRetry
		skillHubLastErr = origLastErr
		skillHubRefreshInFlight = origRefreshInFlight
		skillHubRefreshDone = origRefreshDone
		skillHubInstallHTTPClient = origInstallClient
		skillHubInstallKitURL = origInstallKitURL
		skillHubInstallShellURL = origInstallShellURL
		skillHubBinaryCandidatePaths = origBinaryCandidates
	}
}

func newHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func validSkillHubCatalogJSON(slug string) string {
	return `{"total":1,"generated_at":"2026-03-01T13:44:23Z","featured":["` + slug + `"],"categories":{"AI 智能":["ai"]},"skills":[{"slug":"` + slug + `","name":"` + slug + `","description":"desc","description_zh":"描述","version":"1.0.0","homepage":"https://clawhub.ai/skills/` + slug + `","tags":["ai"],"downloads":1,"stars":1,"installs":1,"updated_at":1772065840450,"score":1.2,"owner":"clawhub"}]}`
}

func buildSkillHubInstallArchive(t *testing.T, installerPath string, installerContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gz)
	installerDir := filepath.Dir(installerPath)
	entries := []struct {
		name string
		body string
		mode int64
	}{
		{name: installerDir, mode: 0o755},
		{name: installerPath, body: installerContent, mode: 0o755},
		{name: filepath.Join(installerDir, "skills_store_cli.py"), body: "print('ok')\n", mode: 0o644},
		{name: filepath.Join(installerDir, "skills_upgrade.py"), body: "print('upgrade')\n", mode: 0o644},
		{name: filepath.Join(installerDir, "version.json"), body: `{"version":"1.0.0"}` + "\n", mode: 0o644},
		{name: filepath.Join(installerDir, "metadata.json"), body: `{"name":"skillhub"}` + "\n", mode: 0o644},
		{name: filepath.Join(installerDir, "skills_index.local.json"), body: `{"skills":[]}` + "\n", mode: 0o644},
	}
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: entry.mode}
		if entry.body == "" {
			header.Typeflag = tar.TypeDir
		} else {
			header.Typeflag = tar.TypeReg
			header.Size = int64(len(entry.body))
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if entry.body != "" {
			if _, err := tarWriter.Write([]byte(entry.body)); err != nil {
				t.Fatalf("write tar body: %v", err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}

func linkSystemCommands(t *testing.T, targetDir string, names ...string) {
	t.Helper()
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("look up %s: %v", name, err)
		}
		if err := os.Symlink(path, filepath.Join(targetDir, name)); err != nil {
			t.Fatalf("symlink %s: %v", name, err)
		}
	}
}

func setTestHomeEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func prependTestPath(t *testing.T, dir string) {
	t.Helper()
	orig := sanitizeTestPath(os.Getenv("PATH"))
	if orig == "" {
		t.Setenv("PATH", dir)
		return
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+orig)
}

func sanitizeTestPath(pathValue string) string {
	if pathValue == "" {
		return ""
	}
	skillHubPath, err := exec.LookPath("skillhub")
	if err != nil {
		return pathValue
	}
	removeDir := filepath.Dir(skillHubPath)
	parts := strings.Split(pathValue, string(os.PathListSeparator))
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if samePathForTest(part, removeDir) {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, string(os.PathListSeparator))
}

func samePathForTest(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func testExecutableBase(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".cmd"
	}
	return name
}

func writeFakeSkillHubCommand(t *testing.T, path, logPath string, includeEnv bool) {
	t.Helper()
	var content string
	var perm os.FileMode = 0o755
	if runtime.GOOS == "windows" {
		perm = 0o644
		content = buildWindowsFakeSkillHubCommand(logPath, includeEnv)
	} else {
		content = buildUnixFakeSkillHubCommand(logPath, includeEnv)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatalf("write fake skillhub: %v", err)
	}
}

func buildUnixFakeSkillHubCommand(logPath string, includeEnv bool) string {
	lines := []string{
		"#!/bin/sh",
		"set -eu",
		"printf 'pwd=%s\\n' \"$PWD\" > " + singleQuote(logPath),
		"printf 'args=%s %s\\n' \"$1\" \"$2\" >> " + singleQuote(logPath),
	}
	if includeEnv {
		lines = append(lines,
			"printf 'pythonhttpsverify=%s\\n' \"${PYTHONHTTPSVERIFY:-}\" >> "+singleQuote(logPath),
			"printf 'sslnoverify=%s\\n' \"${SSL_NO_VERIFY:-}\" >> "+singleQuote(logPath),
		)
	}
	lines = append(lines,
		"mkdir -p \"$PWD/skills/$2\"",
		"printf 'installed %s' \"$2\"",
		"",
	)
	return strings.Join(lines, "\n")
}

func buildWindowsFakeSkillHubCommand(logPath string, includeEnv bool) string {
	lines := []string{
		"@echo off",
		"setlocal",
		`> "` + strings.ReplaceAll(logPath, `"`, `""`) + `" echo pwd=%CD%`,
		`>> "` + strings.ReplaceAll(logPath, `"`, `""`) + `" echo args=%~1 %~2`,
	}
	if includeEnv {
		lines = append(lines,
			`>> "`+strings.ReplaceAll(logPath, `"`, `""`)+`" echo pythonhttpsverify=%PYTHONHTTPSVERIFY%`,
			`>> "`+strings.ReplaceAll(logPath, `"`, `""`)+`" echo sslnoverify=%SSL_NO_VERIFY%`,
		)
	}
	lines = append(lines,
		`mkdir "%CD%\skills\%~2" >nul 2>nul`,
		`<nul set /p "=installed %~2"`,
		`exit /b 0`,
		"",
	)
	return strings.Join(lines, "\r\n")
}

func singleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
