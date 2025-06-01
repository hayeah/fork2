package render

import (
	"strings"
	"testing"

	"github.com/hayeah/fork2/internal/assert"
)

func TestBeforeAfterRendering(t *testing.T) {
	// Create test files with before/after content
	repoFS := createTestFS(map[string]string{
		// Main template with before/after
		"main.md": `---toml
before = "header.md"
after = "footer.md"
---
MAIN CONTENT`,

		// Template with layout and before/after
		"with_layout.md": `---toml
layout = "base.md"
before = "pre.md"
after = "post.md"
---
USER CONTENT`,

		// Template with multiple before/after files
		"multi.md": `---toml
before = "first.md;second.md"
after = "third.md;fourth.md"
---
MULTI CONTENT`,

		// Template with bang prefix for rendering
		"render_test.md": `---toml
before = "!render_me.md"
after = "include_me.md"
---
RENDER TEST`,

		// Support files
		"header.md":     "HEADER",
		"footer.md":     "FOOTER",
		"base.md":       "BASE[{{ .Content }}]",
		"pre.md":        "PRE",
		"post.md":       "POST",
		"first.md":      "FIRST",
		"second.md":     "SECOND",
		"third.md":      "THIRD",
		"fourth.md":     "FOURTH",
		"render_me.md":  "Hello {{ .Name }}!",
		"include_me.md": "INCLUDED",
	})

	ctx := NewResolver(repoFS)
	renderer := NewRenderer(ctx, nil)
	assert := assert.New(t)

	t.Run("simple before/after", func(t *testing.T) {
		tmpl, err := ctx.LoadTemplate("main.md", nil)
		assert.NoError(err)

		output, err := renderer.RenderTemplate(tmpl, &testContent{})
		assert.NoError(err)

		// Should be: HEADER\nFOOTER\nMAIN CONTENT (user content at tail)
		lines := strings.Split(output, "\n")
		assert.Equal(3, len(lines))
		assert.Equal("HEADER", lines[0])
		assert.Equal("FOOTER", lines[1])
		assert.Equal("MAIN CONTENT", lines[2])
	})

	t.Run("with layout and before/after", func(t *testing.T) {
		tmpl, err := ctx.LoadTemplate("with_layout.md", nil)
		assert.NoError(err)

		output, err := renderer.RenderTemplate(tmpl, &testContent{})
		assert.NoError(err)

		// Should be: PRE\nBASE[]\nPOST\nUSER CONTENT
		lines := strings.Split(output, "\n")
		assert.Equal(4, len(lines))
		assert.Equal("PRE", lines[0])
		assert.Equal("BASE[]", lines[1])
		assert.Equal("POST", lines[2])
		assert.Equal("USER CONTENT", lines[3])
	})

	t.Run("multiple before/after files", func(t *testing.T) {
		tmpl, err := ctx.LoadTemplate("multi.md", nil)
		assert.NoError(err)

		output, err := renderer.RenderTemplate(tmpl, &testContent{})
		assert.NoError(err)

		// Should be: FIRST\nSECOND\nTHIRD\nFOURTH\nMULTI CONTENT (user content at tail)
		lines := strings.Split(output, "\n")
		assert.Equal(5, len(lines))
		assert.Equal("FIRST", lines[0])
		assert.Equal("SECOND", lines[1])
		assert.Equal("THIRD", lines[2])
		assert.Equal("FOURTH", lines[3])
		assert.Equal("MULTI CONTENT", lines[4])
	})

	t.Run("bang prefix for template rendering", func(t *testing.T) {
		tmpl, err := ctx.LoadTemplate("render_test.md", nil)
		assert.NoError(err)

		// Use testContent with Name field for template
		data := &testContentWithName{testContent: testContent{}, name: "World"}
		output, err := renderer.RenderTemplate(tmpl, data)
		assert.NoError(err)

		// Should be: Hello World!\nINCLUDED\nRENDER TEST (user content at tail)
		lines := strings.Split(output, "\n")
		assert.Equal(3, len(lines))
		assert.Equal("Hello World!", lines[0])
		assert.Equal("INCLUDED", lines[1])
		assert.Equal("RENDER TEST", lines[2])
	})
}

// Extended test content with Name field for template rendering
type testContentWithName struct {
	testContent
	name string
}

func (c *testContentWithName) Name() string { return c.name }

