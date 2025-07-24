---
select = ".go$"
---
# Go Files

{{ range .SelectedPaths }}
- {{ . }}
{{ end }}
