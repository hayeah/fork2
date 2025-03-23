package heredoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Scan parses a Command's parameters into a struct using json struct tags.
// The target must be a non-nil pointer to a struct.
func (c *Command) Scan(target interface{}) error {
	// Check if target is valid
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return errors.New("unmarshal target must be a non-nil pointer")
	}

	// Get the struct value
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("unmarshal target must be a pointer to struct, got %s", v.Kind())
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get the json tag
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}

		// Split the tag to get the name and options
		parts := strings.Split(tag, ",")
		name := parts[0]

		// Find the parameter with this name
		param := c.GetParam(name)
		if param == nil {
			// If the field is required, return an error
			if len(parts) > 1 && strings.Contains(parts[1], "required") {
				return fmt.Errorf("required parameter %s not found", name)
			}
			// Otherwise, skip this field
			continue
		}

		// Get the field value
		fieldVal := v.Field(i)
		if !fieldVal.CanSet() {
			return fmt.Errorf("cannot set field %s", field.Name)
		}

		// Set the field value based on its type
		if err := setFieldValue(fieldVal, param.Payload); err != nil {
			return fmt.Errorf("failed to set field %s: %v", field.Name, err)
		}
	}

	return nil
}

// setFieldValue converts a string value to the appropriate type and sets the field
func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(i)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(f)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	case reflect.Slice:
		// For slices, try to unmarshal as JSON
		slice := reflect.New(field.Type())
		if err := json.Unmarshal([]byte(value), slice.Interface()); err != nil {
			return err
		}
		field.Set(slice.Elem())
	case reflect.Map:
		// For maps, try to unmarshal as JSON
		m := reflect.New(field.Type())
		if err := json.Unmarshal([]byte(value), m.Interface()); err != nil {
			return err
		}
		field.Set(m.Elem())
	case reflect.Struct:
		// For structs, try to unmarshal as JSON
		s := reflect.New(field.Type())
		if err := json.Unmarshal([]byte(value), s.Interface()); err != nil {
			return err
		}
		field.Set(s.Elem())
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}
