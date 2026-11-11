package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"

	"hoyoverse-tracker/internal/config"
	"hoyoverse-tracker/internal/db"
	"hoyoverse-tracker/internal/monitor"
	"hoyoverse-tracker/ui"
)

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	status := monitor.MonitorInstance.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func handleSummary(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	summary, err := db.GetSummary()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func handleChart(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	days := 14
	if dStr := r.URL.Query().Get("days"); dStr != "" {
		if d, err := strconv.Atoi(dStr); err == nil && d > 0 {
			days = d
		}
	}
	chartData, err := db.GetDailyChartData(days)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chartData)
}

func handleSessions(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	sessions, err := db.GetRecentSessions(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	cfg := config.LoadConfig()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

func StartServer(addr string) (*http.Server, error) {
	mux := http.NewServeMux()

	// REST Endpoints
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/summary", handleSummary)
	mux.HandleFunc("/api/chart", handleChart)
	mux.HandleFunc("/api/sessions", handleSessions)
	mux.HandleFunc("/api/config", handleConfig)

	// Embedded Static & Assets Handlers
	staticSub, _ := fs.Sub(ui.EmbeddedFS, "static")
	assetsSub, _ := fs.Sub(ui.EmbeddedFS, "assets")

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSub))))

	// Manifest and ServiceWorker
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(ui.EmbeddedFS, "static/manifest.json")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/manifest+json")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(ui.EmbeddedFS, "static/sw.js")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write(data)
	})

	// Root Index Handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := fs.ReadFile(ui.EmbeddedFS, "static/index.html")
		if err != nil {
			http.Error(w, "Dashboard not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		_ = srv.ListenAndServe()
	}()

	return srv, nil
}
