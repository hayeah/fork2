package merge

import (
	"fmt"
	"strings"

	"github.com/hayeah/fork2/heredoc"
)

// Update CommandToAction in merge/command.go
func CommandToAction(cmd *heredoc.Command) (Action, error) {
	if cmd == nil {
		return nil, fmt.Errorf("command cannot be nil")
	}

	switch strings.ToLower(cmd.Name) {
	case "modify":
		return &Modify{Command: cmd}, nil
	case "rewrite":
		return &Rewrite{Command: cmd}, nil
	case "create":
		return &Create{Command: cmd}, nil
	case "delete":
		return &Delete{Command: cmd}, nil
	case "exec":
		return &Exec{Command: cmd}, nil
	case "edit":
		// Process the edit command based on the action parameter
		actionParam := cmd.GetParam("action")
		if actionParam == nil || actionParam.Payload == "" {
			return nil, fmt.Errorf("action parameter is required for edit command")
		}

		switch strings.ToLower(actionParam.Payload) {
		case "writeall":
			return &EditWriteAll{Command: cmd}, nil
		case "change":
			return &EditChange{Command: cmd}, nil
		case "insert":
			return &EditInsert{Command: cmd}, nil
		case "append":
			return &EditAppend{Command: cmd}, nil
		case "delete":
			return &EditDelete{Command: cmd}, nil
		default:
			return nil, fmt.Errorf("unsupported edit action: %s", actionParam.Payload)
		}
	default:
		return nil, fmt.Errorf("unsupported command: %s", cmd.Name)
	}
}
