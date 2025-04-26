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

// PrintTokenBreakdown prints a visualization of token contribution by each metric item
func PrintTokenBreakdown(m *metrics.OutputMetrics, barW int, fill rune) {
	// Wait to ensure all workers are done
	m.Wait()

	// Calculate total tokens
	var totalTokens int
	for _, item := range m.Items {
		totalTokens += item.Tokens
	}

	if totalTokens == 0 {
		fmt.Println("No tokens recorded")
		return
	}

	// Copy items into a slice for sorting
	type entry struct {
		key metrics.MetricKey
		pct float64
	}
	entries := make([]entry, 0, len(m.Items))
	for k, item := range m.Items {
		pct := float64(item.Tokens) * 100 / float64(totalTokens)
		entries = append(entries, entry{
			key: k,
			pct: pct,
		})
	}

	// Sort by percentage (ascending - lowest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].pct < entries[j].pct
	})

	// Print the bar chart
	for _, e := range entries {
		bar := strings.Repeat(string(fill), int(e.pct/100*float64(barW)+0.5))
		key := trimMiddle(e.key.String(), 40) // fixed key width
		fmt.Printf("%-*s  %5.1f%%  %s\n", barW, bar, e.pct, key)
	}
}
