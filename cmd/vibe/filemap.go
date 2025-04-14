package main

import (
	"fmt"
	"io"
	"os"
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

		// Read selected file content
		content, err := selection.ReadString()
		if err != nil {
			return fmt.Errorf("failed to read selected content from %s: %w", selection.Path, err)
		}

		// Write file content
		fmt.Fprint(w, content)
	}

	return nil
}
