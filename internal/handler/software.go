package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
	"github.com/zhaoxinyi02/ClawPanel/internal/taskman"
)

// SoftwareInfo 软件信息
type SoftwareInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Installed   bool   `json:"installed"`
	Status      string `json:"status"`   // installed, not_installed, running, stopped, plugin_missing
	Category    string `json:"category"` // runtime, container, service
	Installable bool   `json:"installable"`
	Icon        string `json:"icon,omitempty"`
}

func hasQQPlugin(cfg *config.Config) bool {
	for _, candidate := range []string{
		filepath.Join(cfg.OpenClawDir, "extensions", "qq"),
		filepath.Join(filepath.Dir(cfg.OpenClawDir), "extensions", "qq"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// OpenClawInstance 检测到的 OpenClaw 实例
type OpenClawInstance struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // npm, source, docker, systemd
	Label   string `json:"label"`
	Version string `json:"version"`
	Path    string `json:"path,omitempty"`
	Active  bool   `json:"active"`
	Status  string `json:"status"` // running, stopped, unknown
}

func detectCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Env = config.BuildExecEnv()
	out, err := cmd.Output()
	if err != nil {
		if runtime.GOOS == "darwin" && name != "arch" {
			for _, archFlag := range []string{"-arm64", "-x86_64"} {
				altArgs := append([]string{archFlag, name}, args...)
				alt := exec.Command("arch", altArgs...)
				alt.Env = cmd.Env
				if out2, err2 := alt.Output(); err2 == nil {
					return strings.TrimSpace(string(out2))
				}
			}
		}
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isDockerContainerRunning(name string) bool {
	out := detectCmd("docker", "inspect", "--format", "{{.State.Running}}", name)
	return out == "true"
}

func getDockerContainerStatus(name string) (bool, string) {
	out := detectCmd("docker", "inspect", "--format", "{{.State.Status}}", name)
	if out == "" {
		return false, "not_installed"
	}
	return true, out // running, exited, etc.
}

// GetSoftwareList 获取软件环境列表
func GetSoftwareList(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var list []SoftwareInfo

		// Node.js
		nodeVer := detectCmd("node", "--version")
		list = append(list, SoftwareInfo{
			ID: "nodejs", Name: "Node.js", Description: "JavaScript 运行时",
			Version: nodeVer, Installed: nodeVer != "", Installable: true,
			Status: boolStatus(nodeVer != ""), Category: "runtime", Icon: "terminal",
		})

		// npm
		npmVer := detectCmd("npm", "--version")
		list = append(list, SoftwareInfo{
			ID: "npm", Name: "npm", Description: "Node.js 包管理器",
			Version: npmVer, Installed: npmVer != "", Installable: false,
			Status: boolStatus(npmVer != ""), Category: "runtime", Icon: "package",
		})

		// Docker
		dockerVer := detectCmd("docker", "--version")
		list = append(list, SoftwareInfo{
			ID: "docker", Name: "Docker", Description: "容器运行时",
			Version: dockerVer, Installed: dockerVer != "", Installable: true,
			Status: boolStatus(dockerVer != ""), Category: "runtime", Icon: "box",
		})

		// Git
		gitVer := detectCmd("git", "--version")
		list = append(list, SoftwareInfo{
			ID: "git", Name: "Git", Description: "版本控制系统",
			Version: gitVer, Installed: gitVer != "", Installable: true,
			Status: boolStatus(gitVer != ""), Category: "runtime", Icon: "git-branch",
		})

		// Python
		pythonVer := detectCmd("python3", "--version")
		list = append(list, SoftwareInfo{
			ID: "python", Name: "Python 3", Description: "Python 运行时",
			Version: pythonVer, Installed: pythonVer != "", Installable: true,
			Status: boolStatus(pythonVer != ""), Category: "runtime", Icon: "code",
		})

		// OpenClaw
		ocVer := detectOpenClawVersion(cfg)
		list = append(list, SoftwareInfo{
			ID: "openclaw", Name: "OpenClaw", Description: "AI 助手核心引擎",
			Version: ocVer, Installed: ocVer != "", Installable: true,
			Status: boolStatus(ocVer != ""), Category: "service", Icon: "brain",
		})

		// NapCat (QQ) - detect Docker container OR native Windows Shell install
		napcatExists := false
		napcatStatus := "not_installed"
		napcatVer := ""
		if runtime.GOOS == "windows" {
			// Windows: detect NapCat Shell installation
			napcatDir := getNapCatShellDir(cfg)
			if napcatDir != "" {
				napcatExists = true
				napcatVer = "Shell (Windows)"
				// Check if napcat process is running
				if isNapCatShellRunning() {
					napcatStatus = "running"
				} else {
					napcatStatus = "installed"
				}
			}
		} else {
			// Linux/macOS: detect Docker container
			napcatExists, napcatStatus = getDockerContainerStatus("openclaw-qq")
			if napcatExists {
				napcatVer = "Docker"
			}
		}
		if napcatExists && !hasQQPlugin(cfg) {
			napcatStatus = "plugin_missing"
		}
		list = append(list, SoftwareInfo{
			ID: "napcat", Name: "NapCat (QQ个人号)", Description: "QQ 机器人 OneBot11 协议",
			Version: napcatVer, Installed: napcatExists, Installable: true,
			Status: napcatStatus, Category: "container", Icon: "message-circle",
		})

		// WeChat Bot
		wechatExists, wechatStatus := getDockerContainerStatus("openclaw-wechat")
		wechatVer := ""
		if wechatExists {
			wechatVer = "Docker"
		}
		list = append(list, SoftwareInfo{
			ID: "wechat", Name: "微信机器人", Description: "wechatbot-webhook 微信个人号",
			Version: wechatVer, Installed: wechatExists, Installable: true,
			Status: wechatStatus, Category: "container", Icon: "message-square",
		})

		c.JSON(http.StatusOK, gin.H{"ok": true, "software": list, "platform": runtime.GOOS})
	}
}

func boolStatus(installed bool) string {
	if installed {
		return "installed"
	}
	return "not_installed"
}

func nodeMajorVersion(ver string) int {
	v := strings.TrimSpace(strings.TrimPrefix(ver, "v"))
	if v == "" {
		return -1
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return -1
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return -1
	}
	return major
}

func formatOpenClawManualPrerequisiteError(platform, nodeVer, gitVer string) error {
	if platform != "windows" && platform != "darwin" {
		return nil
	}
	label := platform
	if label == "darwin" {
		label = "macOS"
	}

	missing := make([]string, 0, 2)
	if nodeVer == "" {
		missing = append(missing, "Node.js (>=20)")
	} else if nodeMajorVersion(nodeVer) < 20 {
		missing = append(missing, fmt.Sprintf("Node.js >=20 (当前 %s)", nodeVer))
	}
	if gitVer == "" {
		missing = append(missing, "Git")
	}
	if len(missing) == 0 {
		return nil
	}
	if len(missing) == 1 {
		return fmt.Errorf("检测到 %s 平台缺少 %s，请先手动安装后再执行一键安装 OpenClaw", label, missing[0])
	}
	return fmt.Errorf("检测到 %s 平台缺少 %s，请先手动安装后再执行一键安装 OpenClaw", label, strings.Join(missing, " 和 "))
}

func ensureOpenClawManualPrerequisites() error {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return nil
	}
	nodeVer := detectCmd("node", "--version")
	gitVer := detectCmd("git", "--version")
	return formatOpenClawManualPrerequisiteError(runtime.GOOS, nodeVer, gitVer)
}

func detectOpenClawVersion(cfg *config.Config) string {
	// 1. Try reading from cfg.OpenClawApp FIRST (most reliable for SYSTEM service)
	if cfg.OpenClawApp != "" {
		pkgPath := filepath.Join(cfg.OpenClawApp, "package.json")
		if v := readVersionFromPackageJSON(pkgPath); v != "" {
			return v
		}
	}

	// 2. Try binary path detection independent of PATH (nvm/fnm/service env safe)
	if bin := config.DetectOpenClawBinaryPath(); bin != "" {
		if out := detectCmd(bin, "--version"); out != "" {
			return strings.TrimPrefix(strings.TrimSpace(out), "v")
		}
	}

	// 3. Try npm global package.json (may fail when running as SYSTEM service)
	npmRoot := detectCmd("npm", "root", "-g")
	if npmRoot != "" {
		pkgPath := filepath.Join(npmRoot, "openclaw", "package.json")
		if v := readVersionFromPackageJSON(pkgPath); v != "" {
			return v
		}
	}

	// 3.5 Try common app path discovery from config layer
	if appDir := configPathDiscoverOpenClawApp(); appDir != "" {
		if v := readVersionFromPackageJSON(filepath.Join(appDir, "package.json")); v != "" {
			return v
		}
	}

	// 4. Try openclaw CLI (may not work when running as SYSTEM service)
	ver := detectCmd("openclaw", "--version")
	if ver != "" {
		return strings.TrimPrefix(strings.TrimSpace(ver), "v")
	}

	// 5. Try from config meta.lastTouchedVersion
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig != nil {
		if meta, ok := ocConfig["meta"].(map[string]interface{}); ok {
			if v, ok := meta["lastTouchedVersion"].(string); ok && v != "" {
				return v
			}
		}
	}

	// 6. Try common binary paths
	home, _ := os.UserHomeDir()
	commonPaths := []string{
		"/usr/local/bin/openclaw",
		"/usr/bin/openclaw",
		filepath.Join(home, ".local/bin/openclaw"),
		filepath.Join(home, ".npm-global/bin/openclaw"),
	}
	if runtime.GOOS == "windows" {
		commonPaths = append(commonPaths,
			filepath.Join(home, "AppData", "Roaming", "npm", "openclaw.cmd"),
			`C:\Program Files\nodejs\openclaw.cmd`,
		)
		// Scan real user profiles (when running as SYSTEM service)
		usersDir := `C:\Users`
		if entries, err := os.ReadDir(usersDir); err == nil {
			skip := map[string]bool{"Public": true, "Default": true, "Default User": true, "All Users": true}
			for _, e := range entries {
				if e.IsDir() && !skip[e.Name()] {
					commonPaths = append(commonPaths,
						filepath.Join(usersDir, e.Name(), "AppData", "Roaming", "npm", "openclaw.cmd"),
					)
				}
			}
		}
		// SYSTEM account path (when running as Windows service)
		systemRoot := os.Getenv("SYSTEMROOT")
		if systemRoot != "" {
			commonPaths = append(commonPaths,
				filepath.Join(systemRoot, "system32", "config", "systemprofile", "AppData", "Roaming", "npm", "openclaw.cmd"),
			)
		}
		// npm prefix path
		npmPrefix := detectCmd("npm", "config", "get", "prefix")
		if npmPrefix != "" {
			commonPaths = append(commonPaths, filepath.Join(npmPrefix, "openclaw.cmd"))
		}
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			out := detectCmd(p, "--version")
			if out != "" {
				return strings.TrimPrefix(strings.TrimSpace(out), "v")
			}
		}
	}

	// 7. Try source installs: check common directories for package.json
	sourcePaths := []string{
		filepath.Join(os.Getenv("HOME"), "openclaw"),
		filepath.Join(os.Getenv("HOME"), "openclaw/app"),
		"/opt/openclaw",
		"/usr/lib/node_modules/openclaw",
	}
	for _, sp := range sourcePaths {
		pkgPath := filepath.Join(sp, "package.json")
		if v := readVersionFromPackageJSON(pkgPath); v != "" {
			return v
		}
	}

	// 8. Try Docker container
	dockerVer := detectCmd("docker", "exec", "openclaw", "openclaw", "--version")
	if dockerVer != "" {
		return strings.TrimPrefix(strings.TrimSpace(dockerVer), "v")
	}

	// 9. Try systemd: parse ExecStart from service file to find binary path
	svcContent := detectCmd("systemctl", "cat", "openclaw")
	if svcContent != "" {
		for _, line := range strings.Split(svcContent, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ExecStart=") {
				parts := strings.Fields(strings.TrimPrefix(line, "ExecStart="))
				if len(parts) > 0 {
					bin := parts[0]
					out := detectCmd(bin, "--version")
					if out != "" {
						return strings.TrimPrefix(strings.TrimSpace(out), "v")
					}
				}
				break
			}
		}
	}

	return ""
}

