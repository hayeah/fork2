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
		"common/header":                 "Repo Header Template",
		"templates/local/helper":        "Local Helper Template",
		"templates/subdir/component.md": "Component Template",
		"components/shared/footer.md":   "Footer Template",
	})

	// Create render context with initial path
	ctx := &RenderContext{
		SystemPartials: systemFS,
		RepoPartials:   repoFS,
	}

	tests := []struct {
		name               string
		currentPath        string // Override CurrentTemplatePath if not empty
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
			currentPath:    "templates/main.md",
			partialPath:    "@common/header",
			expectedFSType: "repo",
			expectedFile:   "common/header",
		},
		{
			name:           "Repo root template from deeper",
			currentPath:    "templates/foo/bar/main.md",
			partialPath:    "@common/header",
			expectedFSType: "repo",
			expectedFile:   "common/header",
		},
		{
			currentPath:    "templates/main.md",
			name:           "Local template",
			partialPath:    "./local/helper",
			expectedFSType: "repo",
			expectedFile:   "templates/local/helper",
		},
		{
			name:           "Local template from subfolder",
			currentPath:    "templates/subdir/page.md",
			partialPath:    "./component.md",
			expectedFSType: "repo",
			expectedFile:   "templates/subdir/component.md",
		},
		{
			name:           "Relative path up one directory",
			currentPath:    "templates/subdir/page.md",
			partialPath:    "../local/helper",
			expectedFSType: "repo",
			expectedFile:   "templates/local/helper",
		},
		{
			name:           "Relative path up multiple directories",
			currentPath:    "templates/subdir/nested/page.md",
			partialPath:    "../../local/helper",
			expectedFSType: "repo",
			expectedFile:   "templates/local/helper",
		},
		{
			name:           "Complex relative path traversal",
			currentPath:    "templates/subdir/page.md",
			partialPath:    "../../components/shared/footer.md",
			expectedFSType: "repo",
			expectedFile:   "components/shared/footer.md",
		},
		{
			name:               "Empty CurrentTemplatePath with local path",
			currentPath:        "",
			partialPath:        "./local/helper",
			expectedFSType:     "nil",
			expectedFile:       "",
			expectedErrMessage: "cannot resolve local path without CurrentTemplatePath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			ctx.CurrentTemplatePath = tt.currentPath

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

func TestRelativePathResolution(t *testing.T) {
	// Create test filesystem with nested directory structure
	repoFS := createTestFS(map[string]string{
		"components/button.md":          "Button Component",
		"components/form/input.md":      "Form Input Component",
		"templates/page.md":             "Page Template",
		"templates/blog/post.md":        "Blog Post Template",
		"templates/blog/list.md":        "Blog List Template",
		"templates/admin/dashboard.md":  "Admin Dashboard",
		"templates/admin/users/list.md": "Admin Users List",
		"templates/shared/header.md":    "Shared Header",
		"templates/shared/footer.md":    "Shared Footer",
	})

	// Table driven tests for different current template paths and relative references
	tests := []struct {
		name           string
		currentPath    string
		partialPath    string
		expectedFile   string
		expectedErrMsg string
	}{
		{
			name:         "Simple relative path from template",
			currentPath:  "templates/page.md",
			partialPath:  "./shared/header.md",
			expectedFile: "templates/shared/header.md",
		},
		{
			name:         "Relative path from nested template",
			currentPath:  "templates/blog/post.md",
			partialPath:  "../shared/footer.md",
			expectedFile: "templates/shared/footer.md",
		},
		{
			name:         "Relative path to parent directory and different branch",
			currentPath:  "templates/admin/users/list.md",
			partialPath:  "../../blog/list.md",
			expectedFile: "templates/blog/list.md",
		},
		{
			name:         "Relative path going up to root level component",
			currentPath:  "templates/admin/dashboard.md",
			partialPath:  "../../components/button.md",
			expectedFile: "components/button.md",
		},
		{
			name:         "Complex navigation with multiple ups and downs",
			currentPath:  "templates/blog/list.md",
			partialPath:  "../admin/users/../dashboard.md",
			expectedFile: "templates/admin/dashboard.md",
		},
		{
			name:           "Missing CurrentTemplatePath",
			currentPath:    "",
			partialPath:    "./shared/header.md",
			expectedErrMsg: "cannot resolve local path without CurrentTemplatePath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			ctx := &RenderContext{
				CurrentTemplatePath: tt.currentPath,
				RepoPartials:        repoFS,
			}

			gotFS, gotFile, err := ctx.ResolvePartialPath(tt.partialPath)

			if tt.expectedErrMsg != "" {
				assert.Error(err)
				assert.Contains(err.Error(), tt.expectedErrMsg)
				return
			}

			assert.NoError(err)
			assert.Equal(tt.expectedFile, gotFile)
			assert.Equal(repoFS, gotFS)
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
