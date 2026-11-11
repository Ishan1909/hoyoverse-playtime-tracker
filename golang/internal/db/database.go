package db

import (
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"hoyoverse-tracker/internal/config"
)

var (
	winsqlite3 = windows.NewLazySystemDLL("winsqlite3.dll")

	procOpenV2   = winsqlite3.NewProc("sqlite3_open_v2")
	procClose    = winsqlite3.NewProc("sqlite3_close")
	procExec     = winsqlite3.NewProc("sqlite3_exec")
	procPrepare  = winsqlite3.NewProc("sqlite3_prepare_v2")
	procStep     = winsqlite3.NewProc("sqlite3_step")
	procFinalize = winsqlite3.NewProc("sqlite3_finalize")

	procBindText  = winsqlite3.NewProc("sqlite3_bind_text")
	procBindInt   = winsqlite3.NewProc("sqlite3_bind_int")
	procLastRowID = winsqlite3.NewProc("sqlite3_last_insert_rowid")

	procColText  = winsqlite3.NewProc("sqlite3_column_text")
	procColInt   = winsqlite3.NewProc("sqlite3_column_int")
	procColInt64 = winsqlite3.NewProc("sqlite3_column_int64")
)

const (
	SQLITE_ROW  = 100
	SQLITE_DONE = 101

	SQLITE_OPEN_READWRITE = 0x00000002
	SQLITE_OPEN_CREATE    = 0x00000004
)

var (
	dbHandle uintptr
	dbMutex  sync.Mutex
)

type SessionRecord struct {
	ID                int64  `json:"id"`
	GameKey           string `json:"game_key"`
	GameName          string `json:"game_name"`
	StartTime         string `json:"start_time"`
	EndTime           string `json:"end_time"`
	DurationSeconds   int    `json:"duration_seconds"`
	DurationFormatted string `json:"duration_formatted"`
	Date              string `json:"date"`
	Color             string `json:"color,omitempty"`
	Icon              string `json:"icon,omitempty"`
}

type PerGameStats struct {
	GameKey          string `json:"game_key"`
	GameName         string `json:"game_name"`
	Color            string `json:"color"`
	Icon             string `json:"icon"`
	SessionCount     int    `json:"session_count"`
	TotalSeconds     int    `json:"total_seconds"`
	TotalFormatted   string `json:"total_formatted"`
	LongestFormatted string `json:"longest_formatted"`
}

type MostPlayedInfo struct {
	GameKey   string `json:"game_key"`
	GameName  string `json:"game_name"`
	Seconds   int    `json:"seconds"`
	Formatted string `json:"formatted"`
}

type SummaryStats struct {
	TodaySeconds            int                     `json:"today_playtime_seconds"`
	TodayFormatted          string                  `json:"today_formatted"`
	TodayPlaytimeFormatted   string                  `json:"today_playtime_formatted"`
	WeekSeconds             int                     `json:"week_playtime_seconds"`
	WeekFormatted           string                  `json:"week_formatted"`
	WeekPlaytimeFormatted    string                  `json:"week_playtime_formatted"`
	TotalSeconds            int                     `json:"total_playtime_seconds"`
	TotalFormatted          string                  `json:"total_formatted"`
	TotalPlaytimeFormatted   string                  `json:"total_playtime_formatted"`
	LongestSeconds          int                     `json:"longest_session_seconds"`
	LongestFormatted        string                  `json:"longest_formatted"`
	LongestSessionFormatted string                  `json:"longest_session_formatted"`
	TotalSessions           int                     `json:"total_sessions"`
	MostPlayed              *MostPlayedInfo         `json:"most_played"`
	PerGame                 map[string]PerGameStats `json:"per_game"`
}

type ChartDataset struct {
	GameKey         string    `json:"game_key"`
	Label           string    `json:"label"`
	Color           string    `json:"color"`
	Data            []float64 `json:"data"`
	BackgroundColor string    `json:"backgroundColor"`
	BorderColor     string    `json:"borderColor"`
	BorderWidth     int       `json:"borderWidth"`
	BorderRadius    int       `json:"borderRadius"`
	Stack           string    `json:"stack"`
}

