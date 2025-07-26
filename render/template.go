package render

import (
	"io/fs"
)

// FrontMatter contains metadata parsed from template frontmatter
type FrontMatter struct {
	Layout  string `toml:"layout"`
	Select  string `toml:"select"`
	Dirtree string `toml:"dirtree"`
	Before  string `toml:"before"`
	After   string `toml:"after"`
	Mode    string `toml:"mode"`
}

// Template represents a template with its content and metadata
type Template struct {
	Path           string      // repo-relative
	Body           string      // content with front-matter stripped
	FrontMatter    FrontMatter // parsed TOML front-matter (zero if none)
	RawFrontMatter string      // full unparsed front-matter block, empty when none
	FS             fs.FS       // filesystem where the template was found
}

func NewTemplate(content string) (*Template, error) {
	_, rawFM, body, err := ParseFrontMatter(content)
	if err != nil {
		return nil, err
	}

	var meta FrontMatter
	if rawFM != "" {
		if err := ParseToml(rawFM, &meta); err != nil {
			return nil, err
		}
	}

	return &Template{
		Path:           "",
		Body:           body,
		FrontMatter:    meta,
		RawFrontMatter: rawFM,
		FS:             nil,
	}, nil
}

// LoadTemplateFS is a helper that loads a template from a given filesystem.
//
// Front-matter is parsed with existing ParseFrontMatter / ParseToml helpers.
func LoadTemplateFS(path string, fsys fs.FS) (*Template, error) {
	blob, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}

	t, err := NewTemplate(string(blob))
	if err != nil {
		return nil, err
	}

	t.Path = path
	t.FS = fsys

	return t, nil
}
