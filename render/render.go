// Package render provides template rendering capabilities with support for
// partials from different sources (system, repo, local).
package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/hayeah/fork2/internal/metrics"
)

// Renderer provides template rendering capabilities.
type Renderer struct {
	ctx *Resolver

	// Current template context
	cur *Template // replaces curPath / curFS

	// Metrics for tracking template usage
	metrics *metrics.OutputMetrics
}

// NewRenderer creates a new Renderer with the given RenderContext and metrics.
func NewRenderer(ctx *Resolver, m *metrics.OutputMetrics) *Renderer {
	return &Renderer{
		ctx:     ctx,
		metrics: m,
	}
}

// LoadTemplate loads a template from the given path.
func (r *Renderer) LoadTemplate(path string) (*Template, error) {
	return r.ctx.LoadTemplate(path, r.cur)
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
	tmpl, err := r.LoadTemplate(partialPath)
	if err != nil {
		return "", fmt.Errorf("error loading template %s: %w", partialPath, err)
	}

	// Save the current context
	prev := r.cur

	// Set the new context for this template
	r.cur = tmpl

	// Restore the original context when we're done
	defer func() {
		r.cur = prev
	}()

	// Force no layout for partials
	tmpl.FrontMatter.Layout = ""

	// Render the template
	return r.RenderTemplate(tmpl, data)
}

// Include reads a file and returns its raw contents as a string.
// The path resolution follows the same rules as templates, so callers can use
// system (<vibe/foo>), repo (@foo/bar) and relative (./foo) paths.
func (r *Renderer) Include(path string) (string, error) {
	fsys, filePath, err := r.ctx.ResolvePartialPath(path, r.cur)
	if err != nil {
		return "", fmt.Errorf("error resolving include path %s: %w", path, err)
	}

	b, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return "", fmt.Errorf("error reading include %s: %w", filePath, err)
	}

	if r.metrics != nil {
		r.metrics.Add("include", filePath, b)
	}

	return string(b), nil
}

// Render renders a template, with optional layout wrapping.
// If the template has no layout specified, it's rendered as a standalone template.
// If the template has a layout, it's rendered and then passed as .Content to the layout template.
// data is the data to pass to the template.
func (r *Renderer) Render(contentPath string, data Content) (string, error) {
	// Load the content template
	tmpl, err := r.LoadTemplate(contentPath)
	if err != nil {
		return "", fmt.Errorf("error loading content template %s: %w", contentPath, err)
	}

	return r.RenderTemplate(tmpl, data)
}

// RenderTemplate renders a template and applies any layouts specified in its metadata
func (r *Renderer) RenderTemplate(t *Template, data Content) (string, error) {
	seen := make(map[string]bool)
	return r.renderTemplateInternal(t, data, seen, 0)
}

