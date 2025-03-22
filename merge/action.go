package merge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Action is an interface for operations that can be verified and applied
type Action interface {
	// Verify checks if the action can be performed
	Verify() error
	// Apply performs the action
	Apply() error
}

// Modify represents a search and replace action on a file
// It handles potentially malformed XML files by using string operations
// rather than XML parsing, which makes it more robust for code editing tasks.
type Modify struct {
	file    string // path to the file to modify
	search  string // text to search for
	replace string // text to replace with
	// We don't parse XML directly since the input might be malformed,
	// and we want to preserve exact formatting
}

// NewModify creates a new Modify action
// file: path to the file to modify
// search: text pattern to search for
// replace: text to replace the search pattern with
func NewModify(file, search, replace string) *Modify {
	return &Modify{
		file:    file,
		search:  search,
		replace: replace,
	}
}

// Verify checks if the file exists and contains the search string
func (m *Modify) Verify() error {
	// Check if file exists
	_, err := os.Stat(m.file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", m.file)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	// Check if search and replace strings are provided
	if m.search == "" {
		return errors.New("search string cannot be empty")
	}

	// Read file content
	content, err := os.ReadFile(m.file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Check if search string exists in file
	if !strings.Contains(string(content), m.search) {
		return errors.New("search string not found in file")
	}

	return nil
}

// Apply performs the search and replace operation on the file
func (m *Modify) Apply() error {
	// First verify that the operation can be performed
	if err := m.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Read file content
	content, err := os.ReadFile(m.file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Perform search and replace
	originalContent := string(content)
	newContent := strings.Replace(originalContent, m.search, m.replace, -1)
	
	// Check if any replacements were made
	if originalContent == newContent {
		return errors.New("no replacements were made")
	}

	// Write back to file
	err = os.WriteFile(m.file, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Rewrite represents a complete file rewrite action
type Rewrite struct {
	file    string // path to the file to rewrite
	content string // new content for the file
}

// NewRewrite creates a new Rewrite action
// file: path to the file to rewrite
// content: new content for the file
func NewRewrite(file, content string) *Rewrite {
	return &Rewrite{
		file:    file,
		content: content,
	}
}

// Verify checks if the file exists and can be rewritten
func (r *Rewrite) Verify() error {
	// Check if file exists
	_, err := os.Stat(r.file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", r.file)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	// Check if content is provided
	if r.content == "" {
		return errors.New("content cannot be empty")
	}

	return nil
}

// Apply performs the complete file rewrite
func (r *Rewrite) Apply() error {
	// First verify that the operation can be performed
	if err := r.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Write new content to file
	err := os.WriteFile(r.file, []byte(r.content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Create represents a file creation action
type Create struct {
	file    string // path to the file to create
	content string // content for the new file
}

// NewCreate creates a new Create action
// file: path to the file to create
// content: content for the new file
func NewCreate(file, content string) *Create {
	return &Create{
		file:    file,
		content: content,
	}
}

// Verify checks if the file can be created
func (c *Create) Verify() error {
	// Check if file already exists
	_, err := os.Stat(c.file)
	if err == nil {
		return fmt.Errorf("file already exists: %s", c.file)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check file existence: %w", err)
	}

	// Check if the directory exists
	dir := filepath.Dir(c.file)
	_, err = os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to create the directory
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		} else {
			return fmt.Errorf("failed to access directory %s: %w", dir, err)
		}
	}

	return nil
}

// Apply performs the file creation
func (c *Create) Apply() error {
	// First verify that the operation can be performed
	if err := c.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Write content to new file
	err := os.WriteFile(c.file, []byte(c.content), 0644)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	return nil
}

// Delete represents a file deletion action
type Delete struct {
	file string // path to the file to delete
}

// NewDelete creates a new Delete action
// file: path to the file to delete
func NewDelete(file string) *Delete {
	return &Delete{
		file: file,
	}
}

// Verify checks if the file exists and can be deleted
func (d *Delete) Verify() error {
	// Check if file exists
	_, err := os.Stat(d.file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", d.file)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	return nil
}

// Apply performs the file deletion
func (d *Delete) Apply() error {
	// First verify that the operation can be performed
	if err := d.Verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Delete the file
	err := os.Remove(d.file)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
