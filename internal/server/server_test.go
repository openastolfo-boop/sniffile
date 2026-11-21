package server

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openastolfo-boop/sniffile/internal/config"
)

func TestScanSearchAndPathSafety(t *testing.T) {
	root := t.TempDir()
	data := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello searchable world"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePNG(t, filepath.Join(root, "pug photo.png"))

	cfg := &config.Config{Version: 1}
	if _, _, err := cfg.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	m := NewManager("test", data, filepath.Join(data, "config.json"), cfg)

	if err := m.StartScan(nil, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for m.State().Job.Running {
		if time.Now().After(deadline) {
			t.Fatal("scan did not finish in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st := m.State(); st.Total != 2 {
		t.Fatalf("indexed %d files, want 2", st.Total)
	}

	ts := httptest.NewServer(NewHandler(m))
	defer ts.Close()

	var sr struct {
		Count   int `json:"count"`
		Results []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"results"`
	}

	getJSON(t, ts.URL+"/api/search?q=hello", &sr)
	if sr.Count != 1 || sr.Results[0].Name != "notes.txt" {
		t.Fatalf("text search: %+v", sr)
	}

	getJSON(t, ts.URL+"/api/search?q=pug", &sr)
	if sr.Count != 1 || sr.Results[0].Name != "pug photo.png" {
		t.Fatalf("name search: %+v", sr)
	}
	picPath := sr.Results[0].Path

	resp, err := http.Get(ts.URL + "/api/file?path=" + url.QueryEscape(picPath))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("file endpoint = %d, want 200", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/thumb?path=" + url.QueryEscape(picPath))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("thumb = %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	resp, err = http.Get(ts.URL + "/api/file?path=" + url.QueryEscape("/etc/passwd"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("outside path = %d, want 403", resp.StatusCode)
	}

	getJSON(t, ts.URL+"/api/state", &struct {
		Total int `json:"total"`
	}{})
}

func writePNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 12, 9))
	for y := 0; y < 9; y++ {
		for x := 0; x < 12; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 25), B: 90, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func getJSON(t *testing.T, url string, dst any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatal(err)
	}
}
