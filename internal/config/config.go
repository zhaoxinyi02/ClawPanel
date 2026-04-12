package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	json5 "github.com/titanous/json5"
	"github.com/zhaoxinyi02/ClawPanel/internal/buildinfo"
)

// Config 应用配置
type Config struct {
	Port         int    `json:"port"`
	DataDir      string `json:"dataDir"`
	OpenClawDir  string `json:"openClawDir"`
	OpenClawApp  string `json:"openClawApp"`
	OpenClawWork string `json:"openClawWork"`
	Edition      string `json:"edition,omitempty"`
	JWTSecret    string `json:"jwtSecret"`
	AdminToken   string `json:"adminToken"`
	Debug        bool   `json:"debug"`
	mu           sync.RWMutex
}

const (
	DefaultPort       = 19527
	ConfigFileName    = "clawpanel.json"
	DefaultJWTSecret  = "clawpanel-secret-change-me"
	DefaultAdminToken = "clawpanel"
)

// Load 加载配置，如果不存在则创建默认配置
func Load() (*Config, error) {
	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	cfgPath := filepath.Join(dataDir, ConfigFileName)
	cfg := &Config{
		Port:        DefaultPort,
		DataDir:     dataDir,
		OpenClawDir: getDefaultOpenClawDir(),
		Edition:     buildinfo.NormalizedEdition(),
		JWTSecret:   DefaultJWTSecret,
		AdminToken:  DefaultAdminToken,
		Debug:       false,
	}

	// 从环境变量覆盖
	if v := os.Getenv("CLAWPANEL_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Port)
	}
	if v := os.Getenv("CLAWPANEL_DATA"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("OPENCLAW_DIR"); v != "" {
		cfg.OpenClawDir = v
	}
	if v := os.Getenv("OPENCLAW_CONFIG"); v != "" {
		cfg.OpenClawDir = filepath.Dir(v)
	}
	if v := os.Getenv("OPENCLAW_APP"); v != "" {
		cfg.OpenClawApp = v
	}
	if v := os.Getenv("OPENCLAW_WORK"); v != "" {
		cfg.OpenClawWork = v
	}
	if v := os.Getenv("CLAWPANEL_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("ADMIN_TOKEN"); v != "" {
		cfg.AdminToken = v
	}
	if os.Getenv("CLAWPANEL_DEBUG") == "true" {
		cfg.Debug = true
	}
	if v := strings.TrimSpace(strings.ToLower(os.Getenv("CLAWPANEL_EDITION"))); v != "" {
		cfg.Edition = normalizeEdition(v)
	}

	// 尝试从文件加载
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			fmt.Printf("[ClawPanel] 配置文件解析失败，使用默认配置: %v\n", err)
		}
	}

	// Windows 服务场景下避免误落到 systemprofile
	cfg.OpenClawDir = resolveOpenClawDir(cfg.OpenClawDir)
	cfg.Edition = normalizeEdition(cfg.Edition)
	if cfg.IsLiteEdition() {
		cfg.Port = DefaultPort
		cfg.DataDir = filepath.Join(cfg.InstallRoot(), "data")
		cfg.OpenClawDir = cfg.BundledOpenClawConfigDir()
		cfg.OpenClawApp = cfg.BundledOpenClawAppDir()
		cfg.OpenClawWork = cfg.BundledOpenClawWorkDir()
	}

	// 路径校验：
	// 1) 如果配置中的路径是另一个 OS 的路径格式，则重新探测
	// 2) 如果是相对路径（历史版本遗留），统一升级为绝对路径
	// 例如：Windows 上读到了 /root/.openclaw（Linux 路径），或 Linux 上读到了 C:\... （Windows 路径）
	if isStaleOSPath(cfg.OpenClawDir) || !filepath.IsAbs(cfg.OpenClawDir) {
		if cfg.IsLiteEdition() {
			cfg.OpenClawDir = cfg.BundledOpenClawConfigDir()
		} else {
			cfg.OpenClawDir = getDefaultOpenClawDir()
		}
		cfg.OpenClawWork = ""
		cfg.OpenClawApp = ""
	}

	// 设置默认工作目录（基于 OpenClawDir 的父目录）
	parentDir := filepath.Dir(cfg.OpenClawDir) // e.g. /home/user/openclaw or C:\Users\xxx\.openclaw -> C:\Users\xxx
	if cfg.IsLiteEdition() {
		cfg.OpenClawWork = cfg.BundledOpenClawWorkDir()
	} else if cfg.OpenClawWork == "" || !dirExists(cfg.OpenClawWork) {
		// Try npm global openclaw installation first
		npmGlobalDir := getNpmGlobalOpenClawDir()
		cfg.OpenClawWork = findFirstExistingDir(
			filepath.Join(parentDir, "work"),
			filepath.Join(parentDir, "openclaw", "work"),
			filepath.Join(npmGlobalDir, "agents"),
		)
		if cfg.OpenClawWork == "" {
			cfg.OpenClawWork = filepath.Join(parentDir, "work")
		}
	}
	// 设置默认 App 目录
	if cfg.IsLiteEdition() {
		cfg.OpenClawApp = cfg.BundledOpenClawAppDir()
	} else if cfg.OpenClawApp == "" || !dirExists(cfg.OpenClawApp) || !fileExists(filepath.Join(cfg.OpenClawApp, "package.json")) {
		// Try npm global openclaw installation first
		npmGlobalDir := getNpmGlobalOpenClawDir()
		cfg.OpenClawApp = findFirstExistingDir(
			filepath.Join(parentDir, "app"),
			filepath.Join(parentDir, "openclaw", "app"),
			npmGlobalDir,
		)
		if cfg.OpenClawApp == "" {
			cfg.OpenClawApp = filepath.Join(parentDir, "app")
		}
	}

	// 历史兼容：将相对路径升级为绝对路径，避免在服务环境下被解析到错误目录
	if cfg.OpenClawWork != "" && !filepath.IsAbs(cfg.OpenClawWork) {
		cfg.OpenClawWork = filepath.Join(parentDir, cfg.OpenClawWork)
	}
	if cfg.OpenClawApp != "" && !filepath.IsAbs(cfg.OpenClawApp) {
		cfg.OpenClawApp = filepath.Join(parentDir, cfg.OpenClawApp)
	}

	// 保存配置（确保文件存在）
	cfg.Save()

	return cfg, nil
}

