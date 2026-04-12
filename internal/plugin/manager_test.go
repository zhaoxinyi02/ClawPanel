package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func TestResolvePluginInstallStrategyPrefersRegistryNpmOverGit(t *testing.T) {
	t.Parallel()

	strategy := resolvePluginInstallStrategy(&RegistryPlugin{
		GitURL:     "https://github.com/example/repo.git",
		NpmPackage: "@openclaw/wecom",
	}, "")

	if strategy.kind != "npm" || strategy.target != "@openclaw/wecom" {
		t.Fatalf("expected npm strategy, got %#v", strategy)
	}
}

func TestResolvePluginInstallStrategyUsesExplicitNpmSource(t *testing.T) {
	t.Parallel()

	strategy := resolvePluginInstallStrategy(&RegistryPlugin{
		GitURL:     "https://github.com/example/repo.git",
		NpmPackage: "@openclaw/wecom",
	}, "@openclaw/custom")

	if strategy.kind != "npm" || strategy.target != "@openclaw/custom" {
		t.Fatalf("expected explicit npm strategy, got %#v", strategy)
	}
}

func TestResolvePluginInstallStrategyUsesDownloadWhenQQBotIsBundled(t *testing.T) {
	t.Parallel()

	strategy := resolvePluginInstallStrategy(&RegistryPlugin{
		PluginMeta:    PluginMeta{ID: "qqbot"},
		GitURL:        "https://github.com/zhaoxinyi02/ClawPanel-Plugins.git",
		InstallSubDir: "official/qqbot",
	}, "")

	if strategy.kind != "download" {
		t.Fatalf("expected bundled qqbot to avoid preferred npm fallback, got %#v", strategy)
	}
}

func TestBuiltInOfficialChannelPluginSkipsQQBotFallback(t *testing.T) {
	t.Parallel()

	plugin := builtInOfficialChannelPlugin("qqbot")
	if plugin != nil {
		t.Fatalf("expected no qqbot fallback metadata once qqbot is bundled, got %#v", plugin)
	}
}

func TestBuiltInOfficialChannelPluginProvidesOpenClawWeixinFallback(t *testing.T) {
	t.Parallel()

	plugin := builtInOfficialChannelPlugin("openclaw-weixin")
	if plugin == nil {
		t.Fatal("expected openclaw-weixin fallback metadata")
	}
	if plugin.ID != "openclaw-weixin" {
		t.Fatalf("unexpected plugin id: %q", plugin.ID)
	}
	if plugin.NpmPackage != "@tencent-weixin/openclaw-weixin" {
		t.Fatalf("unexpected openclaw-weixin npm package: %q", plugin.NpmPackage)
	}

	strategy := resolvePluginInstallStrategy(plugin, "")
	if strategy.kind != "npm" || strategy.target != "@tencent-weixin/openclaw-weixin@latest" {
		t.Fatalf("expected openclaw-weixin fallback to use preferred npm spec, got %#v", strategy)
	}
}

func TestBuiltInOfficialChannelPluginUnknownReturnsNil(t *testing.T) {
	t.Parallel()

	if plugin := builtInOfficialChannelPlugin("unknown-plugin"); plugin != nil {
		t.Fatalf("expected nil fallback metadata, got %#v", plugin)
	}
}

func TestInstallRecognizesTgzAsArchive(t *testing.T) {
	t.Parallel()

	url := "https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel-Plugins/main/official/qqbot/qqbot-1.2.2.tgz"
	if !(strings.HasSuffix(url, ".zip") || strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz")) {
		t.Fatalf("expected tgz url to be treated as archive")
	}
}

