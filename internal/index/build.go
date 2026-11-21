package index

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
)

// Version is bumped whenever the on-disk shape changes. Old indexes are simply
// rebuilt; there is nothing precious inside.
const Version = 2

// Doc is one file. Tokens hold path/content/meta words, NameTokens are kept
// separately because the search engine gives file names a stronger vote.
type Doc struct {
	Path       string   `json:"p"`
	Name       string   `json:"n"`
	Kind       string   `json:"k"`
	Ext        string   `json:"e,omitempty"`
	Size       int64    `json:"s"`
	Mod        int64    `json:"m"`
	Width      int      `json:"w,omitempty"`
	Height     int      `json:"h,omitempty"`
	Caption    string   `json:"c,omitempty"`
	Tokens     []string `json:"t,omitempty"`
	NameTokens []string `json:"nt,omitempty"`
	Snippet    string   `json:"sn,omitempty"`
}

// Index is one root folder.
type Index struct {
	Version int       `json:"version"`
	Root    string    `json:"root"`
	BuiltAt time.Time `json:"built_at"`
	Docs    []Doc     `json:"docs"`
}

// Progress is what the UI shows while a scan is running.
type Progress struct {
	Files int
	Dirs  int
	Path  string
}

// Options are the knobs that are not worth a config file.
type Options struct {
	MaxFileMB  int64
	MaxTokens  int
	Content    bool
	SkipHidden bool
	Prev       *Index
	OnProgress func(Progress)
	// OnDoc receives every finished document from a worker goroutine. The
	// manager uses it to publish a partial index while a large scan is still
	// running, so search works from the first second instead of after the
	// whole disk has been walked.
	OnDoc func(Doc)
}

// Noise that only produces ten thousand useless hits.
var skipDirs = map[string]struct{}{
	"node_modules": {}, "__pycache__": {}, ".venv": {}, "venv": {},
	".git": {}, ".hg": {}, ".svn": {}, ".cache": {}, ".npm": {},
	".cargo": {}, ".rustup": {}, ".gradle": {}, ".idea": {}, ".vscode": {},
	"$recycle.bin": {}, "system volume information": {}, ".trash": {},
}

