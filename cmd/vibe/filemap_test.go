package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteFileMap(t *testing.T) {
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
			Ranges: []LineRange{{Start: 2, End: 4}}, // Select lines 2-4
		},
		{
			Path:   file2,
			Ranges: []LineRange{}, // All lines
		},
	}

	// Create a buffer to write to
	var buf strings.Builder

	// Test WriteFileMap
	err = WriteFileMap(&buf, selections, tempDir)
	assert.NoError(err)

	// Check output
	output := buf.String()

	// Expected output
	expected := fmt.Sprintf("File: file1.txt\n```\nLine 2\nLine 3\nLine 4\n```\n\nFile: file2.txt\n```\n%s```\n\n", content2)

	assert.Equal(expected, output)
}

func TestWriteFileMapBinary(t *testing.T) {
	assert := assert.New(t)

	// Create temporary test files
	tempDir := t.TempDir()

	// Create a binary file
	binaryFile := filepath.Join(tempDir, "binary.bin")
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD, 0xFC}

	err := os.WriteFile(binaryFile, binaryContent, 0644)
	assert.NoError(err)

	// Create a text file
	textFile := filepath.Join(tempDir, "text.txt")
	textContent := "This is a text file\n"

	err = os.WriteFile(textFile, []byte(textContent), 0644)
	assert.NoError(err)

	// Create file selections
	selections := []FileSelection{
		{
			Path:   binaryFile,
			Ranges: []LineRange{}, // All content
		},
		{
			Path:   textFile,
			Ranges: []LineRange{}, // All content
		},
	}

	// Create a buffer to write to
	var buf strings.Builder

	// Test WriteFileMap
	err = WriteFileMap(&buf, selections, tempDir)
	assert.NoError(err)

	// Check output - should only contain the text file
	output := buf.String()

	// Expected output - binary file should be skipped
	expected := fmt.Sprintf("File: text.txt\n```\n%s```\n\n", textContent)

	assert.Equal(expected, output)
}

func TestWriteFileMapDirectory(t *testing.T) {
	assert := assert.New(t)

	// Create temporary test directory
	tempDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	assert.NoError(err)

	// Create a text file
	textFile := filepath.Join(tempDir, "text.txt")
	textContent := "This is a text file\n"

	err = os.WriteFile(textFile, []byte(textContent), 0644)
	assert.NoError(err)

	// Create file selections including a directory
	selections := []FileSelection{
		{
			Path:   subDir,
			Ranges: []LineRange{}, // Not applicable for directory
		},
		{
			Path:   textFile,
			Ranges: []LineRange{}, // All content
		},
	}

	// Create a buffer to write to
	var buf strings.Builder

	// Test WriteFileMap
	err = WriteFileMap(&buf, selections, tempDir)
	assert.NoError(err)

	// Check output - should only contain the text file
	output := buf.String()

	// Expected output - directory should be skipped
	expected := fmt.Sprintf("File: text.txt\n```\n%s```\n\n", textContent)

	assert.Equal(expected, output)
}