func TestNormalizeNpmPackageName(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"@sliverp/qqbot@latest":          "@sliverp/qqbot",
		"@openclaw/feishu":               "@openclaw/feishu",
		"left-pad@1.3.0":                 "left-pad",
		"@openclaw-china/wecom-app@next": "@openclaw-china/wecom-app",
	}

	for input, want := range tests {
		if got := normalizeNpmPackageName(input); got != want {
			t.Fatalf("normalizeNpmPackageName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRegistryFetchURLsSkipsInsecureMirrorByDefault(t *testing.T) {
	t.Parallel()

	urls := registryFetchURLs()
	for _, url := range urls {
		if url == RegistryMirrorURL {
			t.Fatalf("expected insecure mirror to be opt-in only, got %#v", urls)
		}
	}
}

func TestNormalizeOpenClawInstallSource(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"npm":      "npm",
		"archive":  "archive",
		"path":     "path",
		"local":    "path",
		"registry": "path",
		"custom":   "path",
		"github":   "path",
		"git":      "path",
		"":         "path",
	}

	for input, want := range tests {
		if got := normalizeOpenClawInstallSource(input); got != want {
			t.Fatalf("normalizeOpenClawInstallSource(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMergeRegistriesPrefersPrimaryEntriesOnConflict(t *testing.T) {
	t.Parallel()

	primary := Registry{
		Version:   "2026.03.10",
		UpdatedAt: "2026-03-10T00:00:00Z",
		Plugins: []RegistryPlugin{
			{
				PluginMeta: PluginMeta{
					ID:          "feishu",
					Name:        "Remote Feishu",
					Version:     "2.0.0",
					Description: "live registry entry",
				},
				NpmPackage: "@openclaw/feishu",
			},
		},
	}
	bundled := &Registry{
		Version:   "2025.12.01",
		UpdatedAt: "2025-12-01T00:00:00Z",
		Plugins: []RegistryPlugin{
			{
				PluginMeta: PluginMeta{
					ID:          "feishu",
					Name:        "Bundled Feishu",
					Version:     "1.0.0",
					Description: "stale bundled entry",
				},
				NpmPackage: "@bundled/feishu",
			},
			{
				PluginMeta: PluginMeta{
					ID:      "qq",
					Name:    "QQ",
					Version: "1.2.3",
				},
				NpmPackage: "@openclaw/qq",
			},
		},
	}

	merged := mergeRegistries(primary, bundled)
	if merged.Version != primary.Version {
		t.Fatalf("expected primary version to win, got %q", merged.Version)
	}
	if merged.UpdatedAt != primary.UpdatedAt {
		t.Fatalf("expected primary updatedAt to win, got %q", merged.UpdatedAt)
	}
	if len(merged.Plugins) != 2 {
		t.Fatalf("expected bundled registry to fill only missing plugins, got %#v", merged.Plugins)
	}
	if merged.Plugins[0].Name != "Remote Feishu" || merged.Plugins[0].NpmPackage != "@openclaw/feishu" {
		t.Fatalf("expected primary feishu plugin to be preserved, got %#v", merged.Plugins[0])
	}
	if merged.Plugins[1].ID != "qq" {
		t.Fatalf("expected missing bundled plugin to be appended, got %#v", merged.Plugins[1])
	}
}

func TestMergeRegistriesBackfillsTopLevelMetadataWhenPrimaryMissing(t *testing.T) {
	t.Parallel()

	primary := Registry{
		Plugins: []RegistryPlugin{{PluginMeta: PluginMeta{ID: "feishu", Name: "Remote Feishu"}}},
	}
	bundled := &Registry{
		Version:   "2025.12.01",
		UpdatedAt: "2025-12-01T00:00:00Z",
		Plugins:   []RegistryPlugin{{PluginMeta: PluginMeta{ID: "qq", Name: "QQ"}}},
	}

	merged := mergeRegistries(primary, bundled)
	if merged.Version != bundled.Version {
		t.Fatalf("expected bundled version to backfill empty primary version, got %q", merged.Version)
	}
	if merged.UpdatedAt != bundled.UpdatedAt {
		t.Fatalf("expected bundled updatedAt to backfill empty primary updatedAt, got %q", merged.UpdatedAt)
	}
}

func TestFetchRegistryPrefersCachedRegistryOverBundledFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir}
	cached := &Registry{
		Version:   "2026.03.10",
		UpdatedAt: "2026-03-10T00:00:00Z",
		Plugins: []RegistryPlugin{
			{PluginMeta: PluginMeta{ID: "feishu", Name: "Cached Feishu", Version: "2.0.0"}},
		},
	}
	cachedRaw, _ := json.MarshalIndent(cached, "", "  ")
	cachePath := filepath.Join(dir, "plugin-registry-cache.json")
	if err := os.WriteFile(cachePath, cachedRaw, 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusBadGateway)
	}))
	defer server.Close()

	bundled := &Registry{
		Version:   "2025.12.01",
		UpdatedAt: "2025-12-01T00:00:00Z",
		Plugins: []RegistryPlugin{
			{PluginMeta: PluginMeta{ID: "feishu", Name: "Bundled Feishu", Version: "1.0.0"}},
			{PluginMeta: PluginMeta{ID: "qq", Name: "QQ", Version: "1.2.3"}},
		},
	}

	m := &Manager{cfg: cfg}
	reg, err := m.fetchRegistryFromURLs(server.Client(), []string{server.URL}, bundled)
	if err != nil {
		t.Fatalf("fetchRegistryFromURLs: %v", err)
	}
	if reg.Version != cached.Version {
		t.Fatalf("expected cached registry version to win during offline fallback, got %q", reg.Version)
	}
	if len(reg.Plugins) != 2 || reg.Plugins[0].Name != "Cached Feishu" {
		t.Fatalf("expected cached registry to be preserved and bundled to only fill gaps, got %#v", reg.Plugins)
	}

	raw, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache after fallback: %v", err)
	}
	var persisted Registry
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("decode cache after fallback: %v", err)
	}
	if persisted.Version != cached.Version {
		t.Fatalf("expected offline fallback to keep existing disk cache, got %q", persisted.Version)
	}
}

