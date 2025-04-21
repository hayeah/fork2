package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// RawFrontMatter represents the raw front matter content and tag
type RawFrontMatter struct {
	Content string
	Tag     string
}

// Instruct represents a parsed user instruction
type Instruct struct {
	FrontMatter *RawFrontMatter
	UserContent string
	Header      *InstructHeader
}

// InstructParser handles parsing of user instructions
type InstructParser struct{}

// NewInstructParser creates a new instruction parser
func NewInstructParser() *InstructParser {
	return &InstructParser{}
}

// Parse parses a user instruction string and returns an Instruct object
func (p *InstructParser) Parse(input string) (*Instruct, error) {
	// First check if input is a file path
	content, err := p.readInstructionContent(input)
	if err != nil {
		return nil, err
	}

	// Parse front matter if present
	tag, frontMatterContent, remainder, err := p.parseFrontMatter(content)
	if err != nil {
		return nil, err
	}

	instruct := &Instruct{
		UserContent: remainder,
		FrontMatter: &RawFrontMatter{
			Content: frontMatterContent,
			Tag:     tag,
		},
	}

	// Parse TOML into header if front matter was found
	if frontMatterContent != "" {
		header, err := p.parseTomlHeader(frontMatterContent)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TOML header: %w", err)
		}
		instruct.Header = header
	}

	return instruct, nil
}

// parseFrontMatter extracts front matter from the instruction content
func (p *InstructParser) parseFrontMatter(data string) (string, string, string, error) {
	lines := strings.Split(data, "\n")
	if len(lines) == 0 {
		// No data => nothing to parse.
		return "", "", data, nil
	}

	// Check if the first line begins with "---", "+++", or "```"
	firstLine := string(lines[0])
	var delimiter, tag string

	switch {
	case strings.HasPrefix(firstLine, "---"):
		delimiter = "---"
		tag = strings.TrimPrefix(firstLine, "---")
	case strings.HasPrefix(firstLine, "+++"):
		delimiter = "+++"
		tag = strings.TrimPrefix(firstLine, "+++")
	case strings.HasPrefix(firstLine, "```"):
		delimiter = "```"
		tag = strings.TrimPrefix(firstLine, "```")
	default:
		// Not front matter at all; just return everything as remainder
		return "", "", string(data), nil
	}

	tag = strings.TrimSpace(tag)

	// Now find the matching closing delimiter line.
	var frontMatterLines []string
	foundClose := false

	i := 1
	for ; i < len(lines); i++ {
		if string(lines[i]) == delimiter {
			foundClose = true
			break
		}
		frontMatterLines = append(frontMatterLines, lines[i])
	}

	if !foundClose {
		return "", "", "", fmt.Errorf(
			"front matter not closed; expected closing delimiter %q", delimiter,
		)
	}

	// Remainder is everything after the line with the closing delimiter
	remainderLines := lines[i+1:]
	remainder := strings.Join(remainderLines, "\n")

	return tag, strings.Join(frontMatterLines, "\n"), remainder, nil
}

// readInstructionContent reads the instruction content from a file if the path exists,
// otherwise returns the instruction string as-is
func (p *InstructParser) readInstructionContent(instruction string) (string, error) {
	// Check if the instruction is a file path
	fileInfo, err := os.Stat(instruction)
	if err == nil && !fileInfo.IsDir() {
		// It's a file, read its content
		content, err := os.ReadFile(instruction)
		if err != nil {
			return "", fmt.Errorf("failed to read instruction file: %v", err)
		}
		return string(content), nil
	}

	// It's not a file, return the instruction string itself
	return instruction, nil
}

// parseTomlHeader parses the TOML content into an InstructHeader
func (p *InstructParser) parseTomlHeader(content string) (*InstructHeader, error) {
	var header InstructHeader
	decoder := toml.NewDecoder(strings.NewReader(content))
	if _, err := decoder.Decode(&header); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	return &header, nil
}

// InstructHeader represents the parsed TOML fields
type InstructHeader struct {
	Layout string          `toml:"layout"`
	Select string          `toml:"select"`
	Files  []FileSelection `toml:"file"`
}

// FileSelectionsWithDirTree extracts file selections from the header and also processes
// the Select string if present, using the provided directory tree to match paths
func (h *InstructHeader) FileSelectionsWithDirTree(dirTree *DirectoryTree) ([]FileSelection, error) {
	return selectFiles(h.Select, dirTree)
}

func selectFiles(selectString string, dirTree *DirectoryTree) ([]FileSelection, error) {
	// Create a new FileSelectionSet to store and coalesce selections
	set := NewFileSelectionSet()

	// Process Select string if present
	if selectString != "" {
		// Parse select string into matchers
		matchers, err := ParseMatchersFromString(selectString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse select string: %w", err)
		}

		// Get all file paths from directory tree
		allPaths := dirTree.SelectAllFiles()

		// Apply each matcher to get matching paths
		for _, matcher := range matchers {
			matches, err := matcher.Match(allPaths)
			if err != nil {
				return nil, fmt.Errorf("matcher error: %w", err)
			}

			// Convert matched paths to FileSelections
			for _, path := range matches {
				// For ExactPathMatcher, we need to preserve any line ranges
				if exactMatcher, ok := matcher.(ExactPathMatcher); ok {
					// Check if the path is the same as the exact matcher's path
					if path == exactMatcher.Path {
						// This is a direct match to the specified path
						// Add to the set with the matcher's ranges
						set.Add(FileSelection{
							Path:   path,
							Ranges: exactMatcher.Ranges,
						})
					} else {
						// This is a match from a directory pattern (=dir)
						// Add to the set without any line ranges
						set.Add(FileSelection{
							Path:   path,
							Ranges: nil, // nil means select the whole file
						})
					}
				} else {
					// For other matcher types, just select the whole file
					set.Add(FileSelection{
						Path:   path,
						Ranges: nil, // nil means select the whole file
					})
				}
			}
		}
	}

	// Return the values from the set (already sorted and with coalesced ranges)
	return set.Values(), nil
}
