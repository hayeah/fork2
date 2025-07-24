package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestTemplatePath returns the absolute path to a template in testdata
func getTestTemplatePath(t *testing.T, templateName string) string {
	t.Helper()
	// Get the absolute path to the test directory
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Find the cmd/vibe directory
	dir := cwd
	for {
		if filepath.Base(dir) == "vibe" && filepath.Base(filepath.Dir(dir)) == "cmd" {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find cmd/vibe directory")
		}
		dir = parent
	}

	return filepath.Join(dir, "testdata", "templates", templateName)
}

// Template file names in testdata
const (
	tmplListSelected = "list_selected.md"
	tmplContent      = "content_test.md"
	tmplData         = "data_test.md"
	tmplContext      = "test_context.md"
)

// runRunner executes the runner and captures output (stdout or file)
func runRunner(t *testing.T, cmd OutCmd, cwd string) string {
	t.Helper()

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(cwd))
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	runner, err := NewAskRunner(cmd)
	require.NoError(t, err)

	if cmd.Output == "-" {
		// Capture stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w
		t.Cleanup(func() { os.Stdout = oldStdout })

		require.NoError(t, runner.Run())

		w.Close()
		var buf bytes.Buffer
		io.Copy(&buf, r)
		return buf.String()
	}

	// Output to file
	require.NoError(t, runner.Run())
	out, err := os.ReadFile(cmd.Output)
	require.NoError(t, err)
	return string(out)
}

// assertHasFiles checks that the output contains all specified files
func assertHasFiles(t *testing.T, out string, files ...string) {
	t.Helper()
	for _, f := range files {
		assert.Contains(t, out, f, "expected %s in output", f)
	}
}

// assertNoFiles checks that the output does not contain any of the specified files
func assertNoFiles(t *testing.T, out string, files ...string) {
	t.Helper()
	for _, f := range files {
		assert.NotContains(t, out, f, "should not contain %s", f)
	}
}

// createTempOutput creates a temporary output file
func createTempOutput(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "output*.txt")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}
