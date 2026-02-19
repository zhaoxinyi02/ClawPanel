# ============================================================
# ClawPanel v4.3.0 — Windows 安装脚本
# 用法: irm https://raw.githubusercontent.com/zhaoxinyi02/ClawPanel/main/install.ps1 | iex
# 或: powershell -ExecutionPolicy Bypass -File install.ps1
# ============================================================

$ErrorActionPreference = "Stop"
$CLAWPANEL_VERSION = "4.3.0"
$CLAWPANEL_REPO = "https://github.com/zhaoxinyi02/ClawPanel.git"

Write-Host ""
Write-Host "  ╔═══════════════════════════════════════════╗" -ForegroundColor Magenta
Write-Host "  ║       🐾 ClawPanel v$CLAWPANEL_VERSION Installer       ║" -ForegroundColor Magenta
Write-Host "  ║   OpenClaw Management Panel Installer     ║" -ForegroundColor Magenta
Write-Host "  ╚═══════════════════════════════════════════╝" -ForegroundColor Magenta
Write-Host ""

# Check Node.js
try {
    $nodeVer = (node -v) -replace 'v', ''
    $major = [int]($nodeVer.Split('.')[0])
    if ($major -lt 18) {
        Write-Host "[ERROR] Node.js version too low (v$nodeVer), need v18+" -ForegroundColor Red
        exit 1
    }
    Write-Host "[OK] Node.js v$nodeVer" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Node.js not found. Install from https://nodejs.org/" -ForegroundColor Red
    exit 1
}

# Check Git
try {
    $gitVer = (git --version) -replace 'git version ', ''
    Write-Host "[OK] Git $gitVer" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Git not found. Install from https://git-scm.com/" -ForegroundColor Red
    exit 1
}

# Gather input
Write-Host ""
Write-Host "[INFO] Please enter installation details (press Enter for defaults):" -ForegroundColor Cyan
Write-Host ""

$DefaultDir = "$env:USERPROFILE\ClawPanel"
$InstallDir = Read-Host "  Install directory [$DefaultDir]"
if ([string]::IsNullOrWhiteSpace($InstallDir)) { $InstallDir = $DefaultDir }

$PanelPort = Read-Host "  Panel port [6199]"
if ([string]::IsNullOrWhiteSpace($PanelPort)) { $PanelPort = "6199" }

$AdminToken = Read-Host "  Admin password [openclaw-qq-admin]"
if ([string]::IsNullOrWhiteSpace($AdminToken)) { $AdminToken = "openclaw-qq-admin" }

$QQAccount = Read-Host "  QQ account (optional, leave empty to skip)"
$OwnerQQ = Read-Host "  Owner QQ (for notifications, 0=none) [0]"
if ([string]::IsNullOrWhiteSpace($OwnerQQ)) { $OwnerQQ = "0" }

$WebuiToken = Read-Host "  NapCat WebUI Token [openclaw-qq-admin]"
if ([string]::IsNullOrWhiteSpace($WebuiToken)) { $WebuiToken = "openclaw-qq-admin" }

# Detect OpenClaw
$OpenClawDir = "$env:USERPROFILE\.openclaw"
$OpenClawWork = ""
$OpenClawApp = ""

if (Test-Path "$env:USERPROFILE\openclaw\app") {
    $OpenClawApp = "$env:USERPROFILE\openclaw\app"
    Write-Host "[OK] Found OpenClaw app: $OpenClawApp" -ForegroundColor Green
}
if (Test-Path "$env:USERPROFILE\openclaw\work") {
    $OpenClawWork = "$env:USERPROFILE\openclaw\work"
}

Write-Host ""
Write-Host "  Install dir:  $InstallDir"
Write-Host "  Panel port:   $PanelPort"
Write-Host "  Admin pass:   ****"
Write-Host ""

$confirm = Read-Host "Proceed with installation? [Y/n]"
if ($confirm -eq "n") { Write-Host "Cancelled."; exit 0 }

# Clone / Update
Write-Host ""
Write-Host "[INFO] Cloning ClawPanel..." -ForegroundColor Cyan
if (Test-Path $InstallDir) {
    Set-Location $InstallDir
    git pull origin main 2>$null
} else {
    git clone $CLAWPANEL_REPO $InstallDir
    Set-Location $InstallDir
}

# Build
Write-Host "[INFO] Installing backend dependencies..." -ForegroundColor Cyan
Set-Location server; npm install --omit=dev 2>$null; Set-Location ..

