package server

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kopeyka/sniffile/internal/config"
	"github.com/kopeyka/sniffile/internal/enrich"
	"github.com/kopeyka/sniffile/internal/index"
	"github.com/kopeyka/sniffile/internal/search"
)

// Job is the scan/describe progress that the UI polls twice a second.
type Job struct {
	Running  bool   `json:"running"`
	Phase    string `json:"phase"` // idle | scanning | describing | done | error | cancelled
	Root     string `json:"root,omitempty"`
	Roots    int    `json:"roots_total"`
	RootDone int    `json:"roots_done"`
	Files    int    `json:"files"`
	Dirs     int    `json:"dirs"`
	Path     string `json:"path,omitempty"`
	Captions int    `json:"captions"`
	Failures int    `json:"failures"`
	Started  int64  `json:"started,omitempty"`
	Ended    int64  `json:"ended,omitempty"`
	Error    string `json:"error,omitempty"`
}

// RootState is one chip in the UI.
type RootState struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Files int    `json:"files"`
	Built int64  `json:"built_at,omitempty"`
}

// State is the full answer to GET /api/state.
type State struct {
	Version      string      `json:"version"`
	Home         string      `json:"home"`
	DataDir      string      `json:"data_dir"`
	Roots        []RootState `json:"roots"`
	Total        int         `json:"total"`
	Ollama       bool        `json:"ollama"`
	OllamaText   string      `json:"ollama_text,omitempty"`
	OllamaVision string      `json:"ollama_vision,omitempty"`
	Job          Job         `json:"job"`
}

// Manager owns the config, the indexes and the single background job slot.
type Manager struct {
	version  string
	dataDir  string
	cfgPath  string
	indexDir string

	enrich *enrich.Client

	mu      sync.RWMutex
	cfg     *config.Config
	indexes map[string]*index.Index
	corpora map[string]*search.Corpus
	job     Job
	cancel  context.CancelFunc

	expandMu sync.Mutex
	expanded map[string]expandEntry
}

type expandEntry struct {
	words []string
	at    time.Time
}

// NewManager loads saved indexes. Missing indexes are quietly ignored.
func NewManager(version, dataDir, cfgPath string, cfg *config.Config) *Manager {
	m := &Manager{
		version:  version,
		dataDir:  dataDir,
		cfgPath:  cfgPath,
		indexDir: filepath.Join(dataDir, "index"),
		cfg:      cfg,
		indexes:  make(map[string]*index.Index, len(cfg.Roots)),
		corpora:  make(map[string]*search.Corpus, len(cfg.Roots)),
		expanded: make(map[string]expandEntry, 32),
		job:      Job{Phase: "idle"},
		enrich:   enrich.New(cfg.OllamaURL),
	}
	for _, r := range cfg.Roots {
		if idx, err := index.Load(m.indexDir, r.Path); err == nil {
			m.indexes[r.Path] = idx
		}
	}
	if m.enrich != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			text, vision, err := m.enrich.Status(ctx)
			if err != nil {
				log.Printf("ollama: %v — plain filename search still works", err)
				return
			}
			log.Printf("ollama: text model %q, vision model %q", text, vision)
		}()
	}
	return m
}

// State builds a consistent snapshot for the UI.
func (m *Manager) State() State {
	home, _ := os.UserHomeDir()
	m.mu.RLock()
	defer m.mu.RUnlock()
	st := State{
		Version: m.version,
		Home:    home,
		DataDir: m.dataDir,
		Roots:   make([]RootState, 0, len(m.cfg.Roots)),
		Ollama:  m.enrich != nil,
		Job:     m.job,
	}
	if m.enrich != nil {
		st.OllamaText, st.OllamaVision = m.enrich.Cached()
	}
	for _, r := range m.cfg.Roots {
		rs := RootState{Path: r.Path, Name: filepath.Base(r.Path), Files: r.Files}
		if idx := m.indexes[r.Path]; idx != nil {
			rs.Files = len(idx.Docs)
			rs.Built = idx.BuiltAt.Unix()
		}
		st.Roots = append(st.Roots, rs)
		st.Total += rs.Files
	}
	return st
}

