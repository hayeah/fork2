package selection

import (
	"unicode"
	"unicode/utf8"
)

// isBinaryFile checks if content is likely binary by sampling the first 100 runes
// and checking if they are printable Unicode characters.
func isBinaryFile(content []byte) bool {
	const sampleSize = 100
	var nonPrintable int
	var totalRunes int

	for i := 0; i < len(content) && totalRunes < sampleSize; {
		r, size := utf8.DecodeRune(content[i:])
		if r == utf8.RuneError {
			nonPrintable++
		} else if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			nonPrintable++
		}
		i += size
		totalRunes++
	}

	if totalRunes == 0 {
		return false
	}
	return float64(nonPrintable)/float64(totalRunes) > 0.1
}
