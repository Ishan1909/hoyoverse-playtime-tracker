import time
import logging
import threading
from datetime import datetime
from typing import Dict, Optional, Any, List
import psutil

from config import get_games, get_settings
from database import record_session, format_duration

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("PlaytimeTracker")


class ActiveSession:
    def __init__(self, game_key: str, game_name: str, pid: int, start_time: datetime):
        self.game_key = game_key
        self.game_name = game_name
        self.pid = pid
        self.start_time = start_time
        self.last_heartbeat = start_time

    @property
    def current_duration_seconds(self) -> int:
        return int((datetime.now() - self.start_time).total_seconds())

    @property
    def current_duration_formatted(self) -> str:
        return format_duration(self.current_duration_seconds)

    def to_dict(self) -> Dict[str, Any]:
        meta = get_games().get(self.game_key, {})
        return {
            "game_key": self.game_key,
            "game_name": self.game_name,
            "icon": meta.get("icon", ""),
            "color": meta.get("color", "#00d2be"),
            "pid": self.pid,
            "start_time": self.start_time.strftime("%Y-%m-%d %H:%M:%S"),
            "current_duration_seconds": self.current_duration_seconds,
            "current_duration_formatted": self.current_duration_formatted
        }


class PlaytimeTracker:
    def __init__(self):
        self.active_sessions: Dict[str, ActiveSession] = {}
        self.lock = threading.Lock()
        self._running = False
        self._thread: Optional[threading.Thread] = None

    def start(self):
        """Start the background monitoring thread."""
        if self._running:
            return
        self._running = True
        self._thread = threading.Thread(target=self._monitor_loop, daemon=True)
        self._thread.start()
        logger.info("Playtime monitor thread started.")

    def stop(self):
        """Stop the background monitoring thread and flush active sessions."""
        if not self._running:
            return
        self._running = False
        if self._thread:
            self._thread.join(timeout=3)
        self._flush_all_sessions()
        logger.info("Playtime monitor stopped.")

    def get_status(self) -> Dict[str, Any]:
        """Returns the current monitoring status and active sessions."""
        with self.lock:
            active_list = [session.to_dict() for session in self.active_sessions.values()]
            is_gaming = len(active_list) > 0
            primary_game = active_list[0] if is_gaming else None

            return {
                "tracker_running": self._running,
                "is_gaming": is_gaming,
                "active_count": len(active_list),
                "active_sessions": active_list,
                "primary_session": primary_game
            }

    def _flush_all_sessions(self):
        """Finalize and save all currently active sessions."""
        with self.lock:
            now = datetime.now()
            for game_key, session in list(self.active_sessions.items()):
                duration = int((now - session.start_time).total_seconds())
                logger.info(f"Finalizing session for {session.game_name} ({duration}s)")
                record_session(
                    game_key=session.game_key,
                    game_name=session.game_name,
                    start_time=session.start_time,
                    end_time=now,
                    duration_seconds=duration
                )
            self.active_sessions.clear()

    def _get_running_process_names(self) -> Dict[str, int]:
        """Scan running processes and return lowercased name -> pid mapping."""
        running = {}
        for p in psutil.process_iter(['name', 'pid']):
            try:
                name = p.info.get('name')
                if name:
                    running[name.lower()] = p.info.get('pid', 0)
            except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.ZombieProcess):
                continue
        return running

    def _monitor_loop(self):
        """Continuous polling loop."""
        while self._running:
            try:
                settings = get_settings()
                poll_interval = settings.get("poll_interval_seconds", 3)
                games_config = get_games()

                # Build lookup of exe_name.lower() -> (game_key, game_name)
                exe_to_game = {}
                for g_key, g_info in games_config.items():
                    for exe in g_info.get("executables", []):
                        exe_to_game[exe.lower()] = (g_key, g_info.get("name", g_key))

                # Scan running system processes
                running_procs = self._get_running_process_names()

                # Check which configured games are running right now
                detected_games = {}  # game_key -> (game_name, pid)
                for exe_name, (g_key, g_name) in exe_to_game.items():
                    if exe_name in running_procs:
                        detected_games[g_key] = (g_name, running_procs[exe_name])

                now = datetime.now()

                with self.lock:
                    # 1. Check for newly launched games
                    for g_key, (g_name, pid) in detected_games.items():
                        if g_key not in self.active_sessions:
                            logger.info(f"Detected game launch: {g_name} (PID: {pid})")
                            self.active_sessions[g_key] = ActiveSession(
                                game_key=g_key,
                                game_name=g_name,
                                pid=pid,
                                start_time=now
                            )
                        else:
                            # Update heartbeat
                            self.active_sessions[g_key].last_heartbeat = now

                    # 2. Check for games that were closed
                    closed_keys = [k for k in self.active_sessions if k not in detected_games]
                    for g_key in closed_keys:
                        session = self.active_sessions[g_key]
                        duration = int((now - session.start_time).total_seconds())
                        logger.info(f"Detected game close: {session.game_name}. Session duration: {format_duration(duration)}")
                        record_session(
                            game_key=session.game_key,
                            game_name=session.game_name,
                            start_time=session.start_time,
                            end_time=now,
                            duration_seconds=duration
                        )
                        del self.active_sessions[g_key]

            except Exception as e:
                logger.error(f"Error in monitor loop: {e}", exc_info=True)

            # Sleep in short increments to allow prompt shutdown
            for _ in range(poll_interval * 2):
                if not self._running:
                    break
                time.sleep(0.5)


tracker_instance = PlaytimeTracker()
