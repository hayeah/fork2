package merge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hayeah/fork2/heredoc"
	"github.com/stretchr/testify/assert"
)

func TestEditWriteAll(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test directory
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "writeall.txt")

	// Build the command
	writeAllCmd := &heredoc.Command{
		Name:    "edit",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "action", Payload: "writeAll"},
			{Name: "content", Payload: "This\nis the new content\nof the file"},
		},
	}

	action, err := CommandToAction(writeAllCmd)
	assert.NoError(err, "Should build an EditWriteAll action from the command")
	writeAllAction, ok := action.(*EditWriteAll)
	assert.True(ok, "Should be an EditWriteAll action")

	// Test Verify
	err = writeAllAction.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = writeAllAction.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Check if the file was created
	_, err = os.Stat(testFile)
	assert.NoError(err, "File should exist after creation")

	// Read the created file
	content, err := os.ReadFile(testFile)
	assert.NoError(err, "Should be able to read created file")

	// Verify the content was written correctly
	assert.Equal("This\nis the new content\nof the file", string(content), "File content should match")

	// Test without content param (should fail)
	emptyCmd := &heredoc.Command{
		Name:    "edit",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "action", Payload: "writeAll"},
		},
	}
	emptyAction, err := CommandToAction(emptyCmd)
	assert.NoError(err)

	err = emptyAction.Verify()
	assert.Error(err, "Verify should fail with missing content")
	assert.Contains(err.Error(), "content parameter is required", "Error should mention content is required")
}

func TestEditChange(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test directory
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "change.txt")

	// Original content
	originalContent := `// foo is a function
function foo() {
   console.log("hello foo");
   return null;
}
`

	// Write initial content to file
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	assert.NoError(err, "Failed to create test file")

	// Build the command
	changeCmd := &heredoc.Command{
		Name:    "edit",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "action", Payload: "change"},
			{Name: "description", Payload: "rewrite foo function as bar"},
			{Name: "search", Payload: `// foo is a function
function foo() {
   console.log("hello foo");
   return null;
}`},
			{Name: "content", Payload: `// bar is a function
function bar() {
   printf("hello bar");
   return null;
}`},
		},
	}

	action, err := CommandToAction(changeCmd)
	assert.NoError(err, "Should build an EditChange action from the command")
	changeAction, ok := action.(*EditChange)
	assert.True(ok, "Should be an EditChange action")

	// Test Verify
	err = changeAction.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = changeAction.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Read the modified file
	modifiedContent, err := os.ReadFile(testFile)
	assert.NoError(err, "Should be able to read modified file")

	// Verify the content was modified correctly
	expectedContent := `// bar is a function
function bar() {
   printf("hello bar");
   return null;
}
`
	assert.Equal(expectedContent, string(modifiedContent), "File content should match the replacement text")

	// Test with non-existent file
	nonExistentCmd := &heredoc.Command{
		Name:    "edit",
		Payload: "non-existent-file.txt",
		Params: []heredoc.Param{
			{Name: "action", Payload: "change"},
			{Name: "search", Payload: "search text"},
			{Name: "content", Payload: "replacement text"},
		},
	}
	nonExistentAction, err := CommandToAction(nonExistentCmd)
	assert.NoError(err)

	err = nonExistentAction.Verify()
	assert.Error(err, "Verify should fail with non-existent file")
	assert.Contains(err.Error(), "file does not exist", "Error should mention file does not exist")
}

func TestEditInsert(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test directory
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "insert.txt")

	// Original content
	originalContent := `// foo is a function
function foo() {
   console.log("hello foo");
   return null;
}
`

	// Write initial content to file
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	assert.NoError(err, "Failed to create test file")

	// Build the command
	insertCmd := &heredoc.Command{
		Name:    "edit",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "action", Payload: "insert"},
			{Name: "description", Payload: "define runBeforeFoo before foo()"},
			{Name: "search", Payload: `// foo is a function
function foo() {`},
			{Name: "content", Payload: `// do stuff before foo
function runBeforeFoo() {
   setupFoo();
}

`},
		},
	}

	action, err := CommandToAction(insertCmd)
	assert.NoError(err, "Should build an EditInsert action from the command")
	insertAction, ok := action.(*EditInsert)
	assert.True(ok, "Should be an EditInsert action")

	// Test Verify
	err = insertAction.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = insertAction.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Read the modified file
	modifiedContent, err := os.ReadFile(testFile)
	assert.NoError(err, "Should be able to read modified file")

	// Verify the content was modified correctly
	expectedContent := `// do stuff before foo
function runBeforeFoo() {
   setupFoo();
}

// foo is a function
function foo() {
   console.log("hello foo");
   return null;
}
`
	assert.Equal(expectedContent, string(modifiedContent), "File content should include the inserted text before the search match")
}

