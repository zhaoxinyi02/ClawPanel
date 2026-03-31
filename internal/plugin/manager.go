package plugin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

const (
	// RegistryURL is the official plugin registry
	RegistryURL = "https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel-Plugins/main/registry.json"
	// RegistryMirrorURL is the China mirror
	RegistryMirrorURL = "http://47.76.58.84:16198/clawpanel/plugins/registry.json"
	// RegistryFallbackURLCN is the Gitee fallback when GitHub is unreachable in CN networks
	RegistryFallbackURLCN = "https://gitee.com/zhaoxinyi02/ClawPanel-Plugins/raw/main/registry.json"
)

var officialFeishuPluginIDs = []string{"openclaw-lark", "feishu-openclaw-plugin"}

var preferredPluginPackageSpecs = map[string]string{
	"qqbot":         "@sliverp/qqbot@latest",
	"feishu":        "@openclaw/feishu@latest",
	"openclaw-lark": "@larksuite/openclaw-lark@latest",
	"dingtalk":      "@largezhou/ddingtalk@latest",
	"wecom":         "@wecom/wecom-openclaw-plugin@latest",
	"wecom-app":     "@openclaw-china/wecom-app@latest",
}

var builtInOfficialChannelPlugins = map[string]RegistryPlugin{
	"qqbot": {
		PluginMeta: PluginMeta{
			ID:          "qqbot",
			Name:        "QQ 官方机器人通道",
			Version:     "latest",
			Author:      "ClawPanel Team",
			Description: "QQ 官方机器人 API 通道插件",
			Category:    "channel",
		},
		NpmPackage: "@sliverp/qqbot",
	},
	"feishu": {
		PluginMeta: PluginMeta{
			ID:          "feishu",
			Name:        "飞书 / Lark",
			Version:     "latest",
			Author:      "ClawPanel Team",
			Description: "飞书 / Lark 通道插件",
			Category:    "channel",
		},
		NpmPackage: "@openclaw/feishu",
	},
	"openclaw-lark": {
		PluginMeta: PluginMeta{
			ID:          "openclaw-lark",
			Name:        "飞书 / Lark（飞书官方版）",
			Version:     "latest",
			Author:      "Feishu Team",
			Description: "飞书官方 OpenClaw 插件",
			Category:    "channel",
		},
		NpmPackage: "@larksuite/openclaw-lark",
	},
	"dingtalk": {
		PluginMeta: PluginMeta{
			ID:          "dingtalk",
			Name:        "钉钉通道插件",
			Version:     "latest",
			Author:      "BytePioneer-AI",
			Description: "钉钉 (DingTalk) 通道插件",
			Category:    "channel",
		},
		NpmPackage: "@largezhou/ddingtalk",
	},
	"wecom": {
		PluginMeta: PluginMeta{
			ID:          "wecom",
			Name:        "企业微信（智能机器人）",
			Version:     "latest",
			Author:      "BytePioneer-AI",
			Description: "企业微信 (WeCom) 智能机器人通道插件",
			Category:    "channel",
		},
		NpmPackage: "@wecom/wecom-openclaw-plugin",
	},
	"wecom-app": {
		PluginMeta: PluginMeta{
			ID:          "wecom-app",
			Name:        "企业微信（自建应用）",
			Version:     "latest",
			Author:      "BytePioneer-AI",
			Description: "企业微信自建应用通道插件",
			Category:    "channel",
		},
		NpmPackage: "@openclaw-china/wecom-app",
	},
}

var (
	gitLookPath    = exec.LookPath
	gitCloneRunner = runGitClone
)

// PluginMeta represents a plugin's metadata (plugin.json)
type PluginMeta struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Author       string            `json:"author"`
	Description  string            `json:"description"`
	Homepage     string            `json:"homepage,omitempty"`
	Repository   string            `json:"repository,omitempty"`
	License      string            `json:"license,omitempty"`
	Category     string            `json:"category,omitempty"` // basic, ai, message, fun, tool
	Tags         []string          `json:"tags,omitempty"`
	Icon         string            `json:"icon,omitempty"`
	MinOpenClaw  string            `json:"minOpenClaw,omitempty"`
	MinPanel     string            `json:"minPanel,omitempty"`
	EntryPoint   string            `json:"entryPoint,omitempty"`   // main script file
	ConfigSchema json.RawMessage   `json:"configSchema,omitempty"` // JSON Schema for config
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Permissions  []string          `json:"permissions,omitempty"`
}

// InstalledPlugin represents a plugin installed on disk
type InstalledPlugin struct {
	PluginMeta
	Enabled     bool                   `json:"enabled"`
	InstalledAt string                 `json:"installedAt"`
	UpdatedAt   string                 `json:"updatedAt,omitempty"`
	Source      string                 `json:"source"` // registry, local, github
	Dir         string                 `json:"dir"`
	Config      map[string]interface{} `json:"config,omitempty"`
	LogLines    []string               `json:"logLines,omitempty"`

	// NeedManifestRepair is true when openclaw.plugin.json is missing or stale.
	// Set by scan, cleared after reconcile.
	NeedManifestRepair bool `json:"-"`
	// NeedConfigSync is true when the plugin needs to be registered in openclaw.json.
	// Set by scan for newly-discovered plugins, cleared after reconcile.
	NeedConfigSync bool `json:"-"`
}

// RegistryPlugin represents a plugin in the registry
type RegistryPlugin struct {
	PluginMeta
	Downloads     int    `json:"downloads,omitempty"`
	Stars         int    `json:"stars,omitempty"`
	DownloadURL   string `json:"downloadUrl,omitempty"`
	GitURL        string `json:"gitUrl,omitempty"`
	InstallSubDir string `json:"installSubDir,omitempty"` // subdirectory within git repo to install
	NpmPackage    string `json:"npmPackage,omitempty"`    // npm package name e.g. @openclaw/feishu
	Screenshot    string `json:"screenshot,omitempty"`
	Readme        string `json:"readme,omitempty"`
}

// Registry represents the plugin registry
type Registry struct {
	Version   string           `json:"version"`
	UpdatedAt string           `json:"updatedAt"`
	Plugins   []RegistryPlugin `json:"plugins"`
}

// Manager handles plugin lifecycle
type Manager struct {
	cfg        *config.Config
	plugins    map[string]*InstalledPlugin
	registry   *Registry
	mu         sync.RWMutex
	pluginsDir string
	configFile string
}

