package selection_test

import (
	selectionPkg "github.com/hayeah/fork2/internal/selection"
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

// -----------------------------------------------------------------------------
// UnionMatcher (logical OR)
// -----------------------------------------------------------------------------

func TestUnionMatcher(t *testing.T) {
	cases := []struct {
		name  string
		match selectionPkg.Matcher
		want  []string
	}{
		{
			"foo OR bar",
			must(selectionPkg.ParseMatcher("foo;bar")),
			[]string{"src/foo.go", "src/foo_test.go", "docs/bar.md"},
		},
		{
			".go OR .md",
			must(selectionPkg.ParseMatcher(".go;.md")),
			[]string{"src/foo.go", "src/foo_test.go", "docs/bar.md", "internal/baz_test.go", "internal/baz.go", "README.md"},
		},
		{
			"internal OR docs",
			must(selectionPkg.ParseMatcher("internal;docs")),
			[]string{"docs/bar.md", "internal/baz_test.go", "internal/baz.go"},
		},
		{
			"empty union part",
			must(selectionPkg.ParseMatcher("foo;")),
			[]string{"src/foo.go", "src/foo_test.go"},
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
func must(m selectionPkg.Matcher, err error) selectionPkg.Matcher {
	if err != nil {
		panic(err)
	}
	return m
}

func TestParseMatcherDotSlashAnchors(t *testing.T) {
	paths := []string{
		"render/foo.go",
		"cmd/render/bar.go",
	}
	m, err := selectionPkg.ParseMatcher("./render")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := m.Match(paths)
	eq(t, got, []string{"render/foo.go"})
}

// -----------------------------------------------------------------------------
// CompoundMatcher (logical AND with |)
// -----------------------------------------------------------------------------

func TestCompoundMatcher(t *testing.T) {
	// Test paths that include test files
	testPaths := []string{
		"cmd/vibe/clipboard.go",
		"cmd/vibe/content_loader.go",
		"cmd/vibe/directory_tree.go",
		"cmd/vibe/directory_tree_test.go",
		"cmd/vibe/filemap.go",
		"cmd/vibe/filemap_test.go",
		"cmd/vibe/main.go",
		"cmd/vibe/out_test.go",
		"render/contentloader.go",
		"render/contentloader_test.go",
		"render/frontmatter.go",
		"render/frontmatter_test.go",
		"render/render.go",
		"render/render_test.go",
	}

	cases := []struct {
		name  string
		match selectionPkg.Matcher
		want  []string
	}{
		{
			"cmd/vibe .go | !test - should exclude test files from cmd/vibe",
			must(selectionPkg.ParseMatcher("cmd/vibe .go | !test")),
			[]string{
				"cmd/vibe/clipboard.go",
				"cmd/vibe/content_loader.go",
				"cmd/vibe/directory_tree.go",
				"cmd/vibe/filemap.go",
				"cmd/vibe/main.go",
			},
		},
		{
			"render .go | !test - should exclude test files from render",
			must(selectionPkg.ParseMatcher("render .go | !test")),
			[]string{
				"render/contentloader.go",
				"render/frontmatter.go",
				"render/render.go",
			},
		},
		{
			"union with compound: (cmd/vibe .go;render .go) | !test",
			must(selectionPkg.ParseMatcher("cmd/vibe .go;render .go | !test")),
			[]string{
				"cmd/vibe/clipboard.go",
				"cmd/vibe/content_loader.go",
				"cmd/vibe/directory_tree.go",
				"cmd/vibe/filemap.go",
				"cmd/vibe/main.go",
				"render/contentloader.go",
				"render/frontmatter.go",
				"render/render.go",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.match
			got, err := m.Match(testPaths)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			eq(t, got, tc.want)
		})
	}
}
