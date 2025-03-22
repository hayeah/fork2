package heredoc

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// Commands is a slice of Command
type Commands []Command

// Command represents a single command with its parameters
type Command struct {
	LineNo  int    // Line number where the command starts
	Name    string // Command name (cannot be empty)
	Payload string // Command payload (can be empty)
	Params  []Param
}

// Description returns the "description" param payload if present, else "".
func (cmd *Command) Description() string {
	descParam := cmd.GetParam("description")
	if descParam == nil {
		return ""
	}
	return descParam.Payload
}

// Param represents a parameter in a command
type Param struct {
	LineNo  int    // Line number where the parameter starts
	Name    string // Parameter name (cannot be empty)
	Payload string // Parameter payload (can be empty)
}

// GetParam retrieves a parameter by name, returns nil if not found
func (c *Command) GetParam(name string) *Param {
	for i := range c.Params {
		if c.Params[i].Name == name {
			return &c.Params[i]
		}
	}
	return nil
}

// Parser handles the parsing of heredoc formatted content
type Parser struct {
	scanner *bufio.Scanner
	lineNo  int     // The line number of the most recently peeked/consumed line
	peeked  *string // If not nil, it holds the last peeked line that hasn't been consumed yet
	eof     bool    // True if we've reached the end of the reader
	strict  bool    // if true, parser is in strict mode
}

// NewParser creates a new Parser instance
func NewParser(r io.Reader) *Parser {
	return &Parser{
		scanner: bufio.NewScanner(r),
	}
}

// Parse parses the input and returns all commands
func Parse(input string) (Commands, error) {
	return ParseReader(strings.NewReader(input))
}

// ParseStrict parses the input in strict mode and returns all commands.
func ParseStrict(input string) (Commands, error) {
	r := strings.NewReader(input)
	p := NewParser(r)
	p.UseStrict()
	return p.Parse()
}

// ParseReader parses from an io.Reader and returns all commands
func ParseReader(r io.Reader) (Commands, error) {
	return NewParser(r).Parse()
}

// Parse parses all commands from the input
func (p *Parser) Parse() (cmds Commands, err error) {
	for {
		cmd, err := p.ParseCommand()
		if err == io.EOF {
			return cmds, nil
		}
		if err != nil {
			return cmds, err
		}
		if cmd == nil {
			break
		}
		cmds = append(cmds, *cmd)
	}
	return cmds, nil
}

func (p *Parser) UseStrict() {
	p.strict = true
}