// NewManager creates a plugin manager
func NewManager(cfg *config.Config) *Manager {
	pluginsDir := filepath.Join(cfg.OpenClawDir, "extensions")
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		os.MkdirAll(pluginsDir, 0755)
	}
	m := &Manager{
		cfg:        cfg,
		plugins:    make(map[string]*InstalledPlugin),
		pluginsDir: pluginsDir,
		configFile: filepath.Join(cfg.DataDir, "plugins.json"),
	}
	m.loadPluginsState()
	m.scanInstalledPlugins()  // read-only: discover plugins
	m.reconcilePluginStates() // write phase: sync manifests & config
	return m
}

// GetPluginsDir returns the plugins directory
func (m *Manager) GetPluginsDir() string {
	return m.pluginsDir
}

// ListInstalled returns all installed plugins with enabled state reconciled against
// openclaw.json so that CLI-side enable/disable toggles are reflected immediately.
func (m *Manager) ListInstalled() []*InstalledPlugin {
	m.mu.RLock()
	result := make([]*InstalledPlugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, cloneInstalledPlugin(p))
	}
	m.mu.RUnlock()

	// Reconcile: read openclaw.json entries and override enabled state so the panel
	// reflects any changes made via the OpenClaw CLI (e.g. `openclaw plugins disable`).
	ocConfig, err := m.cfg.ReadOpenClawJSON()
	if err == nil && ocConfig != nil {
		if pl, ok := ocConfig["plugins"].(map[string]interface{}); ok {
			if entries, ok := pl["entries"].(map[string]interface{}); ok {
				for _, p := range result {
					if entry, ok := entries[p.ID].(map[string]interface{}); ok {
						if enabled, ok := entry["enabled"].(bool); ok {
							p.Enabled = enabled
						}
					}
				}
			}
		}
	}
	return result
}

// GetPlugin returns a specific installed plugin
func (m *Manager) GetPlugin(id string) *InstalledPlugin {
	m.mu.RLock()
	p := m.plugins[id]
	m.mu.RUnlock()
	return cloneInstalledPlugin(p)
}

// FetchRegistry fetches the plugin registry from server
func (m *Manager) FetchRegistry() (*Registry, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	bundled := m.loadBundledRegistry()
	return m.fetchRegistryFromURLs(client, registryFetchURLs(), bundled)
}

func (m *Manager) fetchRegistryFromURLs(client *http.Client, urls []string, bundled *Registry) (*Registry, error) {
	// Try mirror first (faster in China), then GitHub, then Gitee fallback
	var lastErr error
	for _, url := range urls {
		reg, err := func() (*Registry, error) {
			resp, err := client.Get(url)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
			}
			var reg Registry
			if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
				return nil, fmt.Errorf("parse registry: %v", err)
			}
			return &reg, nil
		}()
		if err != nil {
			lastErr = err
			continue
		}
		merged := mergeRegistries(*reg, bundled)
		m.mu.Lock()
		m.registry = &merged
		m.mu.Unlock()
		// Cache to disk
		m.cacheRegistry(&merged)
		return &merged, nil
	}

	// Try cached registry before bundled fallback. A previous successful fetch can be
	// newer than the registry embedded in the current binary.
	if cached := m.loadCachedRegistry(); cached != nil {
		merged := mergeRegistries(*cached, bundled)
		m.mu.Lock()
		m.registry = &merged
		m.mu.Unlock()
		return &merged, nil
	}

	if bundled != nil {
		m.mu.Lock()
		m.registry = bundled
		m.mu.Unlock()
		return bundled, nil
	}

	return nil, fmt.Errorf("获取插件仓库失败: %v", lastErr)
}

// GetRegistry returns the cached registry or fetches it
func (m *Manager) GetRegistry() *Registry {
	m.mu.RLock()
	reg := m.registry
	m.mu.RUnlock()
	if reg != nil {
		return reg
	}
	if cached := m.loadCachedRegistry(); cached != nil {
		bundled := m.loadBundledRegistry()
		merged := mergeRegistries(*cached, bundled)
		m.mu.Lock()
		m.registry = &merged
		m.mu.Unlock()
		return &merged
	}
	if bundled := m.loadBundledRegistry(); bundled != nil {
		m.mu.Lock()
		m.registry = bundled
		m.mu.Unlock()
		return bundled
	}
	return &Registry{Plugins: []RegistryPlugin{}}
}

type pluginInstallStrategy struct {
	kind   string
	target string
}

func resolvePluginInstallStrategy(regPlugin *RegistryPlugin, source string) pluginInstallStrategy {
	source = strings.TrimSpace(source)
	if source != "" {
		if strings.HasPrefix(source, "@") || (!strings.Contains(source, "/") && !strings.HasSuffix(source, ".git") && source != "") {
			return pluginInstallStrategy{kind: "npm", target: source}
		}
		return pluginInstallStrategy{kind: "download", target: source}
	}
	if regPlugin == nil {
		return pluginInstallStrategy{}
	}
	if preferred := preferredPluginPackageSpec(regPlugin); preferred != "" {
		return pluginInstallStrategy{kind: "npm", target: preferred}
	}
	if preferred := preferredPluginDownloadURL(regPlugin); preferred != "" {
		return pluginInstallStrategy{kind: "download", target: preferred}
	}
	if regPlugin.NpmPackage != "" {
		return pluginInstallStrategy{kind: "npm", target: regPlugin.NpmPackage}
	}
	if regPlugin.DownloadURL != "" {
		return pluginInstallStrategy{kind: "download", target: regPlugin.DownloadURL}
	}
	if regPlugin.GitURL != "" {
		return pluginInstallStrategy{kind: "download", target: regPlugin.GitURL}
	}
	return pluginInstallStrategy{}
}

func preferredPluginPackageSpec(regPlugin *RegistryPlugin) string {
	if regPlugin == nil {
		return ""
	}
	return preferredPluginPackageSpecs[regPlugin.ID]
}

func preferredPluginDownloadURL(regPlugin *RegistryPlugin) string {
	if regPlugin == nil {
		return ""
	}
	switch regPlugin.ID {
	case "qqbot":
		return "https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel-Plugins/main/official/qqbot/qqbot-1.2.2.tgz"
	default:
		return ""
	}
}

func builtInOfficialChannelPlugin(pluginID string) *RegistryPlugin {
	plugin, ok := builtInOfficialChannelPlugins[pluginID]
	if !ok {
		return nil
	}
	cp := plugin
	if preferred := preferredPluginPackageSpec(&cp); preferred != "" {
		cp.NpmPackage = normalizeNpmPackageName(preferred)
	}
	return &cp
}

