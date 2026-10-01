package snapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// The kinds a Value can carry.
const (
	kindNumber  = "number"
	kindBoolean = "boolean"
	kindText    = "text"
)

// valueType is the type the key walk treats as one leaf that may hold a
// JSON number, boolean, or string.
var valueType = reflect.TypeOf(Value{})

// MarshalJSON writes the value as the JSON number, boolean, or string its
// kind names.
func (v Value) MarshalJSON() ([]byte, error) {
	switch v.Kind {
	case kindNumber:
		return json.Marshal(v.Number)
	case kindBoolean:
		return json.Marshal(v.Boolean)
	case kindText:
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(v.Text); err != nil {
			return nil, err
		}
		return bytes.TrimRight(buf.Bytes(), "\n"), nil
	}
	return nil, fmt.Errorf("snapshot value has an unknown kind: %q", v.Kind)
}

// UnmarshalJSON reads a JSON number, boolean, or string. It refuses null,
// an object, and a list.
func (v *Value) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	switch x := raw.(type) {
	case float64:
		*v = Value{Kind: kindNumber, Number: x}
	case bool:
		*v = Value{Kind: kindBoolean, Boolean: x}
	case string:
		*v = Value{Kind: kindText, Text: x}
	default:
		return errors.New("snapshot value is not a number, a boolean, or a text")
	}
	return nil
}

// checkValue refuses a value the snapshot cannot write: a number that is
// NaN or infinite, an empty text, or a kind that is none of the three.
func checkValue(v Value) error {
	switch v.Kind {
	case kindNumber:
		if !isFinite(v.Number) {
			return errors.New("is not a finite number")
		}
	case kindBoolean:
	case kindText:
		if v.Text == "" {
			return errors.New("is an empty text")
		}
	default:
		return fmt.Errorf("has an unknown kind: %q", v.Kind)
	}
	return nil
}
