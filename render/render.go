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

// Renderer provides template rendering capabilities.
type Renderer struct {
	ctx *RenderContext
}

// NewRenderer creates a new Renderer with the given RenderContext.
func NewRenderer(ctx *RenderContext) *Renderer {
	return &Renderer{
		ctx: ctx,
	}
}

// loadTemplateContent loads the content of a template from the given path.
func (r *Renderer) loadTemplateContent(templatePath string) (string, error) {
	// Resolve the template path
	templateFS, templateFile, err := r.ctx.ResolvePartialPath(templatePath)
	if err != nil {
		return "", fmt.Errorf("error resolving template path: %w", err)
	}

	// Read the template content
	templateContent, err := readTemplate(templateFS, templateFile)
	if err != nil {
		return "", fmt.Errorf("error reading template: %w", err)
	}

	return templateContent, nil
}

// RenderPartial renders a template without a layout.
// It's a convenience method that calls Render with an empty layoutPath.
//
// Parameters:
//   - partialPath: The path to the template to render. Can be in one of three formats:
//   - System template: <vibe/coder>
//   - Repo root template: @common/header
//   - Local template: ./helpers/buttons (relative to CurrentTemplatePath)
//   - data: The data to pass to the template during rendering
//
// Returns:
//   - A string containing the rendered template
//   - An error if the template could not be loaded or rendered
func (r *Renderer) RenderPartial(partialPath string, data any) (string, error) {
	return r.Render(partialPath, "", data)
}

// Render renders a template, with optional layout wrapping.
// If layoutPath is empty, renders contentPath as a standalone template.
// If layoutPath is provided, renders contentPath within the layout's "main" block.
// data is the data to pass to the template.
func (r *Renderer) Render(contentPath string, layoutPath string, data any) (string, error) {
	// Get the content template
	contentContent, err := r.loadTemplateContent(contentPath)
	if err != nil {
		return "", fmt.Errorf("error loading content template: %w", err)
	}

	// Store original current template path
	originalPath := r.ctx.CurrentTemplatePath
	// Update current template path for local partial resolution
	defer func() { r.ctx.CurrentTemplatePath = originalPath }()
	r.ctx.CurrentTemplatePath = contentPath

	// Create a template set
	tmpl := template.New("")
	tmpl = tmpl.Funcs(template.FuncMap{
		"partial": func(partialPath string) (string, error) {
			// Use Render recursively with empty layoutPath for partials
			return r.RenderPartial(partialPath, data)
		},
	})

	templateTarget := "main"

	if layoutPath != "" {
		// Parse the layout template
		templateTarget = "layout"
		layoutContent, err := r.loadTemplateContent(layoutPath)
		if err != nil {
			return "", fmt.Errorf("error loading layout template: %w", err)
		}

		tmpl, err = tmpl.New("layout").Parse(layoutContent)
		if err != nil {
			return "", fmt.Errorf("error parsing layout template: %w", err)
		}
	}

	// Define the main template block with content
	tmpl, err = tmpl.New("main").Parse(contentContent)
	if err != nil {
		return "", fmt.Errorf("error parsing content template: %w", err)
	}

	// Execute the template
	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, templateTarget, data)
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
