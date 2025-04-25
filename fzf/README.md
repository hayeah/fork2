```markdown
# `fzf` Matcher (package `github.com/hayeah/fork2/fzf`)

A **tiny, deterministic path-filter** inspired by the query language of
[junegunn/fzf](https://github.com/junegunn/fzf) – but built for _static,
predictable_ matches instead of interactive fuzzy finding.

The package is used by the **vibe** CLI inside this repo, yet it is completely
self-contained and can be reused anywhere you need quick, no-dependency
matching of file/URL-like strings.

---

## Quick start

```go
import "github.com/hayeah/fork2/fzf"

// slice of paths you want to filter:
paths := []string{
	"cmd/vibe/select.go",
	"README.md",
	"internal/assert/assert.go",
}

// build a matcher once…
m, err := fzf.NewMatcher("^cmd .go$")
if err != nil { /* handle bad pattern */ }

// …and use it many times
hits, _ := m.Match(paths)
// hits => []string{"cmd/vibe/select.go"}
```

---

## Supported query syntax

*Every query is split on whitespace – each term must match for a path to be
kept (logical **AND**). Matching is **case-insensitive** and all path
separators are normalised to “/”.*

### 1. Plain substring (term `foo`)
* `foo` – keep paths containing “foo” anywhere.

### 2. Anchors (^ and $ in a term)
* `^foo` – term must appear at **start** of the path.
* `bar$` – term must appear at **end** of the path.
* `^foo$` – whole path must equal `foo`.

### 3. Word-boundary operators (leading `'` or wrapping `'...'`)
* `'foo` – **word-prefix** match. `foo` must start at a word boundary
  (start of string or after a non-word rune).
  *Examples:*
  * ✅ `config/select.go` matches pattern `'select`
  * ❌ `unselected.go`   does **not** match
* `'foo'` – **exact-word** match. `foo` must be delimited by word boundaries on
  both sides.
  *Examples:*
  * ✅ `cmd/vibe/select.go` matches pattern `'select'`
  * ❌ `selector.go` does **not** match

> **Word characters** are defined as Unicode letters, digits, or “_”.
> Anything else (`/-.\` etc.) is considered a boundary.

### 4. Multiple terms
* `cmd .go` – keeps paths that contain **both** “cmd” *and* “.go”.

---

## Similarities with *fzf* query language

* **Whitespace-separated terms = AND** – same mental model.
* **`^` and `$` anchors** – behave identically for prefix/suffix/whole-string.
* **Case-insensitive by default** (fzf’s `--ignore-case`).

---

## Key differences from *fzf*

* **Deterministic** – no fuzzy ranking or scoring; either a path matches or it
  doesn’t. This makes results stable in scripts and tests.
* **Exact word operator** – this package introduces
  *word-prefix* (`'foo`) and *exact-word* (`'foo'`) modifiers.
  In upstream *fzf*, a leading single quote disables fuzzy matching; here it
  **enforces word boundaries instead**.
* **Path normalisation is automatic** – Windows back-slashes become “/”.
  Upstream *fzf* treats them literally.
* **No escape syntax** – since the grammar is tiny, queries that consist solely
  of meta characters (`'`, `^`, `$`) are rejected with a helpful error.

---

## Examples

```text
Pattern           Keeps
================= ================================================
""                (all paths – empty query)
cmd               cmd/vibe/select.go, cmd/vibe/ask.go, …
cmd .go           all Go files under cmd/
^README.md$       README.md
^cmd 'select'     only cmd/vibe/select.go
'.txt$            docs/intro.txt
foo               (case-insensitive) matches “foo” or “FOO”
```

---

## Installation

```bash
go get github.com/hayeah/fork2/fzf
```

The package has **zero dependencies** beyond the Go standard library.

---

## Contributing & tests

The matcher is fully covered by unit tests
(`fzf/match_test.go`). Use the usual Go workflows:

```bash
go test ./fzf/...
go vet ./...
```

> Note: Assertions use *stretchr/testify* following the project-wide guidelines
> in `.vibe.md`.

---

## License

[MIT](../LICENSE) (same as the parent repository).
```
