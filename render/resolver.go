package render

import (
	"fmt"
	"io/fs"
	"os"
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

	// For relative paths, we need to store the resolved path for correct subsequent relative resolution
	// For other paths (system, repo, bare), we can keep the original
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		// Keep the resolved file path for relative paths
		tmpl.Path = filePath
	} else {
		// Keep the original path for non-relative paths
		tmpl.Path = path
	}

	return tmpl, nil
}

// NewResolver creates a new Resolver with the given filesystem stack.
func NewResolver(mode string, partials ...fs.FS) *Resolver {
	return &Resolver{Partials: partials, Mode: mode}
}

// resolveTemplateFile resolves a template path to a filesystem and path.
// It searches through all provided filesystems in order and applies the following rules:
//
// Rules based on file extension:
// Paths with .md extension (e.g., "plan.md"):
//   - Must match exactly "plan.md"
//   - Search each filesystem for exact path
//   - Return error if not found in any filesystem
//
// Paths without extension (e.g., "plan"):
//   - If mode is set (e.g., mode="cc"), try in order:
//     a. "{base}.{mode}" (e.g., "plan.cc")
//     b. "{base}.{mode}.md" (e.g., "plan.cc.md")
//   - Then try:
//     c. "{base}" exact path (e.g., "plan")
//     d. "{base}.md" as fallback (e.g., "plan.md")
//   - Search each filesystem with all variants before moving to next
//   - Return error if not found in any filesystem
func (r *Resolver) resolveTemplateFile(base string, filesystems ...fs.FS) (fs.FS, string, error) {
	ext := filepath.Ext(base)

	// Determine search paths based on extension
	var searchPaths []string
	switch {
	case ext == ".md":
		// .md files require exact match only
		searchPaths = []string{base}
	case ext == "":
		// No extension: try mode variants, exact, then .md fallback
		searchPaths = []string{}
		if r.Mode != "" {
			searchPaths = append(searchPaths,
				base+"."+r.Mode,       // plan.{mode}
				base+"."+r.Mode+".md", // plan.{mode}.md
			)
		}
		searchPaths = append(searchPaths,
			base,       // plan
			base+".md", // plan.md
		)
	default:
		// Other extensions: exact match only
		searchPaths = []string{base}
	}

	// Search through filesystems
	for _, fsys := range filesystems {
		for _, path := range searchPaths {
			if _, err := fs.Stat(fsys, path); err == nil {
				return fsys, path, nil
			}
		}
	}

	return nil, "", fmt.Errorf("template %q not found in any filesystem", base)
}

// ResolvePartialPath determines which FS and file should be used for a given partial path.
// It uses the following rules:
// 1. Absolute path /path/to/template.md - use OS filesystem directly
// 2. System Template <vibe/coder> - use the last FS in Partials
// 3. Repo Root Template @common/header - use the first FS in Partials
// 4. Local Template ./helpers/buttons - use the same FS as the current template
// 5. Bare path common/header - search all FS in order until found
//
// Mode-specific behavior:
// - Paths with .md extension (e.g., "plan.md") require exact match; error if not found
// - Paths with other extensions ignore mode variants and search all filesystems
// - Paths WITHOUT extensions (e.g., "plan") try mode variants first (plan.cc.md, plan.cc) then fall back to defaults
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
	case filepath.IsAbs(partialPath):
		// Absolute path - use OS filesystem directly
		dir := filepath.Dir(partialPath)
		base := filepath.Base(partialPath)
		fsys := os.DirFS(dir)
		return ctx.resolveTemplateFile(base, fsys)

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
			// When no parent template exists, resolve relative paths from cwd
			abs, err := filepath.Abs(partialPath)
			if err != nil {
				return nil, "", err
			}
			dir := filepath.Dir(abs)
			base := filepath.Base(abs)
			fsys := &CwdFS{Base: dir}
			return ctx.resolveTemplateFile(base, fsys)
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
		fsys, path, err := ctx.resolveTemplateFile(partialPath, ctx.Partials...)
		if err != nil {
			return nil, "", fmt.Errorf("template %q not found in any filesystem", partialPath)
		}
		return fsys, path, nil
	}
}