func configPathDiscoverOpenClawApp() string {
	// Keep this helper in handler layer so we don't expose internal config methods.
	// It derives app path from known binary path layout when package.json is missing from cfg.
	if bin := config.DetectOpenClawBinaryPath(); bin != "" {
		// .../bin/openclaw -> .../lib/node_modules/openclaw
		parent := filepath.Dir(filepath.Dir(bin))
		candidate := filepath.Join(parent, "lib", "node_modules", "openclaw")
		if info, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// readVersionFromPackageJSON reads the "version" field from a package.json file
func readVersionFromPackageJSON(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	if v, ok := pkg["version"].(string); ok && v != "" {
		return v
	}
	return ""
}

// DetectOpenClawInstances 检测所有 OpenClaw 安装实例
func DetectOpenClawInstances(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var instances []OpenClawInstance

		// 1. npm global install
		var npmPath string
		npmPath = config.DetectOpenClawBinaryPath()
		// Fallback to shell lookup for unusual wrappers
		if npmPath == "" {
			if runtime.GOOS == "windows" {
				npmPath = detectCmd("where", "openclaw")
				if idx := strings.Index(npmPath, "\n"); idx > 0 {
					npmPath = strings.TrimSpace(npmPath[:idx])
				}
			} else {
				npmPath = detectCmd("which", "openclaw")
			}
		}
		if npmPath != "" {
			ver := detectCmd(npmPath, "--version")
			instances = append(instances, OpenClawInstance{
				ID: "npm-global", Type: "npm", Label: "npm 全局安装",
				Version: ver, Path: npmPath, Active: true, Status: "installed",
			})
		}
		// Windows: also check npm global dir directly
		if runtime.GOOS == "windows" && npmPath == "" {
			home, _ := os.UserHomeDir()
			winNpmPath := filepath.Join(home, "AppData", "Roaming", "npm", "openclaw.cmd")
			if _, err := os.Stat(winNpmPath); err == nil {
				ver := detectCmd("openclaw", "--version")
				instances = append(instances, OpenClawInstance{
					ID: "npm-global", Type: "npm", Label: "npm 全局安装",
					Version: ver, Path: winNpmPath, Active: true, Status: "installed",
				})
			}
		}

		// 2. systemd service
		systemdOut := detectCmd("systemctl", "is-active", "openclaw")
		if systemdOut == "active" || systemdOut == "inactive" {
			ver := ""
			if ocConfig, _ := cfg.ReadOpenClawJSON(); ocConfig != nil {
				if meta, ok := ocConfig["meta"].(map[string]interface{}); ok {
					ver, _ = meta["lastTouchedVersion"].(string)
				}
			}
			instances = append(instances, OpenClawInstance{
				ID: "systemd", Type: "systemd", Label: "systemd 服务",
				Version: ver, Active: systemdOut == "active", Status: systemdOut,
			})
		}

		// 3. Docker container
		dockerOut := detectCmd("docker", "ps", "-a", "--filter", "name=openclaw", "--format", "{{.Names}}|{{.Status}}|{{.Image}}")
		if dockerOut != "" {
			for _, line := range strings.Split(dockerOut, "\n") {
				parts := strings.SplitN(line, "|", 3)
				if len(parts) >= 2 {
					name := parts[0]
					status := parts[1]
					image := ""
					if len(parts) >= 3 {
						image = parts[2]
					}
					// Skip our management containers
					if name == "openclaw-qq" || name == "openclaw-wechat" {
						continue
					}
					running := strings.HasPrefix(status, "Up")
					instances = append(instances, OpenClawInstance{
						ID: "docker-" + name, Type: "docker", Label: "Docker: " + name,
						Version: image, Path: name, Active: running,
						Status: func() string {
							if running {
								return "running"
							}
							return "stopped"
						}(),
					})
				}
			}
		}

		// 4. Source code install (check common paths)
		sourcePaths := []string{
			filepath.Join(os.Getenv("HOME"), "openclaw"),
			"/opt/openclaw",
		}
		for _, sp := range sourcePaths {
			pkgPath := filepath.Join(sp, "package.json")
			if _, err := os.Stat(pkgPath); err == nil {
				var pkg map[string]interface{}
				if data, err := os.ReadFile(pkgPath); err == nil {
					json.Unmarshal(data, &pkg)
				}
				ver, _ := pkg["version"].(string)
				instances = append(instances, OpenClawInstance{
					ID: "source-" + sp, Type: "source", Label: "源码: " + sp,
					Version: ver, Path: sp, Active: false, Status: "installed",
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "instances": instances})
	}
}

// InstallSoftware 一键安装软件
func InstallSoftware(cfg *config.Config, tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Software string `json:"software"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Software == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "software required"})
			return
		}

		if tm.HasRunningTask("install_" + req.Software) {
			c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "该软件正在安装中"})
			return
		}

		// Read sudo password
		sudoPass := ""
		if sp := getSudoPass(cfg); sp != "" {
			sudoPass = sp
		}

		var script string
		var taskName string

		switch req.Software {
		case "nodejs":
			taskName = "安装 Node.js"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 Node.js (v22 LTS)..."
$nodeCheck = Get-Command node -ErrorAction SilentlyContinue
if ($nodeCheck) {
  Write-Output "⚠️ Node.js 已安装: $(node --version)"
  Write-Output "如需更新，请从 https://nodejs.org 下载最新版"
  npm config set registry https://registry.npmmirror.com
  exit 0
}
# Check if winget is available
$wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
if ($wingetCheck) {
  Write-Output "📥 通过 winget 安装 Node.js..."
  winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements
} else {
  Write-Output "❌ 请从 https://nodejs.org 手动下载安装 Node.js"
  Write-Output "或安装 winget (App Installer) 后重试"
  exit 1
}
# Refresh PATH
$env:PATH = [System.Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("PATH","User")
npm config set registry https://registry.npmmirror.com
Write-Output "✅ Node.js $(node --version) 安装完成"
`
			} else {
				script = `
set -e
export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
echo "📦 安装 Node.js (v22 LTS)..."

# --- Helper: install Node.js v22 from official binary archive (Linux/macOS) ---
install_node_tarball() {
  local NODE_VER="v22.14.0"
  local OS_NAME
  local PKG_EXT
  local ARCH
  OS_NAME=$(uname | tr '[:upper:]' '[:lower:]')

  case "$(uname -m)" in
    x86_64|amd64) ARCH="x64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="armv7l" ;;
    *) echo "❌ 不支持的 CPU 架构: $(uname -m)"; exit 1 ;;
  esac

  if [ "$OS_NAME" = "darwin" ]; then
    if [ "$ARCH" = "armv7l" ]; then
      echo "❌ macOS 不支持 armv7l 架构"; exit 1
    fi
    PKG_EXT="tar.gz"
  else
    PKG_EXT="tar.xz"
  fi

  local FILE="node-${NODE_VER}-${OS_NAME}-${ARCH}.${PKG_EXT}"
  local URL="https://nodejs.org/dist/${NODE_VER}/${FILE}"
  local MIRROR_URL="https://npmmirror.com/mirrors/node/${NODE_VER}/${FILE}"
  echo "📥 下载 Node.js ${NODE_VER} (${OS_NAME}-${ARCH}) 官方二进制..."
  local TMP_DIR=$(mktemp -d)
  curl -fsSL "$MIRROR_URL" -o "$TMP_DIR/node.tar" 2>/dev/null || \
  curl -fsSL "$URL" -o "$TMP_DIR/node.tar" || {
    echo "❌ Node.js 下载失败"; rm -rf "$TMP_DIR"; exit 1
  }
  echo "📦 解压到 /usr/local ..."
  if [ "$PKG_EXT" = "tar.gz" ]; then
    tar -xzf "$TMP_DIR/node.tar" -C /usr/local --strip-components=1
  else
    tar -xJf "$TMP_DIR/node.tar" -C /usr/local --strip-components=1
  fi
  rm -rf "$TMP_DIR"
  hash -r
}

node_version_ok() {
  local ver=$(node --version 2>/dev/null | sed 's/^v//')
  [ -z "$ver" ] && return 1
  [ "$(echo "$ver" | cut -d. -f1)" -ge 20 ] 2>/dev/null
}

if node_version_ok; then
  echo "✅ Node.js $(node --version) 已安装且版本满足要求"
else
  if command -v node &>/dev/null; then
    echo "⚠️ 当前 Node.js $(node --version) 版本过低 (需要 >= 20)，正在升级..."
  fi

  if [ "$(uname)" = "Darwin" ]; then
    install_node_tarball
  else
    # Try NodeSource first
    DISTRO_FAMILY=""
    if [ -f /etc/os-release ]; then
      . /etc/os-release
      case "$ID $ID_LIKE" in
        *debian*|*ubuntu*) DISTRO_FAMILY="debian" ;;
        *rhel*|*centos*|*fedora*|*amzn*|*rocky*|*alma*) DISTRO_FAMILY="rhel" ;;
      esac
    fi
    [ -z "$DISTRO_FAMILY" ] && command -v apt-get &>/dev/null && DISTRO_FAMILY="debian"
    [ -z "$DISTRO_FAMILY" ] && (command -v dnf &>/dev/null || command -v yum &>/dev/null) && DISTRO_FAMILY="rhel"

    case "$DISTRO_FAMILY" in
      debian)
        echo "📥 尝试 NodeSource (deb)..."
        if curl -fsSL https://deb.nodesource.com/setup_22.x | bash - 2>/dev/null; then
          apt-get install -y nodejs || true
        fi ;;
      rhel)
        echo "📥 尝试 NodeSource (rpm)..."
        if curl -fsSL https://rpm.nodesource.com/setup_22.x | bash - 2>/dev/null; then
          if command -v dnf &>/dev/null; then dnf install -y nodejs || true
          else yum install -y nodejs || true; fi
        fi ;;
    esac

    # Fallback to binary tarball if still not good enough
    if ! node_version_ok; then
      echo "⚠️ 包管理器未能提供 Node.js >= 20，使用官方二进制安装..."
      install_node_tarball
    fi
  fi

  if ! node_version_ok; then
    echo "❌ Node.js >= 20 安装失败，请手动安装"; exit 1
  fi
fi

npm config set registry https://registry.npmmirror.com 2>/dev/null || true
echo "✅ Node.js $(node --version) 安装完成"
echo "✅ npm $(npm --version)"
`
			}
		case "docker":
			taskName = "安装 Docker"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 Docker Desktop..."
$dockerCheck = Get-Command docker -ErrorAction SilentlyContinue
if ($dockerCheck) {
  Write-Output "⚠️ Docker 已安装: $(docker --version)"
  exit 0
}
$wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
if ($wingetCheck) {
  Write-Output "📥 通过 winget 安装 Docker Desktop..."
  winget install Docker.DockerDesktop --accept-source-agreements --accept-package-agreements
  Write-Output "✅ Docker Desktop 已安装，请重启电脑后启动 Docker Desktop"
} else {
  Write-Output "❌ 请从 https://www.docker.com/products/docker-desktop 手动下载安装 Docker Desktop"
  exit 1
}
`
			} else {
				script = `
set -e
export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
export HOME="${HOME:-/var/root}"
echo "📦 安装 Docker..."
if command -v docker &>/dev/null; then
  echo "⚠️ Docker 已安装: $(docker --version)"
  exit 0
fi
if [ "$(uname)" = "Darwin" ]; then
  echo "⚠️ macOS 请手动安装 Docker Desktop: https://www.docker.com/products/docker-desktop"
  echo "或使用 Homebrew: brew install --cask docker"
  if command -v brew &>/dev/null; then
    if [ "$(id -u)" -eq 0 ]; then
      CONSOLE_USER=$(stat -f%Su /dev/console 2>/dev/null || true)
      if [ -n "$CONSOLE_USER" ] && [ "$CONSOLE_USER" != "root" ] && id "$CONSOLE_USER" &>/dev/null; then
        USER_HOME=$(dscl . -read "/Users/$CONSOLE_USER" NFSHomeDirectory 2>/dev/null | awk '{print $2}')
        [ -z "$USER_HOME" ] && USER_HOME="/Users/$CONSOLE_USER"
        echo "📥 检测到 root 环境，切换到用户 $CONSOLE_USER 执行 brew..."
        sudo -u "$CONSOLE_USER" HOME="$USER_HOME" PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin" brew install --cask docker
      else
        echo "❌ 未检测到可用登录用户，无法自动执行 brew 安装"
        exit 1
      fi
    else
      brew install --cask docker
    fi
    echo "✅ Docker Desktop 已安装，请从应用程序中启动 Docker"
  else
    exit 1
  fi
else
  DISTRO_FAMILY=""
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    case "$ID $ID_LIKE" in
      *debian*|*ubuntu*) DISTRO_FAMILY="debian" ;;
      *rhel*|*centos*|*fedora*|*amzn*|*rocky*|*alma*|*opencloudos*|*tencentos*) DISTRO_FAMILY="rhel" ;;
    esac
  fi
  [ -z "$DISTRO_FAMILY" ] && command -v apt-get &>/dev/null && DISTRO_FAMILY="debian"
  [ -z "$DISTRO_FAMILY" ] && (command -v dnf &>/dev/null || command -v yum &>/dev/null) && DISTRO_FAMILY="rhel"

  case "$DISTRO_FAMILY" in
    debian)
      echo "📥 通过阿里云镜像安装 Docker (deb)..."
      apt-get update
      apt-get install -y ca-certificates curl gnupg
      install -m 0755 -d /etc/apt/keyrings
      curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg 2>/dev/null || true
      chmod a+r /etc/apt/keyrings/docker.gpg
      echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME" || echo "jammy") stable" > /etc/apt/sources.list.d/docker.list
      apt-get update
      apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      ;;
    rhel)
      echo "📥 通过阿里云镜像安装 Docker (rpm)..."
      if command -v dnf &>/dev/null; then
        dnf install -y dnf-plugins-core
        dnf config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo 2>/dev/null || true
        dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      else
        yum install -y yum-utils
        yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
        yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      fi
      ;;
    *)
      echo "❌ 不支持的系统发行版，请手动安装 Docker"
      echo "   参考: https://docs.docker.com/engine/install/"
      exit 1
      ;;
  esac

  # Configure Docker mirror
  mkdir -p /etc/docker
  cat > /etc/docker/daemon.json << 'DOCKEREOF'
{
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me"
  ]
}
DOCKEREOF
  systemctl enable docker
  systemctl restart docker
