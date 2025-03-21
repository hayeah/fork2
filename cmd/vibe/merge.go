package main

import (
	"fmt"
)

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
	// Stub implementation for the merge command
	if r.Args.Paste {
		fmt.Println("Merge command with --paste flag is not yet implemented")
	} else {
		fmt.Println("Merge command is not yet implemented")
	}
	return nil
}