Write-Host "[INFO] Building backend..." -ForegroundColor Cyan
Set-Location server; npx tsc 2>$null; Set-Location ..

Write-Host "[INFO] Installing frontend dependencies..." -ForegroundColor Cyan
Set-Location web; npm install 2>$null; Set-Location ..

Write-Host "[INFO] Building frontend..." -ForegroundColor Cyan
Set-Location web; npm run build 2>$null; Set-Location ..

# Config
Write-Host "[INFO] Creating configuration..." -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path data | Out-Null

@"
ADMIN_TOKEN=$AdminToken
OPENCLAW_DIR=$OpenClawDir
OPENCLAW_WORK=$OpenClawWork
OPENCLAW_APP=$OpenClawApp
QQ_ACCOUNT=$QQAccount
WEBUI_TOKEN=$WebuiToken
OWNER_QQ=$OwnerQQ
NAPCAT_TOKEN=
WECHAT_TOKEN=openclaw-wechat
"@ | Set-Content .env

if (-not (Test-Path "data\admin-config.json")) {
@"
{
  "server": { "port": $PanelPort, "host": "0.0.0.0", "token": "$AdminToken" },
  "openclaw": { "configPath": "$($OpenClawDir -replace '\\', '/')/openclaw.json", "autoSetup": true },
  "napcat": { "wsUrl": "ws://127.0.0.1:3001", "accessToken": "", "webuiPort": 6099, "webuiToken": "$WebuiToken" },
  "wechat": { "apiUrl": "http://localhost:3001", "token": "openclaw-wechat", "enabled": false },
  "qq": { "ownerQQ": $OwnerQQ, "antiRecall": { "enabled": true }, "enabledChannels": ["qq"] }
}
"@ | Set-Content "data\admin-config.json"
}

# Create start script
@"
@echo off
title ClawPanel v$CLAWPANEL_VERSION
cd /d "$InstallDir"
set NODE_ENV=production
set OPENCLAW_CONFIG=$OpenClawDir\openclaw.json
set OPENCLAW_WORK=$OpenClawWork
set OPENCLAW_APP=$OpenClawApp
echo Starting ClawPanel on port $PanelPort...
node server\dist\index.js
pause
"@ | Set-Content "start-clawpanel.bat"

# Create desktop shortcut
$DesktopPath = [Environment]::GetFolderPath("Desktop")
$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut("$DesktopPath\ClawPanel.lnk")
$Shortcut.TargetPath = "http://localhost:$PanelPort"
$Shortcut.Description = "ClawPanel - OpenClaw Management Panel"
$Shortcut.Save()
Write-Host "[OK] Desktop shortcut created" -ForegroundColor Green

# Start
Write-Host ""
Write-Host "[INFO] Starting ClawPanel..." -ForegroundColor Cyan
$env:NODE_ENV = "production"
$env:OPENCLAW_CONFIG = "$OpenClawDir\openclaw.json"
$env:OPENCLAW_WORK = $OpenClawWork
$env:OPENCLAW_APP = $OpenClawApp

Start-Process -FilePath "node" -ArgumentList "server\dist\index.js" -WorkingDirectory $InstallDir -WindowStyle Hidden

Start-Sleep -Seconds 2

# Get IP
$LocalIP = (Get-NetIPAddress -AddressFamily IPv4 | Where-Object { $_.InterfaceAlias -notmatch "Loopback" -and $_.IPAddress -ne "127.0.0.1" } | Select-Object -First 1).IPAddress

Write-Host ""
Write-Host "╔═══════════════════════════════════════════════════════╗" -ForegroundColor Green
Write-Host "║         🐾 ClawPanel installed successfully!          ║" -ForegroundColor Green
Write-Host "╚═══════════════════════════════════════════════════════╝" -ForegroundColor Green
Write-Host ""
Write-Host "  Install dir:    $InstallDir" -ForegroundColor Cyan
Write-Host "  Panel URL:      http://localhost:$PanelPort" -ForegroundColor Cyan
if ($LocalIP) {
Write-Host "  LAN URL:        http://${LocalIP}:$PanelPort" -ForegroundColor Cyan
}
Write-Host "  Admin password: $AdminToken" -ForegroundColor Cyan
Write-Host ""
Write-Host "  To start:  .\start-clawpanel.bat" -ForegroundColor Yellow
Write-Host "  To stop:   taskkill /f /im node.exe" -ForegroundColor Yellow
Write-Host ""

# Open browser
Start-Process "http://localhost:$PanelPort"
