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
	FS   fs.FS  // filesystem where the template was found
}
