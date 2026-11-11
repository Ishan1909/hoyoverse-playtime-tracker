import webbrowser
from typing import Callable, Optional
import pystray
from PIL import Image, ImageDraw

from config import get_settings
from tracker import tracker_instance
from app_window import open_app_window


def create_tray_image(width=64, height=64) -> Image.Image:
    """Generate a clean 64x64 RGBA icon for the system tray."""
    image = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)

    # Draw rounded gradient/circle background
    draw.ellipse((4, 4, width - 4, height - 4), fill=(16, 22, 34, 255), outline=(0, 210, 190, 255), width=3)

    # Draw game controller-like shape or diamond inside
    # Diamond / Primogem inspired shape
    cx, cy = width // 2, height // 2
    points = [
        (cx, cy - 18),
        (cx + 14, cy),
        (cx, cy + 18),
        (cx - 14, cy)
    ]
    draw.polygon(points, fill=(0, 210, 190, 255))
    
    # Inner accent
    inner_points = [
        (cx, cy - 8),
        (cx + 6, cy),
        (cx, cy + 8),
        (cx - 6, cy)
    ]
    draw.polygon(inner_points, fill=(255, 255, 255, 230))

    return image


def open_dashboard(icon=None, item=None):
    """Open the local analytics dashboard in dedicated standalone app window."""
    settings = get_settings()
    url = settings.get("dashboard_url", "http://127.0.0.1:5000")
    open_app_window(url)


def show_status(icon: pystray.Icon, item=None):
    """Display a balloon notification with current gaming status."""
    status = tracker_instance.get_status()
    if status["is_gaming"] and status["primary_session"]:
        game = status["primary_session"]
        msg = f"Playing {game['game_name']}\nSession: {game['current_duration_formatted']}"
        icon.notify(msg, title="Active Game Detected")
    else:
        icon.notify("No active games running.\nWaiting for Genshin / Honkai...", title="Playtime Tracker Standby")


def create_tray_app(on_exit_callback: Optional[Callable] = None) -> pystray.Icon:
    """Create and return configured pystray Icon instance."""
    image = create_tray_image()

    def handle_exit(icon, item):
        if on_exit_callback:
            on_exit_callback()
        icon.stop()

    menu = pystray.Menu(
        pystray.MenuItem("Open Dashboard", open_dashboard, default=True),
        pystray.MenuItem("Check Status", show_status),
        pystray.Menu.SEPARATOR,
        pystray.MenuItem("Exit Tracker", handle_exit)
    )

    icon = pystray.Icon(
        name="HoyoversePlaytimeTracker",
        icon=image,
        title="Game Playtime Tracker",
        menu=menu
    )
    return icon
