package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/hayeah/fork2/render"
)

// LsCmd defines the command-line arguments for the ls subcommand
type LsCmd struct {
	Select string `arg:"-s,--select" help:"Select files matching patterns"`
	// optional positional prompt file (template); empty means rely on --select
	Template string `arg:"positional" help:"Path to a prompt/template file"`
}

// LsRunner encapsulates the state and behavior for the ls subcommand
type LsRunner struct {
	Args     LsCmd
	RootPath string
	DirTree  *DirectoryTree
}

// NewLsRunner creates and initializes a new LsRunner
func NewLsRunner(cmd LsCmd, root string) (*LsRunner, error) {
	if cmd.Select == "" && cmd.Template == "" {
		return nil, fmt.Errorf("either --select or <template> must be provided")
	}
	return &LsRunner{
		Args:     cmd,
		RootPath: root,
		DirTree:  NewDirectoryTree(root),
	}, nil
}

// Run executes the ls subcommand
func (r *LsRunner) Run() error {
	pattern := r.Args.Select
	if pattern == "" { // derive from template front-matter
		resolver := render.NewResolver(os.DirFS(r.RootPath))
		templ, err := render.NewRenderer(resolver, nil).LoadTemplate(r.Args.Template)
		if err != nil {
			return err
		}
		pattern = templ.Meta.Select
	}

	selections, err := r.DirTree.SelectFiles(pattern)
	if err != nil {
		return err
	}

	paths := make([]string, 0, len(selections))
	for _, sel := range selections {
		paths = append(paths, sel.Path)
	}
	sort.Strings(paths)

	for _, p := range paths {
		fmt.Println(p)
	}
	return nil
}
