package render

import (
	"io/fs"
)

// Meta contains metadata parsed from template frontmatter
type Meta struct {
	Layout string `toml:"layout"`
	Select string `toml:"select"`
}

// Template represents a template with its content and metadata
type Template struct {
	Path string // repo-relative
	Body string // content without FM
	Meta Meta   // parsed FM (empty if none)
}

// LoadTemplate loads a template from a path and returns the template along with the filesystem it was loaded from
func LoadTemplate(ctx *Resolver, path string, currentTemplatePath string, currentTemplateFS fs.FS) (*Template, fs.FS, error) {
	var content string
	var fsys fs.FS
	var err error

	// Resolve the path to get the content
	content, fsys, err = ctx.ResolvePartial(path, currentTemplatePath, currentTemplateFS)
	if err != nil {
		return nil, nil, err
	}

	// Parse frontmatter
	_, frontMatterContent, body, err := ParseFrontMatter(content)
	if err != nil {
		return nil, nil, err
	}

	// Create template with empty Meta by default
	meta := Meta{}
	if frontMatterContent != "" {
		if err := ParseToml(frontMatterContent, &meta); err != nil {
			return nil, nil, err
		}
	}

	// Create and return the template with the parsed metadata
	tmpl := &Template{
		Path: path,
		Body: body,
		Meta: meta,
	}

	return tmpl, fsys, nil
}
