---
select = ".go$"
dirtree = ".go$"
---
## Working Directory
{{ .WorkingDirectory }}

## Selected Files Count
{{ len .SelectedPaths }} files selected

## Has Repo Prompts
{{ if .RepoPrompts }}Has .vibe.md files{{ else }}No .vibe.md files{{ end }}
