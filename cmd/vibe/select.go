// Package main provides file selection functionality using various pattern matching techniques.
//
// # Pattern Syntax
//
// The pattern syntax supports several matching strategies:
//
// 1. Fuzzy Matching (default):
//   - Example: "foo" matches any path containing characters that fuzzy match "foo"
//   - This is the default when no special prefix is used
//
// 2. Regular Expression Matching:
//   - Prefix: "/"
//   - Example: "/\.go$" matches paths ending with ".go"
//
// 3. Exact Path Matching:
//   - Prefix: "="
//   - Example: "=path/to/file.go" matches only the exact path "path/to/file.go"
//   - Can include line ranges: "=path/to/file.go#10,20" selects lines 10-20
//
// 4. Negation (exclude matches):
//   - Prefix: "!"
//   - Example: "!test" excludes paths that would match the pattern "test"
//   - Can be combined with other pattern types: "!/\.test\.go$"
//
// 5. Compound Patterns (logical AND):
//   - Separator: "|"
//   - Example: "cmd|main" matches paths containing both "cmd" and "main"
//   - Can combine different pattern types: "cmd|/\.go$|!test"
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
	"regexp"
	"strconv"
	"strings"

	"github.com/sahilm/fuzzy"
)

// Matcher is an interface for matching file paths
type Matcher interface {
	// Match takes a slice of paths and returns the matched paths
	Match(paths []string) ([]string, error)
}

// ExactPathMatcher matches exact file paths
type ExactPathMatcher struct {
	FileSelection
}

// Match implements the Matcher interface for ExactPathMatcher
func (m ExactPathMatcher) Match(paths []string) ([]string, error) {
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
}

// Match implements the Matcher interface for FuzzyMatcher
func (m FuzzyMatcher) Match(paths []string) ([]string, error) {
	// Empty pattern selects all files
	if m.Pattern == "" {
		return paths, nil
	}

	matchesSet := NewSet[string]()

	// Fuzzy matching
	fuzzyMatches := fuzzy.Find(m.Pattern, paths)
	for _, match := range fuzzyMatches {
		matchesSet.Add(paths[match.Index])
	}

	return matchesSet.Values(), nil
}

// RegexMatcher uses regular expressions for matching file paths
type RegexMatcher struct {
	Pattern string
	regex   *regexp.Regexp
}

// NewRegexMatcher creates a new RegexMatcher with a pre-compiled regex pattern
func NewRegexMatcher(pattern string) (*RegexMatcher, error) {
	// Empty pattern selects all files
	if pattern == "" {
		return &RegexMatcher{Pattern: pattern}, nil
	}

	// Compile the regex pattern
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	return &RegexMatcher{
		Pattern: pattern,
		regex:   regex,
	}, nil
}

// Match implements the Matcher interface for RegexMatcher
func (m *RegexMatcher) Match(paths []string) ([]string, error) {
	// Empty pattern selects all files
	if m.Pattern == "" {
		return paths, nil
	}

	matchesSet := NewSet[string]()

	// Find matches using regex
	for _, path := range paths {
		if m.regex.MatchString(path) {
			matchesSet.Add(path)
		}
	}

	return matchesSet.Values(), nil
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

// ParseMatcher parses a single pattern string into a Matcher
func ParseMatcher(pattern string) (Matcher, error) {
	pattern = strings.TrimSpace(pattern)
	// Reject patterns starting with "../" as they are potentially dangerous
	if strings.HasPrefix(pattern, "../") {
		return nil, fmt.Errorf("patterns with '../' are not supported for security reasons")
	}

	// Strip "./" prefix if present
	pattern = strings.TrimPrefix(pattern, "./")

	// Check if this is an exact path matcher
	if strings.HasPrefix(pattern, "=") {
		exactPath := pattern[1:] // Remove the leading "="

		// Create a FileSelection using the helper function
		fileSelection, err := parseLineRangeFromPath(exactPath)
		if err != nil {
			return nil, err
		}

		return ExactPathMatcher{FileSelection: fileSelection}, nil
	} else if strings.Contains(pattern, "|") {
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

	// Check if this is a regex pattern
	isRegex := strings.HasPrefix(pattern, "/")
	if isRegex {
		pattern = pattern[1:] // Remove the leading '/'

		// Create a new RegexMatcher
		matcher, err := NewRegexMatcher(pattern)
		if err != nil {
			return nil, err
		}

		return matcher, nil
	}

	// Default to fuzzy matching
	return FuzzyMatcher{
		Pattern: pattern,
	}, nil
}

// parseLineRangeFromPath parses a path string that may contain a line range specification
// Format: path#start,end where start and end are line numbers
// Returns a FileSelection with the path and any line ranges found
func parseLineRangeFromPath(path string) (FileSelection, error) {
	// If there's no hash character, just return the path as is
	if !strings.Contains(path, "#") {
		return FileSelection{Path: path}, nil
	}

	// Use a regular expression to validate and parse the path format
	// The pattern matches: <filepath>#<start>,<end> where start and end are integers
	pattern := `^(.+)#(\d+),(\d+)$`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(path)

	// If the pattern doesn't match, return an error
	if matches == nil {
		return FileSelection{}, fmt.Errorf("invalid file path format: must be in format path#start,end")
	}

	// Extract the file path and line numbers from the regex matches
	filePath := matches[1]
	startLine, err := strconv.Atoi(matches[2])
	if err != nil {
		return FileSelection{}, fmt.Errorf("invalid start line number in range: %v", err)
	}

	endLine, err := strconv.Atoi(matches[3])
	if err != nil {
		return FileSelection{}, fmt.Errorf("invalid end line number in range: %v", err)
	}

	// Create a FileSelection with the path and line range
	return FileSelection{
		Path: filePath,
		Ranges: []LineRange{{
			Start: startLine,
			End:   endLine,
		}},
	}, nil
}

// selectSinglePattern selects file paths based on a pattern
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
//   cmd/.go
//   internal/.go
//
//   # exact path match
//   =path/to/a.txt
//
//   # exact path match and range
//   =path/to/b.txt#1,5
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
