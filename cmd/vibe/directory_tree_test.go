package main

import (
	"bytes"
	"fmt"
	setpkg "github.com/hayeah/fork2/internal/set"
	"os"
	"path/filepath"
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

func TestDirectoryTree_SelectAllFiles(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{
		"a.txt":        "aaa",
		"subdir/b.txt": "bbb",
	})
	assert.NoError(err)

	dt := NewDirectoryTree(tempDir)
	assert.NotNil(dt)

	all := dt.SelectAllFiles()
	assert.ElementsMatch([]string{
		"a.txt",
		"subdir/b.txt",
	}, all)
}

func TestDirectoryTree_GenerateDirectoryTree(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{
		"hello/a.txt":       "A",
		"hello/world/b.txt": "B",
	})
	assert.NoError(err)

	dt := NewDirectoryTree(tempDir)
	assert.NotNil(dt)

	var buf bytes.Buffer
	err = dt.GenerateDirectoryTree(&buf, "")
	assert.NoError(err)
	output := buf.String()
	fmt.Println(output)

	// Check for the expected directory structure
	assert.Contains(output, strings.TrimSpace(`
└── hello/
    ├── a.txt
    └── world/
        └── b.txt
`))

	// Since LoadDirectoryTree now always deals with relative paths,
	// we should still see the absolute path in the output
	absPath, _ := filepath.Abs(tempDir)
	assert.Contains(output, absPath, "Should contain the absolute path to the root directory")
}

func TestDirectoryTree_EmptyDir(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{})
	assert.NoError(err)

	dt := NewDirectoryTree(tempDir)
	assert.NotNil(dt)
	items, err := dt.dirItems()
	assert.NoError(err)
	assert.Equal(1, len(items), "Should contain only the root directory itself")

	allFiles := dt.SelectAllFiles()
	assert.Len(allFiles, 0, "No files to select in an empty dir")
}

func TestDirectoryTree_Filter(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{
		"cmd/a.go":        "A",
		"cmd/b.txt":       "B",
		"cmd/vibe/c.go":   "C",
		"internal/d.go":   "D",
		"internal/e.txt":  "E",
		"src/f.go":        "F",
		"src/pkg/g.go":    "G",
		"src/pkg/test.go": "Test",
		"src/pkg/h.txt":   "H",
	})
	assert.NoError(err)

	dt := NewDirectoryTree(tempDir)
	assert.NotNil(dt)

	items, err := dt.Filter("cmd/;internal/")
	assert.NoError(err)

	// Create a set of paths for easier assertion
	pathSet := setpkg.NewSet[string]()
	for _, item := range items {
		pathSet.Add(item.Path)
	}

	// Use ElementsMatch to verify the exact paths in the result
	expectedPaths := []string{
		".",
		"cmd",
		"cmd/a.go",
		"cmd/b.txt",
		"cmd/vibe",
		"cmd/vibe/c.go",
		"internal",
		"internal/d.go",
		"internal/e.txt",
	}
	assert.ElementsMatch(expectedPaths, pathSet.Values(), "The filtered paths should match the expected set of paths")
}

func TestDirectoryTree_FilterEmptyPattern(t *testing.T) {
	assert := assert.New(t)

	tempDir, err := createTestDirectory(t, map[string]string{
		"a.txt":        "A",
		"subdir/b.txt": "B",
	})
	assert.NoError(err)

	dt := NewDirectoryTree(tempDir)
	assert.NotNil(dt)

	// Test with empty pattern
	allItems, err := dt.dirItems()
	assert.NoError(err)

	filteredItems, err := dt.Filter("")
	assert.NoError(err)

	// With empty pattern, should return all items
	assert.Equal(allItems, filteredItems, "Empty pattern should return all items")
}
