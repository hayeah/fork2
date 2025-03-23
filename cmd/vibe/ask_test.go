package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFrontMatter_NoFrontMatter(t *testing.T) {
	data := []byte(`some instructions
line2
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.Nil(t, cmd, "cmd should be nil if no front matter")
	assert.Equal(t, data, remainder)
	assert.NoError(t, err)
}

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

func TestParseFrontMatter_UnclosedDelimiter(t *testing.T) {
	data := []byte(`+++
--select=merge/.go
some instructions
line2
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.Nil(t, cmd)
	assert.Nil(t, remainder)
	assert.Error(t, err)
}

func TestAskCmd_Merge_Precedence(t *testing.T) {
	dst := &AskCmd{
		TokenEstimator: "simple",
		Diff:           true,
		Instruction:    "CLI instructions",
	}
	src := &AskCmd{
		TokenEstimator: "tiktoken",
		Diff:           false,
		Select:         "some/path",
		Instruction:    "front matter instructions",
	}
	dst.Merge(src)
	assert.Equal(t, "simple", dst.TokenEstimator, "dst wins if non-empty")
	assert.True(t, dst.Diff, "dst wins if it's true")
	assert.Equal(t, "some/path", dst.Select, "src sets select if dst was empty")
	assert.Equal(t, "CLI instructions", dst.Instruction, "dst instruction wins if present")
}

func TestParseFrontMatter_MultipleFlags(t *testing.T) {
	data := []byte(`---
--copy --select-re=xyz
---
real content
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.True(t, cmd.Copy)
	assert.Equal(t, "xyz", cmd.SelectRegex)
	assert.Equal(t, []byte("real content\n"), remainder)
}

func TestParseFrontMatter_TokenEstimator(t *testing.T) {
	data := []byte(`+++
--token-estimator=tiktoken
+++
user instructions
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.Equal(t, "tiktoken", cmd.TokenEstimator)
	assert.Equal(t, []byte("user instructions\n"), remainder)
}

func TestParseFrontMatter_InvalidClosing(t *testing.T) {
	data := []byte(`+++
--copy
---
content
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.Error(t, err)
	assert.Nil(t, cmd)
	assert.Nil(t, remainder)
}

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
