package render

import (
	"testing"

	"github.com/hayeah/fork2/internal/assert"
)

func TestParseFrontMatter(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		name            string
		content         string
		wantTag         string
		wantFrontMatter string
		wantRemainder   string
		wantErr         bool
		wantErrContains string
	}{
		{
			name: "TOML front matter",
			content: `---toml
[[file]]
path = "path/to/file.txt"
---
User instruction here.`,
			wantTag: "toml",
			wantFrontMatter: `[[file]]
path = "path/to/file.txt"`,
			wantRemainder: "User instruction here.",
		},
		{
			name: "Plus delimiter",
			content: `+++toml
[[file]]
path = "path/to/file.txt"
+++
User instruction here.`,
			wantTag: "toml",
			wantFrontMatter: `[[file]]
path = "path/to/file.txt"`,
			wantRemainder: "User instruction here.",
		},
		{
			name:    "Backtick delimiter",
			content: "```toml\n[[file]]\npath = \"path/to/file.txt\"\n```\nUser instruction here.",
			wantTag: "toml",
			wantFrontMatter: `[[file]]
path = "path/to/file.txt"`,
			wantRemainder: "User instruction here.",
		},
		{
			name: "No front matter",
			content: `This is just regular content
without any front matter.`,
			wantTag:         "",
			wantFrontMatter: "",
			wantRemainder: `This is just regular content
without any front matter.`,
		},
		{
			name: "Unclosed front matter",
			content: `---
unclosed front matter
without closing delimiter`,
			wantErr:         true,
			wantErrContains: "front matter not closed",
		},
		{
			name:            "Leading blanks allowed",
			content:         "\n  \n---toml\nkey = \"value\"\n---\nbody",
			wantTag:         "toml",
			wantFrontMatter: "key = \"value\"",
			wantRemainder:   "body",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tag, fm, rem, err := ParseFrontMatter(tc.content)
			if tc.wantErr {
				assert.Error(err)
				assert.Contains(err.Error(), tc.wantErrContains)
				return
			}
			assert.NoError(err)
			assert.Equal(tc.wantTag, tag)
			assert.Equal(tc.wantFrontMatter, fm)
			assert.Equal(tc.wantRemainder, rem)
		})
	}
}
