package main

import (
	"bytes"
	"fmt"
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
	err = dt.GenerateDirectoryTree(&buf)
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
