package jsonc

// ToJSON removes JavaScript-style comments from a JSONC byte slice.
// This is a minimal implementation sufficient for basic cases.
func ToJSON(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	inSingle := false
	inMulti := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		next := byte(0)
		if i+1 < len(data) {
			next = data[i+1]
		}
		if inSingle {
			if c == '\n' {
				inSingle = false
				out = append(out, c)
			}
			continue
		}
		if inMulti {
			if c == '*' && next == '/' {
				inMulti = false
				i++
			}
			continue
		}
		if !inString && c == '/' && next == '/' {
			inSingle = true
			i++
			continue
		}
		if !inString && c == '/' && next == '*' {
			inMulti = true
			i++
			continue
		}
		if c == '"' && (i == 0 || data[i-1] != '\\') {
			inString = !inString
		}
		out = append(out, c)
	}
	return out
}
