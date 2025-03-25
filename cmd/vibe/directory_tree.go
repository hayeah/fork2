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

// selectPattern is a helper function to select file paths based on a pattern
// If pattern is empty, returns all paths
// If pattern starts with '/', treats it as a regex pattern
// If pattern starts with './', strips the prefix for matching
// If pattern starts with '../', returns an error
// Otherwise uses fuzzy matching
func selectPattern(paths []string, pattern string) ([]string, error) {
	// Empty pattern selects all files
	if pattern == "" {
		return paths, nil
	}

	// Reject patterns starting with "../" as they are potentially dangerous
	if strings.HasPrefix(pattern, "../") {
		return nil, fmt.Errorf("patterns with '../' are not supported for security reasons")
	}

	// Strip "./" prefix if present
	if strings.HasPrefix(pattern, "./") {
		pattern = pattern[2:] // Remove the leading "./"
	}

	// Regex pattern starts with '/'
	if strings.HasPrefix(pattern, "/") {
		regexPattern := pattern[1:] // Remove the leading '/'
		regex, err := regexp.Compile(regexPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %v", err)
		}

		var selected []string
		for _, path := range paths {
			if regex.MatchString(path) {
				selected = append(selected, path)
			}
		}
		return selected, nil
	}

	// Otherwise use fuzzy matching
	matches := fuzzy.Find(pattern, paths)
	var selected []string
	for _, match := range matches {
		selected = append(selected, paths[match.Index])
	}
	return selected, nil
}

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
	return selectPattern(filePaths, pattern)
}

// selectByPatterns applies multiple patterns in sequence (a "pattern pipeline").
// It starts with all paths, then for each pattern in patterns, we either intersect
// (for normal patterns) or exclude (for negative patterns) the matches.
//
// Negative patterns start with '!' (e.g. "!_test.go"), which means "filter out
// anything matching _test.go". Otherwise we keep only the matches.
func selectByPatterns(paths []string, patterns []string) ([]string, error) {
	currentSet := paths

	for _, pat := range patterns {
		negate := false
		pattern := pat
		if strings.HasPrefix(pat, "!") {
			negate = true
			pattern = strings.TrimPrefix(pat, "!")
		}

		matched, err := selectPattern(currentSet, pattern)
		if err != nil {
			return nil, err
		}

		if negate {
			// Exclude matched from currentSet
			m := make(map[string]bool, len(currentSet))
			for _, p := range currentSet {
				m[p] = true
			}
			for _, p := range matched {
				delete(m, p)
			}
			var newSet []string
			for p := range m {
				newSet = append(newSet, p)
			}
			currentSet = newSet
		} else {
			// Intersect matched with currentSet
			m := make(map[string]bool, len(matched))
			for _, p := range matched {
				m[p] = true
			}
			var newSet []string
			for _, p := range currentSet {
				if m[p] {
					newSet = append(newSet, p)
				}
			}
			currentSet = newSet
		}
	}

	return currentSet, nil
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
	return selectPattern(filePaths, pattern)
}
