package main

import (
    "io"
    selection "github.com/hayeah/fork2/internal/selection"
)

// FileMapService writes selected files to an output stream.
type FileMapService interface {
    Output(out io.Writer, selections []selection.FileSelection) error
}