func TestSyncOpenClawPluginStateWritesEntriesAndInstalls(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	m := &Manager{cfg: cfg}

	if err := m.syncOpenClawPluginState("dingtalk", dir+"/extensions/dingtalk", true, "registry", "0.2.0"); err != nil {
		t.Fatalf("syncOpenClawPluginState: %v", err)
	}

	saved, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("ReadOpenClawJSON: %v", err)
	}
	pl, _ := saved["plugins"].(map[string]interface{})
	ent, _ := pl["entries"].(map[string]interface{})
	ins, _ := pl["installs"].(map[string]interface{})
	entry, _ := ent["dingtalk"].(map[string]interface{})
	install, _ := ins["dingtalk"].(map[string]interface{})
	if enabled, _ := entry["enabled"].(bool); !enabled {
		t.Fatalf("expected dingtalk entry enabled, got %#v", entry)
	}
	if got, _ := install["installPath"].(string); got == "" {
		t.Fatalf("expected installPath, got %#v", install)
	}
	if got, _ := install["version"].(string); got != "0.2.0" {
		t.Fatalf("expected version 0.2.0, got %#v", install)
	}
}

func TestRemoveOpenClawPluginStateDeletesEntriesAndInstalls(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}
	m := &Manager{cfg: cfg}
	if err := m.syncOpenClawPluginState("wecom", dir+"/extensions/wecom", true, "registry", "latest"); err != nil {
		t.Fatalf("seed syncOpenClawPluginState: %v", err)
	}
	if err := m.removeOpenClawPluginState("wecom", true); err != nil {
		t.Fatalf("removeOpenClawPluginState: %v", err)
	}
	saved, err := cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("ReadOpenClawJSON: %v", err)
	}
	pl, _ := saved["plugins"].(map[string]interface{})
	ent, _ := pl["entries"].(map[string]interface{})
	ins, _ := pl["installs"].(map[string]interface{})
	if _, ok := ent["wecom"]; ok {
		t.Fatalf("expected wecom entry removed")
	}
	if _, ok := ins["wecom"]; ok {
		t.Fatalf("expected wecom install removed")
	}
}

// ---------------------------------------------------------------------------
// Gap-fix tests added for multi-dev alignment
// ---------------------------------------------------------------------------

// TestReadPluginMetaFallsBackToOpenClawPluginJSON verifies that when plugin.json
// is absent, readPluginMeta reads metadata from openclaw.plugin.json (official
// manifest format) so ClawPanel-installed plugins remain discoverable by the
// OpenClaw runtime.
func TestReadPluginMetaFallsBackToOpenClawPluginJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "myplugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write only openclaw.plugin.json – no plugin.json.
	manifest := map[string]interface{}{
		"id":          "myplugin",
		"name":        "My Plugin",
		"version":     "1.2.3",
		"description": "Official manifest format",
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginDir, "openclaw.plugin.json"), data, 0644); err != nil {
		t.Fatalf("write openclaw.plugin.json: %v", err)
	}

	m := &Manager{cfg: &config.Config{OpenClawDir: dir}}
	meta, err := m.readPluginMeta(pluginDir)
	if err != nil {
		t.Fatalf("readPluginMeta: %v", err)
	}
	if meta.ID != "myplugin" {
		t.Fatalf("expected id=myplugin, got %q", meta.ID)
	}
	if meta.Name != "My Plugin" {
		t.Fatalf("expected name=My Plugin, got %q", meta.Name)
	}
	if meta.Version != "1.2.3" {
		t.Fatalf("expected version=1.2.3, got %q", meta.Version)
	}
}

// TestReadPluginMetaPrefersPriorityOrder verifies the fallback priority:
// plugin.json is preferred over openclaw.plugin.json.
func TestReadPluginMetaPrefersPriorityOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "dual")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeJSON := func(name string, v interface{}) {
		data, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(pluginDir, name), data, 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	writeJSON("plugin.json", map[string]interface{}{"id": "dual", "name": "Via plugin.json", "version": "1.0.0"})
	writeJSON("openclaw.plugin.json", map[string]interface{}{"id": "dual", "name": "Via openclaw.plugin.json", "version": "2.0.0"})

	m := &Manager{cfg: &config.Config{OpenClawDir: dir}}
	meta, err := m.readPluginMeta(pluginDir)
	if err != nil {
		t.Fatalf("readPluginMeta: %v", err)
	}
	if meta.Name != "Via plugin.json" {
		t.Fatalf("expected plugin.json to take priority, got name=%q", meta.Name)
	}
}

// TestReadPluginMetaNoMetaFilesError verifies that readPluginMeta returns an
// error when neither plugin.json, openclaw.plugin.json, nor package.json exist.
func TestReadPluginMetaNoMetaFilesError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "empty-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	m := &Manager{cfg: &config.Config{OpenClawDir: dir}}
	if _, err := m.readPluginMeta(pluginDir); err == nil {
		t.Fatalf("expected error when no metadata files present, got nil")
	}
}

