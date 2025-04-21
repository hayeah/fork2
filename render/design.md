# Prompt Template Language Specification

## Overview

This document defines a Markdown-based Prompt Template Language tailored for clarity, simplicity, and Golang-native integration. It supports partials for reusable components, dynamic data embedding, and clear path semantics.

## Key Features

- **Markdown-based**: Leverages Markdown syntax for readability.
- **Golang Templates**: Built upon Go's standard template system, avoiding unnecessary reinvention.
- **Dynamic Data**: Supports embedding dynamic data through standard Go template variables (e.g., `{{ .VariableName }}`).
- **Custom Layouts**: Allows users to define custom layouts.
- **Partials**: Enables modular template inclusion using clearly distinguished partial types.

## Partials Directive

### Syntax

```go
type PartialPath string

// Syntax types:
PartialPath = "<system/partial>" | "@repo_root/partial" | "./local/partial"
```

### Path Types

- **System Template** (`< >`):
  ```markdown
  {{ partial "<vibe/coder>" }}
  ```
  - Always references built-in system-provided partials.

- **Repo Root Template** (`@`):
  ```markdown
  {{ partial "@common/header" }}
  ```
  - Always references partials relative to the repository root.

- **Local Template** (`./`):
  ```markdown
  {{ partial "./helpers/buttons" }}
  ```
  - Always references partials relative to the current template file.

## Layout Structure

The system prompt itself is defined as a layout. Users may create their own layouts, which may reference partials.

### Example Layout

```markdown
{{ partial "<vibe/coder>" }}

# Tools

{{ partial "@vibe/tools/editing" }}

{{ partial "@vibe/tools/git" }}

# Directory Listing

{{ .ListDirectory }}

# Selected Files

{{ .SelectedFiles }}

# Repo Instructions

{{ .RepoInstructions }}

# Env

{{ .System }}

{{ .Now }}

# User Instructions

{{ block "main" . }}
```

## Parsing Rules

- System templates (`< >`) have highest precedence and do not conflict with repo-root or local paths.
- Repo-root templates (`@`) explicitly reference the repository root directory.
- Local templates (`./`) explicitly reference the directory relative to the current template.
