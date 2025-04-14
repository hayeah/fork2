package main

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Common test paths used across multiple tests
var testPaths = []string{
	"src/foo.go",
	"src/foo_test.go",
	"docs/bar.md",
	"internal/baz_test.go",
	"internal/baz.go",
	"README.md",
}

// TestFuzzyMatcher tests the FuzzyMatcher implementation
func TestFuzzyMatcher(t *testing.T) {
	assert := assert.New(t)

	t.Run("EmptyPattern", func(t *testing.T) {
		matcher := FuzzyMatcher{Pattern: ""}
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.Equal(testPaths, results, "Empty pattern should select all paths")
	})

	t.Run("SingleMatch", func(t *testing.T) {
		matcher := FuzzyMatcher{Pattern: "foo"}
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.ElementsMatch([]string{"src/foo.go", "src/foo_test.go"}, results)
	})

	t.Run("NoMatches", func(t *testing.T) {
		matcher := FuzzyMatcher{Pattern: "nonexistent"}
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.Empty(results, "Non-matching pattern should return empty results")
	})

	t.Run("PartialMatch", func(t *testing.T) {
		matcher := FuzzyMatcher{Pattern: "ba"}
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.ElementsMatch([]string{"docs/bar.md", "internal/baz_test.go", "internal/baz.go"}, results)
	})
}

// TestRegexMatcher tests the RegexMatcher implementation
func TestRegexMatcher(t *testing.T) {
	assert := assert.New(t)

	t.Run("EmptyPattern", func(t *testing.T) {
		matcher, err := NewRegexMatcher("")
		assert.NoError(err)
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.Equal(testPaths, results, "Empty regex pattern should select all paths")
	})

	t.Run("ValidPattern", func(t *testing.T) {
		matcher, err := NewRegexMatcher("\\.md$")
		assert.NoError(err)
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.ElementsMatch([]string{"docs/bar.md", "README.md"}, results)
	})

	t.Run("InvalidPattern", func(t *testing.T) {
		_, err := NewRegexMatcher("[invalid")
		assert.Error(err, "Should return error for invalid regex")
		assert.Contains(err.Error(), "invalid regex pattern")
	})

	t.Run("NoMatches", func(t *testing.T) {
		matcher, err := NewRegexMatcher("\\.py$")
		assert.NoError(err)
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.Empty(results, "Non-matching regex should return empty results")
	})
}

// TestNegationMatcher tests the NegationMatcher implementation
func TestNegationMatcher(t *testing.T) {
	assert := assert.New(t)

	t.Run("NegateRegex", func(t *testing.T) {
		// Create a regex matcher for test files
		regexMatcher, err := NewRegexMatcher("_test\\.go$")
		assert.NoError(err)

		// Wrap it with a negation matcher
		negationMatcher := NegationMatcher{Wrapped: regexMatcher}

		results, err := negationMatcher.Match(testPaths)
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"src/foo.go", "docs/bar.md", "internal/baz.go", "README.md"},
			results,
			"Should exclude test files",
		)
	})

	t.Run("NegateFuzzy", func(t *testing.T) {
		// Create a fuzzy matcher for "foo" files
		fuzzyMatcher := FuzzyMatcher{Pattern: "foo"}

		// Wrap it with a negation matcher
		negationMatcher := NegationMatcher{Wrapped: fuzzyMatcher}

		results, err := negationMatcher.Match(testPaths)
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"docs/bar.md", "internal/baz_test.go", "internal/baz.go", "README.md"},
			results,
			"Should exclude foo files",
		)
	})
}

// TestExactPathMatcher tests the ExactPathMatcher implementation
func TestExactPathMatcher(t *testing.T) {
	assert := assert.New(t)

	t.Run("ExactMatch", func(t *testing.T) {
		matcher := ExactPathMatcher{FileSelection{Path: "src/foo.go"}}
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.Equal([]string{"src/foo.go"}, results)
	})

	t.Run("NoMatch", func(t *testing.T) {
		matcher := ExactPathMatcher{FileSelection{Path: "nonexistent.go"}}
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.Empty(results, "Non-matching exact path should return empty results")
	})

	t.Run("WithLineRange", func(t *testing.T) {
		matcher := ExactPathMatcher{FileSelection{Path: "src/foo.go", Ranges: []LineRange{{Start: 10, End: 20}}}}
		results, err := matcher.Match(testPaths)
		assert.NoError(err)
		assert.Equal([]string{"src/foo.go"}, results, "Line range should not affect path matching")
	})
}

