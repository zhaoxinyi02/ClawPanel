package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	json5 "github.com/titanous/json5"
)

// Config 应用配置
type Config struct {
	Port         int    `json:"port"`
	DataDir      string `json:"dataDir"`
	OpenClawDir  string `json:"openClawDir"`
	OpenClawApp  string `json:"openClawApp"`
	OpenClawWork string `json:"openClawWork"`
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

	// 尝试从文件加载
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			fmt.Printf("[ClawPanel] 配置文件解析失败，使用默认配置: %v\n", err)
		}
	}

	// 设置默认工作目录（基于 OpenClawDir 的父目录）
	parentDir := filepath.Dir(cfg.OpenClawDir) // e.g. /home/user/openclaw or C:\Users\xxx\.openclaw -> C:\Users\xxx
	if cfg.OpenClawWork == "" || !dirExists(cfg.OpenClawWork) {
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
	if cfg.OpenClawApp == "" || !dirExists(cfg.OpenClawApp) {
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

	// 保存配置（确保文件存在）
	cfg.Save()

	return cfg, nil
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

	cfgPath := filepath.Join(c.OpenClawDir, "openclaw.json")
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

// WriteOpenClawJSON 写入 openclaw.json
func (c *Config) WriteOpenClawJSON(data map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cfgPath := filepath.Join(c.OpenClawDir, "openclaw.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}
	if err := backupOpenClawBeforeWrite(cfgPath, filepath.Join(c.OpenClawDir, "backups")); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, jsonData, 0644)
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
	if out, err := exec.Command("npm", "root", "-g").Output(); err == nil {
		npmRoot := strings.TrimSpace(string(out))
		if npmRoot != "" {
			openclawDir := filepath.Join(npmRoot, "openclaw")
			if dirExists(openclawDir) {
				return openclawDir
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
	// 1. 配置文件存在
	if c.OpenClawConfigExists() {
		return true
	}
	// 2. 二进制在 PATH 中可用
	if p, err := exec.LookPath("openclaw"); err == nil && p != "" {
		return true
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
		if out, err := exec.Command("npm", "config", "get", "prefix").Output(); err == nil {
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
