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

	"github.com/hayeah/fork2/render"
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

	// Create a template object to handle frontmatter
	tmpl, err := render.NewTemplate(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	// Replace commit placeholder with current commit hash
	commitHash := currentCommit()
	tmpl.RawFrontMatter = replaceCommitPlaceholder(tmpl.RawFrontMatter, commitHash)

	// Rebuild the full file contents
	if tmpl.RawFrontMatter != "" {
		delimiter := "```"
		tag := "toml"

		// Wrap the updated front-matter with the original delimiter
		templateContent = delimiter + tag + "\n" + tmpl.RawFrontMatter + "\n" + delimiter + "\n" + tmpl.Body
	} else {
		// No front-matter, keep the body verbatim
		templateContent = tmpl.Body
	}

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
// injects one at the end when missing.
func replaceCommitPlaceholder(content, hash string) string {
	// Regular expression that matches either:
	//   commit = "deadbeef"
	//   # commit = "deadbeef"
	reCommit := regexp.MustCompile(`\s*(#\s*)?commit\s*=\s*".*"`)

	// Find the first occurrence of a commit placeholder
	loc := reCommit.FindStringIndex(content)

	if loc != nil {
		// Extract the parts before and after the match
		before := content[:loc[0]]
		after := content[loc[1]:]

		// Remove any additional commit placeholders from the after part
		after = reCommit.ReplaceAllString(after, "")

		// Create the new commit line
		commitLine := fmt.Sprintf("\ncommit = \"%s\"", hash)

		// Combine the parts
		return before + commitLine + after
	}

	// No commit placeholder found, append at the end
	return content + "\n commit = \"" + hash + "\""
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