// TestCompoundMatcher tests the CompoundMatcher implementation
func TestCompoundMatcher(t *testing.T) {
	assert := assert.New(t)

	t.Run("MultipleMatchers", func(t *testing.T) {
		// First matcher: files containing "foo"
		fuzzyMatcher := FuzzyMatcher{Pattern: "foo"}

		// Second matcher: files with .go extension
		regexMatcher, err := NewRegexMatcher("\\.go$")
		assert.NoError(err)

		// Combine them in a compound matcher (logical AND)
		compoundMatcher := CompoundMatcher{
			Matchers: []Matcher{fuzzyMatcher, regexMatcher},
		}

		results, err := compoundMatcher.Match(testPaths)
		assert.NoError(err)
		assert.ElementsMatch([]string{"src/foo.go", "src/foo_test.go"}, results)
	})

	t.Run("WithNegation", func(t *testing.T) {
		// First matcher: files containing "foo"
		fuzzyMatcher := FuzzyMatcher{Pattern: "foo"}

		// Second matcher: negate files with _test.go
		testFileMatcher, err := NewRegexMatcher("_test\\.go$")
		assert.NoError(err)
		negationMatcher := NegationMatcher{Wrapped: testFileMatcher}

		// Combine them in a compound matcher
		compoundMatcher := CompoundMatcher{
			Matchers: []Matcher{fuzzyMatcher, negationMatcher},
		}

		results, err := compoundMatcher.Match(testPaths)
		assert.NoError(err)
		assert.Equal([]string{"src/foo.go"}, results)
	})

	t.Run("NoMatches", func(t *testing.T) {
		// First matcher: files containing "foo"
		fuzzyMatcher := FuzzyMatcher{Pattern: "foo"}

		// Second matcher: files with .md extension
		regexMatcher, err := NewRegexMatcher("\\.md$")
		assert.NoError(err)

		// These matchers have no overlapping matches
		compoundMatcher := CompoundMatcher{
			Matchers: []Matcher{fuzzyMatcher, regexMatcher},
		}

		results, err := compoundMatcher.Match(testPaths)
		assert.NoError(err)
		assert.Empty(results, "No overlapping matches should return empty results")
	})
}

