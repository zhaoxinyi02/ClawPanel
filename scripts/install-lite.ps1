# ClawPanel Lite for Windows (preview)
$ErrorActionPreference = "Stop"

$Repo = "zhaoxinyi02/ClawPanel"
$TagPrefix = "lite-v"
$AccelBase = "http://39.102.53.188:16198/clawpanel"
$AccelMeta = "$AccelBase/update-lite.json"
$InstallDir = "C:\ClawPanelLite"
$ServiceName = "clawpanel-lite"
$DefaultVersion = "0.1.7"

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

$DownloadSource = if ($env:DOWNLOAD_SOURCE) { $env:DOWNLOAD_SOURCE } else { "accel" }
$Version = if ($env:VERSION) { $env:VERSION } else { if ($DownloadSource -eq "github") { Get-LatestVersionFromGitHub } else { Get-LatestVersionFromAccel } }
if (-not $Version) { $Version = $DefaultVersion }
$PackageName = "clawpanel-lite-core-v${Version}-windows-amd64.tar.gz"

$PrimaryUrl = if ($DownloadSource -eq "github") { "https://github.com/$Repo/releases/download/${TagPrefix}${Version}/$PackageName" } else { "$AccelBase/releases/$PackageName" }
$FallbackUrl = if ($DownloadSource -eq "github") { "$AccelBase/releases/$PackageName" } else { "https://github.com/$Repo/releases/download/${TagPrefix}${Version}/$PackageName" }

$tmp = Join-Path $env:TEMP $PackageName
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
