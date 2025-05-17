package chart

import (
	"testing"

	"github.com/hayeah/fork2/internal/assert"
	"github.com/hayeah/fork2/internal/metrics"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func constantTermWidth(cols int) func() int { return func() int { return cols } }

// Creates an OutputMetrics with three files + one template bucket:
//
//	a/big.go  : 900 tokens
//	a/small.go:  20 tokens
//	x/y/z.go  :  50 tokens
//	<template>:  30 tokens
//
// → total files = 3, total tokens = 1 000
func fakeMetrics() *metrics.OutputMetrics {
	m := &metrics.OutputMetrics{
		Items: map[metrics.MetricKey]metrics.MetricItem{},
	}

	// helper
	add := func(typ, key string, tokens int) {
		m.Items[metrics.NewKey(typ, key)] = metrics.MetricItem{Tokens: tokens}
	}
	add("file", "a/big.go", 900)
	add("file", "a/small.go", 20)
	add("file", "x/y/z.go", 50)
	add("template", "system", 30)
	return m
}

// ─────────────────────────────────────────────────────────────────────────────
// ❶ collectFileTokens
// ─────────────────────────────────────────────────────────────────────────────

func TestCollectFileTokens(t *testing.T) {
	ass := assert.New(t)
	files, total, fileCnt := collectFileTokens(fakeMetrics())

	ass.Equal(3, fileCnt, "file count mismatch")
	ass.Equal(1000, total, "total tokens in files mismatch") // excludes template
	ass.Equal(3, len(files), "unexpected slice length")

	// ensure we got the correct file names
	var gotPaths []string
	for _, f := range files {
		gotPaths = append(gotPaths, f.Path)
	}
	ass.ElementsMatch(
		[]string{"a/big.go", "a/small.go", "x/y/z.go"},
		gotPaths,
		"unexpected file list",
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// ❷ buildDirTree + roll-up
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildDirTreeRollUp(t *testing.T) {
	ass := assert.New(t)

	files := []fileToken{
		{"a/big.go", 900},
		{"a/small.go", 20},
		{"x/y/z.go", 50},
	}
	root := buildDirTree(files)

	// root tokens should be the sum of all children
	ass.Equal(970, root.Tokens, "root token roll-up failed")

	// spot-check a branch
	aDir := root.Children["a"]
	ass.NotNil(aDir)
	ass.Equal(920, aDir.Tokens, "`a` dir roll-up failed")
}

// ─────────────────────────────────────────────────────────────────────────────
// ❸ collapseSmallDirs
// ─────────────────────────────────────────────────────────────────────────────

func TestCollapseSmallDirsThreshold(t *testing.T) {
	ass := assert.New(t)

	files := []fileToken{
		{"a/big.go", 900},
		{"a/small.go", 20},
		{"x/y/z.go", 50},
	}
	root := buildDirTree(files)
	buckets := collapseSmallDirs(root, 970 /*total*/, 5 /*pct*/)

	var labels []string
	for _, b := range buckets {
		labels = append(labels, b.Label)
	}
	want := []string{"a/big.go", "a/**", "x/y/z.go"}

	ass.Equal(len(want), len(buckets), "bucket count mismatch")
	ass.ElementsMatch(want, labels, "unexpected bucket labels")
}

// ─────────────────────────────────────────────────────────────────────────────
// ❺ layoutChart (light property checks)
// ─────────────────────────────────────────────────────────────────────────────

func TestLayoutChartProperties(t *testing.T) {
	ass := assert.New(t)

	entries := []entry{
		{Label: "a/big.go", Tokens: 900, Pct: 90},
		{Label: "a/**", Tokens: 20, Pct: 2},
		{Label: "x/y/z.go", Tokens: 50, Pct: 5},
	}
	opt := Options{
		BarWidth:  20,
		FillRune:  '#',
		TermWidth: constantTermWidth(80),
	}
	lines := layoutChart(entries, 1000, 3, opt)

	ass.Equal(5, len(lines), "should emit 3 bars + total + summary")
	ass.Contains(lines[0], "#", "bar chars missing")
	ass.Contains(lines[len(lines)-1], "Summary:", "summary line missing")
}
