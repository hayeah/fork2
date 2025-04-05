package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
)

// FrontMatter represents the front matter section of an instruction
type FrontMatter struct {
	Content string
	Tag     string
}

// parseFrontMatter inspects the first line of data and checks if it starts with
// "---" or "+++". If so, we extract the tag as the substring *after* that delimiter,
// gather lines until we reach the corresponding delimiter (e.g. "---" or "+++"), and
// return (tag, frontMatter, remainder, error).
func parseFrontMatter(data string) (string, string, string, error) {
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
	var frontMatterLines []byte
	foundClose := false

	i := 1
	for ; i < len(lines); i++ {
		if string(lines[i]) == delimiter {
			foundClose = true
			break
		}
		frontMatterLines = append(frontMatterLines, lines[i]...)
		frontMatterLines = append(frontMatterLines, '\n')
	}

	if !foundClose {
		return "", "", "", fmt.Errorf(
			"front matter not closed; expected closing delimiter %q", delimiter,
		)
	}

	// Remainder is everything after the line with the closing delimiter
	remainderLines := lines[i+1:]
	remainder := strings.Join(remainderLines, "\n")

	return tag, string(frontMatterLines), string(remainder), nil
}

// parseFlags interprets lines of text as flags for AskCmd using go-arg library
// by temporarily overriding os.Args.
func parseFlags(frontMatter []byte) (*AskCmd, error) {
	cmd := &AskCmd{}

	// Convert front matter to a single line of args
	var args []string
	allLines := bytes.Split(frontMatter, []byte("\n"))
	for _, line := range allLines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// A line might have multiple flags: e.g. "--copy --diff"
		parts := strings.Fields(string(line))
		args = append(args, parts...)
	}

	// Skip parsing if no args
	if len(args) == 0 {
		return cmd, nil
	}

	// Temporarily save original args
	originalArgs := os.Args
	defer func() {
		// Restore original args
		os.Args = originalArgs
	}()

	// Override with our custom args (preserving program name as first arg)
	os.Args = append([]string{originalArgs[0]}, args...)

	// Use go-arg to parse the flags into our struct
	parser, err := arg.NewParser(arg.Config{}, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %v", err)
	}

	if err := parser.Parse(os.Args); err != nil {
		return nil, fmt.Errorf("failed to parse front matter flags: %v", err)
	}

	return cmd, nil
}

// parseInstructionWithFrontMatter parses the instruction from a file or string,
// extracts any front matter, and returns the remaining instruction content along with the parsed front matter.
func parseInstructionWithFrontMatter(runner *AskRunner) (string, error) {
	cmdArgs := &runner.Args

	if cmdArgs.Instruction == "" {
		return "", nil
	}

	// Read the content (from file or raw string)
	instructionContent, err := readInstructionContent(cmdArgs.Instruction)
	if err != nil {
		return "", err
	}

	// Split into lines, skip leading blanks
	lines := bytes.Split(instructionContent, []byte("\n"))
	idx := 0
	for ; idx < len(lines); idx++ {
		if len(bytes.TrimSpace(lines[idx])) > 0 {
			break
		}
	}
	if idx >= len(lines) {
		// All blank
		return "", nil
	}

	firstNonEmpty := bytes.TrimSpace(lines[idx])
	// If it starts with --, or exactly '---' or '+++', parse front matter flags
	if bytes.HasPrefix(firstNonEmpty, []byte("---")) ||
		bytes.HasPrefix(firstNonEmpty, []byte("+++")) {

		tag, frontMatter, remainder, err := parseFrontMatter(string(instructionContent))
		if err != nil {
			return "", err
		}

		// Store the parsed front matter but don't process it here
		runner.ParsedFrontMatter = &FrontMatter{
			Content: frontMatter,
			Tag:     tag,
		}

		return string(remainder), nil
	}

	return string(instructionContent), nil
}

// readInstructionContent reads the instruction content from a file if the path exists,
// otherwise returns the instruction string as-is
func readInstructionContent(instruction string) ([]byte, error) {
	// Check if the instruction is a file path
	fileInfo, err := os.Stat(instruction)
	if err == nil && !fileInfo.IsDir() {
		// It's a file, read its content
		content, err := os.ReadFile(instruction)
		if err != nil {
			return nil, fmt.Errorf("failed to read instruction file: %v", err)
		}
		return content, nil
	}

	// It's not a file, return the instruction string itself
	return []byte(instruction), nil
}
