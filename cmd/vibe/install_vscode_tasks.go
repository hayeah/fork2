package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	jsonc "github.com/muhammadmuzzammil1998/jsonc"
)

// InstallVSCodeTasksCmd defines the install:vscode:tasks subcommand.
type InstallVSCodeTasksCmd struct{}

// InstallVSCodeTasksRunner performs the installation logic.
type InstallVSCodeTasksRunner struct {
	RootPath string
}

func loadJSONC(path string) (map[string]any, error) {
	data := make(map[string]any)
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	cleaned := jsonc.ToJSON(bytes)
	if err := json.Unmarshal(cleaned, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func saveJSON(path string, data map[string]any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func deduplicate(items []map[string]any, key string) []map[string]any {
	seen := make(map[string]bool)
	var unique []map[string]any
	for _, item := range items {
		val, _ := item[key].(string)
		if val == "" || !seen[val] {
			if val != "" {
				seen[val] = true
			}
			unique = append(unique, item)
		}
	}
	return unique
}

func toMapSlice(v any) []map[string]any {
	arr, _ := v.([]any)
	res := make([]map[string]any, 0, len(arr))
	for _, it := range arr {
		if m, ok := it.(map[string]any); ok {
			res = append(res, m)
		}
	}
	return res
}

func mergeJSON(src, dest map[string]any) map[string]any {
	if dest == nil {
		dest = map[string]any{}
	}
	version, ok := dest["version"]
	if !ok {
		if v, ok2 := src["version"]; ok2 {
			version = v
		} else {
			version = "2.0.0"
		}
	}
	dest["version"] = version

	destTasks := toMapSlice(dest["tasks"])
	srcTasks := toMapSlice(src["tasks"])
	destInputs := toMapSlice(dest["inputs"])
	srcInputs := toMapSlice(src["inputs"])

	destTasks = append(destTasks, srcTasks...)
	destInputs = append(destInputs, srcInputs...)

	dest["tasks"] = deduplicate(destTasks, "label")
	dest["inputs"] = deduplicate(destInputs, "id")
	return dest
}

func previewJSON(data map[string]any) {
	b, _ := json.MarshalIndent(data, "", "  ")
	fmt.Printf("\nPreview of merged tasks.json:\n\n%s\n\n", string(b))
}

func promptConfirmation(msg string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s (y/n): ", msg)
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp == "y" || resp == "yes" {
			return true
		}
		if resp == "n" || resp == "no" {
			return false
		}
		fmt.Println("Please enter 'y' or 'n'.")
	}
}

func (r *InstallVSCodeTasksRunner) Run() error {
	srcPath := filepath.Join(r.RootPath, "vibe.tasks.jsonc")
	dstPath := filepath.Join(r.RootPath, ".vscode", "tasks.json")

	src, err := loadJSONC(srcPath)
	if err != nil {
		return err
	}
	if len(src) == 0 {
		return fmt.Errorf("source file %s not found", srcPath)
	}

	dst, err := loadJSONC(dstPath)
	if err != nil {
		return err
	}

	merged := mergeJSON(src, dst)
	previewJSON(merged)

	if !promptConfirmation(fmt.Sprintf("Write merged tasks to %s?", dstPath)) {
		fmt.Println("Operation cancelled.")
		return nil
	}

	if err := saveJSON(dstPath, merged); err != nil {
		return err
	}

	fmt.Printf("✅ Successfully merged %s into %s\n", filepath.Base(srcPath), dstPath)
	return nil
}
