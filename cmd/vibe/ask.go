package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/pkoukk/tiktoken-go"
)

// AskCmd contains the arguments for the 'ask' subcommand
type AskCmd struct {
	TokenEstimator string   `arg:"--token-estimator" help:"Token count estimator to use: 'simple' (size/4) or 'tiktoken'" default:"simple"`
	All            bool     `arg:"-a,--all" help:"Select all files and output immediately"`
	Copy           bool     `arg:"-c,--copy" help:"Copy output to clipboard instead of stdout"`
	Role           string   `arg:"--role" help:"Role/layout to use for output"`
	Select         []string `arg:"--select,separate" help:"Select files matching pattern and output immediately (can be specified multiple times). Use fuzzy match by default or regex pattern with a '/' prefix"`
	Instruction    string   `arg:"positional" help:"User instruction or path to instruction file"`
}

// Merge merges src fields into the current AskCmd instance, with the current instance
// having precedence for any non-empty value or true boolean. So if this.All is already true,
// it stays true. If this.All is false, we take src's value. Same pattern
// for the rest.
func (cmd *AskCmd) Merge(src *AskCmd) {
	if src == nil {
		return
	}

	// If TokenEstimator is empty, overwrite it
	if cmd.TokenEstimator == "" {
		cmd.TokenEstimator = src.TokenEstimator
	}
	// Booleans: once set to true, keep them
	cmd.All = cmd.All || src.All
	cmd.Copy = cmd.Copy || src.Copy

	// Strings: if empty, overwrite
	if cmd.Role == "" {
		cmd.Role = src.Role
	}

	// Strings: if empty, overwrite
	if len(cmd.Select) == 0 {
		cmd.Select = src.Select
	}
	if cmd.Instruction == "" {
		cmd.Instruction = src.Instruction
	}
}

//go:embed repoprompt-diff.md
var diffPrompt string

//go:embed diff-heredoc.md
var diffHeredocPrompt string

// AskRunner encapsulates the state and behavior for the file picker
type AskRunner struct {
	Args           AskCmd
	RootPath       string
	DirTree        *DirectoryTree
	TokenEstimator TokenEstimator
	Instruct       *Instruct
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

	r := &AskRunner{
		Args:           cmdArgs,
		RootPath:       rootPath,
		TokenEstimator: tokenEstimator,
	}

	parser := NewInstructParser()
	instruct, err := parser.Parse(cmdArgs.Instruction)
	if err != nil {
		return nil, fmt.Errorf("failed to parse instruction: %v", err)
	}
	r.Instruct = instruct

	return r, nil
}

// Run executes the file picking process
func (r *AskRunner) Run() error {
	// Gather files/dirs
	var err error
	r.DirTree, err = LoadDirectoryTree(r.RootPath)
	if err != nil {
		return fmt.Errorf("failed to load directory tree: %v", err)
	}

	// Filter phase: select files either automatically or interactively
	selectedFiles, fileSelections, err := r.filterFiles()
	if err != nil {
		return err
	}

	// If we have FileSelections from TOML, output partial files directly
	if fileSelections != nil && len(fileSelections) > 0 {
		err := r.handleOutput(fileSelections)
		if err != nil {
			return err
		}
		return nil
	}

	// If no files were selected (user aborted), return early
	if len(selectedFiles) == 0 {
		fmt.Println("No files selected. Aborting.")
		return nil
	}

	// Convert selectedFiles to FileSelection
	fileSelections = make([]FileSelection, len(selectedFiles))
	for i, path := range selectedFiles {
		fileSelections[i] = FileSelection{
			Path:   path,
			Ranges: nil, // selected all lines
		}
	}

	// Output phase: generate user instruction and handle output
	if err := r.handleOutput(fileSelections); err != nil {
		return err
	}

	// // Calculate and report token count after output is handled
	// totalTokenCount, err := calculateTokenCount(selectedFiles, r.TokenEstimator)
	// if err != nil {
	// 	return fmt.Errorf("error calculating token count: %v", err)
	// }

	// // Print total token count to stderr
	// fmt.Fprintf(os.Stderr, "Total tokens: %d\n", totalTokenCount)

	return nil
}