fi
echo "✅ Docker $(docker --version) 安装完成"
`
			}
		case "git":
			taskName = "安装 Git"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 Git..."
$gitCheck = Get-Command git -ErrorAction SilentlyContinue
if ($gitCheck) {
  Write-Output "⚠️ Git 已安装: $(git --version)"
  exit 0
}
$wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
if ($wingetCheck) {
  Write-Output "📥 通过 winget 安装 Git..."
  winget install Git.Git --accept-source-agreements --accept-package-agreements
  Write-Output "✅ Git 安装完成，请重启终端使 git 命令生效"
} else {
  Write-Output "❌ 请从 https://git-scm.com/download/win 手动下载安装 Git"
  exit 1
}
`
			} else {
				script = `
set -e
echo "📦 安装 Git..."
if [ "$(uname)" = "Darwin" ]; then
  if command -v brew &>/dev/null; then
    brew install git
  else
    xcode-select --install 2>/dev/null || echo "Git 应该已通过 Xcode CLT 安装"
  fi
elif command -v apt-get &>/dev/null; then
  apt-get update
  apt-get install -y git
elif command -v yum &>/dev/null; then
  yum install -y git
else
  echo "❌ 不支持的包管理器，请手动安装 Git"
  exit 1
fi
echo "✅ $(git --version) 安装完成"
`
			}
		case "python":
			taskName = "安装 Python 3"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 Python 3..."
