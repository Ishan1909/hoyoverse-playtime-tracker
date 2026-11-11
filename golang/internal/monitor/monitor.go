package monitor

import (
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"hoyoverse-tracker/internal/config"
	"hoyoverse-tracker/internal/db"
)

var (
	kernel32                    = windows.NewLazySystemDLL("kernel32.dll")
	procSetProcessWorkingSetSize = kernel32.NewProc("SetProcessWorkingSetSize")
)

type ActiveGameInfo struct {
	GameKey                  string    `json:"game_key"`
	GameName                 string    `json:"game_name"`
	PID                      uint32    `json:"pid"`
	StartTime                time.Time `json:"start_time"`
	StartTimeFormatted       string    `json:"start_time_formatted"`
	CurrentDurationSeconds   int       `json:"current_duration_seconds"`
	CurrentDurationFormatted string    `json:"current_duration_formatted"`
	Color                    string    `json:"color"`
	Icon                     string    `json:"icon"`
}

type TrackerStatus struct {
	IsGaming       bool                      `json:"is_gaming"`
	ActiveGames    map[string]ActiveGameInfo `json:"active_games"`
	PrimarySession *ActiveGameInfo           `json:"primary_session"`
	TrackedGames   []string                  `json:"tracked_games"`
	LastPollTime   string                    `json:"last_poll_time"`
}

type ProcessMonitor struct {
	mu           sync.RWMutex
	running      bool
	stopChan     chan struct{}
	activeGames  map[string]*ActiveGameInfo
	activePIDs   map[string]uint32
	pollInterval time.Duration
}

var MonitorInstance = NewProcessMonitor()

func NewProcessMonitor() *ProcessMonitor {
	return &ProcessMonitor{
		activeGames:  make(map[string]*ActiveGameInfo),
		activePIDs:   make(map[string]uint32),
		pollInterval: 3 * time.Second,
		stopChan:     make(chan struct{}),
	}
}

func (m *ProcessMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	// Initial memory trim on startup
	debug.FreeOSMemory()
	TrimWorkingSet()

	go m.loop()
}

func (m *ProcessMonitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopChan)
	m.mu.Unlock()

	// Flush any active sessions to DB on stop
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for key, act := range m.activeGames {
		durationSec := int(now.Sub(act.StartTime).Seconds())
		if durationSec >= 15 {
			_, _ = db.RecordSession(act.GameKey, act.GameName, act.StartTime, now, durationSec)
		}
		delete(m.activeGames, key)
		delete(m.activePIDs, key)
	}
}

func (m *ProcessMonitor) loop() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Memory trim counter: trim memory every 10 cycles when idle
	trimCounter := 0

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.poll()

			// Memory optimization: if no game is running, occasionally trim working set
			m.mu.RLock()
			isGaming := len(m.activeGames) > 0
			m.mu.RUnlock()

			if !isGaming {
				trimCounter++
				if trimCounter >= 3 { // roughly every 9 seconds
					trimCounter = 0
					debug.FreeOSMemory()
					TrimWorkingSet()
				}
			}
		}
	}
}

// Win32 Toolhelp32 Process Polling (Near 0 MB overhead)
func (m *ProcessMonitor) poll() {
	cfg := config.LoadConfig()

	// Map executable name (lowercase) -> gameKey
	exeMap := make(map[string]string)
	for key, g := range cfg.Games {
		for _, exe := range g.Executables {
			exeMap[strings.ToLower(exe)] = key
		}
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	foundGames := make(map[string]uint32) // gameKey -> PID

	if err := windows.Process32First(snapshot, &entry); err == nil {
		for {
			exeName := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
			if key, ok := exeMap[exeName]; ok {
				foundGames[key] = entry.ProcessID
			}

			if err := windows.Process32Next(snapshot, &entry); err != nil {
				break
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 1. Detect newly launched games
	for key, pid := range foundGames {
		if _, exists := m.activeGames[key]; !exists {
			gmeta := cfg.Games[key]
			m.activeGames[key] = &ActiveGameInfo{
				GameKey:            key,
				GameName:           gmeta.Name,
				PID:                pid,
				StartTime:          now,
				StartTimeFormatted: now.Format("15:04:05"),
				Color:              gmeta.Color,
				Icon:               gmeta.Icon,
			}
			m.activePIDs[key] = pid
		}
	}

	// 2. Detect closed games
	for key, act := range m.activeGames {
		if _, stillRunning := foundGames[key]; !stillRunning {
			// Game terminated: record session
			durationSec := int(now.Sub(act.StartTime).Seconds())
			if durationSec >= 15 {
				_, _ = db.RecordSession(act.GameKey, act.GameName, act.StartTime, now, durationSec)
			}
			delete(m.activeGames, key)
			delete(m.activePIDs, key)
		}
	}
}

func (m *ProcessMonitor) GetStatus() TrackerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg := config.LoadConfig()
	tracked := make([]string, 0, len(cfg.Games))
	for _, g := range cfg.Games {
		tracked = append(tracked, g.Name)
	}

	now := time.Now()
	activeMap := make(map[string]ActiveGameInfo)
	var primary *ActiveGameInfo

	for k, act := range m.activeGames {
		durSec := int(now.Sub(act.StartTime).Seconds())
		info := ActiveGameInfo{
			GameKey:                  act.GameKey,
			GameName:                 act.GameName,
			PID:                      act.PID,
			StartTime:                act.StartTime,
			StartTimeFormatted:       act.StartTimeFormatted,
			CurrentDurationSeconds:   durSec,
			CurrentDurationFormatted: db.FormatDuration(durSec),
			Color:                    act.Color,
			Icon:                     act.Icon,
		}
		activeMap[k] = info
		if primary == nil {
			pCopy := info
			primary = &pCopy
		}
	}

	return TrackerStatus{
		IsGaming:       len(activeMap) > 0,
		ActiveGames:    activeMap,
		PrimarySession: primary,
		TrackedGames:   tracked,
		LastPollTime:   now.Format("15:04:05"),
	}
}

// TrimWorkingSet calls Windows SetProcessWorkingSetSize to minimize RAM usage to ~2 MB
func TrimWorkingSet() {
	if runtime.GOOS == "windows" {
		currentProcess := windows.CurrentProcess()
		_, _, _ = procSetProcessWorkingSetSize.Call(uintptr(currentProcess), ^uintptr(0), ^uintptr(0))
	}
}
