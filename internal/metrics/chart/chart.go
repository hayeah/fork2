// Package chart turns OutputMetrics into an ASCII bar chart without
// touching stdout or term.GetSize.  All state is passed in -– easy to test.
package chart

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hayeah/fork2/internal/metrics"
)

// ---------- Public façade --------------------------------------------------

// Options controls layout and I/O behaviour.
type Options struct {
	BarWidth     int        // 0 = auto (35 % of term)
	FillRune     rune       // default '█'
	ThresholdPct float64    // small-dir collapse threshold (e.g. 1 = 1 %)
	TermWidth    func() int // injected; must return columns
	Writer       io.Writer  // destination for the chart
}

// DefaultOptions returns sane defaults that match the old behaviour.
func DefaultOptions(termWidthFn func() int, w io.Writer) Options {
	return Options{
		FillRune:     '█',
		ThresholdPct: 1,
		TermWidth:    termWidthFn,
		Writer:       w,
	}
}

// Print is the single entry-point used by your CLI.
func Print(m *metrics.OutputMetrics, opt Options) error {
	files, total, fileCount := collectFileTokens(m)             // ❶
	root := buildDirTree(files)                                 // ❷
	buckets := collapseSmallDirs(root, total, opt.ThresholdPct) // ❸
	entries := mergeWithExtraMetrics(buckets, m, total)         // ❹
	lines := layoutChart(entries, total, fileCount, opt)        // ❺
	for _, ln := range lines {                                  // ❻
		if _, err := fmt.Fprintln(opt.Writer, ln); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Step ❶: gather per-file tokens -------------------------------

type fileToken struct {
	Path   string
	Tokens int
}

func collectFileTokens(m *metrics.OutputMetrics) ([]fileToken, int, int) {
	m.Wait() // ensure workers finished
	var (
		out         []fileToken
		totalTokens int
		fileCount   int
	)
	for k, v := range m.Items {
		totalTokens += v.Tokens
		if k.Type == "file" {
			out = append(out, fileToken{Path: k.Key, Tokens: v.Tokens})
			fileCount++
		}
	}
	return out, totalTokens, fileCount
}

// ---------- Step ❷: directory tree with cumulative tokens -----------------

type dirNode struct {
	Name     string
	IsFile   bool
	Tokens   int
	Children map[string]*dirNode
}

func buildDirTree(files []fileToken) *dirNode {
	root := &dirNode{Name: ".", Children: map[string]*dirNode{}}
	for _, f := range files {
		parts := strings.Split(filepath.ToSlash(f.Path), "/")
		cur := root
		for i, part := range parts {
			last := i == len(parts)-1
			if cur.Children == nil {
				cur.Children = map[string]*dirNode{}
			}
			if _, ok := cur.Children[part]; !ok {
				cur.Children[part] = &dirNode{
					Name:     part,
					Children: map[string]*dirNode{},
					IsFile:   last,
				}
			}
			cur = cur.Children[part]
		}
		cur.Tokens = f.Tokens
	}
	rollUp(root)
	return root
}

func rollUp(n *dirNode) int {
	if n.IsFile {
		return n.Tokens
	}
	sum := 0
	for _, c := range n.Children {
		sum += rollUp(c)
	}
	n.Tokens = sum
	return sum
}

// ---------- Step ❸: collapse “small fry” dirs into dir/** ------------------

type bucket struct {
	Label  string
	Tokens int
}

func collapseSmallDirs(root *dirNode, total int, thresholdPct float64) []bucket {
	var out []bucket
	var walk func(*dirNode, string)
	thresh := float64(total) * thresholdPct / 100
	walk = func(n *dirNode, path string) {
		cur := path
		if n != root {
			if cur == "" || cur == "." {
				cur = n.Name
			} else {
				cur = filepath.Join(cur, n.Name)
			}
		}
		if n.IsFile {
			out = append(out, bucket{Label: filepath.ToSlash(cur), Tokens: n.Tokens})
			return
		}

		var smallSum int
		for _, c := range n.Children {
			if float64(c.Tokens) < thresh {
				smallSum += c.Tokens
			} else {
				walk(c, cur)
			}
		}
		if smallSum > 0 {
			out = append(out, bucket{
				Label:  filepath.ToSlash(filepath.Join(cur, "**")),
				Tokens: smallSum,
			})
		}
	}
	walk(root, "")
	return out
}

// ---------- Step ❹: merge with template/user/final totals -----------------

type entry struct {
	Label  string
	Tokens int
	Pct    float64
}

func mergeWithExtraMetrics(buckets []bucket, m *metrics.OutputMetrics, total int) []entry {
	var out []entry
	for _, b := range buckets {
		out = append(out, entry{
			Label:  b.Label,
			Tokens: b.Tokens,
			Pct:    pct(b.Tokens, total),
		})
	}
	for k, v := range m.Items {
		if k.Type == "file" {
			continue
		}
		out = append(out, entry{
			Label:  k.String(),
			Tokens: v.Tokens,
			Pct:    pct(v.Tokens, total),
		})
	}
	return out
}

// ---------- Step ❺: convert entries → formatted lines ---------------------

func layoutChart(entries []entry, total, fileCount int, opt Options) []string {
	if len(entries) == 0 {
		return []string{"No tokens recorded"}
	}
	const pctW, tokensW, gapW = 6, 6, 2

	// Sort smallest → largest (like original)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Pct < entries[j].Pct })

	// Infer dynamic widths
	barW := opt.BarWidth
	if barW <= 0 {
		barW = int(float64(opt.TermWidth()) * 0.35)
		barW = min(barW, 30)
	}
	keyW := opt.TermWidth() - (barW + pctW + tokensW + gapW*3)
	if keyW < 8 {
		keyW = 8
	}

	// Largest bucket determines full bar
	maxTokens := 0
	for _, e := range entries {
		if e.Tokens > maxTokens {
			maxTokens = e.Tokens
		}
	}

	trim := func(s string, max int) string {
		if len(s) <= max {
			return s
		}
		return "…" + s[len(s)-max+1:]
	}

	fill := string(opt.FillRune)
	sep := strings.Repeat("─", barW)
	var lines []string

	for _, e := range entries {
		ratio := float64(e.Tokens) / float64(maxTokens)
		barLen := int(ratio*float64(barW) + 0.5)
		if barLen == 0 && e.Tokens > 0 {
			barLen = 1
		}
		bar := strings.Repeat(fill, barLen)
		label := trim(e.Label, keyW)
		lines = append(lines, fmt.Sprintf("%-*s  %5.1f%%  %*d  %-*s",
			barW, bar, e.Pct, tokensW, e.Tokens, keyW, label))
	}

	lines = append(lines, fmt.Sprintf("%-*s  %5.1f%%  %*d  %-*s",
		barW, sep, 100.0, tokensW, total, keyW, "TOTAL"))
	lines = append(lines, fmt.Sprintf("\nSummary: %d files, %d tokens", fileCount, total))

	return lines
}

// ---------- Helpers --------------------------------------------------------

func pct(part, total int) float64 { return float64(part) * 100 / float64(total) }