// ParseCommand parses and returns the next command from the input.
// Returns nil when there are no more commands to parse.
// This method is useful for streaming applications where commands
// need to be processed incrementally.
func (p *Parser) ParseCommand() (*Command, error) {
	// Attempt to skip until a command/param or EOF
	err := p.skipEmptyAndComments()
	if err == io.EOF {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	line, err := p.peekLine()
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(line, ":") {
		// We found a command
		cmd, err := p.parseCommand()
		if err != nil {
			return nil, err
		}
		return &cmd, nil
	} else {
		// The only valid lines that remain after skipUntil... are commands/params
		// If this line isn't a command, it must be an error unless it's "$" param.
		// But a lone "$" param here would also be an error because
		// top-level data should come in as commands.

		if p.strict {
			return nil, errors.New("invalid line outside command: " + line)
		} else {
			_, err = p.consumeLine()
			if err != nil {
				return nil, err
			}
			return p.ParseCommand()
		}
	}
}

// --- Core "peek" / "consume" mechanism --- //

// peekLine returns the next line without consuming it.
// If EOF has been reached, it returns an io.EOF error.
func (p *Parser) peekLine() (string, error) {
	if p.eof {
		return "", io.EOF
	}
	// If we've already peeked a line, return that
	if p.peeked != nil {
		return *p.peeked, nil
	}
	// Otherwise, pull from scanner
	if p.scanner.Scan() {
		p.lineNo++
		line := p.scanner.Text()
		p.peeked = &line
		return line, nil
	}
	// Scanner is done
	p.eof = true
	if err := p.scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// consumeLine returns the next line and advances the scanner for real.
// If EOF has been reached, it returns an io.EOF error.
func (p *Parser) consumeLine() (string, error) {
	line, err := p.peekLine()
	if err != nil {
		return "", err
	}
	// Clear the peek buffer — we've now consumed that line
	p.peeked = nil
	return line, nil
}

// skipEmptyAndComments keeps discarding empty lines / comments. If it finds a line
// starting with ':' or '$', it stops (i.e., the line is still "peeked" but not consumed).
// Returns false if we reach EOF with no valid lines left.
func (p *Parser) skipEmptyAndComments() error {
	for {
		line, err := p.peekLine()
		if err != nil {
			// No more lines
			return err
		}

		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			// This is an empty line or a comment; consume and keep going
			_, err := p.consumeLine()
			if err != nil {
				return err
			}
			continue
		}

		return nil
	}
}

// parseCommand parses a command line (":name payload...") plus its subsequent param lines.
func (p *Parser) parseCommand() (Command, error) {
	// We expect a line starting with ':'
	line, err := p.peekLine()
	if err != nil {
		return Command{}, errors.New("expected command line but found EOF")
	}
	if !strings.HasPrefix(line, ":") {
		return Command{}, errors.New("expected command, got: " + line)
	}

	// Now truly consume this line
	line, err = p.consumeLine()
	if err != nil {
		return Command{}, err
	}
	cmdLineNo := p.lineNo // lineNo is set after scanning

	cmdBody := line[1:] // strip leading ':'

	cmdName, cmdPayload, err := p.parseNameAndPayload(cmdBody)
	if err != nil {
		return Command{}, err
	}
	if cmdName == "" {
		return Command{}, errors.New("command name cannot be empty")
	}

	cmd := Command{
		LineNo:  cmdLineNo,
		Name:    cmdName,
		Payload: cmdPayload,
	}

	// parse parameters until we see a new command (':'), EOF, or invalid line
	for {
		peek, err := p.peekLine()
		if err != nil {
			// no more lines => done
			break
		}
		// skip empty lines & comments
		if strings.TrimSpace(peek) == "" || strings.HasPrefix(peek, "#") {
			_, err := p.consumeLine()
			if err != nil {
				return Command{}, err
			}
			continue
		}

		if strings.HasPrefix(peek, ":") {
			// we reached a new command => stop param parsing
			break
		}

		if strings.HasPrefix(peek, "$") {
			param, err := p.parseParam()
			if err != nil {
				return Command{}, err
			}
			cmd.Params = append(cmd.Params, param)
		} else {
			if p.strict {
				return Command{}, errors.New("invalid line in command parameters: " + peek)
			} else {
				_, err := p.consumeLine()
				if err != nil {
					return Command{}, err
				}
				continue
			}
		}
	}

	return cmd, nil
}

// parseParam parses a param line ("$name payload...").
func (p *Parser) parseParam() (Param, error) {
	line, err := p.peekLine()
	if err != nil {
		return Param{}, errors.New("expected parameter but got EOF")
	}

	if !strings.HasPrefix(line, "$") {
		return Param{}, errors.New("expected parameter, got: " + line)
	}

	// consume it now
	line, err = p.consumeLine()
	if err != nil {
		return Param{}, err
	}
	paramLineNo := p.lineNo
	paramBody := line[1:] // remove the '$'

	paramName, paramPayload, err := p.parseNameAndPayload(paramBody)
	if err != nil {
		return Param{}, err
	}
	if paramName == "" {
		return Param{}, errors.New("parameter name cannot be empty")
	}

	return Param{
		LineNo:  paramLineNo,
		Name:    paramName,
		Payload: paramPayload,
	}, nil
}

// parseNameAndPayload parses a name from a line, reading up to a space or "<" for heredoc
func (p *Parser) parseNameAndPayload(line string) (name string, payload string, err error) {
	// Find the first space or '<'
	spaceIdx := strings.IndexByte(line, ' ')
	heredocIdx := strings.IndexByte(line, '<')

	// If neither found, the whole line is the name
	if spaceIdx == -1 && heredocIdx == -1 {
		return strings.TrimSpace(line), "", nil
	}

	// Find the first delimiter (space or '<')
	idx := spaceIdx
	if heredocIdx != -1 && (spaceIdx == -1 || heredocIdx < spaceIdx) {
		idx = heredocIdx
	}

	// Extract the name
	name = strings.TrimSpace(line[:idx])

	// Check what follows the name
	if idx == spaceIdx {
		// Space follows => line payload
		payload = strings.TrimRight(line[idx+1:], " ")
	} else if idx == heredocIdx {
		// '<' => heredoc
		payload, err = p.parseHeredocPayload(line[idx:])
	}
	return
}

// parseHeredocPayload parses a heredoc payload starting at "<MARKER"
// The line you get here starts with '<', so typically you'll have something like "<EOF"
func (p *Parser) parseHeredocPayload(line string) (string, error) {
	if !strings.HasPrefix(line, "<") {
		return "", errors.New("expected heredoc marker, got: " + line)
	}
	// marker is everything after '<' until we see a newline or end of line
	// but in a typical heredoc usage, there's no extra content after the marker on the same line
	// so let's just consider the entire remainder as the marker token
	marker := strings.TrimSpace(line[1:])
	if marker == "" {
		return "", errors.New("empty heredoc marker")
	}

	var sb strings.Builder
	firstLine := true

	// Read lines until we see the marker on its own
	for {
		next, err := p.peekLine()
		if err != nil {
			return "", errors.New("unclosed heredoc: " + marker)
		}
		if next == marker {
			// consume the marker line and stop
			_, err := p.consumeLine()
			if err != nil {
				return "", err
			}
			break
		}
		_, err = p.consumeLine() // we are using this line
		if err != nil {
			return "", err
		}
		if !firstLine {
			sb.WriteString("\n")
		}
		sb.WriteString(next)
		firstLine = false
	}

	return sb.String(), nil
}