// TestParseMatchersFromString tests the ParseMatchersFromString function
func TestParseMatchersFromString(t *testing.T) {
	assert := assert.New(t)

	t.Run("EmptyInput", func(t *testing.T) {
		matchers, err := ParseMatchersFromString("")
		assert.NoError(err)
		assert.Empty(matchers, "Empty input should return no matchers")
	})

	t.Run("SinglePattern", func(t *testing.T) {
		input := "foo"
		matchers, err := ParseMatchersFromString(input)
		assert.NoError(err)
		assert.Len(matchers, 1, "Should return one matcher")

		// Verify it's a FuzzyMatcher
		fuzzyMatcher, ok := matchers[0].(FuzzyMatcher)
		assert.True(ok, "Should be a FuzzyMatcher")
		assert.Equal("foo", fuzzyMatcher.Pattern)
	})

	t.Run("MultiplePatterns", func(t *testing.T) {
		input := "foo\nbar"
		matchers, err := ParseMatchersFromString(input)
		assert.NoError(err)
		assert.Len(matchers, 2, "Should return two matchers")

		// Verify they're both FuzzyMatchers with correct patterns
		fuzzyMatcher1, ok := matchers[0].(FuzzyMatcher)
		assert.True(ok, "First matcher should be a FuzzyMatcher")
		assert.Equal("foo", fuzzyMatcher1.Pattern)

		fuzzyMatcher2, ok := matchers[1].(FuzzyMatcher)
		assert.True(ok, "Second matcher should be a FuzzyMatcher")
		assert.Equal("bar", fuzzyMatcher2.Pattern)
	})

	t.Run("SkipEmptyLines", func(t *testing.T) {
		input := "foo\n\n\nbar"
		matchers, err := ParseMatchersFromString(input)
		assert.NoError(err)
		assert.Len(matchers, 2, "Should return two matchers, skipping empty lines")
	})

	t.Run("SkipCommentLines", func(t *testing.T) {
		input := "foo\n# This is a comment\nbar"
		matchers, err := ParseMatchersFromString(input)
		assert.NoError(err)
		assert.Len(matchers, 2, "Should return two matchers, skipping comment lines")
	})

	t.Run("MixedPatternTypes", func(t *testing.T) {
		input := `
			foo
			/\.go$
			=exact/path.txt
			!test
		`
		matchers, err := ParseMatchersFromString(input)
		assert.NoError(err)
		assert.Len(matchers, 4, "Should return four matchers of different types")

		// Verify first matcher is a FuzzyMatcher
		_, ok := matchers[0].(FuzzyMatcher)
		assert.True(ok, "First matcher should be a FuzzyMatcher")

		// Verify second matcher is a RegexMatcher
		_, ok = matchers[1].(*RegexMatcher)
		assert.True(ok, "Second matcher should be a RegexMatcher")

		// Verify third matcher is an ExactPathMatcher
		_, ok = matchers[2].(ExactPathMatcher)
		assert.True(ok, "Third matcher should be an ExactPathMatcher")

		// Verify fourth matcher is a NegationMatcher
		_, ok = matchers[3].(NegationMatcher)
		assert.True(ok, "Fourth matcher should be a NegationMatcher")
	})

	t.Run("ExactPathWithLineRange", func(t *testing.T) {
		input := "=path/to/file.txt#10,20"
		matchers, err := ParseMatchersFromString(input)
		assert.NoError(err)
		assert.Len(matchers, 1, "Should return one matcher")

		// Verify it's an ExactPathMatcher with correct line range
		exactMatcher, ok := matchers[0].(ExactPathMatcher)
		assert.True(ok, "Should be an ExactPathMatcher")
		assert.Equal("path/to/file.txt", exactMatcher.FileSelection.Path)
		assert.Len(exactMatcher.FileSelection.Ranges, 1, "Should have one line range")
		assert.Equal(10, exactMatcher.FileSelection.Ranges[0].Start)
		assert.Equal(20, exactMatcher.FileSelection.Ranges[0].End)
	})

	t.Run("InvalidPattern", func(t *testing.T) {
		input := "foo\n/[invalid\nbar"
		_, err := ParseMatchersFromString(input)
		assert.Error(err, "Should return error for invalid pattern")
		assert.Contains(err.Error(), "error parsing pattern '/[invalid'")
	})
}

// TestParseMatcher tests the ParseMatcher function
func TestParseMatcher(t *testing.T) {
	assert := assert.New(t)

	t.Run("FuzzyPattern", func(t *testing.T) {
		matcher, err := ParseMatcher("foo")
		assert.NoError(err)
		_, ok := matcher.(FuzzyMatcher)
		assert.True(ok, "Should create a FuzzyMatcher for normal pattern")
	})

	t.Run("RegexPattern", func(t *testing.T) {
		matcher, err := ParseMatcher("/\\.go$")
		assert.NoError(err)
		_, ok := matcher.(*RegexMatcher)
		assert.True(ok, "Should create a RegexMatcher for pattern starting with '/'")
	})

	t.Run("NegationPattern", func(t *testing.T) {
		matcher, err := ParseMatcher("!foo")
		assert.NoError(err)
		negMatcher, ok := matcher.(NegationMatcher)
		assert.True(ok, "Should create a NegationMatcher for pattern starting with '!'")
		assert.IsType(negMatcher, NegationMatcher{})
		assert.IsType(negMatcher.Wrapped, FuzzyMatcher{})
	})

	t.Run("CompoundPattern", func(t *testing.T) {
		matcher, err := ParseMatcher("foo|/\\.go$")
		assert.NoError(err)
		compMatcher, ok := matcher.(CompoundMatcher)
		assert.True(ok, "Should create a CompoundMatcher for pattern with '|'")
		assert.Len(compMatcher.Matchers, 2, "Should have two matchers")
	})

	t.Run("ExactPathPattern", func(t *testing.T) {
		matcher, err := ParseMatcher("=src/foo.go")
		assert.NoError(err)
		_, ok := matcher.(ExactPathMatcher)
		assert.True(ok, "Should create an ExactPathMatcher for pattern starting with '='")
	})

	t.Run("InvalidPattern", func(t *testing.T) {
		_, err := ParseMatcher("../foo")
		assert.Error(err, "Should reject patterns with '../'")
		assert.Contains(err.Error(), "not supported for security reasons")
	})

	t.Run("EmptyNegationPattern", func(t *testing.T) {
		_, err := ParseMatcher("!")
		assert.Error(err, "Should reject empty negation pattern")
		assert.Contains(err.Error(), "empty negation pattern '!' is not valid")
	})

	t.Run("InvalidRegexPattern", func(t *testing.T) {
		_, err := ParseMatcher("/[invalid")
		assert.Error(err, "Should return error for invalid regex")
		assert.Contains(err.Error(), "invalid regex pattern")
	})
}

