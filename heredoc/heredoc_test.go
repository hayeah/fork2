package heredoc

import (
	"strings"
	"testing"

	"github.com/hayeah/fork2/internal/assert"
)

func TestParseSimpleCommand(t *testing.T) {
	assert := assert.New(t)

	input := `:command`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)
	assert.Len(commands, 1)

	assert.EqualToJSONFixture("simple_command", commands[0])
}

// TestSplitCommandParams tests the helper function splitCommandParams directly.
func TestSplitCommandParams(t *testing.T) {
	assert := assert.New(t)

	// Create a command with repeated parameter names.
	cmd := Command{
		LineNo:  1,
		Name:    "command",
		Payload: "payload",
		Params: []Param{
			{Name: "param", Payload: "value1"},
			{Name: "other", Payload: "value2"},
			{Name: "param", Payload: "value3"},
			{Name: "another", Payload: "value4"},
			{Name: "other", Payload: "value5"},
		},
	}

	splitCmds := splitCommandParams(cmd)
	assert.Len(splitCmds, 2)

	assert.Equal([]Param{
		{Name: "param", Payload: "value1"},
		{Name: "other", Payload: "value2"},
	}, splitCmds[0].Params)

	assert.Equal([]Param{
		{Name: "param", Payload: "value3"},
		{Name: "another", Payload: "value4"},
		{Name: "other", Payload: "value5"},
	}, splitCmds[1].Params)
}

// TestParseCommandWithRepeatedParams tests the parser integration when repeated parameters occur.
func TestParseCommandWithRepeatedParams(t *testing.T) {
	assert := assert.New(t)
	input := `:command payload
$param value1
$other value2

$param value3
$other value4
`

	parser := NewParser(strings.NewReader(input))

	// First command
	cmd1, err := parser.ParseCommand()
	assert.NoError(err)
	assert.NotNil(cmd1)

	type Params struct {
		Param string `json:"param"`
		Other string `json:"other"`
	}
	var p1 Params
	err = cmd1.Scan(&p1)
	assert.NoError(err)
	assert.Equal("value1", p1.Param)
	assert.Equal("value2", p1.Other)

	// Second command
	cmd2, err := parser.ParseCommand()
	assert.NoError(err)
	assert.NotNil(cmd2)

	var p2 Params
	err = cmd2.Scan(&p2)
	assert.NoError(err)
	assert.Equal("value3", p2.Param)
	assert.Equal("value4", p2.Other)

	// Third call -> no more commands
	cmd3, err := parser.ParseCommand()
	assert.NoError(err)
	assert.Nil(cmd3)
}

func TestParseCommandWithInlinePayload(t *testing.T) {
	assert := assert.New(t)

	input := `:command this is an inline payload`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)
	assert.Len(commands, 1)

	assert.EqualToJSONFixture("inline_payload", commands[0])
}

func TestParseCommandWithHeredocPayload(t *testing.T) {
	assert := assert.New(t)

	input := `:command<HEREDOC
This is a heredoc payload
with multiple lines
HEREDOC`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)
	assert.Len(commands, 1)

	assert.EqualToJSONFixture("heredoc_payload", commands[0])
}

func TestParseCommandWithParameters(t *testing.T) {
	assert := assert.New(t)

	input := `:command
$param1 inline payload

$param2<HEREDOC
Heredoc payload for param2
HEREDOC`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)
	assert.Len(commands, 1)

	cmd := commands[0]
	type Params struct {
		Param1 string `json:"param1"`
		Param2 string `json:"param2"`
	}

	var p Params
	err = cmd.Scan(&p)
	assert.NoError(err)

	assert.EqualToJSONFixture("command_with_params", p)
}

func TestParseMultipleCommands(t *testing.T) {
	assert := assert.New(t)

	input := `:command1 payload1
$param1 value1

:command2<HEREDOC
Payload for command2
HEREDOC

$param2<HEREDOC
Value for param2
HEREDOC`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)

	assert.EqualToJSONFixture("multiple_commands", commands)
}

func TestParseWithComments(t *testing.T) {
	assert := assert.New(t)

	input := `# This is a comment
:command payload
# Another comment
$param value
# Final comment`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)
	assert.Len(commands, 1)

	assert.EqualToJSONFixture("command_with_comments", commands[0])
}

func TestParseEmptyLines(t *testing.T) {
	assert := assert.New(t)

	input := `

:command payload

$param value

`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)
	assert.Len(commands, 1)

	assert.EqualToJSONFixture("command_with_empty_lines", commands[0])
}

