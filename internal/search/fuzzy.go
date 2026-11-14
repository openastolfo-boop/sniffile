package search

import (
	"strings"
	"unicode/utf8"
)

// fuzzy finds vocabulary words one edit away from tok. This is the "мопс" vs
// "мопсик" safety net, not a full spell checker.
func (c *Corpus) fuzzy(tok string, max int) []string {
	if utf8.RuneCountInString(tok) < 4 {
		return nil
	}
	runes := []rune(tok)
	bucket := c.vocab[runes[0]]
	out := make([]string, 0, max)
	for _, v := range bucket {
		if v == tok {
			continue
		}
		if abs(utf8.RuneCountInString(v)-len(runes)) > 1 {
			continue
		}
		if editDistanceAtMost1(tok, v) {
			out = append(out, v)
			if len(out) >= max {
				break
			}
		}
	}
	// "landscap" should still find "landscape" even if no synonym exists
	if len(out) < max && len(runes) >= 4 {
		for _, v := range bucket {
			if v == tok || !strings.HasPrefix(v, tok) {
				continue
			}
			out = append(out, v)
			if len(out) >= max {
				break
			}
		}
	}
	return out
}

// editDistanceAtMost1 checks insert/delete/substitute without building a full
// matrix. Returns false for transpositions — good enough here.
func editDistanceAtMost1(a, b string) bool {
	if a == b {
		return true
	}
	ar, br := []rune(a), []rune(b)
	if len(ar) > len(br) {
		ar, br = br, ar
	}
	la, lb := len(ar), len(br)
	if lb-la > 1 {
		return false
	}
	i := 0
	for i < la && ar[i] == br[i] {
		i++
	}
	if i == la {
		return true // a is a prefix of b, one insertion
	}
	if lb > la {
		for j := i; j < la; j++ {
			if ar[j] != br[j+1] {
				return false
			}
		}
		return true
	}
	// same length, one substitution
	for j := i + 1; j < la; j++ {
		if ar[j] != br[j] {
			return false
		}
	}
	return true
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
