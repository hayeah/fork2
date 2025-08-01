// Package main provides file selection functionality using various pattern matching techniques.
//
// # Pattern Syntax
// The pattern syntax supports several matching strategies:
//
// 1. Fuzzy matching: "foo" matches any path containing "foo"
// 2. Compound patterns: "foo|bar" matches paths containing both "foo" AND "bar"
// 3. Union patterns: "foo;bar" matches paths containing either "foo" OR "bar"
//
// # Special Cases
//
// - Empty pattern: "" matches all paths
// - "./" prefix is automatically stripped
// - "../" prefix is rejected for security reasons
package selection

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/hayeah/fork2/fzf"
	setpkg "github.com/hayeah/fork2/internal/set"
)

// Matcher is an interface for matching file paths
type Matcher interface {
	// Match takes a slice of paths and returns the matched paths
	Match(paths []string) ([]string, error)
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
	resultSet := setpkg.NewSet[string]()

	for _, matcher := range m.Matchers {
		matches, err := matcher.Match(paths)
		if err != nil {
			return nil, err
		}
		resultSet.AddValues(matches)
	}

	return resultSet.Values(), nil
}

// splitMatchers splits a pattern by the given separator and parses each part into a Matcher
func splitMatchers(pattern, separator string) ([]Matcher, error) {
	parts := strings.Split(pattern, separator)
	subMatchers := make([]Matcher, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue // Skip empty parts
		}
		matcher, err := ParseMatcher(part)
		if err != nil {
			return nil, err
		}
		subMatchers = append(subMatchers, matcher)
	}

	return subMatchers, nil
}

// ParseMatcher parses a single pattern string into a Matcher
func ParseMatcher(pattern string) (Matcher, error) {
	pattern = strings.TrimSpace(pattern)
	// Reject patterns starting with "../" as they are potentially dangerous
	if strings.HasPrefix(pattern, "../") {
		return nil, fmt.Errorf("patterns with '../' are not supported for security reasons")
	}

	// Check if this is a compound pattern with '|' operator (logical AND)
	if strings.Contains(pattern, "|") {
		subMatchers, err := splitMatchers(pattern, "|")
		if err != nil {
			return nil, fmt.Errorf("compound pattern error: %w", err)
		}

		// If no valid matchers were created, return an error
		if len(subMatchers) == 0 {
			return nil, fmt.Errorf("compound pattern contains no valid patterns")
		}

		// If only one matcher, return it directly without wrapping
		if len(subMatchers) == 1 {
			return subMatchers[0], nil
		}

		return CompoundMatcher{Matchers: subMatchers}, nil
	}

	// Check if this is a union pattern with ';' operator (logical OR, highest precedence)
	if strings.Contains(pattern, ";") {
		subMatchers, err := splitMatchers(pattern, ";")
		if err != nil {
			return nil, fmt.Errorf("union pattern error: %w", err)
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

	// Default to fuzzy matching
	return NewFuzzyMatcher(pattern)
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
