package main

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectPattern(t *testing.T) {
	assert := assert.New(t)

	paths := []string{
		"abc/foo",
		"def/bar",
		"qux",
	}

	// Test empty pattern (select all)
	selected, err := selectSinglePattern(paths, "")
	assert.NoError(err)
	assert.Equal(paths, selected, "Empty pattern should select all paths")

	// Test fuzzy pattern
	fuzzySelected, err := selectSinglePattern(paths, "fo")
	assert.NoError(err)
	assert.Len(fuzzySelected, 1, "Should match only one file with 'fo'")
	assert.Equal("abc/foo", fuzzySelected[0], "Should match 'abc/foo'")

	// Test regex pattern with leading slash
	regexSelected, err := selectSinglePattern(paths, "/^[a-z]{3}/")
	assert.NoError(err)
	assert.Len(regexSelected, 2, "Regex should match two paths with 3-letter dirs")
	assert.Contains(regexSelected, "abc/foo", "Should match 'abc/foo'")
	assert.Contains(regexSelected, "def/bar", "Should match 'def/bar'")

	// Test regex pattern with invalid regex
	_, err = selectSinglePattern(paths, "/[invalid regex")
	assert.Error(err, "Should return error for invalid regex")
	assert.Contains(err.Error(), "invalid regex pattern", "Error message should mention invalid regex")
}

func TestSelectByPatterns(t *testing.T) {
	assert := assert.New(t)

	paths := []string{
		"src/foo.go",
		"src/foo_test.go",
		"docs/bar.md",
		"internal/baz_test.go",
		"internal/baz.go",
		"README.md",
	}

	t.Run("NoPatterns", func(t *testing.T) {
		result, err := selectByPatterns(paths, []string{})
		assert.NoError(err)
		// Empty result set with no patterns
		assert.Empty(result, "No patterns should result in an empty set")
	})

	t.Run("SinglePositiveFuzzy", func(t *testing.T) {
		result, err := selectByPatterns(paths, []string{"baz"})
		assert.NoError(err)
		assert.ElementsMatch([]string{"internal/baz.go", "internal/baz_test.go"}, result)
	})

	t.Run("SingleRegex", func(t *testing.T) {
		result, err := selectByPatterns(paths, []string{"/\\.md$"})
		assert.NoError(err)
		assert.ElementsMatch([]string{"docs/bar.md", "README.md"}, result)
	})

	t.Run("NegateTests", func(t *testing.T) {
		result, err := selectByPatterns(paths, []string{"!_test.go"})
		assert.NoError(err)
		// This should remove anything matching _test.go
		assert.ElementsMatch([]string{"src/foo.go", "docs/bar.md", "internal/baz.go", "README.md"}, result)
	})

	t.Run("MultiplePatterns", func(t *testing.T) {
		// Collect both .go files and .md files (union of matches)
		result, err := selectByPatterns(paths, []string{"/\\.go$", "/\\.md$"})
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{
				"src/foo.go",
				"src/foo_test.go",
				"internal/baz_test.go",
				"internal/baz.go",
				"docs/bar.md",
				"README.md",
			},
			result,
			"Should include all .go and .md files",
		)
	})

	t.Run("InvalidRegex", func(t *testing.T) {
		_, err := selectByPatterns(paths, []string{"/[invalid"})
		assert.Error(err)
		assert.Contains(err.Error(), "invalid regex pattern")
	})
}

func TestSelectPatternWithRelativePaths(t *testing.T) {
	assert := assert.New(t)

	paths := []string{
		"src/foo.go",
		"src/bar.go",
		"docs/readme.md",
	}

	// Test with "./" prefix
	dotSlashSelected, err := selectSinglePattern(paths, "./foo")
	assert.NoError(err)
	assert.Len(dotSlashSelected, 1, "Should match one file with './foo'")
	assert.Equal("src/foo.go", dotSlashSelected[0], "Should match 'src/foo.go'")

	// Test with "../" prefix (should be rejected)
	_, err = selectSinglePattern(paths, "../foo")
	assert.Error(err, "Should reject patterns with '../'")
	assert.Contains(err.Error(), "not supported for security reasons", "Error should mention security reasons")
}

func TestSelectPatternWithFilteringOperator(t *testing.T) {
	assert := assert.New(t)

	paths := []string{
		"cmd/testclip/main.go",
		"cmd/vibe/directory_tree.go",
		"cmd/vibe/directory_tree_test.go",
		"cmd/vibe/doc.md",
		"cmd/vibe/main.go",
		"internal/utils.go",
		"internal/utils_test.go",
	}
	sort.Strings(paths)

	var results []string
	var err error

	// Test filtering operator with cmd and .go files (logical AND)
	results, err = selectSinglePattern(paths, "cmd|.go")

	assert.NoError(err)
	sort.Strings(results)
	assert.Equal(results, []string{
		"cmd/testclip/main.go",
		"cmd/vibe/directory_tree.go",
		"cmd/vibe/directory_tree_test.go",
		"cmd/vibe/main.go",
	})

	// Test filtering operator with negation pattern (exclude test files)
	results, err = selectSinglePattern(paths, "cmd.go|!_test.go")
	assert.NoError(err)
	sort.Strings(results)
	assert.Equal([]string{
		"cmd/testclip/main.go",
		"cmd/vibe/directory_tree.go",
		"cmd/vibe/main.go",
	}, results)

	// Test filtering operator (include only test files)
	results, err = selectSinglePattern(paths, "cmd.go|_test.go")
	assert.NoError(err)
	sort.Strings(results)
	assert.Equal([]string{
		"cmd/vibe/directory_tree_test.go",
	}, results)

	// Test with regex patterns
	cmdRegex, err := selectSinglePattern(paths, "cmd|/tree")
	assert.NoError(err)
	assert.Len(cmdRegex, 2, "Should match cmd files containing 'tree'")
	assert.Contains(cmdRegex, "cmd/vibe/directory_tree.go")
	assert.Contains(cmdRegex, "cmd/vibe/directory_tree_test.go")
}
