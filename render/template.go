package render

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

// LoadTemplate loads a template from a path
func LoadTemplate(ctx *RenderContext, path string) (*Template, error) {
	var content string
	var err error

	// Load content from path
	content, err = ctx.ResolvePartial(path)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter
	_, frontMatterContent, body, err := ParseFrontMatter(content)
	if err != nil {
		return nil, err
	}

	// Create template with empty Meta by default
	tmpl := &Template{
		Path: path,
		Body: body,
		Meta: Meta{},
	}

	// Parse frontmatter into Meta if present
	if frontMatterContent != "" {
		if err := ParseToml(frontMatterContent, &tmpl.Meta); err != nil {
			return nil, err
		}
	}

	return tmpl, nil
}
