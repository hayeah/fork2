package main

import (
	_ "embed" // Used for go:embed directive
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

//go:embed default_task_template.md
var defaultTemplate string

// NewCmd defines the command-line arguments for the new subcommand
type NewCmd struct {
	Copy   string `arg:"--copy" help:"Seed the new file from an existing template"`
	Target string `arg:"positional" help:"Either the task name in free-text, or a path ending in .md"`
}

// NewCmdRunner encapsulates the state and behavior for the new subcommand
type NewCmdRunner struct {
	Args     NewCmd
	RootPath string
}

// NewNewRunner creates and initializes a new NewCmdRunner
func NewNewRunner(cmdArgs NewCmd, rootPath string) (*NewCmdRunner, error) {
	if cmdArgs.Target == "" && cmdArgs.Copy == "" {
		return nil, fmt.Errorf("either a task name or --copy must be provided")
	}

	return &NewCmdRunner{
		Args:     cmdArgs,
		RootPath: rootPath,
	}, nil
}

// Run executes the new command process
func (r *NewCmdRunner) Run() error {
	// Determine the source template content
	var templateContent string
	var err error
	var outputDir string

	// Check if we're in copy mode or if the target is an existing file
	copyMode := r.Args.Copy != ""
	targetIsFile := fileExists(r.Args.Target)
	if targetIsFile && r.Args.Copy == "" {
		// If target is a file, use it directly as the destination
		copyMode = true
		r.Args.Copy = r.Args.Target
		r.Args.Target = ""
	}

	if copyMode {
		// Copy mode: read from the specified template
		templatePath := expandPath(r.Args.Copy)
		templateBytes, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %v", templatePath, err)
		}
		templateContent = string(templateBytes)
		outputDir = filepath.Dir(templatePath)
	} else {
		// Use the default embedded template
		templateContent = defaultTemplate
		outputDir = r.RootPath
	}

	// Replace commit placeholder with current commit hash
	commitHash := currentCommit()
	templateContent = replaceCommitPlaceholder(string(templateContent), commitHash)

	// Determine the destination file name and path
	var taskName string
	var destPath string
	timestamp := time.Now().Format("2006-01-02T15-04")

	if copyMode && r.Args.Target == "" {
		// If copy mode but generate a new timestamped file
		taskName = filepath.Base(r.Args.Copy)
		// Remove the .md extension if present
		taskName = strings.TrimSuffix(taskName, ".md")

	} else {
		// Generate filename based on task name
		taskName = r.Args.Target
	}

	// Trim any existing timestamp prefix (like 2025-05-17T11-43) from the task name
	timestampPattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}`)
	taskName = timestampPattern.ReplaceAllString(taskName, "")
	taskName = dasherize(taskName)

	fileName := fmt.Sprintf("%s.%s.md", timestamp, taskName)
	destPath = filepath.Join(outputDir, fileName)

	// Write the file
	err = os.WriteFile(destPath, []byte(templateContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write template file %s: %v", destPath, err)
	}

	fmt.Printf("✅ Created %s\n", destPath)
	return nil
}

// currentCommit returns the current git commit hash or "unknown" on failure
func currentCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// replaceCommitPlaceholder ensures the template carries the correct commit hash.
// It updates an existing commit key (commented or uncommented) or
// injects one in the right place when missing.
func replaceCommitPlaceholder(content, hash string) string {
	lines := strings.Split(content, "\n")

	// Regular expression that matches either:
	//   commit = "deadbeef"
	//   # commit = "deadbeef"
	reCommit := regexp.MustCompile(`^\s*(#\s*)?commit\s*=\s*".*"$`)

	// First pass – replace an existing commit key if we find one.
	for i, line := range lines {
		if reCommit.MatchString(line) {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				lines[i] = fmt.Sprintf("# commit = \"%s\"", hash)
			} else {
				lines[i] = fmt.Sprintf("commit = \"%s\"", hash)
			}
			return strings.Join(lines, "\n")
		}
	}

	// Second pass – locate the end of the front-matter block (if any).
	insertAt := 0 // default: top of file (no front-matter)
	for i, line := range lines {
		trim := strings.TrimSpace(line)

		// Front-matter lines: start with '#' OR contain '=' assignment.
		if trim == "" { // blank line ends the block
			insertAt = i
			break
		}
		if strings.HasPrefix(trim, "#") || strings.Contains(trim, "=") {
			insertAt = i + 1 // keep advancing while still inside block
			continue
		}
		break // hit non-front-matter line
	}

	// Build result with the new commit line inserted.
	newLines := append([]string{}, lines[:insertAt]...)
	newLines = append(newLines, fmt.Sprintf("# commit = \"%s\"", hash))

	// Preserve a blank separator if we are inserting inside a block
	// that is followed immediately by non-blank content.
	if insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) != "" {
		newLines = append(newLines, "")
	}
	newLines = append(newLines, lines[insertAt:]...)

	return strings.Join(newLines, "\n")
}

// dasherize converts a string to a file path friendly format
func dasherize(text string) string {
	// Replace non-alphanumeric characters with dashes
	re := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	dasherized := re.ReplaceAllString(text, "-")

	// Replace consecutive dashes with a single dash
	re = regexp.MustCompile(`-+`)
	dasherized = re.ReplaceAllString(dasherized, "-")

	// Remove leading and trailing dashes
	dasherized = strings.Trim(dasherized, "-")

	return dasherized
}

// expandPath expands ~ to the user's home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
