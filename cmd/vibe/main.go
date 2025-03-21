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
}

// AskCmd contains the arguments for the 'ask' subcommand
type AskCmd struct {
	TokenEstimator string `arg:"--token-estimator" help:"Token count estimator to use: 'simple' (size/4) or 'tiktoken'" default:"simple"`
	All            bool   `arg:"-a,--all" help:"Select all files and output immediately"`
	Copy           bool   `arg:"-c,--copy" help:"Copy output to clipboard instead of stdout"`
	Diff           bool   `arg:"--diff" help:"Enable diff output format"`
	Select         string `arg:"--select" help:"Select files matching fuzzy pattern and output immediately"`
	SelectRegex    string `arg:"--select-re" help:"Select files matching regex pattern and output immediately"`
	Instruction    string `arg:"positional" help:"User instruction or path to instruction file"`
}

// MergeCmd contains the arguments for the 'merge' subcommand
type MergeCmd struct {
	Paste bool `arg:"--paste" help:"Read input from clipboard"`
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
	default:
		return fmt.Errorf("no subcommand specified, use 'pick' or 'merge'")
	}
}

// main is our entrypoint: parse args and run the application
func main() {
	var args Args
	parser := arg.MustParse(&args)

	// If no subcommand is specified, show help
	if args.Ask == nil && args.Merge == nil {
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
