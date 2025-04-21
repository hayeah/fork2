# Vibe: A Simple Prompt Tool for Your Git Workflow

**vibe** is a small CLI tool that helps you:

1. Gather selected files from your repository into a prompt.
2. Copy that prompt to your clipboard and paste it into your LLM of choice.
3. Merge the edits that the LLM suggests back into your local repo automatically.

## Install

```bash
go install github.com/hayeah/fork2/cmd/vibe@latest
```

## Quick Tutorial

The vibe prompt builder has a flexible pattern matching syntax to

Suppose we have a repo with these files

```
my‑repo
├── cmd/
   ├── main.go
   ├── helper.go
   └── helper_test.go
```

Select all Go files, and copy into the clipboard:

```bash
vibe ask --select .go
```

Select all Go files, but filter out the `_test.go` files. What this is doing is to first select all the go files, then negating the pattern `_test.go` by prefixing it with `!`:

```bash
vibe ask --select '.go|!_test.go'
```

You can then copy the copied files as context into ChatGPT or Claude.


## Render Markdown Files as Prompt Templates

`vibe` can also render markdown files as prompt templates. `explain` is a built-in template, which you can use to generate a prompt for explaining code.


```bash
vibe ask explain --select '.go'
```

This builds a prompt that copies all the selected files, structured like this:

```
<directory tree>

<selected file 1>

<selected file 2>

...

# User Task

Give me an overview and walkthrough of the above code.
```

## Features

- **File Selection UI**
  An interactive TUI that lists all files in your repo (respecting `.gitignore`). You can search, toggle directories, or select individual files.

- **Lightweight Prompt Templates**
  Easily embed your selected files into a base layout or a “role” layout (e.g. `<coder>`). The prompt is rendered with a Go‑native templating system, allowing you to embed partials, inject data, and keep it flexible.

- **`.vibe.md` Files for Context**
  Create `.vibe.md` files in your repo directories. **vibe** picks these up (from the root down to your current directory) and includes them automatically in your prompt. This is handy if you have recurring instructions or explanations to pass on to your LLM.

- **Merging LLM Edits**
  The `vibe merge` command reads the LLM’s proposed changes back in “heredoc” format. If everything checks out, **vibe** applies those edits locally to the relevant files.

## Copy‑and‑Paste Workflow

Below is a simple vibe flow:

1. **Choose a Prompt File**
   Prepare a text file with your instructions or discussion context. This file can contain “front matter” (flags or arguments recognized by vibe) at the top if you like.

2. **Pick a Prompt Layout (“role”)**
   By default, vibe uses the bulitin `<coder>` layout. You can specify something else with `--role base` or `--role plan` or `--role writer`.

3. **Run `vibe ask`**
   ```
   vibe ask userPromptFile --copy
   ```
   This generates a fully composed prompt (selected files, `.vibe.md` context, etc.) and copies the text to your clipboard.

4. **Paste into Your LLM**
   Go to your LLM (e.g. Claude, ChatGPT) and paste the entire text. The AI will produce some modifications or suggestions, using a format that can be automatically merged by vibe.

5. **Copy the LLM’s Response**
   Grab its text from the AI’s output into your clipboard.

6. **Run `vibe merge --paste`**
   This reads your clipboard for changes. If the tool recognizes valid changes, it applies them to your local repo. If there are unknown commands or verification errors, vibe shows them so you can inspect or fix them.

7. **Review, Diff, and Test**
   Look at your Git diff, run tests, keep or revert changes. If needed, refine your prompt file or proceed to your next steps.


## The UserPrompt Template

Each “prompt file” can have **front matter** at the top enclosed in `+++ ... +++` or `--- ... ---` lines.

Suppose you have a prompt file: `ask-diff-to-role.md`

```
+++
--select cmd/vibe/ask.go
--copy
+++

- vibe: change `diff` flag to `role`
	- remove `--diff`
	- add the "--role" flag
		- default to coder
	- when rendering in handleOutput, wrap the specified role in "<...>"
- fix tests
```

- **Front Matter**
  The lines between the triple plus or triple dash markers can include flags/arguments you’d normally pass on the CLI: `--select`, `--all`, `--role`, etc.

Try running vibe on the commit `d73eaad` to generate a full prompt, and copy to your clipboard:

```
vibe ask ask-diff-to-role.md
```

Paste into your LLM, and copy the response. Run merge and paste from your clipboard:

```
vibe merge --paste
```

If you are lucky, the changes will apply automatically.

See these files for the relevant input and output:

- examples/ask-diff-to-role.md
- examples/ask-diff-to-role.prompt.md
- examples/ask-diff-to-role.response.md

## Pattern Types

- **Fuzzy Matching** (default)
  Simple text patterns match files using fuzzy search:
  ```
  --select foo      # Matches files containing "foo" anywhere in the path
  ```

- **Regex Patterns**
  Prefix with `/` to use regular expressions:
  ```
  --select "/\.go$"  # Matches files ending with ".go"
  ```

- **Negation Patterns**
  Prefix with `!` to exclude matches:
  ```
  --select "!_test.go"  # Select files not matching "_test.go"
  ```

- **Compound Filtering**
  Use `|` to combine patterns (logical AND):
  ```
  --select "cmd|main.go"  # Files containing both "cmd" AND "main.go"
  ```

- **Relative Paths**

  ```
  --select "./cmd"  # Same as --select cmd
  ```

- **Multiple Patterns**
  Specify multiple `--select` flags to collect together different sets of files:
  ```
  --select "/\.go$" --select "/\.md$"  # All Go and Markdown files
  ```

## Template Partials

- **Partials**
  If you want to bring in partial templates, you can place lines like `{{ partial "<myRole>" }}` (for system templates) or `{{ partial "@repoRoot/whatever" }}` (for repo‑root partials) or `{{ partial "./someLocalPartial" }}` (for local partials in the same directory).
