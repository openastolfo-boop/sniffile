// Package config stores the tiny amount of state that survives a restart:
// which folders you added, and whether ollama was switched on.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Root is one indexed folder.
type Root struct {
	Path     string    `json:"path"`
	AddedAt  time.Time `json:"added_at"`
	LastScan time.Time `json:"last_scan,omitempty"`
	Files    int       `json:"files,omitempty"`
}

// Config is the whole file.
type Config struct {
	Version    int    `json:"version"`
	Roots      []Root `json:"roots"`
	OllamaURL  string `json:"ollama_url,omitempty"`
	CaptionMod string `json:"caption_model,omitempty"`
	TextMod    string `json:"text_model,omitempty"`
	NoContent  bool   `json:"no_content,omitempty"`
	MaxFileMB  int64  `json:"max_file_mb,omitempty"`
}

// DefaultDataDir follows the usual XDG-ish habit. SNIFFILE_DATA wins if set.
func DefaultDataDir() (string, error) {
	if dir := os.Getenv("SNIFFILE_DATA"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "sniffile"), nil
}

// Load reads config.json. A missing file is not an error, it is a new install.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Version: 1}, nil
		}
		return nil, err
	}
	defer f.Close()
	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	if c.Version == 0 {
		c.Version = 1
	}
	return &c, nil
}

// Save writes atomically so a kill -9 during save cannot eat the file.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// CleanRoot turns any user input into a canonical absolute directory.
func CleanRoot(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	if p == "~" || len(p) >= 2 && p[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", errors.New("not a folder: " + abs)
	}
	return abs, nil
}

// AddRoot inserts a folder, ignoring duplicates. The bool reports whether it
// was actually new.
func (c *Config) AddRoot(p string) (Root, bool, error) {
	clean, err := CleanRoot(p)
	if err != nil {
		return Root{}, false, err
	}
	for _, r := range c.Roots {
		if r.Path == clean {
			return r, false, nil
		}
	}
	r := Root{Path: clean, AddedAt: time.Now().UTC()}
	c.Roots = append(c.Roots, r)
	return r, true, nil
}

// RemoveRoot drops a folder by path.
func (c *Config) RemoveRoot(p string) bool {
	clean, err := CleanRoot(p)
	if err != nil {
		clean = p
	}
	for i, r := range c.Roots {
		if r.Path == clean || r.Path == p {
			c.Roots = append(c.Roots[:i], c.Roots[i+1:]...)
			return true
		}
	}
	return false
}

// HasRoot reports whether p is already indexed.
func (c *Config) HasRoot(p string) bool {
	for _, r := range c.Roots {
		if r.Path == p {
			return true
		}
	}
	return false
}
