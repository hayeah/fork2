---
layout = "base"
select = ".go$ !test"
dirtree = ".go$"
---
## Analysis Request

Please analyze the following Go code:

{{ range .SelectedPaths }}
- {{ . }}
{{ end }}