// Install installs a plugin from registry or URL
func (m *Manager) Install(pluginID string, source string) error {
	return m.InstallWithProgress(pluginID, source, nil)
}

func (m *Manager) InstallWithProgress(pluginID string, source string, logf func(string)) error {
	// Find plugin in registry
	reg := m.GetRegistry()
	var regPlugin *RegistryPlugin
	if strings.TrimSpace(source) == "" {
		if fallback := builtInOfficialChannelPlugin(pluginID); fallback != nil {
			regPlugin = fallback
			if logf != nil {
				logf(fmt.Sprintf("📦 %s 使用内置官方安装信息，直接按官方包安装", pluginID))
			}
		}
	}
	for i := range reg.Plugins {
		if reg.Plugins[i].ID == pluginID {
			regPlugin = &reg.Plugins[i]
			break
		}
	}

	if regPlugin == nil && source == "" {
		if logf != nil {
			logf("🔄 当前缓存仓库未命中插件，正在刷新插件仓库...")
		}
		if fetched, err := m.FetchRegistry(); err == nil && fetched != nil {
			reg = fetched
			for i := range reg.Plugins {
				if reg.Plugins[i].ID == pluginID {
					regPlugin = &reg.Plugins[i]
					break
				}
			}
		} else if logf != nil && err != nil {
			logf(fmt.Sprintf("⚠️ 刷新插件仓库失败，继续使用本地仓库信息: %v", err))
		}
	}

	if regPlugin == nil {
		if fallback := builtInOfficialChannelPlugin(pluginID); fallback != nil {
			regPlugin = fallback
			if logf != nil {
				logf(fmt.Sprintf("📦 在线仓库未命中 %s，使用内置官方插件安装信息直接安装", pluginID))
			}
		}
	}

	if regPlugin == nil && source == "" {
		return fmt.Errorf("插件 %s 不在仓库中，请提供安装源", pluginID)
	}

	strategy := resolvePluginInstallStrategy(regPlugin, source)
	if strategy.kind == "npm" {
		npmSpec := strategy.target
		npmPkg := normalizeNpmPackageName(npmSpec)
		if logf != nil {
			logf(fmt.Sprintf("📦 优先尝试 OpenClaw 官方命令安装插件: %s", npmSpec))
		}
		if err := m.installViaOpenClawCLI(npmSpec, logf); err != nil {
			if logf != nil {
				logf(fmt.Sprintf("⚠️ 官方命令安装失败，回退到面板安装逻辑: %v", err))
			}
			if err := m.installFromNpm(npmSpec); err != nil {
				return fmt.Errorf("官方命令安装失败，且 npm 回退安装失败: %v", err)
			}
		}
		m.scanInstalledPlugins()
		// Find where npm installed it
		npmRoot := ""
		if out, err := exec.Command("npm", "root", "-g").Output(); err == nil {
			npmRoot = strings.TrimSpace(string(out))
		}
		pkgName := npmPkg
		if idx := strings.LastIndex(pkgName, "/"); idx >= 0 {
			pkgName = pkgName[idx+1:]
		}
		installedDir := ""
		if npmRoot != "" {
			// For scoped packages like @openclaw/feishu, dir is @openclaw/feishu
			installedDir = filepath.Join(npmRoot, npmPkg)
			if _, err := os.Stat(installedDir); err != nil {
				installedDir = filepath.Join(npmRoot, pkgName)
			}
		}
		meta := &PluginMeta{ID: pluginID, Name: pluginID}
		if regPlugin != nil {
			meta = &regPlugin.PluginMeta
		}
		if installedDir == "" {
			if extDir, ok := m.findInstalledPluginDir(pluginID); ok {
				installedDir = extDir
			} else {
				installedDir = npmPkg
			}
		}
		installed := &InstalledPlugin{
			PluginMeta:  *meta,
			Enabled:     true,
			InstalledAt: time.Now().Format(time.RFC3339),
			Source:      "npm",
			Dir:         installedDir,
		}
		m.mu.Lock()
		m.plugins[meta.ID] = installed
		m.mu.Unlock()
		m.savePluginsState()
		if err := m.syncOpenClawPluginState(meta.ID, installedDir, installed.Enabled, installed.Source, meta.Version); err != nil {
			return err
		}
		return nil
	}

	// Determine download URL (git/archive)
	downloadURL := strategy.target

	if downloadURL == "" {
		return fmt.Errorf("无法确定插件 %s 的安装方式，请提供 npm 包名或下载地址", pluginID)
	}

	pluginDir := filepath.Join(m.pluginsDir, pluginID)

	// Check if already installed
	if _, err := os.Stat(pluginDir); err == nil {
		return fmt.Errorf("插件 %s 已安装，请先卸载或使用更新功能", pluginID)
	}

	// Determine installSubDir from registry
	installSubDir := ""
	if regPlugin != nil {
		installSubDir = regPlugin.InstallSubDir
	}

	// Install based on source type
	if strings.HasSuffix(downloadURL, ".git") || strings.Contains(downloadURL, "github.com") || strings.Contains(downloadURL, "gitee.com") {
		if installSubDir != "" {
			// Clone full repo to temp dir, then copy subdirectory
			tmpDir, err := os.MkdirTemp("", "clawpanel-plugin-*")
			if err != nil {
				return fmt.Errorf("创建临时目录失败: %v", err)
			}
			defer os.RemoveAll(tmpDir)
			if err := m.installFromGit(downloadURL, tmpDir); err != nil {
				return fmt.Errorf("Git 安装失败: %v", err)
			}
			subPath, err := resolveInstallSubDir(tmpDir, installSubDir)
			if err != nil {
				return err
			}
			if _, err := os.Stat(subPath); err != nil {
				return fmt.Errorf("子目录 %s 在仓库中不存在", installSubDir)
			}
			if err := copyDir(subPath, pluginDir); err != nil {
				os.RemoveAll(pluginDir)
				return fmt.Errorf("复制插件目录失败: %v", err)
			}
		} else {
			if err := m.installFromGit(downloadURL, pluginDir); err != nil {
				os.RemoveAll(pluginDir)
				return fmt.Errorf("Git 安装失败: %v", err)
			}
		}
	} else if strings.HasSuffix(downloadURL, ".zip") || strings.HasSuffix(downloadURL, ".tar.gz") || strings.HasSuffix(downloadURL, ".tgz") {
		// Download archive
		if err := m.installFromArchive(downloadURL, pluginDir); err != nil {
			os.RemoveAll(pluginDir)
			return fmt.Errorf("下载安装失败: %v", err)
		}
	} else {
		// Try git clone as fallback
		if err := m.installFromGit(downloadURL, pluginDir); err != nil {
			os.RemoveAll(pluginDir)
			return fmt.Errorf("安装失败: %v", err)
		}
	}

	// Read plugin metadata
	meta, err := m.readPluginMeta(pluginDir)
	if err != nil {
		// If no plugin.json, create a minimal one
		meta = &PluginMeta{
			ID:   pluginID,
			Name: pluginID,
		}
		if regPlugin != nil {
			meta = &regPlugin.PluginMeta
		}
	}
	if err := m.ensureOpenClawPluginManifest(pluginDir, meta); err != nil {
		return fmt.Errorf("生成 openclaw.plugin.json 失败: %v", err)
	}

	// Install npm dependencies if package.json exists
	if _, err := os.Stat(filepath.Join(pluginDir, "package.json")); err == nil {
		cmd := exec.Command("npm", "install", "--production", "--registry=https://registry.npmmirror.com")
		cmd.Dir = pluginDir
		cmd.Run()
	}

	// Register installed plugin
	installed := &InstalledPlugin{
		PluginMeta:  *meta,
		Enabled:     true,
		InstalledAt: time.Now().Format(time.RFC3339),
		Source:      "registry",
		Dir:         pluginDir,
	}
	if source != "" {
		installed.Source = "custom"
	}

	m.mu.Lock()
	m.plugins[meta.ID] = installed
	m.mu.Unlock()
	m.savePluginsState()
	if err := m.syncOpenClawPluginState(meta.ID, pluginDir, installed.Enabled, installed.Source, meta.Version); err != nil {
		return err
	}

	return nil
}

