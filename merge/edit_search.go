package merge

import (
	"strings"
)

// SearchBlock holds the parsed blocks from a search block.
type SearchBlock struct {
	Begin string
	End   string
}

// ParseSearchBlock parses a multi-line string into a SearchBlock.
// It scans each line; if a line (trimmed) is "..." then it is used as the separator.
// All lines before the separator are concatenated into Begin.
// All lines after the separator are concatenated into End.
func ParseSearchBlock(input string) SearchBlock {
	lines := strings.Split(input, "\n")
	var beginLines []string
	var endLines []string
	separatorFound := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "..." {
			separatorFound = true
			continue
		}
		if !separatorFound {
			beginLines = append(beginLines, line)
		} else {
			endLines = append(endLines, line)
		}
	}
	return SearchBlock{
		Begin: strings.Join(beginLines, "\n"),
		End:   strings.Join(endLines, "\n"),
	}
}

// MatchString finds the matched string in the given content based on Begin and End markers.
// It returns the matched string including Begin and End, or empty string if not found.
func (sb *SearchBlock) MatchString(content string) string {
	if sb.Begin == "" && sb.End == "" {
		return ""
	}

	beginIndex := strings.Index(content, sb.Begin)
	if beginIndex == -1 {
		return ""
	}

	// If there's no End part, just return the Begin string (which matched exactly)
	if sb.End == "" {
		return sb.Begin
	}

	// Find End string after the Begin string
	searchStart := beginIndex + len(sb.Begin)
	endIndex := strings.Index(content[searchStart:], sb.End)
	if endIndex == -1 {
		return ""
	}

	// Calculate the full matched string including Begin and End
	endIndex = searchStart + endIndex + len(sb.End)
	return content[beginIndex:endIndex]
}

func (sb *SearchBlock) Replace(content string, newContent string) string {
	matched := sb.MatchString(content)
	if matched == "" {
		return content
	}
	return strings.Replace(content, matched, newContent, 1)
}

// Insert adds newContent in front of the matched block, preserving the matched text
func (sb *SearchBlock) Insert(content string, newContent string) string {
	matched := sb.MatchString(content)
	if matched == "" {
		return content
	}
	return strings.Replace(content, matched, newContent+matched, 1)
}

// Append adds newContent after the matched block, preserving the matched text
func (sb *SearchBlock) Append(content string, newContent string) string {
	matched := sb.MatchString(content)
	if matched == "" {
		return content
	}
	return strings.Replace(content, matched, matched+newContent, 1)
}

// Delete removes the matched block entirely
func (sb *SearchBlock) Delete(content string) string {
	matched := sb.MatchString(content)
	if matched == "" {
		return content
	}
	return strings.Replace(content, matched, "", 1)
}