func normalizeEdition(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "lite":
		return "lite"
	default:
		return "pro"
	}
}

func (c *Config) IsLiteEdition() bool {
	if c == nil {
		return buildinfo.IsLite()
	}
	return normalizeEdition(c.Edition) == "lite"
}

func (c *Config) IsProEdition() bool {
	return !c.IsLiteEdition()
}

func (c *Config) InstallRoot() string {
	exe, err := os.Executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		if c != nil && strings.TrimSpace(c.DataDir) != "" {
			return filepath.Dir(c.DataDir)
		}
		return "."
	}
	return filepath.Dir(exe)
}

func (c *Config) BundledRuntimeRoot() string {
	return filepath.Join(c.InstallRoot(), "runtime")
}

func (c *Config) BundledOpenClawConfigDir() string {
	return filepath.Join(c.InstallRoot(), "data", "openclaw-config")
}

func (c *Config) BundledOpenClawWorkDir() string {
	return filepath.Join(c.InstallRoot(), "data", "openclaw-work")
}

func (c *Config) BundledOpenClawAppDir() string {
	root := c.BundledRuntimeRoot()
	for _, candidate := range []string{
		filepath.Join(root, "openclaw"),
		filepath.Join(root, "openclaw", "package"),
		filepath.Join(root, "openclaw", "app"),
	} {
		if fileExists(filepath.Join(candidate, "package.json")) || fileExists(filepath.Join(candidate, "openclaw.mjs")) {
			return candidate
		}
	}
	return filepath.Join(root, "openclaw")
}

func (c *Config) BundledOpenClawWorkingDir() string {
	app := strings.TrimSpace(c.BundledOpenClawAppDir())
	if app != "" {
		return app
	}
	return c.OpenClawDir
}

