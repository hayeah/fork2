package merge

import (
	"fmt"
	"strings"

	"github.com/hayeah/fork2/heredoc"
)

// CommandToAction converts a heredoc.Command to an Action with embedding.
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
	default:
		return nil, fmt.Errorf("unsupported command: %s", cmd.Name)
	}
}