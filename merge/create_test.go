package merge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hayeah/fork2/heredoc"
	"github.com/stretchr/testify/assert"
)

func TestCreate(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test directory
	tempDir := t.TempDir()

	// Test file path for a new file
	testFile := filepath.Join(tempDir, "RoundedButton.swift")

	// Content for the new file
	content := `import UIKit
@IBDesignable
class RoundedButton: UIButton {
    @IBInspectable var cornerRadius: CGFloat = 0
}
`

	// Build the command
	createCmd := &heredoc.Command{
		Name:    "create",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "content", Payload: content},
		},
	}

	action, err := CommandToAction(createCmd)
	assert.NoError(err, "Should build a Create action from the command")
	createAction, ok := action.(*Create)
	assert.True(ok, "Should be a Create action")

	// Test Verify
	err = createAction.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = createAction.Apply()
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
	duplicateCmd := &heredoc.Command{
		Name:    "create",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "content", Payload: content},
		},
	}
	duplicateAction, err := CommandToAction(duplicateCmd)
	assert.NoError(err, "Should build a Create action from the command")

	err = duplicateAction.Verify()
	assert.Error(err, "Verify should fail when file already exists")
	assert.Contains(err.Error(), "file already exists", "Error should mention file already exists")

	// Test creating a file with empty content param (should fail on Apply)
	emptyFile := filepath.Join(tempDir, "EmptyFile.txt")
	emptyCmd := &heredoc.Command{
		Name:    "create",
		Payload: emptyFile,
		Params:  []heredoc.Param{},
	}
	emptyAction, err := CommandToAction(emptyCmd)
	assert.NoError(err, "Should build a Create action from the command")

	// Even though no content param means verify won't catch content being missing,
	// the apply will fail due to missing param for content
	err = emptyAction.Verify()
	assert.NoError(err, "Verify might succeed if directory exists, but content param is missing")

	err = emptyAction.Apply()
	assert.Error(err, "Apply should fail with missing content param")
}
