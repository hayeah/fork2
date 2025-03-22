// Package heredoc provides a parser for the heredoc protocol.
package heredoc

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Commands represents a slice of Command structures.
type Commands []Command

// Command represents a single command in the heredoc protocol.
type Command struct {
	LineNo  int    // Line number where the command starts
	Name    string // Command name (cannot be empty)
	Payload string // Command payload (can be empty)
	Params  []Param
}

// GetParam retrieves a parameter by name, returning nil if not found.
func (c *Command) GetParam(name string) *Param {
	for i := range c.Params {
		if c.Params[i].Name == name {
			return &c.Params[i]
		}
	}
	return nil
}

// Param represents a parameter associated with a command.
type Param struct {
	LineNo  int    // Line number where the parameter starts
	Name    string // Parameter name (cannot be empty)
	Payload string // Parameter payload (can be empty)
}

// ParseError represents an error that occurred during parsing.
type ParseError struct {
	LineNo  int
	Message string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parse error at line %d: %s", e.LineNo, e.Message)
}

// ParseReader parses the heredoc protocol from an io.Reader and returns the commands.
func ParseReader(r io.Reader) (Commands, error) {
	scanner := bufio.NewScanner(r)
	var commands Commands
	var currentLine string
	lineNo := 0

	for scanner.Scan() || currentLine != "" {
		var line string
		if currentLine != "" {
			line = currentLine
			currentLine = ""
		} else {
			lineNo++
			line = scanner.Text()
		}

		// Skip empty lines
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Parse command
		if strings.HasPrefix(line, ":") {
			cmd, nextLine, newLineNo, err := parseCommand(scanner, line, lineNo)
			if err != nil {
				return nil, err
			}
			commands = append(commands, cmd)
			lineNo = newLineNo

			if nextLine != "" {
				currentLine = nextLine
			}
			continue
		}

		// If we reach here, we have an invalid line
		return nil, &ParseError{LineNo: lineNo, Message: "unexpected line: " + line}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return commands, nil
}

// parseCommand parses a command block, including its payload and parameters.
func parseCommand(scanner *bufio.Scanner, line string, startLineNo int) (Command, string, int, error) {
	// Extract command name and payload
	line = strings.TrimPrefix(line, ":")
	cmdName, payload, hasHeredoc, err := parseNameAndPayload(line)
	if err != nil {
		return Command{}, "", startLineNo, &ParseError{LineNo: startLineNo, Message: err.Error()}
	}

	if cmdName == "" {
		return Command{}, "", startLineNo, &ParseError{LineNo: startLineNo, Message: "command name cannot be empty"}
	}

	cmd := Command{
		LineNo:  startLineNo,
		Name:    cmdName,
		Payload: payload,
		Params:  []Param{},
	}

	// Parse heredoc payload if present
	currentLineNo := startLineNo
	if hasHeredoc {
		heredocContent, newLineNo, err := parseHeredoc(scanner, startLineNo+1)
		if err != nil {
			return Command{}, "", currentLineNo, err
		}
		cmd.Payload = heredocContent
		currentLineNo = newLineNo
	}

	// Parse parameters
	for scanner.Scan() {
		currentLineNo++
		paramLine := scanner.Text()

		// Skip empty lines
		if len(strings.TrimSpace(paramLine)) == 0 {
			continue
		}

		// Skip comments
		if strings.HasPrefix(paramLine, "#") {
			continue
		}

		// If line starts with "$", it's a parameter
		if strings.HasPrefix(paramLine, "$") {
			param, newLineNo, err := parseParam(scanner, paramLine, currentLineNo)
			if err != nil {
				return Command{}, "", currentLineNo, err
			}
			cmd.Params = append(cmd.Params, param)
			currentLineNo = newLineNo
			continue
		}

		// If line starts with ":", it's a new command, so we're done with this one
		if strings.HasPrefix(paramLine, ":") {
			// Return the current command and the new command line
			return cmd, paramLine, currentLineNo, nil
		}

		// If we get here, we have an invalid line
		return Command{}, "", currentLineNo, &ParseError{LineNo: currentLineNo, Message: "unexpected line in command block: " + paramLine}
	}

	return cmd, "", currentLineNo, nil
}

// parseParam parses a parameter block, including its payload.
func parseParam(scanner *bufio.Scanner, line string, startLineNo int) (Param, int, error) {
	// Extract parameter name and payload
	line = strings.TrimPrefix(line, "$")
	paramName, payload, hasHeredoc, err := parseNameAndPayload(line)
	if err != nil {
		return Param{}, startLineNo, &ParseError{LineNo: startLineNo, Message: err.Error()}
	}

	if paramName == "" {
		return Param{}, startLineNo, &ParseError{LineNo: startLineNo, Message: "parameter name cannot be empty"}
	}

	param := Param{
		LineNo:  startLineNo,
		Name:    paramName,
		Payload: payload,
	}

	currentLineNo := startLineNo

	// Parse heredoc payload if present
	if hasHeredoc {
		heredocContent, newLineNo, err := parseHeredoc(scanner, startLineNo+1)
		if err != nil {
			return Param{}, currentLineNo, err
		}
		param.Payload = heredocContent
		currentLineNo = newLineNo
	}

	return param, currentLineNo, nil
}

// parseNameAndPayload extracts the name and payload from a command or parameter line.
// Returns name, payload, whether it has a heredoc, and any error.
func parseNameAndPayload(line string) (string, string, bool, error) {
	line = strings.TrimSpace(line)

	// Empty line
	if line == "" {
		return "", "", false, nil
	}

	// Check for heredoc marker
	if idx := strings.Index(line, "<HEREDOC"); idx != -1 {
		name := strings.TrimSpace(line[:idx])
		return name, "", true, nil
	}

	// Split name and inline payload
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 1 {
		return parts[0], "", false, nil
	}
	return parts[0], parts[1], false, nil
}

// parseHeredoc parses a heredoc payload until the HEREDOC line is encountered.
// Returns the content, the new line number, and any error.
func parseHeredoc(scanner *bufio.Scanner, startLineNo int) (string, int, error) {
	var contentLines []string
	currentLineNo := startLineNo

	for scanner.Scan() {
		currentLineNo++
		line := scanner.Text()

		if strings.TrimSpace(line) == "HEREDOC" {
			return strings.Join(contentLines, "\n"), currentLineNo, nil
		}

		contentLines = append(contentLines, line)
	}

	// If we get here, we reached EOF without finding the HEREDOC marker
	return "", currentLineNo, &ParseError{LineNo: startLineNo, Message: "unclosed heredoc"}
}
