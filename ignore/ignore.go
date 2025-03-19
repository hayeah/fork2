package ignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// Ignore encapsulates gitignore pattern matching functionality
type Ignore struct {
	matcher  gitignore.Matcher
	rootPath string
}

// NewIgnore creates a new Ignore instance for the given root path
func NewIgnore(rootPath string) (*Ignore, error) {
	// Create a filesystem for the directory
	fs := osfs.New(rootPath)
	// Read gitignore patterns
	patterns, err := gitignore.ReadPatterns(fs, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to read gitignore patterns: %w", err)
	}

	// Create a Matcher that can check if a file or directory is ignored
	matcher := gitignore.NewMatcher(patterns)

	return &Ignore{
		matcher:  matcher,
		rootPath: rootPath,
	}, nil
}

// IsIgnored checks if a path should be ignored according to gitignore rules
func (ig *Ignore) IsIgnored(path string, isDir bool) (bool, error) {
	// Skip .git directory
	if isDir && filepath.Base(path) == ".git" {
		return true, nil
	}

	// Convert absolute path to a relative path for the matcher
	relPath, err := filepath.Rel(ig.rootPath, path)
	if err != nil {
		return false, err
	}

	// Skip the root directory
	if relPath == "." {
		return false, nil
	}

	parts := strings.Split(relPath, string(os.PathSeparator))
	return ig.matcher.Match(parts, isDir), nil
}

// WalkDir walks the file tree rooted at root, calling fn for each file or
// directory in the tree, including root, while respecting gitignore patterns.
func (ig *Ignore) WalkDir(root string, fn func(path string, d os.DirEntry, isDir bool) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		isDir := d.IsDir()

		// Check if the file/directory should be ignored
		ignored, err := ig.IsIgnored(path, isDir)
		if err != nil {
			return err
		}

		if ignored {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		// Call the provided function with the path and directory entry
		return fn(path, d, isDir)
	})
}