func TestEditAppend(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test directory
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "append.txt")

	// Original content
	originalContent := `// foo is a function
function foo() {
   console.log("hello foo");
   return null;
}
`

	// Write initial content to file
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	assert.NoError(err, "Failed to create test file")

	// Build the command
	appendCmd := &heredoc.Command{
		Name:    "edit",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "action", Payload: "append"},
			{Name: "description", Payload: "define runAfterFoo after foo()"},
			{Name: "search", Payload: `// foo is a function
function foo() {
   console.log("hello foo");
   return null;
}`},
			{Name: "content", Payload: `

// do stuff after foo
function runAfterFoo() {
   teardownFoo();
}`},
		},
	}

	action, err := CommandToAction(appendCmd)
	assert.NoError(err, "Should build an EditAppend action from the command")
	appendAction, ok := action.(*EditAppend)
	assert.True(ok, "Should be an EditAppend action")

	// Test Verify
	err = appendAction.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = appendAction.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Read the modified file
	modifiedContent, err := os.ReadFile(testFile)
	assert.NoError(err, "Should be able to read modified file")

	// Verify the content was modified correctly
	expectedContent := `// foo is a function
function foo() {
   console.log("hello foo");
   return null;
}

// do stuff after foo
function runAfterFoo() {
   teardownFoo();
}
`
	assert.Equal(expectedContent, string(modifiedContent), "File content should include the appended text after the search match")
}

func TestEditDelete(t *testing.T) {
	assert := assert.New(t)

	// Create a temporary test directory
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "delete.txt")

	// Original content with multiple functions
	originalContent := `// helper function
function setup() {
   console.log("setting up");
}

// foo is a function
function foo() {
   console.log("hello foo");
   return null;
}

// another helper
function cleanup() {
   console.log("cleaning up");
}
`

	// Write initial content to file
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	assert.NoError(err, "Failed to create test file")

	// Build the command
	deleteCmd := &heredoc.Command{
		Name:    "edit",
		Payload: testFile,
		Params: []heredoc.Param{
			{Name: "action", Payload: "delete"},
			{Name: "description", Payload: "delete foo function"},
			{Name: "search", Payload: `// foo is a function
function foo() {
   console.log("hello foo");
   return null;
}
`},
		},
	}

	action, err := CommandToAction(deleteCmd)
	assert.NoError(err, "Should build an EditDelete action from the command")
	deleteAction, ok := action.(*EditDelete)
	assert.True(ok, "Should be an EditDelete action")

	// Test Verify
	err = deleteAction.Verify()
	assert.NoError(err, "Verify should succeed with valid inputs")

	// Test Apply
	err = deleteAction.Apply()
	assert.NoError(err, "Apply should succeed with valid inputs")

	// Read the modified file
	modifiedContent, err := os.ReadFile(testFile)
	assert.NoError(err, "Should be able to read modified file")

	// Verify the content was modified correctly
	expectedContent := `// helper function
function setup() {
   console.log("setting up");
}


// another helper
function cleanup() {
   console.log("cleaning up");
}
`
	assert.Equal(expectedContent, string(modifiedContent), "File content should not include the deleted text")
}

func TestCommandToActionEdit(t *testing.T) {
	assert := assert.New(t)

	// Test with valid edit action
	validCmd := &heredoc.Command{
		Name:    "edit",
		Payload: "file.txt",
		Params: []heredoc.Param{
			{Name: "action", Payload: "writeAll"},
			{Name: "content", Payload: "content"},
		},
	}

	action, err := CommandToAction(validCmd)
	assert.NoError(err, "Should build an action from a valid edit command")
	_, ok := action.(*EditWriteAll)
	assert.True(ok, "Should be an EditWriteAll action")

	// Test with missing action parameter
	missingActionCmd := &heredoc.Command{
		Name:    "edit",
		Payload: "file.txt",
		Params: []heredoc.Param{
			{Name: "content", Payload: "content"},
		},
	}

	_, err = CommandToAction(missingActionCmd)
	assert.Error(err, "Should fail with missing action parameter")
	assert.Contains(err.Error(), "action parameter is required", "Error should mention action is required")

	// Test with unsupported action
	unsupportedActionCmd := &heredoc.Command{
		Name:    "edit",
		Payload: "file.txt",
		Params: []heredoc.Param{
			{Name: "action", Payload: "unsupported"},
		},
	}

	_, err = CommandToAction(unsupportedActionCmd)
	assert.Error(err, "Should fail with unsupported action")
	assert.Contains(err.Error(), "unsupported edit action", "Error should mention unsupported action")
}