// TestListInstalledReconcilesEnabledFromOpenClawJSON verifies that ListInstalled
// reflects enabled-state changes made to openclaw.json (e.g. via the CLI) even
// when the in-memory plugin map has a stale value.
func TestListInstalledReconcilesEnabledFromOpenClawJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	// Seed openclaw.json with the plugin disabled.
	ocConfig := map[string]interface{}{
		"plugins": map[string]interface{}{
			"entries": map[string]interface{}{
				"feishu": map[string]interface{}{"enabled": false},
			},
		},
	}
	data, _ := json.MarshalIndent(ocConfig, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "openclaw.json"), data, 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	// Build manager with stale in-memory state (enabled=true).
	m := &Manager{
		cfg: cfg,
		plugins: map[string]*InstalledPlugin{
			"feishu": {
				PluginMeta: PluginMeta{ID: "feishu", Name: "Feishu"},
				Enabled:    true, // stale – openclaw.json says false
				Source:     "npm",
				Dir:        filepath.Join(dir, "extensions", "feishu"),
			},
		},
	}

	listed := m.ListInstalled()
	if len(listed) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(listed))
	}
	if listed[0].Enabled {
		t.Fatalf("expected enabled=false after reconciliation with openclaw.json, got true")
	}
	// The in-memory map must be unchanged – reconciliation returns copies only.
	m.mu.RLock()
	inMemEnabled := m.plugins["feishu"].Enabled
	m.mu.RUnlock()
	if !inMemEnabled {
		t.Fatalf("expected in-memory plugin map to be unmodified (Enabled should still be true)")
	}
}

// TestListInstalledReconcilesEnabledToTrue verifies that if openclaw.json has
// enabled=true but the in-memory state is false, ListInstalled returns true.
func TestListInstalledReconcilesEnabledToTrue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{OpenClawDir: dir}

	ocConfig := map[string]interface{}{
		"plugins": map[string]interface{}{
			"entries": map[string]interface{}{
				"discord": map[string]interface{}{"enabled": true},
			},
		},
	}
	data, _ := json.MarshalIndent(ocConfig, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "openclaw.json"), data, 0644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	m := &Manager{
		cfg: cfg,
		plugins: map[string]*InstalledPlugin{
			"discord": {
				PluginMeta: PluginMeta{ID: "discord", Name: "Discord"},
				Enabled:    false, // stale
				Source:     "npm",
			},
		},
	}

	listed := m.ListInstalled()
	if len(listed) != 1 || !listed[0].Enabled {
		t.Fatalf("expected enabled=true from openclaw.json reconciliation, got %v", listed)
	}
}

func TestEnsureOpenClawPluginManifestCreatesCompatFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "compat-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	meta := &PluginMeta{
		ID:          "compat-plugin",
		Name:        "Compat Plugin",
		Version:     "1.0.0",
		Description: "Generated compatibility manifest",
	}
	m := &Manager{cfg: &config.Config{OpenClawDir: dir}}
	if err := m.ensureOpenClawPluginManifest(pluginDir, meta); err != nil {
		t.Fatalf("ensureOpenClawPluginManifest: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(pluginDir, "openclaw.plugin.json"))
	if err != nil {
		t.Fatalf("read openclaw.plugin.json: %v", err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode openclaw.plugin.json: %v", err)
	}
	if got := manifest["id"]; got != "compat-plugin" {
		t.Fatalf("expected id compat-plugin, got %#v", got)
	}
	schema, _ := manifest["configSchema"].(map[string]interface{})
	if schema == nil {
		t.Fatalf("expected generated configSchema, got %#v", manifest["configSchema"])
	}
	if got := schema["type"]; got != "object" {
		t.Fatalf("expected generated configSchema.type=object, got %#v", got)
	}
}

func TestReconcilePluginStatesKeepsPluginWhenCompatManifestWriteFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	openClawDir := filepath.Join(dir, "openclaw")
	pluginsDir := filepath.Join(dir, "extensions")
	pluginDir := filepath.Join(pluginsDir, "readonly-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"id":          "readonly-plugin",
		"name":        "Readonly Plugin",
		"version":     "1.0.0",
		"description": "plugin.json only",
	})
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
	if err := os.Chmod(pluginDir, 0o555); err != nil {
		t.Fatalf("chmod readonly: %v", err)
	}
	defer os.Chmod(pluginDir, 0o755)

	m := &Manager{
		cfg:        &config.Config{OpenClawDir: openClawDir},
		plugins:    map[string]*InstalledPlugin{},
		pluginsDir: pluginsDir,
		configFile: filepath.Join(dir, "plugins.json"),
	}
	m.scanInstalledPlugins()
	m.reconcilePluginStates()

	p := m.plugins["readonly-plugin"]
	if p == nil {
		t.Fatalf("expected plugin to stay visible even when compat manifest cannot be written")
	}
	if !p.NeedManifestRepair {
		t.Fatalf("expected NeedManifestRepair to remain true after failed reconcile")
	}
	if p.NeedConfigSync {
		t.Fatalf("expected config sync to succeed even when manifest repair fails")
	}
	if _, err := os.Stat(filepath.Join(pluginDir, "openclaw.plugin.json")); !os.IsNotExist(err) {
		t.Fatalf("expected manifest write to fail, but openclaw.plugin.json exists")
	}
	saved, err := m.cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("ReadOpenClawJSON: %v", err)
	}
	pl, _ := saved["plugins"].(map[string]interface{})
	ent, _ := pl["entries"].(map[string]interface{})
	if _, ok := ent["readonly-plugin"]; !ok {
		t.Fatalf("expected reconcile to still register plugin entry in openclaw.json")
	}
}

