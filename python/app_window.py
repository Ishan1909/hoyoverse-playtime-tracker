import os
import subprocess
import webbrowser
from pathlib import Path

def open_app_window(url: str = "http://127.0.0.1:5000"):
    """Opens the dashboard in a dedicated frameless native application window."""
    flags = getattr(subprocess, 'DETACHED_PROCESS', 0) | getattr(subprocess, 'CREATE_NEW_PROCESS_GROUP', 0)

    # 1. Try Google Chrome standalone app mode first (fastest and active if Chrome is default)
    for chrome in [
        os.path.join(os.environ.get("ProgramFiles", "C:\\Program Files"), "Google\\Chrome\\Application\\chrome.exe"),
        os.path.join(os.environ.get("ProgramFiles(x86)", "C:\\Program Files (x86)"), "Google\\Chrome\\Application\\chrome.exe"),
        os.path.join(os.environ.get("LOCALAPPDATA", str(Path.home() / "AppData" / "Local")), "Google\\Chrome\\Application\\chrome.exe")
    ]:
        if os.path.exists(chrome):
            try:
                subprocess.Popen([chrome, f"--app={url}", "--new-window"], close_fds=True, creationflags=flags)
                return True
            except Exception:
                pass

    # 2. Try Microsoft Edge standalone app mode
    for edge in [
        os.path.join(os.environ.get("ProgramFiles(x86)", "C:\\Program Files (x86)"), "Microsoft\\Edge\\Application\\msedge.exe"),
        os.path.join(os.environ.get("ProgramFiles", "C:\\Program Files"), "Microsoft\\Edge\\Application\\msedge.exe"),
    ]:
        if os.path.exists(edge):
            try:
                subprocess.Popen([edge, f"--app={url}", "--new-window"], close_fds=True, creationflags=flags)
                return True
            except Exception:
                pass

    # 3. Fallback to default browser
    try:
        webbrowser.open(url)
        return True
    except Exception:
        return False
