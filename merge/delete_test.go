package merge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hayeah/fork2/heredoc"
	"github.com/stretchr/testify/assert"
)

func TestDelete(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test directory
	tempDir := t.TempDir()

	// Test file path for a file to delete
	testFile := filepath.Join(tempDir, "Obsolete", "File.swift")

	// Create the directory and file first
	err := os.MkdirAll(filepath.Dir(testFile), 0755)
	assert.NoError(err, "Should be able to create test directory")

	// Write some content to the file
	err = os.WriteFile(testFile, []byte("This file will be deleted"), 0644)
	assert.NoError(err, "Should be able to create test file")

	// Verify the file exists before deletion
	_, err = os.Stat(testFile)
	assert.NoError(err, "File should exist before deletion")

	// Build the command for delete
	deleteCmd := &heredoc.Command{
		Name:    "delete",
		Payload: testFile,
	}

	action, err := CommandToAction(deleteCmd)
	assert.NoError(err, "Should build a Delete action from the command")
	deleteAction, ok := action.(*Delete)
	assert.True(ok, "Should be a Delete action")

	// Test Verify
	err = deleteAction.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = deleteAction.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Check if the file was deleted
	_, err = os.Stat(testFile)
	assert.Error(err, "File should not exist after deletion")
	assert.True(os.IsNotExist(err), "Error should indicate file does not exist")

	// Test deleting a non-existent file (should fail)
	nonExistentCmd := &heredoc.Command{
		Name:    "delete",
		Payload: filepath.Join(tempDir, "NonExistent.txt"),
	}
	nonExistentAction, err := CommandToAction(nonExistentCmd)
	assert.NoError(err)

	err = nonExistentAction.Verify()
	assert.Error(err, "Verify should fail when file does not exist")
	assert.Contains(err.Error(), "file does not exist", "Error should mention file does not exist")
}
