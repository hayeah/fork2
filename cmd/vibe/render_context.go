package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/hayeah/fork2/render"
)

// //go:embed diff-heredoc.md
// var diffTemplateContent string

//go:embed templates
var systemTemplatesFS embed.FS

// VibeContext encapsulates the state and functionality for the vibe command
type VibeContext struct {
	ask *AskRunner

	RenderContext *render.RenderContext
	Renderer      *render.Renderer

	// Memoization caches
	repoRoot        string
	repoRootOnce    sync.Once
	repoFiles       string
	repoFilesOnce   sync.Once
	repoPrompts     string
	repoPromptsOnce sync.Once
}

// NewVibeContext creates a new VibeContext instance
func NewVibeContext(ask *AskRunner) (*VibeContext, error) {
	ctx := &VibeContext{
		ask: ask,
	}

	systemfs, err := fs.Sub(systemTemplatesFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to create system prompts fs: %v", err)
	}

	// // list system templates
	// var buf strings.Builder
	// _ = fs.WalkDir(systemfs, ".", func(path string, d fs.DirEntry, err error) error {
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if d.IsDir() {
	// 		return nil
	// 	}
	// 	fmt.Fprintln(&buf, path)
	// 	return nil
	// })
	// fmt.Println("System templates:")
	// fmt.Println(buf.String())

	ctx.RenderContext = &render.RenderContext{
		SystemPartials: systemfs,
		RepoPartials:   os.DirFS(ask.RootPath),
	}

	// Create renderer
	ctx.Renderer = render.NewRenderer(ctx.RenderContext)

	return ctx, nil
}

// RepoRoot returns the path to the repository root by looking for a .git directory.
// It memoizes the result so subsequent calls are fast.
func (ctx *VibeContext) RepoRoot() (string, error) {
	var err error
	ctx.repoRootOnce.Do(func() {
		ctx.repoRoot, err = findRepoRoot(ctx.ask.RootPath)
	})
	return ctx.repoRoot, err
}

// RepoDirectoryTree generates the directory tree structure as a string.
// It memoizes the result so subsequent calls are fast.
func (ctx *VibeContext) RepoDirectoryTree() string {
	ctx.repoFilesOnce.Do(func() {
		var buf strings.Builder
		ask := ctx.ask
		if ask.DirTree != nil {
			_ = ask.DirTree.GenerateDirectoryTree(&buf)
		}
		ctx.repoFiles = buf.String()
	})
	return ctx.repoFiles
}

// RepoPrompts loads .vibe.md files from the current directory up to the repo root.
// It memoizes the result so subsequent calls are fast.
func (ctx *VibeContext) RepoPrompts() string {
	ctx.repoPromptsOnce.Do(func() {
		content, err := loadVibeFiles(ctx.ask.RootPath)
		if err != nil {
			// If there's an error, store an empty string
			fmt.Fprintf(os.Stderr, "Warning: failed to load .vibe.md files: %v\n", err)
			ctx.repoPrompts = ""
			return
		}
		ctx.repoPrompts = content
	})
	return ctx.repoPrompts
}

// WriteOutput removed in favor of WriteFileSelections

// WriteFileSelections processes the selected files and outputs the result using the renderer
func (ctx *VibeContext) WriteFileSelections(w io.Writer, userContent string, layoutPath string, fileSelections []FileSelection) error {
	ctx.RenderContext.CurrentTemplatePath = "./"
	defer func() {
		ctx.RenderContext.CurrentTemplatePath = "./"
	}()

	// Prepare template data
	data := map[string]interface{}{
		"RepoDirectoryTree": ctx.RepoDirectoryTree(),
		"RepoPrompts":       ctx.RepoPrompts(),
	}

	// Write the file map of selected files to a string
	var fileMapBuf strings.Builder
	err := WriteFileMap(&fileMapBuf, fileSelections, ctx.ask.RootPath)
	if err != nil {
		return fmt.Errorf("failed to write file map: %v", err)
	}
	data["FileMap"] = fileMapBuf.String()

	// Render the output using the template system
	output, err := ctx.Renderer.Render(render.RenderArgs{
		Content:    userContent,
		LayoutPath: layoutPath,
		Data:       data,
	})
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
