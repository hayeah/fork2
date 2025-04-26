package main

import (
	"fmt"
	"io"
	"os"
	"unicode"
	"unicode/utf8"

	"github.com/hayeah/fork2/internal/metrics"
)

// FileMap represents a mapping of file paths to their contents
type FileMap struct {
	Files map[string]string
}

// FileMapWriter handles writing file selections with optional metrics tracking
type FileMapWriter struct {
	baseDir string
	metrics  *metrics.OutputMetrics
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

// NewWriteFileMap creates a new FileMapWriter with the given base directory and metrics
func NewWriteFileMap(baseDir string, m *metrics.OutputMetrics) *FileMapWriter {
	return &FileMapWriter{
		baseDir: baseDir,
		metrics:  m,
	}
}

// Output writes file selections to the provided writer
func (w *FileMapWriter) Output(out io.Writer, selections []FileSelection) error {
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
		fmt.Fprint(out, content)

		// Add metrics for file content
		if w.metrics != nil {
			w.metrics.Add("file", selection.Path, []byte(content))
		}
	}

	return nil
}
