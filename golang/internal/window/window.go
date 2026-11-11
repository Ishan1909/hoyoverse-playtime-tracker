package window

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func OpenAppWindow(url string) bool {
	if url == "" {
		url = "http://127.0.0.1:5000"
	}

	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")
	localAppData := os.Getenv("LOCALAPPDATA")

	// 1. Try Google Chrome in standalone app mode
	chromeCandidates := []string{
		filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
	}

	for _, cand := range chromeCandidates {
		if _, err := os.Stat(cand); err == nil {
			cmd := exec.Command(cand, "--app="+url, "--new-window")
			cmd.SysProcAttr = &syscall.SysProcAttr{
				CreationFlags: 0x08000000 | 0x00000200, // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
			}
			if err := cmd.Start(); err == nil {
				return true
			}
		}
	}

	// 2. Try Microsoft Edge in standalone app mode
	edgeCandidates := []string{
		filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
	}

	for _, cand := range edgeCandidates {
		if _, err := os.Stat(cand); err == nil {
			cmd := exec.Command(cand, "--app="+url, "--new-window")
			cmd.SysProcAttr = &syscall.SysProcAttr{
				CreationFlags: 0x08000000 | 0x00000200, // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
			}
			if err := cmd.Start(); err == nil {
				return true
			}
		}
	}

	// 3. Fallback: default Windows browser
	cmd := exec.Command("cmd", "/c", "start", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000,
	}
	_ = cmd.Start()
	return true
}
