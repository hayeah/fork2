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

	DirTree *DirectoryTree

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
		DirTree: ask.DirTree,
	}

	systemfs, err := fs.Sub(systemTemplatesFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to create system prompts fs: %v", err)
	}

	ctx.RenderContext = &render.RenderContext{
		SystemPartials: systemfs,
		RepoPartials:   os.DirFS(ask.RootPath),
	}

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
func (ctx *VibeContext) WriteFileSelections(w io.Writer, args render.RenderArgs) error {
	ctx.RenderContext.CurrentTemplatePath = "./"
	defer func() {
		ctx.RenderContext.CurrentTemplatePath = "./"
	}()

	data := make(map[string]interface{})

	// Prepare template data
	if args.Data == nil {
		args.Data = make(map[string]interface{})
	}

	data["RepoDirectoryTree"] = ctx.RepoDirectoryTree()
	data["RepoPrompts"] = ctx.RepoPrompts()

	// Lazily get selected files from DirTree
	selectString := ""
	if ctx.ask != nil {
		selectString = ctx.ask.Args.Select
		if selectString == "" && ctx.ask.Instruct != nil && ctx.ask.Instruct.Header != nil {
			selectString = ctx.ask.Instruct.Header.Select
		}
	}
	selected, err := ctx.DirTree.SelectFiles(selectString)
	if err != nil {
		return fmt.Errorf("failed to select files: %v", err)
	}

	// Write the file map of selected files to a string
	var fileMapBuf strings.Builder
	err = WriteFileMap(&fileMapBuf, selected, ctx.ask.RootPath)
	if err != nil {
		return fmt.Errorf("failed to write file map: %v", err)
	}
	data["FileMap"] = fileMapBuf.String()

	args.Data = data

	// Render the output using the template system
	output, err := ctx.Renderer.Render(args)
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