func (m *Manager) findInstalledPluginDir(pluginID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.plugins[pluginID]; ok && strings.TrimSpace(p.Dir) != "" {
		return p.Dir, true
	}
	return "", false
}

func (m *Manager) installViaOpenClawCLI(spec string, logf func(string)) error {
	var cmd *exec.Cmd
	var err error
	if m.cfg != nil && m.cfg.IsLiteEdition() {
		cmd, err = m.cfg.OpenClawCommand("plugins", "install", spec)
		if err != nil {
			return err
		}
	} else {
		bin := config.DetectOpenClawBinaryPath()
		if strings.TrimSpace(bin) == "" {
			return fmt.Errorf("未找到 openclaw 可执行文件")
		}
		cmd = exec.Command(bin, "plugins", "install", spec)
	}
	cmd.Env = config.BuildExecEnv()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	if logf != nil {
		if err := scanCommandOutput(stdout, logf); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
	} else {
		_, _ = io.Copy(io.Discard, stdout)
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

// InstallLocal installs a plugin from a local directory
func (m *Manager) InstallLocal(srcDir string) error {
	meta, err := m.readPluginMeta(srcDir)
	if err != nil {
		return fmt.Errorf("读取插件信息失败: %v", err)
	}

	pluginDir := filepath.Join(m.pluginsDir, meta.ID)
	if _, err := os.Stat(pluginDir); err == nil {
		return fmt.Errorf("插件 %s 已安装", meta.ID)
	}

	// Copy directory
	if err := copyDir(srcDir, pluginDir); err != nil {
		return fmt.Errorf("复制插件失败: %v", err)
	}

	installed := &InstalledPlugin{
		PluginMeta:  *meta,
		Enabled:     true,
		InstalledAt: time.Now().Format(time.RFC3339),
		Source:      "local",
		Dir:         pluginDir,
	}

	m.mu.Lock()
	m.plugins[meta.ID] = installed
	m.mu.Unlock()
	m.savePluginsState()

	return nil
}

// Uninstall removes a plugin
func (m *Manager) Uninstall(pluginID string, cleanupConfig bool) error {
	return m.UninstallWithProgress(pluginID, cleanupConfig, nil)
}

func (m *Manager) UninstallWithProgress(pluginID string, cleanupConfig bool, logf func(string)) error {
	m.mu.RLock()
	p, ok := m.plugins[pluginID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("插件 %s 未安装", pluginID)
	}
	cp := *p
	m.mu.RUnlock()

	if logf != nil {
		logf("📦 开始卸载插件 " + pluginID)
	}
	if err := m.removeOpenClawPluginState(pluginID, cleanupConfig); err != nil {
		return err
	}
	if err := m.uninstallViaOpenClawCLI(pluginID, logf); err != nil {
		if logf != nil {
			logf(fmt.Sprintf("⚠️ 官方命令卸载失败，回退到面板卸载逻辑: %v", err))
		}
	}

	// Remove plugin directory
	if cp.Dir != "" {
		_ = os.RemoveAll(cp.Dir)
	}

	m.mu.Lock()
	delete(m.plugins, pluginID)
	m.mu.Unlock()
	m.savePluginsState()
	return nil
}

func (m *Manager) uninstallViaOpenClawCLI(pluginID string, logf func(string)) error {
	var cmd *exec.Cmd
	var err error
	if m.cfg != nil && m.cfg.IsLiteEdition() {
		cmd, err = m.cfg.OpenClawCommand("plugins", "uninstall", pluginID)
		if err != nil {
			return err
		}
	} else {
		bin := config.DetectOpenClawBinaryPath()
		if strings.TrimSpace(bin) == "" {
			return fmt.Errorf("未找到 openclaw 可执行文件")
		}
		cmd = exec.Command(bin, "plugins", "uninstall", pluginID)
	}
	cmd.Env = config.BuildExecEnv()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	if logf != nil {
		if err := scanCommandOutput(stdout, logf); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
	} else {
		_, _ = io.Copy(io.Discard, stdout)
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

// Enable enables a plugin
func (m *Manager) Enable(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[pluginID]
	if !ok {
		return fmt.Errorf("插件 %s 未安装", pluginID)
	}
	p.Enabled = true
	m.savePluginsStateUnlocked()
	if err := m.syncOpenClawPluginState(p.ID, p.Dir, true, p.Source, p.Version); err != nil {
		return err
	}
	return nil
}

// Disable disables a plugin
func (m *Manager) Disable(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[pluginID]
	if !ok {
		return fmt.Errorf("插件 %s 未安装", pluginID)
	}
	p.Enabled = false
	m.savePluginsStateUnlocked()
	if err := m.syncOpenClawPluginState(p.ID, p.Dir, false, p.Source, p.Version); err != nil {
		return err
	}
	return nil
}

// UpdateConfig updates a plugin's configuration
func (m *Manager) UpdateConfig(pluginID string, cfg map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[pluginID]
	if !ok {
		return fmt.Errorf("插件 %s 未安装", pluginID)
	}
	p.Config = cfg

	// Also write config.json to plugin directory
	configPath := filepath.Join(p.Dir, "config.json")
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath, data, 0644)

	m.savePluginsStateUnlocked()
	return nil
}

// GetConfig returns a plugin's configuration
func (m *Manager) GetConfig(pluginID string) (map[string]interface{}, json.RawMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[pluginID]
	if !ok {
		return nil, nil, fmt.Errorf("插件 %s 未安装", pluginID)
	}

	// Try to read config from plugin dir
	cfg := p.Config
	if cfg == nil {
		configPath := filepath.Join(p.Dir, "config.json")
		if data, err := os.ReadFile(configPath); err == nil {
			json.Unmarshal(data, &cfg)
		}
	}
	if cfg == nil {
		cfg = map[string]interface{}{}
	}

	return cloneGenericMap(cfg), append(json.RawMessage(nil), p.ConfigSchema...), nil
}

// GetPluginLogs returns recent log lines for a plugin
func (m *Manager) GetPluginLogs(pluginID string) ([]string, error) {
	m.mu.RLock()
	p, ok := m.plugins[pluginID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("插件 %s 未安装", pluginID)
	}

	// Read log file if exists
	logPath := filepath.Join(p.Dir, "plugin.log")
	if data, err := os.ReadFile(logPath); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) > 200 {
			lines = lines[len(lines)-200:]
		}
		return lines, nil
	}

	return p.LogLines, nil
}

// Update updates a plugin to the latest version
func (m *Manager) Update(pluginID string) error {
	m.mu.RLock()
	p, ok := m.plugins[pluginID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("插件 %s 未安装", pluginID)
	}

	if p.Dir == "" {
		return fmt.Errorf("插件目录未知")
	}

	// If it's a git repo, do git pull
	gitDir := filepath.Join(p.Dir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		cmd := exec.Command("git", "pull", "--rebase")
		cmd.Dir = p.Dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git pull 失败: %s %v", string(out), err)
		}

		// Re-read metadata
		if meta, err := m.readPluginMeta(p.Dir); err == nil {
			m.mu.Lock()
			p.PluginMeta = *meta
			p.UpdatedAt = time.Now().Format(time.RFC3339)
			m.mu.Unlock()
			m.savePluginsState()
		}

		// Re-install npm deps
		if _, err := os.Stat(filepath.Join(p.Dir, "package.json")); err == nil {
			cmd := exec.Command("npm", "install", "--production", "--registry=https://registry.npmmirror.com")
			cmd.Dir = p.Dir
			cmd.Run()
		}

		return nil
	}

	// Otherwise, uninstall and reinstall from registry
	source := strings.TrimSpace(strings.ToLower(p.Source))
	if source != "registry" && source != "npm" {
		return fmt.Errorf("非仓库插件无法自动更新，请手动卸载重装")
	}
	if err := m.Uninstall(pluginID, false); err != nil {
		return err
	}
	return m.Install(pluginID, "")
}

