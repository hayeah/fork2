package render

import (
	"testing"

	"github.com/hayeah/fork2/internal/assert"
)

func TestRawFrontMatterCapture(t *testing.T) {
	// Create test filesystem with templates containing frontmatter
	repoFS := createTestFS(map[string]string{
		"simple.md":  "---toml\nlayout=\"base.md\"\n---\nHello",
		"complex.md": "---toml\nlayout=\"base.md\"\nselect=\"*.go\"\ndirtree=\"cmd/;internal/\"\n---\nContent",
		"none.md":    "No frontmatter here",
	})
	ctx := NewResolver(repoFS)
	assert := assert.New(t)

	// Test simple frontmatter capture
	tmpl, err := ctx.LoadTemplate("simple.md", nil)
	assert.NoError(err)
	assert.Equal("layout=\"base.md\"", tmpl.RawFrontMatter)
	assert.Equal("base.md", tmpl.Meta.Layout)
	assert.Equal("Hello", tmpl.Body)

	// Test complex frontmatter capture
	tmpl, err = ctx.LoadTemplate("complex.md", nil)
	assert.NoError(err)
	assert.Equal("layout=\"base.md\"\nselect=\"*.go\"\ndirtree=\"cmd/;internal/\"", tmpl.RawFrontMatter)
	assert.Equal("base.md", tmpl.Meta.Layout)
	assert.Equal("*.go", tmpl.Meta.Select)
	assert.Equal("cmd/;internal/", tmpl.Meta.Dirtree)
	assert.Equal("Content", tmpl.Body)

	// Test no frontmatter
	tmpl, err = ctx.LoadTemplate("none.md", nil)
	assert.NoError(err)
	assert.Equal("", tmpl.RawFrontMatter)
	assert.Equal("", tmpl.Meta.Layout)
	assert.Equal("No frontmatter here", tmpl.Body)
}
