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

	"github.com/alexflint/go-arg"
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
	Args            AskCmd
	RootPath        string
	DirTree         *DirectoryTree
	TokenEstimator  TokenEstimator
	UserInstruction string
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

	// Parse front matter from instruction
	userInstruction, err := parseInstructionWithFrontMatter(&cmdArgs)
	if err != nil {
		return nil, err
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
		Args:            cmdArgs,
		RootPath:        rootPath,
		TokenEstimator:  tokenEstimator,
		UserInstruction: userInstruction,
	}, nil
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

	if r.Args.All {
		// Select all files
		selectedFiles = r.DirTree.SelectAllFiles()
	} else if len(r.Args.Select) > 0 {
		// Stepwise narrowing using multiple patterns, with support for negative patterns
		selectedFiles, err = r.DirTree.SelectByPatterns(r.Args.Select)
		if err != nil {
			return nil, fmt.Errorf("error selecting files with patterns %v: %w", r.Args.Select, err)
		}
		err = nil
		err = nil
	} else {
		// Interactive selection
		selectedFiles, _, err = selectFilesInteractively(r.DirTree, r.TokenEstimator)
		if err != nil {
			return nil, err
		}
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
	// Create a new VibeContext
	vibeCtx, err := NewVibeContext(r)
	if err != nil {
		return fmt.Errorf("failed to create vibe context: %v", err)
	}

	var buf bytes.Buffer
	var out io.Writer
	out = os.Stdout
	if r.Args.Copy {
		out = &buf
	}

	role := r.Args.Role
	if role == "" {
		role = "coder"
	}

	// Wrap the specified role in "<...>"
	wrappedRole := "<" + role + ">"

	// Use VibeContext.WriteOutput
	err = vibeCtx.WriteOutput(out, r.Args.Instruction, wrappedRole, selectedFiles)
	if err != nil {
		return err
	}

	// Handle output based on --copy flag
	if r.Args.Copy {
		// Copy buffer contents to clipboard
		err = clipboard.WriteAll(buf.String())
		if err != nil {
			return fmt.Errorf("failed to copy to clipboard: %v", err)
		}

		fmt.Fprintln(os.Stderr, "Output copied to clipboard")
		return nil
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

// parseFrontMatter attempts to parse front matter from data. It returns a *AskCmd
// and the remainder of the data after the front matter. If there's no front matter,
// returns nil, original data, no error. If front matter is found but not properly
// closed, returns an error.
func parseFrontMatter(data []byte) (*AskCmd, []byte, error) {
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 {
		return nil, data, nil
	}

	// Check if the first line is '---' or '+++'
	delimiter := ""
	if bytes.Equal(lines[0], []byte("---")) || bytes.Equal(lines[0], []byte("+++")) {
		delimiter = string(lines[0])
	} else {
		// no front matter
		return nil, data, nil
	}

	// Find the closing delimiter
	var frontMatterLines []byte
	foundEnd := false
	i := 1
	for ; i < len(lines); i++ {
		if bytes.Equal(lines[i], []byte(delimiter)) {
			foundEnd = true
			break
		}
		frontMatterLines = append(frontMatterLines, lines[i]...)
		frontMatterLines = append(frontMatterLines, '\n')
	}

	if !foundEnd {
		return nil, nil, fmt.Errorf("front matter not closed with %s", delimiter)
	}

	parsedCmd, err := parseFlags(frontMatterLines)
	if err != nil {
		return nil, nil, err
	}

	// remainder is everything after the closing delimiter line
	remainder := bytes.Join(lines[i+1:], []byte("\n"))
	return parsedCmd, remainder, nil
}

// parseFlags interprets lines of text as flags for AskCmd using go-arg library
// by temporarily overriding os.Args.
func parseFlags(frontMatter []byte) (*AskCmd, error) {
	cmd := &AskCmd{}

	// Convert front matter to a single line of args
	var args []string
	allLines := bytes.Split(frontMatter, []byte("\n"))
	for _, line := range allLines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// A line might have multiple flags: e.g. "--copy --diff"
		parts := strings.Fields(string(line))
		args = append(args, parts...)
	}

	// Skip parsing if no args
	if len(args) == 0 {
		return cmd, nil
	}

	// Temporarily save original args
	originalArgs := os.Args
	defer func() {
		// Restore original args
		os.Args = originalArgs
	}()

	// Override with our custom args (preserving program name as first arg)
	os.Args = append([]string{originalArgs[0]}, args...)

	// Use go-arg to parse the flags into our struct
	parser, err := arg.NewParser(arg.Config{}, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %v", err)
	}

	if err := parser.Parse(os.Args); err != nil {
		return nil, fmt.Errorf("failed to parse front matter flags: %v", err)
	}

	return cmd, nil
}

// parseInstructionWithFrontMatter parses the instruction from a file or string,
// extracts any front matter, and returns the remaining instruction content.
// It modifies the provided AskCmd with any flags found in the front matter.
func parseInstructionWithFrontMatter(cmdArgs *AskCmd) (string, error) {
	if cmdArgs.Instruction == "" {
		return "", nil
	}

	// First step: Read the instruction content (from file or use as-is)
	instructionContent, err := readInstructionContent(cmdArgs.Instruction)
	if err != nil {
		return "", err
	}

	// Second step: Parse front matter from the content
	frontCmd, remainder, err := parseFrontMatter(instructionContent)
	if err != nil && frontCmd == nil {
		// invalid front matter => error
		return "", err
	}

	// Apply any front matter flags to command args
	if frontCmd != nil {
		cmdArgs.Merge(frontCmd)
	}

	return string(remainder), nil
}

// readInstructionContent reads the instruction content from a file if the path exists,
// otherwise returns the instruction string as-is
func readInstructionContent(instruction string) ([]byte, error) {
	// Check if the instruction is a file path
	fileInfo, err := os.Stat(instruction)
	if err == nil && !fileInfo.IsDir() {
		// It's a file, read its content
		content, err := os.ReadFile(instruction)
		if err != nil {
			return nil, fmt.Errorf("failed to read instruction file: %v", err)
		}
		return content, nil
	}

	// It's not a file, return the instruction string itself
	return []byte(instruction), nil
}
