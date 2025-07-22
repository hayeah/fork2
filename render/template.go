package render

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// LoadTemplate is a convenience helper that turns an on-disk file into a
// *Template.
//
// Behaviour
//   - If path starts with “/” (absolute) we load it verbatim.
//   - Otherwise we treat it as relative to the current working directory.
//   - Template.FS is an os.DirFS rooted at the directory that contains the file –
//     this allows relative-partial resolution to work exactly the same way it
//     does inside a repo.
//
// Front-matter is parsed with existing ParseFrontMatter / ParseToml helpers.
func LoadTemplate(path string) (*Template, error) {
	// Resolve to absolute path when the caller supplied a relative one.
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(cwd, path)
	}

	// Strip the leading "/" so Renderer's relative-path logic works when it
	// later calls filepath.Dir on Template.Path.
	relPath := strings.TrimPrefix(path, string(filepath.Separator))

	// Create a filesystem rooted at the directory containing the template
	fsys := os.DirFS(filepath.Dir(path))

	// Use the base filename for loading from the filesystem
	baseName := filepath.Base(path)

	// Load the template using the filesystem
	tmpl, err := LoadTemplateFS(baseName, fsys)
	if err != nil {
		return nil, err
	}

	// Update the path to be the relative path
	tmpl.Path = relPath

	return tmpl, nil
}
