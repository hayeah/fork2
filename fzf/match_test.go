// fzf/match_test.go
package fzf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var samplePaths = []string{
	"cmd/vibe/select.go",
	"cmd/vibe/ask.go",
	"cmd/vibe/directory_tree.go",
	"internal/assert/assert.go",
	"pkg/utils/helper_test.go",
	"docs/intro.txt",
	"README.md",
	"main.go",
}

func TestAdvancedMatcher(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		name     string
		pattern  string
		expected []string
		hasErr   bool
	}{
		{
			name:     "empty pattern – returns all",
			pattern:  "",
			expected: samplePaths,
		},
		{
			name:     "simple substring",
			pattern:  "cmd",
			expected: []string{"cmd/vibe/select.go", "cmd/vibe/ask.go", "cmd/vibe/directory_tree.go"},
		},
		{
			name:     "multiple terms – implicit AND",
			pattern:  "cmd .go",
			expected: []string{"cmd/vibe/select.go", "cmd/vibe/ask.go", "cmd/vibe/directory_tree.go"},
		},
		{
			name:     "head anchor",
			pattern:  "^cmd",
			expected: []string{"cmd/vibe/select.go", "cmd/vibe/ask.go", "cmd/vibe/directory_tree.go"},
		},
		{
			name:     "tail anchor",
			pattern:  ".go$",
			expected: []string{"cmd/vibe/select.go", "cmd/vibe/ask.go", "cmd/vibe/directory_tree.go", "internal/assert/assert.go", "pkg/utils/helper_test.go", "main.go"},
		},
		{
			name:     "exact head+tail (whole path)",
			pattern:  "^README.md$",
			expected: []string{"README.md"},
		},
		{
			name:     "word-prefix boundary",
			pattern:  "'select",
			expected: []string{"cmd/vibe/select.go"},
		},
		{
			name:     "word-exact boundary",
			pattern:  "'select'",
			expected: []string{"cmd/vibe/select.go"},
		},
		{
			name:     "case-insensitive",
			pattern:  "readme",
			expected: []string{"README.md"},
		},
		{
			name:    "ill-formed (lonely quote)",
			pattern: "'",
			hasErr:  true,
		},
		{
			name:    "ill-formed (anchor only)",
			pattern: "^",
			hasErr:  true,
		},
		{
			name:     "negation - exclude select",
			pattern:  "!select",
			expected: []string{"cmd/vibe/ask.go", "cmd/vibe/directory_tree.go", "internal/assert/assert.go", "pkg/utils/helper_test.go", "docs/intro.txt", "README.md", "main.go"},
		},
		{
			name:     "negation with multiple terms",
			pattern:  "cmd !_test.go",
			expected: []string{"cmd/vibe/select.go", "cmd/vibe/ask.go", "cmd/vibe/directory_tree.go"},
		},
		{
			name:     "negation with word boundary",
			pattern:  "!'test'",
			expected: []string{"cmd/vibe/select.go", "cmd/vibe/ask.go", "cmd/vibe/directory_tree.go", "internal/assert/assert.go", "docs/intro.txt", "README.md", "main.go"},
		},
		{
			name:    "ill-formed (empty negation)",
			pattern: "!",
			hasErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := NewMatcher(tc.pattern)
			if tc.hasErr {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}
			got, err := m.Match(samplePaths)
			assert.NoError(err)
			assert.Equal(tc.expected, got)
		})
	}
}

func TestContainsWordExact(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		needle string
		want   bool
	}{
		{"empty needle", "test", "", false},
		{"empty string", "", "test", false},
		{"exact match with word boundaries", "hello world", "world", true},
		{"match at beginning", "test string", "test", true},
		{"match at end", "a test", "test", true},
		{"no word boundaries", "unselected", "select", false},
		{"left boundary only", "test-string", "string", true},
		{"right boundary only", "pre-test", "test", true},
		{"multiple occurrences, first match", "test in a test", "test", true},
		{"multiple occurrences, second match", "unselect test", "test", true},
		{"underscore is word char", "pre_test", "test", false},
		{"digit is word char", "pre2test", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsWordExact(tt.s, tt.needle)
			assert.Equal(t, tt.want, got, "containsWordExact(%q, %q)", tt.s, tt.needle)
		})
	}
}

func TestHasWordBoundary(t *testing.T) {
	tests := []struct {
		name string
		s    string
		idx  int
		size int
		want bool
	}{
		{"start of string", "test", 0, 4, true},
		{"end of string", "a test", 2, 4, true},
		{"both boundaries", "hello world", 6, 5, true},
		{"no boundaries", "unselected", 2, 6, false},
		{"left boundary only", "test-string", 5, 6, true},
		{"right boundary only", "pre-test", 0, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasWordBoundary(tt.s, tt.idx, tt.size)
			assert.Equal(t, tt.want, got, "hasWordBoundary(%q, %d, %d)", tt.s, tt.idx, tt.size)
		})
	}
}
