package selection

import (
	"path/filepath"
	"strings"
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

// isLockFile checks if a file is a well-known lock file
func isLockFile(path string) bool {
	filename := filepath.Base(path)
	lockFiles := []string{
		// JavaScript/Node.js
		"package-lock.json",
		"npm-shrinkwrap.json",
		"yarn.lock",
		"pnpm-lock.yaml",
		"bun.lockb",
		// Go
		"go.sum",
		// Python
		"Pipfile.lock",
		"poetry.lock",
		"pdm.lock",
		"requirements.lock",
		// Ruby
		"Gemfile.lock",
		// Rust
		"Cargo.lock",
		// PHP
		"composer.lock",
		// .NET
		"packages.lock.json",
		// Swift
		"Package.resolved",
		// Dart/Flutter
		"pubspec.lock",
	}

	for _, lockFile := range lockFiles {
		if strings.EqualFold(filename, lockFile) {
			return true
		}
	}
	return false
}
