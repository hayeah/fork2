package merge

import (
	"testing"

	"github.com/hayeah/fork2/heredoc"
	"github.com/stretchr/testify/assert"
)

func TestCommandToAction(t *testing.T) {
	assert := assert.New(t)

	// Test nil command
	action, err := CommandToAction(nil)
	assert.Error(err, "Should error with nil command")
	assert.Nil(action, "Action should be nil with error")
	assert.Contains(err.Error(), "command cannot be nil", "Error should mention command cannot be nil")

	// Test unsupported command
	unsupportedCmd := &heredoc.Command{
		Name:    "unsupported",
		Payload: "file.txt",
	}
	action, err = CommandToAction(unsupportedCmd)
	assert.Error(err, "Should error with unsupported command")
	assert.Nil(action, "Action should be nil with error")
	assert.Contains(err.Error(), "unsupported command", "Error should mention unsupported command")

	// Test modify command
	modifyCmd := &heredoc.Command{
		Name:    "modify",
		Payload: "Models/User.swift",
		Params: []heredoc.Param{
			{
				Name:    "description",
				Payload: "Add email property to User struct.",
			},
			{
				Name: "search",
				Payload: `struct User {
  let id: UUID
  var name: String
}`,
			},
			{
				Name: "replace",
				Payload: `struct User {
    let id: UUID
    var name: String
    var email: String
}`,
			},
		},
	}
	action, err = CommandToAction(modifyCmd)
	assert.NoError(err, "Should not error with valid modify command")
	assert.NotNil(action, "Action should not be nil")
	modifyAction, ok := action.(*Modify)
	assert.True(ok, "Action should be a Modify action")
	assert.Equal("Models/User.swift", modifyAction.file, "File path should match")
	assert.Equal(modifyCmd.GetParam("search").Payload, modifyAction.search, "Search should match")
	assert.Equal(modifyCmd.GetParam("replace").Payload, modifyAction.replace, "Replace should match")

	// Test rewrite command
	rewriteCmd := &heredoc.Command{
		Name:    "rewrite",
		Payload: "Models/User.swift",
		Params: []heredoc.Param{
			{
				Name:    "description",
				Payload: "Full file rewrite with new email field",
			},
			{
				Name: "content",
				Payload: `import Foundation
struct User {
    let id: UUID
    var name: String
    var email: String

    init(name: String, email: String) {
        self.id = UUID()
        self.name = name
        self.email = email
    }
}`,
			},
		},
	}
	action, err = CommandToAction(rewriteCmd)
	assert.NoError(err, "Should not error with valid rewrite command")
	assert.NotNil(action, "Action should not be nil")
	rewriteAction, ok := action.(*Rewrite)
	assert.True(ok, "Action should be a Rewrite action")
	assert.Equal("Models/User.swift", rewriteAction.file, "File path should match")
	assert.Equal(rewriteCmd.GetParam("content").Payload, rewriteAction.content, "Content should match")

	// Test create command
	createCmd := &heredoc.Command{
		Name:    "create",
		Payload: "Views/RoundedButton.swift",
		Params: []heredoc.Param{
			{
				Name:    "description",
				Payload: "Create custom RoundedButton class",
			},
			{
				Name: "content",
				Payload: `import UIKit
@IBDesignable
class RoundedButton: UIButton {
    @IBInspectable var cornerRadius: CGFloat = 0
}`,
			},
		},
	}
	action, err = CommandToAction(createCmd)
	assert.NoError(err, "Should not error with valid create command")
	assert.NotNil(action, "Action should not be nil")
	createAction, ok := action.(*Create)
	assert.True(ok, "Action should be a Create action")
	assert.Equal("Views/RoundedButton.swift", createAction.file, "File path should match")
	assert.Equal(createCmd.GetParam("content").Payload, createAction.content, "Content should match")

	// Test delete command
	deleteCmd := &heredoc.Command{
		Name:    "delete",
		Payload: "Obsolete/File.swift",
		Params: []heredoc.Param{
			{
				Name:    "description",
				Payload: "Completely remove the file from the project",
			},
		},
	}
	action, err = CommandToAction(deleteCmd)
	assert.NoError(err, "Should not error with valid delete command")
	assert.NotNil(action, "Action should not be nil")
	deleteAction, ok := action.(*Delete)
	assert.True(ok, "Action should be a Delete action")
	assert.Equal("Obsolete/File.swift", deleteAction.file, "File path should match")

	// Test missing parameters
	// Modify without search
	modifyNoSearchCmd := &heredoc.Command{
		Name:    "modify",
		Payload: "Models/User.swift",
		Params: []heredoc.Param{
			{
				Name: "replace",
				Payload: `struct User {
    let id: UUID
    var name: String
    var email: String
}`,
			},
		},
	}
	action, err = CommandToAction(modifyNoSearchCmd)
	assert.Error(err, "Should error with missing search parameter")
	assert.Nil(action, "Action should be nil with error")
	assert.Contains(err.Error(), "search parameter is required", "Error should mention search parameter is required")

	// Create without content
	createNoContentCmd := &heredoc.Command{
		Name:    "create",
		Payload: "Views/RoundedButton.swift",
		Params: []heredoc.Param{
			{
				Name:    "description",
				Payload: "Create custom RoundedButton class",
			},
		},
	}
	action, err = CommandToAction(createNoContentCmd)
	assert.Error(err, "Should error with missing content parameter")
	assert.Nil(action, "Action should be nil with error")
	assert.Contains(err.Error(), "content parameter is required", "Error should mention content parameter is required")

	// Missing file path
	noFileCmd := &heredoc.Command{
		Name: "delete",
		Params: []heredoc.Param{
			{
				Name:    "description",
				Payload: "Completely remove the file from the project",
			},
		},
	}
	action, err = CommandToAction(noFileCmd)
	assert.Error(err, "Should error with missing file path")
	assert.Nil(action, "Action should be nil with error")
	assert.Contains(err.Error(), "file path is required", "Error should mention file path is required")
}
