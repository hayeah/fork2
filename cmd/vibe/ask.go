package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/atotto/clipboard"
	"github.com/hayeah/fork2"
	"github.com/pkoukk/tiktoken-go"
)

// AskCmd contains the arguments for the 'ask' subcommand
type AskCmd struct {
	TokenEstimator string `arg:"--token-estimator" help:"Token count estimator to use: 'simple' (size/4) or 'tiktoken'" default:"simple"`
	All            bool   `arg:"-a,--all" help:"Select all files and output immediately"`
	Copy           bool   `arg:"-c,--copy" help:"Copy output to clipboard instead of stdout"`
	Diff           bool   `arg:"--diff" help:"Enable diff output format"`
	Select         string `arg:"--select" help:"Select files matching fuzzy pattern and output immediately"`
	SelectRegex    string `arg:"--select-re" help:"Select files matching regex pattern and output immediately"`
	Instruction    string `arg:"positional" help:"User instruction or path to instruction file"`
}

//go:embed repoprompt-diff.md
var diffPrompt string

//go:embed diff-heredoc.md
var diffHeredocPrompt string

// AskRunner encapsulates the state and behavior for the file picker
type AskRunner struct {
	Args           AskCmd
	RootPath       string
	Items          []item
	ChildrenMap    map[string][]string
	TokenEstimator TokenEstimator
}

// NewAskRunner creates and initializes a new PickRunner
func NewAskRunner(cmdArgs AskCmd, rootPath string) (*AskRunner, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("error accessing %s: %v", rootPath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", rootPath)
	}

	// Select the token estimator based on the flag
	var tokenEstimator TokenEstimator
	switch cmdArgs.TokenEstimator {
	case "tiktoken":
		tokenEstimator = estimateTokenCountTiktoken
	case "simple":
		tokenEstimator = estimateTokenCountSimple
	default:
		return nil, fmt.Errorf("unknown token estimator: %s", cmdArgs.TokenEstimator)
	}

	return &AskRunner{
		Args:           cmdArgs,
		RootPath:       rootPath,
		TokenEstimator: tokenEstimator,
	}, nil
}

// Run executes the file picking process
func (r *AskRunner) Run() error {
	// Gather files/dirs
	var err error
	r.Items, r.ChildrenMap, err = gatherFiles(r.RootPath)
	if err != nil {
		return fmt.Errorf("failed to gather files: %v", err)
	}

	// Filter phase: select files either automatically or interactively
	selectedFiles, err := r.filterFiles()
	if err != nil {
		return err
	}

	// If no files were selected (user aborted), return early
	if selectedFiles == nil {
		return nil
	}

	// Output phase: generate user instruction and handle output
	if err := r.handleOutput(selectedFiles); err != nil {
		return err
	}

	// Calculate and report token count after output is handled
	totalTokenCount, err := calculateTokenCount(selectedFiles, r.TokenEstimator)
	if err != nil {
		return fmt.Errorf("error calculating token count: %v", err)
	}

	// Print total token count to stderr
	fmt.Fprintf(os.Stderr, "Total tokens: %d\n", totalTokenCount)

	return nil
}

// filterFiles handles the file selection phase, either automatically or interactively
func (r *AskRunner) filterFiles() ([]string, error) {
	var selectedFiles []string
	var err error

	// Define selector functions for different selection modes
	if r.Args.All {
		// Select all files
		selectedFiles, err = selectAllFiles(r.Items)
	} else if r.Args.Select != "" {
		// Select files matching fuzzy pattern
		pattern := r.Args.Select
		selectedFiles, err = selectFuzzyFiles(r.Items, pattern)
	} else if r.Args.SelectRegex != "" {
		// Select files matching regex pattern
		pattern := r.Args.SelectRegex
		selectedFiles, err = selectRegexFiles(r.Items, pattern)
	} else {
		// Interactive selection
		selectedFiles, _, err = selectFilesInteractively(r.Items, r.ChildrenMap, r.TokenEstimator)
		if err != nil {
			return nil, err
		}

		// If no files were selected (user aborted), return early
		if selectedFiles == nil {
			return nil, nil
		}
	}

	if err != nil {
		return nil, err
	}

	return selectedFiles, nil
}

// calculateTokenCount calculates the total token count for a list of file paths
func calculateTokenCount(filePaths []string, tokenEstimator TokenEstimator) (int, error) {
	totalTokenCount := 0

	for _, path := range filePaths {
		tokenCount, err := tokenEstimator(path)
		if err != nil {
			log.Printf("Error estimating tokens for %s: %v", path, err)
		} else {
			totalTokenCount += tokenCount
		}
	}

	return totalTokenCount, nil
}

// handleOutput processes the user instruction and outputs the result
func (r *AskRunner) handleOutput(selectedFiles []string) error {
	// Handle output based on --copy flag
	if r.Args.Copy {
		// Write to buffer and copy to clipboard
		var buf bytes.Buffer
		err := r.writeOutput(&buf, selectedFiles)
		if err != nil {
			return err
		}

		// Copy buffer contents to clipboard
		err = clipboard.WriteAll(buf.String())
		if err != nil {
			return fmt.Errorf("failed to copy to clipboard: %v", err)
		}

		fmt.Fprintln(os.Stderr, "Output copied to clipboard")
		return nil
	} else {
		// Write output to stdout
		return r.writeOutput(os.Stdout, selectedFiles)
	}
}