func (c *Config) BundledOpenClawEntrypoint() string {
	app := c.BundledOpenClawAppDir()
	for _, candidate := range []string{
		filepath.Join(c.BundledRuntimeRoot(), "bin", bundledOpenClawCLIName()),
		filepath.Join(app, bundledOpenClawCLIName()),
		filepath.Join(app, "openclaw.mjs"),
	} {
		if fileExists(candidate) {
			return candidate
		}
	}
	return filepath.Join(app, "openclaw.mjs")
}

func (c *Config) BundledPluginsDir() string {
	return filepath.Join(c.BundledRuntimeRoot(), "bundled-plugins")
}

func (c *Config) BundledPluginDir(pluginID string) string {
	return filepath.Join(c.BundledPluginsDir(), strings.TrimSpace(pluginID))
}

func bundledOpenClawCLIName() string {
	if runtime.GOOS == "windows" {
		return "openclaw.cmd"
	}
	return "openclaw"
}

func (c *Config) BundledNodeBinaryPath() string {
	root := c.BundledRuntimeRoot()
	for _, candidate := range []string{
		filepath.Join(root, "node", "bin", "node"),
		filepath.Join(root, "node", "node"),
		filepath.Join(root, "bin", "node"),
		filepath.Join(root, "node.exe"),
		filepath.Join(root, "node", "node.exe"),
	} {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func (c *Config) BundledOpenClawLauncherPath() string {
	for _, candidate := range []string{
		filepath.Join(c.InstallRoot(), "bin", "clawlite-openclaw"),
		filepath.Join(c.InstallRoot(), "bin", "clawlite-openclaw.cmd"),
	} {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func (c *Config) DefaultGatewayPort() int {
	if c.IsLiteEdition() {
		return 18790
	}
	return 18789
}

func (c *Config) OpenClawCommand(args ...string) (*exec.Cmd, error) {
	if c.IsLiteEdition() {
		if launcher := c.BundledOpenClawLauncherPath(); launcher != "" {
			cmd := exec.Command(launcher, args...)
			cmd.Dir = c.BundledOpenClawWorkingDir()
			return cmd, nil
		}
		entry := c.BundledOpenClawEntrypoint()
		if strings.HasSuffix(entry, ".mjs") {
			node := c.BundledNodeBinaryPath()
			if node == "" {
				return nil, fmt.Errorf("Lite 版未找到内置 Node.js 运行时")
			}
			cmd := exec.Command(node, append([]string{entry}, args...)...)
			cmd.Dir = c.BundledOpenClawWorkingDir()
			return cmd, nil
		}
		if fileExists(entry) {
			cmd := exec.Command(entry, args...)
			cmd.Dir = c.BundledOpenClawWorkingDir()
			return cmd, nil
		}
	}
	if bin := DetectOpenClawBinaryPath(); bin != "" {
		return exec.Command(bin, args...), nil
	}
	if p, err := exec.LookPath("openclaw"); err == nil && p != "" {
		return exec.Command(p, args...), nil
	}
	return nil, fmt.Errorf("未找到 openclaw 可执行文件")
}

// Save 保存配置到文件
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cfgPath := filepath.Join(c.DataDir, ConfigFileName)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0644)
}

// SetAdminToken 修改管理密码
func (c *Config) SetAdminToken(token string) {
	c.mu.Lock()
	c.AdminToken = token
	c.mu.Unlock()
	c.Save()
}

// GetAdminToken 获取管理密码
func (c *Config) GetAdminToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AdminToken
}

// SetOpenClawWork 修改 OpenClaw 工作区路径
func (c *Config) SetOpenClawWork(path string) {
	c.mu.Lock()
	c.OpenClawWork = path
	c.mu.Unlock()
	c.Save()
}

// GetOpenClawWork 获取 OpenClaw 工作区路径
func (c *Config) GetOpenClawWork() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.OpenClawWork
}

