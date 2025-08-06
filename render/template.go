package render

import (
	"github.com/hayeah/fork2/vibe"
	"io/fs"
)

// Template represents a template with its content and metadata
type Template struct {
	Path           string          // repo-relative
	Body           string          // content with front-matter stripped
	Config         *vibe.AppConfig // parsed TOML front-matter (nil if none)
	RawFrontMatter string          // full unparsed front-matter block, empty when none
	FS             fs.FS           // filesystem where the template was found
}

func NewTemplate(content string) (*Template, error) {
	_, rawFM, body, err := ParseFrontMatter(content)
	if err != nil {
		return nil, err
	}

	var config *vibe.AppConfig
	if rawFM != "" {
		config = &vibe.AppConfig{}
		if err := ParseToml(rawFM, config); err != nil {
			return nil, err
		}
	}

	return &Template{
		Path:           "",
		Body:           body,
		Config:         config,
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
