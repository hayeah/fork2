package render

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// Resolver lookups available template paths across multiple filesystems
type Resolver struct {
	// Search‑order stack of filesystems – first = highest priority, last = builtin defaults
	Partials []fs.FS
	Mode     string
}

// LoadTemplate loads a template from a path and returns the template.
func (r *Resolver) LoadTemplate(path string, cur *Template) (*Template, error) {
	// Resolve the partial path to determine which FS and file to use
	fsys, filePath, err := r.ResolvePartialPath(path, cur)
	if err != nil {
		return nil, fmt.Errorf("error resolving partial path %q: %w", path, err)
	}

	// Load the template using the filesystem
	tmpl, err := LoadTemplateFS(filePath, fsys)
	if err != nil {
		return nil, err
	}

	// Set the original path (not the resolved file path) as the template path
	tmpl.Path = path

	return tmpl, nil
}

// NewResolver creates a new Resolver with the given filesystem stack.
func NewResolver(mode string, partials ...fs.FS) *Resolver {
	return &Resolver{Partials: partials, Mode: mode}
}

// resolveTemplateFile checks for an exact match and, only if no
// extension is present, falls back to the ".md" variant.
// It searches through all provided filesystems in order.
func (r *Resolver) resolveTemplateFile(base string, filesystems ...fs.FS) (fs.FS, string, error) {
	for _, fsys := range filesystems {
		// If mode is set, try mode-specific variant first
		if r.Mode != "" {
			// For files with extensions, insert mode before the extension
			ext := filepath.Ext(base)
			if ext != "" {
				modeVariant := base[:len(base)-len(ext)] + "." + r.Mode + ext
				if _, err := fs.Stat(fsys, modeVariant); err == nil {
					return fsys, modeVariant, nil
				}
			} else {
				// For files without extensions, try mode variant with .md extension first
				modeVariant := base + "." + r.Mode + ".md"
				if _, err := fs.Stat(fsys, modeVariant); err == nil {
					return fsys, modeVariant, nil
				}
				// Then try mode variant without .md extension
				modeVariant = base + "." + r.Mode
				if _, err := fs.Stat(fsys, modeVariant); err == nil {
					return fsys, modeVariant, nil
				}
			}
		}

		// exact path fallback
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

	// Allow "./path" as repo-root shorthand when no template is yet in play.
	if strings.HasPrefix(partialPath, "./") && currentTemplatePath == "" {
		cleaned := strings.TrimPrefix(partialPath, "./")
		return ctx.ResolvePartialPath(cleaned, cur) // tail-recurse as a bare path
	}

	switch {
	case strings.HasPrefix(partialPath, "<") && strings.HasSuffix(partialPath, ">"):
		// System template - use the last FS in Partials
		if len(ctx.Partials) == 0 {
			return nil, "", fmt.Errorf("no filesystem available for system template")
		}
		path := strings.TrimPrefix(strings.TrimSuffix(partialPath, ">"), "<")
		fsys := ctx.Partials[len(ctx.Partials)-1]
		return ctx.resolveTemplateFile(path, fsys)

	case strings.HasPrefix(partialPath, "@"):
		// Repo root template - use the first FS in Partials
		if len(ctx.Partials) == 0 {
			return nil, "", fmt.Errorf("no filesystem available for repo template")
		}
		path := strings.TrimPrefix(partialPath, "@")
		fsys := ctx.Partials[0]
		return ctx.resolveTemplateFile(path, fsys)

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
		return ctx.resolveTemplateFile(fullPath, currentTemplateFS)

	default:
		// Bare path - search through all filesystems in order
		if len(ctx.Partials) == 0 {
			return nil, "", fmt.Errorf("no filesystems available for template lookup")
		}

		// Search through all filesystems
		return ctx.resolveTemplateFile(partialPath, ctx.Partials...)
	}
}
