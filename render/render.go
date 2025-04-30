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

	"github.com/hayeah/fork2/internal/metrics"
)

// Resolver lookups available template paths across multiple filesystems
type Resolver struct {
	// Search‑order stack of filesystems – first = highest priority, last = builtin defaults
	Partials []fs.FS
}

// LoadTemplate loads a template from a path and returns the template.
func (r *Resolver) LoadTemplate(path string, cur *Template) (*Template, error) {
	// Resolve the partial path to determine which FS and file to use
	fsys, filePath, err := r.ResolvePartialPath(path, cur)
	if err != nil {
		return nil, fmt.Errorf("error resolving partial path %q: %w", path, err)
	}

	// Read the file with fs.ReadFile
	content, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter
	_, frontMatterContent, body, err := ParseFrontMatter(string(content))
	if err != nil {
		return nil, err
	}

	// Create template with empty Meta by default
	meta := Meta{}
	if frontMatterContent != "" {
		if err := ParseToml(frontMatterContent, &meta); err != nil {
			return nil, err
		}
	}

	// Create and return the template with the parsed metadata
	tmpl := &Template{
		Path: path,
		Body: body,
		Meta: meta,
		FS:   fsys,
	}

	return tmpl, nil
}

// NewResolver creates a new Resolver with the given filesystem stack.
func NewResolver(partials ...fs.FS) *Resolver {
	return &Resolver{Partials: partials}
}

// resolveTemplateFile checks for an exact match and, only if no
// extension is present, falls back to the ".md" variant.
// It searches through all provided filesystems in order.
func resolveTemplateFile(base string, filesystems ...fs.FS) (fs.FS, string, error) {
	for _, fsys := range filesystems {
		// exact path first
		if _, err := fs.Stat(fsys, base); err == nil {
			return fsys, base, nil
		}
		// no extension → try ".md"
		if filepath.Ext(base) == "" {
			alt := base + ".md"
			if _, err := fs.Stat(fsys, alt); err == nil {
				return fsys, alt, nil
			}
		}
	}
	return nil, "", fmt.Errorf("template %q not found in any filesystem", base)
}

// ResolvePartialPath determines which FS and file should be used for a given partial path.
// It uses the following rules:
// 1. System Template <vibe/coder> - use the last FS in Partials
// 2. Repo Root Template @common/header - use the first FS in Partials
// 3. Local Template ./helpers/buttons - use the same FS as the current template
// 4. Bare path common/header - search all FS in order until found
// Callers may omit the `.md` extension; the resolver will look for both *name* and *name.md*, but only when *name* has no extension.
func (ctx *Resolver) ResolvePartialPath(partialPath string, cur *Template) (fs.FS, string, error) {
	// Derive curPath/curFS from cur (if cur == nil, pass empty string / nil)
	currentTemplatePath := ""
	var currentTemplateFS fs.FS
	if cur != nil {
		currentTemplatePath = cur.Path
		currentTemplateFS = cur.FS
	}

	switch {
	case strings.HasPrefix(partialPath, "<") && strings.HasSuffix(partialPath, ">"):
		// System template - use the last FS in Partials
		if len(ctx.Partials) == 0 {
			return nil, "", fmt.Errorf("no filesystem available for system template")
		}
		path := strings.TrimPrefix(strings.TrimSuffix(partialPath, ">"), "<")
		fsys := ctx.Partials[len(ctx.Partials)-1]
		return resolveTemplateFile(path, fsys)

	case strings.HasPrefix(partialPath, "@"):
		// Repo root template - use the first FS in Partials
		if len(ctx.Partials) == 0 {
			return nil, "", fmt.Errorf("no filesystem available for repo template")
		}
		path := strings.TrimPrefix(partialPath, "@")
		fsys := ctx.Partials[0]
		return resolveTemplateFile(path, fsys)

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
		return resolveTemplateFile(fullPath, currentTemplateFS)

	default:
		// Bare path - search through all filesystems in order
		if len(ctx.Partials) == 0 {
			return nil, "", fmt.Errorf("no filesystems available for template lookup")
		}

		// Search through all filesystems
		return resolveTemplateFile(partialPath, ctx.Partials...)
	}
}

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
	tmpl, err := r.LoadTemplate(contentPath)
	if err != nil {
		return "", fmt.Errorf("error loading content template %s: %w", contentPath, err)
	}

	// Save the current context
	prev := r.cur

	// Set the new context for this template
	r.cur = tmpl

	// Restore the original context when we're done
	defer func() {
		r.cur = prev
	}()

	return r.RenderTemplate(tmpl, data)
}

// RenderTemplate renders a template and applies any layouts specified in its metadata
func (r *Renderer) RenderTemplate(t *Template, data Content) (string, error) {
	// Track seen layouts to prevent infinite recursion
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

	layouts := splitLayouts(t.Meta.Layout)
	if depth+len(layouts) > 10 {
		return "", fmt.Errorf("layout nesting too deep (max 10): %s", t.Path)
	}

	for _, lp := range layouts { // cycle detection
		if seen[lp] {
			return "", fmt.Errorf("layout cycle detected: %s", lp)
		}
		seen[lp] = true
	}

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