// TestSelectSinglePattern tests the selectSinglePattern function
func TestSelectSinglePattern(t *testing.T) {
	assert := assert.New(t)

	paths := []string{
		"abc/foo",
		"def/bar",
		"qux",
	}

	t.Run("EmptyPattern", func(t *testing.T) {
		selected, err := selectSinglePattern(paths, "")
		assert.NoError(err)
		assert.Equal(paths, selected, "Empty pattern should select all paths")
	})

	t.Run("FuzzyPattern", func(t *testing.T) {
		selected, err := selectSinglePattern(paths, "fo")
		assert.NoError(err)
		assert.Len(selected, 1, "Should match only one file with 'fo'")
		assert.Equal("abc/foo", selected[0], "Should match 'abc/foo'")
	})

	t.Run("RegexPattern", func(t *testing.T) {
		selected, err := selectSinglePattern(paths, "/^[a-z]{3}/")
		assert.NoError(err)
		assert.Len(selected, 2, "Regex should match two paths with 3-letter dirs")
		assert.Contains(selected, "abc/foo", "Should match 'abc/foo'")
		assert.Contains(selected, "def/bar", "Should match 'def/bar'")
	})

	t.Run("InvalidRegex", func(t *testing.T) {
		_, err := selectSinglePattern(paths, "/[invalid regex")
		assert.Error(err, "Should return error for invalid regex")
		assert.Contains(err.Error(), "invalid regex pattern")
	})

	t.Run("RelativePath", func(t *testing.T) {
		selected, err := selectSinglePattern(paths, "./foo")
		assert.NoError(err)
		assert.Len(selected, 1, "Should match one file with './foo'")
		assert.Equal("abc/foo", selected[0], "Should match 'abc/foo'")
	})

	t.Run("UnsupportedPath", func(t *testing.T) {
		_, err := selectSinglePattern(paths, "../foo")
		assert.Error(err, "Should reject patterns with '../'")
		assert.Contains(err.Error(), "not supported for security reasons")
	})
}

// TestSelectByPatterns tests the selectByPatterns function
func TestSelectByPatterns(t *testing.T) {
	assert := assert.New(t)

	t.Run("NoPatterns", func(t *testing.T) {
		result, err := selectByPatterns(testPaths, []string{})
		assert.NoError(err)
		assert.Empty(result, "No patterns should result in an empty set")
	})

	t.Run("SinglePattern", func(t *testing.T) {
		result, err := selectByPatterns(testPaths, []string{"baz"})
		assert.NoError(err)
		assert.ElementsMatch([]string{"internal/baz.go", "internal/baz_test.go"}, result)
	})

	t.Run("MultiplePatterns", func(t *testing.T) {
		result, err := selectByPatterns(testPaths, []string{"/\\.go$", "/\\.md$"})
		assert.NoError(err)
		assert.ElementsMatch(testPaths, result, "Should include all .go and .md files")
	})

	t.Run("InvalidPattern", func(t *testing.T) {
		_, err := selectByPatterns(testPaths, []string{"/[invalid"})
		assert.Error(err)
		assert.Contains(err.Error(), "invalid regex pattern")
	})
}

