module github.com/openastolfo-boop/sniffile

go 1.22

require github.com/webview/webview_go v0.0.0-20240831120633-6173450d4dd6

// Local fork of webview_go: the upstream package hard-codes webkit2gtk-4.0,
// while modern distributions only ship 4.1. The fork selects 4.1 by default
// and 4.0 with `-tags webkit2_40`.
replace github.com/webview/webview_go => ./third_party/webview_go