// getDataDir 获取数据目录（与可执行文件同目录）
func getDataDir() string {
	if v := os.Getenv("CLAWPANEL_DATA"); v != "" {
		return v
	}
	// 使用可执行文件所在目录
	exe, err := os.Executable()
	if err != nil {
		return "./data"
	}
	return filepath.Join(filepath.Dir(exe), "data")
}

// getDefaultOpenClawDir 获取默认 OpenClaw 配置目录
func getDefaultOpenClawDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		if runtime.GOOS == "darwin" {
			home = "/var/root"
		} else if runtime.GOOS == "windows" {
			home = os.Getenv("USERPROFILE")
			if home == "" {
				home = `C:\Users\Administrator`
			}
		} else {
			home = "/root"
		}
	}

	// Build candidate list: check multiple possible locations
	candidates := []string{
		filepath.Join(home, ".openclaw"),
	}

	if runtime.GOOS == "windows" {
		// When running as SYSTEM service, os.UserHomeDir() returns
		// C:\Windows\system32\config\systemprofile — not the real user.
		// Probe all user profiles for .openclaw with openclaw.json.
		for _, userHome := range getWindowsUserHomes() {
			candidates = append(candidates, filepath.Join(userHome, ".openclaw"))
		}
	} else {
		// Linux/macOS: also check /home/*/openclaw/config, /root/.openclaw
		if home != "/root" {
			candidates = append(candidates, "/root/.openclaw")
		}
		entries, _ := os.ReadDir("/home")
		for _, e := range entries {
			if e.IsDir() {
				candidates = append(candidates, filepath.Join("/home", e.Name(), ".openclaw"))
			}
		}

		if runtime.GOOS == "darwin" {
			entries, _ := os.ReadDir("/Users")
			for _, e := range entries {
				if e.IsDir() {
					candidates = append(candidates, filepath.Join("/Users", e.Name(), ".openclaw"))
				}
			}
		}
	}

	// Return the first candidate that contains openclaw.json
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "openclaw.json")); err == nil {
			return c
		}
	}

	// If no openclaw.json found, check if npm global openclaw exists
	// This handles the case where OpenClaw is installed via npm but not yet configured
	npmGlobalDir := getNpmGlobalOpenClawDir()
	if npmGlobalDir != "" {
		// Return the first real user's .openclaw directory (or create path for it)
		if runtime.GOOS == "windows" {
			for _, userHome := range getWindowsUserHomes() {
				userOpenClawDir := filepath.Join(userHome, ".openclaw")
				// Return this path even if it doesn't exist yet - it will be created
				return userOpenClawDir
			}
		}
		// For non-Windows or if no user homes found, return home/.openclaw
		return filepath.Join(home, ".openclaw")
	}

	// Fallback: return the first candidate that exists as a directory
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	// Ultimate fallback
	return filepath.Join(home, ".openclaw")
}

// getWindowsUserHomes returns home directories of real user profiles on Windows
func getWindowsUserHomes() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	var homes []string

	// Scan C:\Users\* for real user profiles FIRST (prioritize real users)
	usersDir := `C:\Users`
	entries, err := os.ReadDir(usersDir)
	if err == nil {
		skip := map[string]bool{"Public": true, "Default": true, "Default User": true, "All Users": true}
		for _, e := range entries {
			if !e.IsDir() || skip[e.Name()] {
				continue
			}
			userPath := filepath.Join(usersDir, e.Name())
			// Skip SYSTEM account paths (C:\Windows\system32\config\systemprofile)
			if !strings.Contains(userPath, "system32") && !strings.Contains(userPath, "systemprofile") {
				homes = append(homes, userPath)
			}
		}
	}

	// Only add USERPROFILE env if it's not a SYSTEM path and not already in list
	if up := os.Getenv("USERPROFILE"); up != "" {
		if !strings.Contains(up, "system32") && !strings.Contains(up, "systemprofile") {
			// Check if not already added
			found := false
			for _, h := range homes {
				if h == up {
					found = true
					break
				}
			}
			if !found {
				homes = append(homes, up)
			}
		}
	}

	return homes
}

