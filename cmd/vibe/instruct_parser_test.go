package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFrontMatter(t *testing.T) {
	assert := assert.New(t)
	parser := NewInstructParser()

	// Test TOML front matter
	content := `---toml
[[file]]
path = "path/to/file.txt"
---
User instruction here.`

	tag, frontMatter, remainder, err := parser.parseFrontMatter(content)
	assert.NoError(err)
	assert.Equal("toml", tag)
	assert.Equal("[[file]]\npath = \"path/to/file.txt\"", frontMatter)
	assert.Equal("User instruction here.", remainder)

	// Test alternative delimiter
	// Test TOML front matter
	content = `+++toml
[[file]]
path = "path/to/file.txt"
+++
User instruction here.`

	tag, frontMatter, remainder, err = parser.parseFrontMatter(content)
	assert.NoError(err)
	assert.Equal("toml", tag)
	assert.Equal("[[file]]\npath = \"path/to/file.txt\"", frontMatter)
	assert.Equal("User instruction here.", remainder)
	// Test no front matter
	content = `This is just regular content
without any front matter.`

	tag, frontMatter, remainder, err = parser.parseFrontMatter(content)
	assert.NoError(err)
	assert.Equal("", tag)
	assert.Equal("", frontMatter)
	assert.Equal(content, remainder)

	// Test unclosed front matter
	content = `---
unclosed front matter
without closing delimiter`

	_, _, _, err = parser.parseFrontMatter(content)
	assert.Error(err)
	assert.Contains(err.Error(), "front matter not closed")
}

func TestReadInstructionContent(t *testing.T) {
	assert := assert.New(t)

	// Create temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	content := "This is a test file content."
	err := os.WriteFile(testFile, []byte(content), 0644)
	assert.NoError(err)

	parser := NewInstructParser()

	// Test reading from file
	fileContent, err := parser.readInstructionContent(testFile)
	assert.NoError(err)
	assert.Equal(content, fileContent)

	// Test reading raw string
	rawContent := "This is raw content, not a file path."
	stringContent, err := parser.readInstructionContent(rawContent)
	assert.NoError(err)
	assert.Equal(rawContent, stringContent)
}

func TestFileSelectionsWithDirTree_OnlySelect(t *testing.T) {
	assert := assert.New(t)

	// Create test files using the helper function
	files := map[string]string{
		"file1.go":        "// Test content\n",
		"file2.go":        "// Test content\n",
		"file3.txt":       "// Test content\n",
		"subdir/file4.go": "// Test content\n",
	}

	tmpDir, err := createTestDirectory(t, files)
	assert.NoError(err)
	// No need for defer os.RemoveAll as t.TempDir() handles cleanup

	// Create a list of absolute file paths for verification
	var testFiles []string
	for relPath := range files {
		testFiles = append(testFiles, filepath.Join(tmpDir, relPath))
	}

	// Load the actual directory tree
	dirTree, err := LoadDirectoryTree(tmpDir)
	assert.NoError(err)

	// Test with only Select
	headerSelectOnly := &InstructHeader{
		Select: fmt.Sprintf(`
/\.go$
=%s
`, filepath.Join(tmpDir, "file3.txt")), // file3.txt
	}

	selections, err := headerSelectOnly.FileSelectionsWithDirTree(dirTree)
	assert.NoError(err)
	assert.Len(selections, 4) // All 3 .go files + file3.txt

	// Check that all expected paths are present
	paths := make([]string, len(selections))
	for i, sel := range selections {
		paths[i] = sel.Path
	}

	assert.ElementsMatch(testFiles, paths)
}
