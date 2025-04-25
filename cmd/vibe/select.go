// Package main provides file selection functionality using various pattern matching techniques.
//
// # Pattern Syntax
//
// The pattern syntax supports several matching strategies:
//
// 1. Fuzzy Matching (default):
//   - Example: "foo" matches any path containing characters that fuzzy match "foo"
//   - This is the default when no special prefix is used
//   - Patterns beginning with "/" or containing "* ? **" are treated as plain fuzzy text
//
// 2. Exact Path Matching:
//   - Prefix: "="
//   - Example: "=path/to/file.go" matches only the exact path "path/to/file.go"
//   - Can include line ranges: "=path/to/file.go#10,20" selects lines 10-20
//
// 3. Negation (exclude matches):
//   - Prefix: "!"
//   - Example: "!test" excludes paths that would match the pattern "test"
//   - Can be used at term level in fuzzy matching: "cmd !_test.go" matches cmd files that aren't tests
//
// 4. Compound Patterns (logical AND):
//   - Separator: "|"
//   - Example: "cmd|main" matches paths containing both "cmd" and "main"
//   - Can combine different pattern types: "cmd|=main.go|!test"
//
// 5. Union (logical OR):
//   - Separator: ";"
//   - Example: "cmd;main" matches paths containing either "cmd" OR "main"
//
// # Special Cases
//
// - Empty pattern: "" matches all paths
// - "./" prefix is automatically stripped
// - "../" prefix is rejected for security reasons
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/hayeah/fork2/fzf"
)

// Matcher is an interface for matching file paths
type Matcher interface {
	// Match takes a slice of paths and returns the matched paths
	Match(paths []string) ([]string, error)
}

// ExactPathMatcher matches exact file paths or directories
type ExactPathMatcher struct {
	FileSelection
}

// Match implements the Matcher interface for ExactPathMatcher
func (m ExactPathMatcher) Match(paths []string) ([]string, error) {
	// Check if the path exists and is a directory
	fileInfo, err := os.Stat(m.Path)
	if err == nil && fileInfo.IsDir() {
		// If it's a directory, select all files within that directory
		matchesSet := NewSet[string]()
		pathPrefix := m.Path
		// Ensure the path ends with a separator
		if !strings.HasSuffix(pathPrefix, string(os.PathSeparator)) {
			pathPrefix += string(os.PathSeparator)
		}

		// Find all paths that start with the directory path
		for _, path := range paths {
			if strings.HasPrefix(path, pathPrefix) {
				matchesSet.Add(path)
			}
		}

		return matchesSet.Values(), nil
	}

	// Otherwise, match exact file path as before
	for _, path := range paths {
		if path == m.Path {
			return []string{path}, nil
		}
	}
	return []string{}, nil
}

// FuzzyMatcher uses fuzzy matching for file paths
type FuzzyMatcher struct {
	Pattern string
	matcher *fzf.Matcher // Pointer to avoid copying the matcher
}

// NewFuzzyMatcher creates a new FuzzyMatcher with a pre-parsed pattern
func NewFuzzyMatcher(pattern string) (FuzzyMatcher, error) {
	// Empty pattern selects all files
	if pattern == "" {
		return FuzzyMatcher{Pattern: pattern}, nil
	}

	// Create the fzf matcher
	matcher, err := fzf.NewMatcher(pattern)
	if err != nil {
		return FuzzyMatcher{}, fmt.Errorf("invalid fuzzy pattern: %v", err)
	}

	return FuzzyMatcher{
		Pattern: pattern,
		matcher: &matcher,
	}, nil
}

// Match implements the Matcher interface for FuzzyMatcher
func (m FuzzyMatcher) Match(paths []string) ([]string, error) {
	// Empty pattern still selects everything
	if m.Pattern == "" {
		return paths, nil
	}

	// If we have a pre-parsed matcher, use it
	if m.matcher != nil {
		return m.matcher.Match(paths)
	}

	// Fallback for backward compatibility with existing code
	matcher, err := fzf.NewMatcher(m.Pattern)
	if err != nil {
		return nil, err
	}
	return matcher.Match(paths)
}





// NegationMatcher wraps another matcher and negates its results
type NegationMatcher struct {
	Wrapped Matcher
}

// Match implements the Matcher interface for NegationMatcher
func (m NegationMatcher) Match(paths []string) ([]string, error) {
	// Get the matches from the wrapped matcher
	matches, err := m.Wrapped.Match(paths)
	if err != nil {
		return nil, err
	}

	// Create sets for the input paths and matches
	pathsSet := NewSetFromSlice(paths)
	matchesSet := NewSetFromSlice(matches)

	// Return paths that don't match
	resultSet := pathsSet.Difference(matchesSet)
	return resultSet.Values(), nil
}

// CompoundMatcher applies multiple matchers in sequence (logical AND)
type CompoundMatcher struct {
	Matchers []Matcher
}

// Match implements the Matcher interface for CompoundMatcher
func (m CompoundMatcher) Match(paths []string) ([]string, error) {
	currentPaths := paths
	var err error

	for _, matcher := range m.Matchers {
		currentPaths, err = matcher.Match(currentPaths)
		if err != nil {
			return nil, err
		}
	}
	return currentPaths, nil
}

