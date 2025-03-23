package render

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestFS creates a test filesystem with the given files
func createTestFS(files map[string]string) fs.FS {
	fsys := fstest.MapFS{}
	for path, content := range files {
		fsys[path] = &fstest.MapFile{
			Data: []byte(content),
		}
	}
	return fsys
}

func TestResolvePartialPath(t *testing.T) {
	// Create test filesystems
	systemFS := createTestFS(map[string]string{
		"vibe/coder": "System Coder Template",
	})

	repoFS := createTestFS(map[string]string{
		"common/header":          "Repo Header Template",
		"templates/local/helper": "Local Helper Template",
	})

	// Create render context
	ctx := &RenderContext{
		CurrentTemplatePath: "templates/main.md",
		SystemPartials:      systemFS,
		RepoPartials:        repoFS,
	}

	tests := []struct {
		name               string
		partialPath        string
		expectedFSType     string // "system", "repo", or "nil"
		expectedFile       string
		expectedErrMessage string
	}{
		{
			name:           "System template",
			partialPath:    "<vibe/coder>",
			expectedFSType: "system",
			expectedFile:   "vibe/coder",
		},
		{
			name:           "Repo root template",
			partialPath:    "@common/header",
			expectedFSType: "repo",
			expectedFile:   "common/header",
		},
		{
			name:           "Local template",
			partialPath:    "./local/helper",
			expectedFSType: "repo",
			expectedFile:   "templates/local/helper",
		},
		{
			name:               "Invalid template path",
			partialPath:        "invalid/path",
			expectedFSType:     "nil",
			expectedFile:       "",
			expectedErrMessage: "invalid partial path format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			gotFS, gotFile, err := ctx.ResolvePartialPath(tt.partialPath)

			if tt.expectedErrMessage != "" {
				assert.Error(err)
				assert.Contains(err.Error(), tt.expectedErrMessage)
				assert.Equal("", gotFile)
				assert.Nil(gotFS)
				return
			}

			assert.NoError(err)
			assert.Equal(tt.expectedFile, gotFile)

			// Check the filesystem type instead of comparing directly
			switch tt.expectedFSType {
			case "system":
				assert.Equal(systemFS, gotFS)
			case "repo":
				assert.Equal(repoFS, gotFS)
			case "nil":
				assert.Nil(gotFS)
			}
		})
	}
}
func TestPartialRendering(t *testing.T) {
	// Create test filesystems with nested partials
	systemFS := createTestFS(map[string]string{
		"vibe/coder":  "System {{ partial \"<vibe/footer>\" }}",
		"vibe/footer": "Footer",
	})

	repoFS := createTestFS(map[string]string{
		"common/header":          "Repo Header with {{ .Value }}",
		"templates/local/helper": "Local Helper that uses {{ partial \"@common/header\" }}",
	})

	// Create render context
	ctx := &RenderContext{
		CurrentTemplatePath: "templates/main.md",
		SystemPartials:      systemFS,
		RepoPartials:        repoFS,
	}

	// Create a renderer with the context
	renderer := NewRenderer(ctx)

	// Test data
	testData := struct {
		Value string
	}{
		Value: "test value",
	}

	t.Run("simple partial", func(t *testing.T) {
		assert := assert.New(t)

		// Test a simple partial using Render with empty layoutPath
		result, err := renderer.RenderPartial("@common/header", testData)
		assert.NoError(err, "Partial rendering should not return an error")
		assert.Equal("Repo Header with test value", result, "Partial should render with variable interpolation")
	})

	t.Run("nested partial", func(t *testing.T) {
		assert := assert.New(t)

		// Test a nested partial using Render with empty layoutPath
		result, err := renderer.RenderPartial("<vibe/coder>", testData)
		assert.NoError(err, "Nested partial rendering should not return an error")
		assert.Equal("System Footer", result, "Nested partial should be correctly rendered")
	})

	t.Run("local partial with repo reference", func(t *testing.T) {
		assert := assert.New(t)

		// Test a local partial that references a repo partial
		result, err := renderer.RenderPartial("./local/helper", testData)
		assert.NoError(err, "Local partial rendering should not return an error")
		assert.Equal("Local Helper that uses Repo Header with test value",
			result, "Local partial should correctly include repo partial with variable interpolation")
	})
}

func TestRenderer(t *testing.T) {
	assert := require.New(t)

	// Create test filesystems
	systemFS := createTestFS(map[string]string{
		"vibe/coder": "Coder: {{ .System }}",
	})

	repoFS := createTestFS(map[string]string{
		"layouts/main.md": `{{ partial "<vibe/coder>" }}

# Tools
{{ .ToolList }}

# Directory Listing
{{ .ListDirectory }}

# User Instructions
{{ block "main" . }}{{ end }}`,

		"templates/user.md": "Hello from the user",
	})

	// Create render context
	ctx := &RenderContext{
		CurrentTemplatePath: "",
		SystemPartials:      systemFS,
		RepoPartials:        repoFS,
	}

	// Create test data
	testData := struct {
		System        string
		ListDirectory []string
		SelectedFiles []string
		ToolList      string
	}{
		System:        "Linux",
		ListDirectory: []string{"file1.go", "file2.md"},
		SelectedFiles: []string{"selected1.go"},
		ToolList:      "Tool1, Tool2, Tool3",
	}

	// Create a renderer with the context
	renderer := NewRenderer(ctx)

	// Render with the layout using path to user content
	result, err := renderer.Render("@templates/user.md", "@layouts/main.md", testData)
	assert.NoError(err, "Render should not return an error")

	assert.Equal(`Coder: Linux

# Tools
Tool1, Tool2, Tool3

# Directory Listing
[file1.go file2.md]

# User Instructions
Hello from the user`, result)

}
