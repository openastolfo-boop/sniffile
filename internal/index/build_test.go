package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

func TestBuildParallelDeterministic(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 40; i++ {
		name := filepath.Join(root, fmt.Sprintf("note-%02d.txt", i))
		if err := os.WriteFile(name, []byte(fmt.Sprintf("pugs and invoices number %d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	idx, err := Build(context.Background(), root, Options{Content: true, MaxTokens: 32})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(idx.Docs), 40; got != want {
		t.Fatalf("indexed %d files, want %d", got, want)
	}
	if !sort.SliceIsSorted(idx.Docs, func(i, j int) bool { return idx.Docs[i].Path < idx.Docs[j].Path }) {
		t.Fatal("documents are not sorted by path")
	}
	for _, d := range idx.Docs {
		if d.Snippet == "" {
			t.Fatalf("%s: text content was not read", d.Path)
		}
	}
}

func TestBuildExtensionlessText(t *testing.T) {
	root := t.TempDir()
	note := filepath.Join(root, "жд")
	if err := os.WriteFile(note, []byte("лошадь мопс кровать"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "blob.bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}

	idx, err := Build(context.Background(), root, Options{Content: true})
	if err != nil {
		t.Fatal(err)
	}
	var found *Doc
	for i := range idx.Docs {
		if idx.Docs[i].Name == "жд" {
			found = &idx.Docs[i]
		}
	}
	if found == nil {
		t.Fatal("extensionless file was not indexed")
	}
	if found.Kind != KindText {
		t.Fatalf("kind = %q, want text", found.Kind)
	}
	if found.Snippet == "" {
		t.Fatal("extensionless text was not read")
	}
	hasHorse := false
	for _, tok := range found.Tokens {
		if tok == Stem("лошадь") {
			hasHorse = true
		}
	}
	if !hasHorse {
		t.Fatalf("content tokens %v do not contain the horse stem", found.Tokens)
	}
}

func TestBuildOnDoc(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 25; i++ {
		name := filepath.Join(root, fmt.Sprintf("part-%02d.md", i))
		if err := os.WriteFile(name, []byte("streaming results"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex
	seen := map[string]bool{}
	idx, err := Build(context.Background(), root, Options{
		Content: true,
		OnDoc: func(d Doc) {
			mu.Lock()
			seen[d.Path] = true
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != len(idx.Docs) {
		t.Fatalf("OnDoc saw %d documents, index has %d", len(seen), len(idx.Docs))
	}
	for _, d := range idx.Docs {
		if !seen[d.Path] {
			t.Fatalf("OnDoc missed %s", d.Path)
		}
	}
}

func TestBuildCancel(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 10; i++ {
		_ = os.WriteFile(filepath.Join(root, fmt.Sprintf("f%d.txt", i)), []byte("hello"), 0o644)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, root, Options{Content: true}); err == nil {
		t.Fatal("expected a cancellation error")
	}
}
