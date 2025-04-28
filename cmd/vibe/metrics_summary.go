package main

import (
	"os"

	"github.com/hayeah/fork2/internal/metrics"
	"github.com/hayeah/fork2/internal/metrics/chart"
	"golang.org/x/term"
)

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// PrintTokenBreakdown keeps its old signature so no other call-sites change.
func PrintTokenBreakdown(m *metrics.OutputMetrics) error {
	opt := chart.DefaultOptions(termWidth, os.Stdout)
	return chart.Print(m, opt)
}
