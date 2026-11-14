package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kopeyka/sniffile/internal/search"
	"github.com/kopeyka/sniffile/web"
)

// Server is just the manager plus a thumbnail cache.
type Server struct {
	m      *Manager
	thumbs *thumbCache
}

// NewHandler wires all routes. There is no framework here, and that is fine.
func NewHandler(m *Manager) http.Handler {
	s := &Server{m: m, thumbs: newThumbCache(192)}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.Handle("/assets/", http.StripPrefix("/assets/", s.assets()))
	mux.HandleFunc("/api/state", s.state)
	mux.HandleFunc("/api/roots", s.roots)
	mux.HandleFunc("/api/scan", s.scan)
	mux.HandleFunc("/api/scan/cancel", s.cancelScan)
	mux.HandleFunc("/api/search", s.search)
	mux.HandleFunc("/api/file", s.file)
	mux.HandleFunc("/api/thumb", s.thumb)
	mux.HandleFunc("/api/open", s.open)
	mux.HandleFunc("/api/browse", s.browse)
	return s.security(mux)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(web.Index)
}

func (s *Server) assets() http.Handler {
	sub, err := fs.Sub(web.Assets, "assets")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// no hashes in filenames, so no long cache: hard reload wins
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.m.State())
}

func (s *Server) roots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Path string `json:"path"`
		}
		if !readJSON(w, r, &body) {
			return
		}
		st, err := s.m.AddRoot(body.Path)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	case http.MethodDelete:
		path := r.URL.Query().Get("path")
		if path == "" {
			jsonError(w, http.StatusBadRequest, errors.New("path is required"))
			return
		}
		st, err := s.m.RemoveRoot(path)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Root   string   `json:"root"`
		Roots  []string `json:"roots"`
		Enrich bool     `json:"enrich"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	roots := body.Roots
	if body.Root != "" && body.Root != "all" {
		roots = []string{body.Root}
	}
	if err := s.m.StartScan(roots, body.Enrich); err != nil {
		jsonError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, s.m.State())
}

func (s *Server) cancelScan(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}
	s.m.Cancel()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type searchResponse struct {
	Query    string          `json:"query"`
	Root     string          `json:"root"`
	Kind     string          `json:"kind"`
	Count    int             `json:"count"`
	TookMS   int64           `json:"took_ms"`
	Expanded []string        `json:"expanded,omitempty"`
	Results  []search.Result `json:"results"`
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	root := r.URL.Query().Get("root")
	kind := r.URL.Query().Get("kind")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	started := time.Now()
	results, expanded := s.m.Search(q, root, kind, limit)
	if results == nil {
		results = []search.Result{}
	}
	writeJSON(w, http.StatusOK, searchResponse{
		Query:    q,
		Root:     root,
		Kind:     kind,
		Count:    len(results),
		TookMS:   time.Since(started).Milliseconds(),
		Expanded: expanded,
		Results:  results,
	})
}

func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet, http.MethodHead) {
		return
	}
	real, err := s.m.Resolve(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, http.StatusForbidden, err)
		return
	}
	st, err := os.Stat(real)
	if err != nil || st.IsDir() {
		http.Error(w, "not a file", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=60")
	http.ServeFile(w, r, real)
}

func (s *Server) thumb(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet, http.MethodHead) {
		return
	}
	real, err := s.m.Resolve(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, http.StatusForbidden, err)
		return
	}
	st, err := os.Stat(real)
	if err != nil || st.IsDir() {
		http.Error(w, "not a file", http.StatusNotFound)
		return
	}
	data, err := s.thumbs.get(real, st.ModTime().Unix(), st.Size())
	if err != nil {
		http.Error(w, "no preview", http.StatusUnsupportedMediaType)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Write(data)
}

func (s *Server) open(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Path   string `json:"path"`
		Reveal bool   `json:"reveal"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if err := s.m.OpenInSystem(body.Path, body.Reveal); err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet) {
		return
	}
	res, err := s.m.Browse(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// security is the boring middle layer: no sniffing, no framing, no inline
// scripts. The UI is plain embedded files, so it costs nothing.
func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'")
		// Reject cross-origin writes. Keeps a random tab from opening files.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" {
				if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
					jsonError(w, http.StatusForbidden, errors.New("cross-origin request refused"))
					return
				}
			}
		}
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic on %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func allow(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, m := range methods {
		if r.Method == m {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(methods, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		jsonError(w, http.StatusBadRequest, errors.New("bad json: "+err.Error()))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode: %v", err)
	}
}

func jsonError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
