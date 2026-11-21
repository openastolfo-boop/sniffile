// Package enrich is the optional "make it feel smarter" layer.
//
// It talks to a local ollama instance: a text model turns a query into a few
// English keywords, a vision model writes one-line captions for photos so
// "cute pug" can find pug_2023.jpg. Both are best-effort. No ollama, no drama:
// the plain index keeps working.
package enrich

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/openastolfo-boop/sniffile/internal/index"
)

const defaultBase = "http://127.0.0.1:11434"

// Client is a thin ollama wrapper with a small model cache.
type Client struct {
	base string
	http *http.Client

	mu      sync.Mutex
	models  []string
	text    string
	vision  string
	checked time.Time
	lastErr error
}

// New returns nil when base is empty or "off", which keeps the manager simple.
func New(base string) *Client {
	if base == "" || base == "off" {
		return nil
	}
	if base == "auto" {
		base = defaultBase
	}
	return &Client{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{Timeout: 90 * time.Second},
	}
}

// Status returns the chosen models, refreshing the list if it is stale.
func (c *Client) Status(ctx context.Context) (text, vision string, err error) {
	if err := c.refresh(ctx, false); err != nil {
		return "", "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text, c.vision, c.lastErr
}

// Cached returns the last detected models without touching the network.
func (c *Client) Cached() (text, vision string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text, c.vision
}

// Keywords asks a text model for English search words.
func (c *Client) Keywords(ctx context.Context, q string) ([]string, error) {
	if err := c.refresh(ctx, false); err != nil {
		return nil, err
	}
	c.mu.Lock()
	model := c.text
	c.mu.Unlock()
	if model == "" {
		return nil, errors.New("no text model available in ollama")
	}
	prompt := "You turn file search queries into English search keywords. " +
		"Reply with up to 8 lowercase keywords separated by commas. " +
		"No sentences, no explanations.\n\nQuery: " + q
	raw, err := c.generate(ctx, model, prompt, nil, map[string]any{
		"temperature": 0,
		"num_predict": 40,
	})
	if err != nil {
		return nil, err
	}
	return keywordsFrom(raw, q), nil
}

// DescribeImage returns a short caption for a local image file.
func (c *Client) DescribeImage(ctx context.Context, path string) (string, error) {
	if err := c.refresh(ctx, false); err != nil {
		return "", err
	}
	c.mu.Lock()
	model := c.vision
	c.mu.Unlock()
	if model == "" {
		return "", errors.New("no vision model in ollama (try: ollama pull llava)")
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if st.Size() > 12<<20 {
		return "", errors.New("image too large to describe")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	prompt := "Describe this image in one short sentence of at most 20 words. " +
		"Name the main subject, the setting and the dominant colours. " +
		"Do not start with 'The image'."
	raw, err := c.generate(ctx, model, prompt, []string{base64.StdEncoding.EncodeToString(data)}, map[string]any{
		"temperature": 0,
		"num_predict": 60,
	})
	if err != nil {
		return "", err
	}
	return cleanCaption(raw), nil
}

func (c *Client) refresh(ctx context.Context, force bool) error {
	c.mu.Lock()
	if !force && !c.checked.IsZero() && time.Since(c.checked) < 5*time.Minute {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	var tags struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	err := c.getJSON(ctx, "/api/tags", &tags)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.checked = time.Now()
	if err != nil {
		c.lastErr = err
		return err
	}
	c.lastErr = nil
	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		n := m.Name
		if n == "" {
			n = m.Model
		}
		names = append(names, n)
	}
	c.models = names
	c.text, c.vision = pickModels(names)
	return nil
}

func pickModels(names []string) (text, vision string) {
	for _, n := range names {
		if isVision(n) {
			vision = n
			break
		}
	}
	prefer := []string{"llama3.2", "qwen2.5", "gemma2", "mistral", "phi3", "llama3"}
	for _, p := range prefer {
		for _, n := range names {
			if strings.Contains(strings.ToLower(n), p) && !isVision(n) && !strings.Contains(strings.ToLower(n), "embed") {
				text = n
				break
			}
		}
		if text != "" {
			break
		}
	}
	if text == "" {
		for _, n := range names {
			if !isVision(n) && !strings.Contains(strings.ToLower(n), "embed") {
				text = n
				break
			}
		}
	}
	return text, vision
}

func isVision(name string) bool {
	n := strings.ToLower(name)
	for _, marker := range []string{"llava", "moondream", "bakllava", "minicpm-v", "llama3.2-vision", "qwen2.5vl", "qwen2.5-vl", "gemma3", "-vl"} {
		if strings.Contains(n, marker) {
			return true
		}
	}
	return false
}

type genRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Images  []string       `json:"images,omitempty"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options,omitempty"`
}

type genResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

func (c *Client) generate(ctx context.Context, model, prompt string, images []string, opts map[string]any) (string, error) {
	body := genRequest{Model: model, Prompt: prompt, Images: images, Options: opts}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/generate", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("ollama: %s", strings.TrimSpace(string(msg)))
	}
	var gr genResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&gr); err != nil {
		return "", err
	}
	if gr.Error != "" {
		return "", errors.New(gr.Error)
	}
	return strings.TrimSpace(gr.Response), nil
}

func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama: http %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(v)
}

// keywordsFrom salvages words from a model reply, even a chatty one.
func keywordsFrom(raw, query string) []string {
	seen := map[string]struct{}{}
	for _, w := range index.Tokenize(query) {
		seen[w] = struct{}{}
	}
	out := make([]string, 0, 8)
	for _, w := range index.Normalize(raw) {
		if utf8.RuneCountInString(w) < 3 || utf8.RuneCountInString(w) > 24 || index.IsStopword(w) {
			continue
		}
		if _, dup := seen[w]; dup {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, w)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func cleanCaption(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	for _, prefix := range []string{"The image shows ", "The image is ", "This image shows ", "This is ", "Image: "} {
		if len(s) > len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}
	s = strings.Trim(s, "\"'` ")
	if utf8.RuneCountInString(s) > 200 {
		rs := []rune(s)
		s = string(rs[:200]) + "…"
	}
	rs := []rune(s)
	if len(rs) > 0 && unicode.IsLower(rs[0]) {
		rs[0] = unicode.ToUpper(rs[0])
		s = string(rs)
	}
	return s
}
