package server

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// openPath asks the desktop to open a file. When reveal is true it selects the
// file in the system file manager instead of launching it. Errors are returned
// but a missing xdg-open is common on headless machines, so callers just log
// them.
func openPath(p string, reveal bool) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if reveal {
			cmd = exec.Command("open", "-R", p)
		} else {
			cmd = exec.Command("open", p)
		}
	case "windows":
		if reveal {
			// /select, needs the file path, not its parent folder.
			cmd = exec.Command("explorer.exe", "/select,"+p)
		} else {
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", p)
		}
	default:
		if reveal {
			p = filepath.Dir(p)
		}
		cmd = exec.Command("xdg-open", p)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
