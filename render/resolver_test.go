package render

import (
	"testing"

	"github.com/hayeah/fork2/internal/assert"
)

func TestResolveTemplateFile_ModeHandling(t *testing.T) {
	// Create test filesystem with mode-specific variants
	repoFS := createTestFS(map[string]string{
		"plan.md":        "Default plan",
		"plan.cc.md":     "CC mode plan",
		"config.toml":    "Default config",
		"config.cc.toml": "CC mode config",
		"template":       "Default template without extension",
		"template.cc":    "CC mode template without extension",
		"template.md":    "Default template with md",
		"template.cc.md": "CC mode template with md",
	})

	// Create a second filesystem for testing search behavior
	systemFS := createTestFS(map[string]string{
		"system.toml": "System config",
	})

	assert := assert.New(t)

	t.Run("md extension requires exact match", func(t *testing.T) {
		// With mode="cc", .md files require exact match
		resolver := NewResolver("cc", repoFS)

		// Test: "plan.md" should resolve to exact "plan.md", NOT "plan.cc.md"
		fsys, path, err := resolver.resolveTemplateFile("plan.md", repoFS)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("plan.md", path)

		// Test: non-existent .md file should error immediately
		_, _, err = resolver.resolveTemplateFile("nonexistent.md", repoFS)
		assert.Error(err)
		assert.Contains(err.Error(), "not found in any filesystem")
	})

	t.Run("other extensions search all filesystems", func(t *testing.T) {
		resolver := NewResolver("cc", repoFS, systemFS)

		// Test: "config.toml" should resolve to "config.toml" in first filesystem
		fsys, path, err := resolver.resolveTemplateFile("config.toml", repoFS, systemFS)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("config.toml", path)

		// Test: "system.toml" should be found in second filesystem
		fsys, path, err = resolver.resolveTemplateFile("system.toml", repoFS, systemFS)
		assert.NoError(err)
		assert.Equal(systemFS, fsys)
		assert.Equal("system.toml", path)
	})

	t.Run("no extension uses mode variants", func(t *testing.T) {
		// With mode="cc", paths without extensions should try mode variants first
		resolver := NewResolver("cc", repoFS)

		// Test: "template" should resolve to "template.cc" (mode variant without .md comes first)
		fsys, path, err := resolver.resolveTemplateFile("template", repoFS)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("template.cc", path)

		// Test: "plan" should resolve to "plan.cc.md" (only this variant exists)
		fsys, path, err = resolver.resolveTemplateFile("plan", repoFS)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("plan.cc.md", path)
	})

	t.Run("no mode falls back to defaults", func(t *testing.T) {
		// Without mode, should use default files
		resolver := NewResolver("", repoFS)

		// Test: "plan.md" resolves to itself
		fsys, path, err := resolver.resolveTemplateFile("plan.md", repoFS)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("plan.md", path)

		// Test: "template" resolves to "template" or "template.md"
		fsys, path, err = resolver.resolveTemplateFile("template", repoFS)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("template", path) // Exact match found
	})

	t.Run("non-existent files with explicit extension", func(t *testing.T) {
		resolver := NewResolver("cc", repoFS)

		// Test: Non-existent .md file should error with specific message
		_, _, err := resolver.resolveTemplateFile("nonexistent.md", repoFS)
		assert.Error(err)
		assert.Contains(err.Error(), "not found in any filesystem")

		// Test: Non-existent file with other extension continues searching
		_, _, err = resolver.resolveTemplateFile("nonexistent.toml", repoFS)
		assert.Error(err)
		assert.Contains(err.Error(), "not found")
	})

	t.Run("md files search all filesystems until found", func(t *testing.T) {
		// Create filesystems with different .md files
		fs1 := createTestFS(map[string]string{
			"first.md": "First FS",
		})
		fs2 := createTestFS(map[string]string{
			"second.md": "Second FS",
		})
		fs3 := createTestFS(map[string]string{
			"third.md": "Third FS",
		})

		resolver := NewResolver("", fs1, fs2, fs3)

		// Test: .md file found in first filesystem
		fsys, path, err := resolver.resolveTemplateFile("first.md", fs1, fs2, fs3)
		assert.NoError(err)
		assert.Equal(fs1, fsys)
		assert.Equal("first.md", path)

		// Test: .md file found in second filesystem
		fsys, path, err = resolver.resolveTemplateFile("second.md", fs1, fs2, fs3)
		assert.NoError(err)
		assert.Equal(fs2, fsys)
		assert.Equal("second.md", path)

		// Test: .md file found in third filesystem
		fsys, path, err = resolver.resolveTemplateFile("third.md", fs1, fs2, fs3)
		assert.NoError(err)
		assert.Equal(fs3, fsys)
		assert.Equal("third.md", path)

		// Test: .md file not found in any filesystem
		_, _, err = resolver.resolveTemplateFile("missing.md", fs1, fs2, fs3)
		assert.Error(err)
		assert.Contains(err.Error(), "not found in any filesystem")
	})
}

func TestResolvePartialPath_WithModeAndExtension(t *testing.T) {
	// Create test filesystem with mode-specific variants
	repoFS := createTestFS(map[string]string{
		"templates/plan.md":    "Default plan",
		"templates/plan.cc.md": "CC mode plan",
		"templates/helper.md":  "Default helper",
		"templates/page":       "Page without extension",
		"templates/page.cc.md": "CC mode page",
	})

	assert := assert.New(t)

	t.Run("relative path with explicit extension ignores mode", func(t *testing.T) {
		resolver := NewResolver("cc", repoFS)

		// Current template for relative resolution
		currentTemplate := &Template{
			Path: "templates/main.md",
			FS:   repoFS,
		}

		// Test: "./plan.md" should resolve to "templates/plan.md", NOT "templates/plan.cc.md"
		fsys, path, err := resolver.ResolvePartialPath("./plan.md", currentTemplate)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("templates/plan.md", path)
	})

	t.Run("relative path without extension uses mode", func(t *testing.T) {
		resolver := NewResolver("cc", repoFS)

		// Current template for relative resolution
		currentTemplate := &Template{
			Path: "templates/main.md",
			FS:   repoFS,
		}

		// Test: "./page" should resolve to "templates/page.cc.md"
		fsys, path, err := resolver.ResolvePartialPath("./page", currentTemplate)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("templates/page.cc.md", path)
	})

	t.Run("repo root path with explicit extension ignores mode", func(t *testing.T) {
		resolver := NewResolver("cc", repoFS)

		// Test: "@templates/plan.md" should resolve to "templates/plan.md"
		fsys, path, err := resolver.ResolvePartialPath("@templates/plan.md", nil)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("templates/plan.md", path)
	})

	t.Run("bare path with explicit extension ignores mode", func(t *testing.T) {
		resolver := NewResolver("cc", repoFS)

		// Test: "templates/helper.md" should resolve to exact match
		fsys, path, err := resolver.ResolvePartialPath("templates/helper.md", nil)
		assert.NoError(err)
		assert.Equal(repoFS, fsys)
		assert.Equal("templates/helper.md", path)
	})
}
