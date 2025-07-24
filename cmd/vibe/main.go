package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/alexflint/go-arg"
)

// Args defines the command-line arguments with subcommands
type Args struct {
	Out                *OutCmd                `arg:"subcommand:out" help:"Select files and generate output"`
	Ls                 *LsCmd                 `arg:"subcommand:ls" help:"List files matching patterns"`
	New                *NewCmd                `arg:"subcommand:new" help:"Create a new prompt/template"`
	InstallVSCodeTasks *InstallVSCodeTasksCmd `arg:"subcommand:install:vscode:tasks" help:"Install VS Code tasks for vibe"`
}

// item represents each file or directory in the listing.
type item struct {
	Path       string
	IsDir      bool
	Children   []string // immediate children (for toggling entire sub-tree)
	TokenCount int      // Number of tokens in this file
}

// TokenEstimator is a function type that estimates token count for a file
type TokenEstimator func(fsys fs.FS, filePath string) (int, error)

// Runner encapsulates the state and behavior for the CLI
type Runner struct {
	Args     Args
	RootPath string
}

// NewRunner creates and initializes a new Runner
func NewRunner(args Args) *Runner {
	return &Runner{
		Args:     args,
		RootPath: ".", // Always use current working directory
	}
}

// Run dispatches to the appropriate subcommand
func (r *Runner) Run() error {
	switch {
	case r.Args.Out != nil:
		pickRunner, err := NewAskRunner(*r.Args.Out)
		if err != nil {
			return err
		}
		return pickRunner.Run()
	case r.Args.Ls != nil:
		lsRunner, err := NewLsRunner(*r.Args.Ls, r.RootPath)
		if err != nil {
			return err
		}
		return lsRunner.Run()
	case r.Args.New != nil:
		newRunner, err := NewNewRunner(*r.Args.New, r.RootPath)
		if err != nil {
			return err
		}
		return newRunner.Run()
	case r.Args.InstallVSCodeTasks != nil:
		vsctRunner := NewInstallVSCodeTasksRunner(r.RootPath)
		return vsctRunner.Run()
	default:
		return fmt.Errorf("no subcommand specified, use 'out', 'ls', 'new', or 'install:vscode:tasks'")
	}
}

// main is our entrypoint: parse args and run the application
func main() {
	var args Args
	parser := arg.MustParse(&args)

	// If no subcommand is specified, show help
	if args.Out == nil && args.Ls == nil && args.New == nil && args.InstallVSCodeTasks == nil {
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	runner := NewRunner(args)
	if err := runner.Run(); err != nil {
		log.Fatal(err)
	}
}
