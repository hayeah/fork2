---
select = ".go$"
dirtree = ".go$"
---
## Test All Helpers

### Working Directory
{{ .WorkingDirectory }}

### Selected Files ({{ len .SelectedPaths }} files)
{{ range .SelectedPaths }}
- {{ . }}
{{ end }}

### Directory Tree
{{ .RepoDirectoryTree }}

### File Contents
{{ .FileMap }}

### Repo Instructions
{{ .RepoPrompts }}
