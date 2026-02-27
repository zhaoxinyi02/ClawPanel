# ============================================================
# ClawPanel v5.0.2 一键安装脚本 (Windows PowerShell)
# 用法 (管理员 PowerShell):
#   irm https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install.ps1 | iex
# 或:
#   Invoke-WebRequest -Uri https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/scripts/install.ps1 -OutFile install.ps1; .\install.ps1
# ============================================================

$ErrorActionPreference = "Stop"

$VERSION = "5.0.4"
$INSTALL_DIR = "C:\ClawPanel"
$BINARY_NAME = "clawpanel-windows-amd64.exe"
$REPO = "zhaoxinyi02/ClawPanel"
$SERVICE_NAME = "ClawPanel"
$PORT = "19527"

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

Info "系统信息: Windows/$([System.Environment]::Is64BitOperatingSystem ? 'x64' : 'x86')"
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
$downloadUrl = "https://github.com/$REPO/releases/download/v$VERSION/$BINARY_NAME"
$targetPath = "$INSTALL_DIR\clawpanel.exe"
Info "下载地址: $downloadUrl"

try {
    # 停止旧服务
    sc.exe stop $SERVICE_NAME 2>$null | Out-Null
    Start-Sleep -Seconds 1

    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $downloadUrl -OutFile $targetPath -UseBasicParsing
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