// ReadOpenClawJSON 读取 openclaw.json
func (c *Config) ReadOpenClawJSON() (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cfgPath := filepath.Join(resolveOpenClawDir(c.OpenClawDir), "openclaw.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		// 兼容 JSON5（注释、尾逗号等）
		if err5 := json5.Unmarshal(data, &result); err5 != nil {
			return nil, fmt.Errorf("解析 openclaw.json 失败(JSON/JSON5): %w", err)
		}
	}
	return result, nil
}

// NormalizeOpenClawJSONFile 对磁盘上的 openclaw.json 执行兼容性清洗，并在有变更时回写。
// 返回值 changed 表示文件内容是否被修正。
func (c *Config) NormalizeOpenClawJSONFile() (changed bool, err error) {
	ocConfig, err := c.ReadOpenClawJSON()
	if err != nil {
		return false, err
	}
	if ocConfig == nil {
		return false, nil
	}
	if !NormalizeOpenClawConfigForWrite(ocConfig, c.OpenClawDir) {
		return false, nil
	}
	if err := c.WriteOpenClawJSON(ocConfig); err != nil {
		return false, err
	}
	return true, nil
}

// ReadQQChannelState returns whether the QQ channel is enabled and its access token.
func (c *Config) ReadQQChannelState() (bool, string, error) {
	ocConfig, err := c.ReadOpenClawJSON()
	if err != nil {
		return false, "", err
	}

	channels, _ := ocConfig["channels"].(map[string]interface{})
	qq, _ := channels["qq"].(map[string]interface{})
	if qq == nil {
		return false, "", nil
	}

	enabled, _ := qq["enabled"].(bool)
	token, _ := qq["accessToken"].(string)
	return enabled, strings.TrimSpace(token), nil
}

// WriteOpenClawJSON 写入 openclaw.json
func (c *Config) WriteOpenClawJSON(data map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	resolved := resolveOpenClawDir(c.OpenClawDir)
	if resolved != "" && resolved != c.OpenClawDir {
		c.OpenClawDir = resolved
	}
	cfgPath := filepath.Join(c.OpenClawDir, "openclaw.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}
	if err := backupOpenClawBeforeWrite(cfgPath, filepath.Join(c.OpenClawDir, "backups")); err != nil {
		return err
	}
	// 兼容清洗：避免写入新版 OpenClaw 不接受的 legacy 字段。
	NormalizeOpenClawConfigForWrite(data, c.OpenClawDir)
	jsonData, err := marshalOpenClawJSON(data)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, jsonData, 0644)
}

type openClawJSONOrderContext struct {
	feishuDefaultAccount string
}

func marshalOpenClawJSON(data map[string]interface{}) ([]byte, error) {
	var buf bytes.Buffer
	ctx := openClawJSONOrderContext{}
	if channels, ok := data["channels"].(map[string]interface{}); ok && channels != nil {
		if feishu, ok := channels["feishu"].(map[string]interface{}); ok && feishu != nil {
			ctx.feishuDefaultAccount = strings.TrimSpace(stringValue(feishu["defaultAccount"]))
		}
	}
	if err := writeOrderedJSONValue(&buf, data, 0, nil, ctx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeOrderedJSONValue(buf *bytes.Buffer, value interface{}, depth int, path []string, ctx openClawJSONOrderContext) error {
	if value == nil {
		buf.WriteString("null")
		return nil
	}

	rv := reflect.ValueOf(value)
	for rv.IsValid() && (rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer) {
		if rv.IsNil() {
			buf.WriteString("null")
			return nil
		}
		rv = rv.Elem()
		value = rv.Interface()
	}
	if !rv.IsValid() {
		buf.WriteString("null")
		return nil
	}

	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			break
		}
		obj := make(map[string]interface{}, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			mapValue := iter.Value()
			if mapValue.IsValid() {
				obj[key] = mapValue.Interface()
			} else {
				obj[key] = nil
			}
		}
		return writeOrderedJSONObject(buf, obj, depth, path, ctx)
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			break
		}
		items := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			items[i] = rv.Index(i).Interface()
		}
		return writeOrderedJSONArray(buf, items, depth, path, ctx)
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	buf.Write(raw)
	return nil
}

