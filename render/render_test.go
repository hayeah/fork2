package render

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/hayeah/fork2/internal/assert"
)

//--------------------------------- Helper utilities ---------------------------------

// createTestFS builds an in‑memory filesystem for convenient test setup.
func createTestFS(files map[string]string) fs.FS {
	m := fstest.MapFS{}
	for path, content := range files {
		m[path] = &fstest.MapFile{Data: []byte(content)}
	}
	return m
}

// testContent is a tiny implementation of the Content interface used by the
// renderer. Keeping it here avoids duplication across individual test cases.
type testContent struct{ content string }

func (c *testContent) Content() string     { return c.content }
func (c *testContent) SetContent(s string) { c.content = s }

// templatePtr is a small helper that returns a *Template populated with the
// supplied path & FS only when the path is non‑empty. It keeps the call sites
// for ResolvePartialPath compact and readable.
func templatePtr(p string, fsys fs.FS) *Template {
	if p == "" {
		return nil
	}
	return &Template{Path: p, FS: fsys}
}

//--------------------------------- Core tests ---------------------------------

func TestResolvePartialPath(t *testing.T) {
	// Build a pair of filesystems: one "repo" and one "system" to emulate the
	// search stack that Resolver expects (first = repo, last = system).
	systemFS := createTestFS(map[string]string{
		"vibe/coder": "system coder template",
	})
	repoFS := createTestFS(map[string]string{
		"common/header":                 "repo header template",
		"templates/local/helper":        "local helper template",
		"templates/subdir/component.md": "component template",
		"components/shared/footer.md":   "footer template",
	})

	ctx := NewResolver(repoFS, systemFS)
	assert := assert.New(t)

	cases := []struct {
		name          string
		currentPath   string // path of the template that is currently rendering
		partialPath   string // path we are resolving
		wantFS        fs.FS  // repoFS / systemFS / nil
		wantFile      string // resolved file path
		wantErrSubstr string // substring that must appear in error (if non‑empty)
	}{
		{name: "system template", partialPath: "<vibe/coder>", wantFS: systemFS, wantFile: "vibe/coder"},
		{name: "repo root template", currentPath: "templates/main.md", partialPath: "@common/header", wantFS: repoFS, wantFile: "common/header"},
		{name: "local template", currentPath: "templates/main.md", partialPath: "./local/helper", wantFS: repoFS, wantFile: "templates/local/helper"},
		{name: "relative up one", currentPath: "templates/subdir/page.md", partialPath: "../local/helper", wantFS: repoFS, wantFile: "templates/local/helper"},
		{name: "relative across tree", currentPath: "templates/subdir/page.md", partialPath: "../../components/shared/footer.md", wantFS: repoFS, wantFile: "components/shared/footer.md"},
		{name: "missing cur template for relative", currentPath: "", partialPath: "./local/helper", wantErrSubstr: "cannot resolve relative path without currentTemplatePath"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFS, gotFile, err := ctx.ResolvePartialPath(tc.partialPath, templatePtr(tc.currentPath, repoFS))

			if tc.wantErrSubstr != "" {
				assert.Error(err)
				assert.Contains(err.Error(), tc.wantErrSubstr)
				return
			}
			assert.NoError(err)
			assert.Equal(tc.wantFile, gotFile)
			assert.Equal(tc.wantFS, gotFS)
		})
	}
}

