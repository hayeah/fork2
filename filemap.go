package fork2

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FileMap represents a mapping of file paths to their contents
type FileMap struct {
	Files map[string]string
}

// IsBinaryFile checks if content is likely binary by sampling the first 100 runes
// and checking if they are printable Unicode characters
func IsBinaryFile(content []byte) bool {
	// Sample the first 100 runes
	const sampleSize = 100
	var nonPrintable int
	var totalRunes int

	// Convert bytes to runes and check if they're printable
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

	// If more than 10% of the sampled runes are non-printable, consider it binary
	threshold := 0.1
	if totalRunes == 0 {
		return false // Empty file, not binary
	}
	return float64(nonPrintable)/float64(totalRunes) > threshold
}

// WriteFileMap writes a filemap to the provided writer
func WriteFileMap(w io.Writer, paths []string) error {
	for _, path := range paths {
		// Skip directories
		fileInfo, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("failed to stat file %s: %w", path, err)
		}
		if fileInfo.IsDir() {
			continue
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Check if file is binary
		if IsBinaryFile(content) {
			continue // Skip binary files
		}

		// Determine language for code block based on file extension
		ext := filepath.Ext(path)
		language := ""
		switch strings.ToLower(ext) {
		case ".go":
			language = "go"
		case ".js":
			language = "javascript"
		case ".py":
			language = "python"
		case ".rb":
			language = "ruby"
		case ".java":
			language = "java"
		case ".c", ".cpp", ".h", ".hpp":
			language = "cpp"
		case ".cs":
			language = "csharp"
		case ".php":
			language = "php"
		case ".ts":
			language = "typescript"
		case ".html":
			language = "html"
		case ".css":
			language = "css"
		case ".md":
			language = "markdown"
		case ".json":
			language = "json"
		case ".yaml", ".yml":
			language = "yaml"
		case ".toml":
			language = "toml"
		case ".sh", ".bash":
			language = "bash"
		case ".sql":
			language = "sql"
		default:
			language = ""
		}

		// Write file header
		fmt.Fprintf(w, "File: %s\n", path)
		fmt.Fprintf(w, "```%s\n", language)

		// Write file content
		fmt.Fprint(w, string(content))

		// Ensure the content ends with a newline
		if len(content) > 0 && content[len(content)-1] != '\n' {
			fmt.Fprintln(w)
		}

		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
	}

	return nil
}
