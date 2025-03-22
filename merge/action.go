package merge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hayeah/fork2/heredoc"
)

// Action is an interface for operations that can be verified and applied.
type Action interface {
	Description() string
	Verify() error
	Apply() error
}

// Modify represents a search-and-replace action on a file.
type Modify struct {
	*heredoc.Command
}

func (m *Modify) Description() string {
	return m.Command.Description()
}

func (m *Modify) Verify() error {
	file := m.Payload
	if file == "" {
		return fmt.Errorf("file does not exist: empty file path in modify command")
	}

	searchParam := m.GetParam("search")
	if searchParam == nil || searchParam.Payload == "" {
		return errors.New("search string cannot be empty")
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
		return errors.New("search string not found in file")
	}

	return nil
}

func (m *Modify) Apply() error {
	if err := m.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	file := m.Payload
	search := m.GetParam("search").Payload

	replaceParam := m.GetParam("replace")
	if replaceParam == nil {
		return fmt.Errorf("replace parameter is required for modify command")
	}
	replace := replaceParam.Payload

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	newContent := strings.Replace(string(content), search, replace, -1)
	if newContent == string(content) {
		return errors.New("no replacements were made")
	}

	err = os.WriteFile(file, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Rewrite represents a complete file rewrite action.
type Rewrite struct {
	*heredoc.Command
}

func (r *Rewrite) Description() string {
	return r.Command.Description()
}

func (r *Rewrite) Verify() error {
	file := r.Payload
	if file == "" {
		return fmt.Errorf("file path is required for rewrite command")
	}

	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", file)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	contentParam := r.GetParam("content")
	if contentParam == nil || contentParam.Payload == "" {
		return errors.New("content cannot be empty")
	}

	return nil
}

func (r *Rewrite) Apply() error {
	if err := r.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	file := r.Payload
	newContent := r.GetParam("content").Payload

	err := os.WriteFile(file, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Create represents a file creation action.
type Create struct {
	*heredoc.Command
}

func (c *Create) Description() string {
	return c.Command.Description()
}

func (c *Create) Verify() error {
	file := c.Payload
	if file == "" {
		return fmt.Errorf("file path is required for create command")
	}

	_, err := os.Stat(file)
	if err == nil {
		return fmt.Errorf("file already exists: %s", file)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check file existence: %w", err)
	}

	dir := filepath.Dir(file)
	_, err = os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		} else {
			return fmt.Errorf("failed to access directory %s: %w", dir, err)
		}
	}

	return nil
}

func (c *Create) Apply() error {
	if err := c.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	contentParam := c.GetParam("content")
	if contentParam == nil {
		return fmt.Errorf("content parameter is required for create command")
	}

	file := c.Payload
	err := os.WriteFile(file, []byte(contentParam.Payload), 0644)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	return nil
}

// Delete represents a file deletion action.
type Delete struct {
	*heredoc.Command
}

func (d *Delete) Description() string {
	return d.Command.Description()
}

func (d *Delete) Verify() error {
	file := d.Payload
	if file == "" {
		return fmt.Errorf("file path is required for delete command")
	}

	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", file)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	return nil
}

func (d *Delete) Apply() error {
	if err := d.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	file := d.Payload
	err := os.Remove(file)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
