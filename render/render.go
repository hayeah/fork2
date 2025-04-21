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

// Resolver lookups available template
type Resolver struct {
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
func (ctx *Resolver) ResolvePartial(partialPath string) (string, error) {
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
func (ctx *Resolver) ResolvePartialPath(partialPath string) (fs.FS, string, error) {
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
	ctx *Resolver
}

// NewRenderer creates a new Renderer with the given RenderContext.
func NewRenderer(ctx *Resolver) *Renderer {
	return &Renderer{
		ctx: ctx,
	}
}

// loadTemplate loads a template from the given path.
func (r *Renderer) loadTemplate(templatePath string) (*Template, error) {
	return LoadTemplate(r.ctx, templatePath)
}

// Content is implemented by any data object that can carry the
// pre-rendered inner template.
type Content interface {
	Content() string   // getter
	SetContent(string) // setter (mutates receiver)
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
func (r *Renderer) RenderPartial(partialPath string, data Content) (string, error) {
	// Load the template
	tmpl, err := LoadTemplate(r.ctx, partialPath)
	if err != nil {
		return "", err
	}

	// Force no layout for partials
	tmpl.Meta.Layout = ""

	// Render the template
	return r.RenderTemplate(tmpl, data)
}

// Render renders a template, with optional layout wrapping.
// If the template has no layout specified, it's rendered as a standalone template.
// If the template has a layout, it's rendered and then passed as .Content to the layout template.
// data is the data to pass to the template.
func (r *Renderer) Render(contentPath string, data Content) (string, error) {
	// Load the template
	tmpl, err := LoadTemplate(r.ctx, contentPath)
	if err != nil {
		return "", err
	}

	// Render the template with layouts
	return r.RenderTemplate(tmpl, data)
}

// RenderTemplate renders a template and applies any layouts specified in its metadata
func (r *Renderer) RenderTemplate(t *Template, data Content) (string, error) {
	// Track seen layouts to prevent infinite recursion
	seen := make(map[string]bool)
	return r.renderTemplateInternal(t, data, seen, 0)
}

// renderTemplateInternal is the internal implementation of renderWithLayouts with cycle detection
func (r *Renderer) renderTemplateInternal(t *Template, data Content, seen map[string]bool, depth int) (string, error) {
	// Guard against infinite recursion
	if depth > 10 {
		return "", fmt.Errorf("layout nesting too deep (max 10): %s", t.Path)
	}

	// Check for cycles
	if t.Meta.Layout != "" {
		if seen[t.Meta.Layout] {
			return "", fmt.Errorf("layout cycle detected: %s", t.Meta.Layout)
		}
		seen[t.Meta.Layout] = true
	}

	// Store original current template path
	originalPath := r.ctx.CurrentTemplatePath
	// Update current template path for local partial resolution
	defer func() { r.ctx.CurrentTemplatePath = originalPath }()
	r.ctx.CurrentTemplatePath = t.Path

	// Create a template for content
	contentTmpl := template.New("content")
	contentTmpl = contentTmpl.Funcs(template.FuncMap{
		"partial": func(partialPath string) (string, error) {
			// Use RenderPartial for partials
			return r.RenderPartial(partialPath, data)
		},
	})

	// Parse the content template
	contentTmpl, err := contentTmpl.Parse(t.Body)
	if err != nil {
		return "", fmt.Errorf("error parsing template %s: %w", t.Path, err)
	}

	// Execute the content template
	var contentBuf bytes.Buffer
	err = contentTmpl.Execute(&contentBuf, data)
	if err != nil {
		return "", fmt.Errorf("error executing template %s: %w", t.Path, err)
	}

	renderedContent := contentBuf.String()

	// If no layout is specified, return the content directly
	if t.Meta.Layout == "" {
		return renderedContent, nil
	}

	// Load the layout template
	layoutTmpl, err := LoadTemplate(r.ctx, t.Meta.Layout)
	if err != nil {
		return "", fmt.Errorf("error loading layout %s: %w", t.Meta.Layout, err)
	}

	// Save the original content and set the rendered content
	prev := data.Content()
	data.SetContent(renderedContent)
	// Restore the original content when we're done
	defer func() { data.SetContent(prev) }()

	// Recursively render with the parent layout
	return r.renderTemplateInternal(layoutTmpl, data, seen, depth+1)
}