func TestParseRealWorldExample(t *testing.T) {
	assert := assert.New(t)

	input := `:plan<HEREDOC
Modify cmd/pick/main.go to add support for a second argument that represents user instruction.
HEREDOC

:modify cmd/pick/main.go

$description<HEREDOC
Update Args struct to add UserInstruction parameter
HEREDOC

$search<HEREDOC
// Args defines the command-line arguments
type Args struct {
	TokenEstimator string
	All            bool
	Copy           bool
}
HEREDOC

$replace<HEREDOC
// Args defines the command-line arguments
type Args struct {
	TokenEstimator  string
	All             bool
	Copy            bool
	UserInstruction string
}
HEREDOC`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)
	assert.Len(commands, 2)

	// Check plan command
	assert.EqualToJSONFixture("plan_command", commands[0])

	// Check modify command
	assert.EqualToJSONFixture("modify_command", commands[1])
}

func TestGetParam(t *testing.T) {
	assert := assert.New(t)

	cmd := Command{
		Name: "test",
		Params: []Param{
			{Name: "param1", Payload: "value1"},
			{Name: "param2", Payload: "value2"},
		},
	}

	// Get existing parameter
	param1 := cmd.GetParam("param1")
	assert.NotNil(param1)

	assert.EqualToJSONFixture("existing_param", *param1)

	// Get non-existing parameter
	param3 := cmd.GetParam("param3")
	assert.Nil(param3)
}

func TestInvalidInput(t *testing.T) {
	assert := assert.New(t)

	// Invalid line
	input := `invalid line`
	_, err := ParseStrict(input)
	assert.Error(err)

	// Empty command name
	input = `:`
	_, err = ParseStrict(input)
	assert.Error(err)

	// Empty parameter name
	input = `:command
$`
	_, err = ParseStrict(input)
	assert.Error(err)

	// Unclosed heredoc
	input = `:command<HEREDOC
payload
`
	_, err = ParseStrict(input)
	assert.Error(err)
}

func TestNestedCommands(t *testing.T) {
	assert := assert.New(t)

	input := `:outer
$param1 value1
:inner
$param2 value2`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)
	assert.Len(commands, 2)

	assert.EqualToJSONFixture("nested_commands", commands)
}

func TestParseCommandIncremental(t *testing.T) {
	assert := assert.New(t)

	input := `:command1 payload1
$param1 value1

:command2<HEREDOC
Payload for command2
HEREDOC

$param2<HEREDOC
Value for param2
HEREDOC

:command3
$param3 value3`

	parser := NewParser(strings.NewReader(input))

	// First command
	cmd1, err := parser.ParseCommand()
	assert.NoError(err)
	assert.NotNil(cmd1)

	assert.EqualToJSONFixture("incremental_command1", cmd1)

	// Second command
	cmd2, err := parser.ParseCommand()
	assert.NoError(err)
	assert.NotNil(cmd2)

	assert.EqualToJSONFixture("incremental_command2", cmd2)

	// Third command
	cmd3, err := parser.ParseCommand()
	assert.NoError(err)
	assert.NotNil(cmd3)

	assert.EqualToJSONFixture("incremental_command3", cmd3)

	// Should be EOF now
	cmd4, err := parser.ParseCommand()
	assert.Equal(nil, err)
	assert.Nil(cmd4)
}

func TestParserNonStrictMode(t *testing.T) {
	assert := assert.New(t)

	input := `invalid line
:command
$param1 value
another invalid line
:command2
$param2 value2`

	parser := NewParser(strings.NewReader(input))
	// Non-strict is default (strict=false). Invalid lines are discarded.

	// Parse first command
	cmd1, err := parser.ParseCommand()
	assert.NoError(err)

	assert.EqualToJSONFixture("non_strict_command1", cmd1)

	// Parse second command
	cmd2, err := parser.ParseCommand()
	assert.NoError(err)

	assert.EqualToJSONFixture("non_strict_command2", cmd2)

	// Should be EOF now
	cmd3, err := parser.ParseCommand()
	assert.NoError(err)
	assert.Nil(cmd3)
}

func TestParserStrictMode(t *testing.T) {
	assert := assert.New(t)

	input := `invalid line
:command
$param1 value`

	parser := NewParser(strings.NewReader(input))
	parser.UseStrict()

	// Parse first command
	cmd1, err := parser.ParseCommand()
	assert.Error(err, "Expected error in strict mode due to 'invalid line'")
	assert.Nil(cmd1)
}

func TestNestedHeredoc(t *testing.T) {
	assert := assert.New(t)

	input := `:modify path/heredoc.txt

$description<HEREDOC
make change to a HEREDOC payload
HEREDOC

$search<HEREDOC_2
$search<HEREDOC
heredoc payload
HEREDOC
HEREDOC_2

$replace<HEREDOC_2
$search<HEREDOC
new heredoc payload
HEREDOC
HEREDOC_2`

	commands, err := Parse(input)
	assert.NoError(err)
	assert.Len(commands, 1)

	assert.EqualToJSONFixture("nested_heredoc", commands[0])
}
