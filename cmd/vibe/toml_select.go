package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// LineRange represents a range of lines in a file
type LineRange struct {
	Start int
	End   int
}

// FileSelection represents a file and its selected line ranges
type FileSelection struct {
	Path   string      // Absolute file path
	Ranges []LineRange // Line ranges to include, empty means all lines
}

// ReadString reads selected line ranges from the file.
// If Ranges is empty, it returns the entire file content.
func (fs *FileSelection) ReadString() (string, error) {
	return extractSelectedLines(fs.Path, fs.Ranges)
}

// TomlSelect represents a file selection in TOML
// Used by InstructParser in instruct_parser.go
type TomlSelect struct {
	Path string `toml:"path"` // File path with optional line range
}

// TomlHeader represents the TOML file header
// Used by InstructParser in instruct_parser.go
type TomlHeader struct {
	Files []TomlSelect `toml:"file"`
}

// parseLineRange parses a line range string like "1,5" or "10,15"
func parseLineRange(rangeStr string) (LineRange, error) {
	parts := strings.Split(rangeStr, ",")
	if len(parts) != 2 {
		return LineRange{}, fmt.Errorf("invalid line range format: %s, expected start,end", rangeStr)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return LineRange{}, fmt.Errorf("invalid start line number: %s", parts[0])
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return LineRange{}, fmt.Errorf("invalid end line number: %s", parts[1])
	}

	if start < 1 {
		return LineRange{}, fmt.Errorf("start line must be >= 1, got %d", start)
	}

	if end < start {
		return LineRange{}, fmt.Errorf("end line (%d) must be >= start line (%d)", end, start)
	}

	return LineRange{Start: start, End: end}, nil
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

// extractSelectedLines reads selected line ranges from a file.
// It assumes that the provided sortedRanges are already sorted and merged.
// If sortedRanges is empty, it returns the entire file content.
// Returns an error if the ranges are not sorted or merged.
func extractSelectedLines(filePath string, sortedRanges []LineRange) (string, error) {
	// Open the file.
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// If no ranges specified, return the entire file content.
	if len(sortedRanges) == 0 {
		content, err := io.ReadAll(file)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
		return string(content), nil
	}

	// Validate that sortedRanges is sorted and merged.
	// For sorted order, each range's Start must be less than or equal to the next's.
	// For merged ranges, the current range's End+1 must be strictly less than the next's Start.
	for i := 0; i < len(sortedRanges)-1; i++ {
		if sortedRanges[i].Start > sortedRanges[i+1].Start {
			return "", fmt.Errorf("ranges not sorted: range at index %d has start %d greater than range at index %d start %d", i, sortedRanges[i].Start, i+1, sortedRanges[i+1].Start)
		}
		if sortedRanges[i].End+1 >= sortedRanges[i+1].Start {
			return "", fmt.Errorf("ranges not merged: range at index %d and index %d are overlapping or contiguous", i, i+1)
		}
	}

	// Scan the file once.
	scanner := bufio.NewScanner(file)
	lineNum := 1
	rangeIdx := 0
	var result strings.Builder

	lastSeenRange := -1

	for scanner.Scan() {
		// If we've processed all ranges, break early.
		if rangeIdx >= len(sortedRanges) {
			break
		}

		// Advance the range pointer if the current line number is beyond the current range.
		for rangeIdx < len(sortedRanges) && lineNum > sortedRanges[rangeIdx].End {
			rangeIdx++
		}

		if rangeIdx >= len(sortedRanges) {
			break
		}

		// Write the line if it falls within the current range.
		if lineNum >= sortedRanges[rangeIdx].Start && lineNum <= sortedRanges[rangeIdx].End {
			if rangeIdx > lastSeenRange {
				lastSeenRange = rangeIdx
				fmt.Fprintf(&result, "\n--- %s#%d,%d ---\n", filePath, sortedRanges[rangeIdx].Start, sortedRanges[rangeIdx].End)
			}

			result.WriteString(scanner.Text())
			result.WriteString("\n")
		}

		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	return result.String(), nil
}