// CheckConflicts checks for potential conflicts before installing
func (m *Manager) CheckConflicts(pluginID string) []string {
	var conflicts []string
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.plugins[pluginID]; exists {
		conflicts = append(conflicts, fmt.Sprintf("插件 %s 已安装", pluginID))
	}

	return conflicts
}

// --- Internal methods ---

// scanInstalledPlugins discovers plugins on disk and updates in-memory state.
// It is intentionally read-only: no files are written to either the plugin
// directories or openclaw.json. Plugins that need manifest repair or config
// sync are flagged via NeedManifestRepair / NeedConfigSync for a subsequent
// reconcilePluginStates call.
func (m *Manager) scanInstalledPlugins() {
	m.scanPluginDir(m.pluginsDir, "local", !m.cfg.IsLiteEdition(), false)
	if m.cfg.IsLiteEdition() {
		m.scanLiteRuntimePlugins()
	}
	m.pruneMissingPlugins()
}

func (m *Manager) pruneMissingPlugins() {
	m.mu.Lock()
	defer m.mu.Unlock()

	changed := false
	for id, plugin := range m.plugins {
		if plugin == nil {
			delete(m.plugins, id)
			changed = true
			continue
		}
		dir := strings.TrimSpace(plugin.Dir)
		if dir == "" {
			delete(m.plugins, id)
			changed = true
			continue
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			delete(m.plugins, id)
			changed = true
		}
	}

	if changed {
		m.savePluginsStateUnlocked()
	}
}

func (m *Manager) scanLiteRuntimePlugins() {
	appDir := strings.TrimSpace(m.cfg.OpenClawApp)
	if appDir == "" {
		appDir = m.cfg.BundledOpenClawAppDir()
	}
	if appDir == "" {
		return
	}
	ocConfig, _ := m.cfg.ReadOpenClawJSON()
	channels, _ := ocConfig["channels"].(map[string]interface{})
	entries := map[string]interface{}{}
	if plugins, ok := ocConfig["plugins"].(map[string]interface{}); ok {
		if currentEntries, ok := plugins["entries"].(map[string]interface{}); ok && currentEntries != nil {
			entries = currentEntries
		}
	}
	for _, pluginID := range []string{"telegram", "feishu", "openclaw-lark", "feishu-openclaw-plugin", "qq", "qqbot", "dingtalk", "wecom", "wecom-app"} {
		pluginDir := filepath.Join(appDir, "extensions", pluginID)
		meta, err := m.readPluginMeta(pluginDir)
		if err != nil {
			continue
		}
		enabled := false
		hasChannelEnabled := false
		if ch, ok := channels[pluginID].(map[string]interface{}); ok {
			if v, ok := ch["enabled"].(bool); ok {
				enabled = v
				hasChannelEnabled = true
			}
		}
		if !hasChannelEnabled {
			if entry, ok := entries[meta.ID].(map[string]interface{}); ok {
				if v, ok := entry["enabled"].(bool); ok {
					enabled = v
				}
			}
		}
		source := "bundled"
		version := meta.Version
		m.mu.Lock()
		if existing, exists := m.plugins[meta.ID]; !exists {
			m.plugins[meta.ID] = &InstalledPlugin{
				PluginMeta:  *meta,
				Enabled:     enabled,
				InstalledAt: time.Now().Format(time.RFC3339),
				Source:      source,
				Dir:         pluginDir,
			}
		} else {
			existing.Dir = pluginDir
			existing.PluginMeta = *meta
			existing.Enabled = enabled
			source = existing.Source
		}
		m.mu.Unlock()
		if err := m.syncOpenClawPluginState(meta.ID, pluginDir, enabled, source, version); err != nil {
			continue
		}
	}
}

