package fork2

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

//go:embed prompts
var promptFS embed.FS

// SystemInfo contains information about the system environment
type SystemInfo struct {
	OS                 string
	Shell              string
	HomeDir            string
	WorkingDir         string
	HomeDirPosix       string
	WorkingDirPosix    string
	EnvironmentDetails string
}

// getSystemInfo returns information about the system environment
func getSystemInfo() (SystemInfo, error) {
	// Get OS name
	osName := runtime.GOOS

	// Get shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		// Default shells based on OS
		if runtime.GOOS == "windows" {
			shell = "cmd.exe"
		} else {
			shell = "/bin/bash"
		}
	} else {
		shell = filepath.Base(shell)
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return SystemInfo{}, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Get current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		return SystemInfo{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Build environment details
	envDetails, err := BuildEnvironmentDetails(workingDir)
	if err != nil {
		return SystemInfo{}, fmt.Errorf("failed to build environment details: %w", err)
	}

	return SystemInfo{
		OS:                 osName,
		Shell:              shell,
		HomeDir:            homeDir,
		WorkingDir:         workingDir,
		EnvironmentDetails: envDetails,
	}, nil
}

// EnvironmentDetails contains data for the environment details template
type EnvironmentDetails struct {
	CurrentTime string
	WorkingDir  string
	Files       []string
}

// BuildEnvironmentDetails creates a string with environment details including
// current time, working directory files (respecting .gitignore), and current mode
func BuildEnvironmentDetails(workingDir string) (string, error) {
	// Prepare data
	data := EnvironmentDetails{
		WorkingDir: workingDir,
	}

	data.CurrentTime = time.Now().Format(time.RFC3339)

	// List files
	files, err := ListFilesRespectingGitIgnore(workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to list files: %w", err)
	}

	// Sort files alphabetically
	sort.Strings(files)
	data.Files = files

	return renderTemplate("prompts/environment_details.tmpl", "environment", data)
}

// renderTemplate loads a template from the embedded filesystem, parses it, and renders it with the provided data
func renderTemplate(templatePath, templateName string, data interface{}) (string, error) {
	// Load template
	tmplBytes, err := promptFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	// Parse the template
	tmpl, err := template.New(templateName).Parse(string(tmplBytes))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	// Render template
	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return sb.String(), nil
}

// ListFilesRespectingGitIgnore reads .gitignore patterns from dir/.gitignore
// and returns a slice of file paths that are not excluded by those patterns.
func ListFilesRespectingGitIgnore(dir string) ([]string, error) {
	// Create a filesystem for the directory
	fs := osfs.New(dir)
	// Read gitignore patterns using ReadPatterns
	patterns, err := gitignore.ReadPatterns(fs, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to read gitignore patterns: %w", err)
	}

	// Create a Matcher that can check if a file or directory is ignored
	matcher := gitignore.NewMatcher(patterns)

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Convert absolute path to a relative path, so we can feed it into the matcher
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Skip the root directory
		if relPath == "." {
			return nil
		}

		parts := strings.Split(relPath, string(os.PathSeparator))

		// If this is a directory (other than the root), check if it's ignored.
		// If ignored, skip descending into it.
		if info.IsDir() && path != dir {
			if matcher.Match(parts, true) {
				return filepath.SkipDir
			}
			return nil
		}

		// It's a file; skip if matched by .gitignore
		if !matcher.Match(parts, false) {
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return files, nil
}

// RenderPromptWithSystemInfo renders the prompt template with system information
func RenderPromptWithSystemInfo() (string, error) {
	sysInfo, err := getSystemInfo()
	if err != nil {
		return "", fmt.Errorf("failed to get system info: %w", err)
	}
	return renderTemplate("prompts/system.md", "prompt", sysInfo)
}
