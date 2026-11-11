import sys
import os
from pathlib import Path

# Safe persistent log location in AppData/Local/HoYoverseTracker/data
SAFE_DATA_DIR = Path(os.environ.get("LOCALAPPDATA", str(Path.home() / "AppData" / "Local"))) / "HoYoverseTracker" / "data"
SAFE_DATA_DIR.mkdir(parents=True, exist_ok=True)
LOG_FILE = SAFE_DATA_DIR / "tracker.log"

if sys.stdout is None:
    try:
        sys.stdout = open(LOG_FILE, "a", encoding="utf-8")
    except Exception:
        sys.stdout = open(os.devnull, "w", encoding="utf-8")

if sys.stderr is None:
    sys.stderr = sys.stdout

import time
import argparse
import threading
import logging
import ctypes
import uvicorn
import webbrowser

from config import get_settings
from database import init_db
from tracker import tracker_instance
from server import app
from tray import create_tray_app
from app_window import open_app_window

# Set up logging to both tracker.log and stream
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    handlers=[
        logging.FileHandler(LOG_FILE, encoding="utf-8"),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger("Main")

ERROR_ALREADY_EXISTS = 183


class WindowsSingleInstanceMutex:
    """Enforces only one running instance of the application across Windows."""
    def __init__(self, name="Local\\HoyoversePlaytimeTrackerMutex"):
        self.mutex = ctypes.windll.kernel32.CreateMutexW(None, False, name)
        self.already_running = (ctypes.windll.kernel32.GetLastError() == ERROR_ALREADY_EXISTS)

    def release(self):
        if self.mutex:
            ctypes.windll.kernel32.CloseHandle(self.mutex)
            self.mutex = None


class ServerThread(threading.Thread):
    def __init__(self, host: str, port: int):
        super().__init__(daemon=True)
        self.host = host
        self.port = port
        self.server = None

    def run(self):
        try:
            config = uvicorn.Config(
                app=app,
                host=self.host,
                port=self.port,
                log_level="warning",
                loop="asyncio"
            )
            self.server = uvicorn.Server(config)
            self.server.run()
        except Exception as e:
            logger.error(f"Uvicorn server crashed: {e}", exc_info=True)

    def stop(self):
        if self.server:
            self.server.should_exit = True


def main():
    parser = argparse.ArgumentParser(description="HoYoverse Game Playtime Tracker")
    parser.add_argument("--no-tray", action="store_true", help="Run without system tray icon (headless/console mode)")
    parser.add_argument("--open-dashboard", action="store_true", help="Automatically open dashboard in browser on start")
    parser.add_argument("--background", action="store_true", help="Start silently in background without opening dashboard window")
    args = parser.parse_args()

    # 0. Single-Instance Check
    settings = get_settings()
    host = settings.get("server_host", "127.0.0.1")
    port = settings.get("server_port", 5000)
    dashboard_url = settings.get("dashboard_url", f"http://{host}:{port}")

    mutex_lock = WindowsSingleInstanceMutex()
    if mutex_lock.already_running:
        logger.info("HoYoverse Playtime Tracker is already running in the background.")
        if args.open_dashboard:
            logger.info(f"Opening app window at {dashboard_url}...")
            open_app_window(dashboard_url)
        sys.exit(0)

    # 1. Initialize SQLite Database
    logger.info("Initializing database...")
    init_db()

    # 2. Start Background Playtime Tracker
    logger.info("Starting background playtime monitor...")
    tracker_instance.start()

    # 3. Start Local Web Dashboard API
    settings = get_settings()
    host = settings.get("server_host", "127.0.0.1")
    port = settings.get("server_port", 5000)
    dashboard_url = settings.get("dashboard_url", f"http://{host}:{port}")

    logger.info(f"Starting web dashboard on {dashboard_url}...")
    server_thread = ServerThread(host=host, port=port)
    server_thread.start()

    # Give server a brief moment to bind
    time.sleep(1)

    # Only open dashboard window if explicitly requested (e.g. from app shortcut)
    if args.open_dashboard:
        open_app_window(dashboard_url)

    is_shutting_down = False

    def shutdown():
        nonlocal is_shutting_down
        if is_shutting_down:
            return
        is_shutting_down = True
        logger.info("Shutting down tracker and server...")
        tracker_instance.stop()
        server_thread.stop()
        mutex_lock.release()
        logger.info("Clean shutdown complete.")

    # 4. System Tray or Background Loop
    if not args.no_tray:
        logger.info("Starting system tray icon. Check your Windows taskbar notification area.")
        try:
            tray_icon = create_tray_app(on_exit_callback=shutdown)
            tray_icon.run()
        except Exception as e:
            logger.warning(f"System tray could not start ({e}). Continuing in background mode.")

    # If system tray returned or was unavailable, keep tracker and web server running continuously
    if not is_shutting_down:
        logger.info(f"Running continuously in background. Web dashboard: {dashboard_url}")
        try:
            while not is_shutting_down:
                time.sleep(1)
        except KeyboardInterrupt:
            shutdown()


if __name__ == "__main__":
    main()
