package merge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreate(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test directory
	tempDir := t.TempDir()
	
	// Test file path for a new file
	testFile := filepath.Join(tempDir, "Views", "RoundedButton.swift")
	
	// Content for the new file
	content := `import UIKit
@IBDesignable
class RoundedButton: UIButton {
    @IBInspectable var cornerRadius: CGFloat = 0
}
`

	// Create a Create action
	create := NewCreate(testFile, content)

	// Test Verify
	err := create.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = create.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Check if the file was created
	_, err = os.Stat(testFile)
	assert.NoError(err, "File should exist after creation")

	// Read the created file
	createdContent, err := os.ReadFile(testFile)
	assert.NoError(err, "Should be able to read created file")

	// Verify the content was written correctly
	assert.Equal(content, string(createdContent), "File content should match the provided content")

	// Test creating a file that already exists (should fail)
	duplicateCreate := NewCreate(testFile, content)
	err = duplicateCreate.Verify()
	assert.Error(err, "Verify should fail when file already exists")
	assert.Contains(err.Error(), "file already exists", "Error should mention file already exists")

	// Test creating a file in a non-existent directory that can't be created
	// This is a bit tricky to test in a cross-platform way, so we'll skip it for now
	
	// Test creating a file with empty content (should still work)
	emptyFile := filepath.Join(tempDir, "EmptyFile.txt")
	emptyCreate := NewCreate(emptyFile, "")
	err = emptyCreate.Verify()
	assert.NoError(err, "Verify should succeed with empty content")
	
	err = emptyCreate.Apply()
	assert.NoError(err, "Apply should succeed with empty content")
	
	// Check if the empty file was created
	fileInfo, err := os.Stat(emptyFile)
	assert.NoError(err, "Empty file should exist after creation")
	assert.Zero(fileInfo.Size(), "File should be empty")
}
