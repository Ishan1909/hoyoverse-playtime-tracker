package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type GameConfig struct {
	Name        string   `json:"name"`
	Executables []string `json:"executables"`
	Color       string   `json:"color"`
	Icon        string   `json:"icon"`
}

type Settings struct {
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	ServerHost          string `json:"server_host"`
	ServerPort          int    `json:"server_port"`
	DashboardURL        string `json:"dashboard_url"`
}

type AppConfig struct {
	Settings Settings              `json:"settings"`
	Games    map[string]GameConfig `json:"games"`
}

var DefaultGames = map[string]GameConfig{
	"genshin": {
		Name:        "Genshin Impact",
		Executables: []string{"GenshinImpact.exe", "YuanShen.exe"},
		Color:       "#00d2be",
		Icon:        "/assets/Genshin_Impact_logo.webp",
	},
	"star_rail": {
		Name:        "Honkai: Star Rail",
		Executables: []string{"StarRail.exe"},
		Color:       "#e0a354",
		Icon:        "/assets/HSR_logo.png",
	},
	"honkai_3rd": {
		Name:        "Honkai Impact 3rd",
		Executables: []string{"BH3.exe"},
		Color:       "#9b51e0",
		Icon:        "⚔️",
	},
}

var DefaultSettings = Settings{
	PollIntervalSeconds: 3,
	ServerHost:          "127.0.0.1",
	ServerPort:          5000,
	DashboardURL:        "http://127.0.0.1:5000",
}

func GetDataDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			localAppData = filepath.Join(home, "AppData", "Local")
		} else {
			localAppData = "."
		}
	}
	dir := filepath.Join(localAppData, "HoYoverseTracker", "data")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func GetDBPath() string {
	return filepath.Join(GetDataDir(), "tracker.db")
}

func GetLogPath() string {
	return filepath.Join(GetDataDir(), "tracker.log")
}

func GetConfigFile() string {
	return filepath.Join(GetDataDir(), "games_config.json")
}

func LoadConfig() AppConfig {
	cfgFile := GetConfigFile()
	data, err := os.ReadFile(cfgFile)
	if err == nil {
		var cfg AppConfig
		if err := json.Unmarshal(data, &cfg); err == nil {
			if cfg.Games == nil || len(cfg.Games) == 0 {
				cfg.Games = DefaultGames
			}
			if cfg.Settings.ServerPort == 0 {
				cfg.Settings = DefaultSettings
			}
			return cfg
		}
	}

	cfg := AppConfig{
		Settings: DefaultSettings,
		Games:    DefaultGames,
	}
	if bytes, err := json.MarshalIndent(cfg, "", "    "); err == nil {
		_ = os.WriteFile(cfgFile, bytes, 0644)
	}
	return cfg
}
