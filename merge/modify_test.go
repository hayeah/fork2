package merge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hayeah/fork2/heredoc"
	"github.com/stretchr/testify/assert"
)

func TestModify(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test file
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")

	// Sample content that mimics the Swift struct in the example
	content := `struct User {
  let id: UUID
  var name: String
}
`

	// Write initial content to file
	err := os.WriteFile(testFile, []byte(content), 0644)
	assert.NoError(err, "Failed to create test file")

	// Prepare a command
	search := `struct User {
  let id: UUID
  var name: String
}
`
	replace := `struct User {
    let id: UUID
    var name: String
    var email: String
}
`

	modifyCmd := &heredoc.Command{
		Name:    "modify",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "search", Payload: search},
			{Name: "replace", Payload: replace},
		},
	}

	action, err := CommandToAction(modifyCmd)
	assert.NoError(err, "Should build a Modify action from the command")

	modifyAction, ok := action.(*Modify)
	assert.True(ok, "Should be a Modify action")

	// Test Verify
	err = modifyAction.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = modifyAction.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Read the modified file
	modifiedContent, err := os.ReadFile(testFile)
	assert.NoError(err, "Should be able to read modified file")

	// Verify the content was modified correctly
	assert.Equal(replace, string(modifiedContent), "File content should match the replacement text")

	// Test Verify with non-existent file
	nonExistentCmd := &heredoc.Command{
		Name:    "modify",
		Payload: "non-existent-file.txt",
		Params: []heredoc.Param{
			{Name: "search", Payload: search},
			{Name: "replace", Payload: replace},
		},
	}
	nonExistentAction, err := CommandToAction(nonExistentCmd)
	assert.NoError(err, "CommandToAction should succeed building the Modify action")
	err = nonExistentAction.Verify()
	assert.Error(err, "Verify should fail with non-existent file")
	assert.Contains(err.Error(), "file does not exist", "Error should mention file does not exist")

	// Test Verify with content not found
	wrongSearchCmd := &heredoc.Command{
		Name:    "modify",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "search", Payload: "wrong search content"},
			{Name: "replace", Payload: replace},
		},
	}
	wrongSearchAction, err := CommandToAction(wrongSearchCmd)
	assert.NoError(err, "CommandToAction should build the Modify action")
	err = wrongSearchAction.Verify()
	assert.Error(err, "Verify should fail when search string not found")
	assert.Contains(err.Error(), "search string not found", "Error should mention search string not found")

	// Test Verify with empty search string
	emptySearchCmd := &heredoc.Command{
		Name:    "modify",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "search", Payload: ""},
			{Name: "replace", Payload: replace},
		},
	}
	emptySearchAction, err := CommandToAction(emptySearchCmd)
	assert.NoError(err)
	err = emptySearchAction.Verify()
	assert.Error(err, "Verify should fail with empty search string")
	assert.Contains(err.Error(), "search string cannot be empty", "Error should mention search string cannot be empty")

	// Test Apply with no replacements (should fail)
	noMatchFile := filepath.Join(tempDir, "nomatch.txt")
	err = os.WriteFile(noMatchFile, []byte("This content doesn't match the search pattern"), 0644)
	assert.NoError(err, "Should be able to create no match test file")

	noMatchCmd := &heredoc.Command{
		Name:    "modify",
		Payload: noMatchFile,
		Params: []heredoc.Param{
			{Name: "search", Payload: search},
			{Name: "replace", Payload: replace},
		},
	}
	noMatchAction, err := CommandToAction(noMatchCmd)
	assert.NoError(err)
	err = noMatchAction.Apply()
	assert.Error(err, "Apply should fail when no replacements can be made")
}