// AddRoot registers a folder and immediately queues a scan for it.
func (m *Manager) AddRoot(path string) (State, error) {
	clean, err := config.CleanRoot(path)
	if err != nil {
		return State{}, err
	}
	m.mu.Lock()
	_, added, err := m.cfg.AddRoot(clean)
	if err != nil {
		m.mu.Unlock()
		return State{}, err
	}
	if added {
		if err := m.cfg.Save(m.cfgPath); err != nil {
			log.Printf("config save: %v", err)
		}
	}
	m.mu.Unlock()
	if added {
		if err := m.StartScan([]string{clean}, false); err != nil {
			log.Printf("scan %s: %v", clean, err)
		}
	}
	return m.State(), nil
}

// RemoveRoot forgets a folder and deletes its index file.
func (m *Manager) RemoveRoot(path string) (State, error) {
	m.mu.Lock()
	removed := m.cfg.RemoveRoot(path)
	if removed {
		delete(m.indexes, path)
		delete(m.corpora, path)
		if err := m.cfg.Save(m.cfgPath); err != nil {
			log.Printf("config save: %v", err)
		}
	}
	m.mu.Unlock()
	if removed {
		if err := index.Delete(m.indexDir, path); err != nil {
			log.Printf("delete index: %v", err)
		}
	}
	return m.State(), nil
}

// StartScan starts one background job. Only one may run at a time, which is a
// feature: mechanical sympathy for spinning disks.
func (m *Manager) StartScan(roots []string, describeImages bool) error {
	m.mu.Lock()
	if m.job.Running {
		m.mu.Unlock()
		return errors.New("a scan is already running")
	}
	if len(roots) == 0 {
		for _, r := range m.cfg.Roots {
			roots = append(roots, r.Path)
		}
	}
	if len(roots) == 0 {
		m.mu.Unlock()
		return errors.New("no folders to scan")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.job = Job{
		Running: true,
		Phase:   "scanning",
		Roots:   len(roots),
		Started: time.Now().Unix(),
	}
	m.mu.Unlock()

	go m.runJob(ctx, roots, describeImages)
	return nil
}

// Cancel stops the running job at the next directory boundary.
func (m *Manager) Cancel() {
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) runJob(ctx context.Context, roots []string, describeImages bool) {
	failures := 0
	for i, root := range roots {
		if ctx.Err() != nil {
			break
		}
		m.updateJob(func(j *Job) {
			j.Root = root
			j.RootDone = i
			j.Files, j.Dirs, j.Path = 0, 0, root
		})

		m.mu.RLock()
		prev := m.indexes[root]
		maxMB := m.cfg.MaxFileMB
		content := !m.cfg.NoContent
		m.mu.RUnlock()

		idx, err := index.Build(ctx, root, index.Options{
			MaxFileMB:  maxMB,
			Content:    content,
			SkipHidden: true,
			Prev:       prev,
			OnProgress: func(p index.Progress) {
				m.updateJob(func(j *Job) {
					j.Files = p.Files
					j.Dirs = p.Dirs
					if p.Path != "" {
						j.Path = p.Path
					}
				})
			},
		})
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			failures++
			log.Printf("scan %s: %v", root, err)
			m.updateJob(func(j *Job) { j.Error = err.Error() })
			continue
		}

		m.mu.Lock()
		m.indexes[root] = idx
		delete(m.corpora, root)
		now := time.Now().UTC()
		for k := range m.cfg.Roots {
			if m.cfg.Roots[k].Path == root {
				m.cfg.Roots[k].LastScan = now
				m.cfg.Roots[k].Files = len(idx.Docs)
			}
		}
		if err := m.cfg.Save(m.cfgPath); err != nil {
			log.Printf("config save: %v", err)
		}
		m.mu.Unlock()
		if err := index.Save(m.indexDir, idx); err != nil {
			log.Printf("save index %s: %v", root, err)
		}
		m.updateJob(func(j *Job) { j.RootDone = i + 1 })

		if describeImages && m.enrich != nil {
			m.describe(ctx, root, idx)
		}
	}

	m.mu.Lock()
	phase := "done"
	if ctx.Err() != nil {
		phase = "cancelled"
	} else if failures == len(roots) {
		phase = "error"
	}
	m.job.Running = false
	m.job.Phase = phase
	m.job.Ended = time.Now().Unix()
	m.job.Path = ""
	m.job.Root = ""
	m.mu.Unlock()
}

