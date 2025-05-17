package main

import (
    "io"
    selection "github.com/hayeah/fork2/internal/selection"
)

// DirectoryTreeService defines the operations for working with repository files
// and directories.
type DirectoryTreeService interface {
    SelectFiles(pattern string) ([]selection.FileSelection, error)
    GenerateDirectoryTree(w io.Writer, pattern string) error
    Filter(pattern string) ([]item, error)
}
