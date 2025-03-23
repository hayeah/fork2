// Package render provides template rendering capabilities with support for
// partials from different sources (system, repo, local).
package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"text/template"
)

// RenderContext holds shared data needed by the templating system.
type RenderContext struct {
	// Path of the currently executing template, used for local partial lookups.
	CurrentTemplatePath string

	// FS containing system-level partials (e.g., <vibe/coder>)
	SystemPartials fs.FS

	// FS containing repository-level partials (e.g., @vibe/coder)
	RepoPartials fs.FS
}

// VibeRenderContext extends the base RenderContext to hold more data.
type VibeRenderContext struct {
	RenderContext

	ListDirectory    []string
	SelectedFiles    []string
	RepoInstructions map[string]string
	System           string
	Now              string
}

// PartialContext is the interface for any context that can render partials.
type PartialContext interface {
	Partial(partialPath string, data any) (string, error)
	ResolvePartialPath(partialPath string) (fs.FS, string, error)
}

// Render renders a layout template by wrapping the user's content in a "main" block.
// layoutPath indicates which layout template to use.
// userContentPath indicates the path to the user's content template.
func Render(ctx PartialContext, userContentPath string, layoutPath string) (string, error) {
	// Get the layout content
	layoutFS, layoutFile, err := ctx.ResolvePartialPath(layoutPath)
	if err != nil {
		return "", fmt.Errorf("error resolving layout path: %w", err)
	}

	layoutContent, err := readTemplate(layoutFS, layoutFile)
	if err != nil {
		return "", fmt.Errorf("error reading layout template: %w", err)
	}

	// Get the user content
	userFS, userFile, err := ctx.ResolvePartialPath(userContentPath)
	if err != nil {
		return "", fmt.Errorf("error resolving user content path: %w", err)
	}

	userContent, err := readTemplate(userFS, userFile)
	if err != nil {
		return "", fmt.Errorf("error reading user content template: %w", err)
	}

	// Wrap user content in "main" block
	wrappedContent := fmt.Sprintf(`{{ define "main" }}%s{{ end }}`, userContent)

	// Combine layout with wrapped content
	combinedContent := layoutContent + "\n" + wrappedContent

	// Create a template with a custom partial function
	tmpl := template.New("layout")
	tmpl = tmpl.Funcs(template.FuncMap{
		"partial": func(partialPath string) (string, error) {
			return ctx.Partial(partialPath, ctx)
		},
	})

	// Parse the combined content
	tmpl, err = tmpl.Parse(combinedContent)
	if err != nil {
		return "", fmt.Errorf("error parsing template: %w", err)
	}

	// Execute the template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, ctx)
	if err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}

// readTemplate reads a template file from the specified filesystem.
func readTemplate(fsys fs.FS, filename string) (string, error) {
	content, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// Partial locates and executes the partial specified by partialPath, returning rendered content.
func (ctx *RenderContext) Partial(partialPath string, data any) (string, error) {
	// Resolve the partial path to get the fs and file
	fs, file, err := ctx.ResolvePartialPath(partialPath)
	if err != nil {
		return "", fmt.Errorf("error resolving partial path: %w", err)
	}

	// Read the partial template
	partialContent, err := readTemplate(fs, file)
	if err != nil {
		return "", fmt.Errorf("error reading partial template: %w", err)
	}

	// Store original current template path
	originalPath := ctx.CurrentTemplatePath

	// Update current template path for local partial resolution within this partial
	ctx.CurrentTemplatePath = file

	// Create a template with a custom partial function
	tmpl := template.New(file)
	tmpl = tmpl.Funcs(template.FuncMap{
		"partial": func(nestedPartialPath string) (string, error) {
			return ctx.Partial(nestedPartialPath, data)
		},
	})

	// Parse the partial content
	tmpl, err = tmpl.Parse(partialContent)
	if err != nil {
		// Restore original path before returning error
		ctx.CurrentTemplatePath = originalPath
		return "", fmt.Errorf("error parsing partial template: %w", err)
	}

	// Execute the template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		// Restore original path before returning error
		ctx.CurrentTemplatePath = originalPath
		return "", fmt.Errorf("error executing partial template: %w", err)
	}

	// Restore original path
	ctx.CurrentTemplatePath = originalPath

	return buf.String(), nil
}

// ResolvePartialPath determines which FS and file should be used for a given partial path.
func (ctx *RenderContext) ResolvePartialPath(partialPath string) (fs.FS, string, error) {
	// Path Types:
	// 1. System Template <vibe/coder>
	// 2. Repo Root Template @common/header
	// 3. Local Template ./helpers/buttons

	switch {
	case strings.HasPrefix(partialPath, "<") && strings.HasSuffix(partialPath, ">"):
		// System template
		path := strings.TrimPrefix(strings.TrimSuffix(partialPath, ">"), "<")
		return ctx.SystemPartials, path, nil

	case strings.HasPrefix(partialPath, "@"):
		// Repo root template
		path := strings.TrimPrefix(partialPath, "@")
		return ctx.RepoPartials, path, nil

	case strings.HasPrefix(partialPath, "./"):
		// Local template (relative to current template)
		if ctx.CurrentTemplatePath == "" {
			return nil, "", fmt.Errorf("cannot resolve local path without CurrentTemplatePath")
		}

		// Get the directory of the current template
		currentDir := filepath.Dir(ctx.CurrentTemplatePath)

		// Calculate the path relative to the current template directory
		localPath := strings.TrimPrefix(partialPath, "./")

		// Join the paths to get the full path relative to the repo root
		fullPath := filepath.Join(currentDir, localPath)

		return ctx.RepoPartials, fullPath, nil

	default:
		return nil, "", fmt.Errorf("invalid partial path format: %s", partialPath)
	}
}
