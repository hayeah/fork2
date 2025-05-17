package selection

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestFileSelectionContentLineRange(t *testing.T) {
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
	fs := FileSelection{Path: testFile, Ranges: []LineRange{}}
	contents, err := fs.Contents()
	assert.NoError(err)
	assert.Len(contents, 1)
	assert.Equal(testFile, contents[0].Path)
	assert.Equal(content.String(), contents[0].Content)
	assert.Nil(contents[0].Range)

	// Test extracting specific ranges
	ranges := []LineRange{
		{Start: 1, End: 3},
		{Start: 10, End: 12},
	}
	fs = FileSelection{Path: testFile, Ranges: ranges}
	contents, err = fs.Contents()
	assert.NoError(err)
	assert.Len(contents, 2)

	// Check first range
	assert.Equal(testFile, contents[0].Path)
	assert.Equal("Line 1\nLine 2\nLine 3\n", contents[0].Content)
	assert.Equal(1, contents[0].Range.Start)
	assert.Equal(3, contents[0].Range.End)

	// Check second range
	assert.Equal(testFile, contents[1].Path)
	assert.Equal("Line 10\nLine 11\nLine 12\n", contents[1].Content)
	assert.Equal(10, contents[1].Range.Start)
	assert.Equal(12, contents[1].Range.End)

	// Test ReadString to ensure backward compatibility
	// expected := "--- " + testFile + "#1,3 ---\nLine 1\nLine 2\nLine 3\n--- " + testFile + "#10,12 ---\nLine 10\nLine 11\nLine 12\n"
	// selectedLines, err := fs.ReadString()
	// assert.NoError(err)
	// assert.Equal(expected, selectedLines)

	// Test with non-existent file
	fs = FileSelection{Path: filepath.Join(tempDir, "nonexistent.txt"), Ranges: ranges}
	_, err = fs.Contents()
	assert.Error(err)
}

// TestParseLineRangeFromPath tests the parseLineRangeFromPath function
func TestParseLineRangeFromPath(t *testing.T) {
	assert := assert.New(t)

	t.Run("PathWithoutRange", func(t *testing.T) {
		result, err := ParseFileSelection("path/to/file.go")
		assert.NoError(err)
		assert.Equal("path/to/file.go", result.Path)
		assert.Empty(result.Ranges, "Should have no ranges for path without # marker")
	})

	t.Run("PathWithValidRange", func(t *testing.T) {
		result, err := ParseFileSelection("path/to/file.go#10,20")
		assert.NoError(err)
		assert.Equal("path/to/file.go", result.Path)
		assert.Len(result.Ranges, 1, "Should have one range")
		assert.Equal(10, result.Ranges[0].Start)
		assert.Equal(20, result.Ranges[0].End)
	})

	t.Run("PathWithInvalidFormat", func(t *testing.T) {
		_, err := ParseFileSelection("path/to/file.go#abc,20")
		assert.Error(err)
		assert.Contains(err.Error(), "invalid file path format")
	})

	t.Run("PathWithInvalidEndLine", func(t *testing.T) {
		// This will now fail at the regex matching stage
		_, err := ParseFileSelection("path/to/file.go#10,xyz")
		assert.Error(err)
		assert.Contains(err.Error(), "invalid file path format")
	})

	t.Run("PathWithHashButNoRange", func(t *testing.T) {
		_, err := ParseFileSelection("path/to/file.go#")
		assert.Error(err)
		assert.Contains(err.Error(), "invalid file path format")
	})

	t.Run("PathWithInvalidRangeFormat", func(t *testing.T) {
		_, err := ParseFileSelection("path/to/file.go#10-20")
		assert.Error(err)
		assert.Contains(err.Error(), "invalid file path format")
	})

	t.Run("PathWithMultipleHashes", func(t *testing.T) {
		_, err := ParseFileSelection("path/to/file#10#20")
		assert.Error(err)
		assert.Contains(err.Error(), "invalid file path format")
	})

	t.Run("PathWithMissingComma", func(t *testing.T) {
		_, err := ParseFileSelection("path/to/file.go#1020")
		assert.Error(err)
		assert.Contains(err.Error(), "invalid file path format")
	})
}
