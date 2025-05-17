package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	selection "github.com/hayeah/fork2/internal/selection"
	"github.com/stretchr/testify/assert"
)

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
	selections := []selection.FileSelection{
		{
			Path:   subDir,
			Ranges: []selection.LineRange{}, // Not applicable for directory
		},
		{
			Path:   textFile,
			Ranges: []selection.LineRange{}, // All content
		},
	}

	// Create a buffer to write to
	var buf strings.Builder

	// Test FileMapWriter
	fileMapWriter := NewWriteFileMap(tempDir, nil)
	err = fileMapWriter.Output(&buf, selections)
	assert.NoError(err)

	// Check output - should only contain the text file
	output := buf.String()

	assert.Contains(output, textFile)
	assert.Contains(output, textContent)
}
