package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hayeah/fork2/internal/assert"
)

func TestLsRunner(t *testing.T) {
	assert := assert.New(t)
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a simple file structure for testing
	files := []string{
		"file1.go",
		"file2.go",
		"dir1/file3.go",
		"dir1/file4.txt",
		"dir2/file5.go",
	}

	for _, file := range files {
		path := filepath.Join(tempDir, file)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Create a template file with front-matter
	templateContent := "```\nselect = \".go\"\n```\n\nTemplate content\n"
	templatePath := filepath.Join(tempDir, "template.md")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	t.Run("ls with select flag", func(t *testing.T) {
		// Create a new LsRunner with the select flag
		runner, err := NewLsRunner(LsCmd{
			Select: ".go",
		}, tempDir)
		if err != nil {
			t.Fatalf("Failed to create LsRunner: %v", err)
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run the ls command
		err = runner.Run()
		if err != nil {
			t.Fatalf("Failed to run ls command: %v", err)
		}

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read the captured output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Check the output
		expected := "dir1/file3.go\ndir2/file5.go\nfile1.go\nfile2.go\n"
		assert.Equal(expected, output)
	})

	t.Run("ls with template", func(t *testing.T) {
		// Create a new LsRunner with the template
		runner, err := NewLsRunner(LsCmd{
			Template: "template.md",
		}, tempDir)
		if err != nil {
			t.Fatalf("Failed to create LsRunner: %v", err)
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run the ls command
		err = runner.Run()
		if err != nil {
			t.Fatalf("Failed to run ls command: %v", err)
		}

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read the captured output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Check the output
		expected := "dir1/file3.go\ndir2/file5.go\nfile1.go\nfile2.go\n"
		assert.Equal(expected, output)
	})

	t.Run("ls with no arguments", func(t *testing.T) {
		// Create a new LsRunner with no arguments
		_, err := NewLsRunner(LsCmd{}, tempDir)
		if err == nil {
			t.Fatalf("Expected error for LsRunner with no arguments")
		}
	})
}