// splitLayouts turns `layout = "a;b;c"` into
// `[]string{"a", "b", "c"}` with whitespace trimmed and
// empty segments removed.
func splitLayouts(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitFiles splits a semicolon-separated list of files and trims whitespace
func splitFiles(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// processFile handles loading and processing a file based on bang prefix
// If the path starts with !, it's rendered as a template
// Otherwise, it's included as raw content
func (r *Renderer) processFile(path string, data Content) (string, error) {
	// Check for bang prefix
	if strings.HasPrefix(path, "!") {
		// Remove the bang prefix and render as template
		templatePath := strings.TrimPrefix(path, "!")
		return r.RenderPartial(templatePath, data)
	}

	// No bang prefix - include as raw content
	return r.Include(path)
}

// processFiles processes a list of files and returns their concatenated content
// Each file's content is separated by at least one newline
func (r *Renderer) processFiles(files []string, data Content) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	var parts []string
	for _, file := range files {
		content, err := r.processFile(file, data)
		if err != nil {
			return "", fmt.Errorf("error processing file %s: %w", file, err)
		}
		if content != "" {
			parts = append(parts, content)
		}
	}

	// Join with newline separator to ensure at least one newline between parts
	return strings.Join(parts, "\n"), nil
}

// renderTemplateInternal renders *t* then applies its wrapper layouts
// inner-to-outer, with depth & cycle protection.
// New layout order: [before ...]<<<layouts ...>>>[after...] [user]
func (r *Renderer) renderTemplateInternal(
	t *Template, data Content, seen map[string]bool, depth int,
) (string, error) {

	// ─── Safety guards ────────────────────────────────────────────────────────
	if depth > 10 {
		return "", fmt.Errorf("layout nesting too deep (max 10): %s", t.Path)
	}

	layouts := splitLayouts(t.FrontMatter.Layout)
	if depth+len(layouts) > 10 {
		return "", fmt.Errorf("layout nesting too deep (max 10): %s", t.Path)
	}

	for _, lp := range layouts { // cycle detection
		if seen[lp] {
			return "", fmt.Errorf("layout cycle detected: %s", lp)
		}
		seen[lp] = true
	}

	// Track seen layouts to prevent infinite recursion
	// Save the current context
	prev := r.cur

	// Set the new context for this template
	r.cur = t

	// Restore the original context when we're done
	defer func() {
		r.cur = prev
	}()

	// ─── Process before files (with empty .Content) ─────────────────────────
	beforeFiles := splitFiles(t.FrontMatter.Before)
	beforeContent := ""
	if len(beforeFiles) > 0 {
		// Temporarily set empty content for before templates
		prevContent := data.Content()
		data.SetContent("")

		var err error
		beforeContent, err = r.processFiles(beforeFiles, data)
		if err != nil {
			return "", fmt.Errorf("error processing before files: %w", err)
		}

		// Restore original content
		data.SetContent(prevContent)
	}

	// ─── Apply layouts (with empty .Content for the first layout) ────────────
	layoutContent := ""
	if len(layouts) > 0 {
		// Save original content
		prevContent := data.Content()

		// Start with empty content for the innermost layout
		var rendered string

		// Apply layouts from innermost to outermost
		for i := len(layouts) - 1; i >= 0; i-- {
			wrapperPath := strings.TrimSpace(layouts[i])

			wrapper, err := r.LoadTemplate(wrapperPath)
			if err != nil {
				return "", fmt.Errorf("error loading layout template %s: %w", wrapperPath, err)
			}

			// Swap renderer context for relative-partial resolution.
			prevCur := r.cur
			r.cur = wrapper

			// For the first (innermost) layout, use empty content
			// For subsequent layouts, use the previously rendered content
			if i == len(layouts)-1 {
				data.SetContent("")
			} else {
				data.SetContent(rendered)
			}

			rendered, err = r.renderTemplateInternal(wrapper, data, seen, depth+1)

			r.cur = prevCur

			if err != nil {
				return "", err
			}
		}

		layoutContent = rendered

		// Restore original content
		data.SetContent(prevContent)
	}

	// ─── Process after files (with empty .Content) ──────────────────────────
	afterFiles := splitFiles(t.FrontMatter.After)
	afterContent := ""
	if len(afterFiles) > 0 {
		// Temporarily set empty content for after templates
		prevContent := data.Content()
		data.SetContent("")

		var err error
		afterContent, err = r.processFiles(afterFiles, data)
		if err != nil {
			return "", fmt.Errorf("error processing after files: %w", err)
		}

		// Restore original content
		data.SetContent(prevContent)
	}

	// ─── Render the user content (current template body) ────────────────────
	userContent, err := r.executeTemplate(t, data)
	if err != nil {
		return "", err
	}

	// ─── Combine all parts with newline separation ──────────────────────────
	var parts []string

	if beforeContent != "" {
		parts = append(parts, beforeContent)
	}

	if layoutContent != "" {
		parts = append(parts, layoutContent)
	}

	if afterContent != "" {
		parts = append(parts, afterContent)
	}

	if userContent != "" {
		parts = append(parts, userContent)
	}

	// Track metrics
	if r.metrics != nil {
		r.metrics.Add("template", t.Path, []byte(t.Body))
	}

	// Join all parts with newline separation
	return strings.Join(parts, "\n"), nil
}

// executeTemplate renders a single template body with the "partial" helper.
func (r *Renderer) executeTemplate(t *Template, data Content) (string, error) {
	tmpl, err := template.New("content").Funcs(template.FuncMap{
		"partial": func(path string) (string, error) {
			return r.RenderPartial(path, data)
		},
		"include": func(path string) (string, error) {
			return r.Include(path)
		},
	}).Parse(t.Body)
	if err != nil {
		return "", fmt.Errorf("error parsing template %s: %w", t.Path, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("error executing template %s: %w", t.Path, err)
	}
	return buf.String(), nil
}