func TestRelativePathResolution(t *testing.T) {
	repoFS := createTestFS(map[string]string{
		"components/button.md":          "button component",
		"components/form/input.md":      "form input component",
		"templates/page.md":             "page template",
		"templates/blog/post.md":        "blog post template",
		"templates/blog/list.md":        "blog list template",
		"templates/admin/dashboard.md":  "admin dashboard",
		"templates/admin/users/list.md": "admin users list",
		"templates/shared/header.md":    "shared header",
		"templates/shared/footer.md":    "shared footer",
	})

	ctx := NewResolver(repoFS)
	assert := assert.New(t)

	cases := []struct {
		name          string
		currentPath   string
		partialPath   string
		wantFile      string
		wantErrSubstr string
	}{
		{name: "simple relative", currentPath: "templates/page.md", partialPath: "./shared/header.md", wantFile: "templates/shared/header.md"},
		{name: "nested relative", currentPath: "templates/blog/post.md", partialPath: "../shared/footer.md", wantFile: "templates/shared/footer.md"},
		{name: "parent and branch", currentPath: "templates/admin/users/list.md", partialPath: "../../blog/list.md", wantFile: "templates/blog/list.md"},
		{name: "root level", currentPath: "templates/admin/dashboard.md", partialPath: "../../components/button.md", wantFile: "components/button.md"},
		{name: "complex navigation", currentPath: "templates/blog/list.md", partialPath: "../admin/users/../dashboard.md", wantFile: "templates/admin/dashboard.md"},
		{name: "missing cur template", currentPath: "", partialPath: "./shared/header.md", wantErrSubstr: "cannot resolve relative path without currentTemplatePath"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFS, gotFile, err := ctx.ResolvePartialPath(tc.partialPath, templatePtr(tc.currentPath, repoFS))

			if tc.wantErrSubstr != "" {
				assert.Error(err)
				assert.Contains(err.Error(), tc.wantErrSubstr)
				return
			}
			assert.NoError(err)
			assert.Equal(repoFS, gotFS)
			assert.Equal(tc.wantFile, gotFile)
		})
	}
}

func TestPartialRendering(t *testing.T) {
	systemFS := createTestFS(map[string]string{
		"vibe/coder":  "System {{ partial \"<vibe/footer>\" }}",
		"vibe/footer": "Footer",
	})

	repoFS := createTestFS(map[string]string{
		"common/header":          "Repo Header with {{ .Value }}",
		"templates/main.md":      "ignored",
		"templates/local/helper": "Local Helper that uses {{ partial \"@common/header\" }}",
	})

	ctx := NewResolver(repoFS, systemFS)
	renderer := NewRenderer(ctx)
	assert := assert.New(t)

	data := struct {
		*testContent
		Value string
	}{
		testContent: &testContent{},
		Value:       "test value",
	}

	t.Run("simple partial", func(t *testing.T) {
		got, err := renderer.RenderPartial("@common/header", &data)
		assert.NoError(err)
		assert.Equal("Repo Header with test value", got)
	})

	t.Run("nested partial", func(t *testing.T) {
		got, err := renderer.RenderPartial("<vibe/coder>", &data)
		assert.NoError(err)
		assert.Equal("System Footer", got)
	})

	t.Run("local relative partial", func(t *testing.T) {
		// Seed renderer with a current template so that a relative reference
		// can be resolved correctly.
		renderer.cur = &Template{Path: "templates/main.md", FS: repoFS}

		got, err := renderer.RenderPartial("./local/helper", &data)
		assert.NoError(err)
		assert.Equal("Local Helper that uses Repo Header with test value", got)
	})
}

func TestRendererWithLayout(t *testing.T) {
	assert := assert.New(t)

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

	ctx := NewResolver(repoFS, systemFS)
	renderer := NewRenderer(ctx)

	data := &struct {
		*testContent
		System        string
		ListDirectory []string
		SelectedFiles []string
		ToolList      string
	}{
		testContent:   &testContent{},
		System:        "Linux",
		ListDirectory: []string{"file1.go", "file2.md"},
		SelectedFiles: []string{"selected1.go"},
		ToolList:      "Tool1, Tool2, Tool3",
	}

	// Load the content template, bolt on a layout, and render.
	tmpl, err := ctx.LoadTemplate("templates/user.md", nil)
	assert.NoError(err)
	tmpl.Meta.Layout = "layouts/main.md" // bare path => repo root FS

	out, err := renderer.RenderTemplate(tmpl, data)
	assert.NoError(err)

	assert.Contains(out, "Coder: Linux")
	assert.Contains(out, "Tool1, Tool2, Tool3")
	assert.Contains(out, "[file1.go file2.md]")
}

func TestLayoutCycleDetection(t *testing.T) {
	repoFS := createTestFS(map[string]string{
		"a.md": "---toml\nlayout = \"b.md\"\n---\nA", // a -> b
		"b.md": "---toml\nlayout = \"a.md\"\n---\nB", // b -> a (cycle)
	})

	ctx := NewResolver(repoFS)
	renderer := NewRenderer(ctx)
	assert := assert.New(t)

	_, err := renderer.Render("a.md", &testContent{})
	assert.Error(err)
	assert.Contains(err.Error(), "layout cycle detected")
}

