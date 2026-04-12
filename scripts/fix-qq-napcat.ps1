# ============================================================
# ClawPanel QQ NapCat 插件诊断与修复脚本 (Windows PowerShell)
# 兼容 PowerShell 5.1 及以上版本
# 用法 (管理员 PowerShell):
#   $env:CLAWPANEL_PUBLIC_BASE="http://43.248.142.249:19527"
#   irm "$env:CLAWPANEL_PUBLIC_BASE/scripts/fix-qq-napcat.ps1" | iex
# ============================================================

$ErrorActionPreference = "Continue"

function Ok($msg)   { Write-Host "  [✓] $msg" -ForegroundColor Green }
function Warn($msg) { Write-Host "  [!] $msg" -ForegroundColor Yellow }
function Err($msg)  { Write-Host "  [✗] $msg" -ForegroundColor Red }
function Info($msg) { Write-Host "  [→] $msg" -ForegroundColor Cyan }
function Title($msg){ Write-Host ""; Write-Host "  $msg" -ForegroundColor Magenta; Write-Host "  $("-" * $msg.Length)" -ForegroundColor DarkGray }

Write-Host ""
Write-Host "  ╔══════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "  ║   ClawPanel QQ NapCat 插件诊断与修复工具    ║" -ForegroundColor Cyan
Write-Host "  ╚══════════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""

