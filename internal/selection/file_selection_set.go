package selection

import (
	"sort"
)

// FileSelectionSet is a set data structure that stores unique FileSelection objects by path
// and coalesces their ranges when adding new entries
type FileSelectionSet struct {
	items map[string]*FileSelection
}

// NewFileSelectionSet creates a new empty FileSelectionSet
func NewFileSelectionSet() *FileSelectionSet {
	return &FileSelectionSet{
		items: make(map[string]*FileSelection),
	}
}

// Add adds a FileSelection to the set, coalescing ranges if an entry with the same path already exists
func (s *FileSelectionSet) Add(selection FileSelection) {
	path := selection.Path

	// Check if we already have a selection for this path
	if existing, ok := s.items[path]; ok {
		// If either range is nil, consider it a full file selection
		if selection.Ranges == nil || existing.Ranges == nil {
			// Set it to nil to mean selecting the whole file
			existing.Ranges = nil
		} else {
			// Collect the ranges
			existing.Ranges = append(existing.Ranges, selection.Ranges...)
			// Coalesce the ranges
			existing.Ranges = coalesceRanges(existing.Ranges)
		}
	} else {
		// Create a copy of the selection to avoid modifying the original
		newSelection := FileSelection{
			Path:   selection.Path,
			Ranges: selection.Ranges,
			FS:     selection.FS,
		}

		// If there are ranges, coalesce them
		if len(newSelection.Ranges) > 0 {
			newSelection.Ranges = coalesceRanges(newSelection.Ranges)
		}

		// Add the new selection to the set
		s.items[path] = &newSelection
	}
}

// AddAll adds multiple FileSelection objects to the set
func (s *FileSelectionSet) AddAll(selections []FileSelection) {
	for _, selection := range selections {
		s.Add(selection)
	}
}

// Contains checks if the set contains a FileSelection with the given path
func (s *FileSelectionSet) Contains(path string) bool {
	_, exists := s.items[path]
	return exists
}

// Get returns the FileSelection for the given path, or nil if not found
func (s *FileSelectionSet) Get(path string) *FileSelection {
	if selection, ok := s.items[path]; ok {
		return selection
	}
	return nil
}

// Len returns the number of elements in the set
func (s *FileSelectionSet) Len() int {
	return len(s.items)
}

// Clear removes all elements from the set
func (s *FileSelectionSet) Clear() {
	s.items = make(map[string]*FileSelection)
}

// Values returns a slice containing all FileSelection objects in the set,
// sorted lexicographically by path
func (s *FileSelectionSet) Values() []FileSelection {
	result := make([]FileSelection, 0, len(s.items))

	// Get all paths and sort them
	paths := make([]string, 0, len(s.items))
	for path := range s.items {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Add selections to result in sorted order
	for _, path := range paths {
		result = append(result, *s.items[path])
	}

	return result
}
