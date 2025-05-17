// Package render provides template rendering capabilities with support for
// partials from different sources (system, repo, local).
package render

import (
	"bytes"
	"fmt"
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

// renderTemplateInternal renders *t* then applies its wrapper layouts
// inner-to-outer, with depth & cycle protection.
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

	// ─── Render the current template body ────────────────────────────────────
	rendered, err := r.executeTemplate(t, data)
	if err != nil {
		return "", err
	}

	// No layouts?  We’re done.
	if len(layouts) == 0 {
		if r.metrics != nil {
			r.metrics.Add("template", t.Path, []byte(t.Body))
		}
		return rendered, nil
	}

	// ─── Apply layouts (inner-to-outer) ──────────────────────────────────────
	for i := len(layouts) - 1; i >= 0; i-- {
		wrapperPath := strings.TrimSpace(layouts[i])

		wrapper, err := r.LoadTemplate(wrapperPath)
		if err != nil {
			return "", fmt.Errorf("error loading layout template %s: %w", wrapperPath, err)
		}

		// Swap renderer context for relative-partial resolution.
		prevCur := r.cur
		r.cur = wrapper

		// Inject the already-rendered content, recurse, restore.
		prevContent := data.Content()
		data.SetContent(rendered)

		rendered, err = r.renderTemplateInternal(wrapper, data, seen, depth+1)

		data.SetContent(prevContent)
		r.cur = prevCur

		if err != nil {
			return "", err
		}
	}

	return rendered, nil
}

// executeTemplate renders a single template body with the “partial” helper.
func (r *Renderer) executeTemplate(t *Template, data Content) (string, error) {
	tmpl, err := template.New("content").Funcs(template.FuncMap{
		"partial": func(path string) (string, error) {
			return r.RenderPartial(path, data)
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
