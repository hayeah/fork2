package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/hayeah/fork2"
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

// WriteOutput processes the selected files and outputs the result using the renderer
func (ctx *VibeContext) WriteOutput(w io.Writer, userPath string, systemPath string, selectedFiles []string) error {
	ctx.RenderContext.CurrentTemplatePath = "./"
	defer func() {
		ctx.RenderContext.CurrentTemplatePath = "./"
	}()

	// Sort the selected files
	sort.Strings(selectedFiles)

	// Prepare template data
	data := map[string]interface{}{
		"RepoDirectoryTree": ctx.RepoDirectoryTree(),
		"RepoPrompts":       ctx.RepoPrompts(),
		"SelectedFiles":     selectedFiles,
	}

	// Write the file map of selected files to a string
	var fileMapBuf strings.Builder
	err := fork2.WriteFileMap(&fileMapBuf, selectedFiles, ctx.ask.RootPath)
	if err != nil {
		return fmt.Errorf("failed to write file map: %v", err)
	}
	data["FileMap"] = fileMapBuf.String()

	// Render the output using the template system
	output, err := ctx.Renderer.Render(userPath, systemPath, data)
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
