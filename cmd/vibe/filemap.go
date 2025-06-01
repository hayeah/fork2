package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hayeah/fork2/internal/metrics"
	selection "github.com/hayeah/fork2/internal/selection"
)

// FileMap represents a mapping of file paths to their contents
type FileMap struct {
	Files map[string]string
}

// FileMapWriter handles writing file selections with optional metrics tracking
type FileMapWriter struct {
	baseDir string
	metrics *metrics.OutputMetrics
}

// IsBinaryFile checks if content is likely binary by sampling the first 100 runes
// and checking if they are printable Unicode characters

// NewWriteFileMap creates a new FileMapWriter with the given base directory and metrics
func NewWriteFileMap(baseDir string, m *metrics.OutputMetrics) *FileMapWriter {
	return &FileMapWriter{
		baseDir: baseDir,
		metrics: m,
	}
}

// Output writes file selections to the provided writer
func (w *FileMapWriter) Output(out io.Writer, selections []selection.FileSelection) error {
	for _, selection := range selections {
		// Skip directories
		fileInfo, err := os.Stat(selection.Path)
		if err != nil {
			return fmt.Errorf("failed to stat file %s: %w", selection.Path, err)
		}
		if fileInfo.IsDir() {
			continue
		}

		// Stream file content directly to output and get byte count
		bytesWritten, err := selection.Read(out)
		if err != nil {
			return fmt.Errorf("failed to write selected content from %s: %w", selection.Path, err)
		}

		// Add metrics using byte count estimate
		if w.metrics != nil {
			w.metrics.AddBytesCountAsEstimate("file", selection.Path, int(bytesWritten))
		}
	}

	return nil
}
