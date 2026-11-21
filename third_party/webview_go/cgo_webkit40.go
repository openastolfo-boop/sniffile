//go:build linux && webkit2_40

package webview

// Legacy WebKitGTK 4.0 (Debian 11, Ubuntu 22.04 and older).

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.0
*/
import "C"