func writeOrderedJSONObject(buf *bytes.Buffer, obj map[string]interface{}, depth int, path []string, ctx openClawJSONOrderContext) error {
	if len(obj) == 0 {
		buf.WriteString("{}")
		return nil
	}

	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	keys = orderedJSONMapKeys(keys, path, ctx)

	buf.WriteString("{\n")
	for i, key := range keys {
		writeJSONIndent(buf, depth+1)
		keyRaw, err := json.Marshal(key)
		if err != nil {
			return err
		}
		buf.Write(keyRaw)
		buf.WriteString(": ")
		if err := writeOrderedJSONValue(buf, obj[key], depth+1, append(path, key), ctx); err != nil {
			return err
		}
		if i < len(keys)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeJSONIndent(buf, depth)
	buf.WriteByte('}')
	return nil
}

func writeOrderedJSONArray(buf *bytes.Buffer, items []interface{}, depth int, path []string, ctx openClawJSONOrderContext) error {
	if len(items) == 0 {
		buf.WriteString("[]")
		return nil
	}

	buf.WriteString("[\n")
	for i, item := range items {
		writeJSONIndent(buf, depth+1)
		if err := writeOrderedJSONValue(buf, item, depth+1, path, ctx); err != nil {
			return err
		}
		if i < len(items)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	writeJSONIndent(buf, depth)
	buf.WriteByte(']')
	return nil
}

func orderedJSONMapKeys(keys []string, path []string, ctx openClawJSONOrderContext) []string {
	ordered := append([]string(nil), keys...)
	sort.Strings(ordered)

	if !isFeishuAccountsPath(path) || ctx.feishuDefaultAccount == "" {
		return ordered
	}

	found := false
	for _, key := range ordered {
		if key == ctx.feishuDefaultAccount {
			found = true
			break
		}
	}
	if !found {
		return ordered
	}

	reordered := make([]string, 0, len(ordered))
	reordered = append(reordered, ctx.feishuDefaultAccount)
	for _, key := range ordered {
		if key == ctx.feishuDefaultAccount {
			continue
		}
		reordered = append(reordered, key)
	}
	return reordered
}

func isFeishuAccountsPath(path []string) bool {
	return len(path) == 3 && path[0] == "channels" && path[1] == "feishu" && path[2] == "accounts"
}

func writeJSONIndent(buf *bytes.Buffer, depth int) {
	for i := 0; i < depth; i++ {
		buf.WriteString("  ")
	}
}

func backupOpenClawBeforeWrite(cfgPath, backupDir string) error {
	info, err := os.Stat(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("openclaw.json 路径是目录: %s", cfgPath)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	ts := time.Now().Format("20060102T150405.000")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("pre-edit-%s.json", ts))
	if err := os.WriteFile(backupPath, raw, 0644); err != nil {
		return err
	}
	return cleanupOldPreEditBackups(backupDir, 10)
}

func cleanupOldPreEditBackups(backupDir string, keep int) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil
	}

	type backupFile struct {
		name string
		path string
		mod  time.Time
	}
	var files []backupFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "pre-edit-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, backupFile{
			name: name,
			path: filepath.Join(backupDir, name),
			mod:  info.ModTime(),
		})
	}
	if len(files) <= keep {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].mod.After(files[j].mod)
	})
	for i := keep; i < len(files); i++ {
		_ = os.Remove(files[i].path)
	}
	return nil
}

func resolveOpenClawDir(current string) string {
	if current != "" {
		jsonPath := filepath.Join(current, "openclaw.json")
		if _, err := os.Stat(jsonPath); err == nil {
			return current
		}
		if runtime.GOOS != "windows" {
			return current
		}
		lower := strings.ToLower(current)
		if !strings.Contains(lower, "systemprofile") && !strings.Contains(lower, "system32") {
			return current
		}
	}

	if runtime.GOOS == "windows" {
		for _, userHome := range getWindowsUserHomes() {
			candidate := filepath.Join(userHome, ".openclaw")
			if _, err := os.Stat(filepath.Join(candidate, "openclaw.json")); err == nil {
				return candidate
			}
		}
		legacy := `C:\ClawPanel\openclaw`
		if _, err := os.Stat(filepath.Join(legacy, "openclaw.json")); err == nil {
			return legacy
		}
	}

	if current != "" {
		return current
	}
	return getDefaultOpenClawDir()
}