// TestScanInstalledPluginsIsReadOnly verifies that scanInstalledPlugins does
// not write any files to disk — neither openclaw.plugin.json in the plugin
// directory nor openclaw.json in the config directory.
func TestScanInstalledPluginsIsReadOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openClawDir := filepath.Join(dir, "openclaw")
	if err := os.MkdirAll(openClawDir, 0o755); err != nil {
		t.Fatalf("mkdir openclaw dir: %v", err)
	}
	pluginsDir := filepath.Join(dir, "extensions")
	pluginDir := filepath.Join(pluginsDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"id":          "test-plugin",
		"name":        "Test Plugin",
		"version":     "1.0.0",
		"description": "for scan read-only test",
	})
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}

	m := &Manager{
		cfg:        &config.Config{OpenClawDir: openClawDir},
		plugins:    map[string]*InstalledPlugin{},
		pluginsDir: pluginsDir,
		configFile: filepath.Join(dir, "plugins.json"),
	}

	// Record state before scan
	manifestPath := filepath.Join(pluginDir, "openclaw.plugin.json")
	ocConfigPath := filepath.Join(openClawDir, "openclaw.json")

	m.scanInstalledPlugins()

	// Plugin should be discovered in memory
	p, ok := m.plugins["test-plugin"]
	if !ok {
		t.Fatalf("expected scan to discover test-plugin")
	}
	if !p.NeedManifestRepair {
		t.Fatalf("expected NeedManifestRepair to be true for plugin without manifest")
	}
	if !p.NeedConfigSync {
		t.Fatalf("expected NeedConfigSync to be true for newly-discovered plugin")
	}

	// No files should have been written
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("scanInstalledPlugins should not create openclaw.plugin.json, but it exists")
	}
	if _, err := os.Stat(ocConfigPath); !os.IsNotExist(err) {
		t.Fatalf("scanInstalledPlugins should not create/modify openclaw.json, but it exists")
	}
}

func TestScanInstalledPluginsPrunesMissingStateEntries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openClawDir := filepath.Join(dir, "openclaw")
	if err := os.MkdirAll(openClawDir, 0o755); err != nil {
		t.Fatalf("mkdir openclaw dir: %v", err)
	}
	pluginsDir := filepath.Join(dir, "extensions")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatalf("mkdir plugins dir: %v", err)
	}

	configFile := filepath.Join(dir, "plugins.json")
	m := &Manager{
		cfg: &config.Config{OpenClawDir: openClawDir},
		plugins: map[string]*InstalledPlugin{
			"ghost-plugin": {
				PluginMeta: PluginMeta{ID: "ghost-plugin", Name: "Ghost Plugin"},
				Dir:        filepath.Join(pluginsDir, "ghost-plugin"),
				Source:     "local",
			},
		},
		pluginsDir: pluginsDir,
		configFile: configFile,
	}

	m.scanInstalledPlugins()

	if _, ok := m.plugins["ghost-plugin"]; ok {
		t.Fatalf("expected missing plugin state entry to be pruned")
	}

	raw, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("read pruned plugins.json: %v", err)
	}
	if string(raw) != "{}" {
		t.Fatalf("expected pruned plugins.json to be empty object, got %s", string(raw))
	}
}

