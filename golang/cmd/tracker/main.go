package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"hoyoverse-tracker/internal/config"
	"hoyoverse-tracker/internal/db"
	"hoyoverse-tracker/internal/monitor"
	"hoyoverse-tracker/internal/server"
	"hoyoverse-tracker/internal/tray"
	"hoyoverse-tracker/internal/window"
)

const (
	MutexName            = "Local\\HoyoversePlaytimeTrackerMutex"
	ERROR_ALREADY_EXISTS = 183
)

func createSingleInstanceMutex() (windows.Handle, bool) {
	namePtr, _ := windows.UTF16PtrFromString(MutexName)
	hMutex, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil && err == windows.ERROR_ALREADY_EXISTS {
		return hMutex, true
	}
	return hMutex, false
}

func main() {
	noTray := flag.Bool("no-tray", false, "Run without system tray icon")
	openDashboard := flag.Bool("open-dashboard", false, "Open dashboard in app window")
	background := flag.Bool("background", false, "Start silently in background")
	flag.Parse()

	cfg := config.LoadConfig()
	dashboardURL := cfg.Settings.DashboardURL
	if dashboardURL == "" {
		dashboardURL = fmt.Sprintf("http://%s:%d", cfg.Settings.ServerHost, cfg.Settings.ServerPort)
	}

	// 0. Single-Instance Check via Windows Named Mutex
	hMutex, alreadyRunning := createSingleInstanceMutex()
	if alreadyRunning {
		if *openDashboard {
			window.OpenAppWindow(dashboardURL)
		}
		os.Exit(0)
	}
	defer windows.CloseHandle(hMutex)

	// Set up file logging to %LOCALAPPDATA%\HoYoverseTracker\data\tracker.log
	logFile, err := os.OpenFile(config.GetLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}
	log.Println("[INFO] Starting HoYoverse Playtime Tracker (Go Native Optimizer)...")

	// 1. Initialize SQLite Database
	if err := db.InitDB(); err != nil {
		log.Printf("[ERROR] Database initialization failed: %v\n", err)
		os.Exit(1)
	}
	log.Println("[INFO] Database initialized successfully.")

	// 2. Start Background Playtime Monitor
	monitor.MonitorInstance.Start()
	log.Println("[INFO] Background playtime monitor thread started.")

	// 3. Start Embedded HTTP Dashboard Server
	serverAddr := fmt.Sprintf("%s:%d", cfg.Settings.ServerHost, cfg.Settings.ServerPort)
	httpServer, err := server.StartServer(serverAddr)
	if err != nil {
		log.Printf("[ERROR] Failed to start HTTP dashboard server: %v\n", err)
	} else {
		log.Printf("[INFO] Web dashboard active on http://%s\n", serverAddr)
	}

	// Brief pause to allow HTTP server binding
	time.Sleep(300 * time.Millisecond)

	// If launched explicitly from shortcut or without --background flag, open dashboard
	if *openDashboard || !*background {
		window.OpenAppWindow(dashboardURL)
	}

	// Trim memory on startup
	debug.FreeOSMemory()
	monitor.TrimWorkingSet()

	// Clean shutdown handler
	shutdown := func() {
		log.Println("[INFO] Stopping tracker and server...")
		monitor.MonitorInstance.Stop()
		if httpServer != nil {
			_ = httpServer.Close()
		}
		log.Println("[INFO] Clean shutdown complete.")
		os.Exit(0)
	}

	// Handle OS interrupt signals (Ctrl+C, SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		shutdown()
	}()

	// 4. Start Windows System Tray App
	if !*noTray {
		log.Println("[INFO] Starting Win32 system tray icon.")
		trayApp := tray.NewTrayApp(dashboardURL, shutdown)
		if err := trayApp.Run(); err != nil {
			log.Printf("[WARNING] System tray run error: %v. Continuing in background.\n", err)
		}
	}

	// Keep alive if tray finishes or was disabled
	select {}
}
