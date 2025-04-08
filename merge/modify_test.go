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

	t.Run("multi-line search block", func(t *testing.T) {
		tfile := filepath.Join(tempDir, "multiline.txt")
		src := `begin
middle
end`
		err := os.WriteFile(tfile, []byte(src), 0644)
		assert.NoError(err)

		search := `begin
...
end`
		replace := `something else`

		multiCmd := &heredoc.Command{
			Name:    "modify",
			Payload: tfile,
			Params: []heredoc.Param{
				{Name: "search", Payload: search},
				{Name: "replace", Payload: replace},
			},
		}

		multiAct, err := CommandToAction(multiCmd)
		assert.NoError(err)

		err = multiAct.Verify()
		assert.NoError(err, "Should find multiline search block in file")

		err = multiAct.Apply()
		assert.NoError(err, "Apply should succeed with multiline block")

		result, err := os.ReadFile(tfile)
		assert.NoError(err)
		assert.Equal("something else", string(result), "Should replace entire block between begin...end")
	})

	t.Run("end without begin returns error", func(t *testing.T) {
		endNoBeginFile := filepath.Join(tempDir, "endNoBegin.txt")
		src := "end content"
		err := os.WriteFile(endNoBeginFile, []byte(src), 0644)
		assert.NoError(err, "Should write test file")

		search := `...
end content`
		replace := "whatever"

		cmd := &heredoc.Command{
			Name:    "modify",
			Payload: endNoBeginFile,
			Params: []heredoc.Param{
				{Name: "search", Payload: search},
				{Name: "replace", Payload: replace},
			},
		}

		act, err := CommandToAction(cmd)
		assert.NoError(err, "Should create a Modify action")

		err = act.Verify()
		assert.Error(err, "Should fail verify if end is present but no begin")
		assert.Contains(err.Error(), "search block has End but no Begin")
	})
}

func TestModifyWithOmission(t *testing.T) {
	// assert := assert.New(t)

	// Create a temporary test directory
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "omission.txt")

	// Original content with multiple sections
	originalContent := `// Start of file
package demo

import (
    "fmt"
    "strings"
)

// foo is a function that does foo things
func foo() {
    fmt.Println("This is the foo function")
    doSomethingInFoo()
    return
}

// bar is another function
func bar() {
    fmt.Println("This is the bar function")
    return
}
`

	// Write initial content to file
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	assert.NoError(t, err, "Failed to create test file")

	// Test case 1: Replace with omission - complete function replacement
	t.Run("replace function with omission", func(t *testing.T) {
		assert := assert.New(t)

		search := `// foo is a function that does foo things
func foo() {
...
}
`
		replace := `// foo2 is a better function
func foo2() {
    fmt.Println("This is the improved foo2 function")
    return
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

		// Verify should succeed
		err = action.Verify()
		assert.NoError(err, "Verify should succeed with valid search using omission")

		// Apply the modification
		err = action.Apply()
		assert.NoError(err, "Apply should succeed")

		// Read the modified file
		modifiedContent, err := os.ReadFile(testFile)
		assert.NoError(err, "Should be able to read modified file")

		// Expected content after replacement
		expectedContent := `// Start of file
package demo

import (
    "fmt"
    "strings"
)

// foo2 is a better function
func foo2() {
    fmt.Println("This is the improved foo2 function")
    return
}

// bar is another function
func bar() {
    fmt.Println("This is the bar function")
    return
}
`
		assert.Equal(expectedContent, string(modifiedContent), "File content should match expected after omission replacement")
	})

	// Test case 2: Replace with omission - partial content
	t.Run("replace partial content with omission", func(t *testing.T) {
		assert := assert.New(t)

		// Create a new test file for this case
		testFile2 := filepath.Join(tempDir, "omission2.txt")
		err := os.WriteFile(testFile2, []byte(originalContent), 0644)
		assert.NoError(err, "Failed to create second test file")

		search := `import (
...
)
`
		replace := `import (
    "fmt"
    "strings"
    "os"
)
`

		modifyCmd := &heredoc.Command{
			Name:    "modify",
			Payload: testFile2,
			Params: []heredoc.Param{
				{Name: "search", Payload: search},
				{Name: "replace", Payload: replace},
			},
		}

		action, err := CommandToAction(modifyCmd)
		assert.NoError(err, "Should build a Modify action from the command")

		// Verify should succeed
		err = action.Verify()
		assert.NoError(err, "Verify should succeed with valid search using omission")

		// Apply the modification
		err = action.Apply()
		assert.NoError(err, "Apply should succeed")

		// Read the modified file
		modifiedContent, err := os.ReadFile(testFile2)
		assert.NoError(err, "Should be able to read modified file")

		// Expected content after replacement
		expectedContent := `// Start of file
package demo

import (
    "fmt"
    "strings"
    "os"
)

// foo is a function that does foo things
func foo() {
    fmt.Println("This is the foo function")
    doSomethingInFoo()
    return
}

// bar is another function
func bar() {
    fmt.Println("This is the bar function")
    return
}
`
		assert.Equal(expectedContent, string(modifiedContent), "File content should match expected after omission replacement")
	})

	// Test case 3: Error when End is specified but no Begin
	t.Run("error when End without Begin", func(t *testing.T) {
		assert := assert.New(t)

		search := `
...
)
`
		modifyCmd := &heredoc.Command{
			Name:    "modify",
			Payload: testFile,
			Params: []heredoc.Param{
				{Name: "search", Payload: search},
				{Name: "replace", Payload: "anything"},
			},
		}

		action, err := CommandToAction(modifyCmd)
		assert.NoError(err, "Should build a Modify action from the command")

		// Verify should fail with error about End without Begin
		err = action.Verify()
		assert.Error(err, "Verify should fail when End is specified without Begin")
		assert.Contains(err.Error(), "search block has End but no Begin", "Error should mention End without Begin")
	})

	// Test case 4: Backward compatibility - simple search/replace without omission
	t.Run("simple search replace without omission", func(t *testing.T) {
		assert := assert.New(t)

		// Create a new test file for this case
		testFile3 := filepath.Join(tempDir, "omission3.txt")
		err := os.WriteFile(testFile3, []byte(originalContent), 0644)
		assert.NoError(err, "Failed to create third test file")

		search := "// bar is another function"
		replace := "// renamed_bar is another function"

		modifyCmd := &heredoc.Command{
			Name:    "modify",
			Payload: testFile3,
			Params: []heredoc.Param{
				{Name: "search", Payload: search},
				{Name: "replace", Payload: replace},
			},
		}

		action, err := CommandToAction(modifyCmd)
		assert.NoError(err, "Should build a Modify action from the command")

		// Verify should succeed
		err = action.Verify()
		assert.NoError(err, "Verify should succeed with simple search")

		// Apply the modification
		err = action.Apply()
		assert.NoError(err, "Apply should succeed")

		// Read the modified file
		modifiedContent, err := os.ReadFile(testFile3)
		assert.NoError(err, "Should be able to read modified file")

		// Check if content was modified correctly
		assert.Contains(string(modifiedContent), replace, "File should contain the replacement text")
		assert.NotContains(string(modifiedContent), search, "File should not contain the original search text")
	})
}
