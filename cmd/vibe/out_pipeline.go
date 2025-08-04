package main

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/hayeah/fork2/internal/metrics"
	"github.com/hayeah/fork2/internal/selection"
	"github.com/hayeah/fork2/render"
	"github.com/hayeah/fork2/vibe"
)

// OutPipeline groups all services needed by the out command.
type OutPipeline struct {
	DirTree  *DirectoryTree
	Renderer *render.Renderer
	FileMap  *FileMapWriter
	Metrics  *metrics.OutputMetrics
	Loader   ContentLoader
	Config   *vibe.AppConfig

	Template *render.Template
}

// outData implements render.Content and exposes helpers for templates.
type outData struct {
	pipeline         *OutPipeline
	selectPattern    string
	dirTreePattern   string
	rootPath         string
	WorkingDirectory string
	ContentStr       string
	Data             map[string]string

	fileMapOnce sync.Once
	fileMap     string
	fileMapErr  error

	treeOnce sync.Once
	tree     string
	treeErr  error

	promptsOnce sync.Once
	prompts     string
	promptsErr  error

	selectedOnce  sync.Once
	selectedPaths []string

	selectionsOnce sync.Once
	selections     []selection.FileSelection
	selectionsErr  error
}

func (d *outData) Content() string     { return d.ContentStr }
func (d *outData) SetContent(s string) { d.ContentStr = s }

func (d *outData) FileMap() (string, error) {
	d.fileMapOnce.Do(func() {
		if d.selectPattern == "" {
			d.fileMap = ""
			return
		}
		sels, err := d.getSelections()
		if err != nil {
			d.fileMapErr = err
			return
		}
		var buf strings.Builder
		d.fileMapErr = d.pipeline.FileMap.Output(&buf, sels)
		d.fileMap = buf.String()
	})
	return d.fileMap, d.fileMapErr
}

func (d *outData) RepoDirectoryTree() (string, error) {
	d.treeOnce.Do(func() {
		var buf strings.Builder
		d.treeErr = d.pipeline.DirTree.GenerateDirectoryTree(&buf, d.dirTreePattern)
		d.tree = buf.String()
	})
	return d.tree, d.treeErr
}

func (d *outData) RepoPrompts() (string, error) {
	d.promptsOnce.Do(func() {
		d.prompts, d.promptsErr = loadVibeFiles(d.rootPath)
	})
	return d.prompts, d.promptsErr
}

// getSelections returns the FileSelection slice matched by the
// template's `select` pattern, memoised for reuse by other helpers.
func (d *outData) getSelections() ([]selection.FileSelection, error) {
	d.selectionsOnce.Do(func() {
		if d.selectPattern == "" {
			return
		}
		d.selections, d.selectionsErr = d.pipeline.DirTree.SelectFiles(d.selectPattern)
	})
	return d.selections, d.selectionsErr
}

func (d *outData) SelectedPaths() []string {
	d.selectedOnce.Do(func() {
		sels, err := d.getSelections()
		if err != nil {
			return // ignore error; template helpers should stay silent
		}
		for _, s := range sels {
			d.selectedPaths = append(d.selectedPaths, s.Path)
		}
	})
	return d.selectedPaths
}

// Output returns a WriterCloser for the configured output destination.
func (p *OutPipeline) Output() (io.WriteCloser, error) {
	return NewOutputWriter(p.Config.Output)
}

// Run executes the rendering pipeline.
func (p *OutPipeline) Run() error {
	// Create output writer
	out, err := p.Output()
	if err != nil {
		return err
	}
	defer out.Close()

	dataMap, err := parseDataParams(p.Config.DataPairs)
	if err != nil {
		return err
	}

	content, err := p.Loader.Content()
	if err != nil {
		return fmt.Errorf("failed to load content: %w", err)
	}

	tmpl := p.Template
	if tmpl == nil {
		return fmt.Errorf("template not set")
	}

	// Set the AppConfig on the template so Before/After templates are processed
	tmpl.Config = p.Config

	selectPattern := p.Config.Select
	dirTreePattern := p.Config.Dirtree

	root := p.DirTree.RootPath

	data := &outData{
		pipeline:         p,
		selectPattern:    selectPattern,
		dirTreePattern:   dirTreePattern,
		rootPath:         root,
		WorkingDirectory: p.Config.WorkingDirectory,
		ContentStr:       content,
		Data:             dataMap,
	}

	rendered, err := p.Renderer.RenderTemplate(tmpl, data)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprint(out, rendered); err != nil {
		return fmt.Errorf("failed to write output: %v", err)
	}

	p.Metrics.Wait()
	if err := PrintTokenBreakdown(p.Metrics); err != nil {
		return err
	}
	return nil
}
