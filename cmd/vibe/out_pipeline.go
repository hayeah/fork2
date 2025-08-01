package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/hayeah/fork2/internal/metrics"
	"github.com/hayeah/fork2/internal/selection"
	"github.com/hayeah/fork2/render"
)

// OutPipeline groups all services needed by the out command.
type OutPipeline struct {
	DT       *DirectoryTree
	Renderer *render.Renderer
	FileMap  *FileMapWriter
	Metrics  *metrics.OutputMetrics
	Loader   ContentLoader
	Env      *AppEnv

	Template     *render.Template
	ContentSpecs []string
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
		d.treeErr = d.pipeline.DT.GenerateDirectoryTree(&buf, d.dirTreePattern)
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
		d.selections, d.selectionsErr = d.pipeline.DT.SelectFiles(d.selectPattern)
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

// Run executes the rendering pipeline using args for configuration.
func (p *OutPipeline) Run(out io.Writer) error {
	dataMap, err := parseDataParams(p.Env.DataPairs)
	if err != nil {
		return err
	}

	content := ""
	if len(p.ContentSpecs) > 0 {
		c, err := p.Loader.LoadSources(context.Background(), p.ContentSpecs)
		if err != nil {
			return fmt.Errorf("failed to load content: %w", err)
		}
		content = c
	}

	tmpl := p.Template
	if tmpl == nil {
		return fmt.Errorf("template not set")
	}

	selectPattern := tmpl.FrontMatter.Select
	dirTreePattern := tmpl.FrontMatter.Dirtree

	root := p.DT.RootPath

	data := &outData{
		pipeline:         p,
		selectPattern:    selectPattern,
		dirTreePattern:   dirTreePattern,
		rootPath:         root,
		WorkingDirectory: string(p.Env.WorkingDirectory),
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