func (m *Manager) scanPluginDir(baseDir string, source string, defaultEnabled bool, skipExisting bool) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(baseDir, entry.Name())
		meta, err := m.readPluginMeta(pluginDir)
		if err != nil {
			continue
		}

		needManifest := false
		manifestPath := filepath.Join(pluginDir, "openclaw.plugin.json")
		if _, statErr := os.Stat(manifestPath); os.IsNotExist(statErr) {
			needManifest = true
		}

		enabled := defaultEnabled
		m.mu.Lock()
		if existing, exists := m.plugins[meta.ID]; !exists {
			m.plugins[meta.ID] = &InstalledPlugin{
				PluginMeta:         *meta,
				Enabled:            enabled,
				InstalledAt:        time.Now().Format(time.RFC3339),
				Source:             source,
				Dir:                pluginDir,
				NeedManifestRepair: needManifest,
				NeedConfigSync:     true,
			}
		} else {
			if skipExisting {
				existing.NeedManifestRepair = existing.NeedManifestRepair || needManifest
				enabled = existing.Enabled
				source = existing.Source
				m.mu.Unlock()
				continue
			}
			existing.Dir = pluginDir
			existing.PluginMeta = *meta
			existing.NeedManifestRepair = needManifest
			existing.NeedConfigSync = true
			enabled = existing.Enabled
			source = existing.Source
		}
		m.mu.Unlock()
	}
}

// reconcilePluginStates performs deferred writes for plugins flagged during
// scan. It generates missing openclaw.plugin.json manifests and syncs new
// plugin entries into openclaw.json. Explicit install/enable/disable flows
// handle their own writes independently.
func (m *Manager) reconcilePluginStates() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.plugins))
	for id := range m.plugins {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		m.mu.RLock()
		p := m.plugins[id]
		if p == nil {
			m.mu.RUnlock()
			continue
		}
		needManifest := p.NeedManifestRepair
		needSync := p.NeedConfigSync
		dir := p.Dir
		enabled := p.Enabled
		source := p.Source
		version := p.Version
		meta := p.PluginMeta
		m.mu.RUnlock()

		if needManifest {
			if err := m.ensureOpenClawPluginManifest(dir, &meta); err != nil {
				log.Printf("warn: could not write openclaw.plugin.json for %s: %v", id, err)
			} else {
				m.mu.Lock()
				if pp := m.plugins[id]; pp != nil {
					pp.NeedManifestRepair = false
				}
				m.mu.Unlock()
			}
		}
		if needSync {
			effectiveEnabled, err := m.syncDiscoveredOpenClawPluginState(id, dir, enabled, source, version)
			if err != nil {
				continue
			}
			m.mu.Lock()
			if pp := m.plugins[id]; pp != nil {
				pp.Enabled = effectiveEnabled
				pp.NeedConfigSync = false
			}
			m.mu.Unlock()
		}
	}
}

func (m *Manager) readPluginMeta(dir string) (*PluginMeta, error) {
	// Preference order: plugin.json → openclaw.plugin.json (official manifest) → package.json.
	candidates := []string{
		filepath.Join(dir, "plugin.json"),
		filepath.Join(dir, "openclaw.plugin.json"),
		filepath.Join(dir, "package.json"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var meta PluginMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		if meta.ID == "" {
			meta.ID = filepath.Base(dir)
		}
		if meta.Name == "" {
			meta.Name = meta.ID
		}
		return &meta, nil
	}
	return nil, fmt.Errorf("no plugin.json, openclaw.plugin.json or package.json found in %s", dir)
}

func (m *Manager) ensureOpenClawPluginManifest(dir string, meta *PluginMeta) error {
	manifestPath := filepath.Join(dir, "openclaw.plugin.json")
	if _, err := os.Stat(manifestPath); err == nil {
		return nil
	}
	if meta == nil || strings.TrimSpace(meta.ID) == "" {
		return nil
	}
	manifest := map[string]interface{}{
		"id": strings.TrimSpace(meta.ID),
	}
	if name := strings.TrimSpace(meta.Name); name != "" {
		manifest["name"] = name
	}
	if description := strings.TrimSpace(meta.Description); description != "" {
		manifest["description"] = description
	}
	if version := strings.TrimSpace(meta.Version); version != "" {
		manifest["version"] = version
	}
	if len(meta.ConfigSchema) > 0 {
		var schema interface{}
		if err := json.Unmarshal(meta.ConfigSchema, &schema); err == nil && schema != nil {
			manifest["configSchema"] = schema
		}
	}
	if _, ok := manifest["configSchema"]; !ok {
		manifest["configSchema"] = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, raw, 0644)
}

func (m *Manager) loadPluginsState() {
	data, err := os.ReadFile(m.configFile)
	if err != nil {
		return
	}
	var plugins map[string]*InstalledPlugin
	if json.Unmarshal(data, &plugins) == nil {
		m.plugins = plugins
	}
}

func (m *Manager) savePluginsState() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.savePluginsStateUnlocked()
}

func (m *Manager) savePluginsStateUnlocked() {
	data, _ := json.MarshalIndent(m.plugins, "", "  ")
	os.WriteFile(m.configFile, data, 0644)
}

func (m *Manager) cacheRegistry(reg *Registry) {
	data, _ := json.MarshalIndent(reg, "", "  ")
	os.WriteFile(filepath.Join(m.cfg.DataDir, "plugin-registry-cache.json"), data, 0644)
}

func (m *Manager) loadCachedRegistry() *Registry {
	data, err := os.ReadFile(filepath.Join(m.cfg.DataDir, "plugin-registry-cache.json"))
	if err != nil {
		return nil
	}
	var reg Registry
	if json.Unmarshal(data, &reg) == nil {
		return &reg
	}
	return nil
}

func (m *Manager) loadBundledRegistry() *Registry {
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	registryPath := filepath.Join(filepath.Dir(filepath.Dir(exePath)), "plugins", "registry.json")
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return nil
	}
	var reg Registry
	if json.Unmarshal(data, &reg) == nil {
		return &reg
	}
	return nil
}

