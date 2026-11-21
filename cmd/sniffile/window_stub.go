//go:build !cgo || (!linux && !windows && !darwin)

package main

import "errors"

// desktopAvailable is always false in builds without a native window backend.
func desktopAvailable() bool { return false }

func runWindow(string) error {
	return errors.New("this build has no desktop window (CGO disabled or unsupported OS) — run sniffile -server")
}

func terminateWindow() {}
