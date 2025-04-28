package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hayeah/fork2/internal/metrics"
	"golang.org/x/term"
)

// trimPrefix returns s unchanged if len(s) ≤ max; otherwise returns
// "…" + the last max-1 runes, preserving the suffix.
func trimPrefix(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "…" + s[len(s)-max+1:]
}

// termWidth returns the width of the terminal, or 80 as a fallback.
func termWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80 // Safe fallback when stdout is not a TTY or on error
	}
	return width
}

// PrintTokenBreakdown prints a visualization of token contribution by each metric item.
// Bars are normalized to the largest bucket, not to 100% of the total.
// The layout automatically adjusts to the terminal width, with paths trimmed
// to preserve the suffix when necessary.
func PrintTokenBreakdown(m *metrics.OutputMetrics, barW int, fill rune) {
	// Fixed width constants
	const (
		pctW    = 6 // "100.0%"
		tokensW = 6 // right-aligned "123456"
		gapW    = 2 // two spaces between every column
	)

	// Determine dynamic column widths based on terminal size
	tw := termWidth()
	if barW <= 0 {
		// Auto-size the bar width if not specified
		barW = int(float64(tw) * 0.35) // 35% for the bar
	}
	keyW := tw - (barW + pctW + tokensW + gapW*3)
	if keyW < 8 {
		keyW = 8 // never collapse the key column too much
	}

	// Wait to ensure all workers are done
	m.Wait()

	// Calculate total tokens and find maximum token count
	var totalTokens int
	var maxTokens int
	var fileCount int
	for k, item := range m.Items {
		totalTokens += item.Tokens
		if item.Tokens > maxTokens {
			maxTokens = item.Tokens
		}
		if k.Type == "file" {
			fileCount++
		}
	}

	if totalTokens == 0 || maxTokens == 0 {
		fmt.Println("No tokens recorded")
		return
	}

	// Copy items into a slice for sorting
	type entry struct {
		key    metrics.MetricKey
		tokens int
		pct    float64
	}
	entries := make([]entry, 0, len(m.Items))
	for k, item := range m.Items {
		pct := float64(item.Tokens) * 100 / float64(totalTokens)
		entries = append(entries, entry{
			key:    k,
			tokens: item.Tokens,
			pct:    pct,
		})
	}

	// Sort by percentage (ascending - lowest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].pct < entries[j].pct
	})

	// Print the bar chart
	for _, e := range entries {
		// Scale bar length relative to the largest bucket
		ratio := float64(e.tokens) / float64(maxTokens)
		barLen := int(ratio*float64(barW) + 0.5)
		if barLen == 0 && e.tokens > 0 {
			barLen = 1 // always show a dot for non-zero buckets
		}
		bar := strings.Repeat(string(fill), barLen)
		key := trimPrefix(e.key.String(), keyW)
		fmt.Printf("%-*s  %5.1f%%  %*d  %-*s\n", barW, bar, e.pct, tokensW, e.tokens, keyW, key)
	}

	// Print a totals row
	sep := strings.Repeat("─", barW)
	fmt.Printf("%-*s  %5.1f%%  %*d  %-*s\n", barW, sep, 100.0, tokensW, totalTokens, keyW, "TOTAL")

	// Print a summary line
	fmt.Printf("\nSummary: %d files, %d tokens\n", fileCount, totalTokens)
}
