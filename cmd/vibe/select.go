// Package main provides file selection functionality using various pattern matching techniques.
//
// # Pattern Syntax
//
// The pattern syntax supports several matching strategies:
//
// 1. Fuzzy matching (default): "foo" matches any path containing "foo"
//   - Negation: "!pattern" excludes paths matching "pattern" (handled by fzf matcher)
//
// 2. Exact path matching: "=path/to/file.txt" matches only that exact path
// 3. Compound patterns: "foo|bar" matches paths containing both "foo" AND "bar"
// 4. Union patterns: "foo;bar" matches paths containing either "foo" OR "bar"
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
	return m.matcher.Match(paths)
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
