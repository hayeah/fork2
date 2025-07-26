package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runRunner executes the runner and captures output (stdout or file)
func runRunner(t *testing.T, cmd OutCmd, root string) string {
	t.Helper()

	// Set the root directory for the command
	cmd.Root = root

	// Add testdata/templates to the template search path
	cmd.TemplatePaths = []string{"testdata/templates"}

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
