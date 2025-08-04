package vibe

import (
	"strings"
)

// TemplateNames represents a list of template names that can be parsed from
// a semicolon-separated string (e.g., "foo;bar;baz")
type TemplateNames []string

// String returns the semicolon-separated string representation
func (tn TemplateNames) String() string {
	return strings.Join(tn, ";")
}

// UnmarshalText implements encoding.TextUnmarshaler to parse from TOML
func (tn *TemplateNames) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		*tn = nil
		return nil
	}

	// Split by semicolon and trim whitespace
	parts := strings.Split(s, ";")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	*tn = result
	return nil
}

// AppConfig groups runtime configuration values for the vibe application.
// It combines values from command-line arguments and template frontmatter.
type AppConfig struct {
	// From command-line arguments
	RootPath         string
	WorkingDirectory string
	DataPairs        []string
	Output           string // Output destination: '-' for stdout, file path, or empty for clipboard

	// From template frontmatter (can be overridden by command-line)
	Root          string        `toml:"root"`
	TemplatePaths []string      `toml:"templatePaths"`
	Content       []string      `toml:"content"`
	Mode          string        `toml:"mode"`
	Layout        TemplateNames `toml:"layout"`
	Select        string        `toml:"select"`
	Dirtree       string        `toml:"dirtree"`
	Before        TemplateNames `toml:"before"`
	After         TemplateNames `toml:"after"`
}
