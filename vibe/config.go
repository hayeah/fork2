package vibe

// AppConfig groups runtime configuration values for the vibe application.
// It combines values from command-line arguments and template frontmatter.
type AppConfig struct {
	// From command-line arguments
	RootPath         string
	WorkingDirectory string
	DataPairs        []string
	Output           string // Output destination: '-' for stdout, file path, or empty for clipboard

	// From template frontmatter (can be overridden by command-line)
	Root          string   `toml:"root"`
	TemplatePaths []string `toml:"templatePaths"`
	Content       []string `toml:"content"`
	Mode          string   `toml:"mode"`
	Layout        string   `toml:"layout"`
	Select        string   `toml:"select"`
	Dirtree       string   `toml:"dirtree"`
	Before        string   `toml:"before"`
	After         string   `toml:"after"`
}
