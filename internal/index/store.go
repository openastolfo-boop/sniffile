package index

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Key is a short, stable name for a root folder. Full paths are ugly on disk.
func Key(root string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	return hex.EncodeToString(sum[:])[:16]
}

// FilePath is where the index for root lives inside dir.
func FilePath(dir, root string) string {
	return filepath.Join(dir, Key(root)+".json.gz")
}

// Save writes the index atomically: temp file, fsync-ish close, rename.
func Save(dir string, idx *Index) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	final := FilePath(dir, idx.Root)
	tmp := final + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	gz, _ := gzip.NewWriterLevel(f, gzip.BestSpeed)
	enc := json.NewEncoder(gz)
	if err := enc.Encode(idx); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := gz.Close(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, final)
}

// Load reads a saved index. Missing files are a normal first-run situation.
func Load(dir, root string) (*Index, error) {
	f, err := os.Open(FilePath(dir, root))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	var idx Index
	if err := json.NewDecoder(gz).Decode(&idx); err != nil {
		return nil, err
	}
	if idx.Version != Version {
		return nil, fmt.Errorf("index format %d, this build wants %d", idx.Version, Version)
	}
	return &idx, nil
}

// Delete removes a saved index, used when a root is dropped.
func Delete(dir, root string) error {
	err := os.Remove(FilePath(dir, root))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
