# Contributing to Sniffile

Thanks for taking a look. This is a small tool and it would like to stay small.

## Ground rules

- **Almost zero dependencies.** Standard library plus the vendored
  `webview_go` fork that provides the native window. Everything else is CGO
  through GTK/WebKit already present on the system. If a feature seems to need
  another library, open an issue first and argue for it.
- **No build step for the UI.** `web/assets/` is plain HTML/CSS/JS, embedded
  with `go:embed`. Keep it that way.
- **Native first.** `sniffile` opens a window; `-server` is the fallback for
  headless machines. Do not turn the desktop app back into a browser launcher.
- **Keep it local.** No telemetry, no remote services, no analytics. The only
  allowed network peer is a local ollama instance.
- **Match the style.** Lowercase comments that explain *why*, not *what*.
  `gofmt` clean. Tests for ranking, tokenization and path safety.

## Development

```sh
make build     # bin/sniffile (native window, CGO)
make test
make vet
make fmt
make dist deb  # optional: packages
```

Requires Go 1.22+ and GTK 3 + WebKitGTK development headers:

- Arch: `gtk3 webkit2gtk-4.1`
- Debian/Ubuntu: `libgtk-3-dev libwebkit2gtk-4.1-dev`
- Fedora: `gtk3-devel webkit2gtk4.1-devel`
- Older distros with WebKitGTK 4.0: `make build-legacy`

## What is easy to help with

- More words in the Russian/English thesaurus (`internal/search/dict.go`).
- Better tokenization for CJK, German compounds, etc.
- A regression test for a search that returns the wrong file.
- Thumbnail edge cases (HEIC, huge images, EXIF orientation).
- Docs, translations, small UI fixes.

## Pull requests

1. Keep the diff focused; one topic per PR.
2. Add or update a test if you touch ranking, parsing or path handling.
3. Describe the user-visible change in the PR body and, if relevant, in
   `README.md`.

## Reporting bugs

Please include:

- OS and `sniffile version`
- what you typed and what you expected
- the relevant lines from the footer/terminal log
- whether ollama was on

Do not paste personal file names if you would rather not — replace them with
`example.jpg`.
