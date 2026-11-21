# How Sniffile is put together

```
 folder ──► index.Build ──► gzip+json ──► search.NewCorpus ──► BM25 ──► UI
              │                              ▲
              └── optional: ollama captions ─┘
```

## Packages

| package | job |
|---|---|
| `cmd/sniffile` | flags, banner, `scan` subcommand, native window, graceful shutdown |
| `internal/index` | parallel walk, tokenize, read headers/content, save/load |
| `internal/search` | inverted index, BM25, thesaurus, fuzzy matching |
| `internal/enrich` | optional HTTP client for a local ollama |
| `internal/server` | manager (one scan job), REST API, thumbnails, path checks |
| `web` | `go:embed` of the plain HTML/CSS/JS front end |
| `packaging/` | desktop entry, icons, Arch/RPM/Windows/deb metadata |
| `scripts/` | package builders for deb, rpm, apk, Arch and Windows |
| `debian/` | debhelper source packaging |

## Desktop window

The app is a native window, not a website. On Linux and Windows the binary is
built with CGO and links the vendored `webview_go` fork to GTK 3 + WebKitGTK
(or WebView2 on Windows). The HTTP server still exists, but only on
`127.0.0.1` with an OS-assigned port, and the window is pointed at it. That
keeps the UI plain HTML/CSS/JS while the user never sees a browser.

Two build variants cover the WebKitGTK split:

- default: `webkit2gtk-4.1` (Debian 12+, Ubuntu 23.04+, Fedora 38+, Arch)
- legacy: `-tags webkit2_40` for Debian 11 / Ubuntu 22.04.

`WEBKIT_DISABLE_DMABUF_RENDERER=1` is set automatically on Linux because
WebKitGTK 2.52 crashes in its DMA-BUF renderer on some Wayland + NVIDIA
setups. `SNIFFILE_DMABUF=1` opts out.

`-server` keeps the old browser path for headless machines. If the native
window cannot be created, the process logs the reason and falls back to the
browser instead of exiting silently.

## Packaging

| target | output |
|---|---|
| `make dist` | portable `.tar.gz` (binary, desktop file, icons, docs) |
| `make deb` | Debian binary package built with `ar` + `tar`, no debhelper needed |
| `make rpm` | RPM via `rpmbuild` or `nfpm` |
| `make apk` | Alpine package via `nfpm` |
| `make arch` | pacman package via `makepkg` |
| `make windows` | `.exe`, portable `.zip`, NSIS `setup.exe` |

`scripts/package-deb.sh` deliberately does not depend on `dpkg-deb`, so a
release can be assembled on Arch or Fedora. CI builds the native variants on
real Ubuntu 24.04 (4.1) and Ubuntu 22.04 (4.0), Windows through MSYS2, and Arch
inside an `archlinux` container.

## Index format

One file per root: `~/.config/sniffile/index/<sha256(root)[:16]>.json.gz`.

```json
{
  "version": 1,
  "root": "/home/you/Pictures",
  "built_at": "2026-11-14T18:01:12Z",
  "docs": [
    {
      "p": "/home/you/Pictures/pug.jpg",
      "n": "pug.jpg",
      "k": "image",
      "e": "jpg",
      "s": 482113,
      "m": 1762000000,
      "w": 1920, "h": 1080,
      "c": "A fawn pug sitting on a grey sofa",
      "t": ["pictures", "image", "landscape", "2026"],
      "nt": ["pug", "jpg"],
      "sn": ""
    }
  ]
}
```

Documents are sorted by path. The format is a cache: bump `index.Version` and
old files are ignored and rebuilt. Atomic writes (tmp + rename) mean a kill -9
during a scan cannot corrupt an existing index.

Token budget per document is capped at 72 words. Names get three votes, captions
2.5, everything else one. Text files contribute the first 192 KB of contents.

## Ranking

BM25 with `k1 = 1.4`, `b = 0.72`:

```
idf(t)   = ln(1 + (N - df + 0.5) / (df + 0.5))
score(d) = Σ idf(t) · (w · (k1 + 1)) / (w + k1 · (1 - b + b · len(d) / avg))
```

Then two human-sized boosts: `+5` when the whole query appears in the file name,
and coverage weighting `0.55 + 0.55 · matched_original_terms / all_terms`.
Candidates are sorted, filtered by kind, and capped at 60 rows.

Query processing:

1. `Normalize` — lowercase, `ё → е`, split camelCase and punctuation.
2. Stopwords removed, light stemming (Russian suffixes, English plurals).
3. Thesaurus adds up to a few English synonyms at weight 0.6.
4. Only if *nothing* matched: one-edit vocabulary pass at weight 0.5, then
   optionally one ollama keyword expansion pass at weight 0.45.

A kind filter (`image`, `code`, …) is applied before the final sort so a narrow
search doesn't waste slots.

## Optional ollama

- Detected once at startup (`GET /api/tags`, 400 ms timeout).
- Text model: chosen by a small preference list (llama3.2, qwen2.5, mistral…).
  Only used when a query returns zero results. Cached per query for 10 minutes.
- Vision model: llava/moondream/…, used only during `-describe` scans.
  Four consecutive failures stop the caption pass with a readable message;
  the filename index is kept.

Ollama being down is never an error the user has to care about.

## Concurrency

- One background job slot, guarded by a mutex. `POST /api/scan` returns `409`
  when one is already running.
- A scan walks directories on one goroutine and processes files on up to eight
  workers: metadata, 192 KB text heads and image headers are independent and
  dominate the wall clock. Documents are sorted by path at the end, so the
  output is deterministic.
- Saved indexes load in parallel at startup, and in-memory BM25 corpora are
  warmed in the background so the first search does not block typing.
- Corpora are immutable snapshots; a scan invalidates the snapshot at the end,
  so searches never see half-updated data.
- Thumbnails live in a size-bounded in-memory map (192 entries, arbitrary
  eviction). Rebuilds are cheap; no disk cache.

## Security model

- Listens on `127.0.0.1` by default and warns loudly otherwise.
- Every file, thumbnail, open and reveal request resolves symlinks and must stay
  inside a configured root (`Manager.Resolve`). `/api/file?path=/etc/passwd`
  is a 403.
- Writes reject cross-origin requests. CSP forbids inline scripts, framing and
  external origins.
- The server never follows symlinks while indexing, which also prevents loops.

There is no auth because there is no remote access by design. If you bind to a
LAN address, you are on your own — the warning says so.

## Deliberate trade-offs

- No embeddings. Fast to build, inspectable, and works on a 2012 laptop.
- No PDF parsing. A pure-Go PDF text extractor is a large dependency.
- Grayscale thumbnails via a 40-line area-average scaler. Slower and uglier than
  a good imaging library, but it keeps `go.mod` empty.
- One index per root instead of a global database. Deleting a scope is `rm`.
