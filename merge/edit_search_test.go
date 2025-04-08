package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSearchBlockWithSeparator(t *testing.T) {
	input := `// RepoDirectoryTree generates the directory tree structure as a string.
// It memoizes the result so subsequent calls are fast.
func (ctx *VibeContext) RepoDirectoryTree() string {
	...
	return ctx.repoFiles
}
`

	sb, _ := ParseSearchBlock(input)
	expectedBegin := `// RepoDirectoryTree generates the directory tree structure as a string.
// It memoizes the result so subsequent calls are fast.
func (ctx *VibeContext) RepoDirectoryTree() string {`
	expectedEnd := "\treturn ctx.repoFiles\n}\n"
	assert.Equal(t, expectedBegin, sb.Begin)
	assert.Equal(t, expectedEnd, sb.End)
}

func TestParseSearchBlockWithoutSeparator(t *testing.T) {
	input := `line1
line2
line3
`
	sb, _ := ParseSearchBlock(input)
	expectedBegin := `line1
line2
line3
`
	expectedEnd := ""
	assert.Equal(t, expectedBegin, sb.Begin)
	assert.Equal(t, expectedEnd, sb.End)
}

func TestParseSearchBlockEmptyInput(t *testing.T) {
	input := ""
	sb, _ := ParseSearchBlock(input)
	assert.Equal(t, "", sb.Begin)
	assert.Equal(t, "", sb.End)
}

func TestMatchString(t *testing.T) {
	assert := assert.New(t)

	// Test case with Begin and End
	content := `line0
line1
line2
line3
line4
line5`

	sb := SearchBlock{
		Begin: "line1",
		End:   "line4",
	}

	expected := `line1
line2
line3
line4`

	result := sb.MatchString(content)
	assert.Equal(expected, result, "should match from line1 to line4")

	// Test case where Begin isn't found
	sb = SearchBlock{
		Begin: "nonexistent",
		End:   "line4",
	}

	result = sb.MatchString(content)
	assert.Equal("", result, "should return empty string when Begin isn't found")

	// Test case where End isn't found
	sb = SearchBlock{
		Begin: "line1",
		End:   "nonexistent",
	}

	result = sb.MatchString(content)
	assert.Equal("", result, "should return empty string when End isn't found")

	// Test case with only Begin
	sb = SearchBlock{
		Begin: "line4\n",
		End:   "",
	}

	result = sb.MatchString(content)
	assert.Equal("line4\n", result, "should match Begin exactly")

	// Test case with empty search block
	sb = SearchBlock{
		Begin: "",
		End:   "",
	}

	result = sb.MatchString(content)
	assert.Equal("", result, "should return empty string with empty search block")
}

func TestMatchStringFromParsedBlock(t *testing.T) {
	assert := assert.New(t)

	searchTemplate := `line1
...
line4
`

	sb, _ := ParseSearchBlock(searchTemplate)

	content := `line0
line1
line2
line3
line4
line5`

	expected := `line1
line2
line3
line4
`

	result := sb.MatchString(content)
	assert.Equal(expected, result, "should match from line1 to line4 using parsed SearchBlock")
}

func TestReplace(t *testing.T) {
	assert := assert.New(t)

	content := `line0
line1
line2
line3
line4
line5`

	// Match from line1 to line4, replace with "foobar"
	sb := SearchBlock{
		Begin: "line1",
		End:   "line4",
	}

	replaced := sb.Replace(content, "foobar")
	expected := `line0
foobar
line5`

	assert.Equal(expected, replaced, "should replace matched from line1 to line4 with 'foobar'")

	// Test no match scenario
	sb = SearchBlock{
		Begin: "lineXXX",
		End:   "lineYYY",
	}
	replaced = sb.Replace(content, "noop")
	assert.Equal(content, replaced, "should return original content if no match found")
}

func TestInsert(t *testing.T) {
	assert := assert.New(t)
	content := `line0
line1
line2
line3
line4
line5`
	sb := SearchBlock{
		Begin: "line1",
		End:   "line4",
	}
	inserted := sb.Insert(content, "INSERTED\n")
	expected := `line0
INSERTED
line1
line2
line3
line4
line5`
	assert.Equal(expected, inserted, "should insert in front of matched block")

	// test no match
	sb = SearchBlock{
		Begin: "xxx",
		End:   "yyy",
	}
	inserted = sb.Insert(content, "INSERTED\n")
	assert.Equal(content, inserted, "should do nothing if not matched")
}

func TestAppend(t *testing.T) {
	assert := assert.New(t)
	content := `line0
line1
line2
line3
line4
line5`
	sb := SearchBlock{
		Begin: "line1",
		End:   "line4\n",
	}
	appended := sb.Append(content, "APPENDED\n")
	expected := `line0
line1
line2
line3
line4
APPENDED
line5`
	assert.Equal(expected, appended, "should append after matched block")

	// test no match
	sb = SearchBlock{
		Begin: "xxx",
		End:   "yyy",
	}
	appended = sb.Append(content, "APPENDED\n")
	assert.Equal(content, appended, "should do nothing if not matched")
}

func TestEditActionDelete(t *testing.T) {
	assert := assert.New(t)
	content := `line0
line1
line2
line3
line4
line5`
	sb := SearchBlock{
		Begin: "line1",
		End:   "line4\n",
	}
	deleted := sb.Delete(content)
	expected := `line0
line5`
	assert.Equal(expected, deleted, "should delete matched block")

	// test no match
	sb = SearchBlock{
		Begin: "xxx",
		End:   "yyy",
	}
	deleted = sb.Delete(content)
	assert.Equal(content, deleted, "should do nothing if not matched")
}
