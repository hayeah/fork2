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

	// Check if the first line begins with "---" or "+++"
	firstLine := string(lines[0])
	var delimiter, tag string

	switch {
	case strings.HasPrefix(firstLine, "---"):
		delimiter = "---"
		tag = strings.TrimPrefix(firstLine, "---")
	case strings.HasPrefix(firstLine, "+++"):
		delimiter = "+++"
		tag = strings.TrimPrefix(firstLine, "+++")
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
	Files []FileSelection `toml:"file"`
}

// FileSelections extracts file selections from the header
func (h *InstructHeader) FileSelections() ([]FileSelection, error) {
	// Map to store FileSelections by path for easy lookup
	selectionsMap := make(map[string]*FileSelection)

	// Process each select entry
	for _, select_ := range h.Files {
		fileSelection, err := ParseFileSelection(select_.Path)
		if err != nil {
			return nil, err
		}

		// Check if we already have a selection for this file
		if existing, ok := selectionsMap[fileSelection.Path]; ok {
			// if either range is nil, consider it a full file selection
			if fileSelection.Ranges == nil || existing.Ranges == nil {
				// set it to nil to mean selecting the whole file
				existing.Ranges = nil
			}

			// collect the range
			existing.Ranges = append(existing.Ranges, fileSelection.Ranges...)
		} else {
			// Create new entry
			selectionsMap[fileSelection.Path] = &fileSelection
		}
	}

	// Convert map to slice
	var fileSelections []FileSelection
	for _, selection := range selectionsMap {
		// Coalesce overlapping ranges if any
		if len(selection.Ranges) > 0 {
			selection.Ranges = coalesceRanges(selection.Ranges)
		}
		fileSelections = append(fileSelections, *selection)
	}

	return fileSelections, nil
}