// UnionMatcher applies multiple matchers and combines their results (logical OR)
type UnionMatcher struct {
	Matchers []Matcher
}

// Match implements the Matcher interface for UnionMatcher
func (m UnionMatcher) Match(paths []string) ([]string, error) {
	resultSet := NewSet[string]()

	for _, matcher := range m.Matchers {
		matches, err := matcher.Match(paths)
		if err != nil {
			return nil, err
		}
		resultSet.AddValues(matches)
	}

	return resultSet.Values(), nil
}

// ParseMatcher parses a single pattern string into a Matcher
func ParseMatcher(pattern string) (Matcher, error) {
	pattern = strings.TrimSpace(pattern)
	// Reject patterns starting with "../" as they are potentially dangerous
	if strings.HasPrefix(pattern, "../") {
		return nil, fmt.Errorf("patterns with '../' are not supported for security reasons")
	}

	// Strip "./" prefix if present
	pattern = strings.TrimPrefix(pattern, "./")

	// Check if this is a union pattern with ';' operator (logical OR, highest precedence)
	if strings.Contains(pattern, ";") {
		// Split by ';' and trim whitespace from each part
		parts := strings.Split(pattern, ";")
		subMatchers := make([]Matcher, 0, len(parts))

		for _, part := range parts {
			// Trim whitespace from each part
			part = strings.TrimSpace(part)
			if part == "" {
				continue // Skip empty parts
			}
			matcher, err := ParseMatcher(part)
			if err != nil {
				return nil, fmt.Errorf("in union pattern part '%s': %v", part, err)
			}
			subMatchers = append(subMatchers, matcher)
		}

		// If no valid matchers were created, return an error
		if len(subMatchers) == 0 {
			return nil, fmt.Errorf("union pattern contains no valid patterns")
		}

		// If only one matcher, return it directly without wrapping
		if len(subMatchers) == 1 {
			return subMatchers[0], nil
		}

		return UnionMatcher{Matchers: subMatchers}, nil
	}

	if strings.Contains(pattern, "|") {
		// Handle compound patterns with '|' operator (logical AND)

		parts := strings.Split(pattern, "|")
		subMatchers := make([]Matcher, 0, len(parts))

		for _, part := range parts {
			matcher, err := ParseMatcher(part)
			if err != nil {
				return nil, fmt.Errorf("in pattern part '%s': %v", part, err)
			}
			subMatchers = append(subMatchers, matcher)
		}

		return CompoundMatcher{Matchers: subMatchers}, nil
	}

	// Check if this is a negation pattern
	isNegation := strings.HasPrefix(pattern, "!")
	if isNegation {
		// Wrap with NegationMatcher if it's a negation pattern
		pattern = pattern[1:] // Remove the leading "!"
		// Empty pattern after negation would match everything, which would exclude everything
		if pattern == "" {
			return nil, fmt.Errorf("empty negation pattern '!' is not valid")
		}
		matcher, err := ParseMatcher(pattern)
		if err != nil {
			return nil, err
		}

		return NegationMatcher{
			Wrapped: matcher,
		}, nil
	}

	// Check if this is an exact path matcher
	if strings.HasPrefix(pattern, "=") {
		exactPath := pattern[1:] // Remove the leading "="

		// Create a FileSelection using the helper function
		fileSelection, err := ParseFileSelection(exactPath)
		if err != nil {
			return nil, err
		}

		return ExactPathMatcher{FileSelection: fileSelection}, nil
	}

	// Note: Patterns beginning with "/" or containing glob characters are now treated as plain fuzzy text

	// Default to fuzzy matching
	return NewFuzzyMatcher(pattern)
}



func selectSinglePattern(paths []string, pattern string) ([]string, error) {
	// Empty pattern selects all paths
	if pattern == "" {
		return paths, nil
	}

	// Create a matcher for the pattern
	matcher, err := ParseMatcher(pattern)
	if err != nil {
		return nil, err
	}

	// Apply the matcher
	return matcher.Match(paths)
}

// ParseMatchersFromString parses a string containing multiple patterns into a slice of Matchers
// It skips empty lines and comment lines that start with #
// Example input:
//
//	cmd/.go
//	internal/.go
//
//	# exact path match
//	=path/to/a.txt
//
//	# exact path match and range
//	=path/to/b.txt#1,5
func ParseMatchersFromString(input string) ([]Matcher, error) {
	var matchers []Matcher
	scanner := bufio.NewScanner(strings.NewReader(input))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comment lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the pattern into a matcher
		matcher, err := ParseMatcher(line)
		if err != nil {
			return nil, fmt.Errorf("error parsing pattern '%s': %w", line, err)
		}

		matchers = append(matchers, matcher)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning input: %w", err)
	}

	return matchers, nil
}

// selectByPatterns collects matches from multiple patterns
func selectByPatterns(paths []string, patterns []string) ([]string, error) {
	// Create an empty result set
	resultSet := NewSet[string]()

	// Process each pattern sequentially
	for _, pattern := range patterns {
		// For positive patterns, select matching paths to add to result set
		matches, err := selectSinglePattern(paths, pattern)
		if err != nil {
			return nil, fmt.Errorf("pattern '%s': %v", pattern, err)
		}

		// Add matches to the result set
		resultSet.AddValues(matches)
	}

	// Return the values from the set
	return resultSet.Values(), nil
}