type ChartData struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
}

func FormatDuration(seconds int) string {
	if seconds <= 0 {
		return "0s"
	}
	hours := seconds / 3600
	mins := (seconds % 3600) / 60
	secs := seconds % 60
	if hours > 0 {
		if mins > 0 {
			return fmt.Sprintf("%dh %02dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if mins > 0 {
		if secs > 0 && mins < 5 {
			return fmt.Sprintf("%dm %ds", mins, secs)
		}
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%ds", secs)
}

func getDB() (uintptr, error) {
	if dbHandle != 0 {
		return dbHandle, nil
	}

	dbPath := config.GetDBPath()
	p, err := windows.BytePtrFromString(dbPath)
	if err != nil {
		return 0, err
	}

	var handle uintptr
	flags := uintptr(SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE)
	ret, _, _ := procOpenV2.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&handle)), flags, 0)
	if ret != 0 {
		return 0, fmt.Errorf("sqlite3_open_v2 error code: %d", ret)
	}

	dbHandle = handle
	return dbHandle, nil
}

func InitDB() error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	h, err := getDB()
	if err != nil {
		return err
	}

	initSQL := `
	PRAGMA journal_mode = WAL;
	PRAGMA busy_timeout = 5000;
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_key TEXT NOT NULL,
		game_name TEXT NOT NULL,
		start_time TEXT NOT NULL,
		end_time TEXT NOT NULL,
		duration_seconds INTEGER NOT NULL,
		date TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_date ON sessions(date);
	CREATE INDEX IF NOT EXISTS idx_sessions_game ON sessions(game_key);
	`
	q, _ := windows.BytePtrFromString(initSQL)
	procExec.Call(h, uintptr(unsafe.Pointer(q)), 0, 0, 0)
	return nil
}

func execPrepare(h uintptr, query string) (uintptr, error) {
	q, err := windows.BytePtrFromString(query)
	if err != nil {
		return 0, err
	}
	var stmt uintptr
	ret, _, _ := procPrepare.Call(h, uintptr(unsafe.Pointer(q)), uintptr(len(query)), uintptr(unsafe.Pointer(&stmt)), 0)
	if ret != 0 {
		return 0, fmt.Errorf("prepare failed: %d", ret)
	}
	return stmt, nil
}

