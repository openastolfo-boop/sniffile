//go:build linux && !webkit2_40

package webview

// Most current distributions (Debian 12+, Ubuntu 23.04+, Fedora 38+, Arch)
// ship WebKitGTK 4.1 only. Build with -tags webkit2_40 for older distros.

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
*/
import "C"
