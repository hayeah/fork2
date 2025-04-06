package main

import (
	"os"
	"path/filepath"
	"strings"
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

func TestFileSelections(t *testing.T) {
	assert := assert.New(t)

	// Test parsing TOML with multiple selections
	tomlContent := `
[[file]]
path = "path/to/a.txt"

[[file]]
path = "path/to/b.txt#1,5"

[[file]]
path = "path/to/b.txt#10,15"
`

	parser := NewInstructParser()
	header, err := parser.parseTomlHeader(tomlContent)
	assert.NoError(err)

	selections, err := header.FileSelections()
	assert.NoError(err)
	assert.Len(selections, 2) // Two files: a.txt and b.txt

	// Find a.txt and b.txt in the selections
	var aTxt, bTxt *FileSelection
	for i := range selections {
		if strings.HasSuffix(selections[i].Path, "a.txt") {
			aTxt = &selections[i]
		} else if strings.HasSuffix(selections[i].Path, "b.txt") {
			bTxt = &selections[i]
		}
	}

	// Verify a.txt
	assert.NotNil(aTxt)
	assert.Equal("path/to/a.txt", aTxt.Path)
	assert.Empty(aTxt.Ranges)

	// Verify b.txt
	assert.NotNil(bTxt)
	assert.Equal("path/to/b.txt", bTxt.Path)
	assert.Len(bTxt.Ranges, 2)
	assert.Equal(LineRange{Start: 1, End: 5}, bTxt.Ranges[0])
	assert.Equal(LineRange{Start: 10, End: 15}, bTxt.Ranges[1])
}

func TestParse(t *testing.T) {
	assert := assert.New(t)

	parser := NewInstructParser()

	// Test parsing with TOML front matter
	input := `---toml
[[file]]
path = "path/to/file.txt#1,10"
---
This is the user instruction.`

	instruct, err := parser.Parse(input)
	assert.NoError(err)
	assert.NotNil(instruct)
	assert.NotNil(instruct.FrontMatter)
	assert.Equal("toml", instruct.FrontMatter.Tag)
	assert.NotNil(instruct.Header)
	assert.Equal("This is the user instruction.", instruct.UserContent)

	selections, err := instruct.Header.FileSelections()
	assert.NoError(err)
	assert.Len(selections, 1)
	assert.Equal("path/to/file.txt", selections[0].Path)
	assert.Len(selections[0].Ranges, 1)
	assert.Equal(LineRange{Start: 1, End: 10}, selections[0].Ranges[0])
}

// mockDirectoryTree is a simple mock for testing
type mockDirectoryTree struct {
	paths []string
}

func (m *mockDirectoryTree) SelectAllFiles() []string {
	return m.paths
}

func TestFileSelectionsWithDirTree_OnlyFiles(t *testing.T) {
	assert := assert.New(t)

	// Create a mock directory tree
	mockDirTree := &mockDirectoryTree{
		paths: []string{
			"/tmp/file1.go",
			"/tmp/file2.go",
			"/tmp/file3.txt",
			"/tmp/subdir/file4.go",
		},
	}

	// Test with only Files
	headerFilesOnly := &InstructHeader{
		Files: []FileSelection{
			{Path: "/tmp/file1.go", Ranges: []LineRange{{Start: 1, End: 5}}},
		},
	}

	// First test normal FileSelections()
	fs, err := headerFilesOnly.FileSelections()
	assert.NoError(err)
	assert.Len(fs, 1, "FileSelections should return 1 selection")

	// Now test with the directory tree
	selections, err := headerFilesOnly.FileSelectionsWithDirTree(mockDirTree)
	assert.NoError(err)
	assert.NotNil(selections, "Selections should not be nil")
	assert.Greater(len(selections), 0, "Selections should not be empty")
}

func TestFileSelectionsWithDirTree_OnlySelect(t *testing.T) {
	assert := assert.New(t)

	// Create a mock directory tree
	mockDirTree := &mockDirectoryTree{
		paths: []string{
			"/tmp/file1.go",
			"/tmp/file2.go",
			"/tmp/file3.txt",
			"/tmp/subdir/file4.go",
		},
	}

	// Test with only Select
	headerSelectOnly := &InstructHeader{
		Select: `
/\.go$
=/tmp/file3.txt
`,
	}

	selections, err := headerSelectOnly.FileSelectionsWithDirTree(mockDirTree)
	assert.NoError(err)
	assert.Len(selections, 4) // All 3 .go files + file3.txt

	// Check that all expected paths are present
	paths := make([]string, len(selections))
	for i, sel := range selections {
		paths[i] = sel.Path
	}

	assert.ElementsMatch([]string{
		"/tmp/file1.go",
		"/tmp/file2.go",
		"/tmp/file3.txt",
		"/tmp/subdir/file4.go",
	}, paths)
}

func TestFileSelectionsWithDirTree_Both(t *testing.T) {
	assert := assert.New(t)

	// Create a mock directory tree
	mockDirTree := &mockDirectoryTree{
		paths: []string{
			"/tmp/file1.go",
			"/tmp/file2.go",
			"/tmp/file3.txt",
			"/tmp/subdir/file4.go",
		},
	}

	// Test with both Select and Files
	header := &InstructHeader{
		Select: `
/\.go$
=/tmp/file3.txt
`,
		Files: []FileSelection{
			{Path: "/tmp/file1.go", Ranges: []LineRange{{Start: 1, End: 5}}},
		},
	}

	selections, err := header.FileSelectionsWithDirTree(mockDirTree)
	assert.NoError(err)
	assert.NotNil(selections, "Selections should not be nil")
	assert.Greater(len(selections), 0, "Selections should not be empty")

	// Check all expected paths are present
	paths := make(map[string]bool)
	for _, sel := range selections {
		paths[sel.Path] = true
		// If this is file1.go, check for its ranges
		if sel.Path == "/tmp/file1.go" && len(sel.Ranges) > 0 {
			assert.Equal(1, sel.Ranges[0].Start, "file1.go should have range start=1")
			assert.Equal(5, sel.Ranges[0].End, "file1.go should have range end=5")
		}
	}

	expectedPaths := []string{
		"/tmp/file1.go",
		"/tmp/file2.go",
		"/tmp/file3.txt",
		"/tmp/subdir/file4.go",
	}

	for _, path := range expectedPaths {
		assert.True(paths[path], "Expected path %s was not selected", path)
	}
}
