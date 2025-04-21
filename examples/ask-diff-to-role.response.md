:plan<HEREDOC
1. Modify the AskCmd struct to replace Diff with Role
2. Update the Merge method to handle Role instead of Diff
3. Update the handleOutput method to use the Role instead of the conditional logic
4. Fix the tests in ask_test.go
HEREDOC

:modify cmd/vibe/ask.go

$description<HEREDOC
Replace the --diff flag with a --role flag that defaults to "coder"
HEREDOC

$search<HEREDOC
// AskCmd contains the arguments for the 'ask' subcommand
type AskCmd struct {
	TokenEstimator string   `arg:"--token-estimator" help:"Token count estimator to use: 'simple' (size/4) or 'tiktoken'" default:"simple"`
	All            bool     `arg:"-a,--all" help:"Select all files and output immediately"`
	Copy           bool     `arg:"-c,--copy" help:"Copy output to clipboard instead of stdout"`
	Diff           bool     `arg:"--diff" help:"Enable diff output format"`
	Select         []string `arg:"--select,separate" help:"Select files matching fuzzy pattern and output immediately (can be specified multiple times)"`
	SelectRegex    string   `arg:"--select-re" help:"Select files matching regex pattern and output immediately"`
	Instruction    string   `arg:"positional" help:"User instruction or path to instruction file"`
}
HEREDOC

$replace<HEREDOC
// AskCmd contains the arguments for the 'ask' subcommand
type AskCmd struct {
	TokenEstimator string   `arg:"--token-estimator" help:"Token count estimator to use: 'simple' (size/4) or 'tiktoken'" default:"simple"`
	All            bool     `arg:"-a,--all" help:"Select all files and output immediately"`
	Copy           bool     `arg:"-c,--copy" help:"Copy output to clipboard instead of stdout"`
	Role           string   `arg:"--role" help:"Specify template role to use" default:"coder"`
	Select         []string `arg:"--select,separate" help:"Select files matching fuzzy pattern and output immediately (can be specified multiple times)"`
	SelectRegex    string   `arg:"--select-re" help:"Select files matching regex pattern and output immediately"`
	Instruction    string   `arg:"positional" help:"User instruction or path to instruction file"`
}
HEREDOC

:modify cmd/vibe/ask.go

$description<HEREDOC
Update the Merge method to handle Role instead of Diff
HEREDOC

$search<HEREDOC
	// Booleans: once set to true, keep them
	cmd.All = cmd.All || src.All
	cmd.Copy = cmd.Copy || src.Copy
	cmd.Diff = cmd.Diff || src.Diff

	// Strings: if empty, overwrite
HEREDOC

$replace<HEREDOC
	// Booleans: once set to true, keep them
	cmd.All = cmd.All || src.All
	cmd.Copy = cmd.Copy || src.Copy

	// Strings: if empty, overwrite
	if cmd.Role == "" {
		cmd.Role = src.Role
	}
HEREDOC

:modify cmd/vibe/ask.go

$description<HEREDOC
Update the handleOutput method to use the Role instead of the conditional logic
HEREDOC

$search<HEREDOC
	role := "<base>"
	if r.Args.Diff {
		role = "<coder>"
	}
HEREDOC

$replace<HEREDOC
	role := "<" + r.Args.Role + ">"
HEREDOC

:modify cmd/vibe/ask_test.go

$description<HEREDOC
Fix the TestParseFrontMatter_Delimited test
HEREDOC

$search<HEREDOC
func TestParseFrontMatter_Delimited(t *testing.T) {
	data := []byte(`+++
--diff
--all
+++
some instructions
line2
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.True(t, cmd.All)
	assert.True(t, cmd.Diff)
	assert.Equal(t, []byte("some instructions\nline2\n"), remainder)
}
HEREDOC

$replace<HEREDOC
func TestParseFrontMatter_Delimited(t *testing.T) {
	data := []byte(`+++
--role=base
--all
+++
some instructions
line2
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.True(t, cmd.All)
	assert.Equal(t, "base", cmd.Role)
	assert.Equal(t, []byte("some instructions\nline2\n"), remainder)
}
HEREDOC

:modify cmd/vibe/ask_test.go

$description<HEREDOC
Fix the TestAskCmd_Merge_Precedence test
HEREDOC

$search<HEREDOC
func TestAskCmd_Merge_Precedence(t *testing.T) {
	dst := &AskCmd{
		TokenEstimator: "simple",
		Diff:           true,
		Instruction:    "CLI instructions",
	}
	src := &AskCmd{
		TokenEstimator: "tiktoken",
		Diff:           false,
		Select:         []string{"some/path"},
		Instruction:    "front matter instructions",
	}
	dst.Merge(src)
	assert.Equal(t, "simple", dst.TokenEstimator, "dst wins if non-empty")
	assert.True(t, dst.Diff, "dst wins if it's true")
	assert.Equal(t, []string{"some/path"}, dst.Select, "src sets select if dst was empty")
	assert.Equal(t, "CLI instructions", dst.Instruction, "dst instruction wins if present")
}
HEREDOC

$replace<HEREDOC
func TestAskCmd_Merge_Precedence(t *testing.T) {
	dst := &AskCmd{
		TokenEstimator: "simple",
		Role:           "custom",
		Instruction:    "CLI instructions",
	}
	src := &AskCmd{
		TokenEstimator: "tiktoken",
		Role:           "base",
		Select:         []string{"some/path"},
		Instruction:    "front matter instructions",
	}
	dst.Merge(src)
	assert.Equal(t, "simple", dst.TokenEstimator, "dst wins if non-empty")
	assert.Equal(t, "custom", dst.Role, "dst role wins if non-empty")
	assert.Equal(t, []string{"some/path"}, dst.Select, "src sets select if dst was empty")
	assert.Equal(t, "CLI instructions", dst.Instruction, "dst instruction wins if present")
}
HEREDOC

:modify cmd/vibe/ask_test.go

$description<HEREDOC
Fix the TestNewAskRunner_FrontMatterParsing test
HEREDOC

$search<HEREDOC
func TestNewAskRunner_FrontMatterParsing(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Test with instruction string containing front matter
	cmdArgs := AskCmd{
		TokenEstimator: "simple",
		Instruction:    "---\n--diff\n---\nThis is a test instruction",
	}

	runner, err := NewAskRunner(cmdArgs, tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, runner)
	assert.True(t, runner.Args.Diff)
	assert.Equal(t, "This is a test instruction", runner.UserInstruction)
}
HEREDOC

$replace<HEREDOC
func TestNewAskRunner_FrontMatterParsing(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Test with instruction string containing front matter
	cmdArgs := AskCmd{
		TokenEstimator: "simple",
		Instruction:    "---\n--role=base\n---\nThis is a test instruction",
	}

	runner, err := NewAskRunner(cmdArgs, tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, runner)
	assert.Equal(t, "base", runner.Args.Role)
	assert.Equal(t, "This is a test instruction", runner.UserInstruction)
}
HEREDOC

:exec git add

$args<HEREDOC
cmd/vibe/ask.go cmd/vibe/ask_test.go
HEREDOC

:exec git commit

$args<HEREDOC
-m "Replace --diff flag with --role flag"
HEREDOC
