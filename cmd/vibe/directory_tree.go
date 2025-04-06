package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hayeah/fork2/ignore"
)

// DirectoryTree holds directory listing info.
type DirectoryTree struct {
	RootPath string
	Items    []item
}

// LoadDirectoryTree constructs a new DirectoryTree by walking rootPath.
func LoadDirectoryTree(rootPath string) (*DirectoryTree, error) {
	var items []item

	ig, err := ignore.NewIgnore(rootPath)
	if err != nil {
		return nil, err
	}

	// Use WalkDirGitIgnore to walk the directory tree while respecting gitignore
	// The paths are walked in lexical order (directories first, then files).
	err = ig.WalkDir(rootPath, func(path string, d os.DirEntry, isDir bool) error {
		// If rootPath is an absolute path, path is also an absolute path.

		// Convert path to a relative path to rootPath
		relPath, err := filepath.Rel(rootPath, path)
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
	if err != nil {
		return nil, err
	}

	return &DirectoryTree{
		RootPath: rootPath,
		Items:    items,
	}, nil
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

// GenerateDirectoryTree writes a tree-like directory structure to w based on dt.
func (dt *DirectoryTree) GenerateDirectoryTree(w io.Writer) error {
	diagram := NewDirectoryTreeDiagram(dt)
	return diagram.Generate(w)
}

// DirectoryTreeDiagram handles the generation of tree diagrams from directory items.
type DirectoryTreeDiagram struct {
	RootPath string
	Items    []item
}

// NewDirectoryTreeDiagram creates a new DirectoryTreeDiagram from a DirectoryTree.
func NewDirectoryTreeDiagram(dt *DirectoryTree) *DirectoryTreeDiagram {
	return &DirectoryTreeDiagram{
		RootPath: dt.RootPath,
		Items:    dt.Items,
	}
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
