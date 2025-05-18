package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hayeah/fork2/internal/hujsonutil"
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

// vscodeTask represents a single VS Code task definition.
type vscodeTask struct {
	Label        string                 `json:"label"`
	Type         string                 `json:"type"`
	Command      string                 `json:"command"`
	Args         []string               `json:"args,omitempty"`
	Options      map[string]interface{} `json:"options,omitempty"`
	Presentation map[string]interface{} `json:"presentation,omitempty"`
	// Keep the "problemMatcher" property even when empty so VS Code doesn't
	// fall back to default problem matchers. "omitempty" would drop the
	// field when the slice is empty, resulting in the config missing this
	// property entirely.
	ProblemMatcher []interface{} `json:"problemMatcher"`
}

// vscodeInput represents a VS Code input definition.
type vscodeInput struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// tasksFile is a partial representation of tasks.json that only cares about
// tasks and inputs.
type tasksFile struct {
	Tasks  []vscodeTask  `json:"tasks"`
	Inputs []vscodeInput `json:"inputs"`
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

// mergeJSON merges the embedded `src` tasks/inputs into `dest` while
// preserving ordering, comments, and without duplicating labels/ids.
// It relies on hujsonutil.Value.InsertToArray for all “insert or create” work.
func mergeJSON(dest, src *hujson.Value) (*hujson.Value, error) {
	// Wrap helpers
	dv := hujsonutil.NewValue(dest)

	// Standardised clones are only used for (un)marshalling into Go structs.
	destStd := dest.Clone()
	destStd.Standardize()
	srcStd := src.Clone()
	srcStd.Standardize()

	var destObj, srcObj tasksFile
	if err := json.Unmarshal(destStd.Pack(), &destObj); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(srcStd.Pack(), &srcObj); err != nil {
		return nil, err
	}

	// Insert new tasks
	for _, task := range srcObj.Tasks {
		if task.Label == "" || taskExists(destObj.Tasks, task.Label) {
			continue
		}
		if err := dv.InsertToArray("/tasks", task); err != nil {
			return nil, err
		}
		destObj.Tasks = append(destObj.Tasks, task) // keep duplicate check up-to-date
	}

	// Insert new inputs
	for _, in := range srcObj.Inputs {
		if in.ID == "" || inputExists(destObj.Inputs, in.ID) {
			continue
		}
		if err := dv.InsertToArray("/inputs", in); err != nil {
			return nil, err
		}
		destObj.Inputs = append(destObj.Inputs, in)
	}

	// Nothing else to touch – comments are preserved inside dv.
	return dest, nil
}

// taskExists checks if a task with the given label already exists in the slice.
func taskExists(tasks []vscodeTask, label string) bool {
	for _, t := range tasks {
		if t.Label == label {
			return true
		}
	}
	return false
}

// inputExists checks if an input with the given id already exists in the slice.
func inputExists(inputs []vscodeInput, id string) bool {
	for _, in := range inputs {
		if in.ID == id {
			return true
		}
	}
	return false
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
