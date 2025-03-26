package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLineRange(t *testing.T) {
	assert := assert.New(t)

	// Test valid ranges
	r1, err := parseLineRange("1,5")
	assert.NoError(err)
	assert.Equal(LineRange{Start: 1, End: 5}, r1)

	r2, err := parseLineRange("10,15")
	assert.NoError(err)
	assert.Equal(LineRange{Start: 10, End: 15}, r2)

	// Test invalid ranges
	_, err = parseLineRange("5,3")
	assert.Error(err)
	assert.Contains(err.Error(), "end line (3) must be >= start line (5)")

	_, err = parseLineRange("0,5")
	assert.Error(err)
	assert.Contains(err.Error(), "start line must be >= 1")

	_, err = parseLineRange("a,5")
	assert.Error(err)
	assert.Contains(err.Error(), "invalid start line number")

	_, err = parseLineRange("1,b")
	assert.Error(err)
	assert.Contains(err.Error(), "invalid end line number")

	_, err = parseLineRange("1-5")
	assert.Error(err)
	assert.Contains(err.Error(), "invalid line range format")
}

func TestParseFilePathWithRange(t *testing.T) {
	assert := assert.New(t)

	// Current directory as root path
	rootPath, err := os.Getwd()
	assert.NoError(err)

	// Test path without range
	s1, err := parseFilePathWithRange("path/to/file.txt", rootPath)
	assert.NoError(err)
	assert.Equal(filepath.Join(rootPath, "path/to/file.txt"), s1.Path)
	assert.Empty(s1.Ranges)

	// Test path with range
	s2, err := parseFilePathWithRange("path/to/file.txt#1,5", rootPath)
	assert.NoError(err)
	assert.Equal(filepath.Join(rootPath, "path/to/file.txt"), s2.Path)
	assert.Len(s2.Ranges, 1)
	assert.Equal(LineRange{Start: 1, End: 5}, s2.Ranges[0])

	// Test absolute path
	absPath, err := filepath.Abs("path/to/file.txt")
	assert.NoError(err)
	s3, err := parseFilePathWithRange(absPath, rootPath)
	assert.NoError(err)
	assert.Equal(absPath, s3.Path)
	assert.Empty(s3.Ranges)

	// Test invalid path with range
	_, err = parseFilePathWithRange("path/to/file.txt#5,3", rootPath)
	assert.Error(err)
	assert.Contains(err.Error(), "end line (3) must be >= start line (5)")
}

func TestCoalesceRanges(t *testing.T) {
	assert := assert.New(t)

	// Test non-overlapping ranges
	ranges1 := []LineRange{
		{Start: 1, End: 5},
		{Start: 10, End: 15},
	}
	coalesced1 := coalesceRanges(ranges1)
	assert.Equal(ranges1, coalesced1)

	// Test overlapping ranges
	ranges2 := []LineRange{
		{Start: 1, End: 10},
		{Start: 5, End: 15},
	}
	coalesced2 := coalesceRanges(ranges2)
	assert.Equal([]LineRange{{Start: 1, End: 15}}, coalesced2)

	// Test adjacent ranges
	ranges3 := []LineRange{
		{Start: 1, End: 5},
		{Start: 6, End: 10},
	}
	coalesced3 := coalesceRanges(ranges3)
	assert.Equal([]LineRange{{Start: 1, End: 10}}, coalesced3)

	// Test multiple overlapping ranges
	ranges4 := []LineRange{
		{Start: 5, End: 10},
		{Start: 1, End: 3},
		{Start: 8, End: 15},
		{Start: 20, End: 25},
	}
	coalesced4 := coalesceRanges(ranges4)
	assert.Equal([]LineRange{
		{Start: 1, End: 3},
		{Start: 5, End: 15},
		{Start: 20, End: 25},
	}, coalesced4)
}

func TestParseTomlSelections(t *testing.T) {
	assert := assert.New(t)

	// Current directory as root path
	rootPath, err := os.Getwd()
	assert.NoError(err)

	// Test parsing TOML with multiple selections
	tomlContent := `
[[select]]
file = "path/to/a.txt"

[[select]]
file = "path/to/b.txt#1,5"

[[select]]
file = "path/to/b.txt#10,15"
`

	selections, err := ParseTomlSelections(strings.NewReader(tomlContent), rootPath)
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
	assert.Equal(filepath.Join(rootPath, "path/to/a.txt"), aTxt.Path)
	assert.Empty(aTxt.Ranges)

	// Verify b.txt
	assert.NotNil(bTxt)
	assert.Equal(filepath.Join(rootPath, "path/to/b.txt"), bTxt.Path)
	assert.Len(bTxt.Ranges, 2)
	assert.Equal(LineRange{Start: 1, End: 5}, bTxt.Ranges[0])
	assert.Equal(LineRange{Start: 10, End: 15}, bTxt.Ranges[1])
}

func TestExtractSelectedLines(t *testing.T) {
	assert := assert.New(t)

	// Create temporary test files
	tempDir := t.TempDir()

	// Create test file with 20 lines
	testFile := filepath.Join(tempDir, "test.txt")
	var content strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&content, "Line %d\n", i)
	}
	err := os.WriteFile(testFile, []byte(content.String()), 0644)
	assert.NoError(err)

	// Test extracting all lines (no ranges)
	allLines, err := ExtractSelectedLines(testFile, []LineRange{})
	assert.NoError(err)
	assert.Equal(content.String(), allLines)

	// Test extracting specific ranges
	ranges := []LineRange{
		{Start: 1, End: 3},
		{Start: 10, End: 12},
	}
	expected := "Line 1\nLine 2\nLine 3\nLine 10\nLine 11\nLine 12\n"
	selectedLines, err := ExtractSelectedLines(testFile, ranges)
	assert.NoError(err)
	assert.Equal(expected, selectedLines)

	// Test with non-existent file
	_, err = ExtractSelectedLines(filepath.Join(tempDir, "nonexistent.txt"), ranges)
	assert.Error(err)
}

func TestGeneratePartialContent(t *testing.T) {
	assert := assert.New(t)

	// Create temporary test files
	tempDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")

	content1 := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"
	content2 := "Line A\nLine B\nLine C\nLine D\nLine E\n"

	err := os.WriteFile(file1, []byte(content1), 0644)
	assert.NoError(err)

	err = os.WriteFile(file2, []byte(content2), 0644)
	assert.NoError(err)

	// Create file selections
	selections := []FileSelection{
		{
			Path:   file1,
			Ranges: []LineRange{{Start: 2, End: 4}},
		},
		{
			Path:   file2,
			Ranges: []LineRange{}, // All lines
		},
	}

	// Test generating partial content
	result, err := GeneratePartialContent(selections)
	assert.NoError(err)
	assert.Len(result, 2)

	// Check content for file1
	assert.Equal("Line 2\nLine 3\nLine 4\n", result[file1])

	// Check content for file2
	assert.Equal(content2, result[file2])

	// Test with non-existent file
	badSelections := []FileSelection{
		{
			Path:   filepath.Join(tempDir, "nonexistent.txt"),
			Ranges: []LineRange{},
		},
	}

	_, err = GeneratePartialContent(badSelections)
	assert.Error(err)
}
