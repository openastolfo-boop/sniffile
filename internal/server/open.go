package server

import (
	"os/exec"
	"runtime"
)

// openPath asks the desktop to open a file. Errors are returned but a missing
// xdg-open is common on headless machines, so callers just log them.
func openPath(p string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", p)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
