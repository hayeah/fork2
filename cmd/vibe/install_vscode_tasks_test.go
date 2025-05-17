package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallVSCodeTasks(t *testing.T) {
	assert := assert.New(t)

	tempDir := t.TempDir()

	srcContent := `{
        // comment
        "version": "2.0.0",
        "tasks": [
            {"label": "taskA", "command": "echo A"},
            {"label": "taskB", "command": "echo B"}
        ],
        "inputs": [
            {"id": "input1", "type": "promptString"}
        ]
    }`
	os.WriteFile(filepath.Join(tempDir, "vibe.tasks.jsonc"), []byte(srcContent), 0644)

	os.Mkdir(filepath.Join(tempDir, ".vscode"), 0755)
	dstContent := `{
        "version": "2.0.0",
        "tasks": [
            {"label": "taskB", "command": "echo B old"},
            {"label": "taskC", "command": "echo C"}
        ]
    }`
	os.WriteFile(filepath.Join(tempDir, ".vscode", "tasks.json"), []byte(dstContent), 0644)

	runner := &InstallVSCodeTasksRunner{RootPath: tempDir}

	// Feed confirmation
	f, err := os.CreateTemp(tempDir, "input")
	assert.NoError(err)
	_, err = f.WriteString("y\n")
	assert.NoError(err)
	f.Close()
	inputFile, _ := os.Open(f.Name())
	oldStdin := os.Stdin
	os.Stdin = inputFile
	defer func() { os.Stdin = oldStdin; inputFile.Close() }()

	err = runner.Run()
	assert.NoError(err)

	data, err := os.ReadFile(filepath.Join(tempDir, ".vscode", "tasks.json"))
	assert.NoError(err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	assert.NoError(err)

	tasks := result["tasks"].([]any)
	labels := []string{}
	for _, v := range tasks {
		m := v.(map[string]any)
		labels = append(labels, m["label"].(string))
	}
	assert.Equal([]string{"taskB", "taskC", "taskA"}, labels)

	inputs := result["inputs"].([]any)
	inputIDs := []string{}
	for _, v := range inputs {
		m := v.(map[string]any)
		inputIDs = append(inputIDs, m["id"].(string))
	}
	assert.Equal([]string{"input1"}, inputIDs)
}