// TestCompoundPatterns tests compound patterns with the | operator
func TestCompoundPatterns(t *testing.T) {
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

	t.Run("SimpleCompound", func(t *testing.T) {
		results, err := selectSinglePattern(paths, "cmd|.go")
		assert.NoError(err)
		sort.Strings(results)
		assert.ElementsMatch([]string{
			"cmd/testclip/main.go",
			"cmd/vibe/directory_tree.go",
			"cmd/vibe/directory_tree_test.go",
			"cmd/vibe/main.go",
		}, results)
	})

	t.Run("WithNegation", func(t *testing.T) {
		results, err := selectSinglePattern(paths, "cmd.go|!_test.go")
		assert.NoError(err)
		sort.Strings(results)
		assert.ElementsMatch([]string{
			"cmd/testclip/main.go",
			"cmd/vibe/directory_tree.go",
			"cmd/vibe/main.go",
		}, results)
	})

	t.Run("WithRegex", func(t *testing.T) {
		results, err := selectSinglePattern(paths, "cmd|/tree")
		assert.NoError(err)
		sort.Strings(results)
		assert.ElementsMatch([]string{
			"cmd/vibe/directory_tree.go",
			"cmd/vibe/directory_tree_test.go",
		}, results)
	})

	t.Run("ComplexCompound", func(t *testing.T) {
		results, err := selectSinglePattern(paths, "cmd|/vibe|!doc.md")
		assert.NoError(err)
		sort.Strings(results)
		assert.ElementsMatch([]string{
			"cmd/vibe/directory_tree.go",
			"cmd/vibe/directory_tree_test.go",
			"cmd/vibe/main.go",
		}, results)
	})
}

// TestUnionMatcher tests the UnionMatcher implementation
func TestUnionMatcher(t *testing.T) {
	assert := assert.New(t)

	// Test paths for union pattern tests
	testPaths := []string{
		"src/foo.go",
		"src/bar.go",
		"src/baz_test.go",
		"internal/qux.go",
		"internal/qux_test.go",
		"docs/README.md",
	}

	t.Run("BasicUnion", func(t *testing.T) {
		// Create two matchers
		fuzzyMatcher1 := FuzzyMatcher{Pattern: "foo"}
		fuzzyMatcher2 := FuzzyMatcher{Pattern: "qux"}

		// Combine them in a union matcher (logical OR)
		unionMatcher := UnionMatcher{
			Matchers: []Matcher{fuzzyMatcher1, fuzzyMatcher2},
		}

		results, err := unionMatcher.Match(testPaths)
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"src/foo.go", "internal/qux.go", "internal/qux_test.go"},
			results,
			"Union matcher should match paths that match any of the matchers",
		)
	})

	t.Run("UnionWithNegation", func(t *testing.T) {
		// Create a fuzzy matcher
		fuzzyMatcher := FuzzyMatcher{Pattern: "src"}

		// Create a negation matcher
		testFileMatcher, err := NewRegexMatcher("_test\\.go$")
		assert.NoError(err)
		negationMatcher := NegationMatcher{Wrapped: testFileMatcher}

		// Combine them in a union matcher
		unionMatcher := UnionMatcher{
			Matchers: []Matcher{fuzzyMatcher, negationMatcher},
		}

		results, err := unionMatcher.Match(testPaths)
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"src/foo.go", "src/bar.go", "src/baz_test.go", "internal/qux.go", "docs/README.md"},
			results,
			"Union matcher should match paths that match any of the matchers",
		)
	})

	t.Run("EmptyUnion", func(t *testing.T) {
		// Empty union matcher should return empty results
		unionMatcher := UnionMatcher{Matchers: []Matcher{}}
		results, err := unionMatcher.Match(testPaths)
		assert.NoError(err)
		assert.Empty(results, "Empty union matcher should return empty results")
	})
}