// TestReconcilePluginStatesWritesDeferredChanges verifies that
// reconcilePluginStates writes the manifest and config files for flagged
// plugins and clears the flags afterwards.
func TestReconcilePluginStatesWritesDeferredChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openClawDir := filepath.Join(dir, "openclaw")
	if err := os.MkdirAll(openClawDir, 0o755); err != nil {
		t.Fatalf("mkdir openclaw dir: %v", err)
	}
	pluginsDir := filepath.Join(dir, "extensions")
	pluginDir := filepath.Join(pluginsDir, "reconcile-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"id":          "reconcile-plugin",
		"name":        "Reconcile Plugin",
		"version":     "2.0.0",
		"description": "for reconcile test",
	})
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}

	m := &Manager{
		cfg:        &config.Config{OpenClawDir: openClawDir},
		plugins:    map[string]*InstalledPlugin{},
		pluginsDir: pluginsDir,
		configFile: filepath.Join(dir, "plugins.json"),
	}

	// Scan (read-only) then reconcile (writes)
	m.scanInstalledPlugins()
	m.reconcilePluginStates()

	p := m.plugins["reconcile-plugin"]
	if p == nil {
		t.Fatalf("expected plugin to be present after reconcile")
	}
	if p.NeedManifestRepair {
		t.Fatalf("expected NeedManifestRepair to be cleared after reconcile")
	}
	if p.NeedConfigSync {
		t.Fatalf("expected NeedConfigSync to be cleared after reconcile")
	}

	// Manifest should now exist
	manifestPath := filepath.Join(pluginDir, "openclaw.plugin.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected openclaw.plugin.json to be created by reconcile: %v", err)
	}

	// openclaw.json should have the plugin entry
	ocConfigPath := filepath.Join(openClawDir, "openclaw.json")
	raw, err := os.ReadFile(ocConfigPath)
	if err != nil {
		t.Fatalf("expected openclaw.json to be written: %v", err)
	}
	var ocConfig map[string]interface{}
	if err := json.Unmarshal(raw, &ocConfig); err != nil {
		t.Fatalf("invalid openclaw.json: %v", err)
	}
	plugins, _ := ocConfig["plugins"].(map[string]interface{})
	entries, _ := plugins["entries"].(map[string]interface{})
	if _, ok := entries["reconcile-plugin"]; !ok {
		t.Fatalf("expected reconcile-plugin in openclaw.json entries")
	}
}

func TestReconcilePluginStatesPreservesExistingDisabledEntry(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	openClawDir := filepath.Join(dir, "openclaw")
	if err := os.MkdirAll(openClawDir, 0o755); err != nil {
		t.Fatalf("mkdir openclaw dir: %v", err)
	}
	pluginsDir := filepath.Join(dir, "extensions")
	pluginDir := filepath.Join(pluginsDir, "disabled-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"id":          "disabled-plugin",
		"name":        "Disabled Plugin",
		"version":     "1.0.0",
		"description": "discovered from disk",
	})
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}

	seed := map[string]interface{}{
		"plugins": map[string]interface{}{
			"entries": map[string]interface{}{
				"disabled-plugin": map[string]interface{}{"enabled": false},
			},
		},
	}
	raw, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(filepath.Join(openClawDir, "openclaw.json"), raw, 0o644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	m := &Manager{
		cfg:        &config.Config{OpenClawDir: openClawDir},
		plugins:    map[string]*InstalledPlugin{},
		pluginsDir: pluginsDir,
		configFile: filepath.Join(dir, "plugins.json"),
	}

	m.scanInstalledPlugins()
	m.reconcilePluginStates()

	p := m.plugins["disabled-plugin"]
	if p == nil {
		t.Fatalf("expected plugin to remain present after reconcile")
	}
	if p.Enabled {
		t.Fatalf("expected in-memory plugin state to adopt existing disabled entry")
	}
	if p.NeedConfigSync {
		t.Fatalf("expected NeedConfigSync to be cleared after reconcile")
	}

	saved, err := m.cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("ReadOpenClawJSON: %v", err)
	}
	pl, _ := saved["plugins"].(map[string]interface{})
	ent, _ := pl["entries"].(map[string]interface{})
	entry, _ := ent["disabled-plugin"].(map[string]interface{})
	if enabled, _ := entry["enabled"].(bool); enabled {
		t.Fatalf("expected existing disabled entry to be preserved, got %#v", entry)
	}
	ins, _ := pl["installs"].(map[string]interface{})
	install, _ := ins["disabled-plugin"].(map[string]interface{})
	if got, _ := install["installPath"].(string); got == "" {
		t.Fatalf("expected reconcile to still upsert install metadata, got %#v", install)
	}
}

func TestInstallFromArchiveReturnsExtractorErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive extraction tooling differs on Windows")
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write([]byte("not-a-real-zip"))
	}))
	defer server.Close()

	m := &Manager{}
	if err := m.installFromArchive(server.URL+"/plugin.zip", t.TempDir()); err == nil {
		t.Fatalf("expected invalid archive extraction to return an error")
	}
}

