# ClawPanel Lite for Windows (preview)
$ErrorActionPreference = "Stop"

$Repo = "zhaoxinyi02/ClawPanel"
$GiteeRepo = "zxy000006/ClawPanel"
$TagPrefix = "lite-v"
$GiteeRawBase = "https://gitee.com/$GiteeRepo/raw/main"
$GiteeMeta = "$GiteeRawBase/release/update-lite.json"
$GiteeReleaseBase = "https://gitee.com/$GiteeRepo/releases/download"
$InstallDir = "C:\ClawPanelLite"
$ServiceName = "clawpanel-lite"
$DefaultVersion = "0.1.9"

function Get-LatestVersionFromGitHub {
  $items = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases?per_page=20" -UseBasicParsing
  $tag = [string](($items | Where-Object { $_.tag_name -like "$TagPrefix*" } | Select-Object -First 1).tag_name)
  if ($tag) { return ($tag -replace "^$TagPrefix", '') }
  return $null
}

function Get-LatestVersionFromGitee {
  $info = Invoke-RestMethod -Uri $GiteeMeta -UseBasicParsing
  return [string]$info.latest_version
}

function Download-File($Url, $Target) {
  Invoke-WebRequest -Uri $Url -OutFile $Target -UseBasicParsing
}

$DownloadSource = if ($env:DOWNLOAD_SOURCE) { $env:DOWNLOAD_SOURCE } else { $null }
if (-not $DownloadSource) {
  Write-Host "  [Lite] 请选择下载线路：" -ForegroundColor Cyan
  Write-Host "    1) GitHub      中国香港及境外服务器推荐" -ForegroundColor White
  Write-Host "    2) Gitee       中国大陆服务器推荐，更稳当一些" -ForegroundColor White
  $sourceChoice = Read-Host "  请输入 [1/2]（默认 2）"
  if ($sourceChoice -eq '1') { $DownloadSource = 'github' } else { $DownloadSource = 'gitee' }
}
$Version = if ($env:VERSION) { $env:VERSION } else { if ($DownloadSource -eq "github") { Get-LatestVersionFromGitHub } else { Get-LatestVersionFromGitee } }
if (-not $Version) { $Version = $DefaultVersion }
$PackageName = "clawpanel-lite-core-v${Version}-windows-amd64.tar.gz"

$PrimaryUrl = if ($DownloadSource -eq "github") { "https://github.com/$Repo/releases/download/${TagPrefix}${Version}/$PackageName" } else { "$GiteeReleaseBase/${TagPrefix}${Version}/$PackageName" }
$FallbackUrl = if ($DownloadSource -eq "github") { "$GiteeReleaseBase/${TagPrefix}${Version}/$PackageName" } else { "https://github.com/$Repo/releases/download/${TagPrefix}${Version}/$PackageName" }

$tmp = Join-Path $env:TEMP $PackageName
if ($DownloadSource -eq 'github') {
  Write-Host "  [Lite] 已选择 GitHub（中国香港及境外服务器推荐），失败时自动回退到 Gitee。" -ForegroundColor Cyan
} else {
  Write-Host "  [Lite] 已选择 Gitee（中国大陆服务器推荐），失败时自动回退到 GitHub。" -ForegroundColor Cyan
}
try { Download-File $PrimaryUrl $tmp } catch { Download-File $FallbackUrl $tmp }

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Get-Service $ServiceName -ErrorAction SilentlyContinue | Stop-Service -Force -ErrorAction SilentlyContinue
Remove-Item "$InstallDir\*" -Recurse -Force -ErrorAction SilentlyContinue
tar -xzf $tmp -C $InstallDir

Rename-Item -Path "$InstallDir\clawpanel-lite.exe" -NewName "clawpanel-lite.exe" -ErrorAction SilentlyContinue
New-Item -ItemType SymbolicLink -Path "C:\Windows\System32\clawlite-openclaw.cmd" -Target "$InstallDir\bin\clawlite-openclaw.cmd" -Force | Out-Null

sc.exe delete $ServiceName 2>$null | Out-Null
Start-Sleep -Seconds 1
sc.exe create $ServiceName binPath= "`"$InstallDir\clawpanel-lite.exe`"" start= auto DisplayName= "ClawPanel Lite v$Version" | Out-Null
sc.exe start $ServiceName | Out-Null

Write-Host "ClawPanel Lite for Windows installed at: $InstallDir"