// TestUnionPatterns tests union patterns with the ; operator
func TestUnionPatterns(t *testing.T) {
	assert := assert.New(t)

	// Test paths for union pattern tests
	testPaths := []string{
		"src/foo.go",
		"src/bar.go",
		"src/baz_test.go",
		"internal/qux.go",
		"internal/qux_test.go",
		"docs/README.md",
	}

	t.Run("BasicUnionPattern", func(t *testing.T) {
		// "foo;qux" should match paths containing "foo" OR "qux"
		results, err := selectSinglePattern(testPaths, "foo;qux")
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"src/foo.go", "internal/qux.go", "internal/qux_test.go"},
			results,
		)
	})

	t.Run("UnionWithWhitespace", func(t *testing.T) {
		// " foo ; qux " should match paths containing "foo" OR "qux"
		// with whitespace being stripped
		results, err := selectSinglePattern(testPaths, " foo ; qux ")
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"src/foo.go", "internal/qux.go", "internal/qux_test.go"},
			results,
		)
	})

	t.Run("UnionWithEmptyParts", func(t *testing.T) {
		// "foo;;qux" should match paths containing "foo" OR "qux"
		// with empty parts being skipped
		results, err := selectSinglePattern(testPaths, "foo;;qux")
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"src/foo.go", "internal/qux.go", "internal/qux_test.go"},
			results,
		)
	})

	t.Run("UnionWithDifferentMatcherTypes", func(t *testing.T) {
		// "foo;/_test\\.go$/" should match paths containing "foo" OR ending with "_test.go"
		results, err := selectSinglePattern(testPaths, "foo;/_test\\.go$")
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"src/foo.go", "src/baz_test.go", "internal/qux_test.go"},
			results,
		)
	})

	t.Run("UnionWithCompound", func(t *testing.T) {
		// "foo;src|bar" should match paths containing "foo" OR (paths containing both "src" AND "bar")
		results, err := selectSinglePattern(testPaths, "foo;src|bar")
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"src/foo.go", "src/bar.go"},
			results,
		)
	})

	t.Run("UnionWithNegation", func(t *testing.T) {
		// "foo;!test" should match paths containing "foo" OR paths NOT containing "test"
		results, err := selectSinglePattern(testPaths, "foo;!test")
		assert.NoError(err)
		assert.ElementsMatch(
			[]string{"src/foo.go", "src/bar.go", "internal/qux.go", "docs/README.md"},
			results,
		)
	})

	t.Run("InvalidUnionPattern", func(t *testing.T) {
		// ";;;" should return an error as it contains no valid patterns
		_, err := selectSinglePattern(testPaths, ";;;")
		assert.Error(err)
		assert.Contains(err.Error(), "union pattern contains no valid patterns")
	})
}

func TestGlobMatcher(t *testing.T) {
	assert := assert.New(t)
	paths := testPaths // reuse the testPaths from the global var

	t.Run("SingleStar", func(t *testing.T) {
		matcher := GlobMatcher{Pattern: "src/*.go"}
		results, err := matcher.Match(paths)
		assert.NoError(err)
		assert.ElementsMatch([]string{"src/foo.go", "src/foo_test.go"}, results)
	})

	t.Run("DoubleStar", func(t *testing.T) {
		matcher := GlobMatcher{Pattern: "**/*.md"}
		results, err := matcher.Match(paths)
		assert.NoError(err)
		assert.ElementsMatch([]string{"docs/bar.md", "README.md"}, results)
	})

	t.Run("NoMatches", func(t *testing.T) {
		matcher := GlobMatcher{Pattern: "*.py"}
		results, err := matcher.Match(paths)
		assert.NoError(err)
		assert.Empty(results)
	})

	t.Run("InvalidPattern", func(t *testing.T) {
		matcher := GlobMatcher{Pattern: "["}
		results, err := matcher.Match(paths)
		assert.Error(err)
		assert.Contains(err.Error(), "invalid glob pattern")
		assert.Nil(results)
	})

	t.Run("RecursivePattern", func(t *testing.T) {
		paths2 := []string{
			"foo/bar/test1_test.go",
			"foo/test2_test.go",
			"bar/baz/test3_test.go",
			"bar/doc.md",
		}
		matcher, err := NewGlobMatcher("**/*_test.go")
		assert.NoError(err)
		results, err := matcher.Match(paths2)
		assert.NoError(err)
		assert.ElementsMatch([]string{
			"foo/bar/test1_test.go",
			"foo/test2_test.go",
			"bar/baz/test3_test.go",
		}, results)
	})
}