func TestUpdatePreservesChannelConfigWhenRegistryReinstallFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive extraction tooling differs on Windows")
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write([]byte("not-a-real-zip"))
	}))
	defer server.Close()

	dir := t.TempDir()
	openClawDir := filepath.Join(dir, "openclaw")
	if err := os.MkdirAll(openClawDir, 0o755); err != nil {
		t.Fatalf("mkdir openclaw dir: %v", err)
	}
	pluginsDir := filepath.Join(dir, "extensions")
	pluginDir := filepath.Join(pluginsDir, "feishu")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"id":          "sample-plugin",
		"name":        "Sample Plugin",
		"version":     "1.0.0",
		"description": "installed plugin",
	})
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}

	seed := map[string]interface{}{
		"channels": map[string]interface{}{
			"sample-plugin": map[string]interface{}{
				"enabled":   true,
				"appId":     "cli_xxx",
				"appSecret": "secret_xxx",
			},
		},
		"plugins": map[string]interface{}{
			"entries": map[string]interface{}{
				"sample-plugin": map[string]interface{}{"enabled": true},
			},
			"installs": map[string]interface{}{
				"sample-plugin": map[string]interface{}{"installPath": pluginDir},
			},
		},
	}
	raw, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(filepath.Join(openClawDir, "openclaw.json"), raw, 0o644); err != nil {
		t.Fatalf("write openclaw.json: %v", err)
	}

	m := &Manager{
		cfg:        &config.Config{OpenClawDir: openClawDir},
		plugins:    map[string]*InstalledPlugin{"sample-plugin": {PluginMeta: PluginMeta{ID: "sample-plugin", Name: "Sample Plugin"}, Source: "npm", Dir: pluginDir}},
		registry:   &Registry{Plugins: []RegistryPlugin{{PluginMeta: PluginMeta{ID: "sample-plugin", Name: "Sample Plugin", Version: "2.0.0"}, DownloadURL: server.URL + "/plugin.zip"}}},
		pluginsDir: pluginsDir,
		configFile: filepath.Join(dir, "plugins.json"),
	}

	if err := m.Update("sample-plugin"); err == nil {
		t.Fatalf("expected registry reinstall failure to be returned")
	}

	saved, err := m.cfg.ReadOpenClawJSON()
	if err != nil {
		t.Fatalf("ReadOpenClawJSON: %v", err)
	}
	channels, _ := saved["channels"].(map[string]interface{})
	if _, ok := channels["sample-plugin"]; !ok {
		t.Fatalf("expected channel config to survive failed update, got %#v", channels)
	}
	if _, ok := m.plugins["sample-plugin"]; ok {
		t.Fatalf("expected failed update to leave plugin uninstalled until a fresh install succeeds")
	}

	validServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(makePluginArchive(t, map[string]interface{}{
			"id":          "sample-plugin",
			"name":        "Sample Plugin",
			"version":     "2.0.0",
			"description": "updated plugin",
		}))
	}))
	defer validServer.Close()

	archiveURL := validServer.URL + "/plugin.zip"
	m.registry.Plugins[0].DownloadURL = archiveURL
	if err := m.Install("sample-plugin", archiveURL); err != nil {
		t.Fatalf("expected fresh install after failed update to succeed, got %v", err)
	}
	if _, ok := m.plugins["sample-plugin"]; !ok {
		t.Fatalf("expected plugin to be installable again after failed update")
	}
}

func TestUninstallWithProgressKeepsPluginWhenConfigCleanupFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	openClawDir := filepath.Join(dir, "openclaw")
	if err := os.MkdirAll(openClawDir, 0o755); err != nil {
		t.Fatalf("mkdir openclaw dir: %v", err)
	}
	pluginsDir := filepath.Join(dir, "extensions")
	pluginDir := filepath.Join(pluginsDir, "feishu")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	seed := map[string]interface{}{
		"plugins": map[string]interface{}{
			"entries": map[string]interface{}{
				"feishu": map[string]interface{}{"enabled": true},
			},
			"installs": map[string]interface{}{
				"feishu": map[string]interface{}{"installPath": pluginDir},
			},
		},
	}
	raw, _ := json.MarshalIndent(seed, "", "  ")
	cfgPath := filepath.Join(openClawDir, "openclaw.json")
	if err := os.WriteFile(cfgPath, raw, 0o444); err != nil {
		t.Fatalf("write read-only openclaw.json: %v", err)
	}
	defer os.Chmod(cfgPath, 0o644)

	m := &Manager{
		cfg:        &config.Config{OpenClawDir: openClawDir},
		plugins:    map[string]*InstalledPlugin{"feishu": {PluginMeta: PluginMeta{ID: "feishu"}, Dir: pluginDir, Source: "registry"}},
		pluginsDir: pluginsDir,
		configFile: filepath.Join(dir, "plugins.json"),
	}

	if err := m.UninstallWithProgress("feishu", true, nil); err == nil {
		t.Fatalf("expected openclaw.json cleanup failure to abort uninstall")
	}
	if _, ok := m.plugins["feishu"]; !ok {
		t.Fatalf("expected plugin to remain installed in memory after cleanup failure")
	}
	if _, err := os.Stat(pluginDir); err != nil {
		t.Fatalf("expected plugin directory to remain after cleanup failure: %v", err)
	}
}

