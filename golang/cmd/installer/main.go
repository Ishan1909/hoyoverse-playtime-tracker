package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

//go:embed HoYoverseTracker.exe
var trackerBinary []byte

//go:embed app_icon.ico
var appIconBytes []byte

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	procMessageBoxW       = user32.NewProc("MessageBoxW")
	procSHChangeNotify    = shell32.NewProc("SHChangeNotify")
)

const (
	MB_YESNO       = 0x00000004
	MB_ICONQUESTION = 0x00000020
	MB_ICONINFO     = 0x00000040
	IDYES           = 6
)

func showDialog(title, text string, flags uint32) int {
	tPtr, _ := windows.UTF16PtrFromString(title)
	mPtr, _ := windows.UTF16PtrFromString(text)
	ret, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), uintptr(flags))
	return int(ret)
}

func runCmdSilent(cmdStr string) {
	cmd := exec.Command("cmd", "/c", cmdStr)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	_ = cmd.Run()
}

func regAdd(key, valueName, valType, data string) {
	var cmd *exec.Cmd
	if valType == "REG_DWORD" {
		cmd = exec.Command("reg", "add", key, "/v", valueName, "/t", valType, "/d", data, "/f")
	} else if valueName == "" {
		cmd = exec.Command("reg", "add", key, "/ve", "/t", valType, "/d", data, "/f")
	} else {
		cmd = exec.Command("reg", "add", key, "/v", valueName, "/t", valType, "/d", data, "/f")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_ = cmd.Run()
}

func main() {
	isSilent := false
	for _, arg := range os.Args[1:] {
		if arg == "--silent" || arg == "-s" || arg == "/S" || arg == "/silent" {
			isSilent = true
		}
	}

	// 0. Prompt User (unless silent)
	if !isSilent {
		res := showDialog(
			"HoYoverse Playtime Tracker Setup",
			"Welcome to HoYoverse Playtime Tracker Setup!\n\n"+
				"This will install the ultra-optimized (< 5 MB RAM) background playtime tracker and dashboard:\n\n"+
				"• Track Genshin Impact & Honkai: Star Rail playtime\n"+
				"• Safe playtime database preserved in %LOCALAPPDATA%\n"+
				"• Silent headless auto-start on Windows login\n"+
				"• Start Menu and Desktop shortcuts\n\n"+
				"Do you want to install HoYoverse Playtime Tracker?",
			MB_YESNO|MB_ICONQUESTION,
		)
		if res != IDYES {
			os.Exit(0)
		}
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	appData := os.Getenv("APPDATA")
	homeDir, _ := os.UserHomeDir()

	installDir := filepath.Join(localAppData, "Programs", "HoYoversePlaytimeTracker")
	dataDir := filepath.Join(localAppData, "HoYoverseTracker", "data")
	desktopDir := filepath.Join(homeDir, "Desktop")
	startMenuDir := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs")
	startFolder := filepath.Join(startMenuDir, "HoYoverse Playtime Tracker")

	_ = os.MkdirAll(installDir, 0755)
	_ = os.MkdirAll(dataDir, 0755)
	_ = os.MkdirAll(startFolder, 0755)

	// 1. Terminate old instances
	runCmdSilent("taskkill /F /IM HoYoverseTracker.exe /T 2>nul")
	time.Sleep(500 * time.Millisecond)

	// Clean up legacy Python files if upgrading
	_ = os.RemoveAll(filepath.Join(installDir, "_internal"))

	// 2. Write embedded binary and icon
	exePath := filepath.Join(installDir, "HoYoverseTracker.exe")
	if err := os.WriteFile(exePath, trackerBinary, 0755); err != nil {
		showDialog("Installation Error", fmt.Sprintf("Failed to install application file:\n%v", err), 0x10)
		os.Exit(1)
	}

	iconPath := filepath.Join(installDir, "app_icon.ico")
	if len(appIconBytes) > 0 {
		_ = os.WriteFile(iconPath, appIconBytes, 0644)
	}

	// 3. Create Desktop and Start Menu Shortcuts via Windows Script Host
	psShortcutScript := fmt.Sprintf(`
$WshShell = New-Object -ComObject WScript.Shell
$targets = @(
    '%s',
    '%s',
    '%s',
    '%s'
)
foreach ($p in $targets) {
    $s = $WshShell.CreateShortcut($p)
    $s.TargetPath = '%s'
    $s.Arguments = '--open-dashboard'
    $s.WorkingDirectory = '%s'
    $s.Description = 'HoYoverse Playtime Tracker Dashboard'
    $s.IconLocation = '%s, 0'
    $s.Save()
}
`,
		filepath.Join(desktopDir, "HoYoverse Playtime Tracker.lnk"),
		filepath.Join(startMenuDir, "HoYoverse Playtime Tracker.lnk"),
		filepath.Join(startMenuDir, "HoYoverse Tracker.lnk"),
		filepath.Join(startFolder, "HoYoverse Playtime Tracker.lnk"),
		exePath,
		installDir,
		iconPath,
	)

	psCmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psShortcutScript)
	psCmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_ = psCmd.Run()

	// 4. Create Uninstall.bat
	uninstScript := filepath.Join(installDir, "Uninstall.bat")
	uninstContent := fmt.Sprintf(`@echo off
title Uninstall HoYoverse Playtime Tracker
echo Stopping HoYoverse Playtime Tracker...
taskkill /F /IM HoYoverseTracker.exe /T 2>nul
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v "HoYoversePlaytimeTracker" /f 2>nul
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\HoYoversePlaytimeTracker" /f 2>nul
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths\HoYoverseTracker.exe" /f 2>nul
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths\hoyoverse.exe" /f 2>nul
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths\hoyoverse-tracker.exe" /f 2>nul

echo Removing shortcuts...
del "%s" 2>nul
del "%s" 2>nul
del "%s" 2>nul
rmdir /s /q "%s" 2>nul

echo Note: Your playtime database is safely preserved at:
echo %%LOCALAPPDATA%%\HoYoverseTracker\data\tracker.db
echo.
set /p DEL_DB="Do you also want to delete your saved playtime database? (y/N): "
if /i "%%DEL_DB%%"=="y" (
    rmdir /s /q "%s" 2>nul
    echo Playtime database deleted.
)

echo Removing application files...
cd /d "%%TEMP%%"
rmdir /s /q "%s" 2>nul

echo.
echo HoYoverse Playtime Tracker has been successfully uninstalled.
pause
`,
		filepath.Join(desktopDir, "HoYoverse Playtime Tracker.lnk"),
		filepath.Join(startMenuDir, "HoYoverse Playtime Tracker.lnk"),
		filepath.Join(startMenuDir, "HoYoverse Tracker.lnk"),
		startFolder,
		filepath.Join(localAppData, "HoYoverseTracker"),
		installDir,
	)
	_ = os.WriteFile(uninstScript, []byte(uninstContent), 0644)

	// 5. Register in Windows Settings -> Apps -> Installed Apps
	regUninst := `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\HoYoversePlaytimeTracker`
	regAdd(regUninst, "DisplayName", "REG_SZ", "HoYoverse Playtime Tracker")
	regAdd(regUninst, "DisplayVersion", "REG_SZ", "2.0.0")
	regAdd(regUninst, "Publisher", "REG_SZ", "HoYoverse Playtime Tracker")
	regAdd(regUninst, "UninstallString", "REG_SZ", fmt.Sprintf("\"%s\"", uninstScript))
	regAdd(regUninst, "DisplayIcon", "REG_SZ", fmt.Sprintf("\"%s\"", iconPath))
	regAdd(regUninst, "InstallLocation", "REG_SZ", fmt.Sprintf("\"%s\"", installDir))
	regAdd(regUninst, "NoModify", "REG_DWORD", "1")
	regAdd(regUninst, "NoRepair", "REG_DWORD", "1")
	regAdd(regUninst, "EstimatedSize", "REG_DWORD", "15000")

	// 6. Register App Paths for Windows Search
	for _, alias := range []string{"HoYoverseTracker.exe", "hoyoverse.exe", "hoyoverse-tracker.exe"} {
		regApp := fmt.Sprintf(`HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths\%s`, alias)
		regAdd(regApp, "", "REG_SZ", fmt.Sprintf("\"%s\"", exePath))
		regAdd(regApp, "Path", "REG_SZ", fmt.Sprintf("\"%s\"", installDir))
	}

	// 7. Register Startup App with --background
	regRun := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	regAdd(regRun, "HoYoversePlaytimeTracker", "REG_SZ", fmt.Sprintf("\"%s\" --background", exePath))

	// 8. Refresh Windows Shell Cache
	procSHChangeNotify.Call(0x08000000, 0x1000, 0, 0)

	// 9. Start Tracker in background
	launchCmd := exec.Command(exePath, "--background")
	launchCmd.Dir = installDir
	launchCmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000 | 0x00000200,
	}
	_ = launchCmd.Start()

	// 10. Success Dialog & Open Dashboard
	if !isSilent {
		showDialog(
			"HoYoverse Playtime Tracker Setup",
			"HoYoverse Playtime Tracker has been installed successfully!\n\n"+
				"• Optimized tracker active in background (< 5 MB RAM)\n"+
				"• Desktop & Start Menu shortcuts created\n"+
				"• Playtime database safely preserved\n\n"+
				"Click OK to open your dashboard.",
			MB_ICONINFO,
		)
	}

	// Open standalone app window
	dashCmd := exec.Command(exePath, "--open-dashboard")
	dashCmd.Dir = installDir
	dashCmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000,
	}
	_ = dashCmd.Start()
}
