# ============================================================
# ClawPanel 一键安装脚本 (Windows PowerShell)
# 兼容 PowerShell 5.1 及以上版本
# 用法 (管理员 PowerShell):
#   irm https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install.ps1 | iex
# 或:
#   Invoke-WebRequest -Uri https://gitee.com/zxy000006/ClawPanel/raw/main/scripts/install.ps1 -OutFile install.ps1; .\install.ps1
# ============================================================

$ErrorActionPreference = "Stop"

$REPO = "zhaoxinyi02/ClawPanel"
$GITEE_REPO = "zxy000006/ClawPanel"
$TAG_PREFIX = "pro-v"
$GITEE_RAW_BASE = "https://gitee.com/$GITEE_REPO/raw/main"
$GITEE_RELEASE_BASE = "https://gitee.com/$GITEE_REPO/releases/download"
$GITEE_META = "$GITEE_RAW_BASE/release/update-pro.json"
$INSTALL_DIR = "C:\ClawPanel"
$SERVICE_NAME = "ClawPanel"
$PORT = "19527"

# ==================== 自动获取最新版本 ====================
Write-Host "  [ClawPanel] 获取最新版本信息..." -ForegroundColor Cyan
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $releaseInfo = Invoke-RestMethod -Uri $GITEE_META -UseBasicParsing
    $tag = [string]$releaseInfo.latest_version
    if ([string]::IsNullOrWhiteSpace($tag)) {
        throw "empty latest_version"
    }
    $VERSION = $tag -replace '^v', ''
    if ([string]::IsNullOrWhiteSpace($VERSION) -or ($VERSION -notmatch '^[0-9][0-9A-Za-z._-]*$')) {
        throw "invalid tag_name: $tag"
    }
    Write-Host "  [ClawPanel] 最新版本: v$VERSION" -ForegroundColor Green
} catch {
    Write-Host "  [ClawPanel] 无法获取最新版本，使用默认版本..." -ForegroundColor Yellow
    try {
        $releaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$REPO/releases?per_page=20" -UseBasicParsing
        $tag = [string](($releaseInfo | Where-Object { $_.tag_name -like "$TAG_PREFIX*" } | Select-Object -First 1).tag_name)
        $VERSION = $tag -replace "^$TAG_PREFIX", ''
    } catch {
        $VERSION = "5.2.10"
    }
}

$BINARY_NAME = "clawpanel-v${VERSION}-windows-amd64.exe"
$DownloadSource = if ($env:DOWNLOAD_SOURCE) { $env:DOWNLOAD_SOURCE } else { $null }

if (-not $DownloadSource) {
    Write-Host "  [ClawPanel] 请选择下载线路：" -ForegroundColor Cyan
    Write-Host "    1) GitHub      中国香港及境外服务器推荐" -ForegroundColor White
    Write-Host "    2) Gitee       中国大陆服务器推荐，更稳当一些" -ForegroundColor White
    $sourceChoice = Read-Host "  请输入 [1/2]（默认 2）"
    if ($sourceChoice -eq '1') { $DownloadSource = 'github' } else { $DownloadSource = 'gitee' }
}

# ==================== 工具函数 ====================
function Log($msg)  { Write-Host "  [ClawPanel] $msg" -ForegroundColor Green }
function Info($msg) { Write-Host "  [ClawPanel] $msg" -ForegroundColor Cyan }
function Warn($msg) { Write-Host "  [ClawPanel] $msg" -ForegroundColor Yellow }
function Err($msg)  { Write-Host "  [ClawPanel] $msg" -ForegroundColor Red; Read-Host "  按回车键退出"; exit 1 }
function Step($n, $total, $msg) { Write-Host "  [$n/$total] $msg" -ForegroundColor Magenta }

# ==================== Banner ====================
function Print-Banner {
    Write-Host ""
    Write-Host "  =================================================================" -ForegroundColor Magenta
    Write-Host "                                                                   " -ForegroundColor Magenta
    Write-Host "    ██████╗██╗      █████╗ ██╗    ██╗██████╗  █████╗ ███╗   ██╗    " -ForegroundColor Magenta
    Write-Host "   ██╔════╝██║     ██╔══██╗██║    ██║██╔══██╗██╔══██╗████╗  ██║    " -ForegroundColor Magenta
    Write-Host "   ██║     ██║     ███████║██║ █╗ ██║██████╔╝███████║██╔██╗ ██║    " -ForegroundColor Magenta
    Write-Host "   ██║     ██║     ██╔══██║██║███╗██║██╔═══╝ ██╔══██║██║╚██╗██║    " -ForegroundColor Magenta
    Write-Host "   ╚██████╗███████╗██║  ██║╚███╔███╔╝██║     ██║  ██║██║ ╚████║    " -ForegroundColor Magenta
    Write-Host "    ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝ ╚═╝     ╚═╝  ╚═╝╚═╝ ╚═══╝    " -ForegroundColor Magenta
    Write-Host "                                                                   " -ForegroundColor Magenta
    Write-Host "    ClawPanel v$VERSION - OpenClaw 智能管理面板                     " -ForegroundColor Magenta
    Write-Host "    https://github.com/$REPO                                       " -ForegroundColor Magenta
    Write-Host "                                                                   " -ForegroundColor Magenta
    Write-Host "  =================================================================" -ForegroundColor Magenta
    Write-Host ""
}

# ==================== 主安装流程 ====================
Print-Banner

# 检查管理员权限
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (-NOT $isAdmin) {
    Err "请右键选择「以管理员身份运行」PowerShell，然后重新执行此脚本！"
}

if ([System.Environment]::Is64BitOperatingSystem) { $sysArch = 'x64' } else { $sysArch = 'x86' }
Info "系统信息: Windows/$sysArch"
Info "安装目录: $INSTALL_DIR"
Write-Host ""