$pyCheck = Get-Command python -ErrorAction SilentlyContinue
if ($pyCheck) {
  Write-Output "⚠️ Python 已安装: $(python --version)"
  exit 0
}
$wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
if ($wingetCheck) {
  Write-Output "📥 通过 winget 安装 Python 3..."
  winget install Python.Python.3.12 --accept-source-agreements --accept-package-agreements
  Write-Output "✅ Python 安装完成，请重启终端"
} else {
  Write-Output "❌ 请从 https://www.python.org/downloads/ 手动下载安装 Python"
  exit 1
}
`
			} else {
				script = `
set -e
echo "📦 安装 Python 3..."
if [ "$(uname)" = "Darwin" ]; then
  if command -v brew &>/dev/null; then
    brew install python@3
  else
    echo "❌ macOS 请先安装 Homebrew 或从 python.org 下载"
    exit 1
  fi
elif command -v apt-get &>/dev/null; then
  apt-get update
  apt-get install -y python3 python3-pip python3-venv
elif command -v yum &>/dev/null; then
  yum install -y python3 python3-pip
else
  echo "❌ 不支持的包管理器，请手动安装 Python 3"
  exit 1
fi
# Set pip mirror
pip3 config set global.index-url https://pypi.tuna.tsinghua.edu.cn/simple 2>/dev/null || true
echo "✅ $(python3 --version) 安装完成"
`
			}
		case "openclaw":
			if err := ensureOpenClawManualPrerequisites(); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
				return
			}
			taskName = "安装 OpenClaw"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 OpenClaw..."
$baseDir = "C:\ClawPanel"

# ---- 前置环境检查：Windows 必须手动预装 Node.js 与 Git ----
$nodeCheck = Get-Command node -ErrorAction SilentlyContinue
if (-not $nodeCheck) {
  Write-Output "❌ 未检测到 Node.js。Windows 一键安装 OpenClaw 前请先手动安装 Node.js (>=20): https://nodejs.org"
  exit 1
}

$nodeVerRaw = & node --version 2>$null
$nodeMajor = -1
if ($nodeVerRaw -match '^v?(\d+)') {
  $nodeMajor = [int]$matches[1]
}
if ($nodeMajor -lt 20) {
  Write-Output "❌ 当前 Node.js 版本过低 ($nodeVerRaw)。请先手动升级到 Node.js >=20 后再安装 OpenClaw"
  exit 1
}

$gitCheck = Get-Command git -ErrorAction SilentlyContinue
if (-not $gitCheck) {
  Write-Output "❌ 未检测到 Git。Windows 一键安装 OpenClaw 前请先手动安装 Git: https://git-scm.com/download/win"
  exit 1
}

# ---- 修复服务环境下 Git 运行时路径缺失（git-submodule / sh）----
$gitExe = $gitCheck.Source
$gitDir = Split-Path -Parent $gitExe
$gitRoot = Split-Path -Parent $gitDir
$gitRootLeaf = (Split-Path -Leaf $gitRoot).ToLower()
if ($gitRootLeaf -eq "mingw64" -or $gitRootLeaf -eq "usr") {
  $gitRoot = Split-Path -Parent $gitRoot
}
$gitCmdDir = Join-Path $gitRoot "cmd"
$gitMingwBin = Join-Path $gitRoot "mingw64\\bin"
$gitUsrBin = Join-Path $gitRoot "usr\\bin"
$gitExecPath = Join-Path $gitRoot "mingw64\\libexec\\git-core"

$pathParts = @()
foreach ($p in @($gitCmdDir, $gitMingwBin, $gitUsrBin, $gitExecPath)) {
  if (Test-Path $p) { $pathParts += $p }
}
if ($pathParts.Count -gt 0) {
  $env:Path = (($pathParts -join ';') + ';' + $env:Path)
}
if (Test-Path $gitExecPath) {
  $env:GIT_EXEC_PATH = $gitExecPath
}

$gitSubmodulePath = Join-Path $gitExecPath "git-submodule"
if (-not (Test-Path $gitSubmodulePath)) {
  Write-Output "❌ Git 安装不完整（缺少 git-submodule），请重新安装 Git 后重试"
  exit 1
}

$gitRemoteHttps = Join-Path $gitExecPath "git-remote-https.exe"
if (-not (Test-Path $gitRemoteHttps)) {
  Write-Output "❌ Git 安装不完整（缺少 git-remote-https），请重新安装 Git 后重试"
  exit 1
}

# 某些 Git for Windows 发行版没有 sh.exe（只有 dash.exe/bash.exe），为 git-submodule 创建 shim
$shCmd = Get-Command sh -ErrorAction SilentlyContinue
if (-not $shCmd) {
  $gitShimDir = Join-Path $baseDir "git-shim"
  if (-not (Test-Path $gitShimDir)) {
    New-Item -ItemType Directory -Path $gitShimDir -Force | Out-Null
  }
  $shellExe = $null
  $dashCmd = Get-Command dash -ErrorAction SilentlyContinue
  if ($dashCmd) {
    $shellExe = $dashCmd.Source
  } else {
    $bashCmd = Get-Command bash -ErrorAction SilentlyContinue
    if ($bashCmd) { $shellExe = $bashCmd.Source }
  }
  if ($shellExe) {
    $shimPath = Join-Path $gitShimDir "sh.cmd"
    @"
@echo off
"$shellExe" %*
"@ | Set-Content -Path $shimPath -Encoding ASCII -Force
    $env:Path = "$gitShimDir;" + $env:Path
  }
}

if ((-not (Get-Command bash -ErrorAction SilentlyContinue)) -and (-not (Get-Command sh -ErrorAction SilentlyContinue))) {
  Write-Output "❌ Git 运行环境不完整（缺少 bash/sh），请重新安装 Git 并勾选命令行工具组件"
  exit 1
}

Write-Output "✅ 环境检查通过: Node.js $nodeVerRaw / $(& git --version)"

# Configure npm to use Chinese mirror for faster downloads
# ---- 修复 SYSTEM 账户路径与 Git SSH 权限问题 ----
$npmPrefix = Join-Path $baseDir "npm-global"
$npmCache = Join-Path $baseDir "npm-cache"
$homeDir = Join-Path $baseDir "home"
$openclawDir = Join-Path $baseDir "openclaw"
$workDir = Join-Path $baseDir "work"

foreach ($d in @($baseDir, $npmPrefix, $npmCache, $homeDir, $openclawDir, $workDir, (Join-Path $npmPrefix "node_modules"))) {
  if (-not (Test-Path $d)) {
    New-Item -ItemType Directory -Path $d -Force | Out-Null
  }
}

# 避免落到 C:\Windows\system32\config\systemprofile
$env:HOME = $homeDir
$env:USERPROFILE = $homeDir
$env:APPDATA = Join-Path $homeDir "AppData\\Roaming"
$env:LOCALAPPDATA = Join-Path $homeDir "AppData\\Local"
if (-not (Test-Path $env:APPDATA)) { New-Item -ItemType Directory -Path $env:APPDATA -Force | Out-Null }
if (-not (Test-Path $env:LOCALAPPDATA)) { New-Item -ItemType Directory -Path $env:LOCALAPPDATA -Force | Out-Null }

Write-Output "📝 配置 npm 镜像源与可写目录..."
& npm.cmd config set registry https://registry.npmmirror.com --global 2>$null
& npm.cmd config set prefix $npmPrefix --global 2>$null
& npm.cmd config set cache $npmCache --global 2>$null
Write-Output "📁 npm 全局安装目录: $npmPrefix"
Write-Output "📁 npm 缓存目录: $npmCache"

# GitHub 依赖强制走 HTTPS，避免 SYSTEM 账户缺少 SSH key 报错
git config --global --replace-all url."https://github.com/".insteadOf "ssh://git@github.com/" 2>$null
git config --global --add url."https://github.com/".insteadOf "git@github.com:" 2>$null

# 预检：确认 SSH 形式 GitHub URL 能被重写并访问
git --no-replace-objects ls-remote ssh://git@github.com/whiskeysockets/libsignal-node.git 1>$null 2>$null
if ($LASTEXITCODE -ne 0) {
  Write-Output "❌ GitHub 访问预检失败（git ls-remote）。请检查 Git 安装完整性与网络连接后重试"
  exit 1
}

# 清理 npm 可能残留的临时 git-clone 目录，避免“destination already exists”
$gitTmpRoot = Join-Path $npmCache "_cacache\\tmp"
if (Test-Path $gitTmpRoot) {
  Get-ChildItem -Path $gitTmpRoot -Filter "git-clone*" -Force -ErrorAction SilentlyContinue | ForEach-Object {
    Remove-Item -LiteralPath $_.FullName -Recurse -Force -ErrorAction SilentlyContinue
  }
}

Write-Output "📥 正在通过 npm 安装 OpenClaw..."
& npm.cmd install -g openclaw@latest --registry=https://registry.npmmirror.com --no-fund --no-audit 2>&1
if ($LASTEXITCODE -ne 0) {
  Write-Output "⚠️ 首次安装失败 (exit code: $LASTEXITCODE)，正在重试..."
  & npm.cmd cache verify 2>$null
  & npm.cmd install -g openclaw@latest --registry=https://registry.npmmirror.com --force --no-fund --no-audit 2>&1
  if ($LASTEXITCODE -ne 0) {
    Write-Output "❌ OpenClaw 安装失败，请检查网络连接或手动运行: npm install -g openclaw@latest"
    exit 1
  }
}

# Refresh PATH so we can find openclaw.cmd
$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
# Also add npm global bin to PATH
if ($npmPrefix -and (Test-Path $npmPrefix)) {
  $env:Path = "$npmPrefix;$env:Path"
}

# Verify installation
$openclawCmd = Join-Path $npmPrefix "openclaw.cmd"
if (-not (Test-Path $openclawCmd)) {
  $openclawCmd = "openclaw"
}
$ocVer = & $openclawCmd --version 2>$null
if ($LASTEXITCODE -eq 0 -and $ocVer) {
  Write-Output "✅ OpenClaw $ocVer 安装完成"
} else {
  Write-Output "⚠️ npm 安装完成但未能直接读取 openclaw 版本，继续初始化配置"
}

Write-Output "📝 初始化配置..."
$openclawConfig = Join-Path $openclawDir "openclaw.json"
if (-not (Test-Path $openclawConfig)) {
  Write-Output "📝 创建基础配置文件..."
  $ocVer = & $openclawCmd --version 2>$null
  if (-not $ocVer) { $ocVer = "2026.3.2" }
  @"
{
  "meta": {
    "version": "$ocVer",
    "lastTouchedVersion": "$ocVer"
  },
  "channels": {
    "qq": {
      "enabled": false
    }
  },
  "gateway": {
    "mode": "local",
    "port": 19000
  },
  "workspace": {
    "path": "$($workDir -replace '\\', '\\')"
  }
}
"@ | Set-Content $openclawConfig -Force
  Write-Output "✅ 配置文件已创建: $openclawConfig"
}

$env:OPENCLAW_DIR = $openclawDir
$env:OPENCLAW_STATE_DIR = $openclawDir
$env:OPENCLAW_CONFIG_PATH = $openclawConfig
& $openclawCmd init 2>$null

Write-Output "✅ 全部完成"
`
			} else {
				script = `
set -e
echo "📦 安装 OpenClaw..."
export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"

# --- Helper: install Node.js v22 from official binary archive (Linux/macOS) ---
install_node_tarball() {
  local NODE_VER="v22.14.0"
  local OS_NAME
  local PKG_EXT
  local ARCH
  OS_NAME=$(uname | tr '[:upper:]' '[:lower:]')

  case "$(uname -m)" in
    x86_64|amd64) ARCH="x64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="armv7l" ;;
    *) echo "❌ 不支持的 CPU 架构: $(uname -m)"; exit 1 ;;
  esac

  if [ "$OS_NAME" = "darwin" ]; then
    if [ "$ARCH" = "armv7l" ]; then
      echo "❌ macOS 不支持 armv7l 架构"; exit 1
    fi
    PKG_EXT="tar.gz"
  else
    PKG_EXT="tar.xz"
  fi

  local FILE="node-${NODE_VER}-${OS_NAME}-${ARCH}.${PKG_EXT}"
  local URL="https://nodejs.org/dist/${NODE_VER}/${FILE}"
  local MIRROR_URL="https://npmmirror.com/mirrors/node/${NODE_VER}/${FILE}"
  echo "📥 下载 Node.js ${NODE_VER} (${OS_NAME}-${ARCH}) 官方二进制..."
  local TMP_DIR=$(mktemp -d)
  # Try China mirror first, then official
  curl -fsSL "$MIRROR_URL" -o "$TMP_DIR/node.tar" 2>/dev/null || \
  curl -fsSL "$URL" -o "$TMP_DIR/node.tar" || {
    echo "❌ Node.js 下载失败"
    rm -rf "$TMP_DIR"
    exit 1
  }
  echo "📦 解压到 /usr/local ..."
  if [ "$PKG_EXT" = "tar.gz" ]; then
    tar -xzf "$TMP_DIR/node.tar" -C /usr/local --strip-components=1
  else
    tar -xJf "$TMP_DIR/node.tar" -C /usr/local --strip-components=1
  fi
  rm -rf "$TMP_DIR"
  hash -r
  echo "✅ Node.js $(node --version) 安装完成 (官方二进制)"
}

# --- Helper: check if node version >= 20 ---
node_version_ok() {
  local ver
  ver=$(node --version 2>/dev/null | sed 's/^v//')
  [ -z "$ver" ] && return 1
  local major
  major=$(echo "$ver" | cut -d. -f1)
  [ "$major" -ge 20 ] 2>/dev/null
}

# 1. Ensure prerequisites are preinstalled on macOS
if [ "$(uname)" = "Darwin" ]; then
  if ! node_version_ok; then
    if command -v node &>/dev/null; then
      echo "❌ 当前 Node.js $(node --version) 版本过低。macOS 一键安装 OpenClaw 前请先手动升级到 Node.js >= 20"
    else
      echo "❌ 未检测到 Node.js。macOS 一键安装 OpenClaw 前请先手动安装 Node.js >= 20: https://nodejs.org"
    fi
    exit 1
  fi
  if ! command -v git &>/dev/null; then
    echo "❌ 未检测到 Git。macOS 一键安装 OpenClaw 前请先手动安装 Git（Xcode Command Line Tools 或 brew install git）"
    exit 1
  fi
  echo "✅ 环境检查通过: Node.js $(node --version) / $(git --version)"
elif ! node_version_ok; then
  if command -v node &>/dev/null; then
    echo "⚠️ 当前 Node.js $(node --version) 版本过低 (需要 >= 20)，正在升级..."
  else
    echo "⚠️ 未检测到 Node.js，正在自动安装..."
  fi

  # Linux: Try NodeSource first, then fallback to binary tarball
  DISTRO_FAMILY=""
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    case "$ID $ID_LIKE" in
      *debian*|*ubuntu*) DISTRO_FAMILY="debian" ;;
      *rhel*|*centos*|*fedora*|*amzn*|*rocky*|*alma*) DISTRO_FAMILY="rhel" ;;
    esac
  fi
  [ -z "$DISTRO_FAMILY" ] && command -v apt-get &>/dev/null && DISTRO_FAMILY="debian"
  [ -z "$DISTRO_FAMILY" ] && (command -v dnf &>/dev/null || command -v yum &>/dev/null) && DISTRO_FAMILY="rhel"

  NODESOURCE_OK=false
  case "$DISTRO_FAMILY" in
    debian)
      echo "📥 尝试 NodeSource (deb)..."
      if curl -fsSL https://deb.nodesource.com/setup_22.x | bash - 2>/dev/null; then
        apt-get install -y nodejs && NODESOURCE_OK=true
      fi
      ;;
    rhel)
      echo "📥 尝试 NodeSource (rpm)..."
      if curl -fsSL https://rpm.nodesource.com/setup_22.x | bash - 2>/dev/null; then
        if command -v dnf &>/dev/null; then
          dnf install -y nodejs && NODESOURCE_OK=true
        else
          yum install -y nodejs && NODESOURCE_OK=true
        fi
      fi
      ;;
  esac

  # If NodeSource didn't work or gave old version, use binary tarball
  if ! node_version_ok; then
    echo "⚠️ 包管理器未能提供 Node.js >= 20，使用官方二进制安装..."
    install_node_tarball
  fi

  # Final check (Linux)
  if ! node_version_ok; then
    echo "❌ Node.js >= 20 安装失败，请手动安装后重试"
    echo "   参考: https://nodejs.org/en/download/"
    exit 1
  fi
fi
echo "✅ Node.js $(node --version) ready"

# 2. Ensure npm mirror is set
npm config set registry https://registry.npmmirror.com 2>/dev/null || true

# 3. Install OpenClaw
echo "📥 正在通过 npm 安装 OpenClaw..."
npm install -g openclaw@latest --registry=https://registry.npmmirror.com
echo "✅ OpenClaw $(openclaw --version 2>/dev/null || echo '已安装') 安装完成"

# 4. Initialize config
echo "📝 初始化配置..."
openclaw init 2>/dev/null || true

# 5. Install QQ plugin (OneBot11 channel) if not present
# Robust OpenClaw dir resolution for sudo/service environments:
# - avoid empty HOME -> /.openclaw
# - auto-fix legacy relative path (.openclaw)
OPENCLAW_DIR="${OPENCLAW_DIR:-}"
if [ -z "$OPENCLAW_DIR" ] || [ "$OPENCLAW_DIR" = "/" ] || [ "$OPENCLAW_DIR" = ".openclaw" ]; then
  if [ -n "${SUDO_USER:-}" ] && command -v dscl &>/dev/null; then
    UHOME=$(dscl . -read "/Users/${SUDO_USER}" NFSHomeDirectory 2>/dev/null | awk '{print $2}')
  fi
  if [ -z "${UHOME:-}" ]; then
    UHOME="${HOME:-/var/root}"
  fi
  OPENCLAW_DIR="${UHOME}/.openclaw"
fi
case "$OPENCLAW_DIR" in
  /*) ;;
  *) OPENCLAW_DIR="${HOME:-/var/root}/$OPENCLAW_DIR" ;;
esac
QQ_EXT_DIR="$OPENCLAW_DIR/extensions/qq"
if [ ! -d "$QQ_EXT_DIR" ]; then
  echo "📥 安装 QQ (OneBot11) 通道插件..."
  mkdir -p "$OPENCLAW_DIR/extensions"
  QQ_TGZ=$(mktemp)
  if curl -fsSL "http://39.102.53.188:16198/clawpanel/bin/qq-plugin.tgz" -o "$QQ_TGZ" 2>/dev/null && \
     tar xzf "$QQ_TGZ" -C "$OPENCLAW_DIR/extensions/" && \
     [ -d "$QQ_EXT_DIR" ]; then
    chown -R root:root "$QQ_EXT_DIR" 2>/dev/null || true
    echo "✅ QQ 插件安装完成"
  else
    echo "❌ QQ 个人号插件安装失败，无法继续配置 QQ 通道"
    rm -f "$QQ_TGZ"
    exit 1
  fi
  rm -f "$QQ_TGZ"
fi

if [ ! -d "$QQ_EXT_DIR" ]; then
  echo "❌ QQ 个人号插件未安装，无法继续安装 NapCat"
  exit 1
fi

# 6. Ensure gateway.mode=local and channels.qq in openclaw.json
OC_CFG="$OPENCLAW_DIR/openclaw.json"
if [ -f "$OC_CFG" ] && command -v python3 &>/dev/null; then
  python3 -c "
import json, sys
try:
    with open('$OC_CFG') as f:
        cfg = json.load(f)
except:
    cfg = {}
changed = False
# Ensure gateway.mode
gw = cfg.setdefault('gateway', {})
if gw.get('mode') != 'local':
    gw['mode'] = 'local'
    changed = True
# Ensure channels.qq (the QQ plugin reads wsUrl from channels.qq config)
ch = cfg.setdefault('channels', {})
qq = ch.setdefault('qq', {})
if not qq.get('wsUrl'):
    qq['wsUrl'] = 'ws://127.0.0.1:3001'
    changed = True
if 'enabled' not in qq:
    qq['enabled'] = True
    changed = True
# Ensure plugins.entries.qq
pl = cfg.setdefault('plugins', {})
ent = pl.setdefault('entries', {})
if 'qq' not in ent:
    ent['qq'] = {'enabled': True}
    changed = True
# Ensure plugins.installs.qq
ins = pl.setdefault('installs', {})
if 'qq' not in ins:
    ins['qq'] = {'installPath': '$QQ_EXT_DIR', 'source': 'archive', 'version': '1.0.0'}
    changed = True
if changed:
    with open('$OC_CFG', 'w') as f:
        json.dump(cfg, f, indent=2)
    print('✅ openclaw.json 配置已更新')
else:
    print('✅ openclaw.json 配置无需更新')
" 2>/dev/null || echo "⚠️ 配置自动更新跳过"
fi

echo "✅ 全部完成"
`
			}
		case "napcat":
			taskName = "安装 NapCat (QQ个人号)"
			if runtime.GOOS == "windows" {
				script = buildNapCatWindowsInstallScript(cfg)
			} else {
				script = buildNapCatInstallScript(cfg)
			}

		case "wechat":
			taskName = "安装微信机器人"
			script = buildWeChatInstallScript(cfg)

		default:
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "不支持的软件: " + req.Software})
			return
		}

		task := tm.CreateTask(taskName, "install_"+req.Software)

		go func() {
			var err error
			if sudoPass != "" && runtime.GOOS != "windows" {
				// Linux/macOS installs need sudo (including OpenClaw which auto-installs Node.js)
				err = tm.RunScriptWithSudo(task, sudoPass, script)
			} else {
				err = tm.RunScript(task, script)
			}
			tm.FinishTask(task, err)
		}()

		c.JSON(http.StatusOK, gin.H{"ok": true, "taskId": task.ID})
	}
}

