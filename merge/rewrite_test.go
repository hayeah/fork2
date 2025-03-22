package merge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hayeah/fork2/heredoc"
	"github.com/stretchr/testify/assert"
)

func TestRewrite(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "User.swift")

	// Sample content that mimics the Swift struct in the example
	initialContent := `import Foundation
struct User {
    let id: UUID
    var name: String
}
`

	// Write initial content to file
	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	assert.NoError(err, "Failed to create test file")

	// New content for rewrite
	newContent := `import Foundation
struct User {
    let id: UUID
    var name: String
    var email: String

    init(name: String, email: String) {
        self.id = UUID()
        self.name = name
        self.email = email
    }
}
`

	// Build the command
	rewriteCmd := &heredoc.Command{
		Name:    "rewrite",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "content", Payload: newContent},
		},
	}

	action, err := CommandToAction(rewriteCmd)
	assert.NoError(err, "Should build a Rewrite action from the command")
	rewriteAction, ok := action.(*Rewrite)
	assert.True(ok, "Should be a Rewrite action")

	// Test Verify
	err = rewriteAction.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = rewriteAction.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Read the modified file
	modifiedContent, err := os.ReadFile(testFile)
	assert.NoError(err, "Should be able to read modified file")

	// Verify the content was rewritten correctly
	assert.Equal(newContent, string(modifiedContent), "File content should match the new content")

	// Test Verify with non-existent file
	nonExistentCmd := &heredoc.Command{
		Name:    "rewrite",
		Payload: "non-existent-file.txt",
		Params: []heredoc.Param{
			{Name: "content", Payload: newContent},
		},
	}
	nonExistentAction, err := CommandToAction(nonExistentCmd)
	assert.NoError(err)

	err = nonExistentAction.Verify()
	assert.Error(err, "Verify should fail with non-existent file")
	assert.Contains(err.Error(), "file does not exist", "Error should mention file does not exist")

	// Test Verify with empty content
	emptyContentCmd := &heredoc.Command{
		Name:    "rewrite",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "content", Payload: ""},
		},
	}
	emptyContentAction, err := CommandToAction(emptyContentCmd)
	assert.NoError(err)

	err = emptyContentAction.Verify()
	assert.Error(err, "Verify should fail with empty content")
	assert.Contains(err.Error(), "content cannot be empty", "Error should mention content cannot be empty")
}
