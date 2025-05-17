package hujsonutil

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tailscale/hujson"
)

func TestInsertToArrayCreate(t *testing.T) {
	assert := assert.New(t)

	v, err := hujson.Parse([]byte(`{"foo":"bar"}`))
	assert.NoError(err)

	w := NewValue(&v)
	err = w.InsertToArray("/items", 1)
	assert.NoError(err)

	vStd := w.Clone()
	vStd.Standardize()

	var obj map[string]interface{}
	err = json.Unmarshal(vStd.Pack(), &obj)
	assert.NoError(err)

	arr, ok := obj["items"].([]interface{})
	assert.True(ok)
	assert.Equal([]interface{}{float64(1)}, arr)
}

func TestInsertToArrayAppend(t *testing.T) {
	assert := assert.New(t)

	v, err := hujson.Parse([]byte(`{"items":[1]}`))
	assert.NoError(err)

	w := NewValue(&v)
	err = w.InsertToArray("/items", 2)
	assert.NoError(err)

	vStd := w.Clone()
	vStd.Standardize()
	var obj map[string]interface{}
	err = json.Unmarshal(vStd.Pack(), &obj)
	assert.NoError(err)

	arr, ok := obj["items"].([]interface{})
	assert.True(ok)
	assert.Equal([]interface{}{float64(1), float64(2)}, arr)
}

func TestInsertToArrayNested(t *testing.T) {
	assert := assert.New(t)

	v, err := hujson.Parse([]byte(`{"a":{}}`))
	assert.NoError(err)

	w := NewValue(&v)
	err = w.InsertToArray("/a/b", "x")
	assert.NoError(err)

	vStd := w.Clone()
	vStd.Standardize()
	var obj map[string]interface{}
	err = json.Unmarshal(vStd.Pack(), &obj)
	assert.NoError(err)

	a := obj["a"].(map[string]interface{})
	arr, ok := a["b"].([]interface{})
	assert.True(ok)
	assert.Equal([]interface{}{"x"}, arr)
}

func TestInsertToArrayNonArray(t *testing.T) {
	assert := assert.New(t)

	v, err := hujson.Parse([]byte(`{"items": {}}`))
	assert.NoError(err)

	w := NewValue(&v)
	err = w.InsertToArray("/items", 1)
	assert.Error(err)
}