// describe asks the vision model for captions on images that don't have one.
// It never blocks the scan forever: a handful of failures in a row and it
// gives up with a readable error.
func (m *Manager) describe(ctx context.Context, root string, idx *index.Index) {
	m.updateJob(func(j *Job) {
		j.Phase = "describing"
		j.Captions = 0
		j.Files, j.Dirs = 0, 0
	})
	added := 0
	consecutive := 0
	var lastErr error
	for i := range idx.Docs {
		if ctx.Err() != nil {
			return
		}
		d := &idx.Docs[i]
		if d.Kind != index.KindImage || d.Caption != "" {
			continue
		}
		one, cancel := context.WithTimeout(ctx, 45*time.Second)
		caption, err := m.enrich.DescribeImage(one, d.Path)
		cancel()
		if err != nil {
			consecutive++
			lastErr = err
			m.updateJob(func(j *Job) { j.Failures++ })
			if consecutive >= 4 {
				m.updateJob(func(j *Job) { j.Error = err.Error() })
				break
			}
			continue
		}
		consecutive = 0
		d.Caption = caption
		added++
		m.updateJob(func(j *Job) {
			j.Captions++
			j.Path = d.Path
		})
		if added%25 == 0 {
			if err := index.Save(m.indexDir, idx); err != nil {
				log.Printf("save captions: %v", err)
			}
		}
	}
	if added > 0 {
		if err := index.Save(m.indexDir, idx); err != nil {
			log.Printf("save captions: %v", err)
		}
		m.mu.Lock()
		delete(m.corpora, root)
		m.mu.Unlock()
	}
	if lastErr != nil && added == 0 {
		m.updateJob(func(j *Job) { j.Error = lastErr.Error() })
	}
	if ctx.Err() == nil {
		m.updateJob(func(j *Job) { j.Phase = "scanning" })
	}
}

func (m *Manager) updateJob(fn func(*Job)) {
	m.mu.Lock()
	fn(&m.job)
	m.mu.Unlock()
}

// rootsFor resolves the UI's scope chip into concrete index roots.
func (m *Manager) rootsFor(root string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if root == "" || root == "all" || root == "*" {
		out := make([]string, 0, len(m.cfg.Roots))
		for _, r := range m.cfg.Roots {
			out = append(out, r.Path)
		}
		return out
	}
	return []string{root}
}

// corpus loads/builds the in-memory inverted index for one root.
func (m *Manager) corpus(root string) *search.Corpus {
	m.mu.RLock()
	c := m.corpora[root]
	idx := m.indexes[root]
	m.mu.RUnlock()
	if c != nil {
		return c
	}
	if idx == nil {
		loaded, err := index.Load(m.indexDir, root)
		if err != nil {
			return nil
		}
		m.mu.Lock()
		if m.indexes[root] == nil {
			m.indexes[root] = loaded
		}
		idx = m.indexes[root]
		m.mu.Unlock()
	}
	c = search.NewCorpus(idx.Docs)
	m.mu.Lock()
	m.corpora[root] = c
	m.mu.Unlock()
	return c
}