// generateUserInstruction creates the user instruction string
// If instructionArg is a readable file, it reads the content
// Otherwise, it uses the instructionArg as the instruction
func (r *AskRunner) generateUserInstruction() (string, error) {
	instructionArg := r.Args.Instruction
	if instructionArg == "" {
		return "", nil
	}

	// Check if the instruction is a file path
	fileInfo, err := os.Stat(instructionArg)
	if err == nil && !fileInfo.IsDir() {
		// It's a file, read its content
		content, err := os.ReadFile(instructionArg)
		if err != nil {
			return "", fmt.Errorf("failed to read instruction file: %v", err)
		}
		return "\n# User Instructions\n" + string(content) + "\n\n", nil
	}

	// It's not a file, use as-is
	return "\n# User Instructions\n" + instructionArg + "\n\n", nil
}

// writeOutput outputs the directory tree, file map, and token count
func (r *AskRunner) writeOutput(w io.Writer, selectedFiles []string) error {
	// Sort the selected files
	sort.Strings(selectedFiles)

	// If diff output is enabled, include the diff prompt at the beginning
	if r.Args.Diff {
		_, err := fmt.Fprint(w, diffHeredocPrompt)
		// _, err := fmt.Fprint(w, diffPrompt)
		if err != nil {
			return fmt.Errorf("failed to write diff prompt: %v", err)
		}
	}

	// Generate directory tree structure
	err := generateDirectoryTree(w, r.RootPath, r.Items)
	if err != nil {
		return fmt.Errorf("failed to generate directory tree: %v", err)
	}

	// Write the file map of selected files
	err = fork2.WriteFileMap(w, selectedFiles, r.RootPath)
	if err != nil {
		return fmt.Errorf("failed to write file map: %v", err)
	}

	// Generate and include user instruction if provided
	if r.Args.Instruction != "" {
		userInstruction, err := r.generateUserInstruction()
		if err != nil {
			return err
		}

		// Write the user instruction
		_, err = fmt.Fprintln(w, userInstruction)
		if err != nil {
			return fmt.Errorf("failed to write user instruction: %v", err)
		}
		// Add a blank line after the instruction
		_, err = fmt.Fprintln(w)
		if err != nil {
			return fmt.Errorf("failed to write newline: %v", err)
		}
	}

	// Add the reminder
	_, err = fmt.Fprintln(w, "IMPORTANT: Do not just write me the code. Output your response in the format described in the instructions. Quote the response as code for display.")
	if err != nil {
		return fmt.Errorf("failed to write reminder: %v", err)
	}

	return nil
}

// generateDirectoryTree creates a tree-like structure for the directory and writes it to the provided writer
// using the items already gathered by gatherFiles
func generateDirectoryTree(w io.Writer, rootPath string, items []item) error {
	type treeNode struct {
		path     string
		name     string
		isDir    bool
		children []*treeNode
	}

	// Create a map to store nodes by their path
	nodeMap := make(map[string]*treeNode)

	// Create the root node
	rootName := filepath.Base(rootPath)
	rootNode := &treeNode{
		path:     rootPath,
		name:     rootName,
		isDir:    true,
		children: []*treeNode{},
	}
	nodeMap[rootPath] = rootNode

	// Process the items to build the tree structure
	for _, item := range items {
		// Skip the root itself
		if item.Path == rootPath {
			continue
		}

		name := filepath.Base(item.Path)
		parent := filepath.Dir(item.Path)

		// Create a new node if it doesn't exist yet
		if _, exists := nodeMap[item.Path]; !exists {
			node := &treeNode{
				path:     item.Path,
				name:     name,
				isDir:    item.IsDir,
				children: []*treeNode{},
			}
			nodeMap[item.Path] = node
		}

		// Add this node to its parent's children
		if parentNode, ok := nodeMap[parent]; ok {
			if childNode, ok := nodeMap[item.Path]; ok {
				parentNode.children = append(parentNode.children, childNode)
			}
		}
	}

	// Write the opening file_map tag
	fmt.Fprintln(w, "<file_map>")

	// Function to recursively build the tree string
	var writeTreeNode func(node *treeNode, prefix string, isLast bool) error
	writeTreeNode = func(node *treeNode, prefix string, isLast bool) error {
		// Sort children by name
		sort.Slice(node.children, func(i, j int) bool {
			// Directories first, then files
			if node.children[i].isDir != node.children[j].isDir {
				return node.children[i].isDir
			}
			return node.children[i].name < node.children[j].name
		})

		// Add this node to the result
		if node.path == rootPath {
			// Use the absolute path for the root node
			absPath, err := filepath.Abs(rootPath)
			if err != nil {
				absPath = rootPath // Fallback to rootPath if Abs fails
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

		// Add children
		for i, child := range node.children {
			isLastChild := i == len(node.children)-1
			newPrefix := prefix
			if node.path != rootPath {
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

	// Write the tree structure
	if err := writeTreeNode(rootNode, "", true); err != nil {
		return err
	}

	// Write the closing file_map tag
	fmt.Fprintln(w, "</file_map>")

	return nil
}

// estimateTokenCountSimple estimates tokens using the simple size/4 method
func estimateTokenCountSimple(filePath string) (int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	// Simple estimation: 1 token per 4 characters
	return len(data) / 4, nil
}

// estimateTokenCountTiktoken estimates tokens using the tiktoken-go library
func estimateTokenCountTiktoken(filePath string) (int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	// Use tiktoken-go to count tokens
	tke, err := tiktoken.GetEncoding("cl100k_base") // Using the same encoding as GPT-4
	if err != nil {
		return 0, fmt.Errorf("failed to get tiktoken encoding: %v", err)
	}

	tokens := tke.Encode(string(data), nil, nil)
	return len(tokens), nil
}
