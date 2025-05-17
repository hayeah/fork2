package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alexflint/go-arg"
)

// Args defines the command-line arguments with subcommands
type Args struct {
	Ask   *AskCmd   `arg:"subcommand:ask" help:"Select files and generate output"`
	Merge *MergeCmd `arg:"subcommand:merge" help:"Merge changes"`
	Ls    *LsCmd    `arg:"subcommand:ls" help:"List files matching patterns"`
	New   *NewCmd   `arg:"subcommand:new" help:"Create a new prompt/template"`
}

// item represents each file or directory in the listing.
type item struct {
	Path       string
	IsDir      bool
	Children   []string // immediate children (for toggling entire sub-tree)
	TokenCount int      // Number of tokens in this file
}

// TokenEstimator is a function type that estimates token count for a file
type TokenEstimator func(filePath string) (int, error)

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
	case r.Args.Ask != nil:
		pickRunner, err := NewAskRunner(*r.Args.Ask, r.RootPath)
		if err != nil {
			return err
		}
		return pickRunner.Run()
	case r.Args.Merge != nil:
		mergeRunner, err := NewMergeRunner(*r.Args.Merge, r.RootPath)
		if err != nil {
			return err
		}
		return mergeRunner.Run()
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
	default:
		return fmt.Errorf("no subcommand specified, use 'ask', 'merge', 'ls', or 'new'")
	}
}

// main is our entrypoint: parse args and run the application
func main() {
	var args Args
	parser := arg.MustParse(&args)

	// If no subcommand is specified, show help
	if args.Ask == nil && args.Merge == nil && args.Ls == nil && args.New == nil {
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	runner := NewRunner(args)
	if err := runner.Run(); err != nil {
		log.Fatal(err)
	}
}

// min returns the smaller of a and b
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
