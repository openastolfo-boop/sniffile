// Package web holds the embedded UI.
//
// There is no build step on purpose: edit the files under assets/, run
// `make build`, and the binary carries the new UI. No node_modules anywhere.
package web

import "embed"

//go:embed assets
var Assets embed.FS

// Index is served for "/" and cached in memory for the lifetime of the process.
var Index = func() []byte {
	b, err := Assets.ReadFile("assets/index.html")
	if err != nil {
		panic("web/assets is broken: " + err.Error())
	}
	return b
}()