// Search queries one root, several roots, or everything.
func (m *Manager) Search(q, root, kind string, limit int) ([]search.Result, []string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	var all []search.Result
	for _, r := range m.rootsFor(root) {
		if c := m.corpus(r); c != nil {
			all = append(all, c.Search(q, search.Options{Kind: kind, Limit: limit})...)
		}
	}
	all = mergeResults(all, limit)

	var expanded []string
	if len(all) == 0 && m.enrich != nil && len(m.rootsFor(root)) > 0 {
		expanded = m.expand(q)
		if len(expanded) > 0 {
			for _, r := range m.rootsFor(root) {
				if c := m.corpus(r); c != nil {
					all = append(all, c.Search(q, search.Options{Kind: kind, Limit: limit, Extra: expanded})...)
				}
			}
			all = mergeResults(all, limit)
		}
	}
	return all, expanded
}

// expand asks ollama for English keywords. The cache is tiny and dumb on
// purpose; a personal search tool doesn't need an embedding warehouse.
func (m *Manager) expand(q string) []string {
	key := strings.ToLower(strings.TrimSpace(q))
	m.expandMu.Lock()
	if e, ok := m.expanded[key]; ok && time.Since(e.at) < 10*time.Minute {
		m.expandMu.Unlock()
		return e.words
	}
	m.expandMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	words, err := m.enrich.Keywords(ctx, q)
	if err != nil {
		log.Printf("query expansion: %v", err)
		words = nil
	}
	m.expandMu.Lock()
	if len(m.expanded) > 256 {
		m.expanded = make(map[string]expandEntry, 64)
	}
	m.expanded[key] = expandEntry{words: words, at: time.Now()}
	m.expandMu.Unlock()
	return words
}

func mergeResults(in []search.Result, limit int) []search.Result {
	best := make(map[string]search.Result, len(in))
	for _, r := range in {
		if old, ok := best[r.Path]; ok && old.Score >= r.Score {
			continue
		}
		best[r.Path] = r
	}
	out := make([]search.Result, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Path < out[j].Path
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Resolve validates that a path really lives inside one of the added roots.
// Symlinks are resolved first, so ../.. tricks don't work.
func (m *Manager) Resolve(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	m.mu.RLock()
	roots := make([]string, 0, len(m.cfg.Roots))
	for _, r := range m.cfg.Roots {
		roots = append(roots, r.Path)
	}
	m.mu.RUnlock()
	for _, root := range roots {
		if real == root || strings.HasPrefix(real, root+string(os.PathSeparator)) {
			return real, nil
		}
	}
	return "", errors.New("outside the folders you added")
}

// OpenInSystem launches the desktop default app. reveal=true opens the
// containing folder instead.
func (m *Manager) OpenInSystem(p string, reveal bool) error {
	real, err := m.Resolve(p)
	if err != nil {
		return err
	}
	if reveal {
		real = filepath.Dir(real)
	}
	return openPath(real)
}

// BrowseEntry is one folder in the picker.
type BrowseEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Browse lists subfolders. It deliberately walks the whole disk: you need to
// be able to pick a folder that is not indexed yet.
type BrowseResult struct {
	Path      string        `json:"path"`
	Parent    string        `json:"parent,omitempty"`
	Home      string        `json:"home"`
	Dirs      []BrowseEntry `json:"dirs"`
	Truncated bool          `json:"truncated,omitempty"`
}

// Browse returns subfolders of path (default: home).
func (m *Manager) Browse(path string) (*BrowseResult, error) {
	home, _ := os.UserHomeDir()
	if path == "" {
		path = home
	}
	if path == "~" {
		path = home
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(home, path[2:])
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		abs = filepath.Dir(abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	res := &BrowseResult{Path: abs, Home: home}
	if parent := filepath.Dir(abs); parent != abs {
		res.Parent = parent
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if e.Type()&os.ModeSymlink != 0 {
			continue // avoid surprise loops in the picker
		}
		res.Dirs = append(res.Dirs, BrowseEntry{Name: e.Name(), Path: filepath.Join(abs, e.Name())})
		if len(res.Dirs) >= 1000 {
			res.Truncated = true
			break
		}
	}
	sort.Slice(res.Dirs, func(i, j int) bool {
		return strings.ToLower(res.Dirs[i].Name) < strings.ToLower(res.Dirs[j].Name)
	})
	return res, nil
}