func mergeRegistries(primary Registry, bundled *Registry) Registry {
	if bundled == nil {
		return primary
	}
	merged := primary
	index := make(map[string]struct{}, len(merged.Plugins))
	for _, plugin := range merged.Plugins {
		index[plugin.ID] = struct{}{}
	}
	for _, plugin := range bundled.Plugins {
		if _, ok := index[plugin.ID]; !ok {
			merged.Plugins = append(merged.Plugins, plugin)
			index[plugin.ID] = struct{}{}
		}
	}
	if merged.Version == "" && bundled.Version != "" {
		merged.Version = bundled.Version
	}
	if merged.UpdatedAt == "" && bundled.UpdatedAt != "" {
		merged.UpdatedAt = bundled.UpdatedAt
	}
	return merged
}

func (m *Manager) syncOpenClawPluginState(pluginID, installPath string, enabled bool, source string, version string) error {
	_, err := m.writeOpenClawPluginState(pluginID, installPath, enabled, source, version, false)
	return err
}

func (m *Manager) syncDiscoveredOpenClawPluginState(pluginID, installPath string, enabled bool, source string, version string) (bool, error) {
	return m.writeOpenClawPluginState(pluginID, installPath, enabled, source, version, true)
}

func (m *Manager) writeOpenClawPluginState(pluginID, installPath string, enabled bool, source string, version string, preserveExistingEnabled bool) (bool, error) {
	ocConfig, err := m.cfg.ReadOpenClawJSON()
	if err != nil || ocConfig == nil {
		ocConfig = map[string]interface{}{}
	}
	pl, _ := ocConfig["plugins"].(map[string]interface{})
	if pl == nil {
		pl = map[string]interface{}{}
		ocConfig["plugins"] = pl
	}
	ent, _ := pl["entries"].(map[string]interface{})
	if ent == nil {
		ent = map[string]interface{}{}
		pl["entries"] = ent
	}
	entry, _ := ent[pluginID].(map[string]interface{})
	if entry == nil {
		entry = map[string]interface{}{}
		ent[pluginID] = entry
	}
	effectiveEnabled := enabled
	if preserveExistingEnabled {
		if existingEnabled, ok := entry["enabled"].(bool); ok {
			effectiveEnabled = existingEnabled
		} else {
			entry["enabled"] = enabled
		}
	} else {
		entry["enabled"] = enabled
	}

	ins, _ := pl["installs"].(map[string]interface{})
	if ins == nil {
		ins = map[string]interface{}{}
		pl["installs"] = ins
	}
	item, _ := ins[pluginID].(map[string]interface{})
	if item == nil {
		item = map[string]interface{}{}
		ins[pluginID] = item
	}
	if installPath != "" {
		item["installPath"] = installPath
	}
	if normalized := normalizeOpenClawInstallSource(source); normalized != "" {
		item["source"] = normalized
	}
	if version != "" {
		item["version"] = version
	}
	if _, ok := item["installedAt"]; !ok {
		item["installedAt"] = time.Now().UTC().Format(time.RFC3339)
	}
	return effectiveEnabled, m.cfg.WriteOpenClawJSON(ocConfig)
}

func normalizeOpenClawInstallSource(source string) string {
	switch strings.TrimSpace(strings.ToLower(source)) {
	case "npm":
		return "npm"
	case "archive":
		return "archive"
	case "path", "local", "registry", "custom", "github", "git":
		return "path"
	default:
		return "path"
	}
}

func registryFetchURLs() []string {
	urls := []string{RegistryURL, RegistryFallbackURLCN}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("CLAWPANEL_ALLOW_INSECURE_PLUGIN_MIRROR")), "true") {
		urls = append([]string{RegistryMirrorURL}, urls...)
	}
	return urls
}

func scanCommandOutput(stdout io.Reader, logf func(string)) error {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			logf(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取命令输出失败: %w", err)
	}
	return nil
}

func resolveInstallSubDir(rootDir, installSubDir string) (string, error) {
	subPath := filepath.Join(rootDir, filepath.FromSlash(installSubDir))
	rel, err := filepath.Rel(rootDir, subPath)
	if err != nil {
		return "", fmt.Errorf("解析插件子目录失败: %v", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("插件子目录 %s 越界，已拒绝安装", installSubDir)
	}
	return subPath, nil
}

func (m *Manager) removeOpenClawPluginState(pluginID string, cleanupConfig bool) error {
	ocConfig, err := m.cfg.ReadOpenClawJSON()
	if err != nil || ocConfig == nil {
		return err
	}
	pl, _ := ocConfig["plugins"].(map[string]interface{})
	if pl == nil {
		return nil
	}
	if ent, ok := pl["entries"].(map[string]interface{}); ok {
		delete(ent, pluginID)
	}
	if ins, ok := pl["installs"].(map[string]interface{}); ok {
		delete(ins, pluginID)
	}
	if cleanupConfig {
		cleanupChannelConfigForPlugin(ocConfig, pluginID)
	}
	return m.cfg.WriteOpenClawJSON(ocConfig)
}

func cleanupChannelConfigForPlugin(ocConfig map[string]interface{}, pluginID string) {
	if ocConfig == nil {
		return
	}
	channels, _ := ocConfig["channels"].(map[string]interface{})
	if channels == nil {
		return
	}
	pluginEntries, _ := ocConfig["plugins"].(map[string]interface{})
	entries, _ := pluginEntries["entries"].(map[string]interface{})
	installs, _ := pluginEntries["installs"].(map[string]interface{})
	stillInstalled := func(id string) bool {
		if entries != nil {
			if _, ok := entries[id]; ok {
				return true
			}
		}
		if installs != nil {
			if _, ok := installs[id]; ok {
				return true
			}
		}
		return false
	}
	switch pluginID {
	case "openclaw-lark", "feishu-openclaw-plugin":
		if !stillInstalled("feishu") {
			delete(channels, "feishu")
		}
	case "feishu":
		officialStillInstalled := false
		for _, id := range officialFeishuPluginIDs {
			if stillInstalled(id) {
				officialStillInstalled = true
				break
			}
		}
		if !officialStillInstalled {
			delete(channels, "feishu")
		}
	case "wecom", "wecom-app", "dingtalk", "qqbot", "discord", "mattermost", "line", "matrix", "twitch", "msteams":
		delete(channels, pluginID)
	}
}

func (m *Manager) installFromNpm(pkgSpec string) error {
	cmd := exec.Command("npm", "install", "-g", pkgSpec, "--registry=https://registry.npmmirror.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Retry without mirror
		cmd2 := exec.Command("npm", "install", "-g", pkgSpec)
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return fmt.Errorf("%s\n%s", string(out), string(out2))
		}
	}
	return nil
}

