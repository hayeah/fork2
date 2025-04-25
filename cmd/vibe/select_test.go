package main

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// shared fixtures & helpers
// -----------------------------------------------------------------------------

var paths = []string{
	"src/foo.go",
	"src/foo_test.go",
	"docs/bar.md",
	"internal/baz_test.go",
	"internal/baz.go",
	"README.md",
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	assert := assert.New(t)
	sort.Strings(got)
	sort.Strings(want)
	assert.ElementsMatch(want, got)
}

// Note: FuzzyMatcher and NegationMatcher tests have been removed as negation is now handled by fzf.NewMatcher

// -----------------------------------------------------------------------------
// ExactPathMatcher
// -----------------------------------------------------------------------------

func TestExactPathMatcher(t *testing.T) {
	// tmp-dir test is special – run once then fall through to table cases
	t.Run("directory match", func(t *testing.T) {
		tmp, err := os.MkdirTemp("", "vibe-test-*")
		if err != nil {
			t.Skip("tmpdir unavailable")
		}
		defer os.RemoveAll(tmp)
		files := []string{tmp + "/a.txt", tmp + "/b.go", tmp + "/sub/c.md"}
		_ = os.MkdirAll(tmp+"/sub", 0o755)
		for _, f := range files {
			_ = os.WriteFile(f, nil, 0o644)
		}
		all := append(files, "x/other.txt")
		m := ExactPathMatcher{FileSelection{Path: tmp}}
		got, _ := m.Match(all)
		eq(t, got, files)
	})

	cases := []struct {
		name string
		sel  FileSelection
		want []string
	}{
		{"file", FileSelection{Path: "src/foo.go"}, []string{"src/foo.go"}},
		{"no-hit", FileSelection{Path: "xxx"}, nil},
		{"line-range ignored", FileSelection{
			Path: "src/foo.go", Ranges: []LineRange{{Start: 1, End: 3}},
		}, []string{"src/foo.go"}},
		{"missing dir", FileSelection{Path: "/no/dir"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := ExactPathMatcher{tc.sel}
			got, _ := m.Match(paths)
			eq(t, got, tc.want)
		})
	}
}

// -----------------------------------------------------------------------------
// CompoundMatcher (logical AND)
// -----------------------------------------------------------------------------

func TestCompoundMatcher(t *testing.T) {
	cases := []struct {
		name  string
		match Matcher
		want  []string
	}{
		{
			"foo AND .go",
			must(ParseMatcher("foo | .go")),
			[]string{"src/foo.go", "src/foo_test.go"},
		},
		{
			"foo AND !test",
			must(ParseMatcher("foo | !_test.go")),
			[]string{"src/foo.go"},
		},
		{
			"no overlap",
			must(ParseMatcher("docs | .md")),
			[]string{"docs/bar.md"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.match
			got, _ := m.Match(paths)
			eq(t, got, tc.want)
		})
	}
}

// -----------------------------------------------------------------------------
// UnionMatcher (logical OR)
// -----------------------------------------------------------------------------

func TestUnionMatcher(t *testing.T) {
	cases := []struct {
		name  string
		match Matcher
		want  []string
	}{
		{
			"foo OR bar",
			must(ParseMatcher("foo;bar")),
			[]string{"src/foo.go", "src/foo_test.go", "docs/bar.md"},
		},
		{
			".go OR .md",
			must(ParseMatcher(".go;.md")),
			[]string{"src/foo.go", "src/foo_test.go", "docs/bar.md", "internal/baz_test.go", "internal/baz.go", "README.md"},
		},
		{
			"internal OR docs",
			must(ParseMatcher("internal;docs")),
			[]string{"docs/bar.md", "internal/baz_test.go", "internal/baz.go"},
		},
		{
			"empty union part",
			must(ParseMatcher("foo;")),
			[]string{"src/foo.go", "src/foo_test.go"},
		},
		{
			"complex union with compound",
			must(ParseMatcher("foo | .go;docs | .md")),
			[]string{"src/foo.go", "src/foo_test.go", "docs/bar.md"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.match
			got, _ := m.Match(paths)
			eq(t, got, tc.want)
		})
	}
}

// -----------------------------------------------------------------------------
// util
// -----------------------------------------------------------------------------

// must unwraps matcher creation for brevity in table tests.
func must(m Matcher, err error) Matcher {
	if err != nil {
		panic(err)
	}
	return m
}
