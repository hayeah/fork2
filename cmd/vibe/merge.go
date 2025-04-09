package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/hayeah/fork2/heredoc"
	"github.com/hayeah/fork2/merge"
)

// MergeCmd contains the arguments for the 'merge' subcommand
type MergeCmd struct {
	Paste        bool `arg:"--paste" help:"Read input from clipboard"`
	Dry          bool `arg:"--dry" help:"Dry run (verification only)"`
	SkipCommit   bool `arg:"--skip-commit" help:"Skip git commit actions"`
}

// MergeRunner encapsulates the state and behavior for the merge command
type MergeRunner struct {
	Args     MergeCmd
	RootPath string
}

// NewMergeRunner creates and initializes a new MergeRunner
func NewMergeRunner(cmdArgs MergeCmd, rootPath string) (*MergeRunner, error) {
	return &MergeRunner{
		Args:     cmdArgs,
		RootPath: rootPath,
	}, nil
}

// Run executes the merge process
func (r *MergeRunner) Run() error {
	// Read heredoc content
	content, err := r.readHeredocContent()
	if err != nil {
		return err
	}

	// Parse commands
	commands, err := r.parseCommands(content)
	if err != nil {
		return err
	}

	// Verify commands and convert to actions
	actions, err := r.verify(commands)
	if err != nil {
		return err
	}

	// Apply actions
	if r.Args.Dry {
		return nil
	}

	return r.apply(actions)
}

// readHeredocContent reads the heredoc content from clipboard or stdin
func (r *MergeRunner) readHeredocContent() (string, error) {
	var content string
	var err error

	// Get content from clipboard if --paste flag is set
	if r.Args.Paste {
		content, err = clipboard.ReadAll()
		if err != nil {
			return "", fmt.Errorf("failed to read from clipboard: %w", err)
		}
		if content == "" {
			return "", fmt.Errorf("clipboard is empty")
		}
	} else {
		// Read from stdin if no --paste flag
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read from stdin: %w", err)
		}
		content = string(bytes)
		if content == "" {
			return "", fmt.Errorf("no input provided")
		}
	}

	return content, nil
}

// parseCommands parses the heredoc content into commands
func (r *MergeRunner) parseCommands(content string) (heredoc.Commands, error) {
	// Create a heredoc parser
	parser := heredoc.NewParser(strings.NewReader(content))

	// Parse all commands
	commands, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse heredoc: %w", err)
	}

	if len(commands) == 0 {
		return nil, fmt.Errorf("no commands found in input")
	}

	return commands, nil
}

// verify verifies all commands and converts them to actions
func (r *MergeRunner) verify(commands heredoc.Commands) ([]merge.Action, error) {
	var actions []merge.Action
	var verificationErrors []string
	var unknownCommands []string

	for _, cmd := range commands {
		action, err := merge.CommandToAction(&cmd)
		if err != nil {
			// Log error for unknown command but continue
			unknownCommands = append(unknownCommands, fmt.Sprintf("line %d: %s - %s", cmd.LineNo, cmd.Name, err.Error()))
			continue
		}

		// Skip git commit actions if --skip-commit flag is set
		if r.Args.SkipCommit {
			if execAction, ok := action.(*merge.Exec); ok {
				if strings.HasPrefix(execAction.Payload, "git commit") {
					fmt.Printf("[SKIP] %d %s %s (due to --skip-commit)\n", cmd.LineNo, cmd.Name, action.Description())
					continue
				}
			}
		}

		// Verify the action
		if err := action.Verify(); err != nil {
			verificationErrors = append(verificationErrors, fmt.Sprintf("line %d: %s - %s", cmd.LineNo, cmd.Name, err.Error()))
			continue
		}

		actions = append(actions, action)
		fmt.Printf("[OK] %d %s %s\n", cmd.LineNo, cmd.Name, action.Description())
	}

	// Print unknown commands
	if len(unknownCommands) > 0 {
		fmt.Println("Unknown commands (ignored):")
		for _, msg := range unknownCommands {
			fmt.Printf("  - %s\n", msg)
		}
	}

	// If there are verification errors, don't apply any changes
	if len(verificationErrors) > 0 {
		fmt.Println("Verification errors:")
		for _, msg := range verificationErrors {
			fmt.Printf("  - %s\n", msg)
		}
		return nil, fmt.Errorf("verification failed for %d command(s)", len(verificationErrors))
	}

	return actions, nil
}

// apply applies all actions
func (r *MergeRunner) apply(actions []merge.Action) error {
	fmt.Printf("Applying %d command(s)...\n", len(actions))
	for i, action := range actions {
		if err := action.Apply(); err != nil {
			return fmt.Errorf("failed to apply command %d: %w", i+1, err)
		}
	}

	fmt.Println("All commands applied successfully!")
	return nil
}
