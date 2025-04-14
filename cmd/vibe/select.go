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
	"strings"

	"github.com/bmatcuk/doublestar/v4"
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

// GlobMatcher uses standard glob patterns (including '**') to match file paths
type GlobMatcher struct {
	Pattern string
}

func NewGlobMatcher(pattern string) (GlobMatcher, error) {
	if !doublestar.ValidatePattern(pattern) {
		return GlobMatcher{}, fmt.Errorf("invalid glob pattern '%s'", pattern)
	}
	return GlobMatcher{Pattern: pattern}, nil
}

func (m GlobMatcher) Match(paths []string) ([]string, error) {
	matchesSet := NewSet[string]()

	for _, p := range paths {
		match, err := doublestar.Match(m.Pattern, p)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern '%s': %v", m.Pattern, err)
		}
		if match {
			matchesSet.Add(p)
		}
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

	// Check if this is an exact path matcher
	if strings.HasPrefix(pattern, "=") {
		exactPath := pattern[1:] // Remove the leading "="

		// Create a FileSelection using the helper function
		fileSelection, err := ParseFileSelection(exactPath)
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

	// Check if this is a glob pattern
	if isGlobPattern(pattern) {
		return NewGlobMatcher(pattern)
	}

	// Default to fuzzy matching
	return FuzzyMatcher{
		Pattern: pattern,
	}, nil
}

// selectSinglePattern selects file paths based on a pattern
func isGlobPattern(pat string) bool {
	// A simple check for wildcard chars used by globbing
	// This includes '*', '?', or the '**' sequence
	return strings.ContainsAny(pat, "*?") || strings.Contains(pat, "**")
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
