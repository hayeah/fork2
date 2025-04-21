package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/atotto/clipboard"
	"github.com/hayeah/fork2/render"
)

// //go:embed diff-heredoc.md
// var diffTemplateContent string

//go:embed templates
var systemTemplatesFS embed.FS

// VibeContext encapsulates the state and functionality for the vibe command
type VibeContext struct {
	ask *AskRunner

	RenderContext *render.Resolver
	Renderer      *render.Renderer

	Select string

	DirTree *DirectoryTree
}

// NewVibeContext creates a new VibeContext instance
func NewVibeContext(ask *AskRunner) (*VibeContext, error) {
	ctx := &VibeContext{
		ask:     ask,
		DirTree: ask.DirTree,
	}

	systemfs, err := fs.Sub(systemTemplatesFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to create system prompts fs: %v", err)
	}
	ctx.RenderContext = render.NewResolver(os.DirFS(ask.RootPath), systemfs)

	ctx.Renderer = render.NewRenderer(ctx.RenderContext)

	return ctx, nil
}

// RepoDirectoryTree generates the directory tree structure as a string.
func (ctx *VibeContext) RepoDirectoryTree() (string, error) {
	var buf strings.Builder
	err := ctx.DirTree.GenerateDirectoryTree(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RepoPrompts loads .vibe.md files from the current directory up to the repo root.
func (ctx *VibeContext) RepoPrompts() (string, error) {
	return loadVibeFiles(ctx.ask.RootPath)
}

func (ctx *VibeContext) FileMap() (string, error) {
	if ctx.Select == "" {
		return "", nil
	}

	selected, err := ctx.DirTree.SelectFiles(ctx.Select)
	if err != nil {
		return "", fmt.Errorf("failed to select files: %v", err)
	}

	// Write the file map of selected files to a string
	var fileMapBuf strings.Builder
	err = WriteFileMap(&fileMapBuf, selected, ctx.ask.RootPath)
	if err != nil {
		return "", fmt.Errorf("failed to write file map: %v", err)
	}

	return fileMapBuf.String(), nil
}

// WriteFileSelections processes the selected files and outputs the result using the renderer
func (ctx *VibeContext) WriteFileSelections(w io.Writer, contentPath string, layoutPath string) error {
	// We can no longer support inline content directly through LoadTemplate
	// If this is an inline template, we need to handle it differently
	var tmpl *render.Template
	var err error

	// Load the content template from path
	tmpl, _, err = render.LoadTemplate(ctx.RenderContext, contentPath, "./", ctx.RenderContext.Partials[0])
	if err != nil {
		return fmt.Errorf("error loading content template: %w", err)
	}

	// Override the template layout if CLI provided one
	if layoutPath != "" {
		tmpl.Meta.Layout = layoutPath
	}

	selectPattern := tmpl.Meta.Select
	if selectPattern == "" {
		selectPattern = ctx.ask.Args.Select
	}
	ctx.Select = selectPattern

	fmt.Println("select", selectPattern)

	data := newVibeContextMemoized(ctx)
	// Render the output using the template system
	output, err := ctx.Renderer.RenderTemplate(tmpl, data)
	if err != nil {
		return err
	}

	// Write the rendered output to the writer
	_, err = fmt.Fprint(w, output)
	if err != nil {
		return fmt.Errorf("failed to write output: %v", err)
	}

	return nil
}

// newVibeContextMemoized creates a memoized version of the VibeContext.
//
// text/template does not auto invoke .Field if it's a function value.
// But it DOES auto invoke .Method. That's why we need to create all these wrapper
// methods.
//
// See: https://github.com/golang/go/issues/3999
func newVibeContextMemoized(ctx *VibeContext) *VibeContextMemoized {
	return &VibeContextMemoized{
		FileMapOnce:           sync.OnceValues(ctx.FileMap),
		RepoDirectoryTreeOnce: sync.OnceValues(ctx.RepoDirectoryTree),
		RepoPromptsOnce:       sync.OnceValues(ctx.RepoPrompts),
	}
}

type VibeContextMemoized struct {
	content string

	FileMapOnce           func() (string, error)
	RepoDirectoryTreeOnce func() (string, error)
	RepoPromptsOnce       func() (string, error)
}

// Content returns the current clipboard content as a string.
func (ctx *VibeContextMemoized) Content() string {
	return ctx.content
}

// SetContent sets the current content. (void stub for now)
func (ctx *VibeContextMemoized) SetContent(content string) {
	ctx.content = content
}

// ClipboardText returns the current clipboard content as a string.
func (ctx *VibeContextMemoized) ClipboardText() (string, error) {
	return clipboard.ReadAll()
}

func (ctx *VibeContextMemoized) FileMap() (string, error) {
	return ctx.FileMapOnce()
}

func (ctx *VibeContextMemoized) RepoDirectoryTree() (string, error) {
	return ctx.RepoDirectoryTreeOnce()
}

func (ctx *VibeContextMemoized) RepoPrompts() (string, error) {
	return ctx.RepoPromptsOnce()
}