// isStaleOSPath 检测配置文件里的路径是否来自另一个 OS（跨平台路径污染）
// 例如 Windows 上读到 /root/.openclaw（Unix 绝对路径），或 Linux 上读到 C:\Users\...
func isStaleOSPath(p string) bool {
	if p == "" {
		return false
	}
	if runtime.GOOS == "windows" {
		// Unix 绝对路径 /xxx 在 Windows 上无效
		return strings.HasPrefix(p, "/")
	}
	// Unix 上：Windows 驱动器路径 C:\ 或 C:/ 无效
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return true
	}
	return false
}

// dirExists 检查目录是否存在
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// findFirstExistingDir returns the first directory path that exists, or "" if none
func findFirstExistingDir(paths ...string) string {
	for _, p := range paths {
		if p != "" && dirExists(p) {
			return p
		}
	}
	return ""
}

// getNpmGlobalOpenClawDir returns the npm global openclaw installation directory
func getNpmGlobalOpenClawDir() string {
	// Try npm root -g command first
	cmd := exec.Command("npm", "root", "-g")
	cmd.Env = BuildExecEnv()
	if out, err := cmd.Output(); err == nil {
		npmRoot := strings.TrimSpace(string(out))
		if npmRoot != "" {
			openclawDir := filepath.Join(npmRoot, "openclaw")
			if dirExists(openclawDir) {
				return openclawDir
			}
		}
	}

	// Try npm prefix -g command as a fallback
	cmd = exec.Command("npm", "config", "get", "prefix")
	cmd.Env = BuildExecEnv()
	if out, err := cmd.Output(); err == nil {
		prefix := strings.TrimSpace(string(out))
		if prefix != "" && prefix != "undefined" {
			openclawDir := filepath.Join(prefix, "lib", "node_modules", "openclaw")
			if dirExists(openclawDir) {
				return openclawDir
			}
		}
	}

	// Scan version manager directories directly (nvm/fnm/volta), independent of PATH.
	for _, home := range candidateHomes() {
		patterns := []string{
			filepath.Join(home, ".nvm", "versions", "node", "*", "lib", "node_modules", "openclaw"),
			filepath.Join(home, ".local", "share", "fnm", "node-versions", "*", "installation", "lib", "node_modules", "openclaw"),
			filepath.Join(home, ".fnm", "node-versions", "*", "installation", "lib", "node_modules", "openclaw"),
			filepath.Join(home, ".volta", "tools", "image", "node", "*", "lib", "node_modules", "openclaw"),
		}
		for _, pattern := range patterns {
			for _, p := range globVersionedDirs(pattern) {
				if dirExists(p) {
					return p
				}
			}
		}
	}

	// Fallback: check common npm global paths
	if runtime.GOOS == "windows" {
		for _, userHome := range getWindowsUserHomes() {
			paths := []string{
				filepath.Join(userHome, "AppData", "Roaming", "npm", "node_modules", "openclaw"),
			}
			for _, p := range paths {
				if dirExists(p) {
					return p
				}
			}
		}
		// Check Program Files
		if dirExists(`C:\Program Files\nodejs\node_modules\openclaw`) {
			return `C:\Program Files\nodejs\node_modules\openclaw`
		}
	} else {
		// Linux/macOS
		paths := []string{
			"/usr/lib/node_modules/openclaw",
			"/usr/local/lib/node_modules/openclaw",
		}
		home, _ := os.UserHomeDir()
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".npm-global", "lib", "node_modules", "openclaw"),
				filepath.Join(home, ".local", "lib", "node_modules", "openclaw"),
			)
		}
		for _, p := range paths {
			if dirExists(p) {
				return p
			}
		}
	}
	return ""
}

