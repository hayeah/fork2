package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/hayeah/fork2/ignore"
	"github.com/sahilm/fuzzy"
)

// gatherFiles recursively walks the directory and returns a sorted list of item
// plus a children map for toggling entire subtrees.
// It respects .gitignore patterns in the directory.
func gatherFiles(rootPath string) ([]item, map[string][]string, error) {
	var items []item
	childrenMap := make(map[string][]string)

	ig, err := ignore.NewIgnore(rootPath)
	if err != nil {
		return nil, nil, err
	}

	// Use WalkDirGitIgnore to walk the directory tree while respecting gitignore patterns
	err = ig.WalkDir(rootPath, func(path string, d os.DirEntry, isDir bool) error {
		items = append(items, item{
			Path:       path,
			IsDir:      isDir,
			TokenCount: 0, // Initialize token count to 0
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Sort items by path for a consistent listing
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})

	// Build childrenMap (immediate children only). We'll handle deeper toggling recursively.
	pathIndex := make(map[string]int, len(items))
	for i, it := range items {
		pathIndex[it.Path] = i
	}

	for _, it := range items {
		if it.IsDir {
			var childList []string
			// We'll gather direct children by checking whether:
			//   parent == it.Path
			// or path.Dir(child) == it.Path
			for _, possibleChild := range items {
				// If the dir of possibleChild is it.Path, consider it a child
				if filepath.Dir(possibleChild.Path) == it.Path && possibleChild.Path != it.Path {
					childList = append(childList, possibleChild.Path)
				}
			}
			childrenMap[it.Path] = childList
		}
	}

	return items, childrenMap, nil
}

// selectAllFiles selects all non-directory items
func selectAllFiles(items []item) ([]string, error) {
	var selectedFiles []string
	for _, it := range items {
		if !it.IsDir {
			selectedFiles = append(selectedFiles, it.Path)
		}
	}
	return selectedFiles, nil
}

// selectFuzzyFiles selects files matching a fuzzy pattern using the fuzzy library
func selectFuzzyFiles(items []item, pattern string) ([]string, error) {
	// Create a list of file paths (excluding directories)
	var filePaths []string
	var fileItems []item
	for _, it := range items {
		if !it.IsDir {
			filePaths = append(filePaths, it.Path)
			fileItems = append(fileItems, it)
		}
	}

	// Use the fuzzy library to find matches
	matches := fuzzy.Find(pattern, filePaths)

	// Extract the matched files
	var selectedFiles []string
	for _, match := range matches {
		selectedFiles = append(selectedFiles, fileItems[match.Index].Path)
	}

	return selectedFiles, nil
}

// selectRegexFiles selects files matching a regex pattern
func selectRegexFiles(items []item, pattern string) ([]string, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	var selectedFiles []string
	for _, it := range items {
		if !it.IsDir && regex.MatchString(it.Path) {
			selectedFiles = append(selectedFiles, it.Path)
		}
	}

	return selectedFiles, nil
}
