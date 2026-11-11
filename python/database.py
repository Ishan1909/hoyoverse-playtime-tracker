import sqlite3
from datetime import datetime, timedelta
from typing import Dict, List, Any, Optional
from config import DB_PATH, get_games


def get_connection():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def init_db():
    """Create sessions table and indexes if they don't exist."""
    with get_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS sessions (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                game_key TEXT NOT NULL,
                game_name TEXT NOT NULL,
                start_time TEXT NOT NULL,
                end_time TEXT NOT NULL,
                duration_seconds INTEGER NOT NULL,
                date TEXT NOT NULL
            )
        """)
        cursor.execute("CREATE INDEX IF NOT EXISTS idx_sessions_game_key ON sessions(game_key)")
        cursor.execute("CREATE INDEX IF NOT EXISTS idx_sessions_date ON sessions(date)")
        conn.commit()


def record_session(game_key: str, game_name: str, start_time: datetime, end_time: datetime, duration_seconds: int) -> int:
    """Record a completed game session into the database with duplicate & overlap protection."""
    if duration_seconds < 5:
        # Ignore accidental launches under 5 seconds
        return -1

    date_str = start_time.strftime("%Y-%m-%d")
    start_str = start_time.strftime("%Y-%m-%d %H:%M:%S")
    end_str = end_time.strftime("%Y-%m-%d %H:%M:%S")

    with get_connection() as conn:
        cursor = conn.cursor()

        # Check for duplicates or overlapping sessions for this game
        cursor.execute("""
            SELECT id, start_time, end_time, duration_seconds 
            FROM sessions 
            WHERE game_key = ? 
            ORDER BY id DESC LIMIT 5
        """, (game_key,))
        recent_rows = cursor.fetchall()
        for r in recent_rows:
            try:
                r_start = datetime.strptime(r["start_time"], "%Y-%m-%d %H:%M:%S")
                r_end = datetime.strptime(r["end_time"], "%Y-%m-%d %H:%M:%S")

                start_diff = abs((start_time - r_start).total_seconds())
                end_diff = abs((end_time - r_end).total_seconds())

                # If timestamps match within 60s, it's a duplicate from concurrent process or slight clock drift
                if start_diff <= 60 and end_diff <= 60:
                    return r["id"]

                # If sessions overlap, merge into the encompassing timeframe
                if start_time < r_end and end_time > r_start:
                    merged_start = min(start_time, r_start)
                    merged_end = max(end_time, r_end)
                    merged_duration = int((merged_end - merged_start).total_seconds())
                    cursor.execute("""
                        UPDATE sessions 
                        SET start_time = ?, end_time = ?, duration_seconds = ?
                        WHERE id = ?
                    """, (
                        merged_start.strftime("%Y-%m-%d %H:%M:%S"),
                        merged_end.strftime("%Y-%m-%d %H:%M:%S"),
                        merged_duration,
                        r["id"]
                    ))
                    conn.commit()
                    return r["id"]
            except Exception:
                continue

        cursor.execute("""
            INSERT INTO sessions (game_key, game_name, start_time, end_time, duration_seconds, date)
            VALUES (?, ?, ?, ?, ?, ?)
        """, (game_key, game_name, start_str, end_str, duration_seconds, date_str))
        conn.commit()
        return cursor.lastrowid


def format_duration(seconds: int) -> str:
    """Format seconds into readable human string: e.g. 2h 15m or 45m or 30s."""
    if seconds < 60:
        return f"{seconds}s"
    hours = seconds // 3600
    minutes = (seconds % 3600) // 60
    if hours > 0:
        return f"{hours}h {minutes:02d}m"
    return f"{minutes}m"


def get_summary() -> Dict[str, Any]:
    """Retrieve aggregate statistics: total, 7-day, today, per-game breakdowns."""
    today_str = datetime.now().strftime("%Y-%m-%d")
    seven_days_ago = (datetime.now() - timedelta(days=7)).strftime("%Y-%m-%d")

    configured_games = get_games()

    with get_connection() as conn:
        cursor = conn.cursor()

        # Overall totals
        cursor.execute("SELECT COUNT(*), COALESCE(SUM(duration_seconds), 0), COALESCE(MAX(duration_seconds), 0) FROM sessions")
        total_sessions, total_seconds, longest_session_seconds = cursor.fetchone()

        # Today's total
        cursor.execute("SELECT COALESCE(SUM(duration_seconds), 0) FROM sessions WHERE date = ?", (today_str,))
        today_seconds = cursor.fetchone()[0]

        # Last 7 days total
        cursor.execute("SELECT COALESCE(SUM(duration_seconds), 0) FROM sessions WHERE date >= ?", (seven_days_ago,))
        week_seconds = cursor.fetchone()[0]

        # Per game stats
        cursor.execute("""
            SELECT 
                game_key,
                game_name,
                COUNT(*) as session_count,
                SUM(duration_seconds) as total_seconds,
                MAX(duration_seconds) as longest_seconds
            FROM sessions
            GROUP BY game_key
        """)
        game_rows = cursor.fetchall()
        
        per_game = {}
        for row in game_rows:
            key = row["game_key"]
            meta = configured_games.get(key, {})
            per_game[key] = {
                "game_key": key,
                "game_name": row["game_name"],
                "color": meta.get("color", "#4a90e2"),
                "icon": meta.get("icon", "🎮"),
                "session_count": row["session_count"],
                "total_seconds": row["total_seconds"],
                "total_formatted": format_duration(row["total_seconds"]),
                "longest_formatted": format_duration(row["longest_seconds"])
            }

        # Include configured games with 0 playtime so they show up
        for key, meta in configured_games.items():
            if key not in per_game:
                per_game[key] = {
                    "game_key": key,
                    "game_name": meta.get("name", key),
                    "color": meta.get("color", "#4a90e2"),
                    "icon": meta.get("icon", "🎮"),
                    "session_count": 0,
                    "total_seconds": 0,
                    "total_formatted": "0m",
                    "longest_formatted": "0m"
                }

        # Determine most played game
        most_played = None
        if game_rows:
            top_game = max(game_rows, key=lambda r: r["total_seconds"])
            most_played = {
                "game_key": top_game["game_key"],
                "game_name": top_game["game_name"],
                "formatted": format_duration(top_game["total_seconds"])
            }

        return {
            "total_seconds": total_seconds,
            "total_formatted": format_duration(total_seconds),
            "today_seconds": today_seconds,
            "today_formatted": format_duration(today_seconds),
            "week_seconds": week_seconds,
            "week_formatted": format_duration(week_seconds),
            "total_sessions": total_sessions,
            "longest_session_formatted": format_duration(longest_session_seconds),
            "most_played": most_played,
            "per_game": per_game
        }


def get_daily_chart_data(days: int = 14) -> Dict[str, Any]:
    """Retrieve daily playtime broken down by game for the past `days` days."""
    end_date = datetime.now().date()
    start_date = end_date - timedelta(days=days - 1)
    
    date_list = [(start_date + timedelta(days=i)).strftime("%Y-%m-%d") for i in range(days)]
    date_labels = [(start_date + timedelta(days=i)).strftime("%b %d") for i in range(days)]
    
    configured_games = get_games()

    with get_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            SELECT date, game_key, SUM(duration_seconds) as day_seconds
            FROM sessions
            WHERE date >= ?
            GROUP BY date, game_key
        """, (start_date.strftime("%Y-%m-%d"),))
        rows = cursor.fetchall()

    # Map (date, game_key) -> hours (float)
    data_lookup = {}
    for r in rows:
        # rounded to 2 decimal places for hours
        hours = round(r["day_seconds"] / 3600.0, 2)
        data_lookup[(r["date"], r["game_key"])] = hours

    datasets = []
    for g_key, g_meta in configured_games.items():
        daily_hours = [data_lookup.get((d, g_key), 0.0) for d in date_list]
        datasets.append({
            "label": g_meta.get("name", g_key),
            "game_key": g_key,
            "color": g_meta.get("color", "#4a90e2"),
            "data": daily_hours
        })

    return {
        "labels": date_labels,
        "raw_dates": date_list,
        "datasets": datasets
    }


def get_recent_sessions(limit: int = 25) -> List[Dict[str, Any]]:
    """Retrieve recent game sessions ordered by start_time descending."""
    configured_games = get_games()
    with get_connection() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            SELECT id, game_key, game_name, start_time, end_time, duration_seconds, date
            FROM sessions
            ORDER BY id DESC
            LIMIT ?
        """, (limit,))
        rows = cursor.fetchall()

    sessions = []
    for r in rows:
        key = r["game_key"]
        meta = configured_games.get(key, {})
        sessions.append({
            "id": r["id"],
            "game_key": key,
            "game_name": r["game_name"],
            "color": meta.get("color", "#4a90e2"),
            "icon": meta.get("icon", "🎮"),
            "start_time": r["start_time"],
            "end_time": r["end_time"],
            "duration_seconds": r["duration_seconds"],
            "duration_formatted": format_duration(r["duration_seconds"]),
            "date": r["date"]
        })
    return sessions
