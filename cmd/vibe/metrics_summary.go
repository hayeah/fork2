package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// PrintTokenBreakdown prints a bar chart of token use.
// Any directory’s *children* whose combined tokens are < 5 % of the repo
// are collapsed into a single “dir/**” bucket.
func PrintTokenBreakdown(m *metrics.OutputMetrics, barW int, fill rune) {
	//----------------------------------------------------------------------
	// ❶  Gather per-file metrics
	//----------------------------------------------------------------------
	type filePair struct {
		path   string
		tokens int
	}

	m.Wait() // ensure workers are done

	var (
		files       []filePair
		totalTokens int
		fileCount   int
	)
	for k, v := range m.Items {
		totalTokens += v.Tokens
		if k.Type == "file" {
			files = append(files, filePair{path: k.Key, tokens: v.Tokens})
			fileCount++
		}
	}
	if totalTokens == 0 {
		fmt.Println("No tokens recorded")
		return
	}

	//----------------------------------------------------------------------
	// ❷  Build a directory tree with cumulative token counts
	//----------------------------------------------------------------------
	type node struct {
		name     string
		isFile   bool
		tokens   int
		children map[string]*node
	}
	root := &node{name: ".", children: map[string]*node{}}

	for _, p := range files {
		parts := strings.Split(filepath.ToSlash(p.path), "/")
		cur := root
		for i, part := range parts {
			last := i == len(parts)-1
			if last {
				cur.children[part] = &node{name: part, isFile: true, tokens: p.tokens}
				continue
			}
			if cur.children == nil {
				cur.children = map[string]*node{}
			}
			if _, ok := cur.children[part]; !ok {
				cur.children[part] = &node{name: part, children: map[string]*node{}}
			}
			cur = cur.children[part]
		}
	}

	var rollUp func(*node) int
	rollUp = func(n *node) int {
		if n.isFile {
			return n.tokens
		}
		sum := 0
		for _, c := range n.children {
			sum += rollUp(c)
		}
		n.tokens = sum
		return sum
	}
	rollUp(root)

	//----------------------------------------------------------------------
	// ❸  Collapse “small fry” into dir/** buckets            ← NEW LOGIC
	//----------------------------------------------------------------------
	const thresh = 0.01

	type bucket struct {
		name   string
		tokens int
	}
	var buckets []bucket

	var collect func(n *node, path string)
	collect = func(n *node, path string) {
		// Build the current path
		cur := path
		if n != root {
			if cur == "" || cur == "." {
				cur = n.name
			} else {
				cur = filepath.Join(cur, n.name)
			}
		}

		if n.isFile {
			// A file is always emitted by its parent’s logic (see below)
			buckets = append(buckets, bucket{filepath.ToSlash(cur), n.tokens})
			return
		}

		var smallSum int
		for _, c := range n.children {
			if float64(c.tokens) < float64(totalTokens)*thresh {
				smallSum += c.tokens
			} else {
				collect(c, cur)
			}
		}
		if smallSum > 0 {
			label := filepath.ToSlash(filepath.Join(cur, "**"))
			buckets = append(buckets, bucket{label, smallSum})
		}
	}
	collect(root, "")

	//----------------------------------------------------------------------
	// ❹  Merge with any template/user/final metrics
	//----------------------------------------------------------------------
	type entry struct {
		key    metrics.MetricKey
		tokens int
		pct    float64
	}
	entries := make([]entry, 0, len(buckets)+len(m.Items))

	for _, b := range buckets {
		k := metrics.NewKey("file", b.name)
		pct := float64(b.tokens) * 100 / float64(totalTokens)
		entries = append(entries, entry{k, b.tokens, pct})
	}
	for k, v := range m.Items {
		if k.Type == "file" {
			continue
		}
		pct := float64(v.Tokens) * 100 / float64(totalTokens)
		entries = append(entries, entry{k, v.Tokens, pct})
	}

	//----------------------------------------------------------------------
	// ❺  Layout & print (unchanged)
	//----------------------------------------------------------------------
	const (
		pctW, tokensW, gapW = 6, 6, 2
	)
	if barW <= 0 {
		barW = int(float64(termWidth()) * 0.35)
	}
	keyW := termWidth() - (barW + pctW + tokensW + gapW*3)
	if keyW < 8 {
		keyW = 8
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].pct < entries[j].pct })

	maxTokens := 0
	for _, e := range entries {
		if e.tokens > maxTokens {
			maxTokens = e.tokens
		}
	}

	for _, e := range entries {
		ratio := float64(e.tokens) / float64(maxTokens)
		barLen := int(ratio*float64(barW) + 0.5)
		if barLen == 0 && e.tokens > 0 {
			barLen = 1
		}
		bar := strings.Repeat(string(fill), barLen)
		key := trimPrefix(e.key.String(), keyW)
		fmt.Printf("%-*s  %5.1f%%  %*d  %-*s\n",
			barW, bar, e.pct, tokensW, e.tokens, keyW, key)
	}

	sep := strings.Repeat("─", barW)
	fmt.Printf("%-*s  %5.1f%%  %*d  %-*s\n",
		barW, sep, 100.0, tokensW, totalTokens, keyW, "TOTAL")
	fmt.Printf("\nSummary: %d files, %d tokens\n", fileCount, totalTokens)
}
