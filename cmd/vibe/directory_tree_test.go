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