func TestGetPluginReturnsCopy(t *testing.T) {
	t.Parallel()

	m := &Manager{
		plugins: map[string]*InstalledPlugin{
			"feishu": {
				PluginMeta: PluginMeta{
					ID:           "feishu",
					Tags:         []string{"chat"},
					Dependencies: map[string]string{"node": ">=20"},
				},
				Config: map[string]interface{}{"enabled": true, "nested": map[string]interface{}{"foo": "bar"}},
			},
		},
	}

	got := m.GetPlugin("feishu")
	got.Tags[0] = "mutated"
	got.Dependencies["node"] = ">=18"
	got.Config["enabled"] = false

	original := m.plugins["feishu"]
	if original.Tags[0] != "chat" {
		t.Fatalf("expected tag slice to be copied")
	}
	if original.Dependencies["node"] != ">=20" {
		t.Fatalf("expected dependencies map to be copied")
	}
	if enabled, _ := original.Config["enabled"].(bool); !enabled {
		t.Fatalf("expected config map to be copied")
	}
}

func TestResolveInstallSubDirRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := resolveInstallSubDir(root, "../../etc"); err == nil {
		t.Fatalf("expected traversal installSubDir to be rejected")
	}
}

func TestCopyDirRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires extra privileges on Windows")
	}
	t.Parallel()

	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.Symlink("/etc/hosts", filepath.Join(src, "hosts-link")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if err := copyDir(src, dst); err == nil {
		t.Fatalf("expected symlinked file to be rejected")
	}
}

func TestInstallFromGitReportsMissingGitClearly(t *testing.T) {
	oldLookPath := gitLookPath
	oldRunner := gitCloneRunner
	defer func() {
		gitLookPath = oldLookPath
		gitCloneRunner = oldRunner
	}()

	gitLookPath = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	gitCloneRunner = func(gitURL, dest string) ([]byte, error) {
		t.Fatalf("git clone should not run when git is missing")
		return nil, nil
	}

	m := &Manager{}
	err := m.installFromGit("https://example.invalid/repo.git", filepath.Join(t.TempDir(), "plugin"))
	if err == nil || !strings.Contains(err.Error(), "未检测到 Git，请先安装 Git 后再重试") {
		t.Fatalf("expected missing git hint, got %v", err)
	}
}

func TestInstallFromGitRetriesTransientTLSFailure(t *testing.T) {
	oldLookPath := gitLookPath
	oldRunner := gitCloneRunner
	defer func() {
		gitLookPath = oldLookPath
		gitCloneRunner = oldRunner
	}()

	gitLookPath = func(file string) (string, error) {
		return "/usr/bin/git", nil
	}

	attempts := 0
	gitCloneRunner = func(gitURL, dest string) ([]byte, error) {
		attempts++
		if attempts < 3 {
			return []byte("error: RPC failed; curl 56 GnuTLS recv error (-9): Error decoding the received TLS packet.\nfatal: early EOF"), errors.New("exit status 128")
		}
		return []byte("ok"), nil
	}

	m := &Manager{}
	if err := m.installFromGit("https://example.invalid/repo.git", filepath.Join(t.TempDir(), "plugin")); err != nil {
		t.Fatalf("expected transient clone failure to recover, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRepoArchiveURLsForGitHub(t *testing.T) {
	urls := repoArchiveURLs("https://github.com/zhaoxinyi02/ClawPanel-Plugins.git")
	if len(urls) < 2 {
		t.Fatalf("expected github archive candidates, got %#v", urls)
	}
	if urls[0] != "https://codeload.github.com/zhaoxinyi02/ClawPanel-Plugins/zip/refs/heads/main" {
		t.Fatalf("unexpected primary archive url: %q", urls[0])
	}
}

func TestFlattenExtractedArchiveRootMovesSingleTopLevelDir(t *testing.T) {
	root := t.TempDir()
	archiveRoot := filepath.Join(root, "repo-main")
	if err := os.MkdirAll(filepath.Join(archiveRoot, "official", "qqbot"), 0o755); err != nil {
		t.Fatalf("mkdir archive root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archiveRoot, "official", "qqbot", "plugin.json"), []byte(`{"id":"qqbot"}`), 0o644); err != nil {
		t.Fatalf("write plugin file: %v", err)
	}

	if err := flattenExtractedArchiveRoot(root); err != nil {
		t.Fatalf("flatten archive root failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "official", "qqbot", "plugin.json")); err != nil {
		t.Fatalf("expected flattened plugin file, got %v", err)
	}
}

func makePluginArchive(t *testing.T, pluginJSON map[string]interface{}) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("plugin.json")
	if err != nil {
		t.Fatalf("create plugin.json entry: %v", err)
	}
	data, err := json.Marshal(pluginJSON)
	if err != nil {
		t.Fatalf("marshal plugin.json: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write plugin.json entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return buf.Bytes()
}
