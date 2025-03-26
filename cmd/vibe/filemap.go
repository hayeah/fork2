package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode"
	"unicode/utf8"
)

// FileMap represents a mapping of file paths to their contents
type FileMap struct {
	Files map[string]string
}

// IsBinaryFile checks if content is likely binary by sampling the first 100 runes
// and checking if they are printable Unicode characters
func IsBinaryFile(content []byte) bool {
	// Sample the first 100 runes
	const sampleSize = 100
	var nonPrintable int
	var totalRunes int

	// Convert bytes to runes and check if they're printable
	for i := 0; i < len(content) && totalRunes < sampleSize; {
		r, size := utf8.DecodeRune(content[i:])
		if r == utf8.RuneError {
			nonPrintable++
		} else if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			nonPrintable++
		}
		i += size
		totalRunes++
	}

	// If more than 10% of the sampled runes are non-printable, consider it binary
	threshold := 0.1
	if totalRunes == 0 {
		return false // Empty file, not binary
	}
	return float64(nonPrintable)/float64(totalRunes) > threshold
}

// WriteFileMap writes a filemap to the provided writer for the given file selections
func WriteFileMap(w io.Writer, selections []FileSelection, baseDir string) error {
	for _, selection := range selections {
		// Skip directories
		fileInfo, err := os.Stat(selection.Path)
		if err != nil {
			return fmt.Errorf("failed to stat file %s: %w", selection.Path, err)
		}
		if fileInfo.IsDir() {
			continue
		}

		// Read file content using FileSelection.ReadString()
		content, err := selection.ReadString()
		if err != nil {
			return fmt.Errorf("failed to read selected content from %s: %w", selection.Path, err)
		}

		// Check if content is binary
		if IsBinaryFile([]byte(content)) {
			continue // Skip binary files
		}

		// Get relative path for display
		relPath, err := filepath.Rel(baseDir, selection.Path)
		if err != nil {
			relPath = selection.Path // Fallback to absolute path
		}
		
		// Write file header
		fmt.Fprintf(w, "File: %s\n", relPath)
		fmt.Fprint(w, "```\n")

		// Write file content
		fmt.Fprint(w, content)

		// Ensure the content ends with a newline
		if len(content) > 0 && content[len(content)-1] != '\n' {
			fmt.Fprintln(w)
		}

		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
	}

	return nil
}
