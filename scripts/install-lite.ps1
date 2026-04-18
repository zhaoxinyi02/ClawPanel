# ClawPanel Lite for Windows (preview)
$ErrorActionPreference = "Stop"

$ClawPanelPublicBase = if ($env:CLAWPANEL_PUBLIC_BASE) { $env:CLAWPANEL_PUBLIC_BASE.TrimEnd('/') } else { "http://43.248.142.249:19527" }
$Repo = "zhaoxinyi02/ClawPanel"
$TagPrefix = "lite-v"
$AccelBase = if ($env:ACCEL_BASE) { $env:ACCEL_BASE } else { "$ClawPanelPublicBase/api/panel/update-mirror" }
$AccelMeta = if ($env:ACCEL_META_URL) { $env:ACCEL_META_URL } else { "$AccelBase/lite" }
$InstallDir = "C:\ClawPanelLite"
$DataDir = "C:\ProgramData\ClawPanelLite"
$ServiceName = "clawpanel-lite"

function Get-LatestVersionFromGitHub {
  $items = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases?per_page=20" -UseBasicParsing
  $tag = [string](($items | Where-Object { $_.tag_name -like "$TagPrefix*" } | Select-Object -First 1).tag_name)
  if ($tag) { return ($tag -replace "^$TagPrefix", '') }
  return $null
}

function Get-LatestVersionFromAccel {
  $info = Invoke-RestMethod -Uri $AccelMeta -UseBasicParsing
  return [string]$info.latest_version
}

function Download-File($Url, $Target) {
  Invoke-WebRequest -Uri $Url -OutFile $Target -UseBasicParsing
}

$DownloadSource = if ($env:DOWNLOAD_SOURCE) { $env:DOWNLOAD_SOURCE } else { $null }
$LocalPackage = $env:LOCAL_PACKAGE
if (-not $DownloadSource) {
  Write-Host "  [Lite] 请选择下载线路：" -ForegroundColor Cyan
  Write-Host "    1) GitHub      中国香港及境外服务器推荐" -ForegroundColor White
  Write-Host "    2) 加速服务器  中国大陆服务器推荐，更稳当一些" -ForegroundColor White
  $sourceChoice = Read-Host "  请输入 [1/2]（默认 2）"
  if ($sourceChoice -eq '1') { $DownloadSource = 'github' } else { $DownloadSource = 'accel' }
}
$Version = if ($env:VERSION) { $env:VERSION } else { if ($DownloadSource -eq "github") { Get-LatestVersionFromGitHub } else { Get-LatestVersionFromAccel } }
if (-not $Version) {
  Write-Error "无法获取最新版本号。请检查网络连接，或通过 `$env:VERSION='x.y.z' 手动指定版本后重试。"
  exit 1
}
$PackageName = "clawpanel-lite-core-v${Version}-windows-amd64.tar.gz"

$PrimaryUrl = if ($DownloadSource -eq "github") { "https://github.com/$Repo/releases/download/${TagPrefix}${Version}/$PackageName" } else { "$AccelBase/lite/files/$PackageName" }
$FallbackUrl = if ($DownloadSource -eq "github") { "$AccelBase/lite/files/$PackageName" } else { "https://github.com/$Repo/releases/download/${TagPrefix}${Version}/$PackageName" }

$tmp = Join-Path $env:TEMP $PackageName
if ($LocalPackage) {
  if (-not (Test-Path $LocalPackage)) { Write-Error "指定的本地 Lite 构建包不存在: $LocalPackage"; exit 1 }
  Copy-Item -Path $LocalPackage -Destination $tmp -Force
  Write-Host "  [Lite] 已使用当前目录中的本地 Lite 构建包进行安装。" -ForegroundColor Cyan
} elseif ($DownloadSource -eq 'github') {
  Write-Host "  [Lite] 已选择 GitHub（中国香港及境外服务器推荐），失败时自动回退到加速服务器。" -ForegroundColor Cyan
} else {
  Write-Host "  [Lite] 已选择加速服务器（中国大陆服务器推荐），失败时自动回退到 GitHub。" -ForegroundColor Cyan
}
if (-not $LocalPackage) {
  try { Download-File $PrimaryUrl $tmp } catch { Download-File $FallbackUrl $tmp }
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
Get-Service $ServiceName -ErrorAction SilentlyContinue | Stop-Service -Force -ErrorAction SilentlyContinue

$legacyDataDir = Join-Path $InstallDir "data"
if (Test-Path $legacyDataDir) {
  $hasExternalData = (Test-Path (Join-Path $DataDir "clawpanel.json")) -or (Test-Path (Join-Path $DataDir "openclaw-config\openclaw.json"))
  if (-not $hasExternalData) {
    Write-Host "  [Lite] 检测到旧版内置数据目录，迁移到外部持久化目录..." -ForegroundColor Cyan
    Copy-Item -Path (Join-Path $legacyDataDir '*') -Destination $DataDir -Recurse -Force -ErrorAction SilentlyContinue
  } else {
    Write-Host "  [Lite] 检测到旧版内置数据目录，但外部数据目录已有内容，跳过自动覆盖。" -ForegroundColor Cyan
  }
}

Remove-Item "$InstallDir\*" -Recurse -Force -ErrorAction SilentlyContinue
tar -xzf $tmp -C $InstallDir

Remove-Item "$InstallDir\data" -Recurse -Force -ErrorAction SilentlyContinue
cmd /c mklink /D "$InstallDir\data" "$DataDir" | Out-Null

Rename-Item -Path "$InstallDir\clawpanel-lite.exe" -NewName "clawpanel-lite.exe" -ErrorAction SilentlyContinue
New-Item -ItemType SymbolicLink -Path "C:\Windows\System32\clawlite-openclaw.cmd" -Target "$InstallDir\bin\clawlite-openclaw.cmd" -Force | Out-Null

sc.exe delete $ServiceName 2>$null | Out-Null
Start-Sleep -Seconds 1
sc.exe create $ServiceName binPath= "`"$InstallDir\clawpanel-lite.exe`"" start= auto DisplayName= "ClawPanel Lite v$Version" | Out-Null
reg add "HKLM\SYSTEM\CurrentControlSet\Services\$ServiceName" /v Environment /t REG_MULTI_SZ /d "CLAWPANEL_EDITION=lite\0CLAWPANEL_DATA=$DataDir\0NODE_OPTIONS=--max-old-space-size=2048\0" /f | Out-Null
sc.exe start $ServiceName | Out-Null

Write-Host "ClawPanel Lite for Windows installed at: $InstallDir"
Write-Host "ClawPanel Lite data stored at: $DataDir"
