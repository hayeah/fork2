package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/pkoukk/tiktoken-go"
)

// OutCmd contains the arguments for the 'out' subcommand
type OutCmd struct {
	TokenEstimator string `arg:"--token-estimator" help:"Token count estimator to use: 'simple' (size/4) or 'tiktoken'" default:"simple"`
	All            bool   `arg:"-a,--all" help:"Select all files and output immediately"`
	// Output sets the destination for the generated prompt: '-' for stdout, a file path to write the output, or empty to copy to clipboard
	Output        string   `arg:"-o,--output" help:"Output destination: '-' for stdout; file path to write; if not set, copy to clipboard"`
	Layout        string   `arg:"--layout" help:"Layout to use for output"`
	Select        string   `arg:"-s,--select" help:"Select files matching patterns"`
	SelectDirTree string   `arg:"-t,--dirtree" help:"Filter the directory-tree diagram with the same pattern syntax as --select"`
	Data          []string `arg:"-d,--data,separate" help:"key=value pairs exposed to templates as .Data.* (repeatable)"`
	Metrics       string   `arg:"-m,--metrics" help:"Write metrics JSON ('-' = stdout)"`
	Content       []string `arg:"-c,--content,separate" help:"Content source specifications: '-' for stdin, file paths, URLs, or literals (repeatable)"`
	Mode          string   `arg:"--mode,-m" help:"Template specialization mode"`
	Root          string   `arg:"-r,--root" help:"Path to repo root (default: .)"`
	Template      string   `arg:"positional" help:"User instruction or path to instruction file"`
	TemplatePaths []string // Additional paths to search for templates (not exposed as CLI arg)
}

// OutRunner encapsulates the state and behavior for the file picker
type OutRunner struct {
	Args     OutCmd
	Pipeline *OutPipeline
}

// Run executes the file picking process
func (r *OutRunner) Run() error {
	var buf bytes.Buffer
	var dest io.Writer
	switch {
	case r.Args.Output == "-":
		dest = os.Stdout
	case r.Args.Output != "":
		file, err := os.Create(r.Args.Output)
		if err != nil {
			return fmt.Errorf("failed to create output file %s: %v", r.Args.Output, err)
		}
		defer file.Close()
		dest = file
	default:
		dest = &buf
	}

	r.Pipeline.ContentSpecs = r.Args.Content

	if err := r.Pipeline.Run(dest); err != nil {
		return err
	}

	if r.Args.Output == "" {
		if err := clipboard.WriteAll(buf.String()); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %v", err)
		}
		fmt.Fprintln(os.Stderr, "Output copied to clipboard")
	}

	return nil
}

// parseDataParams parses data parameters from CLI flags into a map
// Each parameter can be a single key=value pair or URL-style query parameters (key1=val1&key2=val2)
// Supports both single "k=v" and "k1=v1&k2=v2" styles.
func parseDataParams(params []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, raw := range params {
		// ParseQuery handles splitting on '&' and '='
		vals, err := url.ParseQuery(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid data parameter %q: %w", raw, err)
		}

		for key, arr := range vals {
			// Take only the first value if there are duplicates
			if len(arr) > 0 {
				result[key] = arr[0]
			}
		}
	}
	return result, nil
}

// calculateTokenCount calculates the total token count for a list of file paths
func calculateTokenCount(fsys fs.FS, filePaths []string, tokenEstimator TokenEstimator) (int, error) {
	totalTokenCount := 0

	for _, path := range filePaths {
		tokenCount, err := tokenEstimator(fsys, path)
		if err != nil {
			log.Printf("Error estimating tokens for %s: %v", path, err)
		} else {
			totalTokenCount += tokenCount
		}
	}

	return totalTokenCount, nil
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
func estimateTokenCountSimple(fsys fs.FS, filePath string) (int, error) {
	data, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return 0, err
	}
	// Simple estimation: 1 token per 4 characters
	return len(data) / 4, nil
}

// estimateTokenCountTiktoken estimates tokens using the tiktoken-go library
func estimateTokenCountTiktoken(fsys fs.FS, filePath string) (int, error) {
	data, err := fs.ReadFile(fsys, filePath)
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
