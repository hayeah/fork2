// cmd/vibe/advanced_matcher.go
package fzf

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// Matcher deterministically filters paths by multi-term rules.
type Matcher struct {
	terms []advTerm
}

type advTerm struct {
	raw        string
	text       string // lower-cased core text
	anchorHead bool   // ^foo
	anchorTail bool   // foo$
	wordPrefix bool   // 'foo
	wordExact  bool   // 'foo'
}

func NewMatcher(pattern string) (Matcher, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return Matcher{}, nil // match everything
	}
	parts := strings.Fields(pattern)
	terms := make([]advTerm, 0, len(parts))

	for _, p := range parts {
		t := advTerm{raw: p}

		// --- word-boundary quote handling first ---------------------------------
		if strings.HasPrefix(p, "'") {
			p = p[1:]
			if len(p) == 0 {
				return Matcher{}, fmt.Errorf("empty term after leading quote in %q", t.raw)
			}
			if strings.HasSuffix(p, "'") {
				t.wordExact = true // both boundaries
				p = p[:len(p)-1]
				if p == "" {
					return Matcher{}, fmt.Errorf("empty term in %q", t.raw)
				}
			} else {
				t.wordPrefix = true // leading boundary only
			}
		}

		// --- ^ / $ path anchors ---------------------------------------------------
		if strings.HasPrefix(p, "^") {
			t.anchorHead = true
			p = p[1:]
		}
		if strings.HasSuffix(p, "$") {
			t.anchorTail = true
			p = p[:len(p)-1]
		}
		if p == "" {
			return Matcher{}, fmt.Errorf("empty term after stripping modifiers in %q", t.raw)
		}

		// normalise for case-insensitive, platform-independent comparison
		t.text = strings.ToLower(filepath.ToSlash(p))
		terms = append(terms, t)
	}
	return Matcher{terms: terms}, nil
}

// Match implements Matcher.
func (m Matcher) Match(paths []string) ([]string, error) {
	if len(m.terms) == 0 {
		return paths, nil
	}
	var out []string
NextPath:
	for _, path := range paths {
		normal := strings.ToLower(filepath.ToSlash(path))

		for _, term := range m.terms {
			if !termMatches(term, normal) {
				continue NextPath
			}
		}
		out = append(out, path) // all terms satisfied
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------
func termMatches(t advTerm, path string) bool {
	// --- fast path: ^foo$, no boundary mods ------------------------------------
	if t.anchorHead && t.anchorTail && !(t.wordExact || t.wordPrefix) {
		return path == t.text
	}

	// compute the slice we need to inspect according to ^ / $
	sub := path
	if t.anchorHead {
		if !strings.HasPrefix(path, t.text) {
			return false
		}
		sub = path[:len(t.text)] // exact prefix region
	}
	if t.anchorTail {
		if !strings.HasSuffix(path, t.text) {
			return false
		}
		sub = path[len(path)-len(t.text):] // exact suffix region
	}

	// --- word-boundary checks --------------------------------------------------
	switch {
	case t.wordExact:
		return containsWordExact(sub, t.text)
	case t.wordPrefix:
		return containsWordPrefix(sub, t.text)
	default:
		// simple substring test (already handled ^/$ prefixes above)
		return strings.Contains(sub, t.text)
	}
}

// wordExact: match whole word (both sides are word boundaries).
// containsWordExact reports whether `needle` appears in `s` delimited on both
// sides by a word boundary (start/end of string, or non-word rune).
func containsWordExact(s, needle string) bool {
	if needle == "" {
		return false
	}

	for start := 0; start <= len(s)-len(needle); {
		rel := strings.Index(s[start:], needle)
		if rel < 0 {
			break
		}
		idx := start + rel // absolute index of this match

		if hasWordBoundary(s, idx, len(needle)) {
			return true
		}
		start = idx + 1 // move one byte forward and keep searching
	}
	return false
}

// hasWordBoundary checks both sides of s[idx : idx+size] for boundaries.
func hasWordBoundary(s string, idx, size int) bool {
	leftOK := idx == 0 || !isWordChar(rune(s[idx-1]))
	rightOK := idx+size == len(s) || !isWordChar(rune(s[idx+size]))
	return leftOK && rightOK
}

// wordPrefix: only left side boundary must hold.
func containsWordPrefix(s, needle string) bool {
	if needle == "" {
		return false
	}

	for start := 0; start <= len(s)-len(needle); {
		rel := strings.Index(s[start:], needle)
		if rel < 0 {
			break
		}
		idx := start + rel

		// only left-hand word boundary must hold
		if idx == 0 || !isWordChar(rune(s[idx-1])) {
			return true
		}
		start = idx + 1
	}
	return false
}

// crude word-char definition: Unicode letter or digit or underscore.
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
