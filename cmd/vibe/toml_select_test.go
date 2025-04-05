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
	allLines, err := extractSelectedLines(testFile, []LineRange{})
	assert.NoError(err)
	assert.Equal(content.String(), allLines)

	// Test extracting specific ranges
	ranges := []LineRange{
		{Start: 1, End: 3},
		{Start: 10, End: 12},
	}
	expected := "Line 1\nLine 2\nLine 3\nLine 10\nLine 11\nLine 12\n"
	selectedLines, err := extractSelectedLines(testFile, ranges)
	assert.NoError(err)
	assert.Equal(expected, selectedLines)

	// Test with non-existent file
	_, err = extractSelectedLines(filepath.Join(tempDir, "nonexistent.txt"), ranges)
	assert.Error(err)
}
