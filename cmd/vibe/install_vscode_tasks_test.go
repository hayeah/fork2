package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

func TestInstallVSCodeTasks(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := ioutil.TempDir("", "vscode-tasks-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .vscode directory
	vscodeDir := filepath.Join(tempDir, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		t.Fatalf("Failed to create .vscode dir: %v", err)
	}

	// Create a stub tasks.json with a comment to ensure comments are preserved
	stubTasksJSON := `{
		// This is a comment that should be preserved
		"version": "2.0.0",
		"tasks": [
			{
				"label": "Existing Task",
				"type": "shell",
				"command": "echo",
				"args": ["hello"]
			}
		],
		"inputs": []
	}`

	stubTasksPath := filepath.Join(vscodeDir, "tasks.json")
	if err := ioutil.WriteFile(stubTasksPath, []byte(stubTasksJSON), 0644); err != nil {
		t.Fatalf("Failed to write stub tasks.json: %v", err)
	}

	// Create a pipe to simulate user input
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
	}()

	// Write "y\n" to the pipe to simulate user confirmation
	go func() {
		defer w.Close()
		w.Write([]byte("y\n"))
	}()

	// Run the installer
	runner := NewInstallVSCodeTasksRunner(tempDir)
	if err := runner.Run(); err != nil {
		t.Fatalf("Failed to run installer: %v", err)
	}

	// Read the resulting file
	resultBytes, err := ioutil.ReadFile(stubTasksPath)
	if err != nil {
		t.Fatalf("Failed to read result file: %v", err)
	}
	result := string(resultBytes)

	// Parse the result to check structure
	val, err := hujson.Parse(resultBytes)
	if err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Check that comments are preserved
	if !strings.Contains(result, "// This is a comment that should be preserved") {
		t.Error("Original comment was not preserved")
	}

	// Standardize the parsed HuJSON before unmarshalling so that the JSON
	// data is valid, while still allowing the file on disk to retain the
	// original comments for human readability.
	valStd := val.Clone()
	valStd.Standardize()

	// Check that the embedded tasks were added
	var resultObj map[string]interface{}
	if err := json.Unmarshal(valStd.Pack(), &resultObj); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	tasks, ok := resultObj["tasks"].([]interface{})
	if !ok {
		t.Fatal("Tasks array not found or not an array")
	}

	// Check that we have the expected number of tasks (original + embedded)
	// The embedded tasks.jsonc has 2 tasks
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	// Check that duplicate tasks are not added
	taskLabels := make(map[string]bool)
	for _, task := range tasks {
		taskMap := task.(map[string]interface{})
		label := taskMap["label"].(string)
		if taskLabels[label] {
			t.Errorf("Duplicate task found: %s", label)
		}
		taskLabels[label] = true
	}

	// Check that the original task is still present
	if !taskLabels["Existing Task"] {
		t.Error("Original task was not preserved")
	}

	// Check that the embedded tasks were added
	if !taskLabels["Copy the current template file"] || !taskLabels["Render the template file"] {
		t.Error("Embedded tasks were not added correctly")
	}
}