// GetTasks 获取任务列表
func GetTasks(tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tasks := tm.GetRecentTasks()
		c.JSON(http.StatusOK, gin.H{"ok": true, "tasks": tasks})
	}
}

// GetTaskDetail 获取任务详情
func GetTaskDetail(tm *taskman.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		task := tm.GetTask(id)
		if task == nil {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "任务不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "task": task})
	}
}

func getSudoPass(cfg *config.Config) string {
	spPath := filepath.Join(cfg.DataDir, "sudo-password.txt")
	data, err := os.ReadFile(spPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func buildNapCatInstallScript(cfg *config.Config) string {
	_, wsToken, _ := cfg.ReadQQChannelState()
	wsTokenB64 := base64.StdEncoding.EncodeToString([]byte(wsToken))

	return fmt.Sprintf(`
set -e
export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
echo "📦 安装 NapCat (QQ个人号) Docker 容器..."

# Auto-install Docker if missing
if ! command -v docker &>/dev/null; then
  echo "⚠️ 未检测到 Docker，正在自动安装..."
  DISTRO_FAMILY=""
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    case "$ID $ID_LIKE" in
      *debian*|*ubuntu*) DISTRO_FAMILY="debian" ;;
      *rhel*|*centos*|*fedora*|*amzn*|*rocky*|*alma*|*opencloudos*|*tencentos*) DISTRO_FAMILY="rhel" ;;
    esac
  fi
  [ -z "$DISTRO_FAMILY" ] && command -v apt-get &>/dev/null && DISTRO_FAMILY="debian"
  [ -z "$DISTRO_FAMILY" ] && (command -v dnf &>/dev/null || command -v yum &>/dev/null) && DISTRO_FAMILY="rhel"

  case "$DISTRO_FAMILY" in
    debian)
      echo "📥 通过阿里云镜像安装 Docker (deb)..."
      apt-get update
      apt-get install -y ca-certificates curl gnupg
      install -m 0755 -d /etc/apt/keyrings
      curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg 2>/dev/null || true
      chmod a+r /etc/apt/keyrings/docker.gpg
      echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://mirrors.aliyun.com/docker-ce/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME" || echo "jammy") stable" > /etc/apt/sources.list.d/docker.list
      apt-get update
      apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      ;;
    rhel)
      echo "📥 通过阿里云镜像安装 Docker (rpm)..."
      if command -v dnf &>/dev/null; then
        dnf install -y dnf-plugins-core
        dnf config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo 2>/dev/null || true
        dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      else
        yum install -y yum-utils
        yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
        yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      fi
      ;;
    *)
      echo "❌ 不支持的系统发行版，请手动安装 Docker 后重试"
      echo "   参考: https://docs.docker.com/engine/install/"
      exit 1
      ;;
  esac

  systemctl enable docker
  systemctl start docker
  echo "✅ Docker $(docker --version) 安装完成"
fi

if ! docker info &>/dev/null; then
  echo "⚠️ Docker 服务未运行，正在启动..."
  if [ "$(uname)" = "Darwin" ]; then
    open -a Docker 2>/dev/null || true
    for i in $(seq 1 30); do
      if docker info &>/dev/null; then
        break
      fi
      sleep 2
    done
  else
    systemctl start docker
    sleep 2
  fi
fi

# Configure Docker mirror (Linux only)
if [ "$(uname)" = "Darwin" ]; then
  echo "ℹ️ macOS 跳过 /etc/docker/daemon.json 镜像配置"
else
  if [ ! -f /etc/docker/daemon.json ] || ! grep -q "registry-mirrors" /etc/docker/daemon.json 2>/dev/null; then
    echo "🔧 配置 Docker 镜像加速器..."
    mkdir -p /etc/docker
    cat > /etc/docker/daemon.json << 'DOCKEREOF'
{
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me"
  ]
}
DOCKEREOF
    systemctl daemon-reload
    systemctl restart docker
    sleep 2
  fi
fi

if ! docker info &>/dev/null; then
  echo "❌ Docker 服务仍未就绪，请手动启动 Docker Desktop 后重试"
  exit 1
fi

# Check if already exists
if docker inspect openclaw-qq &>/dev/null; then
  echo "⚠️ openclaw-qq 容器已存在，正在重新创建..."
  docker stop openclaw-qq 2>/dev/null || true
  docker rm openclaw-qq 2>/dev/null || true
fi

echo "📥 拉取 NapCat 镜像..."
docker pull mlikiowa/napcat-docker:latest

# Detect existing logged-in account for quick re-login
ACCOUNT_ARG=""
if docker inspect openclaw-qq &>/dev/null; then
  PREV_ACCOUNT=$(docker inspect openclaw-qq --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null | grep '^ACCOUNT=' | cut -d= -f2)
  if [ -n "$PREV_ACCOUNT" ]; then
    ACCOUNT_ARG="-e ACCOUNT=$PREV_ACCOUNT"
    echo "� 使用快速登录账号: $PREV_ACCOUNT"
  fi
fi

echo "� 创建容器..."
docker run -d \
  --name openclaw-qq \
  --restart unless-stopped \
  -p 3000:3000 \
  -p 3001:3001 \
  -p 6099:6099 \
  -e NAPCAT_GID=0 \
  -e NAPCAT_UID=0 \
  -e WEBUI_TOKEN=clawpanel-qq \
  $ACCOUNT_ARG \
  -v napcat-qq-session:/app/.config/QQ \
  -v napcat-config:/app/napcat/config \
  -v %s:/root/.openclaw:rw \
  -v %s:/root/openclaw/work:rw \
  mlikiowa/napcat-docker:latest

echo "⏳ 等待容器启动..."
sleep 5

# Configure OneBot11 WebSocket + HTTP
echo "🔧 配置 OneBot11 (WS + HTTP)..."
export WS_TOKEN_B64=%q
WS_TOKEN_JSON=$(python3 - <<'PY'
import base64, json, os
token = base64.b64decode(os.environ.get("WS_TOKEN_B64", "")).decode("utf-8")
print(json.dumps(token), end="")
PY
)

docker exec openclaw-qq bash -c "cat > /app/napcat/config/onebot11.json << OBEOF
{
  \"network\": {
    \"websocketServers\": [{
      \"name\": \"ws-server\",
      \"enable\": true,
      \"host\": \"0.0.0.0\",
      \"port\": 3001,
      \"token\": ${WS_TOKEN_JSON},
      \"reportSelfMessage\": true,
      \"enableForcePushEvent\": true,
      \"messagePostFormat\": \"array\",
      \"debug\": false,
      \"heartInterval\": 30000
    }],
    \"httpServers\": [{
      \"name\": \"http-api\",
      \"enable\": true,
      \"host\": \"0.0.0.0\",
      \"port\": 3000,
      \"token\": \"\"
    }],
    \"httpSseServers\": [],
    \"httpClients\": [],
    \"websocketClients\": [],
    \"plugins\": []
  },
  \"musicSignUrl\": \"\",
  \"enableLocalFile2Url\": true,
  \"parseMultMsg\": true,
  \"imageDownloadProxy\": \"\"
}
OBEOF"

# Configure WebUI
docker exec openclaw-qq bash -c 'cat > /app/napcat/config/webui.json << WUEOF
{
  "host": "0.0.0.0",
  "port": 6099,
  "token": "clawpanel-qq",
  "loginRate": 3
}
WUEOF'

echo "🔄 重启容器使配置生效..."
docker restart openclaw-qq

echo "⏳ 等待 NapCat 服务启动..."
for i in $(seq 1 30); do
  if curl -s -o /dev/null -w '' http://127.0.0.1:6099 2>/dev/null; then
    echo "✅ NapCat WebUI (6099) 已就绪"
    break
  fi
  sleep 2
done

# Verify ports
WS_OK=false
HTTP_OK=false
for i in $(seq 1 10); do
  if bash -c 'echo > /dev/tcp/127.0.0.1/3001' 2>/dev/null; then WS_OK=true; fi
  if bash -c 'echo > /dev/tcp/127.0.0.1/3000' 2>/dev/null; then HTTP_OK=true; fi
  if $WS_OK && $HTTP_OK; then break; fi
  sleep 2
done

if $WS_OK; then
  echo "✅ OneBot11 WebSocket (3001) 已就绪"
else
  echo "⚠️ OneBot11 WebSocket (3001) 尚未就绪，可能需要先扫码登录QQ"
fi
if $HTTP_OK; then
  echo "✅ OneBot11 HTTP API (3000) 已就绪"
else
  echo "⚠️ OneBot11 HTTP API (3000) 尚未就绪，可能需要先扫码登录QQ"
fi

echo "✅ NapCat (QQ个人号) 安装完成"
echo "📝 请在通道管理中扫码登录 QQ"
`, cfg.OpenClawDir, cfg.OpenClawWork, wsTokenB64)
}

// getNapCatShellDir returns the NapCat Shell installation directory on Windows, or "" if not found
func getNapCatShellDir(cfg *config.Config) string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(cfg.DataDir, "napcat"),
		filepath.Join(home, "NapCat"),
		filepath.Join(home, "Desktop", "NapCat"),
		`C:\NapCat`,
		filepath.Join(home, "AppData", "Local", "NapCat"),
	}
	markers := []string{"napcat.bat", "NapCatWinBootMain.exe"}
	for _, dir := range candidates {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		// Recursive walk to find napcat.bat or NapCatWinBootMain.exe anywhere under candidate
		found := ""
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || found != "" {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				for _, m := range markers {
					if strings.EqualFold(d.Name(), m) {
						found = filepath.Dir(path)
						return filepath.SkipAll
					}
				}
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	return ""
}

// isNapCatShellRunning checks if a NapCat Shell process is running on Windows
func isNapCatShellRunning() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	out := detectCmd("tasklist", "/FI", "IMAGENAME eq NapCatWinBootMain.exe", "/NH")
	if strings.Contains(out, "NapCatWinBootMain") {
		return true
	}
	// Also check for napcat.exe process
	out2 := detectCmd("tasklist", "/FI", "IMAGENAME eq napcat.exe", "/NH")
	return strings.Contains(out2, "napcat.exe")
}

// buildNapCatWindowsInstallScript builds a PowerShell script to install NapCat Shell on Windows
func buildNapCatWindowsInstallScript(cfg *config.Config) string {
	installDir := filepath.Join(cfg.DataDir, "napcat")
	return fmt.Sprintf(`
$ErrorActionPreference = "Stop"
Write-Output "📦 安装 NapCat (QQ个人号) Windows Shell 版本..."
Write-Output "无需 Docker，直接运行 NapCat Shell"

$INSTALL_DIR = "%s"
$OPENCLAW_DIR = "%s"
$QQ_PLUGIN_DIR = Join-Path $OPENCLAW_DIR "extensions\qq"
if (-not (Test-Path $INSTALL_DIR)) {
  New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
}

if (-not (Test-Path $QQ_PLUGIN_DIR)) {
  Write-Output "📥 安装 QQ (OneBot11) 通道插件..."
  New-Item -ItemType Directory -Force -Path (Split-Path $QQ_PLUGIN_DIR -Parent) | Out-Null
  $QQ_TGZ = Join-Path $env:TEMP "qq-plugin.tgz"
  try {
    Invoke-WebRequest -Uri "http://39.102.53.188:16198/clawpanel/bin/qq-plugin.tgz" -OutFile $QQ_TGZ -UseBasicParsing -TimeoutSec 60
    tar -xzf $QQ_TGZ -C (Split-Path $QQ_PLUGIN_DIR -Parent)
  } catch {
    Write-Output "❌ QQ 个人号插件安装失败，无法继续安装 NapCat"
    exit 1
  } finally {
    Remove-Item -Force $QQ_TGZ -ErrorAction SilentlyContinue
  }
}

if (-not (Test-Path $QQ_PLUGIN_DIR)) {
  Write-Output "❌ QQ 个人号插件未安装，无法继续安装 NapCat"
  exit 1
}

Write-Output "📥 下载 NapCat Shell Windows OneKey 版..."
$NAPCAT_ZIP = Join-Path $INSTALL_DIR "napcat-onekey.zip"
$urls = @(
  "https://github.com/NapNeko/NapCatQQ/releases/latest/download/NapCat.Shell.Windows.OneKey.zip",
  "https://ghfast.top/https://github.com/NapNeko/NapCatQQ/releases/latest/download/NapCat.Shell.Windows.OneKey.zip"
)
$downloaded = $false
foreach ($url in $urls) {
  try {
    Write-Output "尝试下载: $url"
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $url -OutFile $NAPCAT_ZIP -UseBasicParsing -TimeoutSec 120
    $downloaded = $true
    Write-Output "✅ 下载完成"
    break
  } catch {
    Write-Output "⚠️ 下载失败: $_"
  }
}
if (-not $downloaded) {
  Write-Output "❌ 所有下载源均失败，请手动从 GitHub 下载 NapCat.Shell.Windows.OneKey.zip"
  exit 1
}

Write-Output "📦 解压 NapCat Shell..."
Expand-Archive -Force -Path $NAPCAT_ZIP -DestinationPath $INSTALL_DIR
Remove-Item -Force $NAPCAT_ZIP -ErrorAction SilentlyContinue

# Find the NapCat Shell directory (could be nested)
$shellDir = ""
$batFiles = Get-ChildItem -Path $INSTALL_DIR -Recurse -Filter "napcat.bat" -ErrorAction SilentlyContinue
if ($batFiles) {
  $shellDir = $batFiles[0].DirectoryName
} else {
  $exeFiles = Get-ChildItem -Path $INSTALL_DIR -Recurse -Filter "NapCatWinBootMain.exe" -ErrorAction SilentlyContinue
  if ($exeFiles) {
    $shellDir = $exeFiles[0].DirectoryName
  }
}

if ($shellDir -ne "") {
  Write-Output "🔧 配置 OneBot11 (WS + HTTP)..."
  $configDir = Join-Path $shellDir "config"
  if (-not (Test-Path $configDir)) {
    New-Item -ItemType Directory -Force -Path $configDir | Out-Null
  }

  $onebot11 = @'
{
  "network": {
    "websocketServers": [{
      "name": "ws-server",
      "enable": true,
      "host": "0.0.0.0",
      "port": 3001,
      "token": "",
      "reportSelfMessage": true,
      "enableForcePushEvent": true,
      "messagePostFormat": "array",
      "debug": false,
      "heartInterval": 30000
    }],
    "httpServers": [{
      "name": "http-api",
      "enable": true,
      "host": "0.0.0.0",
      "port": 3000,
      "token": ""
    }],
    "httpSseServers": [],
    "httpClients": [],
    "websocketClients": [],
    "plugins": []
  },
  "musicSignUrl": "",
  "enableLocalFile2Url": true,
  "parseMultMsg": true,
  "imageDownloadProxy": ""
}
'@
  $onebot11 | Set-Content -Path (Join-Path $configDir "onebot11.json") -Encoding UTF8

  $webui = @'
{
  "host": "0.0.0.0",
  "port": 6099,
  "token": "clawpanel-qq",
  "loginRate": 3
}
'@
  $webui | Set-Content -Path (Join-Path $configDir "webui.json") -Encoding UTF8

  # Start NapCat Shell process
  Write-Output "🚀 启动 NapCat Shell..."
  $batFile = Join-Path $shellDir "napcat.bat"
  $exeFile = Join-Path $shellDir "NapCatWinBootMain.exe"
  if (Test-Path $batFile) {
    $q = [char]34
    $batArgs = "/c " + $q + $batFile + $q
    Start-Process -FilePath "cmd.exe" -ArgumentList $batArgs -WorkingDirectory $shellDir -WindowStyle Hidden
    Write-Output "✅ NapCat Shell 已通过 napcat.bat 启动"
  } elseif (Test-Path $exeFile) {
    Start-Process -FilePath $exeFile -WorkingDirectory $shellDir -WindowStyle Hidden
    Write-Output "✅ NapCat Shell 已通过 NapCatWinBootMain.exe 启动"
  } else {
    Write-Output "⚠️ 未找到启动文件，请手动启动 NapCat Shell"
  }
  Start-Sleep -Seconds 3

  Write-Output "✅ NapCat Shell (Windows) 安装完成"
  Write-Output "📁 安装目录: $shellDir"
  Write-Output "📝 请在通道管理中配置 QQ 并扫码登录"
} else {
  Write-Output "⚠️ NapCat 已下载但未找到启动文件，请手动检查: $INSTALL_DIR"
}
`, installDir, cfg.OpenClawDir)
}

func buildWeChatInstallScript(cfg *config.Config) string {
	return `
set -e
echo "📦 安装微信机器人 Docker 容器..."

if ! command -v docker &>/dev/null; then
  echo "❌ 需要先安装 Docker"
  exit 1
fi

# Check if already exists
if docker inspect openclaw-wechat &>/dev/null; then
  echo "⚠️ openclaw-wechat 容器已存在，正在重新创建..."
  docker stop openclaw-wechat 2>/dev/null || true
  docker rm openclaw-wechat 2>/dev/null || true
fi

echo "📥 拉取 wechatbot-webhook 镜像..."
docker pull dannicool/docker-wechatbot-webhook:latest

echo "🔧 创建容器..."
docker run -d \
  --name openclaw-wechat \
  --restart unless-stopped \
  -p 3002:3001 \
  -e LOGIN_API_TOKEN=clawpanel-wechat \
  -e RECVD_MSG_API=http://host.docker.internal:19527/api/wechat/callback \
  -e ACCEPT_RECVD_MSG_MYSELF=false \
  -e LOG_LEVEL=info \
  -v wechat-data:/app/data \
  --add-host=host.docker.internal:host-gateway \
  dannicool/docker-wechatbot-webhook:latest

echo "⏳ 等待容器启动..."
sleep 3

echo "✅ 微信机器人安装完成"
echo "📝 请在通道管理中配置微信并扫码登录"
`
}
