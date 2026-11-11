import sys
import json
import os
import shutil
from pathlib import Path

def _resolve_base_dir() -> Path:
    # 1. PyInstaller MEIPASS
    if hasattr(sys, '_MEIPASS'):
        p = Path(sys._MEIPASS)
        if (p / "assets").exists() or (p / "static").exists():
            return p
    # 2. Executable parent directory
    exe_dir = Path(sys.executable).parent
    if (exe_dir / "_internal" / "assets").exists():
        return exe_dir / "_internal"
    if (exe_dir / "assets").exists():
        return exe_dir
    # 3. Source directory
    src_dir = Path(__file__).resolve().parent
    return src_dir

BASE_DIR = _resolve_base_dir()

# Safe persistent user data location in AppData/Local/HoYoverseTracker/data
SAFE_DATA_DIR = Path(os.environ.get("LOCALAPPDATA", str(Path.home() / "AppData" / "Local"))) / "HoYoverseTracker" / "data"
SAFE_DATA_DIR.mkdir(parents=True, exist_ok=True)

DB_PATH = SAFE_DATA_DIR / "tracker.db"

# Automatically migrate legacy database if present
legacy_db = BASE_DIR / "tracker.db"
if legacy_db.exists() and not DB_PATH.exists():
    try:
        shutil.copy2(legacy_db, DB_PATH)
    except Exception:
        pass

CONFIG_FILE = SAFE_DATA_DIR / "games_config.json"
if not CONFIG_FILE.exists() and (BASE_DIR / "games_config.json").exists():
    try:
        shutil.copy2(BASE_DIR / "games_config.json", CONFIG_FILE)
    except Exception:
        pass

DEFAULT_GAMES = {
    "genshin": {
        "name": "Genshin Impact",
        "executables": ["GenshinImpact.exe", "YuanShen.exe"],
        "color": "#00d2be",
        "icon": "/assets/Genshin_Impact_logo.webp"
    },
    "star_rail": {
        "name": "Honkai: Star Rail",
        "executables": ["StarRail.exe"],
        "color": "#e0a354",
        "icon": "/assets/HSR_logo.png"
    },
    "honkai_3rd": {
        "name": "Honkai Impact 3rd",
        "executables": ["BH3.exe"],
        "color": "#9b51e0",
        "icon": "⚔️"
    }
}

DEFAULT_SETTINGS = {
    "poll_interval_seconds": 3,
    "server_host": "127.0.0.1",
    "server_port": 5000,
    "dashboard_url": "http://127.0.0.1:5000"
}


def load_config():
    """Load configuration from JSON if present, else create default."""
    if not CONFIG_FILE.exists():
        initial_data = {
            "settings": DEFAULT_SETTINGS,
            "games": DEFAULT_GAMES
        }
        with open(CONFIG_FILE, "w", encoding="utf-8") as f:
            json.dump(initial_data, f, indent=4)
        return initial_data

    try:
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return {"settings": DEFAULT_SETTINGS, "games": DEFAULT_GAMES}


def get_games():
    cfg = load_config()
    return cfg.get("games", DEFAULT_GAMES)


def get_settings():
    cfg = load_config()
    return cfg.get("settings", DEFAULT_SETTINGS)
