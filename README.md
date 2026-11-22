# Sniffile

**Local file search that stays local.** A native desktop app — one small Go
binary, a black-and-white interface, no browser tab and no cloud in the middle.

Type what you remember — `photo with a pug`, `invoice march`,
`sunset 2024` — and get the file and the folder it lives in.

![sniffile](docs/screenshot.png)

## What it does

- Indexes folders you choose, or all of them at once.
- Searches file names, folder names, dates, extensions, image orientation and
  the text inside plain-text/source files.
- Understands Russian and English queries through a small hand-written
  thesaurus (мопс → pug, фотка → photo, машина → car, …).
- Survives typos with a one-edit fuzzy fallback.
- Shows grayscale thumbnails, auto-rotates phone photos from their EXIF
  orientation, opens files in your default app, reveals folders.
- Optionally uses a **local** [ollama](https://ollama.com) instance for query
  keywords and for one-line photo captions. Nothing leaves the machine.

One Go dependency (`webview_go`) for the native window. No npm, no JavaScript
build step, no Electron.

## Install

Pick your distribution. The desktop app needs GTK 3 and WebKitGTK; everything
else is inside the binary.

### Arch, Manjaro, EndeavourOS, CachyOS, Garuda

```sh
sudo pacman -S --needed base-devel go gtk3 webkit2gtk-4.1
git clone https://github.com/openastolfo-boop/sniffile
cd sniffile
make build && sudo make install PREFIX=/usr
```

Or build a proper pacman package: `make arch` (writes `dist/sniffile-*.pkg.tar.zst`).

### Debian 12+, Ubuntu 23.04+, Mint, Pop!_OS, Kali, Raspberry Pi OS

```sh
sudo apt install build-essential pkg-config golang-go libgtk-3-dev libwebkit2gtk-4.1-dev
git clone https://github.com/openastolfo-boop/sniffile
cd sniffile
make build && sudo make install PREFIX=/usr
```

`make deb` produces a `.deb` you can install with `sudo apt install ./dist/sniffile_*.deb`.

### Ubuntu 22.04, Debian 11 and older (WebKitGTK 4.0)

Install `libwebkit2gtk-4.0-dev` instead and build with the legacy ABI:

```sh
make build-legacy && sudo make install PREFIX=/usr
```

### Fedora, RHEL, Rocky, Alma, openSUSE

```sh
sudo dnf install gcc make pkgconf-pkg-config golang gtk3-devel webkit2gtk4.1-devel
git clone https://github.com/openastolfo-boop/sniffile
cd sniffile
make build && sudo make install PREFIX=/usr
```

`make rpm` builds an RPM (needs `rpmbuild` or `nfpm`).

### Alpine

```sh
sudo apk add build-base go pkgconf gtk+3.0-dev webkit2gtk-4.1-dev
make build && sudo make install PREFIX=/usr
```

`make apk` builds a native `.apk`, but it must run on Alpine: the binary has to
be linked against musl, not glibc.

### Any other Linux

Grab the portable tarball from the releases page, unpack it anywhere and run
`./sniffile`. It only needs GTK 3 and WebKitGTK 4.1 from the system. To add a
menu entry, copy `sniffile.desktop` to `~/.local/share/applications/`.

### Windows 10 and 11

Download `sniffile-*-windows-amd64-setup.exe` (or the portable `.zip`) from the
releases page. Microsoft Edge WebView2 is required; it ships with current
Windows 10/11 and the installer offers the official download if it is missing.

To build it yourself, use MSYS2:

```sh
pacman -S mingw-w64-x86_64-gcc mingw-w64-x86_64-nsis zip
./scripts/build-windows.sh
```

### macOS

`make build` works on macOS (Cocoa/WebKit is provided by the system). There is
no signed `.app` bundle yet.

## Use

```sh
sniffile                          # native desktop window
sniffile -server                  # same UI in a browser, if you prefer
sniffile scan ~/Pictures ~/Docs   # index from the terminal, no window
sniffile scan -describe ~/Pics    # caption photos with ollama (see below)
```

The window is the default. `-server` is only for headless machines and people
who explicitly want a browser tab.

In the UI: `+ folder` adds a scope, the × on a chip removes it, `all folders`
searches everything. `reindex` rebuilds the current scope. Dropdown filters
narrow by file kind.

### Flags

| flag | default | meaning |
|---|---|---|
| `-server` | `false` | use the browser instead of the native window |
| `-addr` | `127.0.0.1:8765` | listen address (the desktop picks a free port) |
| `-data` | `~/.config/sniffile` | config + index folder |
| `-ollama` | `auto` | `auto`, `off`, or a base URL |
| `-open` | `true` | open the browser on start (server mode only) |
| `-max-file-mb` | saved / 64 | skip files bigger than this |
| `-no-content` | `false` | names only, don't read text files |
| `-v` | `false` | log every request |

### Keyboard

`/` search · `j`/`k` or arrows move · `enter` open · `o` open · `r` reveal in
folder · `esc` close.

## How the search works

1. **Walk.** The folder is scanned once. Hidden folders, `.git`, `node_modules`
   and similar junk are skipped. For every file we keep the path, size, date,
   kind and a capped bag of words from names, paths, metadata and (for text
   files) contents.
2. **Rank.** BM25 over an in-memory inverted index. File-name words weigh 3,
   captions 2.5, the rest 1. A whole-query-in-name bonus and a coverage bonus
   keep the obvious answer on top.
3. **Expand.** A small bilingual thesaurus adds English synonyms, so Russian
   queries can hit files named in English. If a query finds literally nothing
   and ollama is running, a local text model is asked for a few English
   keywords and the search is repeated.
4. **Forgive.** Words with no matches at all get a one-edit-distance vocabulary
   pass (`pugg` → `pug`, `кошке` → `кошка`).

There are no embeddings, no vector database, no model download. That is a
deliberate trade: the index builds in seconds and stays inspectable. For
photo *content* search, see the next section.

## Optional: ollama

Sniffile detects a local ollama at `http://127.0.0.1:11434` and uses it only if
it exists.

**Text model** (llama3.2, qwen2.5, mistral, …): translates a dead-end query
into English keywords. Results are cached for ten minutes.

**Vision model** (llava, moondream, …): when you tick `describe photos` before
reindexing, every image gets a one-line caption stored in the index. Captions
are searchable, so `собака на диване` can find `IMG_4821.jpg`.

```sh
ollama pull llava
sniffile scan -describe ~/Pictures
```

No ollama? Everything except content-based photo search still works.

## Where the data lives

```
~/.config/sniffile/config.json      roots, ollama url, limits
~/.config/sniffile/index/*.json.gz  one gzip+json index per root
```

Override the folder with `-data` or `SNIFFILE_DATA`. Delete the `index/` folder
any time; it is a cache.

The only network calls are HTTP requests to your own `127.0.0.1:11434` when
ollama is enabled. There is no telemetry, no update check, no account.

## Honest limits

- Keyword matching, not semantic embeddings. A photo is findable by name or
  caption, not by what a neural net thinks it looks like — unless you add ollama.
- PDF, DOCX, EPUB contents are not parsed. Only plain text and source code.
- EXIF orientation is applied to thumbnails, but HEIC/AVIF/RAW previews are
  not decoded. Those files are still searchable by name.
- No OCR. Text inside screenshots is not indexed.
- Files over 64 MB are skipped by default.
- Chinese/Japanese text is tokenized as one long word. Works poorly. PRs welcome.
- One scan job at a time. This is a feature on spinning disks.
- On Wayland + NVIDIA, Sniffile disables WebKit's DMA-BUF renderer because
  WebKitGTK 2.52 crashes there. Set `SNIFFILE_DMABUF=1` if your driver works.

## Config file

`config.json` is meant to be readable and editable:

```json
{
  "version": 1,
  "roots": [
    { "path": "/home/you/Pictures", "added_at": "2026-11-14T18:00:00Z",
      "last_scan": "2026-11-14T18:01:12Z", "files": 4211 }
  ],
  "ollama_url": "http://127.0.0.1:11434",
  "max_file_mb": 64
}
```

## Development

```sh
make build     # bin/sniffile
make run       # go run ./cmd/sniffile
make test      # go test ./...
make vet
make fmt
```

Layout:

```
cmd/sniffile      entry point, CLI, native window
internal/index    walker, tokenizer, index format
internal/search   inverted index, BM25, synonyms, fuzzy
internal/enrich   optional ollama client
internal/server   HTTP API + print the manager
web               embedded HTML/CSS/JS
packaging/        desktop entry, icons, Arch/RPM/Windows/deb metadata
scripts/          package builders for deb, rpm, apk, Arch, Windows
debian/           debhelper source packaging
```

### Packaging

```sh
make dist      # portable tar.gz
make deb       # Debian/Ubuntu binary package (no debhelper needed)
make rpm       # Fedora/openSUSE package (rpmbuild or nfpm)
make apk       # Alpine package (nfpm)
make arch      # Arch package via makepkg
make windows   # Windows .exe, portable zip and NSIS installer
```

The release workflow builds Linux amd64 packages, a legacy WebKitGTK 4.0
tarball, an Arch package and Windows artifacts for every `v*` tag.

Pull requests are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md). Please keep
the dependency count at zero.

## License

MIT. See [LICENSE](LICENSE).

---

[Русская версия](README.ru.md)
