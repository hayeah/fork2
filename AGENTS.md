# Wire Dependency Injection

- DO NOT edit wire_gen.go directly
- In the project root, you can run `make wire` to generate wire_gen.go
- If you change providers, always run `make wire` to update wire_gen.go
- Avoid having a provider that provides a native base golang type like `string` or `int`.
  - Define a custom type, and provide that. i.e. `type MyConfigString string`

Or if you prefer, yo ucan run wire to generate the injection code manually:

```bash
go run github.com/google/wire/cmd/wire ./cmd/vibe
```
