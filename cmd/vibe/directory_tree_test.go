package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTestDirectory(t *testing.T, files map[string]string) (string, error) {
	t.Helper()
	tempDir := t.TempDir()

	for relPath, content := range files {
		path := filepath.Join(tempDir, relPath)
		dir := filepath.Dir(path)

		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return "", err
		}
		err = os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			return "", err
		}
	}
	return tempDir, nil
}

func TestDirectoryTree_LoadDirectoryTree(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{
		"sub/file1.txt": "hello world",
		"file2.go":      "package main\n",
	})
	assert.NoError(err)

	subDir := filepath.Join(tempDir, "sub")

	dt, err := LoadDirectoryTree(tempDir)
	assert.NoError(err)
	assert.NotNil(dt)

	// We expect 3 items in total: tempDir, subDir, file2.go, plus file1.txt
	// Actually 4 items if we count root + sub + file2 + file1
	assert.GreaterOrEqual(len(dt.Items), 4, "Should have at least 4 items in directory tree")

	var foundSub bool
	var foundFile2 bool
	var foundFile1 bool
	for _, it := range dt.Items {
		switch it.Path {
		case subDir:
			foundSub = true
		case filepath.Join(subDir, "file1.txt"):
			foundFile1 = true
		case filepath.Join(tempDir, "file2.go"):
			foundFile2 = true
		}
	}
	assert.True(foundSub, "subDir should be in the listing")
	assert.True(foundFile1, "file1.txt should be in the listing")
	assert.True(foundFile2, "file2.go should be in the listing")

	assert.Equal(tempDir, dt.RootPath, "DirectoryTree RootPath should match the provided rootPath")
}

func TestDirectoryTree_SelectAllFiles(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{
		"a.txt":        "aaa",
		"subdir/b.txt": "bbb",
	})
	assert.NoError(err)

	dt, err := LoadDirectoryTree(tempDir)
	assert.NoError(err)

	all := dt.SelectAllFiles()
	assert.Len(all, 2, "Should only find 2 files (a.txt, subdir/b.txt)")
	assert.Contains(all, filepath.Join(tempDir, "a.txt"))
	assert.Contains(all, filepath.Join(tempDir, "subdir", "b.txt"))
}

func TestDirectoryTree_SelectFuzzyFiles(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{
		"dirA/alpha.go":  "// alpha",
		"dirB/beta.md":   "beta doc",
		"dirA/gamma.txt": "gamma data",
	})
	assert.NoError(err)

	dt, err := LoadDirectoryTree(tempDir)
	assert.NoError(err)

	fuzzA, err := dt.SelectFilesByPattern("alp")
	assert.NoError(err)
	assert.Len(fuzzA, 1, "Should find only alpha.go with pattern 'alp'")
	assert.Contains(fuzzA, filepath.Join(tempDir, "dirA", "alpha.go"))

	fuzzAll, err := dt.SelectFilesByPattern(".")
	assert.NoError(err)
	assert.GreaterOrEqual(len(fuzzAll), 3, "Should find multiple files with '.'")
}

func TestDirectoryTree_SelectRegexFiles(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{
		"foo/file1_test.go": "// file1_test",
		"foo/file2.py":      "# file2",
		"foo/file3_test.go": "// file3_test",
	})
	assert.NoError(err)

	dt, err := LoadDirectoryTree(tempDir)
	assert.NoError(err)

	testGoFiles, err := dt.SelectRegexFiles("_test.go$")
	assert.NoError(err)
	assert.Len(testGoFiles, 2, "Should find 2 files that end in _test.go")
	assert.Contains(testGoFiles, filepath.Join(tempDir, "foo", "file1_test.go"))
	assert.Contains(testGoFiles, filepath.Join(tempDir, "foo", "file3_test.go"))

	pyFiles, err := dt.SelectRegexFiles(`\.py$`)
	assert.NoError(err)
	assert.Len(pyFiles, 1)
	assert.Contains(pyFiles, filepath.Join(tempDir, "foo", "file2.py"))
}

func TestDirectoryTree_GenerateDirectoryTree(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{
		"hello/a.txt":       "A",
		"hello/world/b.txt": "B",
	})
	assert.NoError(err)

	dt, err := LoadDirectoryTree(tempDir)
	assert.NoError(err)
	assert.NotNil(dt)

	var buf bytes.Buffer
	err = dt.GenerateDirectoryTree(&buf)
	assert.NoError(err)
	output := buf.String()
	fmt.Println(output)

	// assert.Equal(output, "hello\nworld\n")

	// // We won't assert exact line structure, but let's check for some markers:
	assert.Contains(output, strings.TrimSpace(`
└── hello
    ├── world
    │   └── b.txt
    └── a.txt
`))

	absPath, _ := filepath.Abs(tempDir)
	assert.Contains(output, absPath, "Should contain the absolute path to the root directory")
}

func TestDirectoryTree_EmptyDir(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{})
	assert.NoError(err)

	dt, err := LoadDirectoryTree(tempDir)
	assert.NoError(err)
	assert.NotNil(dt)
	assert.Equal(1, len(dt.Items), "Should contain only the root directory itself")

	var allFiles []string
	allFiles = dt.SelectAllFiles()
	assert.Len(allFiles, 0, "No files to select in an empty dir")
}

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
