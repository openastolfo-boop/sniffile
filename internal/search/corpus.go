package search

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kopeyka/sniffile/internal/index"
)

type posting struct {
	doc int32
	w   float32
}

// Corpus is the in-memory inverted index. It is built lazily from an
// index.Index; a 50k file library takes well under a second on a laptop.
type Corpus struct {
	Docs   []index.Doc
	post   map[string][]posting
	docLen []float32
	avgLen float32
	vocab  map[rune][]string
}

// Options narrow and tune one search.
type Options struct {
	Kind  string
	Limit int
	Extra []string // extra raw words, e.g. from an LLM expansion
}

// Result is one row in the UI.
type Result struct {
	Path    string   `json:"path"`
	Name    string   `json:"name"`
	Dir     string   `json:"dir"`
	Kind    string   `json:"kind"`
	Size    int64    `json:"size"`
	Mod     int64    `json:"mod"`
	Score   float32  `json:"score"`
	Matched []string `json:"matched,omitempty"`
	Snippet string   `json:"snippet,omitempty"`
	Caption string   `json:"caption,omitempty"`
	Width   int      `json:"width,omitempty"`
	Height  int      `json:"height,omitempty"`
}

// NewCorpus flattens the docs into postings. Field weights are dead simple:
// file name 3, caption 2.5, everything else 1.
func NewCorpus(docs []index.Doc) *Corpus {
	// Snapshot the slice: scans may add captions to the index while a search
	// is running, and a read-only corpus is much easier to reason about.
	docs = append([]index.Doc(nil), docs...)
	c := &Corpus{
		Docs: docs,
		post: make(map[string][]posting, len(docs)*2),
	}
	n := len(docs)
	c.docLen = make([]float32, n)
	var total float32
	for i := range docs {
		d := &docs[i]
		w := make(map[string]float32, len(d.Tokens)+len(d.NameTokens)+8)
		for _, t := range d.Tokens {
			w[t] = 1
		}
		for _, t := range d.NameTokens {
			w[t] = 3
		}
		if d.Caption != "" {
			for _, t := range index.Tokenize(d.Caption) {
				if w[t] < 2.5 {
					w[t] = 2.5
				}
			}
		}
		var length float32
		for t, wt := range w {
			c.post[t] = append(c.post[t], posting{doc: int32(i), w: wt})
			length += wt
		}
		c.docLen[i] = length
		total += length
	}
	if n > 0 {
		c.avgLen = total / float32(n)
	}
	if c.avgLen == 0 {
		c.avgLen = 1
	}
	c.vocab = make(map[rune][]string, 256)
	for t := range c.post {
		r := []rune(t)[0]
		c.vocab[r] = append(c.vocab[r], t)
	}
	return c
}

