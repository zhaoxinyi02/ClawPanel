package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	Status      string `json:"status"` // installed, not_installed, running, stopped
	Category    string `json:"category"` // runtime, container, service
	Installable bool   `json:"installable"`
	Icon        string `json:"icon,omitempty"`
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
	currentPath := os.Getenv("PATH")
	if runtime.GOOS == "windows" {
		// Windows: add npm global bin path
		home, _ := os.UserHomeDir()
		extraPaths := strings.Join([]string{
			filepath.Join(home, "AppData", "Roaming", "npm"),
			filepath.Join(home, ".local", "bin"),
			`C:\Program Files\nodejs`,
		}, ";")
		cmd.Env = append(os.Environ(), "PATH="+currentPath+";"+extraPaths)
	} else {
		home, _ := os.UserHomeDir()
		extraPaths := "/usr/local/bin:/usr/bin:/bin:/snap/bin"
		extraPaths += ":" + filepath.Join(home, ".local", "bin")
		extraPaths += ":" + filepath.Join(home, ".npm-global", "bin")
		if runtime.GOOS == "darwin" {
			extraPaths += ":/opt/homebrew/bin:/opt/homebrew/sbin"
		}
		cmd.Env = append(os.Environ(), "PATH="+currentPath+":"+extraPaths)
	}
	out, err := cmd.Output()
	if err != nil {
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

func detectOpenClawVersion(cfg *config.Config) string {
	// 1. Try reading from cfg.OpenClawApp FIRST (most reliable for SYSTEM service)
	if cfg.OpenClawApp != "" {
		pkgPath := filepath.Join(cfg.OpenClawApp, "package.json")
		if v := readVersionFromPackageJSON(pkgPath); v != "" {
			return v
		}
	}
	
	// 2. Try npm global package.json (may fail when running as SYSTEM service)
	npmRoot := detectCmd("npm", "root", "-g")
	if npmRoot != "" {
		pkgPath := filepath.Join(npmRoot, "openclaw", "package.json")
		if v := readVersionFromPackageJSON(pkgPath); v != "" {
			return v
		}
	}

	// 3. Try openclaw CLI (may not work when running as SYSTEM service)
	ver := detectCmd("openclaw", "--version")
	if ver != "" {
		return strings.TrimPrefix(strings.TrimSpace(ver), "v")
	}

	// 4. Try from config meta.lastTouchedVersion
	ocConfig, _ := cfg.ReadOpenClawJSON()
	if ocConfig != nil {
		if meta, ok := ocConfig["meta"].(map[string]interface{}); ok {
			if v, ok := meta["lastTouchedVersion"].(string); ok && v != "" {
				return v
			}
		}
	}

	// 4. Try common binary paths
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

	// 5. Try source installs: check common directories for package.json
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

	// 6. Try Docker container
	dockerVer := detectCmd("docker", "exec", "openclaw", "openclaw", "--version")
	if dockerVer != "" {
		return strings.TrimPrefix(strings.TrimSpace(dockerVer), "v")
	}

	// 7. Try systemd: parse ExecStart from service file to find binary path
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

	// 8. Config file exists but no version extractable
	if ocConfig != nil {
		return "installed"
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
		if runtime.GOOS == "windows" {
			npmPath = detectCmd("where", "openclaw")
			// 'where' may return multiple lines, take the first
			if idx := strings.Index(npmPath, "\n"); idx > 0 {
				npmPath = strings.TrimSpace(npmPath[:idx])
			}
		} else {
			npmPath = detectCmd("which", "openclaw")
		}
		if npmPath != "" {
			ver := detectCmd("openclaw", "--version")
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
						Status: func() string { if running { return "running" }; return "stopped" }(),
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

# --- Helper: install Node.js v22 from official binary tarball ---
install_node_tarball() {
  local NODE_VER="v22.14.0"
  local ARCH
  case "$(uname -m)" in
    x86_64|amd64) ARCH="x64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="armv7l" ;;
    *) echo "❌ 不支持的 CPU 架构: $(uname -m)"; exit 1 ;;
  esac
  local URL="https://nodejs.org/dist/${NODE_VER}/node-${NODE_VER}-linux-${ARCH}.tar.xz"
  local MIRROR_URL="https://npmmirror.com/mirrors/node/${NODE_VER}/node-${NODE_VER}-linux-${ARCH}.tar.xz"
  echo "📥 下载 Node.js ${NODE_VER} (${ARCH}) 官方二进制..."
  local TMP_DIR=$(mktemp -d)
  curl -fsSL "$MIRROR_URL" -o "$TMP_DIR/node.tar.xz" 2>/dev/null || \
  curl -fsSL "$URL" -o "$TMP_DIR/node.tar.xz" || {
    echo "❌ Node.js 下载失败"; rm -rf "$TMP_DIR"; exit 1
  }
  echo "📦 解压到 /usr/local ..."
  tar -xJf "$TMP_DIR/node.tar.xz" -C /usr/local --strip-components=1
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
    if ! command -v brew &>/dev/null; then
      echo "❌ macOS 需要先安装 Homebrew: https://brew.sh"; exit 1
    fi
    brew install node@22 || brew upgrade node@22 || true
    brew link --overwrite node@22 || true
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
echo "📦 安装 Docker..."
if command -v docker &>/dev/null; then
  echo "⚠️ Docker 已安装: $(docker --version)"
  exit 0
fi
if [ "$(uname)" = "Darwin" ]; then
  echo "⚠️ macOS 请手动安装 Docker Desktop: https://www.docker.com/products/docker-desktop"
  echo "或使用 Homebrew: brew install --cask docker"
  if command -v brew &>/dev/null; then
    brew install --cask docker
    echo "✅ Docker Desktop 已安装，请从应用程序中启动"
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
			taskName = "安装 OpenClaw"
			if runtime.GOOS == "windows" {
				script = `
$ErrorActionPreference = "Continue"
Write-Output "📦 安装 OpenClaw..."

# ---- 检查并自动安装 Node.js ----
$nodeCheck = Get-Command node -ErrorAction SilentlyContinue
if (-not $nodeCheck) {
  Write-Output "⚠️ 未检测到 Node.js，正在自动安装..."
  $wingetCheck = Get-Command winget -ErrorAction SilentlyContinue
  if ($wingetCheck) {
    Write-Output "📥 通过 winget 安装 Node.js LTS..."
    winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements --silent 2>&1
    # 刷新 PATH
    $env:PATH = [System.Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("PATH","User")
    $nodeCheck = Get-Command node -ErrorAction SilentlyContinue
    if (-not $nodeCheck) {
      Write-Output "❌ Node.js 自动安装失败，请手动从 https://nodejs.org 下载安装后重试"
      exit 1
    }
    Write-Output "✅ Node.js $(node --version) 安装完成"
  } else {
    Write-Output "❌ 未找到 winget，请手动从 https://nodejs.org 下载安装 Node.js 后重试"
    exit 1
  }
}

# Configure npm to use Chinese mirror for faster downloads
Write-Output "📝 配置 npm 镜像源..."
npm config set registry https://registry.npmmirror.com 2>$null

# Ensure npm global prefix is set to user-accessible path
$npmPrefix = npm config get prefix 2>$null
Write-Output "📁 npm 全局安装目录: $npmPrefix"

Write-Output "📥 正在通过 npm 安装 OpenClaw..."
npm install -g openclaw@latest --registry=https://registry.npmmirror.com 2>&1
if ($LASTEXITCODE -ne 0) {
  Write-Output "⚠️ 首次安装失败 (exit code: $LASTEXITCODE)，正在重试..."
  # Retry with force flag
  npm install -g openclaw@latest --registry=https://registry.npmmirror.com --force 2>&1
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
$ocCmd = Get-Command openclaw -ErrorAction SilentlyContinue
if ($ocCmd) {
  $ocVer = & openclaw --version 2>$null
  Write-Output "✅ OpenClaw $ocVer 安装完成"
} else {
  # Check common locations
  $possiblePaths = @(
    (Join-Path $npmPrefix "openclaw.cmd"),
    (Join-Path $env:APPDATA "npm\openclaw.cmd"),
    "C:\Program Files\nodejs\openclaw.cmd"
  )
  $found = $false
  foreach ($p in $possiblePaths) {
    if (Test-Path $p) {
      Write-Output "✅ OpenClaw 安装完成 (位置: $p)"
      $found = $true
      break
    }
  }
  if (-not $found) {
    Write-Output "⚠️ npm 安装完成但未找到 openclaw 命令，可能需要重启面板"
  }
}

Write-Output "📝 初始化配置..."
try { openclaw init 2>$null } catch { Write-Output "初始化跳过（可能已存在配置）" }
Write-Output "✅ 全部完成"
`
			} else {
				script = `
set -e
echo "📦 安装 OpenClaw..."
export PATH="/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"

# --- Helper: install Node.js v22 from official binary tarball (works on ANY Linux) ---
install_node_tarball() {
  local NODE_VER="v22.14.0"
  local ARCH
  case "$(uname -m)" in
    x86_64|amd64) ARCH="x64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="armv7l" ;;
    *) echo "❌ 不支持的 CPU 架构: $(uname -m)"; exit 1 ;;
  esac
  local URL="https://nodejs.org/dist/${NODE_VER}/node-${NODE_VER}-linux-${ARCH}.tar.xz"
  local MIRROR_URL="https://npmmirror.com/mirrors/node/${NODE_VER}/node-${NODE_VER}-linux-${ARCH}.tar.xz"
  echo "📥 下载 Node.js ${NODE_VER} (${ARCH}) 官方二进制..."
  local TMP_DIR=$(mktemp -d)
  # Try China mirror first, then official
  curl -fsSL "$MIRROR_URL" -o "$TMP_DIR/node.tar.xz" 2>/dev/null || \
  curl -fsSL "$URL" -o "$TMP_DIR/node.tar.xz" || {
    echo "❌ Node.js 下载失败"
    rm -rf "$TMP_DIR"
    exit 1
  }
  echo "📦 解压到 /usr/local ..."
  tar -xJf "$TMP_DIR/node.tar.xz" -C /usr/local --strip-components=1
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

# 1. Ensure Node.js >= 20 is available
if ! node_version_ok; then
  if command -v node &>/dev/null; then
    echo "⚠️ 当前 Node.js $(node --version) 版本过低 (需要 >= 20)，正在升级..."
  else
    echo "⚠️ 未检测到 Node.js，正在自动安装..."
  fi

  if [ "$(uname)" = "Darwin" ]; then
    if ! command -v brew &>/dev/null; then
      echo "❌ macOS 需要先安装 Homebrew: https://brew.sh"
      exit 1
    fi
    brew install node@22 || brew upgrade node@22 || true
    brew link --overwrite node@22 || true
  else
    # Try NodeSource first, then fallback to binary tarball
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
  fi

  # Final check
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
OPENCLAW_DIR="${OPENCLAW_DIR:-$HOME/.openclaw}"
QQ_EXT_DIR="$OPENCLAW_DIR/extensions/qq"
if [ ! -d "$QQ_EXT_DIR" ]; then
  echo "📥 安装 QQ (OneBot11) 通道插件..."
  mkdir -p "$OPENCLAW_DIR/extensions"
  QQ_TGZ=$(mktemp)
  curl -fsSL "http://39.102.53.188:16198/clawpanel/bin/qq-plugin.tgz" -o "$QQ_TGZ" 2>/dev/null && \
  tar xzf "$QQ_TGZ" -C "$OPENCLAW_DIR/extensions/" && \
  chown -R root:root "$QQ_EXT_DIR" 2>/dev/null && \
  echo "✅ QQ 插件安装完成" || echo "⚠️ QQ 插件安装失败（可稍后手动安装）"
  rm -f "$QQ_TGZ"
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
  systemctl start docker
  sleep 2
fi

# Configure Docker mirror (always ensure, even if Docker was pre-installed)
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

# Check if already exists
if docker inspect openclaw-qq &>/dev/null; then
  echo "⚠️ openclaw-qq 容器已存在，正在重新创建..."
  docker stop openclaw-qq 2>/dev/null || true
  docker rm openclaw-qq 2>/dev/null || true
fi

echo "📥 拉取 NapCat 镜像..."
docker pull mlikiowa/napcat-docker:latest

echo "🔧 创建容器..."
docker run -d \
  --name openclaw-qq \
  --restart unless-stopped \
  -p 3000:3000 \
  -p 3001:3001 \
  -p 6099:6099 \
  -e NAPCAT_GID=0 \
  -e NAPCAT_UID=0 \
  -e WEBUI_TOKEN=clawpanel-qq \
  -v napcat-qq-session:/app/.config/QQ \
  -v napcat-config:/app/napcat/config \
  -v %s:/root/.openclaw:rw \
  -v %s:/root/openclaw/work:rw \
  mlikiowa/napcat-docker:latest

echo "⏳ 等待容器启动..."
sleep 5

# Configure OneBot11 WebSocket + HTTP
echo "🔧 配置 OneBot11 (WS + HTTP)..."
docker exec openclaw-qq bash -c 'cat > /app/napcat/config/onebot11.json << OBEOF
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
OBEOF'

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
`, cfg.OpenClawDir, cfg.OpenClawWork)
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
if (-not (Test-Path $INSTALL_DIR)) {
  New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
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
`, installDir)
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
