# HoYoverse Playtime Tracker 🎮

An ultra-lightweight, automated background playtime tracker and glassmorphic dashboard for **Genshin Impact**, **Honkai: Star Rail**, and **Honkai Impact 3rd**.

[![Download Latest Release](https://img.shields.io/badge/Download-Setup--HoYoverseTracker.exe-00d2be?style=for-the-badge&logo=windows&logoColor=white)](https://github.com/Ishan1909/hoyoverse-playtime-tracker/releases/download/v1.0.0/Setup-HoYoverseTracker.exe)
[![Release](https://img.shields.io/badge/Release-v1.0.0-e0a354?style=for-the-badge)](https://github.com/Ishan1909/hoyoverse-playtime-tracker/releases/tag/v1.0.0)

---

## ⚡ Performance Highlights

| Metric | Go Native Architecture (`golang/`) | Python Prototype (`python/`) |
| :--- | :--- | :--- |
| **Idle Memory (RAM)** | **0.80 MB** ⚡ | ~50.2 MB |
| **Startup Memory** | **7.5 MB** | ~65 MB |
| **Single-File Installer** | **9.51 MB** | 35.8 MB |
| **Standalone Executable** | **7.36 MB** | 75 MB (unpacked) |
| **Monitoring Overhead** | Pure Win32 Snapshot (**0.0% CPU**) | psutil loop |
| **Database Driver** | Native `C:\Windows\System32\winsqlite3.dll` | Python `sqlite3` driver |

---

## 🚀 Features

- **Automated Process Detection**: Win32 `Toolhelp32` process snapshot polling actively tracks when game executables launch and close.
- **Silent Background Operation**: Runs headless on Windows login via `--background` without opening windows or consuming resources.
- **Micro-Memory Footprint**: Automated working set trimming keeps idle RAM usage under **1 Megabyte** (0.7–0.8 MB).
- **Interactive Dashboard**: Glassmorphic UI with 14-day playtime trend charts, per-game playtime share doughnut, and recent session logs.
- **Native Windows Tray**: Right-click menu with instant status check, dashboard launch, and clean exit.
- **Zero Data Loss**: Session history is safely persisted in an independent SQLite database at `%LOCALAPPDATA%\HoYoverseTracker\data\tracker.db`.

---

## 📁 Repository Structure

```
.
├── golang/            # Production native Go implementation (< 1 MB RAM)
│   ├── cmd/
│   │   ├── tracker/   # Native Windows GUI application & tray
│   │   └── installer/ # Self-contained single-file installer
│   ├── internal/      # Config, SQLite engine, process monitor, embedded HTTP server
│   ├── ui/            # Embedded Tailwind dashboard, Chart.js, official logos
│   └── build.ps1      # Automated compilation script
│
├── python/            # Original Python prototype
│   ├── tracker.py     # psutil monitoring loop
│   ├── server.py      # FastAPI REST server
│   ├── tray.py        # pystray system tray integration
│   ├── static/        # Web dashboard assets
│   └── requirements.txt
│
└── README.md
```

---

## 📦 Download & Installation

The compiled single-file installer is self-contained and requires no prerequisites:

- 📥 **Direct Download**: **[Setup-HoYoverseTracker.exe (v1.0.0)](https://github.com/Ishan1909/hoyoverse-playtime-tracker/releases/download/v1.0.0/Setup-HoYoverseTracker.exe)**
- 🏷️ **GitHub Release Page**: **[View Release v1.0.0](https://github.com/Ishan1909/hoyoverse-playtime-tracker/releases/tag/v1.0.0)**

Run the setup wizard to automatically configure the background monitor, start menu entry, and desktop shortcuts.

---

## 🛠️ Building from Source

### Go Native Tracker
```powershell
cd golang
go mod download
.\build.ps1
```

### Python Prototype
```powershell
cd python
pip install -r requirements.txt
python main.py
```
