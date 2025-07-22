# Repo Context

Use the background information provided below to help you accomplish the task.

- Repo Directory Tree
- Selected Files

## Repo Directory Tree

{{ .RepoDirectoryTree}}

## Selected Files

{{- range .SelectedPaths }}
@{{ . }}
{{- end }}
