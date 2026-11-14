package search

import (
	"testing"

	"github.com/kopeyka/sniffile/internal/index"
)

func doc(path, name string, kind string, nameTokens []string, tokens []string, caption string) index.Doc {
	return index.Doc{
		Path:       path,
		Name:       name,
		Kind:       kind,
		NameTokens: nameTokens,
		Tokens:     tokens,
		Caption:    caption,
	}
}

func testCorpus() *Corpus {
	return NewCorpus([]index.Doc{
		doc("/home/u/Pictures/pug_sofa.jpg", "pug_sofa.jpg", index.KindImage,
			[]string{"pug", "sofa"},
			[]string{"pictures", "image", "landscape", "2024"},
			"A cute pug sitting on a grey sofa"),
		doc("/home/u/Pictures/cat_window.jpg", "cat_window.jpg", index.KindImage,
			[]string{"cat", "window"},
			[]string{"pictures", "image", "portrait", "2024"},
			"A tabby cat looking out of a rainy window"),
		doc("/home/u/Documents/invoice_2024.pdf", "invoice_2024.pdf", index.KindDoc,
			[]string{"invoice", "2024", "pdf"},
			[]string{"documents", "doc", "2024"},
			""),
		doc("/home/u/code/main.go", "main.go", index.KindCode,
			[]string{"main", "go"},
			[]string{"code", "package", "main", "func", "search"},
			""),
	})
}

func TestRussianQueryFindsCaption(t *testing.T) {
	res := testCorpus().Search("фотка с красивым мопсом", Options{Limit: 10})
	if len(res) == 0 {
		t.Fatal("no results")
	}
	if res[0].Name != "pug_sofa.jpg" {
		t.Fatalf("top result = %q, want pug_sofa.jpg (all: %v)", res[0].Name, names(res))
	}
}

func TestNameBeatsContent(t *testing.T) {
	res := testCorpus().Search("cat", Options{Limit: 10})
	if len(res) == 0 || res[0].Name != "cat_window.jpg" {
		t.Fatalf("top result = %v, want cat_window.jpg", names(res))
	}
}

func TestKindFilter(t *testing.T) {
	res := testCorpus().Search("2024", Options{Kind: index.KindDoc, Limit: 10})
	if len(res) != 1 || res[0].Name != "invoice_2024.pdf" {
		t.Fatalf("got %v, want only the invoice", names(res))
	}
}

func TestFuzzy(t *testing.T) {
	res := testCorpus().Search("pugg", Options{Limit: 10})
	if len(res) == 0 || res[0].Name != "pug_sofa.jpg" {
		t.Fatalf("fuzzy got %v, want pug_sofa.jpg", names(res))
	}
}

func TestRussianStemMatchesEnglishFile(t *testing.T) {
	// "мопсом" must land on the "pug" tokens through the thesaurus
	res := testCorpus().Search("мопсом", Options{Limit: 10})
	if len(res) == 0 || res[0].Name != "pug_sofa.jpg" {
		t.Fatalf("got %v, want pug_sofa.jpg", names(res))
	}
}

func TestNoQuery(t *testing.T) {
	if res := testCorpus().Search("   ", Options{}); res != nil {
		t.Fatalf("expected nil for empty query, got %v", names(res))
	}
}

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"pug", "pug", true},
		{"pugg", "pug", true},
		{"pug", "pugs", true},
		{"pug", "pig", true},
		{"pug", "pugsy", false},
		{"кошка", "кошке", true},
	}
	for _, c := range cases {
		if got := editDistanceAtMost1(c.a, c.b); got != c.want {
			t.Errorf("editDistanceAtMost1(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func names(res []Result) []string {
	out := make([]string, 0, len(res))
	for _, r := range res {
		out = append(out, r.Name)
	}
	return out
}
