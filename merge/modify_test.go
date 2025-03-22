package merge

import (
	"os"
	"path/filepath"
	"testing"

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

	// Create a Modify action
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

	modify := NewModify(testFile, search, replace)

	// Test Verify
	err = modify.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = modify.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Read the modified file
	modifiedContent, err := os.ReadFile(testFile)
	assert.NoError(err, "Should be able to read modified file")

	// Verify the content was modified correctly
	assert.Equal(replace, string(modifiedContent), "File content should match the replacement text")

	// Test Verify with non-existent file
	nonExistentModify := NewModify("non-existent-file.txt", search, replace)
	err = nonExistentModify.Verify()
	assert.Error(err, "Verify should fail with non-existent file")
	assert.Contains(err.Error(), "file does not exist", "Error should mention file does not exist")

	// Test Verify with content not found
	wrongSearchModify := NewModify(testFile, "wrong search content", replace)
	err = wrongSearchModify.Verify()
	assert.Error(err, "Verify should fail when search string not found")
	assert.Contains(err.Error(), "search string not found", "Error should mention search string not found")

	// Test Verify with empty search string
	emptySearchModify := NewModify(testFile, "", replace)
	err = emptySearchModify.Verify()
	assert.Error(err, "Verify should fail with empty search string")
	assert.Contains(err.Error(), "search string cannot be empty", "Error should mention search string cannot be empty")

	// Test Apply with no replacements (should fail)
	// First, create a file with content that doesn't match the search pattern
	noMatchFile := filepath.Join(tempDir, "nomatch.txt")
	err = os.WriteFile(noMatchFile, []byte("This content doesn't match the search pattern"), 0644)
	assert.NoError(err, "Should be able to create no match test file")

	noMatchModify := NewModify(noMatchFile, search, replace)
	err = noMatchModify.Apply()
	assert.Error(err, "Apply should fail when no replacements can be made")
}