// OpenClawConfigExists 检查 openclaw.json 是否存在
func (c *Config) OpenClawConfigExists() bool {
	cfgPath := filepath.Join(c.OpenClawDir, "openclaw.json")
	_, err := os.Stat(cfgPath)
	return err == nil
}

// OpenClawInstalled 检查 OpenClaw 是否已安装（配置文件存在 或 二进制可执行）
// 解决 npm 安装后配置文件尚未生成但二进制已可用的情况
func (c *Config) OpenClawInstalled() bool {
	if c.IsLiteEdition() {
		if c.OpenClawConfigExists() {
			return true
		}
		if c.OpenClawApp != "" {
			if _, err := os.Stat(filepath.Join(c.OpenClawApp, "package.json")); err == nil {
				return true
			}
		}
		if launcher := c.BundledOpenClawLauncherPath(); launcher != "" {
			return true
		}
		entry := c.BundledOpenClawEntrypoint()
		if fileExists(entry) {
			return true
		}
		if entry != "" && strings.HasSuffix(entry, ".mjs") && c.BundledNodeBinaryPath() != "" {
			return true
		}
		return false
	}

	// 1. 配置文件存在
	if c.OpenClawConfigExists() {
		return true
	}
	// 1.5 OpenClaw 应用目录存在（package.json）
	if c.OpenClawApp != "" {
		if _, err := os.Stat(filepath.Join(c.OpenClawApp, "package.json")); err == nil {
			return true
		}
	}
	// 1.8 npm 全局/版本管理器目录可探测到 OpenClaw
	if appDir := getNpmGlobalOpenClawDir(); appDir != "" {
		if _, err := os.Stat(filepath.Join(appDir, "package.json")); err == nil {
			return true
		}
	}
	// 2. 二进制在 PATH 中或可探测路径中可用
	if p, err := exec.LookPath("openclaw"); err == nil && p != "" {
		return true
	}
	if p := DetectOpenClawBinaryPath(); p != "" {
		return true
	}
	// 2.5 常见绝对路径兜底（服务环境 PATH 可能不完整）
	commonBins := []string{
		"/usr/local/bin/openclaw",
		"/usr/bin/openclaw",
		"/opt/homebrew/bin/openclaw",
	}
	for _, bin := range commonBins {
		if _, err := os.Stat(bin); err == nil {
			return true
		}
	}
	// 3. Windows: 检查 npm 全局目录（包括普通用户和 SYSTEM 账户）
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		winPaths := []string{
			filepath.Join(home, "AppData", "Roaming", "npm", "openclaw.cmd"),
			filepath.Join(home, "AppData", "Roaming", "npm", "openclaw"),
		}
		// Also check real user profiles (when running as SYSTEM service)
		for _, userHome := range getWindowsUserHomes() {
			winPaths = append(winPaths,
				filepath.Join(userHome, "AppData", "Roaming", "npm", "openclaw.cmd"),
				filepath.Join(userHome, "AppData", "Roaming", "npm", "openclaw"),
			)
		}
		// Also check SYSTEM account paths (when running as Windows service)
		systemProfile := os.Getenv("SYSTEMROOT")
		if systemProfile != "" {
			winPaths = append(winPaths,
				filepath.Join(systemProfile, "system32", "config", "systemprofile", "AppData", "Roaming", "npm", "openclaw.cmd"),
			)
		}
		// Check Program Files nodejs path
		winPaths = append(winPaths, `C:\Program Files\nodejs\openclaw.cmd`)
		// Check npm prefix path
		cmd := exec.Command("npm", "config", "get", "prefix")
		cmd.Env = BuildExecEnv()
		if out, err := cmd.Output(); err == nil {
			prefix := strings.TrimSpace(string(out))
			if prefix != "" {
				winPaths = append(winPaths, filepath.Join(prefix, "openclaw.cmd"))
			}
		}
		for _, wp := range winPaths {
			if _, err := os.Stat(wp); err == nil {
				return true
			}
		}
	}
	return false
}
