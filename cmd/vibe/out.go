package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/hayeah/fork2/internal/metrics"
	"github.com/hayeah/fork2/render"
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
	Template      string   `arg:"positional" help:"User instruction or path to instruction file"`
}

// OutRunner encapsulates the state and behavior for the file picker
type OutRunner struct {
	Args           OutCmd
	RootPath       string
	DirTree        *DirectoryTree
	TokenEstimator TokenEstimator
	Data           map[string]string
	Metrics        *metrics.OutputMetrics
}

// NewAskRunner creates and initializes a new PickRunner
func NewAskRunner(cmdArgs OutCmd, rootPath string) (*OutRunner, error) {
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

	// Parse data parameters (key=value pairs)
	data, err := parseDataParams(cmdArgs.Data)
	if err != nil {
		return nil, err
	}

	// Initialize metrics counter based on token estimator
	var counter metrics.Counter
	switch cmdArgs.TokenEstimator {
	case "tiktoken":
		counter, err = metrics.NewTiktokenCounter("gpt-3.5-turbo")
		if err != nil {
			// Fall back to simple counter on error
			counter = &metrics.SimpleCounter{}
		}
	default:
		counter = &metrics.SimpleCounter{}
	}

	r := &OutRunner{
		Args:           cmdArgs,
		RootPath:       rootPath,
		TokenEstimator: tokenEstimator,
		Data:           data,
		Metrics:        metrics.NewOutputMetrics(counter, runtime.NumCPU()),
	}

	return r, nil
}

// Run executes the file picking process
func (r *OutRunner) Run() error {
	// Gather files/dirs
	r.DirTree = NewDirectoryTree(r.RootPath)

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

	pipe, err := BuildOutPipeline(r.RootPath, r.Args)
	if err != nil {
		return err
	}

	tmpl, err := r.overrideTemplate(pipe)
	if err != nil {
		return err
	}
	pipe.Template = tmpl
	pipe.ContentSpecs = r.Args.Content
	pipe.DataPairs = r.Args.Data

	if err := pipe.Run(dest); err != nil {
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

// overrideTemplate loads the template and applies command-line overrides to its frontmatter.
func (r *OutRunner) overrideTemplate(pipe *OutPipeline) (*render.Template, error) {
	templatePath := r.Args.Template
	if templatePath == "" && r.Args.Select != "" {
		templatePath = "files"
	}
	templatePath = strings.TrimPrefix(templatePath, "./")

	tmpl, err := pipe.Renderer.LoadTemplate(templatePath)
	if err != nil {
		return nil, err
	}

	if r.Args.Layout != "" {
		tmpl.FrontMatter.Layout = r.Args.Layout
	}
	if r.Args.Select != "" {
		tmpl.FrontMatter.Select = r.Args.Select
	}
	if r.Args.SelectDirTree != "" {
		tmpl.FrontMatter.Dirtree = r.Args.SelectDirTree
	}

	if tmpl.FrontMatter.Layout == "" && tmpl.FrontMatter.Select != "" {
		tmpl.FrontMatter.Layout = "files"
	}

	return tmpl, nil
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
