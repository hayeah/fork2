package render

import (
	"testing"

	"github.com/hayeah/fork2/internal/assert"
)

func TestParseFrontMatter(t *testing.T) {
	assert := assert.New(t)

	// Test TOML front matter
	content := `---toml
[[file]]
path = "path/to/file.txt"
---
User instruction here.`

	tag, frontMatter, remainder, err := ParseFrontMatter(content)
	assert.NoError(err)
	assert.Equal("toml", tag)
	assert.Equal("[[file]]\npath = \"path/to/file.txt\"", frontMatter)
	assert.Equal("User instruction here.", remainder)

	// Test alternative delimiter (plus signs)
	content = `+++toml
[[file]]
path = "path/to/file.txt"
+++
User instruction here.`

	tag, frontMatter, remainder, err = ParseFrontMatter(content)
	assert.NoError(err)
	assert.Equal("toml", tag)
	assert.Equal("[[file]]\npath = \"path/to/file.txt\"", frontMatter)
	assert.Equal("User instruction here.", remainder)

	// Test backtick delimiter
	content = "```toml\n[[file]]\npath = \"path/to/file.txt\"\n```\nUser instruction here."

	tag, frontMatter, remainder, err = ParseFrontMatter(content)
	assert.NoError(err)
	assert.Equal("toml", tag)
	assert.Equal("[[file]]\npath = \"path/to/file.txt\"", frontMatter)
	assert.Equal("User instruction here.", remainder)
	// Test no front matter
	content = `This is just regular content
without any front matter.`

	tag, frontMatter, remainder, err = ParseFrontMatter(content)
	assert.NoError(err)
	assert.Equal("", tag)
	assert.Equal("", frontMatter)
	assert.Equal(content, remainder)

	// Test unclosed front matter
	content = `---
unclosed front matter
without closing delimiter`

	_, _, _, err = ParseFrontMatter(content)
	assert.Error(err)
	assert.Contains(err.Error(), "front matter not closed")
}
