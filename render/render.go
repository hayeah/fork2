// Package render provides template rendering capabilities with support for
// partials from different sources (system, repo, local).
package render

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"text/template"

	"github.com/hayeah/fork2/internal/metrics"
)

// countingWriter is an io.Writer that counts bytes written
type countingWriter struct {
	w     io.Writer
	count int64
}

func (cw *countingWriter) Write(p []byte) (n int, err error) {
	n, err = cw.w.Write(p)
	cw.count += int64(n)
	return n, err
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

// RenderPartialTo renders a template without a layout to the provided writer.
// It's a convenience method that calls RenderTo with an empty layoutPath.
//
// Parameters:
//   - w: The writer to render to
//   - partialPath: The path to the template to render. Can be in one of three formats:
//   - System template: <vibe/coder>
//   - Repo root template: @common/header
//   - Local template: ./helpers/buttons (relative to current template path)
//   - data: The data to pass to the template during rendering
//
// Returns:
//   - An error if the template could not be loaded or rendered
func (r *Renderer) RenderPartialTo(w io.Writer, partialPath string, data Content) error {
	// Load the template
	tmpl, err := r.LoadTemplate(partialPath)
	if err != nil {
		return fmt.Errorf("error loading template %s: %w", partialPath, err)
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
	return r.RenderTemplateTo(w, tmpl, data)
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
	var buf bytes.Buffer
	err := r.RenderPartialTo(&buf, partialPath, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
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

// RenderTo renders a template to the provided writer, with optional layout wrapping.
// If the template has no layout specified, it's rendered as a standalone template.
// If the template has a layout, it's rendered and then passed as .Content to the layout template.
// data is the data to pass to the template.
func (r *Renderer) RenderTo(w io.Writer, contentPath string, data Content) error {
	// Load the content template
	tmpl, err := r.LoadTemplate(contentPath)
	if err != nil {
		return fmt.Errorf("error loading content template %s: %w", contentPath, err)
	}

	return r.RenderTemplateTo(w, tmpl, data)
}

// Render renders a template, with optional layout wrapping.
// If the template has no layout specified, it's rendered as a standalone template.
// If the template has a layout, it's rendered and then passed as .Content to the layout template.
// data is the data to pass to the template.
func (r *Renderer) Render(contentPath string, data Content) (string, error) {
	var buf bytes.Buffer
	err := r.RenderTo(&buf, contentPath, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderTemplateTo renders a template to the provided writer and applies any layouts specified in its metadata
func (r *Renderer) RenderTemplateTo(w io.Writer, t *Template, data Content) error {
	seen := make(map[string]bool)
	return r.renderTemplateInternal(w, t, data, seen, 0)
}

// RenderTemplate renders a template and applies any layouts specified in its metadata
func (r *Renderer) RenderTemplate(t *Template, data Content) (string, error) {
	var buf bytes.Buffer
	err := r.RenderTemplateTo(&buf, t, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// splitSemicolon splits a semicolon-separated string into a slice,
// trimming whitespace and removing empty segments.
// Used for both layout and file lists.
func splitSemicolon(s string) []string {
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

// processFiles processes a list of files and writes their concatenated content to w
// Each file's content is separated by at least one newline
func (r *Renderer) processFiles(w io.Writer, files []string, data Content) error {
	if len(files) == 0 {
		return nil
	}

	for i, file := range files {
		content, err := r.processFile(file, data)
		if err != nil {
			return fmt.Errorf("error processing file %s: %w", file, err)
		}
		if content != "" {
			if i > 0 {
				_, err := w.Write([]byte{'\n'})
				if err != nil {
					return err
				}
			}
			_, err := io.WriteString(w, content)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// renderTemplateInternal renders *t* then applies its wrapper layouts
// inner-to-outer, with depth & cycle protection.
// New layout order: [before ...]<<<layouts ...>>>[after...] [user]
func (r *Renderer) renderTemplateInternal(
	w io.Writer, t *Template, data Content, seen map[string]bool, depth int,
) error {

	// ─── Safety guards ────────────────────────────────────────────────────────
	if depth > 10 {
		return fmt.Errorf("layout nesting too deep (max 10): %s", t.Path)
	}

	layouts := splitSemicolon(t.FrontMatter.Layout)
	if depth+len(layouts) > 10 {
		return fmt.Errorf("layout nesting too deep (max 10): %s", t.Path)
	}

	for _, lp := range layouts { // cycle detection
		if seen[lp] {
			return fmt.Errorf("layout cycle detected: %s", lp)
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

	// Track metrics early
	if r.metrics != nil {
		r.metrics.Add("template", t.Path, []byte(t.Body))
	}

	// ─── Process before files (with empty .Content) ─────────────────────────
	beforeFiles := splitSemicolon(t.FrontMatter.Before)
	if len(beforeFiles) > 0 {
		// Temporarily set empty content for before templates
		prevContent := data.Content()
		data.SetContent("")

		// Use a counting writer to track if anything was written
		cw := &countingWriter{w: w}
		if err := r.processFiles(cw, beforeFiles, data); err != nil {
			return fmt.Errorf("error processing before files: %w", err)
		}

		// Add newline if we actually wrote something
		if cw.count > 0 {
			_, err := w.Write([]byte{'\n'})
			if err != nil {
				return err
			}
		}

		// Restore original content
		data.SetContent(prevContent)
	}

	// ─── Apply layouts (with empty .Content for the first layout) ────────────
	if len(layouts) > 0 {
		// Save original content
		prevContent := data.Content()

		// Use a temporary buffer for iterating through layouts
		var layoutBuf bytes.Buffer

		// Apply layouts from innermost to outermost
		for i := len(layouts) - 1; i >= 0; i-- {
			wrapperPath := strings.TrimSpace(layouts[i])

			wrapper, err := r.LoadTemplate(wrapperPath)
			if err != nil {
				return fmt.Errorf("error loading layout template %s: %w", wrapperPath, err)
			}

			// Swap renderer context for relative-partial resolution.
			prevCur := r.cur
			r.cur = wrapper

			// For the first (innermost) layout, use empty content
			// For subsequent layouts, use the previously rendered content
			if i == len(layouts)-1 {
				data.SetContent("")
			} else {
				data.SetContent(layoutBuf.String())
			}

			// Reset the buffer for the next iteration
			layoutBuf.Reset()

			if err := r.renderTemplateInternal(&layoutBuf, wrapper, data, seen, depth+1); err != nil {
				r.cur = prevCur
				return err
			}

			r.cur = prevCur
		}

		// Write the final layout content to the main writer
		if layoutBuf.Len() > 0 {
			_, err := w.Write(layoutBuf.Bytes())
			if err != nil {
				return err
			}
			_, err = w.Write([]byte{'\n'})
			if err != nil {
				return err
			}
		}

		// Restore original content
		data.SetContent(prevContent)
	}

	// ─── Process after files (with empty .Content) ──────────────────────────
	afterFiles := splitSemicolon(t.FrontMatter.After)
	if len(afterFiles) > 0 {
		// Temporarily set empty content for after templates
		prevContent := data.Content()
		data.SetContent("")

		// Use a counting writer to track if anything was written
		cw := &countingWriter{w: w}
		if err := r.processFiles(cw, afterFiles, data); err != nil {
			return fmt.Errorf("error processing after files: %w", err)
		}

		// Add newline if we actually wrote something
		if cw.count > 0 {
			_, err := w.Write([]byte{'\n'})
			if err != nil {
				return err
			}
		}

		// Restore original content
		data.SetContent(prevContent)
	}

	// ─── Render the user content (current template body) ────────────────────
	if err := r.executeTemplate(w, t, data); err != nil {
		return err
	}

	return nil
}

// executeTemplate renders a single template body with the "partial" helper.
func (r *Renderer) executeTemplate(w io.Writer, t *Template, data Content) error {
	tmpl, err := template.New("content").Funcs(template.FuncMap{
		"partial": func(path string) (string, error) {
			return r.RenderPartial(path, data)
		},
		"include": func(path string) (string, error) {
			return r.Include(path)
		},
	}).Parse(t.Body)
	if err != nil {
		return fmt.Errorf("error parsing template %s: %w", t.Path, err)
	}

	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("error executing template %s: %w", t.Path, err)
	}
	return nil
}
