package merge

import (
	"os"
	"path/filepath"
	"testing"

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

	// Create a Delete action
	deleteAction := NewDelete(testFile)

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
	nonExistentDelete := NewDelete(filepath.Join(tempDir, "NonExistent.txt"))
	err = nonExistentDelete.Verify()
	assert.Error(err, "Verify should fail when file does not exist")
	assert.Contains(err.Error(), "file does not exist", "Error should mention file does not exist")
}