func TestBeforeAfterWithComplexLayouts(t *testing.T) {
	repoFS := createTestFS(map[string]string{
		// Complex nested layout scenario
		"page.md": `---toml
layout = "inner.md;outer.md"
before = "page_before.md"
after = "page_after.md"
---
PAGE CONTENT`,

		"inner.md": `---toml
before = "inner_before.md"
after = "inner_after.md"
---
INNER[{{ .Content }}]`,

		"outer.md": `---toml
before = "outer_before.md"
after = "outer_after.md"
---
OUTER[{{ .Content }}]`,

		// Before/after files
		"page_before.md":  "PAGE_BEFORE",
		"page_after.md":   "PAGE_AFTER",
		"inner_before.md": "INNER_BEFORE",
		"inner_after.md":  "INNER_AFTER",
		"outer_before.md": "OUTER_BEFORE",
		"outer_after.md":  "OUTER_AFTER",
	})

	ctx := NewResolver(repoFS)
	renderer := NewRenderer(ctx, nil)
	assert := assert.New(t)

	tmpl, err := ctx.LoadTemplate("page.md", nil)
	assert.NoError(err)

	output, err := renderer.RenderTemplate(tmpl, &testContent{})
	assert.NoError(err)

	// Expected order with new layout system:
	// PAGE_BEFORE (from page.md)
	// INNER_BEFORE (from inner.md - first layout)
	// INNER[] (inner.md body with empty content)
	// INNER_AFTER (from inner.md)
	// wrapped in OUTER[...] from outer.md
	// OUTER_BEFORE/OUTER_AFTER from outer.md
	// PAGE_AFTER (from page.md)
	// PAGE CONTENT (user content at tail)
	expected := []string{
		"PAGE_BEFORE",
		"INNER_BEFORE",
		"INNER_AFTER",
		"INNER[OUTER_BEFORE",
		"OUTER_AFTER",
		"OUTER[]]",
		"PAGE_AFTER",
		"PAGE CONTENT",
	}

	assert.Equal(strings.Join(expected, "\n"), output)
}

func TestBeforeAfterEmptyContent(t *testing.T) {
	repoFS := createTestFS(map[string]string{
		// Template with before/after that use .Content
		"test.md": `---toml
before = "!before_template.md"
after = "!after_template.md"
---
TEST`,

		"before_template.md": "BEFORE:{{ .Content }}",
		"after_template.md":  "AFTER:{{ .Content }}",
	})

	ctx := NewResolver(repoFS)
	renderer := NewRenderer(ctx, nil)
	assert := assert.New(t)

	tmpl, err := ctx.LoadTemplate("test.md", nil)
	assert.NoError(err)

	data := &testContent{content: "ORIGINAL"}
	output, err := renderer.RenderTemplate(tmpl, data)
	assert.NoError(err)

	// Before and after templates should see empty .Content
	lines := strings.Split(output, "\n")
	assert.Equal(3, len(lines))
	assert.Equal("BEFORE:", lines[0])
	assert.Equal("AFTER:", lines[1])
	assert.Equal("TEST", lines[2])

	// Original content should be preserved
	assert.Equal("ORIGINAL", data.Content())
}

func TestBeforeAfterErrorHandling(t *testing.T) {
	repoFS := createTestFS(map[string]string{
		"missing_before.md": `---toml
before = "nonexistent.md"
---
CONTENT`,

		"missing_after.md": `---toml
after = "nonexistent.md"
---
CONTENT`,

		"invalid_template.md": `---toml
before = "!invalid.md"
---
CONTENT`,

		"invalid.md": "{{ .BadMethod }}",
	})

	ctx := NewResolver(repoFS)
	renderer := NewRenderer(ctx, nil)
	assert := assert.New(t)

	t.Run("missing before file", func(t *testing.T) {
		tmpl, err := ctx.LoadTemplate("missing_before.md", nil)
		assert.NoError(err)

		_, err = renderer.RenderTemplate(tmpl, &testContent{})
		assert.Error(err)
		assert.Contains(err.Error(), "error processing before files")
		assert.Contains(err.Error(), "nonexistent.md")
	})

	t.Run("missing after file", func(t *testing.T) {
		tmpl, err := ctx.LoadTemplate("missing_after.md", nil)
		assert.NoError(err)

		_, err = renderer.RenderTemplate(tmpl, &testContent{})
		assert.Error(err)
		assert.Contains(err.Error(), "error processing after files")
		assert.Contains(err.Error(), "nonexistent.md")
	})

	t.Run("invalid template in before", func(t *testing.T) {
		tmpl, err := ctx.LoadTemplate("invalid_template.md", nil)
		assert.NoError(err)

		_, err = renderer.RenderTemplate(tmpl, &testContent{})
		assert.Error(err)
		assert.Contains(err.Error(), "error processing before files")
	})
}

func TestBeforeAfterWithRelativePaths(t *testing.T) {
	repoFS := createTestFS(map[string]string{
		"pages/index.md": `---toml
before = "./includes/header.md;../shared/nav.md"
after = "./includes/footer.md"
---
INDEX`,

		"pages/includes/header.md": "HEADER",
		"pages/includes/footer.md": "FOOTER",
		"shared/nav.md":            "NAV",
	})

	ctx := NewResolver(repoFS)
	renderer := NewRenderer(ctx, nil)
	assert := assert.New(t)

	tmpl, err := ctx.LoadTemplate("pages/index.md", nil)
	assert.NoError(err)

	output, err := renderer.RenderTemplate(tmpl, &testContent{})
	assert.NoError(err)

	// Should be: HEADER\nNAV\nFOOTER\nINDEX (user content at tail)
	lines := strings.Split(output, "\n")
	assert.Equal(4, len(lines))
	assert.Equal("HEADER", lines[0])
	assert.Equal("NAV", lines[1])
	assert.Equal("FOOTER", lines[2])
	assert.Equal("INDEX", lines[3])
}
