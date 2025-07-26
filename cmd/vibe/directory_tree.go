package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	selection "github.com/hayeah/fork2/internal/selection"
	setpkg "github.com/hayeah/fork2/internal/set"

	"github.com/hayeah/fork2/ignore"
)

// DirectoryTree holds directory listing info.
type DirectoryTree struct {
	RootPath string
	dirItems func() ([]item, error) // Memoized function for walkItems
	fsys     fs.FS                  // File system to use for file operations
}

// NewDirectoryTree constructs a DirectoryTree for the given rootPath, but does not walk the directory yet.
func NewDirectoryTree(rootPath string) *DirectoryTree {
	dt := &DirectoryTree{
		RootPath: rootPath,
		fsys:     os.DirFS(rootPath),
	}
	dt.dirItems = sync.OnceValues(dt.dirItemsImpl)
	return dt
}

// dirItemsImpl is the actual implementation that walks the directory tree.
func (dt *DirectoryTree) dirItemsImpl() ([]item, error) {
	var items []item

	ig, err := ignore.NewIgnore(dt.RootPath)
	if err != nil {
		return nil, err
	}
	err = ig.WalkDir(dt.RootPath, func(path string, d os.DirEntry, isDir bool) error {
		relPath, err := filepath.Rel(dt.RootPath, path)
		if err != nil {
			return err
		}
		items = append(items, item{
			Path:       relPath,
			IsDir:      isDir,
			TokenCount: 0,
		})
		return nil
	})
	return items, err
}

// SelectAllFiles returns all non-directory file paths
func (dt *DirectoryTree) SelectAllFiles() []string {
	items, err := dt.dirItems()
	if err != nil {
		return nil
	}
	var filePaths []string
	for _, it := range items {
		if !it.IsDir {
			filePaths = append(filePaths, it.Path)
		}
	}
	return filePaths
}

// SelectFiles returns file selections for the given select string (no memoization).
func (dt *DirectoryTree) SelectFiles(selectString string) ([]selection.FileSelection, error) {
	set := selection.NewFileSelectionSet()
	if selectString != "" {
		matchers, err := selection.ParseMatchersFromString(selectString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse select string: %w", err)
		}

		allPaths := dt.SelectAllFiles()
		for _, matcher := range matchers {
			matchedPaths, err := matcher.Match(allPaths)
			if err != nil {
				return nil, err
			}
			for _, path := range matchedPaths {
				set.Add(selection.NewFileSelection(dt.fsys, path, nil))
			}
		}
	}
	return set.Values(), nil
}

// Filter returns the minimal set of items that contains every path
// matched by pattern plus all their ancestor directories.
func (dt *DirectoryTree) Filter(pattern string) ([]item, error) {
	if pattern == "" {
		return dt.dirItems()
	}

	// 1. build a matcher list (reuse ParseMatchersFromString from select.go)
	matchers, err := selection.ParseMatchersFromString(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pattern: %w", err)
	}

	// 2. collect all non-dir file paths, run every matcher, build a set.
	allItems, err := dt.dirItems()
	if err != nil {
		return nil, err
	}

	// Get all file paths (non-directories)
	var filePaths []string
	for _, it := range allItems {
		if !it.IsDir {
			filePaths = append(filePaths, it.Path)
		}
	}

	// Apply matchers to get matched paths
	matchedSet := setpkg.NewSet[string]()
	for _, matcher := range matchers {
		matchedPaths, err := matcher.Match(filePaths)
		if err != nil {
			return nil, err
		}
		matchedSet.AddValues(matchedPaths)
	}

	// 3. walk that set, add every ancestor dir to the set.
	pathSet := setpkg.NewSet[string]()
	for _, path := range matchedSet.Values() {
		// Add the file path itself
		pathSet.Add(path)

		// Add all ancestor directories
		current := path
		for {
			parent := filepath.Dir(current)
			if parent == "." || parent == current {
				break
			}
			pathSet.Add(parent)
			current = parent
		}
	}

	// 4. return only the items whose Path is in the set, preserving order.
	var filteredItems []item
	for _, it := range allItems {
		if pathSet.Contains(it.Path) || it.Path == "." || it.Path == "" {
			filteredItems = append(filteredItems, it)
		}
	}

	return filteredItems, nil
}

// GenerateDirectoryTree writes a tree-like directory structure to w based on dt.
func (dt *DirectoryTree) GenerateDirectoryTree(w io.Writer, pattern string) error {
	diagram, err := NewDirectoryTreeDiagram(dt, pattern)
	if err != nil {
		return err
	}
	return diagram.Generate(w)
}

// DirectoryTreeDiagram handles the generation of tree diagrams from directory items.
type DirectoryTreeDiagram struct {
	RootPath string
	Items    []item
}

// NewDirectoryTreeDiagram creates a new DirectoryTreeDiagram from a DirectoryTree.
func NewDirectoryTreeDiagram(dt *DirectoryTree, pattern string) (*DirectoryTreeDiagram, error) {
	var items []item
	var err error

	if pattern == "" {
		items, err = dt.dirItems()
	} else {
		items, err = dt.Filter(pattern)
	}

	if err != nil {
		return nil, err
	}

	return &DirectoryTreeDiagram{
		RootPath: dt.RootPath,
		Items:    items,
	}, nil
}

func (dtd *DirectoryTreeDiagram) Generate(w io.Writer) error {
	// treeNode represents a node in the directory tree.
	type treeNode struct {
		path     string // Relative path
		name     string
		isDir    bool
		children []*treeNode
	}

	// Create a map to hold nodes by their relative path.
	nodeMap := make(map[string]*treeNode)

	// Create the root node (represented as ".").
	rootNode := &treeNode{
		path:     ".",
		name:     filepath.Base(dtd.RootPath),
		isDir:    true,
		children: []*treeNode{},
	}
	nodeMap["."] = rootNode

	// Build the tree structure from the directory items.
	for _, item := range dtd.Items {
		if item.Path == "" || item.Path == "." {
			continue
		}
		name := filepath.Base(item.Path)
		parent := filepath.Dir(item.Path)
		if parent == "" || parent == "." {
			parent = "."
		}

		// Create the node if it doesn't exist.
		if _, ok := nodeMap[item.Path]; !ok {
			nodeMap[item.Path] = &treeNode{
				path:     item.Path,
				name:     name,
				isDir:    item.IsDir,
				children: []*treeNode{},
			}
		}
		// Append the node to its parent's children.
		if parentNode, ok := nodeMap[parent]; ok {
			parentNode.children = append(parentNode.children, nodeMap[item.Path])
		}
	}

	// Define the recursive function to write the tree.
	var writeTreeNode func(node *treeNode, prefix string, isLast bool) error
	writeTreeNode = func(node *treeNode, prefix string, isLast bool) error {
		// Print the root node with its absolute path.
		if node.path == "." {
			absPath, err := filepath.Abs(dtd.RootPath)
			if err != nil {
				absPath = dtd.RootPath
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

			// Add a trailing slash for directories
			displayName := node.name
			if node.isDir {
				displayName += "/"
			}

			_, err := fmt.Fprintln(w, prefix+connector+displayName)
			if err != nil {
				return err
			}
		}

		// Recursively print each child.
		for i, child := range node.children {
			isLastChild := i == len(node.children)-1
			newPrefix := prefix
			if node.path != "." {
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
