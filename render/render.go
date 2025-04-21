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

// Resolver lookups available template paths across multiple filesystems
type Resolver struct {
	// Search‑order stack of filesystems – first = highest priority, last = builtin defaults
	Partials []fs.FS
}

// ResolvePartial resolves a partial template path and returns its content and the filesystem it was found in.
// This is a higher-level method that combines path resolution and content loading.
func (ctx *Resolver) ResolvePartial(partialPath string, currentTemplatePath string, currentTemplateFS fs.FS) (string, fs.FS, error) {
	// Resolve the partial path to determine which FS and file to use
	fsys, filePath, err := ctx.ResolvePartialPath(partialPath, currentTemplatePath, currentTemplateFS)
	if err != nil {
		return "", nil, fmt.Errorf("error resolving partial path %q: %w", partialPath, err)
	}

	content, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return "", nil, err
	}
	return string(content), fsys, nil
}

// NewResolver creates a new Resolver with the given filesystem stack.
func NewResolver(partials ...fs.FS) *Resolver {
	return &Resolver{Partials: partials}
}

// ResolvePartialPath determines which FS and file should be used for a given partial path.
// It uses the following rules:
// 1. System Template <vibe/coder> - use the last FS in Partials
// 2. Repo Root Template @common/header - use the first FS in Partials
// 3. Local Template ./helpers/buttons - use the same FS as the current template
// 4. Bare path common/header - search all FS in order until found
func (ctx *Resolver) ResolvePartialPath(partialPath string, currentTemplatePath string, currentTemplateFS fs.FS) (fs.FS, string, error) {
	switch {
	case strings.HasPrefix(partialPath, "<") && strings.HasSuffix(partialPath, ">"):
		// System template - use the last FS in Partials
		path := strings.TrimPrefix(strings.TrimSuffix(partialPath, ">"), "<")
		if len(ctx.Partials) == 0 {
			return nil, "", fmt.Errorf("no filesystem available for system template")
		}
		return ctx.Partials[len(ctx.Partials)-1], path, nil

	case strings.HasPrefix(partialPath, "@"):
		// Repo root template - use the first FS in Partials
		path := strings.TrimPrefix(partialPath, "@")
		if len(ctx.Partials) == 0 {
			return nil, "", fmt.Errorf("no filesystem available for repo template")
		}
		return ctx.Partials[0], path, nil

	case strings.HasPrefix(partialPath, "./") || strings.HasPrefix(partialPath, "../") || partialPath == "." || partialPath == "..":
		// Local template (relative to current template)
		if currentTemplatePath == "" {
			return nil, "", fmt.Errorf("cannot resolve relative path without currentTemplatePath")
		}
		if currentTemplateFS == nil {
			return nil, "", fmt.Errorf("cannot resolve relative path without currentTemplateFS")
		}

		// Get the directory of the current template
		currentDir := filepath.Dir(currentTemplatePath)

		// Resolve the path relative to the current template
		fullPath := filepath.Join(currentDir, partialPath)
		fullPath = filepath.Clean(fullPath)

		return currentTemplateFS, fullPath, nil

	default:
		// Bare path - search through all filesystems in order
		if len(ctx.Partials) == 0 {
			return nil, "", fmt.Errorf("no filesystems available for template lookup")
		}

		// Try each filesystem in order until we find the file
		for _, fsys := range ctx.Partials {
			// Check if the file exists in this filesystem
			if _, err := fs.Stat(fsys, partialPath); err == nil {
				return fsys, partialPath, nil
			}
		}

		return nil, "", fmt.Errorf("template %q not found in any filesystem", partialPath)
	}
}

// Renderer provides template rendering capabilities.
type Renderer struct {
	ctx *Resolver

	// Current template context
	curPath string
	curFS   fs.FS
}

// NewRenderer creates a new Renderer with the given RenderContext.
func NewRenderer(ctx *Resolver) *Renderer {
	return &Renderer{
		ctx: ctx,
	}
}

// loadTemplate loads a template from the given path.
func (r *Renderer) loadTemplate(templatePath string) (*Template, fs.FS, error) {
	return LoadTemplate(r.ctx, templatePath, r.curPath, r.curFS)
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
//   - Local template: ./helpers/buttons (relative to current template path)
//   - data: The data to pass to the template during rendering
//
// Returns:
//   - A string containing the rendered template
//   - An error if the template could not be loaded or rendered
func (r *Renderer) RenderPartial(partialPath string, data Content) (string, error) {
	// Load the template
	tmpl, tmplFS, err := r.loadTemplate(partialPath)
	if err != nil {
		return "", fmt.Errorf("error loading template %s: %w", partialPath, err)
	}

	// Save the current context
	prevPath, prevFS := r.curPath, r.curFS

	// Set the new context for this template
	r.curPath, r.curFS = tmpl.Path, tmplFS

	// Restore the original context when we're done
	defer func() {
		r.curPath, r.curFS = prevPath, prevFS
	}()

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
	// Load the content template
	tmpl, tmplFS, err := r.loadTemplate(contentPath)
	if err != nil {
		return "", fmt.Errorf("error loading content template %s: %w", contentPath, err)
	}

	// Save the current context
	prevPath, prevFS := r.curPath, r.curFS

	// Set the new context for this template
	r.curPath, r.curFS = tmpl.Path, tmplFS

	// Restore the original context when we're done
	defer func() {
		r.curPath, r.curFS = prevPath, prevFS
	}()

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

	// We're already in the correct context from the caller
	// The curPath and curFS should be set to t.Path and the FS that provided t

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

	// If a layout is specified, load and render it with the content
	if t.Meta.Layout != "" {
		// Load the layout template
		layoutTmpl, layoutFS, err := r.loadTemplate(t.Meta.Layout)
		if err != nil {
			return "", fmt.Errorf("error loading layout template %s: %w", t.Meta.Layout, err)
		}

		// Save the current context
		prevPath, prevFS := r.curPath, r.curFS

		// Set the new context for the layout template
		r.curPath, r.curFS = layoutTmpl.Path, layoutFS

		// Restore the original context when we're done with this layout
		defer func() {
			r.curPath, r.curFS = prevPath, prevFS
		}()

		// Save the original content and set the rendered content
		prev := data.Content()
		data.SetContent(renderedContent)
		// Restore the original content when we're done
		defer func() { data.SetContent(prev) }()

		// Recursively render with the parent layout
		return r.renderTemplateInternal(layoutTmpl, data, seen, depth+1)
	}

	// If no layout is specified, return the content directly
	return renderedContent, nil
}
