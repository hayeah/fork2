package merge

import (
	"fmt"
	"os"
	"strings"

	"github.com/hayeah/fork2/heredoc"
)

// Edit represents the base edit action.
type Edit struct {
	*heredoc.Command
}

// Description returns the action description.
func (e *Edit) Description() string {
	descParam := e.GetParam("description")
	if descParam != nil {
		return descParam.Payload
	}
	return fmt.Sprintf("Edit file: %s", e.Payload)
}

// EditWriteAll represents a complete file rewrite action.
type EditWriteAll struct {
	*heredoc.Command
}

func (e *EditWriteAll) Verify() error {
	file := e.Payload
	if file == "" {
		return fmt.Errorf("file path is required for edit writeAll command")
	}

	contentParam := e.GetParam("content")
	if contentParam == nil || contentParam.Payload == "" {
		return fmt.Errorf("content parameter is required for writeAll action")
	}

	return nil
}

func (e *EditWriteAll) Apply() error {
	if err := e.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	file := e.Payload
	contentParam := e.GetParam("content")
	if contentParam == nil {
		return fmt.Errorf("content parameter is required for writeAll action")
	}

	// Create directories if they don't exist
	dir := strings.TrimSuffix(file, "/"+strings.TrimLeft(file, "/"))
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory structure: %w", err)
		}
	}

	if err := os.WriteFile(file, []byte(contentParam.Payload), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// EditChange represents a search-and-replace action on a file.
type EditChange struct {
	*heredoc.Command
}

func (e *EditChange) Verify() error {
	file := e.Payload
	if file == "" {
		return fmt.Errorf("file path is required for edit change command")
	}

	searchParam := e.GetParam("search")
	if searchParam == nil || searchParam.Payload == "" {
		return fmt.Errorf("search parameter is required for change action")
	}

	contentParam := e.GetParam("content")
	if contentParam == nil || contentParam.Payload == "" {
		return fmt.Errorf("content parameter is required for change action")
	}

	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", file)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if !strings.Contains(string(content), searchParam.Payload) {
		return fmt.Errorf("search string not found in file")
	}

	return nil
}

func (e *EditChange) Apply() error {
	if err := e.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	file := e.Payload
	searchParam := e.GetParam("search")
	contentParam := e.GetParam("content")

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	newContent := strings.Replace(string(content), searchParam.Payload, contentParam.Payload, 1)
	if newContent == string(content) {
		return fmt.Errorf("no changes were made")
	}

	if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// EditInsert represents an action to insert content before a matched block.
type EditInsert struct {
	*heredoc.Command
}

func (e *EditInsert) Verify() error {
	file := e.Payload
	if file == "" {
		return fmt.Errorf("file path is required for edit insert command")
	}

	searchParam := e.GetParam("search")
	if searchParam == nil || searchParam.Payload == "" {
		return fmt.Errorf("search parameter is required for insert action")
	}

	contentParam := e.GetParam("content")
	if contentParam == nil || contentParam.Payload == "" {
		return fmt.Errorf("content parameter is required for insert action")
	}

	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", file)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if !strings.Contains(string(content), searchParam.Payload) {
		return fmt.Errorf("search string not found in file")
	}

	return nil
}

func (e *EditInsert) Apply() error {
	if err := e.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	file := e.Payload
	searchParam := e.GetParam("search")
	contentParam := e.GetParam("content")

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	strContent := string(content)
	searchStr := searchParam.Payload
	index := strings.Index(strContent, searchStr)

	if index == -1 {
		return fmt.Errorf("search string not found in file")
	}

	// Insert content before the search match
	newContent := strContent[:index] + contentParam.Payload + strContent[index:]

	if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// EditAppend represents an action to append content after a matched block.
type EditAppend struct {
	*heredoc.Command
}

func (e *EditAppend) Verify() error {
	file := e.Payload
	if file == "" {
		return fmt.Errorf("file path is required for edit append command")
	}

	searchParam := e.GetParam("search")
	if searchParam == nil || searchParam.Payload == "" {
		return fmt.Errorf("search parameter is required for append action")
	}

	contentParam := e.GetParam("content")
	if contentParam == nil || contentParam.Payload == "" {
		return fmt.Errorf("content parameter is required for append action")
	}

	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", file)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if !strings.Contains(string(content), searchParam.Payload) {
		return fmt.Errorf("search string not found in file")
	}

	return nil
}

func (e *EditAppend) Apply() error {
	if err := e.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	file := e.Payload
	searchParam := e.GetParam("search")
	contentParam := e.GetParam("content")

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	strContent := string(content)
	searchStr := searchParam.Payload
	index := strings.Index(strContent, searchStr)

	if index == -1 {
		return fmt.Errorf("search string not found in file")
	}

	// Append content after the search match
	endIndex := index + len(searchStr)
	newContent := strContent[:endIndex] + contentParam.Payload + strContent[endIndex:]

	if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// EditDelete represents an action to delete a matched block of content.
type EditDelete struct {
	*heredoc.Command
}

func (e *EditDelete) Verify() error {
	file := e.Payload
	if file == "" {
		return fmt.Errorf("file path is required for edit delete command")
	}

	searchParam := e.GetParam("search")
	if searchParam == nil || searchParam.Payload == "" {
		return fmt.Errorf("search parameter is required for delete action")
	}

	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", file)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if !strings.Contains(string(content), searchParam.Payload) {
		return fmt.Errorf("search string not found in file")
	}

	return nil
}

func (e *EditDelete) Apply() error {
	if err := e.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	file := e.Payload
	searchParam := e.GetParam("search")

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	strContent := string(content)
	searchStr := searchParam.Payload
	index := strings.Index(strContent, searchStr)

	if index == -1 {
		return fmt.Errorf("search string not found in file")
	}

	// Delete the matched block
	newContent := strContent[:index] + strContent[index+len(searchStr):]

	if err := os.WriteFile(file, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