func (m *Manager) installFromGit(gitURL, dest string) error {
	for _, archiveURL := range repoArchiveURLs(gitURL) {
		if err := m.installFromArchive(archiveURL, dest); err == nil {
			return nil
		}
		_ = os.RemoveAll(dest)
	}

	if _, err := gitLookPath("git"); err != nil {
		return fmt.Errorf("未检测到 Git，请先安装 Git 后再重试")
	}

	var lastErr error
	var lastOut []byte
	for attempt := 1; attempt <= 3; attempt++ {
		out, err := gitCloneRunner(gitURL, dest)
		if err == nil {
			return nil
		}
		lastErr = err
		lastOut = out
		if attempt == 3 || !isTransientGitCloneError(string(out)) {
			break
		}
		_ = os.RemoveAll(dest)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	msg := strings.TrimSpace(string(lastOut))
	if msg == "" {
		msg = strings.TrimSpace(lastErr.Error())
	}
	if isTransientGitCloneError(msg) {
		return fmt.Errorf("网络异常，Git 拉取失败（已自动重试）: %s: %s", lastErr, msg)
	}
	return fmt.Errorf("%s: %s", lastErr, msg)
}

func repoArchiveURLs(repoURL string) []string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(repoURL, ".git"))
	switch {
	case strings.HasPrefix(trimmed, "https://github.com/"):
		repoPath := strings.TrimPrefix(trimmed, "https://github.com/")
		return []string{
			"https://codeload.github.com/" + repoPath + "/zip/refs/heads/main",
			"https://codeload.github.com/" + repoPath + "/zip/refs/heads/master",
		}
	case strings.HasPrefix(trimmed, "https://gitee.com/"):
		repoPath := strings.TrimPrefix(trimmed, "https://gitee.com/")
		return []string{
			"https://gitee.com/" + repoPath + "/repository/archive/main.zip",
			"https://gitee.com/" + repoPath + "/repository/archive/master.zip",
		}
	default:
		return nil
	}
}

func normalizeNpmPackageName(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	if strings.HasPrefix(spec, "@") {
		slash := strings.Index(spec, "/")
		if slash < 0 || slash == len(spec)-1 {
			return spec
		}
		rest := spec[slash+1:]
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			return spec[:slash+1+at]
		}
		return spec
	}
	if at := strings.LastIndex(spec, "@"); at > 0 {
		return spec[:at]
	}
	return spec
}

func runGitClone(gitURL, dest string) ([]byte, error) {
	args := []string{
		"-c", "http.version=HTTP/1.1",
		"-c", "http.lowSpeedLimit=1024",
		"-c", "http.lowSpeedTime=30",
		"clone", "--depth=1", "--single-branch", gitURL, dest,
	}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_LFS_SKIP_SMUDGE=1",
	)
	out, err := cmd.CombinedOutput()
	return out, err
}

func isTransientGitCloneError(msg string) bool {
	lower := strings.ToLower(msg)
	signatures := []string{
		"rpc failed",
		"gnutls recv error",
		"error decoding the received tls packet",
		"unexpected disconnect while reading sideband packet",
		"early eof",
		"invalid index-pack output",
		"connection reset by peer",
		"tls",
		"timeout",
	}
	for _, sig := range signatures {
		if strings.Contains(lower, sig) {
			return true
		}
	}
	return false
}

func (m *Manager) installFromArchive(url, dest string) error {
	return m.installFromArchiveWithRetry(url, dest)
}

func flattenExtractedArchiveRoot(dest string) error {
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	var dirs []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		}
	}
	if len(entries) != 1 || len(dirs) != 1 {
		return nil
	}

	rootDir := filepath.Join(dest, dirs[0].Name())
	children, err := os.ReadDir(rootDir)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := os.Rename(filepath.Join(rootDir, child.Name()), filepath.Join(dest, child.Name())); err != nil {
			return err
		}
	}
	return os.Remove(rootDir)
}

func (m *Manager) installFromArchiveWithRetry(url, dest string) error {
	client := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			ForceAttemptHTTP2:   false,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 15 * time.Second,
		},
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	tmpFile := filepath.Join(dest, "plugin-archive.tmp")
	defer os.Remove(tmpFile)

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		_ = os.Remove(tmpFile)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "ClawPanel-PluginInstaller")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		f, err := os.Create(tmpFile)
		if err != nil {
			resp.Body.Close()
			return err
		}
		_, copyErr := io.Copy(f, resp.Body)
		closeErr := f.Close()
		resp.Body.Close()
		if copyErr == nil && closeErr == nil {
			lastErr = nil
			break
		}
		if copyErr != nil {
			lastErr = copyErr
		} else {
			lastErr = closeErr
		}
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	if lastErr != nil {
		return fmt.Errorf("下载插件失败: %v", lastErr)
	}

	if strings.HasSuffix(url, ".zip") {
		cmd := exec.Command("unzip", "-o", tmpFile, "-d", dest)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("解压 zip 失败: %v: %s", err, strings.TrimSpace(string(out)))
		}
	} else if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		cmd := exec.Command("tar", "-xzf", tmpFile, "-C", dest)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("解压 tar.gz 失败: %v: %s", err, strings.TrimSpace(string(out)))
		}
	}

	if err := flattenExtractedArchiveRoot(dest); err != nil {
		return err
	}
	return nil
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("检测到不支持的符号链接: %s", path)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

func cloneInstalledPlugin(p *InstalledPlugin) *InstalledPlugin {
	if p == nil {
		return nil
	}
	cp := *p
	cp.Tags = append([]string(nil), p.Tags...)
	cp.Permissions = append([]string(nil), p.Permissions...)
	cp.LogLines = append([]string(nil), p.LogLines...)
	cp.ConfigSchema = append(json.RawMessage(nil), p.ConfigSchema...)
	if p.Dependencies != nil {
		cp.Dependencies = make(map[string]string, len(p.Dependencies))
		for key, value := range p.Dependencies {
			cp.Dependencies[key] = value
		}
	}
	cp.Config = cloneGenericMap(p.Config)
	return &cp
}

func cloneGenericMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	raw, err := json.Marshal(src)
	if err != nil {
		return nil
	}
	var dst map[string]interface{}
	if err := json.Unmarshal(raw, &dst); err != nil {
		return nil
	}
	return dst
}