// Search runs BM25 over the corpus, then applies a few human-sounding boosts:
// whole-query-in-name, matched-in-name, and coverage of the original words.
func (c *Corpus) Search(q string, opt Options) []Result {
	if c == nil || len(c.Docs) == 0 {
		return nil
	}
	if opt.Limit <= 0 {
		opt.Limit = 60
	}
	orig := ParseQuery(q)
	if len(orig) == 0 {
		return nil
	}

	type term struct {
		text  string
		label string
		w     float32
	}
	terms := make([]term, 0, 32)
	seen := make(map[string]float32, 32)
	add := func(text, label string, w float32) {
		if text == "" {
			return
		}
		if old, ok := seen[text]; ok {
			if w > old {
				seen[text] = w
				for i := range terms {
					if terms[i].text == text {
						terms[i].w = w
						terms[i].label = label
					}
				}
			}
			return
		}
		seen[text] = w
		terms = append(terms, term{text: text, label: label, w: w})
	}

	for _, w := range orig {
		add(w.text, w.label, 1)
		for _, syn := range Synonyms(w.text) {
			add(syn, syn, 0.6)
		}
	}
	for _, raw := range opt.Extra {
		for _, w := range index.Tokenize(raw) {
			add(w, w, 0.45)
		}
	}
	// fuzzy fallback only for words nobody has ever seen
	for _, w := range orig {
		if len(terms) >= 40 {
			break
		}
		if len(c.post[w.text]) > 0 {
			continue
		}
		for _, alt := range c.fuzzy(w.text, 6) {
			add(alt, alt, 0.5)
		}
	}
	if len(terms) == 0 {
		return nil
	}

	const (
		k1 = float32(1.4)
		b  = float32(0.72)
	)
	nDocs := float32(len(c.Docs))
	scores := make(map[int32]float32, 256)
	for _, t := range terms {
		ps := c.post[t.text]
		if len(ps) == 0 {
			continue
		}
		df := float32(len(ps))
		idf := float32(math.Log(1 + float64((nDocs-df+0.5)/(df+0.5))))
		for _, p := range ps {
			dl := c.docLen[p.doc]
			den := p.w + k1*(1-b+b*dl/c.avgLen)
			scores[p.doc] += t.w * idf * (p.w * (k1 + 1)) / den
		}
	}
	if len(scores) == 0 {
		return nil
	}

	type cand struct {
		id    int32
		score float32
	}
	cands := make([]cand, 0, len(scores))
	for id, sc := range scores {
		cands = append(cands, cand{id: id, score: sc})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score == cands[j].score {
			return c.Docs[cands[i].id].Path < c.Docs[cands[j].id].Path
		}
		return cands[i].score > cands[j].score
	})
	if len(cands) > 400 {
		cands = cands[:400]
	}

	joined := strings.ReplaceAll(strings.Join(rawWords(q), " "), " ", "")
	out := make([]Result, 0, opt.Limit)
	for _, cd := range cands {
		if len(out) >= opt.Limit*2 {
			break
		}
		d := &c.Docs[cd.id]
		if opt.Kind != "" && d.Kind != opt.Kind {
			continue
		}
		nameLower := strings.ToLower(d.Name)
		tokenSet := make(map[string]struct{}, len(d.Tokens)+len(d.NameTokens))
		for _, t := range d.Tokens {
			tokenSet[t] = struct{}{}
		}
		for _, t := range d.NameTokens {
			tokenSet[t] = struct{}{}
		}
		matched := 0
		for _, o := range orig {
			if _, ok := tokenSet[o.text]; ok {
				matched++
			}
		}
		coverage := float32(matched) / float32(len(orig))

		bonus := float32(0)
		for _, o := range orig {
			if strings.Contains(nameLower, o.text) {
				bonus++
			}
		}
		if len(joined) >= 3 && strings.Contains(strings.ReplaceAll(nameLower, " ", ""), joined) {
			bonus += 5
		}

		var why []string
		for _, t := range terms {
			if len(why) >= 4 {
				break
			}
			if strings.Contains(nameLower, t.text) ||
				(d.Caption != "" && strings.Contains(strings.ToLower(d.Caption), t.text)) {
				why = append(why, t.label)
			}
		}

		out = append(out, Result{
			Path:    d.Path,
			Name:    d.Name,
			Dir:     filepath.Dir(d.Path),
			Kind:    d.Kind,
			Size:    d.Size,
			Mod:     d.Mod,
			Score:   cd.score * (0.55 + 0.55*coverage) * (1 + 0.15*bonus),
			Matched: why,
			Snippet: d.Snippet,
			Caption: d.Caption,
			Width:   d.Width,
			Height:  d.Height,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Path < out[j].Path
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > opt.Limit {
		out = out[:opt.Limit]
	}
	return out
}

// qword is a stemmed query word plus the spelling the user typed.
type qword struct {
	text  string
	label string
}

// ParseQuery is exported for tests and for the "did you mean" line later.
func ParseQuery(q string) []qword {
	words := rawWords(q)
	out := make([]qword, 0, len(words))
	seen := make(map[string]struct{}, len(words))
	for _, w := range words {
		if utf8.RuneCountInString(w) < 2 || index.IsStopword(w) {
			continue
		}
		st := index.Stem(w)
		if _, dup := seen[st]; dup {
			continue
		}
		seen[st] = struct{}{}
		out = append(out, qword{text: st, label: w})
		if len(out) >= 10 {
			break
		}
	}
	return out
}

func rawWords(q string) []string {
	words := index.Normalize(q)
	out := words[:0]
	for _, w := range words {
		if len(w) >= 1 && !index.IsStopword(w) {
			out = append(out, w)
		}
	}
	return out
}
