import os
from pathlib import Path
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse

from config import BASE_DIR, get_games, get_settings
from database import get_summary, get_daily_chart_data, get_recent_sessions
from tracker import tracker_instance

app = FastAPI(title="Game Playtime Tracker API")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

STATIC_DIR = BASE_DIR / "static"
ASSETS_DIR = BASE_DIR / "assets"

if not STATIC_DIR.exists():
    for cand in [Path(__file__).resolve().parent / "static", Path(sys.executable).parent / "static", Path(sys.executable).parent / "_internal" / "static"]:
        if cand.exists():
            STATIC_DIR = cand
            break

if not ASSETS_DIR.exists():
    for cand in [Path(__file__).resolve().parent / "assets", Path(sys.executable).parent / "assets", Path(sys.executable).parent / "_internal" / "assets"]:
        if cand.exists():
            ASSETS_DIR = cand
            break


@app.get("/api/status")
def get_tracker_status():
    """Return real-time tracker status and currently active games."""
    return tracker_instance.get_status()


@app.get("/api/summary")
def get_stats_summary():
    """Return aggregated playtime stats (today, week, all-time, per-game)."""
    return get_summary()


@app.get("/api/chart")
def get_chart_data(days: int = 14):
    """Return daily trend chart data for Chart.js."""
    return get_daily_chart_data(days=days)


@app.get("/api/sessions")
def get_sessions_history(limit: int = 50):
    """Return recent completed sessions."""
    return get_recent_sessions(limit=limit)


@app.get("/api/config")
def get_config_info():
    """Return tracked games and configuration details."""
    return {
        "games": get_games(),
        "settings": get_settings()
    }


# Mount static and assets directories
if STATIC_DIR.exists():
    app.mount("/static", StaticFiles(directory=str(STATIC_DIR)), name="static")

if ASSETS_DIR.exists():
    app.mount("/assets", StaticFiles(directory=str(ASSETS_DIR)), name="assets")


@app.get("/manifest.json")
def get_manifest():
    return FileResponse(STATIC_DIR / "manifest.json", media_type="application/manifest+json")


@app.get("/sw.js")
def get_service_worker():
    return FileResponse(STATIC_DIR / "sw.js", media_type="application/javascript")


@app.get("/")
def serve_index():
    index_file = STATIC_DIR / "index.html"
    if index_file.exists():
        return FileResponse(index_file)
    return {"message": "Playtime Tracker API running. Static dashboard not found."}
