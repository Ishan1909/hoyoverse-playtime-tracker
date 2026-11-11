$ErrorActionPreference = "Stop"

$goPath = "C:\Program Files\Go\bin"
if (Test-Path "$goPath\go.exe") {
    $env:Path = "$goPath;$env:Path"
}
$goBin = "$env:USERPROFILE\go\bin"
if (Test-Path $goBin) {
    $env:Path = "$goBin;$env:Path"
}

$optDir = $PSScriptRoot
Set-Location $optDir

$releaseDir = Join-Path $optDir "release"
New-Item -ItemType Directory -Path $releaseDir -Force | Out-Null

$iconSrc = Join-Path $optDir "ui\static\app_icon.ico"
$installerIcon = Join-Path $optDir "cmd\installer\app_icon.ico"
Copy-Item -Path $iconSrc -Destination $installerIcon -Force

Write-Host "=== 0. Compiling Windows Icon Resources (.syso) ===" -ForegroundColor Cyan
if (Get-Command go-winres -ErrorAction SilentlyContinue) {
    Set-Location (Join-Path $optDir "cmd\tracker")
    go-winres simply --icon $iconSrc --manifest gui --file-description "HoYoverse Playtime Tracker" --product-name "HoYoverse Playtime Tracker"
    
    Set-Location (Join-Path $optDir "cmd\installer")
    go-winres simply --icon $iconSrc --manifest gui --file-description "HoYoverse Playtime Tracker Setup" --product-name "HoYoverse Playtime Tracker Setup"
    
    Set-Location $optDir
}

Write-Host "`n=== 1. Building HoYoverseTracker.exe (Native Windows GUI) ===" -ForegroundColor Cyan
# -H=windowsgui makes it run as a native GUI process with no console window
# -s -w strips debug symbols and symbols table for smallest binary size
$trackerExe = Join-Path $optDir "cmd\installer\HoYoverseTracker.exe"
go build -ldflags "-s -w -H=windowsgui" -o $trackerExe ./cmd/tracker

$trackerRelease = Join-Path $releaseDir "HoYoverseTracker.exe"
Copy-Item -Path $trackerExe -Destination $trackerRelease -Force

$trackerSize = (Get-Item $trackerRelease).Length / 1MB
Write-Host "HoYoverseTracker.exe built successfully ($([math]::Round($trackerSize, 2)) MB)" -ForegroundColor Green

Write-Host "`n=== 2. Building Setup-HoYoverseTracker.exe (Self-Contained Installer) ===" -ForegroundColor Cyan
$installerRelease = Join-Path $releaseDir "Setup-HoYoverseTracker.exe"
go build -ldflags "-s -w -H=windowsgui" -o $installerRelease ./cmd/installer

$installerSize = (Get-Item $installerRelease).Length / 1MB
Write-Host "Setup-HoYoverseTracker.exe built successfully ($([math]::Round($installerSize, 2)) MB)" -ForegroundColor Green

Write-Host "`n=== Build Complete! Release files located in: $releaseDir ===" -ForegroundColor Yellow
Get-ChildItem $releaseDir | Select-Object Name, Length, LastWriteTime
