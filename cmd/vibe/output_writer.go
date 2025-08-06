package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/atotto/clipboard"
)

// ClipboardWriter wraps a buffer and writes to clipboard on close.
type ClipboardWriter struct {
	*bytes.Buffer
}

// Close writes the buffer content to clipboard.
func (cw *ClipboardWriter) Close() error {
	if err := clipboard.WriteAll(cw.String()); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %v", err)
	}
	fmt.Fprintln(os.Stderr, "Output copied to clipboard")
	return nil
}

// StdoutWriter wraps stdout with a no-op Close method.
type StdoutWriter struct {
	io.Writer
}

// Close is a no-op for stdout.
func (sw *StdoutWriter) Close() error {
	return nil
}

// NewOutputWriter creates a new output writer based on the output specification.
func NewOutputWriter(output string) (io.WriteCloser, error) {
	switch {
	case output == "-":
		// stdout
		return &StdoutWriter{Writer: os.Stdout}, nil
	case output != "":
		// file
		return os.Create(output)
	default:
		// clipboard
		return &ClipboardWriter{Buffer: &bytes.Buffer{}}, nil
	}
}
