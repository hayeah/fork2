# ASCII Token-Count Bar Chart
*Technical design overview for `internal/metrics/chart`*

---

## 1 — Purpose

`chart` turns a `metrics.OutputMetrics` object (token counts gathered elsewhere in the code-base) into a tidy, **self-contained ASCII bar chart**.
All state flows through arguments; no globals, no direct calls to `fmt.Print` or `term.GetSize`, which makes the code easy to test and embed.

---

## 2 — Public API surface

* **`Options`** – user-tunable layout + I/O knobs
  * `BarWidth` (int) hard-set bar width, or `0` to auto-size (≈ 35 % of terminal).
  * `FillRune` (rune) glyph used for the bar (default `█`).
  * `ThresholdPct` (float64) folders below this share-of-total collapse into `dir/**`.
  * `TermWidth` (func() int) callback returning terminal width in columns.
  * `Writer` (io.Writer) where the finished chart is written.

* **`DefaultOptions(termWidthFn, writer)`** – sensible defaults.

* **`Print(metrics, options)`** – *single front door* that:
  1. Collects data,
  2. Builds the chart,
  3. Streams it to `options.Writer`.

---

## 3 — Pipeline (five deterministic steps)

```text
collectFileTokens  ➜  buildDirTree  ➜  collapseSmallDirs  ➜
mergeWithExtraMetrics  ➜  layoutChart  ➜  io.Writer
```

1. **Collect file tokens**
   *Scans `metrics.Items` and returns:*
   * `[]fileToken` – `(Path, Tokens)` for every file,
   * `totalTokens` – sum of all tokens (files + non-file buckets),
   * `fileCount` – number of files only.

2. **Build directory tree** (`dirNode`)
   *Creates an in-memory tree mirroring the repo structure and rolls token counts upward.*

3. **Collapse “small fry”** (`bucket`)
   *Folders where `tokens < total * ThresholdPct / 100` get folded into `dir/**`, reducing noise.*

4. **Merge extra buckets** (`entry`)
   *Adds non-file metrics (e.g. `template:system`, `user`, `final`) so the chart covers **all** token sources.*

5. **Layout chart**
   *Determines column widths, scales bars to the largest bucket, truncates long paths from the left, and formats every row:*

   ```
   <bar><spaces>  <pct>  <tokens>  <path>
   ```

   The function also appends:
   * A separating “TOTAL” bar.
   * A summary line: `Summary: 123 files, 4567 tokens`.

---

## 4 — Key data structures (private)

* `fileToken` `{ Path string; Tokens int }` – flat slice.
* `dirNode` recursive tree `(Name, IsFile, Tokens, Children map)`.
* `bucket` collapsed `(Label, Tokens)`.
* `entry` print-ready `(Label, Tokens, Pct)`.

---

## 5 — Algorithmic complexity

| Phase | Time | Space |
| ----- | ---- | ----- |
| Collect tokens | *O(n)* | *O(f)* |
| Build tree | *O(f · p)*¹ | *O(f · p)* |
| Collapse | *O(nodes)* | *O(nodes)* |
| Merge + layout | *O(buckets)* | *O(buckets)* |

¹ `p` = average path components; typically shallow (< 10).

The pipeline is linear in the number of input files and tree nodes; memory overhead is modest and bounded by the directory structure.

---

## 6 — Sample output (80-column terminal)

```
█                               2.0%      20  a/small.go
█                               3.0%      30  template:system
██                              5.0%      50  x/y/z.go
████████████████████████████   90.0%     900  a/big.go
────────────────────────────  100.0%    1000  TOTAL

Summary: 3 files, 1000 tokens
```

---

## 7 — Customization & extension ideas

* **Change the bar glyph** – set `Options.FillRune = '*'` for a lighter look.
* **Tweak collapse threshold** – raise `ThresholdPct` to keep more tiny paths.
* **Inject alternative width calculators** – for GUI apps pass a stub that always returns a fixed width.
* **Switch to colorised output** – wrap `Print` with your own function that post-processes each line and adds ANSI codes (bars stay ASCII).
