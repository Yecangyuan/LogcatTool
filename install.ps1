#Requires -Version 5.1
<#
.SYNOPSIS
    Install LogCaTool for Windows 7 SP1+ from the go-win7 branch releases.

.DESCRIPTION
    Downloads the Windows 7 compatible build of logcatool (produced with the
    XTLS/go-win7 patched Go SDK) and extracts it to the requested directory.

.PARAMETER InstallDir
    Directory where logcatool.exe will be extracted.
    Default: $env:LOCALAPPDATA\Programs\logcatool

.PARAMETER Version
    Release tag to install, e.g. "v0.1.0". Use "latest" to fetch the most
    recent release from the go-win7 branch on GitHub.
    Default: latest

.EXAMPLE
    .\install.ps1

.EXAMPLE
    .\install.ps1 -Version v0.1.0 -InstallDir C:\tools\logcatool
#>
param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\logcatool",
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"

# Ensure TLS 1.2 is available on older Windows/PS versions.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = "Yecangyuan/LogcatTool"

if ($Version -eq "latest") {
    Write-Host "Fetching latest release tag from GitHub ..."
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    $Version = $release.tag_name
    Write-Host "Latest release is $Version"
}

$zipName = "logcatool_${Version}_windows_amd64.zip"
$url = "https://github.com/$Repo/releases/download/$Version/$zipName"

Write-Host "Downloading $url ..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$zipPath = Join-Path $env:TEMP $zipName

Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

Write-Host "Extracting to $InstallDir ..."
Expand-Archive -Path $zipPath -DestinationPath $InstallDir -Force
Remove-Item $zipPath

$exe = Join-Path $InstallDir "logcatool.exe"
if (-not (Test-Path $exe)) {
    throw "logcatool.exe was not found after extraction at $exe"
}

$installedVersion = & $exe -v 2>&1
Write-Host "Installed: $installedVersion"

Write-Host ""
Write-Host "logcatool was installed to: $InstallDir"
Write-Host "Add the directory to your PATH if it is not already included:"
Write-Host "    [Environment]::SetEnvironmentVariable('Path', `$env:Path + ';$InstallDir', 'User')"
Write-Host ""
Write-Host "Prerequisites for Windows 7:"
Write-Host "    - Windows 7 SP1 with Convenience Rollup KB3125574"
Write-Host "    - Android SDK Platform Tools (adb.exe) in PATH for live logcat"
