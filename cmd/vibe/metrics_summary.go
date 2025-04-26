package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hayeah/fork2/internal/metrics"
)

// trimMiddle returns s unchanged if len(s) ≤ max; otherwise returns
// the first max/2-1 runes + "…" + last max/2-1 runes.
func trimMiddle(s string, max int) string {
	if len(s) <= max {
		return s
	}

	half := max/2 - 1
	return s[:half] + "…" + s[len(s)-half:]
}

// PrintTokenBreakdown prints a visualization of token contribution by each metric item.
// Bars are normalized to the largest bucket, not to 100% of the total.
func PrintTokenBreakdown(m *metrics.OutputMetrics, barW int, fill rune) {
	// Wait to ensure all workers are done
	m.Wait()

	// Calculate total tokens and find maximum token count
	var totalTokens int
	var maxTokens int
	for _, item := range m.Items {
		totalTokens += item.Tokens
		if item.Tokens > maxTokens {
			maxTokens = item.Tokens
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
		key := trimMiddle(e.key.String(), 40) // fixed key width
		fmt.Printf("%-*s  %5.1f%%  %s\n", barW, bar, e.pct, key)
	}
}
