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
	//
	// Note: os.DirFS("/home/user/myrepo") to prevent relative path escape
	RepoPartials fs.FS
}

// ResolvePartial resolves a partial template path and returns its content.
// This is a higher-level method that combines path resolution and content loading.
func (ctx *RenderContext) ResolvePartial(partialPath string) (string, error) {
	// Resolve the partial path to determine which FS and file to use
	fsys, filePath, err := ctx.ResolvePartialPath(partialPath)
	if err != nil {
		return "", fmt.Errorf("error resolving partial path %q: %w", partialPath, err)
	}

	content, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
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

	default:
		// Local template (relative to current template)
		if ctx.CurrentTemplatePath == "" {
			return nil, "", fmt.Errorf("cannot resolve local path without CurrentTemplatePath")
		}

		// Get the directory of the current template
		currentDir := filepath.Dir(ctx.CurrentTemplatePath)

		// Join the paths to get the full path relative to the repo root
		fullPath := filepath.Join(currentDir, partialPath)
		fullPath = filepath.Clean(fullPath)

		return ctx.RepoPartials, fullPath, nil
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
	return r.ctx.ResolvePartial(templatePath)
}

// RenderArgs contains all the arguments needed for template rendering.
type RenderArgs struct {
	// Content is the direct template content string to render
	Content string
	// ContentPath is the path to the template to render
	ContentPath string
	// Layout is the direct layout template content string
	Layout string
	// LayoutPath is the path to the layout template
	LayoutPath string
	// Data is the data to pass to the template during rendering
	Data any
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
	return r.Render(RenderArgs{
		ContentPath: partialPath,
		Data:        data,
	})
}

// Render renders a template, with optional layout wrapping.
// If layoutPath is empty, renders contentPath as a standalone template.
// If layoutPath is provided, renders contentPath within the layout's "main" block.
// data is the data to pass to the template.
func (r *Renderer) Render(args RenderArgs) (string, error) {
	// Get content either from direct content or from path
	contentContent := args.Content
	contentPath := args.ContentPath
	data := args.Data

	// If content is not provided directly, load it from path
	if contentContent == "" && contentPath != "" {
		var err error
		contentContent, err = r.loadTemplateContent(contentPath)
		if err != nil {
			return "", fmt.Errorf("error loading content template: %w", err)
		}
	} else if contentContent == "" && contentPath == "" {
		return "", fmt.Errorf("either Content or ContentPath must be provided")
	}

	// Store original current template path
	originalPath := r.ctx.CurrentTemplatePath
	// Update current template path for local partial resolution if path is provided
	defer func() { r.ctx.CurrentTemplatePath = originalPath }()
	if contentPath != "" {
		r.ctx.CurrentTemplatePath = contentPath
	}

	// Create a template set
	tmpl := template.New("")
	tmpl = tmpl.Funcs(template.FuncMap{
		"partial": func(partialPath string) (string, error) {
			// Use Render recursively with empty layoutPath for partials
			return r.RenderPartial(partialPath, data)
		},
	})

	templateTarget := "main"

	// Handle layout - either from direct content or from path
	layoutContent := args.Layout
	layoutPath := args.LayoutPath

	if layoutContent != "" || layoutPath != "" {
		// Set template target to "layout"
		templateTarget = "layout"

		// If layout content is not provided directly, load it from path
		if layoutContent == "" && layoutPath != "" {
			var err error
			layoutContent, err = r.loadTemplateContent(layoutPath)
			if err != nil {
				return "", fmt.Errorf("error loading layout template: %w", err)
			}
		}

		var err error
		tmpl, err = tmpl.New("layout").Parse(layoutContent)
		if err != nil {
			return "", fmt.Errorf("error parsing layout template: %w", err)
		}
	}

	// Define the main template block with content
	var err error
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
