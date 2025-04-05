package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sahilm/fuzzy"
)

// selectSinglePattern is a helper function to select file paths based on a pattern
// If pattern is empty, returns all paths
// If pattern starts with '!', negates the pattern (excludes matches)
// If pattern starts with '/', treats it as a regex pattern
// If pattern starts with './', strips the prefix for matching
// If pattern starts with '../', returns an error
// Otherwise uses fuzzy matching
// If pattern contains '|', it splits the pattern and applies each part as a filter (logical AND)
func selectSinglePattern(paths []string, pattern string) ([]string, error) {
	// Empty pattern selects all files
	if pattern == "" {
		return paths, nil
	}

	// Check if this is a compound pattern with '|' operator (logical AND)
	if strings.Contains(pattern, "|") {
		parts := strings.Split(pattern, "|")
		currentPaths := paths
		var err error

		for _, part := range parts {
			// Apply each pattern part sequentially, narrowing down the results
			currentPaths, err = selectSinglePattern(currentPaths, part)
			if err != nil {
				return nil, fmt.Errorf("in pattern part '%s': %v", part, err)
			}
		}
		return currentPaths, nil
	}

	// Check if this is a negation pattern
	isNegation := strings.HasPrefix(pattern, "!")
	if isNegation {
		pattern = pattern[1:] // Remove the leading "!"
		// Empty pattern after negation would match everything, which would exclude everything
		if pattern == "" {
			return nil, fmt.Errorf("empty negation pattern '!' is not valid")
		}
	}

	// Reject patterns starting with "../" as they are potentially dangerous
	if strings.HasPrefix(pattern, "../") {
		return nil, fmt.Errorf("patterns with '../' are not supported for security reasons")
	}

	// Strip "./" prefix if present
	if strings.HasPrefix(pattern, "./") {
		pattern = pattern[2:] // Remove the leading "./"
	}

	// Create sets for the input paths and matches
	pathsSet := NewSetFromSlice(paths)
	matchesSet := NewSet[string]()

	// Find matches based on the pattern type
	if strings.HasPrefix(pattern, "/") {
		// Regex pattern
		regexPattern := pattern[1:] // Remove the leading '/'
		regex, err := regexp.Compile(regexPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %v", err)
		}

		for _, path := range paths {
			if regex.MatchString(path) {
				matchesSet.Add(path)
			}
		}
	} else {
		// Fuzzy matching
		fuzzyMatches := fuzzy.Find(pattern, paths)
		for _, match := range fuzzyMatches {
			matchesSet.Add(paths[match.Index])
		}
	}

	// If this is a negation pattern, return paths that don't match
	if isNegation {
		resultSet := pathsSet.Difference(matchesSet)
		return resultSet.Values(), nil
	}

	return matchesSet.Values(), nil
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
