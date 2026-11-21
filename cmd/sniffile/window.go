//go:build cgo && (linux || windows || darwin)

package main

import (
	"errors"
	"os"
	"runtime"
	"sync"

	webview "github.com/webview/webview_go"
)

var (
	winMu sync.Mutex
	win   webview.WebView
)

// desktopAvailable reports whether a native window can be opened here.
func desktopAvailable() bool {
	if runtime.GOOS == "linux" {
		return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	}
	return true
}

// prepareDesktopEnv works around a WebKitGTK + NVIDIA/Wayland crash: the
// DMA-BUF renderer fails to allocate buffers and the whole window dies with a
// Wayland protocol error. The shared-memory renderer is slower on paper but
// always works; users can opt back in with SNIFFILE_DMABUF=1.
func prepareDesktopEnv() {
	if runtime.GOOS != "linux" {
		return
	}
	if os.Getenv("SNIFFILE_DMABUF") != "1" && os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
}

// runWindow opens the native window and blocks until the user closes it.
func runWindow(url string) error {
	prepareDesktopEnv()
	w := webview.New(false)
	if w == nil {
		return errors.New("cannot create a native window")
	}
	winMu.Lock()
	win = w
	winMu.Unlock()
	defer func() {
		winMu.Lock()
		win = nil
		w.Destroy() // under the lock, so terminateWindow cannot race it
		winMu.Unlock()
	}()

	w.SetTitle("Sniffile — local file search")
	w.SetSize(1180, 820, webview.HintNone)
	w.SetSize(720, 520, webview.HintMin)
	w.Navigate(url)
	w.Run()
	return nil
}

// terminateWindow closes the window from another goroutine (ctrl-c).
// It is safe to call when no window is open.
func terminateWindow() {
	winMu.Lock()
	w := win
	winMu.Unlock()
	if w != nil {
		w.Terminate()
	}
}
