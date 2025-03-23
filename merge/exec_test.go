package merge

import (
	"runtime"
	"testing"

	"github.com/hayeah/fork2/heredoc"
	"github.com/stretchr/testify/assert"
)

// mockPromptResponse controls the response for the promptForConfirmation function in tests
var mockPromptResponse bool

// Mock promptForConfirmation for testing
func mockPromptForConfirmation(_ string) bool {
	return mockPromptResponse
}

func TestExec(t *testing.T) {
	// Store original promptForConfirmation and restore after test
	originalPromptFn := promptForConfirmation
	defer func() { promptForConfirmation = originalPromptFn }()

	// Mock the prompt function for testing
	promptForConfirmation = mockPromptForConfirmation
	
	assert := assert.New(t)

	// Test with a simple echo command
	var execCommand string
	if runtime.GOOS == "windows" {
		execCommand = "cmd /c echo"
	} else {
		execCommand = "echo"
	}

	execCmd := &heredoc.Command{
		Name:    "exec",
		Payload: execCommand,
		Params: []heredoc.Param{
			{Name: "args", Payload: "Hello World"},
		},
	}

	action, err := CommandToAction(execCmd)
	assert.NoError(err, "Should build an Exec action from the command")
	execAction, ok := action.(*Exec)
	assert.True(ok, "Should be an Exec action")

	// Test Verify
	err = execAction.Verify()
	assert.NoError(err, "Verify should succeed with valid executable")

	// Test Apply - we won't execute it in tests to avoid side effects,
	// but we can check the command setup

	// Test with non-existent executable
	nonExistentCmd := &heredoc.Command{
		Name:    "exec",
		Payload: "nonexistentcmd",
	}
	nonExistentAction, err := CommandToAction(nonExistentCmd)
	assert.NoError(err, "Should build an Exec action from the command")

	err = nonExistentAction.Verify()
	assert.Error(err, "Verify should fail with non-existent executable")
	assert.Contains(err.Error(), "executable not found", "Error should mention executable not found")

	// Test with empty command
	emptyCmd := &heredoc.Command{
		Name:    "exec",
		Payload: "",
	}
	emptyAction, err := CommandToAction(emptyCmd)
	assert.NoError(err, "Should build an Exec action from the command")

	err = emptyAction.Verify()
	assert.Error(err, "Verify should fail with empty command")
	assert.Contains(err.Error(), "command cannot be empty", "Error should mention command cannot be empty")
}