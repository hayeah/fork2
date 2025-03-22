package merge

import (
	"fmt"
	"strings"

	"github.com/hayeah/fork2/heredoc"
)

// CommandToAction converts a heredoc Command to the appropriate Action implementation
func CommandToAction(cmd *heredoc.Command) (Action, error) {
	if cmd == nil {
		return nil, fmt.Errorf("command cannot be nil")
	}

	// Extract command name without the leading ":"
	cmdName := strings.ToLower(cmd.Name)

	switch cmdName {
	case "modify":
		return createModifyAction(cmd)
	case "rewrite":
		return createRewriteAction(cmd)
	case "create":
		return createCreateAction(cmd)
	case "delete":
		return createDeleteAction(cmd)
	default:
		return nil, fmt.Errorf("unsupported command: %s", cmd.Name)
	}
}

// createModifyAction creates a Modify action from a command
func createModifyAction(cmd *heredoc.Command) (Action, error) {
	// Get required parameters
	file := cmd.Payload
	if file == "" {
		return nil, fmt.Errorf("file path is required for modify command")
	}

	// Get search parameter
	searchParam := cmd.GetParam("search")
	if searchParam == nil {
		return nil, fmt.Errorf("search parameter is required for modify command")
	}

	// Get replace parameter
	replaceParam := cmd.GetParam("replace")
	if replaceParam == nil {
		return nil, fmt.Errorf("replace parameter is required for modify command")
	}

	return NewModify(file, searchParam.Payload, replaceParam.Payload), nil
}

// createRewriteAction creates a Rewrite action from a command
func createRewriteAction(cmd *heredoc.Command) (Action, error) {
	// Get required parameters
	file := cmd.Payload
	if file == "" {
		return nil, fmt.Errorf("file path is required for rewrite command")
	}

	// Get content parameter
	contentParam := cmd.GetParam("content")
	if contentParam == nil {
		return nil, fmt.Errorf("content parameter is required for rewrite command")
	}

	return NewRewrite(file, contentParam.Payload), nil
}

// createCreateAction creates a Create action from a command
func createCreateAction(cmd *heredoc.Command) (Action, error) {
	// Get required parameters
	file := cmd.Payload
	if file == "" {
		return nil, fmt.Errorf("file path is required for create command")
	}

	// Get content parameter
	contentParam := cmd.GetParam("content")
	if contentParam == nil {
		return nil, fmt.Errorf("content parameter is required for create command")
	}

	return NewCreate(file, contentParam.Payload), nil
}

// createDeleteAction creates a Delete action from a command
func createDeleteAction(cmd *heredoc.Command) (Action, error) {
	// Get required parameters
	file := cmd.Payload
	if file == "" {
		return nil, fmt.Errorf("file path is required for delete command")
	}

	return NewDelete(file), nil
}