func RecordSession(gameKey, gameName string, startTime, endTime time.Time, durationSec int) (int64, error) {
	if durationSec < 15 {
		return 0, nil
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	h, err := getDB()
	if err != nil {
		return 0, err
	}

	startStr := startTime.Format("2006-01-02 15:04:05")
	endStr := endTime.Format("2006-01-02 15:04:05")
	dateStr := startTime.Format("2006-01-02")

	// Deduplication: check if session already recorded within 60s
	checkSQL := `
	SELECT id FROM sessions
	WHERE game_key = ?
	  AND ABS(strftime('%s', start_time) - strftime('%s', ?)) <= 60
	  AND ABS(strftime('%s', end_time) - strftime('%s', ?)) <= 60
	LIMIT 1;
	`
	stmt, err := execPrepare(h, checkSQL)
	if err == nil {
		bKey, _ := windows.BytePtrFromString(gameKey)
		bStart, _ := windows.BytePtrFromString(startStr)
		bEnd, _ := windows.BytePtrFromString(endStr)

		procBindText.Call(stmt, 1, uintptr(unsafe.Pointer(bKey)), uintptr(len(gameKey)), 0)
		procBindText.Call(stmt, 2, uintptr(unsafe.Pointer(bStart)), uintptr(len(startStr)), 0)
		procBindText.Call(stmt, 3, uintptr(unsafe.Pointer(bEnd)), uintptr(len(endStr)), 0)

		stepRet, _, _ := procStep.Call(stmt)
		if stepRet == SQLITE_ROW {
			rowID, _, _ := procColInt64.Call(stmt, 0)
			procFinalize.Call(stmt)
			return int64(rowID), nil // already recorded
		}
		procFinalize.Call(stmt)
	}

	insertSQL := `INSERT INTO sessions (game_key, game_name, start_time, end_time, duration_seconds, date) VALUES (?, ?, ?, ?, ?, ?);`
	stmt, err = execPrepare(h, insertSQL)
	if err != nil {
		return 0, err
	}
	defer procFinalize.Call(stmt)

	bKey, _ := windows.BytePtrFromString(gameKey)
	bName, _ := windows.BytePtrFromString(gameName)
	bStart, _ := windows.BytePtrFromString(startStr)
	bEnd, _ := windows.BytePtrFromString(endStr)
	bDate, _ := windows.BytePtrFromString(dateStr)

	procBindText.Call(stmt, 1, uintptr(unsafe.Pointer(bKey)), uintptr(len(gameKey)), 0)
	procBindText.Call(stmt, 2, uintptr(unsafe.Pointer(bName)), uintptr(len(gameName)), 0)
	procBindText.Call(stmt, 3, uintptr(unsafe.Pointer(bStart)), uintptr(len(startStr)), 0)
	procBindText.Call(stmt, 4, uintptr(unsafe.Pointer(bEnd)), uintptr(len(endStr)), 0)
	procBindInt.Call(stmt, 5, uintptr(durationSec))
	procBindText.Call(stmt, 6, uintptr(unsafe.Pointer(bDate)), uintptr(len(dateStr)), 0)

	procStep.Call(stmt)
	lastID, _, _ := procLastRowID.Call(h)
	return int64(lastID), nil
}

func GetSummary() (SummaryStats, error) {
	var summary SummaryStats
	summary.PerGame = make(map[string]PerGameStats)

	dbMutex.Lock()
	defer dbMutex.Unlock()

	h, err := getDB()
	if err != nil {
		return summary, err
	}

	cfg := config.LoadConfig()
	todayStr := time.Now().Format("2006-01-02")
	weekStr := time.Now().AddDate(0, 0, -6).Format("2006-01-02")

	// 1. Overall totals
	if stmt, err := execPrepare(h, `SELECT COUNT(*), COALESCE(SUM(duration_seconds), 0), COALESCE(MAX(duration_seconds), 0) FROM sessions;`); err == nil {
		if r, _, _ := procStep.Call(stmt); r == SQLITE_ROW {
			cnt, _, _ := procColInt.Call(stmt, 0)
			tot, _, _ := procColInt.Call(stmt, 1)
			lng, _, _ := procColInt.Call(stmt, 2)
			summary.TotalSessions = int(cnt)
			summary.TotalSeconds = int(tot)
			summary.LongestSeconds = int(lng)
		}
		procFinalize.Call(stmt)
	}
	summary.TotalFormatted = FormatDuration(summary.TotalSeconds)
	summary.TotalPlaytimeFormatted = summary.TotalFormatted
	summary.LongestFormatted = FormatDuration(summary.LongestSeconds)
	summary.LongestSessionFormatted = summary.LongestFormatted

	// 2. Today's total
	if stmt, err := execPrepare(h, `SELECT COALESCE(SUM(duration_seconds), 0) FROM sessions WHERE date = ?;`); err == nil {
		bToday, _ := windows.BytePtrFromString(todayStr)
		procBindText.Call(stmt, 1, uintptr(unsafe.Pointer(bToday)), uintptr(len(todayStr)), 0)
		if r, _, _ := procStep.Call(stmt); r == SQLITE_ROW {
			tot, _, _ := procColInt.Call(stmt, 0)
			summary.TodaySeconds = int(tot)
		}
		procFinalize.Call(stmt)
	}
	summary.TodayFormatted = FormatDuration(summary.TodaySeconds)
	summary.TodayPlaytimeFormatted = summary.TodayFormatted

	// 3. Past 7 days total
	if stmt, err := execPrepare(h, `SELECT COALESCE(SUM(duration_seconds), 0) FROM sessions WHERE date >= ?;`); err == nil {
		bWeek, _ := windows.BytePtrFromString(weekStr)
		procBindText.Call(stmt, 1, uintptr(unsafe.Pointer(bWeek)), uintptr(len(weekStr)), 0)
		if r, _, _ := procStep.Call(stmt); r == SQLITE_ROW {
			tot, _, _ := procColInt.Call(stmt, 0)
			summary.WeekSeconds = int(tot)
		}
		procFinalize.Call(stmt)
	}
	summary.WeekFormatted = FormatDuration(summary.WeekSeconds)
	summary.WeekPlaytimeFormatted = summary.WeekFormatted

	// 4. Per-game breakdown
	if stmt, err := execPrepare(h, `SELECT game_key, game_name, COUNT(*), SUM(duration_seconds), MAX(duration_seconds) FROM sessions GROUP BY game_key;`); err == nil {
		for {
			r, _, _ := procStep.Call(stmt)
			if r != SQLITE_ROW {
				break
			}
			tKey, _, _ := procColText.Call(stmt, 0)
			tName, _, _ := procColText.Call(stmt, 1)
			cnt, _, _ := procColInt.Call(stmt, 2)
			tot, _, _ := procColInt.Call(stmt, 3)
			lng, _, _ := procColInt.Call(stmt, 4)

			key := windows.BytePtrToString((*byte)(unsafe.Pointer(tKey)))
			name := windows.BytePtrToString((*byte)(unsafe.Pointer(tName)))

			color := "#4a90e2"
			icon := "🎮"
			if g, ok := cfg.Games[key]; ok {
				color = g.Color
				icon = g.Icon
			}

			summary.PerGame[key] = PerGameStats{
				GameKey:          key,
				GameName:         name,
				Color:            color,
				Icon:             icon,
				SessionCount:     int(cnt),
				TotalSeconds:     int(tot),
				TotalFormatted:   FormatDuration(int(tot)),
				LongestFormatted: FormatDuration(int(lng)),
			}
		}
		procFinalize.Call(stmt)
	}

	// Configured games with 0 playtime
	for k, g := range cfg.Games {
		if _, exists := summary.PerGame[k]; !exists {
			summary.PerGame[k] = PerGameStats{
				GameKey:          k,
				GameName:         g.Name,
				Color:            g.Color,
				Icon:             g.Icon,
				SessionCount:     0,
				TotalSeconds:     0,
				TotalFormatted:   "0s",
				LongestFormatted: "0s",
			}
		}
	}

	// 5. Most played game calculation
	maxPlayedSec := 0
	for _, pg := range summary.PerGame {
		if pg.TotalSeconds > maxPlayedSec {
			maxPlayedSec = pg.TotalSeconds
			summary.MostPlayed = &MostPlayedInfo{
				GameKey:   pg.GameKey,
				GameName:  pg.GameName,
				Seconds:   pg.TotalSeconds,
				Formatted: pg.TotalFormatted,
			}
		}
	}

	return summary, nil
}

func GetDailyChartData(days int) (ChartData, error) {
	var chart ChartData
	chart.Labels = make([]string, 0, days)
	chart.Datasets = make([]ChartDataset, 0)

	dbMutex.Lock()
	defer dbMutex.Unlock()

	h, err := getDB()
	if err != nil {
		return chart, err
	}

	cfg := config.LoadConfig()

	dates := make([]string, days)
	now := time.Now()
	for i := days - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		dates[days-1-i] = d
		t, _ := time.Parse("2006-01-02", d)
		chart.Labels = append(chart.Labels, t.Format("02 Jan"))
	}

	startDate := dates[0]
	gameDateHours := make(map[string]map[string]float64)

	chartSQL := `SELECT date, game_key, SUM(duration_seconds) FROM sessions WHERE date >= ? GROUP BY date, game_key;`
	if stmt, err := execPrepare(h, chartSQL); err == nil {
		bStart, _ := windows.BytePtrFromString(startDate)
		procBindText.Call(stmt, 1, uintptr(unsafe.Pointer(bStart)), uintptr(len(startDate)), 0)

		for {
			r, _, _ := procStep.Call(stmt)
			if r != SQLITE_ROW {
				break
			}
			tDate, _, _ := procColText.Call(stmt, 0)
			tKey, _, _ := procColText.Call(stmt, 1)
			sec, _, _ := procColInt.Call(stmt, 2)

			date := windows.BytePtrToString((*byte)(unsafe.Pointer(tDate)))
			key := windows.BytePtrToString((*byte)(unsafe.Pointer(tKey)))

			if _, ok := gameDateHours[key]; !ok {
				gameDateHours[key] = make(map[string]float64)
			}
			hours := math.Round((float64(sec)/3600.0)*100) / 100
			gameDateHours[key][date] = hours
		}
		procFinalize.Call(stmt)
	}

	for k, g := range cfg.Games {
		dataPoints := make([]float64, days)
		hasData := false
		for idx, d := range dates {
			if h, exists := gameDateHours[k][d]; exists {
				dataPoints[idx] = h
				if h > 0 {
					hasData = true
				}
			} else {
				dataPoints[idx] = 0.0
			}
		}
		if hasData || k == "genshin" || k == "star_rail" {
			chart.Datasets = append(chart.Datasets, ChartDataset{
				GameKey:         k,
				Label:           g.Name,
				Color:           g.Color,
				Data:            dataPoints,
				BackgroundColor: g.Color + "aa",
				BorderColor:     g.Color,
				BorderWidth:     1,
				BorderRadius:    4,
				Stack:           "playtime",
			})
		}
	}

	return chart, nil
}

