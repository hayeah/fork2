package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hayeah/fork2/ignore"
	"github.com/sahilm/fuzzy"
)

// DirectoryTree holds directory listing info.
type DirectoryTree struct {
	RootPath    string
	Items       []item
	ChildrenMap map[string][]string
}

// LoadDirectoryTree constructs a new DirectoryTree by walking rootPath.
func LoadDirectoryTree(rootPath string) (*DirectoryTree, error) {
	items, childrenMap, err := gatherFiles(rootPath)
	if err != nil {
		return nil, err
	}
	return &DirectoryTree{
		RootPath:    rootPath,
		Items:       items,
		ChildrenMap: childrenMap,
	}, nil
}

// gatherFiles recursively walks the directory and returns a sorted list of items
// plus a children map for toggling entire subtrees, respecting .gitignore.
func gatherFiles(rootPath string) ([]item, map[string][]string, error) {
	var items []item
	childrenMap := make(map[string][]string)

	ig, err := ignore.NewIgnore(rootPath)
	if err != nil {
		return nil, nil, err
	}

	// Use WalkDirGitIgnore to walk the directory tree while respecting gitignore
	err = ig.WalkDir(rootPath, func(path string, d os.DirEntry, isDir bool) error {
		items = append(items, item{
			Path:       path,
			IsDir:      isDir,
			TokenCount: 0,
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Sort items by path for consistent listing
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})

	// Build childrenMap (immediate children only)
	for _, it := range items {
		if it.IsDir {
			parentPath := it.Path
			var childList []string
			for _, possibleChild := range items {
				if filepath.Dir(possibleChild.Path) == parentPath && possibleChild.Path != parentPath {
					childList = append(childList, possibleChild.Path)
				}
			}
			childrenMap[parentPath] = childList
		}
	}

	return items, childrenMap, nil
}

// GenerateDirectoryTree writes a tree-like directory structure to w based on dt.
func (dt *DirectoryTree) GenerateDirectoryTree(w io.Writer) error {
	type treeNode struct {
		path     string
		name     string
		isDir    bool
		children []*treeNode
	}

	// Create a map to store nodes by path
	nodeMap := make(map[string]*treeNode)

	// Create the root node
	rootName := filepath.Base(dt.RootPath)
	rootNode := &treeNode{
		path:     dt.RootPath,
		name:     rootName,
		isDir:    true,
		children: []*treeNode{},
	}
	nodeMap[dt.RootPath] = rootNode

	// Process dt.Items to build the tree
	for _, item := range dt.Items {
		// Skip the root itself
		if item.Path == dt.RootPath {
			continue
		}
		name := filepath.Base(item.Path)
		parent := filepath.Dir(item.Path)

		if _, ok := nodeMap[item.Path]; !ok {
			nodeMap[item.Path] = &treeNode{
				path:     item.Path,
				name:     name,
				isDir:    item.IsDir,
				children: []*treeNode{},
			}
		}
		// Add this node to its parent's children
		if parentNode, ok := nodeMap[parent]; ok {
			if childNode, ok := nodeMap[item.Path]; ok {
				parentNode.children = append(parentNode.children, childNode)
			}
		}
	}

	// sort + write
	var writeTreeNode func(node *treeNode, prefix string, isLast bool) error
	writeTreeNode = func(node *treeNode, prefix string, isLast bool) error {
		sort.Slice(node.children, func(i, j int) bool {
			// Directories first, then files
			if node.children[i].isDir != node.children[j].isDir {
				return node.children[i].isDir
			}
			return node.children[i].name < node.children[j].name
		})
		if node.path == dt.RootPath {
			absPath, err := filepath.Abs(dt.RootPath)
			if err != nil {
				absPath = dt.RootPath
			}
			_, err = fmt.Fprintln(w, absPath)
			if err != nil {
				return err
			}
		} else {
			connector := "├── "
			if isLast {
				connector = "└── "
			}
			_, err := fmt.Fprintln(w, prefix+connector+node.name)
			if err != nil {
				return err
			}
		}
		for i, child := range node.children {
			isLastChild := i == len(node.children)-1
			newPrefix := prefix
			if node.path != dt.RootPath {
				if isLast {
					newPrefix += "    "
				} else {
					newPrefix += "│   "
				}
			}
			if err := writeTreeNode(child, newPrefix, isLastChild); err != nil {
				return err
			}
		}
		return nil
	}

	return writeTreeNode(rootNode, "", true)
}

// SelectFn is a function type that selects file paths based on a pattern
type SelectFn func(paths []string, pattern string) ([]string, error)

// SelectAllFiles returns all non-directory file paths
func (dt *DirectoryTree) SelectAllFiles() []string {
	var filePaths []string
	for _, it := range dt.Items {
		if !it.IsDir {
			filePaths = append(filePaths, it.Path)
		}
	}
	return filePaths
}

// SelectFilesByPattern returns file paths matching a pattern
// If pattern is empty, returns all paths
// If pattern starts with '/', treats it as a regex pattern
// Otherwise uses fuzzy matching
func (dt *DirectoryTree) SelectFilesByPattern(pattern string) ([]string, error) {
	filePaths := dt.SelectAllFiles()
	return selectSinglePattern(filePaths, pattern)
}

// SelectByPatterns applies multiple patterns in sequence to filter the directory tree files.
// It starts with all files, then for each pattern in patterns, we either intersect
// (for normal patterns) or exclude (for negative patterns) the matches.
//
// Negative patterns start with '!' (e.g. "!_test.go"), which means "filter out
// anything matching _test.go". Otherwise we keep only the matches.
func (dt *DirectoryTree) SelectByPatterns(patterns []string) ([]string, error) {
	// Start from all non-directory files
	allFiles := dt.SelectAllFiles()
	return selectByPatterns(allFiles, patterns)
}

// SelectRegexFiles returns file paths matching a regex pattern
func (dt *DirectoryTree) SelectRegexFiles(pattern string) ([]string, error) {
	filePaths := dt.SelectAllFiles()
	// Ensure the pattern starts with '/' for regex
	if !strings.HasPrefix(pattern, "/") {
		pattern = "/" + pattern
	}
	return selectSinglePattern(filePaths, pattern)
}

// selectSinglePattern is a helper function to select file paths based on a pattern
// If pattern is empty, returns all paths
// If pattern starts with '!', negates the pattern (excludes matches)
// If pattern starts with '/', treats it as a regex pattern
// If pattern starts with './', strips the prefix for matching
// If pattern starts with '../', returns an error
// Otherwise uses fuzzy matching
// If pattern contains '|', it splits the pattern and applies each part as a filter (logical AND)
func selectSinglePattern(paths []string, pattern string) ([]string, error) {
	// Empty pattern selects all files
	if pattern == "" {
		return paths, nil
	}

	// Check if this is a compound pattern with '|' operator (logical AND)
	if strings.Contains(pattern, "|") {
		parts := strings.Split(pattern, "|")
		currentPaths := paths
		var err error

		for _, part := range parts {
			// Apply each pattern part sequentially, narrowing down the results
			currentPaths, err = selectSinglePattern(currentPaths, part)
			if err != nil {
				return nil, fmt.Errorf("in pattern part '%s': %v", part, err)
			}
		}
		return currentPaths, nil
	}

	// Check if this is a negation pattern
	isNegation := strings.HasPrefix(pattern, "!")
	if isNegation {
		pattern = pattern[1:] // Remove the leading "!"
		// Empty pattern after negation would match everything, which would exclude everything
		if pattern == "" {
			return nil, fmt.Errorf("empty negation pattern '!' is not valid")
		}
	}

	// Reject patterns starting with "../" as they are potentially dangerous
	if strings.HasPrefix(pattern, "../") {
		return nil, fmt.Errorf("patterns with '../' are not supported for security reasons")
	}

	// Strip "./" prefix if present
	if strings.HasPrefix(pattern, "./") {
		pattern = pattern[2:] // Remove the leading "./"
	}

	// Create sets for the input paths and matches
	pathsSet := NewSetFromSlice(paths)
	matchesSet := NewSet[string]()

	// Find matches based on the pattern type
	if strings.HasPrefix(pattern, "/") {
		// Regex pattern
		regexPattern := pattern[1:] // Remove the leading '/'
		regex, err := regexp.Compile(regexPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %v", err)
		}

		for _, path := range paths {
			if regex.MatchString(path) {
				matchesSet.Add(path)
			}
		}
	} else {
		// Fuzzy matching
		fuzzyMatches := fuzzy.Find(pattern, paths)
		for _, match := range fuzzyMatches {
			matchesSet.Add(paths[match.Index])
		}
	}

	// If this is a negation pattern, return paths that don't match
	if isNegation {
		resultSet := pathsSet.Difference(matchesSet)
		return resultSet.Values(), nil
	}

	return matchesSet.Values(), nil
}

// selectByPatterns collects matches from multiple patterns
func selectByPatterns(paths []string, patterns []string) ([]string, error) {
	// Create an empty result set
	resultSet := NewSet[string]()

	// Process each pattern sequentially
	for _, pattern := range patterns {
		// For positive patterns, select matching paths to add to result set
		matches, err := selectSinglePattern(paths, pattern)
		if err != nil {
			return nil, fmt.Errorf("pattern '%s': %v", pattern, err)
		}

		// Add matches to the result set
		resultSet.AddValues(matches)
	}

	// Return the values from the set
	return resultSet.Values(), nil
}
