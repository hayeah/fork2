package selection

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// LineRange represents a range of lines in a file
type LineRange struct {
	Start int
	End   int
}

// FileSelectionContent represents the content of a file selection
type FileSelectionContent struct {
	Path    string     // File path
	Content string     // Content of the selection
	Range   *LineRange // Line range, nil means the whole file content
}

// The pattern matches: <filepath>#<start>,<end> where start and end are integers
var reFileSelection = regexp.MustCompile(`^(.+)#(\d+),(\d+)$`)

// ParseFileSelection parses a path string that may contain a line range specification
// Format: path#start,end where start and end are line numbers
// Returns a FileSelection with the path and any line ranges found
func ParseFileSelection(path string) (FileSelection, error) {
	// If there's no hash character, just return the path as is
	if !strings.Contains(path, "#") {
		return FileSelection{Path: path}, nil
	}

	// Use a regular expression to validate and parse the path format
	matches := reFileSelection.FindStringSubmatch(path)

	// If the pattern doesn't match, return an error
	if matches == nil {
		return FileSelection{}, fmt.Errorf("invalid file path format: must be in format path#start,end")
	}

	// Extract the file path and line numbers from the regex matches
	filePath := matches[1]
	startLine, err := strconv.Atoi(matches[2])
	if err != nil {
		return FileSelection{}, fmt.Errorf("invalid start line number in range: %v", err)
	}

	endLine, err := strconv.Atoi(matches[3])
	if err != nil {
		return FileSelection{}, fmt.Errorf("invalid end line number in range: %v", err)
	}

	// Create a FileSelection with the path and line range
	return FileSelection{
		Path: filePath,
		Ranges: []LineRange{{
			Start: startLine,
			End:   endLine,
		}},
	}, nil
}

// FileSelection represents a file and its selected line ranges
type FileSelection struct {
	Path   string      // File path
	Ranges []LineRange // Line ranges to include, empty means all lines
}

// ReadString reads selected line ranges from the file.
// If Ranges is empty, it returns the entire file content.
func (fs *FileSelection) ReadString() (string, error) {
	contents, err := fs.Contents()
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for _, content := range contents {
		if content.Range == nil {
			fmt.Fprintf(&result, "\n<!-- Read File: %s -->\n", content.Path)
		} else {
			fmt.Fprintf(&result, "\n<!-- Read File: %s#%d,%d -->\n", content.Path, content.Range.Start, content.Range.End)
		}

		result.WriteString(content.Content)
	}

	return result.String(), nil
}

// Contents reads selected line ranges from the file and returns a slice of FileSelectionContent.
// If Ranges is empty, it returns the entire file content as a single FileSelectionContent with Range set to nil.
func (fs *FileSelection) Contents() ([]FileSelectionContent, error) {
	// Sort and merge ranges if needed
	sortedRanges := coalesceRanges(fs.Ranges)

	// Extract the content for each range
	return fs.extractContents(sortedRanges)
}

// extractContents reads selected line ranges from a file.
// It assumes that the provided sortedRanges are already sorted and merged.
// If sortedRanges is empty, it returns the entire file content.
// Assumes that the ranges are already sorted and merged.
func (fs *FileSelection) extractContents(sortedRanges []LineRange) ([]FileSelectionContent, error) {
	// Open the file
	file, err := os.Open(fs.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", fs.Path, err)
	}
	defer file.Close()

	// Read the entire file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", fs.Path, err)
	}

	// Check if it's a binary file first (early return)
	if isBinaryFile(content) {
		return []FileSelectionContent{{
			Path:    fs.Path,
			Content: "[binary file omitted]",
			Range:   nil,
		}}, nil
	}

	// If no ranges specified, return the entire file content (early return)
	if len(sortedRanges) == 0 {
		return []FileSelectionContent{{
			Path:    fs.Path,
			Content: string(content),
			Range:   nil,
		}}, nil
	}

	// Split content into lines (using strings for simplicity)
	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	// Build final slice of FileSelectionContent
	results := make([]FileSelectionContent, len(sortedRanges))
	for i, rng := range sortedRanges {
		var builder strings.Builder

		// Adjust range to be within bounds
		// Convert 1-based line numbers to 0-based array indices
		start := rng.Start - 1
		end := rng.End
		if start < 0 {
			start = 0
		}
		if end > len(lines) {
			end = len(lines)
		}

		// Extract lines for this range
		for lineIdx := start; lineIdx < end; lineIdx++ {
			if lineIdx < len(lines) {
				builder.WriteString(lines[lineIdx])
				builder.WriteString("\n")
			}
		}

		rangeCopy := rng
		results[i] = FileSelectionContent{
			Path:    fs.Path,
			Range:   &rangeCopy,
			Content: builder.String(),
		}
	}

	return results, nil
}

// coalesceRanges merges overlapping line ranges
func coalesceRanges(ranges []LineRange) []LineRange {
	if len(ranges) <= 1 {
		return ranges
	}

	// Sort ranges by start line
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Start < ranges[j].Start
	})

	// Merge overlapping ranges
	result := []LineRange{ranges[0]}
	for i := 1; i < len(ranges); i++ {
		current := ranges[i]
		last := &result[len(result)-1]

		// If current range overlaps or is adjacent to the last range
		if current.Start <= last.End+1 {
			// Extend the last range if needed
			if current.End > last.End {
				last.End = current.End
			}
		} else {
			// Add as a new range
			result = append(result, current)
		}
	}

	return result
}
