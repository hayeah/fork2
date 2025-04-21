package render

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

// RawFrontMatter represents the raw front matter content and tag
type RawFrontMatter struct {
	Content string
	Tag     string
}

// ParseFrontMatter extracts front matter from the template content
func ParseFrontMatter(data string) (string, string, string, error) {
	lines := strings.Split(data, "\n")
	if len(lines) == 0 {
		// No data => nothing to parse.
		return "", "", data, nil
	}

	// Check if the first line begins with "---", "+++", or "```"
	firstLine := string(lines[0])
	var delimiter, tag string

	switch {
	case strings.HasPrefix(firstLine, "---"):
		delimiter = "---"
		tag = strings.TrimPrefix(firstLine, "---")
	case strings.HasPrefix(firstLine, "+++"):
		delimiter = "+++"
		tag = strings.TrimPrefix(firstLine, "+++")
	case strings.HasPrefix(firstLine, "```"):
		delimiter = "```"
		tag = strings.TrimPrefix(firstLine, "```")
	default:
		// Not front matter at all; just return everything as remainder
		return "", "", string(data), nil
	}

	tag = strings.TrimSpace(tag)

	// Now find the matching closing delimiter line.
	var frontMatterLines []string
	foundClose := false

	i := 1
	for ; i < len(lines); i++ {
		if string(lines[i]) == delimiter {
			foundClose = true
			break
		}
		frontMatterLines = append(frontMatterLines, lines[i])
	}

	if !foundClose {
		return "", "", "", fmt.Errorf(
			"front matter not closed; expected closing delimiter %q", delimiter,
		)
	}

	// Remainder is everything after the line with the closing delimiter
	remainderLines := lines[i+1:]
	remainder := strings.Join(remainderLines, "\n")

	return tag, strings.Join(frontMatterLines, "\n"), remainder, nil
}

// ParseToml parses TOML content into the provided structure
func ParseToml(content string, v interface{}) error {
	decoder := toml.NewDecoder(strings.NewReader(content))
	if _, err := decoder.Decode(v); err != nil {
		return fmt.Errorf("failed to parse TOML: %w", err)
	}
	return nil
}
