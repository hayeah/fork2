package hujsonutil

import (
	"encoding/json"
	"fmt"

	"github.com/tailscale/hujson"
)

// Value wraps hujson.Value to provide convenience helpers.
type Value struct {
	*hujson.Value
}

// NewValue wraps a hujson.Value.
func NewValue(v *hujson.Value) *Value {
	return &Value{Value: v}
}

// InsertToArray inserts value at the end of the array located at path.
// The path uses JSON Pointer syntax. If the array does not exist, it is created.
// Returns an error if the path points to a non-array value.
func (v *Value) InsertToArray(path string, val any) error {
	if v.Value == nil {
		return fmt.Errorf("nil Value")
	}

	// Marshal the inserted value so we can parse it as HuJSON.
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	elem, err := hujson.Parse(b)
	if err != nil {
		return err
	}

	// See if the array exists.
	existing := v.Find(path)
	if existing != nil {
		if _, ok := existing.Value.(*hujson.Array); !ok {
			return fmt.Errorf("path %s is not an array", path)
		}
		patch := fmt.Sprintf(`[{"op":"add","path":"%s/-","value":%s}]`, path, elem.Pack())
		return v.Patch([]byte(patch))
	}

	// Create the array and insert the element.
	patch := fmt.Sprintf(`[`+
		`{"op":"add","path":"%s","value":[]},`+
		`{"op":"add","path":"%s/-","value":%s}`+
		`]`, path, path, elem.Pack())
	return v.Patch([]byte(patch))
}