$TOTAL = 5

# ---- Step 1 ----
Step 1 $TOTAL "创建安装目录..."
New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
New-Item -ItemType Directory -Force -Path "$INSTALL_DIR\data" | Out-Null
Log "目录已创建: $INSTALL_DIR"

# ---- Step 2 ----
Step 2 $TOTAL "下载 ClawPanel v$VERSION..."
$downloadUrl = if ($DownloadSource -eq "github") { "https://github.com/$REPO/releases/download/${TAG_PREFIX}${VERSION}/$BINARY_NAME" } else { "$GITEE_RELEASE_BASE/${TAG_PREFIX}${VERSION}/$BINARY_NAME" }
$fallbackUrl = if ($DownloadSource -eq "github") { "$GITEE_RELEASE_BASE/${TAGPrefix}${VERSION}/$BINARY_NAME" } else { "https://github.com/$REPO/releases/download/${TAGPrefix}${VERSION}/$BINARY_NAME" }
$targetPath = "$INSTALL_DIR\clawpanel.exe"
if ($DownloadSource -eq 'github') {
    Info "已选择 GitHub（中国香港及境外服务器推荐），失败时自动回退到 Gitee。"
} else {
    Info "已选择 Gitee（中国大陆服务器推荐），失败时自动回退到 GitHub。"
}
Info "下载地址: $downloadUrl"

try {
    # 停止旧服务
    sc.exe stop $SERVICE_NAME 2>$null | Out-Null
    Start-Sleep -Seconds 1

    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $ProgressPreference = 'SilentlyContinue'
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $targetPath -UseBasicParsing
    } catch {
        Invoke-WebRequest -Uri $fallbackUrl -OutFile $targetPath -UseBasicParsing
    }
    $fileSize = [math]::Round((Get-Item $targetPath).Length / 1MB, 1)
    Log "下载完成 (${fileSize} MB)"
} catch {
    Err "下载失败: $_`n请检查网络连接或手动下载: $downloadUrl"
}

# ---- Step 3 ----
Step 3 $TOTAL "注册 Windows 服务（开机自启动）..."
# 删除旧服务
sc.exe delete $SERVICE_NAME 2>$null | Out-Null
Start-Sleep -Seconds 1

# 创建新服务
$scResult = sc.exe create $SERVICE_NAME binPath= "`"$targetPath`"" start= auto DisplayName= "ClawPanel v$VERSION"
if ($LASTEXITCODE -eq 0) {
    Log "服务已注册，开机自启动已启用"
} else {
    Warn "服务注册: $scResult"
}

# 设置描述和失败重启
sc.exe description $SERVICE_NAME "ClawPanel v$VERSION - OpenClaw 智能助手管理面板" 2>$null | Out-Null
sc.exe failure $SERVICE_NAME reset= 86400 actions= restart/5000/restart/10000/restart/30000 2>$null | Out-Null

# ---- Step 4 ----
Step 4 $TOTAL "配置防火墙规则..."
netsh advfirewall firewall delete rule name="ClawPanel" 2>$null | Out-Null
$fwResult = netsh advfirewall firewall add rule name="ClawPanel" dir=in action=allow protocol=TCP localport=$PORT
if ($fwResult -match "Ok|确定") {
    Log "已放行端口 $PORT"
} else {
    Warn "防火墙配置可能失败，请手动放行端口 $PORT"
}

# ---- Step 5 ----
Step 5 $TOTAL "启动 ClawPanel 服务..."
$startResult = sc.exe start $SERVICE_NAME 2>&1
if ($LASTEXITCODE -eq 0) {
    Log "服务启动成功"
} else {
    Warn "服务启动: $startResult"
    Write-Host "  可手动启动: sc start ClawPanel" -ForegroundColor Yellow
}

# ==================== 安装完成 ====================
Write-Host ""
Write-Host "  =================================================================" -ForegroundColor Green
Write-Host "                                                                   " -ForegroundColor Green
Write-Host "    ClawPanel v$VERSION 安装完成!                                   " -ForegroundColor Green
Write-Host "                                                                   " -ForegroundColor Green
Write-Host "  =================================================================" -ForegroundColor Green
Write-Host ""
Write-Host "  面板地址:  http://localhost:$PORT" -ForegroundColor Cyan
Write-Host "  默认密码:  clawpanel" -ForegroundColor Cyan
Write-Host ""
Write-Host "  安装目录:  $INSTALL_DIR" -ForegroundColor White
Write-Host "  数据目录:  $INSTALL_DIR\data" -ForegroundColor White
Write-Host "  配置文件:  $INSTALL_DIR\data\config.json (首次启动后生成)" -ForegroundColor White
Write-Host ""
Write-Host "  管理命令:" -ForegroundColor White
Write-Host "    sc start ClawPanel    # 启动" -ForegroundColor Gray
Write-Host "    sc stop ClawPanel     # 停止" -ForegroundColor Gray
Write-Host "    sc query ClawPanel    # 查看状态" -ForegroundColor Gray
Write-Host ""
Write-Host "  卸载命令:" -ForegroundColor White
Write-Host "    sc stop ClawPanel" -ForegroundColor Gray
Write-Host "    sc delete ClawPanel" -ForegroundColor Gray
Write-Host "    Remove-Item -Recurse -Force $INSTALL_DIR" -ForegroundColor Gray
Write-Host ""
Write-Host "  !! 请登录后立即修改默认密码 !!" -ForegroundColor Red
Write-Host ""
Write-Host "  =================================================================" -ForegroundColor Green
Write-Host ""

Read-Host "  按回车键退出"