// filterFiles handles the file selection phase, either automatically or interactively
// Returns either FileSelections (if using TOML front matter) or a list of file paths
func (r *AskRunner) filterFiles() ([]string, []FileSelection, error) {
	var selectedFiles []string
	var err error

	// If we have file selections from the parsed instruction, return them directly
	if r.Instruct != nil {
		fs, err := r.Instruct.Header.FileSelectionsWithDirTree(r.DirTree)
		return nil, fs, err
	}

	// Process front matter if present
	if r.Instruct != nil && r.Instruct.FrontMatter != nil && r.Instruct.FrontMatter.Tag != "toml" {
		// Parse flags from front matter and merge with command args
		// Note: This is kept for backward compatibility but could be deprecated
		cmd, err := parseFlags([]byte(r.Instruct.FrontMatter.Content))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse front matter flags: %w", err)
		}
		r.Args.Merge(cmd)
	}

	// Continue with regular file selection
	if r.Args.All {
		// Select all files
		selectedFiles = r.DirTree.SelectAllFiles()
	} else if len(r.Args.Select) > 0 {
		// Stepwise narrowing using multiple patterns, with support for negative patterns
		selectedFiles, err = r.DirTree.SelectByPatterns(r.Args.Select)
		if err != nil {
			return nil, nil, fmt.Errorf("error selecting files with patterns %v: %w", r.Args.Select, err)
		}
	}

	// else {
	// 	// Interactive selection
	// 	selectedFiles, _, err = selectFilesInteractively(r.DirTree, r.TokenEstimator)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// 	if selectedFiles == nil {
	// 		return nil, nil, nil
	// 	}
	// }
	if err != nil {
		return nil, nil, err
	}
	return selectedFiles, nil, nil
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
func (r *AskRunner) handleOutput(selectedFiles []FileSelection) error {
	// Create a new vibe context for rendering
	vibeCtx, err := NewVibeContext(r)
	if err != nil {
		return fmt.Errorf("failed to create vibe context: %v", err)
	}

	var buf bytes.Buffer
	out := io.Writer(os.Stdout)
	if r.Args.Copy {
		out = &buf
	}

	role := r.Args.Role
	if role == "" {
		role = "coder"
	}
	wrappedRole := "<" + role + ">"

	err = vibeCtx.WriteFileSelections(out, r.Instruct.UserContent, wrappedRole, selectedFiles)
	if err != nil {
		return err
	}

	if r.Args.Copy {
		if err := clipboard.WriteAll(buf.String()); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %v", err)
		}
		fmt.Fprintln(os.Stderr, "Output copied to clipboard")
	}

	return nil
}

// findRepoRoot returns the path to the repository root by looking for a .git directory.
// It starts from the given directory and moves up until it finds .git or reaches the filesystem root.
func findRepoRoot(startPath string) (string, error) {
	currentPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	for {
		gitPath := filepath.Join(currentPath, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			return currentPath, nil
		}

		// Check if we've reached the filesystem root
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// We've reached the root without finding .git
			return "", fmt.Errorf("no .git directory found up to filesystem root")
		}

		currentPath = parentPath
	}
}

// loadVibeFiles loads .vibe.md files from the current directory up to the repo root.
// It returns the content of all found .vibe.md files concatenated in order from repo root down to current dir.
func loadVibeFiles(startPath string) (string, error) {
	// Find the repo root
	repoRoot, err := findRepoRoot(startPath)
	if err != nil {
		// Not finding a repo root is not a fatal error, we just won't load .vibe.md files
		return "", nil
	}

	// Build the list of directories from repo root to current dir
	var dirs []string
	currentPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	// Start with the repo root
	dirs = append(dirs, repoRoot)

	// Add all directories from repo root down to current dir (if different)
	if currentPath != repoRoot {
		rel, err := filepath.Rel(repoRoot, currentPath)
		if err != nil {
			return "", err
		}

		parts := strings.Split(rel, string(filepath.Separator))
		path := repoRoot
		for _, part := range parts {
			path = filepath.Join(path, part)
			if path != repoRoot { // Don't add repo root twice
				dirs = append(dirs, path)
			}
		}
	}

	// Load .vibe.md files from each directory
	var content strings.Builder
	for _, dir := range dirs {
		vibePath := filepath.Join(dir, ".vibe.md")
		if info, err := os.Stat(vibePath); err == nil && !info.IsDir() {
			data, err := os.ReadFile(vibePath)
			if err != nil {
				return "", fmt.Errorf("error reading %s: %v", vibePath, err)
			}

			// find relative path to repo root
			rel, err := filepath.Rel(repoRoot, dir)
			if err != nil {
				return "", fmt.Errorf("error finding relative path: %v", err)
			}
			relPath := filepath.Join(rel, ".vibe.md")

			content.WriteString("<!-- " + relPath + " -->\n")
			content.Write(data)
			content.WriteString("\n\n")
		}
	}

	return content.String(), nil
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
