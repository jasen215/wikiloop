//go:build darwin

package tray

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
)

// Action represents a tray menu action.
type Action int

const (
	ActionOpenUI Action = iota
	ActionOpenKBDir
	ActionSettings
	ActionQuit
)

// Run starts the system tray icon and menu. Actions are sent to the action channel.
func Run(kbRoot string, port int, actionCh chan<- Action) {
	systray.Run(func() {
		systray.SetIcon(iconPNG)
		systray.SetTooltip("WikiLoop Knowledge Base")

		mOpenUI := systray.AddMenuItem("Open Dashboard", "Open Web UI in browser")
		mOpenKB := systray.AddMenuItem("Open KB Directory", "Open knowledge base folder")
		systray.AddSeparator()
		mSettings := systray.AddMenuItem("Settings", "Open settings page")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit WikiLoop")

		go func() {
			for {
				select {
				case <-mOpenUI.ClickedCh:
					url := fmt.Sprintf("http://localhost:%d", port)
					openBrowser(url)
					actionCh <- ActionOpenUI
				case <-mOpenKB.ClickedCh:
					openFileManager(kbRoot)
					actionCh <- ActionOpenKBDir
				case <-mSettings.ClickedCh:
					url := fmt.Sprintf("http://localhost:%d/settings", port)
					openBrowser(url)
					actionCh <- ActionSettings
				case <-mQuit.ClickedCh:
					systray.Quit()
					// Non-blocking send: receiver may not be ready.
					select {
					case actionCh <- ActionQuit:
					default:
					}
					return
				}
			}
		}()
	}, func() {
		// cleanup
	})
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	}
	if cmd != nil {
		if err := cmd.Start(); err != nil {
			log.Printf("tray: failed to open browser: %v", err)
		}
	}
}

func openFileManager(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	}
	if cmd != nil {
		if err := cmd.Start(); err != nil {
			log.Printf("tray: failed to open file manager: %v", err)
		}
	}
}
