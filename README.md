# Vibe: A Simple Tool for Building Complex Prompts

- [Install](#install)
- [Pattern Matching Tutorial](#pattern-matching-tutorial)
  - [Basic patterns](#basic-patterns)
  - [Operators](#operators)
  - [Worked Examples](#worked-examples)
- [List Matched Files](#list-matched-files)
- [Prompt Templates](#prompt-templates)
  - [How rendering works](#how-rendering-works)
  - [Example Chat Log](#example-chat-log)
- [Layout as Wrapper](#layout-as-wrapper)
- [Prompt Lookup Paths](#prompt-lookup-paths)
- [Builtin Prompts](#builtin-prompts)

`vibe` is a CLI that helps you build complex prompts by selecting repo files, and rendering reuseable prompt templates.

- **File selection** – match any subset of your repo with a mini pattern language (e.g. `.go !_test`). Inspired by tools like [repomix](https://github.com/yamadashy/repomix) and [Repo Prompt](https://repoprompt.com/).
- **Composable templates** – layouts and partials let you wrap prefixes/suffixes, inject roles or tool specs, and branch on env/CLI vars. Think a blog engine like Hugo/Jekyll, but for prompts.
- **Shareable workflows** – keep prompt recipes in‑repo so the whole team can reuse and version them.

## Install

```bash
go install github.com/hayeah/fork2/cmd/vibe@latest
```

## Pattern Matching Tutorial

vibe uses a tiny pattern language inspired by fzf to tell which paths you want and which you don’t.

*Every matching query is split on whitespace – each term must match for a path to be
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

* `cmd .go` – keeps paths that contain **both** "cmd" *and* ".go".

### 5. Operators

* **Negation** (`!`) – Exclude paths matching a pattern. Example: `!test` excludes paths containing "test".
* **Union** (`;`) – Combine multiple patterns with OR logic. Example: `.go;.md` matches Go files OR Markdown files.
* **Compound** (`|`) – Apply filters to previous results like a pipe. Example: `.go;.md | !test` first matches all Go OR Markdown files, then filters out any containing "test".

### Examples

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

- **Fuzzy / substring match**: `.go`, `util`
- **Anchors** – `^cmd`, `.go$`, `^README.md$`
- **Word-boundaries** – `'select` (word-prefix) · `'select'` (exact word)
- **Negation inside a term** – `!_test.go`
- **Union** – `.go;.md`

Examples:

- `.go`
  Select every file whose path **contains** `.go`.

- `!.md`
  Exclude markdown files.

- `.go util`
  Select `.go` files that also matches `util`

- `.go !_test.go`
  Select `.go` files **but not** their tests.

- `.go;.md`
  All Go files **or** plus all `.md` files.

- `.go !_test.go;.md`
  All non test `.go` files, plus `.md` files

- `cmd/vibe .go;render .go | !test`
  All `.go` files from `cmd/vibe` OR `render` directories, then exclude any containing "test"

#### Worked Examples

Show me every Go file in the repo

```bash
vibe out --select .go
```

Study utilities only

```bash
vibe out --select '.go util'
# Meaning: “choose files that end in .go AND include ‘util’
```

Show me every Go file in the repo, but leave out any test files

```bash
vibe out --select '.go !_test.go'
# similar to using inverted grep to filter:
# git ls-files '*.go' | grep -v '_test.go'
```

Grab Markdown plus Go code

```bash
vibe out --select '.go;.md'
```

Let's combine everything. Select all go files that are not test files, plus all markdown files:

```bash
vibe out --select '.go !_test.go;.md'
```

Instead of using `;`, you could also put multiple patterns on different lines:

```bash
vibe out --select '.go
!_test.go;.md'
```

Use compound patterns to filter union results:

```bash
# Select Go files from two directories, then exclude test files
vibe out --select 'cmd/vibe .go;render .go | !test'

# This is equivalent to:
# 1. First: match all .go files in cmd/vibe OR all .go files in render
# 2. Then: filter out any files containing "test" from the combined results
```

## List Matched Files

The `ls` command allows you to list files that match a pattern without generating a prompt. This is useful for inspecting which files would be included in a prompt before running the `out` command.

```bash
# List all Go files
vibe ls --select '.go'

# List files from a template's select pattern
vibe ls templates/my_prompt.md

# List files using a mode-specific template
vibe ls --mode=cc templates/my_prompt.md
```

When using a template file, the `ls` command will use the `select` pattern defined in the template's front-matter.

## Prompt Templates

Prompt templates turn vibe into a tiny static‑site generator—except the "pages" it builds are prompts instead of HTML. Each template is just a text or markdown file that contains two distinct parts:

Front‑matter (TOML) – establishes how the template is rendered. The block is enclosed in a fenced code block at the very start of the file.

````md
```
layout = "files"
```

Give me an overview and walkthrough of the above code.
````

The template body is normal text/markdown that may use Go text/template syntax
`{{ ... }}` to reference data that vibe makes available at render time.

### Template Specialization with Mode

The `--mode/-m` flag allows you to create specialized versions of templates for different contexts. When a mode is specified, vibe will first look for a mode-specific variant of the template before falling back to the default.

```bash
# Uses files.cc.md if it exists, otherwise files.md
vibe out --mode=cc files

# Uses analyze.gpt4.md if it exists, otherwise analyze.md
vibe out --mode=gpt4 analyze
```

Mode variants are created by inserting the mode name before the file extension:
- `files.md` → `files.cc.md` (for `--mode=cc`)
- `explain.md` → `explain.gpt4.md` (for `--mode=gpt4`)

This feature is useful for:
- Tool-specific templates (e.g., templates optimized for Claude Code vs ChatGPT)
- Model-specific variations (e.g., different instructions for GPT-4 vs Claude)
- Environment-specific templates (e.g., development vs production)

The mode applies to all template resolution including layouts and partials, allowing you to create complete template sets for different contexts.

### How rendering works

When you run

```bash
vibe out explain.md --select '.go !_test.go'
```

vibe:

1. **Collects source files** that match the pattern.
2. **Builds a data object** containing things like:
   - `.RepoDirectoryTree` – a pretty `tree` ‑style listing
   - `.FileMap` – a collapsed or full listing of the selected files
   - `.RepoPrompts` – any repo‑wide instructions (`*.prompt.md` in the root)
   - `.WorkingDirectory` – the directory `vibe` was run from
   - Environment variables (`.Env`)
   - CLI variables (`--var key=value` → `.Vars.key`)
3. **Parses _explain.md_** as a Go template, plugging the data into the placeholders.
4. **Applies the chosen layout** (see next section) to wrap the result.

Because it is plain Go templating, you can:

- **Branch** on values:
  ```md
  {{ if eq .Env.CI "true" -}}
  _Running inside CI – keep the answer short_
  {{- end }}
  ```

### Example chat log

1. **Repository in focus** – [LibraDB](https://github.com/amit-davidson/LibraDB), a minimalist key‑value store implemented in roughly **1 k LOC of Go**.
2. **Prompt generated by `vibe out`** – see the exact prompt here: <https://gist.github.com/hayeah/82043a1ef35a4cd0b09c58a0a351f2fe>.
3. **ChatGPT o3 response** – the model’s full answer to that prompt: <https://chatgpt.com/share/6807af90-8208-800e-baac-2b4bc3b4b461>.
4. **Reference for comparison** – the original LibraDB deep‑dive blog post: <https://medium.com/better-programming/build-a-nosql-database-from-the-scratch-in-1000-lines-of-code-8ed1c15ed924>.

## Layout as Wrapper

Where templates define **what** you want to say, **layouts** define **how** you surround that content. Think of a layout as the `<html><head><body>` of a prompt.

A layout is itself a Go template that _must_ contain a single placeholder named `.Content` where the inner template will be injected:

```md
<!-- layouts/files.md -->

Use the background information below to help you accomplish the task.

- **Repo Directory Tree**
- **Selected Files**

## Repo Directory Tree

{{ .RepoDirectoryTree }}

## Selected Files

{{ .FileMap }}

{{- if .RepoPrompts }}

## Repo‑Wide Instructions

{{ .RepoPrompts }}
{{- end }}

---

## User Task

{{ .Content }}

<!-- Optionally finish with reminders, tools spec, etc. -->
```

Anything outside `.Content` is a **wrapper** that you can standardise across an organisation. Typical uses:

- **Role instructions** up top (“You are an expert Go reviewer …”).
- **Context** in the middle (directory tree, diff stats, benchmarks).
- **Safety rails** at the bottom (response format, length limit, tool usage).

## Template Data

You can pass key-value pairs to your templates using the `-d/--data` flag. These values are accessible in your templates via the `.Data` map:

```bash
# Pass individual key-value pairs
vibe out -d model=gpt4 -d format=json myPrompt.md

# Or use URL-style query parameters
vibe out -d "model=gpt4&format=json" myPrompt.md
```

In your templates, access these values using the `.Data` map:

```md
<!-- Use specific instructions based on model -->
{{- if eq .Data.model "gpt4" }}
You are using GPT-4. Please provide a detailed analysis.
{{- else }}
Please provide a concise summary.
{{- end }}

<!-- Format output based on user preference -->
{{- if eq .Data.format "json" }}
Return your response in JSON format.
{{- end }}
```

This feature is useful for creating templates that can adapt based on runtime parameters without modifying the template itself.

## Prompt Lookup Paths

When finding a template to render, `vibe` searches through multiple locations in a specific priority order:

1. **Current Repository**: Templates in the repository where the command is running have the highest priority. This allows project-specific templates to override any others.

2. **`$VIBE_PROMPTS` Environment Variable**: A colon-separated list of absolute paths (similar to the `PATH` environment variable). For example:
   ```bash
   # Add multiple template directories
   export VIBE_PROMPTS="/path/to/templates:/another/path/to/templates"
   ```
   This is useful for sharing templates across multiple projects or teams without modifying each repository.

3. **User's `~/.vibe` Directory**: Personal templates stored in your home directory. This is a good place for templates you use across different projects.

4. **Built-in System Prompts**: Default templates that come with `vibe`. These have the lowest priority and will only be used if no matching template is found in the other locations.

This precedence order allows for flexible template overriding. For example, you could have a base template in the system prompts, customize it in your `~/.vibe` directory, and then further customize it for specific projects or teams.

## Builtin Prompts

The `explain.md` example is already built-in as a system prompt. You can invoke it:

```bash
vibe out explain --select '.go'
```
