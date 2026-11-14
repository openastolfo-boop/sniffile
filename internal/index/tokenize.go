// Package index turns a folder into a small flat search index.
//
// Nothing clever happens here on purpose: we walk the tree, pull words out of
// names, paths and (optionally) text files, and keep them next to the file.
// The ranking lives in ../search.
package index

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// A deliberately short stop list. Long lists quietly eat useful queries like
// "the office" or "игра престолов".
var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {},
	"but": {}, "by": {}, "for": {}, "from": {}, "in": {}, "is": {}, "it": {},
	"of": {}, "on": {}, "or": {}, "that": {}, "the": {}, "to": {}, "was": {},
	"with": {},
	"и":    {}, "в": {}, "во": {}, "на": {}, "с": {}, "со": {}, "к": {}, "ко": {},
	"по": {}, "из": {}, "от": {}, "до": {}, "за": {}, "для": {}, "не": {},
	"ни": {}, "но": {}, "или": {}, "у": {}, "о": {}, "об": {}, "про": {},
	"это": {}, "как": {}, "так": {}, "же": {}, "бы": {}, "ли": {},
}

// IsStopword is exported because the query parser needs the same opinion.
func IsStopword(w string) bool {
	_, ok := stopwords[w]
	return ok
}

// Fold lowercases and irons out ё/е so the two spellings meet in the middle.
func Fold(w string) string {
	w = strings.ToLower(w)
	return strings.ReplaceAll(w, "ё", "е")
}

// Normalize splits text into raw lowercase words. No stemming, no filtering.
func Normalize(s string) []string {
	// camelCase -> camel case
	var sb strings.Builder
	var prev rune
	for _, r := range s {
		if unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
			sb.WriteByte(' ')
		}
		sb.WriteRune(r)
		prev = r
	}
	words := strings.FieldsFunc(sb.String(), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for i, w := range words {
		words[i] = Fold(w)
	}
	return words
}

// Tokenize is Normalize + stopword removal + a very light stem.
func Tokenize(s string) []string {
	words := Normalize(s)
	out := make([]string, 0, len(words))
	for _, w := range words {
		if utf8.RuneCountInString(w) < 2 || IsStopword(w) {
			continue
		}
		out = append(out, Stem(w))
	}
	return out
}

// Stem is intentionally crude. It only has to be consistent between the
// document and the query, not linguistically correct.
func Stem(w string) string {
	if w == "" {
		return w
	}
	runes := []rune(w)
	if !hasCyrillic(runes) {
		if containsDigit(w) {
			return w
		}
		switch {
		case len(w) > 4 && strings.HasSuffix(w, "ies"):
			return w[:len(w)-3] + "y"
		case len(w) > 4 && strings.HasSuffix(w, "sses"):
			return w[:len(w)-2]
		case len(w) > 3 && strings.HasSuffix(w, "s") &&
			!strings.HasSuffix(w, "ss") &&
			!strings.HasSuffix(w, "us") &&
			!strings.HasSuffix(w, "is"):
			return w[:len(w)-1]
		}
		return w
	}
	if containsDigit(w) {
		return w
	}
	for _, suf := range ruSuffixes {
		sr := []rune(suf)
		if len(runes)-len(sr) < 3 {
			continue
		}
		if hasSuffixRunes(runes, sr) {
			return string(runes[:len(runes)-len(sr)])
		}
	}
	return w
}

// longest suffixes first; order inside a length group does not matter
var ruSuffixes = []string{
	"ами", "ями", "ыми", "ими", "ого", "его", "ому", "ему",
	"ая", "яя", "ое", "ее", "ые", "ие", "ый", "ий", "ой", "ей", "ым", "им",
	"ов", "ев", "ах", "ях", "ам", "ям", "ом", "ем", "ую", "юю", "ия", "ии", "ью",
	"а", "я", "ы", "и", "е", "о", "у", "ю", "ь",
}

func hasCyrillic(rs []rune) bool {
	for _, r := range rs {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

func hasSuffixRunes(s, suf []rune) bool {
	if len(suf) > len(s) {
		return false
	}
	for i := 1; i <= len(suf); i++ {
		if s[len(s)-i] != suf[len(suf)-i] {
			return false
		}
	}
	return true
}

func containsDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