func TestLayoutDeepNestingLimit(t *testing.T) {
	// Build 12 templates nested one inside another (index 0 has layout 1, etc.)
	files := map[string]string{}
	for i := 0; i < 12; i++ {
		body := "T" + string(rune('0'+i))
		if i < 11 { // last template has no layout
			body = "---toml\nlayout = \"t" + string(rune('0'+i+1)) + ".md\"\n---\n" + body
		}
		files["t"+string(rune('0'+i))+".md"] = body
	}

	repoFS := createTestFS(files)
	ctx := NewResolver(repoFS)
	renderer := NewRenderer(ctx)
	assert := assert.New(t)

	_, err := renderer.Render("t0.md", &testContent{})
	assert.Error(err)
	assert.Contains(err.Error(), "layout nesting too deep")
}

func TestLoadTemplateParsesFrontMatter(t *testing.T) {
	repoFS := createTestFS(map[string]string{
		"foo.md":  "---toml\nlayout=\"base.md\"\n---\nHello",
		"base.md": "Base {{ .Content }}",
	})
	ctx := NewResolver(repoFS)
	assert := assert.New(t)

	tmpl, err := ctx.LoadTemplate("foo.md", nil)
	assert.NoError(err)
	assert.Equal("base.md", tmpl.Meta.Layout)
}

// TestTemplatePrecedenceOrder verifies that when the same template exists in multiple
// filesystem layers, it's resolved from the highest-priority layer according to the
// precedence order: repo → VIBE_PROMPTS → ~/.vibe → built-in templates
func TestTemplatePrecedenceOrder(t *testing.T) {
	// Create fake FS layers mimicking the different sources
	repoFS := createTestFS(map[string]string{
		"common/header.md":  "REPO HEADER",
		"unique/repo.md":    "REPO UNIQUE",
	})

	vibePromptsFS := createTestFS(map[string]string{
		"common/header.md":       "VIBE_PROMPTS HEADER",
		"common/footer.md":       "VIBE_PROMPTS FOOTER",
		"unique/vibe_prompts.md": "VIBE_PROMPTS UNIQUE",
	})

	userVibeFS := createTestFS(map[string]string{
		"common/header.md":    "USER_VIBE HEADER",
		"common/footer.md":    "USER_VIBE FOOTER",
		"common/sidebar.md":   "USER_VIBE SIDEBAR",
		"unique/user_vibe.md": "USER_VIBE UNIQUE",
	})

	systemFS := createTestFS(map[string]string{
		"common/header.md":  "SYSTEM HEADER",
		"common/footer.md":  "SYSTEM FOOTER",
		"common/sidebar.md": "SYSTEM SIDEBAR",
		"common/nav.md":     "SYSTEM NAV",
		"unique/system.md":  "SYSTEM UNIQUE",
	})

	// Create resolver with all layers in the correct order
	ctx := NewResolver(repoFS, vibePromptsFS, userVibeFS, systemFS)
	renderer := NewRenderer(ctx)
	assert := assert.New(t)

	// Test cases for templates that exist in multiple layers
	testCases := []struct {
		name     string
		path     string
		expected string
	}{
		{"template in all layers", "common/header.md", "REPO HEADER"},
		{"template in vibe_prompts, user_vibe, system", "common/footer.md", "VIBE_PROMPTS FOOTER"},
		{"template in user_vibe and system", "common/sidebar.md", "USER_VIBE SIDEBAR"},
		{"template only in system", "common/nav.md", "SYSTEM NAV"},
		{"unique repo template", "unique/repo.md", "REPO UNIQUE"},
		{"unique vibe_prompts template", "unique/vibe_prompts.md", "VIBE_PROMPTS UNIQUE"},
		{"unique user_vibe template", "unique/user_vibe.md", "USER_VIBE UNIQUE"},
		{"unique system template", "unique/system.md", "SYSTEM UNIQUE"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load the template and verify it comes from the expected layer
			tmpl, err := ctx.LoadTemplate(tc.path, nil)
			assert.NoError(err)

			// Render the template and check the content
			output, err := renderer.RenderTemplate(tmpl, &testContent{})
			assert.NoError(err)
			assert.Equal(tc.expected, output)
		})
	}
}
