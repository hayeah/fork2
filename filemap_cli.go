package fork2

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alexflint/go-arg"
)

// FileMapArgs defines the command-line arguments for the filemap CLI
type FileMapArgs struct {
	InputDir   string `arg:"positional" help:"Input directory (default: current directory)"`
	OutputFile string `arg:"-o,--output" help:"Output file path (default: stdout)"`
	Absolute   bool   `arg:"-a,--absolute" help:"Output absolute paths instead of relative paths"`
}

// FileMapCLI represents the filemap CLI application
type FileMapCLI struct {
	Args *FileMapArgs
}

// InitFileMapCLI initializes the filemap CLI application
func InitFileMapCLI() (*FileMapCLI, error) {
	// Parse command-line arguments
	args := &FileMapArgs{}
	arg.MustParse(args)

	return &FileMapCLI{
		Args: args,
	}, nil
}

// Run executes the filemap CLI application
func (cli *FileMapCLI) Run() error {

	// Use InputDir if provided, otherwise use current working directory
	inputDir := cli.Args.InputDir
	if inputDir == "" {
		// Get current working directory
		workingDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		inputDir = workingDir
	}

	// List files respecting .gitignore
	files, err := listFilesRespectingGitIgnore(inputDir)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	var inputPaths = files

	if cli.Args.Absolute {
		// Convert relative paths to absolute paths
		absPaths := make([]string, 0, len(files))
		for _, file := range files {
			absPath := filepath.Join(inputDir, file)
			absPaths = append(absPaths, absPath)
		}
		inputPaths = absPaths
	}

	// Determine output writer
	var w io.Writer
	if cli.Args.OutputFile == "" {
		w = os.Stdout
	} else {
		f, err := os.Create(cli.Args.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	var baseDir = cli.Args.InputDir
	if baseDir == "" {
		// set to cwd
		baseDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Write filemap
	err = WriteFileMap(w, inputPaths, baseDir)
	if err != nil {
		return fmt.Errorf("failed to write filemap: %w", err)
	}

	if cli.Args.OutputFile != "" {
		fmt.Printf("Filemap written to %s\n", cli.Args.OutputFile)
	}

	return nil
}