// Build walks root and returns a fresh index. If Prev is set, image captions
// from the previous index are carried over as long as the file did not change.
func Build(ctx context.Context, root string, opt Options) (*Index, error) {
	root = filepath.Clean(root)
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, errors.New("not a directory: " + root)
	}
	if opt.MaxFileMB <= 0 {
		opt.MaxFileMB = 64
	}
	if opt.MaxTokens <= 0 {
		opt.MaxTokens = 512
	}
	maxBytes := opt.MaxFileMB << 20

	prev := map[string]Doc{}
	if opt.Prev != nil {
		for _, d := range opt.Prev.Docs {
			prev[d.Path] = d
		}
	}

	idx := &Index{Version: Version, Root: root, BuiltAt: time.Now().UTC()}
	var files, dirs int64
	lastReport := time.Now().Add(-time.Second).UnixNano()
	var reportMu sync.Mutex
	report := func(p string) {
		if opt.OnProgress == nil {
			return
		}
		now := time.Now().UnixNano()
		if now-atomic.LoadInt64(&lastReport) < int64(120*time.Millisecond) {
			return
		}
		if !atomic.CompareAndSwapInt64(&lastReport, atomic.LoadInt64(&lastReport), now) {
			return
		}
		reportMu.Lock()
		opt.OnProgress(Progress{
			Files: int(atomic.LoadInt64(&files)),
			Dirs:  int(atomic.LoadInt64(&dirs)),
			Path:  p,
		})
		reportMu.Unlock()
	}

	// File metadata, text heads and image headers are the expensive part of a
	// scan, and they are independent. A small worker pool keeps NVMe and SSD
	// scans CPU-bound instead of syscall-bound without hammering spinning
	// disks too hard (eight readers at most).
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}
	type task struct {
		path string
		name string
		info fs.FileInfo
	}
	tasks := make(chan task, workers*4)
	var docsMu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				if ctx.Err() != nil {
					continue // drain so WalkDir never blocks
				}
				doc := makeDoc(root, t.path, t.name, t.info, opt, prev)
				docsMu.Lock()
				idx.Docs = append(idx.Docs, doc)
				docsMu.Unlock()
				if opt.OnDoc != nil {
					opt.OnDoc(doc)
				}
				atomic.AddInt64(&files, 1)
				report(t.path)
			}
		}()
	}

	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // unreadable corner of the disk, keep going
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		name := d.Name()
		if d.IsDir() {
			if p != root {
				if opt.SkipHidden && strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				if _, skip := skipDirs[strings.ToLower(name)]; skip {
					return filepath.SkipDir
				}
			}
			atomic.AddInt64(&dirs, 1)
			report(p)
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // symlinks can loop, devices are not files
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil || info.Size() > maxBytes {
			return nil
		}
		select {
		case tasks <- task{path: p, name: name, info: info}:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	close(tasks)
	wg.Wait()
	if err != nil {
		return nil, err
	}
	sort.Slice(idx.Docs, func(i, j int) bool { return idx.Docs[i].Path < idx.Docs[j].Path })
	if opt.OnProgress != nil {
		reportMu.Lock()
		opt.OnProgress(Progress{Files: int(files), Dirs: int(dirs)})
		reportMu.Unlock()
	}
	return idx, nil
}

// makeDoc turns one file into its index entry. It runs on a worker goroutine;
// prev is read-only and safe to share.
func makeDoc(root, p, name string, info fs.FileInfo, opt Options, prev map[string]Doc) Doc {
	// Incremental rescan: an unchanged file keeps its old tokens, snippet,
	// dimensions and caption. A reindex is then a stat walk instead of reading
	// hundreds of thousands of files again.
	if old, ok := prev[p]; ok && old.Size == info.Size() && old.Mod == info.ModTime().Unix() {
		old.Path = p
		old.Name = name
		return old
	}

	kind := KindOf(name)
	ext := Extension(name)
	tokens := make([]string, 0, opt.MaxTokens)
	seen := make(map[string]struct{}, opt.MaxTokens*2)
	add := func(words ...string) {
		for _, w := range words {
			if w == "" || len(tokens) >= opt.MaxTokens {
				continue
			}
			if _, dup := seen[w]; dup {
				continue
			}
			seen[w] = struct{}{}
			tokens = append(tokens, w)
		}
	}

	rel, rerr := filepath.Rel(root, p)
	if rerr != nil {
		rel = name
	}
	relDir := filepath.Dir(rel)
	if relDir == "." {
		relDir = ""
	}
	add(Tokenize(relDir)...)
	nameTokens := Tokenize(name)
	add(nameTokens...)
	add(Tokenize(ext)...)
	add(kind)
	add(strconv.Itoa(info.ModTime().Year()))

	doc := Doc{
		Path:       p,
		Name:       name,
		Kind:       kind,
		Ext:        ext,
		Size:       info.Size(),
		Mod:        info.ModTime().Unix(),
		NameTokens: nameTokens,
	}

	switch kind {
	case KindText, KindCode:
		if opt.Content {
			if head, herr := readHead(p, 192<<10); herr == nil {
				add(Tokenize(string(head))...)
				doc.Snippet = makeSnippet(string(head))
			}
		}
	case KindOther:
		// Files without a useful extension are often plain notes too. Sniff
		// the first bytes instead of trusting the name; binaries are rejected
		// by the NUL/UTF-8 check.
		if opt.Content {
			if head, herr := readHead(p, 64<<10); herr == nil && looksLikeText(head) {
				add(Tokenize(string(head))...)
				doc.Snippet = makeSnippet(string(head))
				doc.Kind = KindText
			}
		}
	case KindImage:
		if w, h := ImageSize(p); w > 0 && h > 0 {
			doc.Width, doc.Height = w, h
			switch ratio := float64(w) / float64(h); {
			case ratio > 1.2:
				add("landscape")
			case ratio < 0.83:
				add("portrait")
			default:
				add("square")
			}
		}
		if old, ok := prev[p]; ok && old.Mod == doc.Mod {
			doc.Caption = old.Caption
		}
	}

	doc.Tokens = tokens
	return doc
}

// looksLikeText is deliberately conservative: NUL bytes, invalid UTF-8 and
// control characters mean "binary". It is only used for unknown extensions.
func looksLikeText(b []byte) bool {
	if len(b) == 0 || bytes.IndexByte(b, 0) >= 0 || !utf8.Valid(b) {
		return false
	}
	ctrl := 0
	for _, r := range string(b) {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			ctrl++
		}
	}
	return ctrl*100 <= utf8.RuneCount(b)
}

func readHead(p string, n int64) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, n))
}
