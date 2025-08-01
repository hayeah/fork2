package selection

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSelection_ReadString_LockFiles(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "lockfile-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	lockFiles := map[string]string{
		"package-lock.json": `{"name": "test", "lockfileVersion": 2}`,
		"go.sum":            "github.com/example/module v1.0.0 h1:...",
		"yarn.lock":         "# THIS IS AN AUTOGENERATED FILE",
	}

	for filename, content := range lockFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Test each lock file
	for filename := range lockFiles {
		t.Run(filename, func(t *testing.T) {
			fs := NewFileSelection(os.DirFS(tmpDir), filename, nil)

			result, err := fs.ReadString()
			if err != nil {
				t.Fatalf("ReadString() error = %v", err)
			}

			// Check that the file path is included in the comment
			if !strings.Contains(result, filename) {
				t.Errorf("Result does not contain filename %s", filename)
			}

			// Check that the content is omitted
			if !strings.Contains(result, "[lock file omitted]") {
				t.Errorf("Result does not contain '[lock file omitted]' for %s", filename)
			}

			// Ensure the actual content is not included
			if strings.Contains(result, lockFiles[filename]) {
				t.Errorf("Result contains actual lock file content for %s", filename)
			}
		})
	}

	// Test a regular file to ensure it's not affected
	t.Run("regular file", func(t *testing.T) {
		regularFile := filepath.Join(tmpDir, "regular.txt")
		regularContent := "This is regular content"
		if err := os.WriteFile(regularFile, []byte(regularContent), 0644); err != nil {
			t.Fatal(err)
		}

		fs := NewFileSelection(os.DirFS(tmpDir), "regular.txt", nil)
		result, err := fs.ReadString()
		if err != nil {
			t.Fatalf("ReadString() error = %v", err)
		}

		// Check that regular file content is included
		if !strings.Contains(result, regularContent) {
			t.Errorf("Result does not contain regular file content")
		}

		// Ensure it's not marked as omitted
		if strings.Contains(result, "[lock file omitted]") {
			t.Errorf("Regular file incorrectly marked as lock file")
		}
	})
}
