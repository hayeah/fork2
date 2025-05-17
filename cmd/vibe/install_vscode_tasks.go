package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/hujson"
)

//go:embed tasks.jsonc
var tasksJSONCContent []byte

// InstallVSCodeTasksCmd represents the install:vscode:tasks subcommand
type InstallVSCodeTasksCmd struct {
	// No additional flags needed for this command
}

// InstallVSCodeTasksRunner handles the installation of VS Code tasks
type InstallVSCodeTasksRunner struct {
	RootPath string
}

// NewInstallVSCodeTasksRunner creates a new runner for installing VS Code tasks
func NewInstallVSCodeTasksRunner(rootPath string) *InstallVSCodeTasksRunner {
	return &InstallVSCodeTasksRunner{
		RootPath: rootPath,
	}
}

// Run executes the installation of VS Code tasks
func (r *InstallVSCodeTasksRunner) Run() error {
	// Resolve the destination path
	vscodeDir := filepath.Join(r.RootPath, ".vscode")
	tasksPath := filepath.Join(vscodeDir, "tasks.json")

	// Ensure .vscode directory exists
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .vscode directory: %w", err)
	}

	// Load the embedded tasks.jsonc
	embeddedValue, err := loadJSONCBytes(tasksJSONCContent)
	if err != nil {
		return fmt.Errorf("failed to parse embedded tasks.jsonc: %w", err)
	}

	// Check if destination tasks.json exists
	var destValue *hujson.Value
	if _, err := os.Stat(tasksPath); err == nil {
		// Load existing tasks.json
		destValue, err = loadJSONC(tasksPath)
		if err != nil {
			return fmt.Errorf("failed to parse existing tasks.json: %w", err)
		}
	} else if os.IsNotExist(err) {
		// No existing file, just use the embedded one
		fmt.Printf("No existing tasks.json found. Creating new file at %s\n", tasksPath)
		destValue = embeddedValue
	} else {
		return fmt.Errorf("failed to check for existing tasks.json: %w", err)
	}

	// If we have an existing file, merge the tasks
	if destValue != embeddedValue {
		fmt.Println("Merging with existing tasks.json...")
		merged, err := mergeJSON(destValue, embeddedValue)
		if err != nil {
			return fmt.Errorf("failed to merge tasks: %w", err)
		}
		destValue = merged
	}

	// Format the result
	destValue.Format()

	// Preview the changes
	fmt.Println("Preview of the updated tasks.json:")
	previewJSON(destValue)

	// Ask for confirmation
	if !promptConfirmation("Do you want to save these changes?") {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// Save the result
	if err := saveJSON(tasksPath, destValue); err != nil {
		return fmt.Errorf("failed to save tasks.json: %w", err)
	}

	fmt.Printf("Successfully updated %s\n", tasksPath)
	return nil
}

// loadJSONCBytes parses JSONC bytes into a hujson.Value
func loadJSONCBytes(data []byte) (*hujson.Value, error) {
	ast, err := hujson.Parse(data)
	if err != nil {
		return nil, err
	}
	return &ast, nil
}

// loadJSONC loads a JSONC file into a hujson.Value
func loadJSONC(path string) (*hujson.Value, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadJSONCBytes(data)
}

// saveJSON saves a hujson.Value to a file
func saveJSON(path string, val *hujson.Value) error {
	return ioutil.WriteFile(path, val.Pack(), 0644)
}

// mergeJSON merges two JSON values, preserving comments
func mergeJSON(dest, src *hujson.Value) (*hujson.Value, error) {
	// Extract tasks and inputs from both files
	var destObj, srcObj map[string]interface{}
	if err := json.Unmarshal(dest.Pack(), &destObj); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(src.Pack(), &srcObj); err != nil {
		return nil, err
	}

	// Get tasks arrays
	destTasks, destHasTasks := destObj["tasks"].([]interface{})
	srcTasks, srcHasTasks := srcObj["tasks"].([]interface{})

	// Get inputs arrays
	destInputs, destHasInputs := destObj["inputs"].([]interface{})
	srcInputs, srcHasInputs := srcObj["inputs"].([]interface{})

	// Create patch operations
	var patchOps []map[string]interface{}

	// Handle tasks
	if srcHasTasks {
		if !destHasTasks {
			// If dest doesn't have tasks, add the entire array
			patchOps = append(patchOps, map[string]interface{}{
				"op":    "add",
				"path":  "/tasks",
				"value": srcTasks,
			})
		} else {
			// Merge tasks, avoiding duplicates
			destTasksMap := toMapSlice(destTasks, "label")

			for _, task := range srcTasks {
				taskMap := task.(map[string]interface{})
				label := taskMap["label"].(string)
				if _, exists := destTasksMap[label]; !exists {
					patchOps = append(patchOps, map[string]interface{}{
						"op":    "add",
						"path":  "/tasks/-",
						"value": task,
					})
				}
			}
		}
	}

	// Handle inputs
	if srcHasInputs {
		if !destHasInputs {
			// If dest doesn't have inputs, add the entire array
			patchOps = append(patchOps, map[string]interface{}{
				"op":    "add",
				"path":  "/inputs",
				"value": srcInputs,
			})
		} else {
			// Merge inputs, avoiding duplicates
			// srcInputsMap not used directly as we iterate through srcInputs
			destInputsMap := toMapSlice(destInputs, "id")

			for _, input := range srcInputs {
				inputMap := input.(map[string]interface{})
				id := inputMap["id"].(string)
				if _, exists := destInputsMap[id]; !exists {
					patchOps = append(patchOps, map[string]interface{}{
						"op":    "add",
						"path":  "/inputs/-",
						"value": input,
					})
				}
			}
		}
	}

	// If no patches needed, return the original
	if len(patchOps) == 0 {
		return dest, nil
	}

	// Convert patch operations to JSON
	patchBytes, err := json.Marshal(patchOps)
	if err != nil {
		return nil, err
	}

	// Clone the destination to avoid modifying it if patch fails
	destClone := dest.Clone()

	// Apply the patch
	if err := destClone.Patch(patchBytes); err != nil {
		return nil, err
	}

	return &destClone, nil
}

// toMapSlice converts a slice of maps to a map keyed by the specified field
func toMapSlice(slice []interface{}, keyField string) map[string]interface{} {
	result := make(map[string]interface{})
	for _, item := range slice {
		if m, ok := item.(map[string]interface{}); ok {
			if key, ok := m[keyField].(string); ok {
				result[key] = item
			}
		}
	}
	return result
}

// previewJSON prints a preview of the JSON value
func previewJSON(val *hujson.Value) {
	fmt.Println(string(val.Pack()))
}

// promptConfirmation asks the user for confirmation
func promptConfirmation(prompt string) bool {
	fmt.Printf("%s (y/N): ", prompt)
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(response) == "y"
}
