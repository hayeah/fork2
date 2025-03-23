package heredoc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSimpleCommand(t *testing.T) {
	assert := assert.New(t)

	input := `:command`

	commands, err := ParseReader(strings.NewReader(input))
	assert.NoError(err)
	assert.Len(commands, 1)
	assert.Equal("command", commands[0].Name)
	assert.Equal("", commands[0].Payload)
	assert.Empty(commands[0].Params)
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
	assert.Equal("command", commands[0].Name)
	assert.Equal("this is an inline payload", commands[0].Payload)
	assert.Empty(commands[0].Params)
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
	assert.Equal("command", commands[0].Name)
	assert.Equal("This is a heredoc payload\nwith multiple lines", commands[0].Payload)
	assert.Empty(commands[0].Params)
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
	assert.Equal("inline payload", p.Param1)
	assert.Equal("Heredoc payload for param2", p.Param2)
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
	assert.Len(commands, 2)

	cmd1 := commands[0]
	assert.Equal("command1", cmd1.Name)
	assert.Equal("payload1", cmd1.Payload)
	assert.Len(cmd1.Params, 1)
	assert.Equal("param1", cmd1.Params[0].Name)
	assert.Equal("value1", cmd1.Params[0].Payload)

	cmd2 := commands[1]
	assert.Equal("command2", cmd2.Name)
	assert.Equal("Payload for command2", cmd2.Payload)
	assert.Len(cmd2.Params, 1)
	assert.Equal("param2", cmd2.Params[0].Name)
	assert.Equal("Value for param2", cmd2.Params[0].Payload)
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

	cmd := commands[0]
	assert.Equal("command", cmd.Name)
	assert.Equal("payload", cmd.Payload)
	assert.Len(cmd.Params, 1)
	assert.Equal("param", cmd.Params[0].Name)
	assert.Equal("value", cmd.Params[0].Payload)
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

	cmd := commands[0]
	assert.Equal("command", cmd.Name)
	assert.Equal("payload", cmd.Payload)
	assert.Len(cmd.Params, 1)
	assert.Equal("param", cmd.Params[0].Name)
	assert.Equal("value", cmd.Params[0].Payload)
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
	planCmd := commands[0]
	assert.Equal("plan", planCmd.Name)
	assert.Equal("Modify cmd/pick/main.go to add support for a second argument that represents user instruction.", planCmd.Payload)
	assert.Empty(planCmd.Params)

	// Check modify command
	modifyCmd := commands[1]
	assert.Equal("modify", modifyCmd.Name)
	assert.Equal("cmd/pick/main.go", modifyCmd.Payload)
	assert.Len(modifyCmd.Params, 3)

	descParam := modifyCmd.GetParam("description")
	assert.NotNil(descParam)
	assert.Equal("Update Args struct to add UserInstruction parameter", descParam.Payload)

	searchParam := modifyCmd.GetParam("search")
	assert.NotNil(searchParam)
	assert.Equal("// Args defines the command-line arguments\ntype Args struct {\n\tTokenEstimator string\n\tAll            bool\n\tCopy           bool\n}", searchParam.Payload)

	replaceParam := modifyCmd.GetParam("replace")
	assert.NotNil(replaceParam)
	assert.Equal("// Args defines the command-line arguments\ntype Args struct {\n\tTokenEstimator  string\n\tAll             bool\n\tCopy            bool\n\tUserInstruction string\n}", replaceParam.Payload)
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
	assert.Equal("param1", param1.Name)
	assert.Equal("value1", param1.Payload)

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

	assert.Equal("outer", commands[0].Name)
	assert.Len(commands[0].Params, 1)
	assert.Equal("param1", commands[0].Params[0].Name)

	assert.Equal("inner", commands[1].Name)
	assert.Len(commands[1].Params, 1)
	assert.Equal("param2", commands[1].Params[0].Name)
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
	assert.Equal("command1", cmd1.Name)
	assert.Equal("payload1", cmd1.Payload)
	assert.Len(cmd1.Params, 1)
	assert.Equal("param1", cmd1.Params[0].Name)
	assert.Equal("value1", cmd1.Params[0].Payload)

	// Second command
	cmd2, err := parser.ParseCommand()
	assert.NoError(err)
	assert.NotNil(cmd2)
	assert.Equal("command2", cmd2.Name)
	assert.Equal("Payload for command2", cmd2.Payload)
	assert.Len(cmd2.Params, 1)
	assert.Equal("param2", cmd2.Params[0].Name)
	assert.Equal("Value for param2", cmd2.Params[0].Payload)

	// Third command
	cmd3, err := parser.ParseCommand()
	assert.NoError(err)
	assert.NotNil(cmd3)
	assert.Equal("command3", cmd3.Name)
	assert.Equal("", cmd3.Payload)
	assert.Len(cmd3.Params, 1)
	assert.Equal("param3", cmd3.Params[0].Name)
	assert.Equal("value3", cmd3.Params[0].Payload)

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
	assert.Equal("command", cmd1.Name)
	assert.Len(cmd1.Params, 1)
	assert.Equal("param1", cmd1.Params[0].Name)
	assert.Equal("value", cmd1.Params[0].Payload)

	// Parse second command
	cmd2, err := parser.ParseCommand()
	assert.NoError(err)
	assert.Equal("command2", cmd2.Name)
	assert.Len(cmd2.Params, 1)
	assert.Equal("param2", cmd2.Params[0].Name)
	assert.Equal("value2", cmd2.Params[0].Payload)

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

	cmd := commands[0]
	assert.Equal("modify", cmd.Name)
	assert.Equal("path/heredoc.txt", cmd.Payload)
	assert.Len(cmd.Params, 3)

	// Check description param
	descParam := cmd.GetParam("description")
	assert.NotNil(descParam)
	assert.Equal("make change to a HEREDOC payload", descParam.Payload)

	searchParam := cmd.GetParam("search")
	assert.NotNil(searchParam)
	assert.Equal(`$search<HEREDOC
heredoc payload
HEREDOC`, searchParam.Payload)

	// Check replace param with nested HEREDOC
	replaceParam := cmd.GetParam("replace")
	assert.NotNil(replaceParam)
	assert.Equal(`$search<HEREDOC
new heredoc payload
HEREDOC`, replaceParam.Payload)
}