func GetRecentSessions(limit int) ([]SessionRecord, error) {
	sessions := make([]SessionRecord, 0)

	dbMutex.Lock()
	defer dbMutex.Unlock()

	h, err := getDB()
	if err != nil {
		return sessions, err
	}

	cfg := config.LoadConfig()

	sqlQuery := `SELECT id, game_key, game_name, start_time, end_time, duration_seconds, date FROM sessions ORDER BY id DESC LIMIT ?;`
	stmt, err := execPrepare(h, sqlQuery)
	if err != nil {
		return sessions, err
	}
	defer procFinalize.Call(stmt)

	procBindInt.Call(stmt, 1, uintptr(limit))

	for {
		r, _, _ := procStep.Call(stmt)
		if r != SQLITE_ROW {
			break
		}
		rowID, _, _ := procColInt64.Call(stmt, 0)
		tKey, _, _ := procColText.Call(stmt, 1)
		tName, _, _ := procColText.Call(stmt, 2)
		tStart, _, _ := procColText.Call(stmt, 3)
		tEnd, _, _ := procColText.Call(stmt, 4)
		durSec, _, _ := procColInt.Call(stmt, 5)
		tDate, _, _ := procColText.Call(stmt, 6)

		key := windows.BytePtrToString((*byte)(unsafe.Pointer(tKey)))
		name := windows.BytePtrToString((*byte)(unsafe.Pointer(tName)))
		start := windows.BytePtrToString((*byte)(unsafe.Pointer(tStart)))
		end := windows.BytePtrToString((*byte)(unsafe.Pointer(tEnd)))
		date := windows.BytePtrToString((*byte)(unsafe.Pointer(tDate)))

		color := "#4a90e2"
		icon := "🎮"
		if g, ok := cfg.Games[key]; ok {
			color = g.Color
			icon = g.Icon
		}

		sessions = append(sessions, SessionRecord{
			ID:                int64(rowID),
			GameKey:           key,
			GameName:          name,
			StartTime:         start,
			EndTime:           end,
			DurationSeconds:   int(durSec),
			DurationFormatted: FormatDuration(int(durSec)),
			Date:              date,
			Color:             color,
			Icon:              icon,
		})
	}

	return sessions, nil
}
