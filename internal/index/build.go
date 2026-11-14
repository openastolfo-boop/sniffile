package index

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Version is bumped whenever the on-disk shape changes. Old indexes are simply
// rebuilt; there is nothing precious inside.
const Version = 1

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
		opt.MaxTokens = 72
	}
	maxBytes := opt.MaxFileMB << 20

	prev := map[string]Doc{}
	if opt.Prev != nil {
		for _, d := range opt.Prev.Docs {
			if d.Caption != "" {
				prev[d.Path] = d
			}
		}
	}

	idx := &Index{Version: Version, Root: root, BuiltAt: time.Now().UTC()}
	var files, dirs int
	lastReport := time.Now().Add(-time.Second)
	report := func(p string) {
		if opt.OnProgress == nil || time.Since(lastReport) < 120*time.Millisecond {
			return
		}
		lastReport = time.Now()
		opt.OnProgress(Progress{Files: files, Dirs: dirs, Path: p})
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
			dirs++
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
		idx.Docs = append(idx.Docs, doc)
		files++
		report(p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(idx.Docs, func(i, j int) bool { return idx.Docs[i].Path < idx.Docs[j].Path })
	if opt.OnProgress != nil {
		opt.OnProgress(Progress{Files: files, Dirs: dirs})
	}
	return idx, nil
}

func readHead(p string, n int64) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, n))
}
