---
---
## Data Parameters Test

Model: {{ .Data.model }}
Format: {{ .Data.format }}
Debug: {{ .Data.debug }}

{{ if eq .Data.model "gpt4" }}
Using GPT-4 specific configuration
{{ else }}
Using default configuration
{{ end }}
