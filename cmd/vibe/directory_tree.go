package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

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

// SelectAllFiles returns all non-directory file paths
func (dt *DirectoryTree) SelectAllFiles() []string {
	var selected []string
	for _, it := range dt.Items {
		if !it.IsDir {
			selected = append(selected, it.Path)
		}
	}
	return selected
}

// SelectFuzzyFiles returns file paths matching a fuzzy pattern
func (dt *DirectoryTree) SelectFuzzyFiles(pattern string) ([]string, error) {
	var filePaths []string
	var fileItems []item
	for _, it := range dt.Items {
		if !it.IsDir {
			filePaths = append(filePaths, it.Path)
			fileItems = append(fileItems, it)
		}
	}
	matches := fuzzy.Find(pattern, filePaths)
	var selected []string
	for _, match := range matches {
		selected = append(selected, fileItems[match.Index].Path)
	}
	return selected, nil
}

// SelectRegexFiles returns file paths matching a regex pattern
func (dt *DirectoryTree) SelectRegexFiles(pattern string) ([]string, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}
	var selected []string
	for _, it := range dt.Items {
		if !it.IsDir && regex.MatchString(it.Path) {
			selected = append(selected, it.Path)
		}
	}
	return selected, nil
}