# 刷新 PATH（管理员会话可能丢失用户 PATH）
$env:PATH = [System.Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("PATH","User")
$home2 = [System.Environment]::GetFolderPath("UserProfile")

# 将 nvm-windows 所有 node 版本的 bin 目录加入 PATH
$nvmRoot = [System.Environment]::GetEnvironmentVariable("NVM_HOME", "User")
if (-not $nvmRoot) { $nvmRoot = [System.Environment]::GetEnvironmentVariable("NVM_HOME", "Machine") }
if (-not $nvmRoot) { $nvmRoot = Join-Path $home2 "AppData\Roaming\nvm" }
if (Test-Path $nvmRoot) {
    Get-ChildItem $nvmRoot -Directory | ForEach-Object {
        $binPath = Join-Path $_.FullName ""
        if ($env:PATH -notlike "*$binPath*") { $env:PATH = "$binPath;$env:PATH" }
    }
}

# ── 1. 检测 Node.js ──────────────────────────────────────────
Title "1. 检测 Node.js"
$nodeCmd = Get-Command node -ErrorAction SilentlyContinue
if ($nodeCmd) {
    $nodeVer = node --version 2>$null
    Ok "Node.js $nodeVer"
} else {
    Err "Node.js 未安装"
    Info "尝试通过 winget 自动安装 Node.js LTS..."
    $wingetCmd = Get-Command winget -ErrorAction SilentlyContinue
    if ($wingetCmd) {
        winget install OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements --silent 2>&1
        $env:PATH = [System.Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("PATH","User")
        $nodeCmd = Get-Command node -ErrorAction SilentlyContinue
        if ($nodeCmd) {
            Ok "Node.js $(node --version) 已安装"
        } else {
            Err "自动安装失败，请从 https://nodejs.org 手动下载安装 Node.js 后重试"
            exit 1
        }
    } else {
        Err "未找到 winget，请从 https://nodejs.org 手动安装 Node.js"
        exit 1
    }
}

# ── 2. 检测 openclaw CLI ──────────────────────────────────────
Title "2. 检测 OpenClaw"
$ocExe = $null

# 1) 先尝试 PATH 中直接找
$ocCmd = Get-Command openclaw -ErrorAction SilentlyContinue
if ($ocCmd) { $ocExe = $ocCmd.Source }

# 2) 从 npm prefix 目录查找（nvm 每个版本有独立 prefix）
if (-not $ocExe) {
    try {
        $npmPrefix = (npm config get prefix 2>$null).Trim()
        $candidate = Join-Path $npmPrefix "openclaw.cmd"
        if (Test-Path $candidate) { $ocExe = $candidate }
    } catch {}
}

# 3) 扫描 nvm 目录下所有 node 版本的全局 bin
if (-not $ocExe -and (Test-Path $nvmRoot)) {
    Get-ChildItem $nvmRoot -Directory | ForEach-Object {
        $candidate = Join-Path $_.FullName "openclaw.cmd"
        if ((Test-Path $candidate) -and (-not $ocExe)) { $ocExe = $candidate }
    }
}

if ($ocExe) {
    $ocVer = & $ocExe --version 2>$null
    Ok "openclaw $ocVer ($ocExe)"
} else {
    Warn "openclaw 未找到，正在安装..."
    npm install -g openclaw@latest --registry=https://registry.npmmirror.com 2>&1
    # 刷新 PATH 后再查
    $env:PATH = [System.Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("PATH","User")
    $ocCmd2 = Get-Command openclaw -ErrorAction SilentlyContinue
    if ($ocCmd2) {
        $ocExe = $ocCmd2.Source
        Ok "openclaw $(& $ocExe --version 2>$null) 已安装"
    } else {
        Err "openclaw 安装失败，请手动运行: npm install -g openclaw@latest"
    }
}

# ── 3. 定位 openclaw.json ────────────────────────────────────
Title "3. 定位 OpenClaw 配置"
$candidates = @(
    (Join-Path $home2 "openclaw\config\openclaw.json"),
    (Join-Path $home2 ".openclaw\openclaw.json"),
    "C:\openclaw\config\openclaw.json"
)
$ocJson = $null
foreach ($c in $candidates) {
    if (Test-Path $c) { $ocJson = $c; break }
}
if ($ocJson) {
    Ok "openclaw.json: $ocJson"
} else {
    Warn "openclaw.json 未找到，尝试初始化..."
    try { openclaw init 2>$null } catch {}
    $ocJson = Join-Path $home2 "openclaw\config\openclaw.json"
    if (Test-Path $ocJson) {
        Ok "openclaw.json 已初始化: $ocJson"
    } else {
        Err "初始化失败，请手动运行: openclaw init"
    }
}

# ── 4. 检查 QQ 扩展目录 ──────────────────────────────────────
Title "4. 检测 QQ 扩展插件"
$npmRoot = npm root -g 2>$null
if ($npmRoot) {
    $qqExtDir = Join-Path $npmRoot "openclaw\extensions\qq"
    if (Test-Path $qqExtDir) {
        Ok "QQ 扩展目录: $qqExtDir"
    } else {
        Warn "QQ 扩展目录不存在，尝试重装 openclaw..."
        npm install -g openclaw@latest --registry=https://registry.npmmirror.com 2>&1
        if (Test-Path $qqExtDir) {
            Ok "QQ 扩展已随 openclaw 重装"
        } else {
            Warn "此版本 openclaw 可能已内置 QQ 扩展或路径不同，跳过"
        }
    }
} else {
    Warn "无法获取 npm 全局目录"
}

# ── 5. 检测 NapCat Shell 进程 ────────────────────────────────
Title "5. 检测 NapCat (Windows Shell)"
$napcatProc = Get-Process -Name "NapCatWinBootMain" -ErrorAction SilentlyContinue
$napcatProc2 = Get-Process -Name "napcat" -ErrorAction SilentlyContinue
if ($napcatProc -or $napcatProc2) {
    Ok "NapCat 进程运行中"
    # 检测 OneBot WS 端口
    try {
        $tcp = New-Object System.Net.Sockets.TcpClient
        $tcp.Connect("127.0.0.1", 3001)
        $tcp.Close()
        Ok "OneBot WS 端口 3001 可达"
    } catch {
        Warn "OneBot WS 端口 3001 不可达（NapCat 可能正在初始化）"
    }
} else {
    Warn "NapCat 未运行"
    Info "请通过 ClawPanel → NapCat 管理页面启动 NapCat Shell"
    Info "或参考文档: https://github.com/zhaoxinyi02/ClawPanel/blob/main/docs/qq-napcat-guide.md"
}

# ── 6. 检查 openclaw.json QQ 通道配置 ───────────────────────
Title "6. 检查 QQ 通道配置"
if ($ocJson -and (Test-Path $ocJson)) {
    try {
        $ocConfig = Get-Content $ocJson -Raw | ConvertFrom-Json
        $qqCh = $ocConfig.channels.qq
        if ($qqCh) {
            if ($qqCh.enabled -eq $true) {
                Ok "channels.qq.enabled = true"
            } else {
                Warn "channels.qq.enabled = false，如需 QQ 通道请在 ClawPanel 中启用"
            }
            if ($qqCh.wsUrl -eq "ws://127.0.0.1:3001") {
                Ok "channels.qq.wsUrl = ws://127.0.0.1:3001"
            } elseif ($qqCh.wsUrl) {
                Ok "channels.qq.wsUrl = $($qqCh.wsUrl)"
            } else {
                Warn "channels.qq.wsUrl 未配置，建议值: ws://127.0.0.1:3001"
            }
        } else {
            Warn "openclaw.json 中未找到 channels.qq 配置，请在 ClawPanel 通道配置中添加"
        }
    } catch {
        Warn "无法解析 openclaw.json: $_"
    }
}

# ── 7. 完成 ─────────────────────────────────────────────────
Write-Host ""
Write-Host "  ══════════════════════════════════════════════" -ForegroundColor Green
Write-Host "  诊断完成！如仍有问题，请提交 Issue:" -ForegroundColor Green
Write-Host "  https://github.com/zhaoxinyi02/ClawPanel/issues" -ForegroundColor Green
Write-Host "  ══════════════════════════════════════════════" -ForegroundColor Green
Write-Host ""
