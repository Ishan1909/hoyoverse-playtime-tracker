package tray

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"hoyoverse-tracker/internal/monitor"
	"hoyoverse-tracker/internal/window"
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procLoadImageW          = user32.NewProc("LoadImageW")

	procShell_NotifyIconW   = shell32.NewProc("Shell_NotifyIconW")
)

const (
	WM_USER         = 0x0400
	WM_TRAYICON     = WM_USER + 1
	WM_COMMAND      = 0x0111
	WM_DESTROY      = 0x0002
	WM_LBUTTONUP    = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP    = 0x0205

	NIM_ADD    = 0x00000000
	NIM_MODIFY = 0x00000001
	NIM_DELETE = 0x00000002

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004
	NIF_INFO    = 0x00000010

	MF_STRING    = 0x00000000
	MF_SEPARATOR = 0x00000800

	TPM_RIGHTBUTTON = 0x0002

	ID_OPEN_DASHBOARD = 1001
	ID_STATUS         = 1002
	ID_EXIT           = 1003
)

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             windows.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            windows.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeoutOrVersion uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     windows.Handle
}

type POINT struct {
	X int32
	Y int32
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type MSG struct {
	HWnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type TrayApp struct {
	hwnd       windows.Handle
	nid        NOTIFYICONDATAW
	dashboardURL string
	onExit     func()
}

var currentTray *TrayApp

func windowProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAYICON:
		switch lParam {
		case WM_LBUTTONDBLCLK, WM_LBUTTONUP:
			if currentTray != nil {
				window.OpenAppWindow(currentTray.dashboardURL)
			}
		case WM_RBUTTONUP:
			if currentTray != nil {
				currentTray.showMenu()
			}
		}
		return 0

	case WM_COMMAND:
		id := uint32(wParam & 0xFFFF)
		switch id {
		case ID_OPEN_DASHBOARD:
			if currentTray != nil {
				window.OpenAppWindow(currentTray.dashboardURL)
			}
		case ID_STATUS:
			if currentTray != nil {
				currentTray.showStatusNotification()
			}
		case ID_EXIT:
			if currentTray != nil {
				currentTray.Stop()
			}
		}
		return 0

	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func NewTrayApp(dashboardURL string, onExit func()) *TrayApp {
	return &TrayApp{
		dashboardURL: dashboardURL,
		onExit:       onExit,
	}
}

func loadAppIcon() windows.Handle {
	hInst, _, _ := procGetModuleHandleW.Call(0)

	// 1. Try loading embedded resource icon (ID 1)
	hIcon, _, _ := procLoadImageW.Call(hInst, uintptr(1), 1 /* IMAGE_ICON */, 16, 16, 0)
	if hIcon != 0 {
		return windows.Handle(hIcon)
	}
	hIcon, _, _ = procLoadIconW.Call(hInst, uintptr(1))
	if hIcon != 0 {
		return windows.Handle(hIcon)
	}

	// 2. Try loading from app_icon.ico file on disk
	exePath, err := os.Executable()
	if err == nil {
		candidates := []string{
			filepath.Join(filepath.Dir(exePath), "app_icon.ico"),
			filepath.Join(filepath.Dir(exePath), "static", "app_icon.ico"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "HoYoversePlaytimeTracker", "app_icon.ico"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "HoYoverseTracker", "data", "app_icon.ico"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				cPtr, _ := windows.UTF16PtrFromString(c)
				hIcon, _, _ = procLoadImageW.Call(0, uintptr(unsafe.Pointer(cPtr)), 1, 16, 16, 0x00000010 /* LR_LOADFROMFILE */)
				if hIcon != 0 {
					return windows.Handle(hIcon)
				}
			}
		}
	}

	// 3. Fallback to standard system icon
	hIcon, _, _ = procLoadIconW.Call(0, uintptr(32512)) // IDI_APPLICATION
	return windows.Handle(hIcon)
}

func (t *TrayApp) Run() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	currentTray = t

	className, _ := windows.UTF16PtrFromString("HoYoverseTrackerTrayClass")
	windowName, _ := windows.UTF16PtrFromString("HoYoverseTrackerTray")

	// Custom application controller icon
	hIcon := loadAppIcon()

	wndClass := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		LpfnWndProc:   syscall.NewCallback(windowProc),
		HIcon:         hIcon,
		HIconSm:       hIcon,
		LpszClassName: className,
	}

	ret, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wndClass)))
	if ret == 0 {
		// Could already be registered
	}

	hwndRet, _, errCreate := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	)
	if hwndRet == 0 {
		return fmt.Errorf("CreateWindowEx failed: %v", errCreate)
	}

	t.hwnd = windows.Handle(hwndRet)

	t.nid = NOTIFYICONDATAW{
		CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATAW{})),
		HWnd:             t.hwnd,
		UID:              1,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		UCallbackMessage: WM_TRAYICON,
		HIcon:            hIcon,
	}

	tip, _ := windows.UTF16FromString("HoYoverse Playtime Tracker")
	copy(t.nid.SzTip[:], tip)

	procShell_NotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&t.nid)))

	var msg MSG
	for {
		mret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if mret == 0 || int32(mret) == -1 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	return nil
}

func (t *TrayApp) showMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	titleText, _ := windows.UTF16PtrFromString("HoYoverse Tracker")
	procAppendMenuW.Call(hMenu, MF_STRING, 0, uintptr(unsafe.Pointer(titleText)))
	procAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)

	openText, _ := windows.UTF16PtrFromString("Open Dashboard")
	procAppendMenuW.Call(hMenu, MF_STRING, uintptr(ID_OPEN_DASHBOARD), uintptr(unsafe.Pointer(openText)))

	statusText, _ := windows.UTF16PtrFromString("Check Game Status")
	procAppendMenuW.Call(hMenu, MF_STRING, uintptr(ID_STATUS), uintptr(unsafe.Pointer(statusText)))

	procAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)

	exitText, _ := windows.UTF16PtrFromString("Exit")
	procAppendMenuW.Call(hMenu, MF_STRING, uintptr(ID_EXIT), uintptr(unsafe.Pointer(exitText)))

	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(uintptr(t.hwnd))

	procTrackPopupMenu.Call(
		hMenu,
		TPM_RIGHTBUTTON,
		uintptr(pt.X),
		uintptr(pt.Y),
		0,
		uintptr(t.hwnd),
		0,
	)
}

func (t *TrayApp) showStatusNotification() {
	status := monitor.MonitorInstance.GetStatus()
	title := "HoYoverse Tracker: Standby"
	msg := "Waiting for Genshin Impact or Honkai: Star Rail..."

	if status.IsGaming && status.PrimarySession != nil {
		title = fmt.Sprintf("Gaming Active: %s", status.PrimarySession.GameName)
		msg = fmt.Sprintf("Current Session: %s", status.PrimarySession.CurrentDurationFormatted)
	}

	t.nid.UFlags |= NIF_INFO
	copy(t.nid.SzInfoTitle[:], windows.StringToUTF16(title))
	copy(t.nid.SzInfo[:], windows.StringToUTF16(msg))

	procShell_NotifyIconW.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&t.nid)))
}

func (t *TrayApp) Stop() {
	procShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&t.nid)))
	if t.hwnd != 0 {
		procDestroyWindow.Call(uintptr(t.hwnd))
	}
	if t.onExit != nil {
		t.onExit()
	}
}
