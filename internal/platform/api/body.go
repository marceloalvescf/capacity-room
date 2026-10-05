package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// MaxBody is the limit on a request body.
const MaxBody = 64 << 10

var unknownField = regexp.MustCompile(`^json: unknown field "(.*)"$`)

// DecodeBody reads at most MaxBody bytes of r and decodes one JSON object
// into v, refusing an unknown key, a repeated key, a wrong type, a missing
// required field, and anything after the object. It returns a Problem
// REQUEST_INVALID whose fields name each problem with missing, invalid,
// unknown, or too_long. required lists the keys that must be present;
// maxLen gives the longest value of a string key. A problem of the body as a
// whole sits under the empty key.
func DecodeBody(r *http.Request, v any, required []string, maxLen map[string]int) *Problem {
	fail := func(fields map[string]string) *Problem {
		return &Problem{Code: CodeRequestInvalid, Fields: fields}
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, MaxBody+1))
	if err != nil {
		return fail(map[string]string{"": "invalid"})
	}
	if len(data) > MaxBody {
		return fail(map[string]string{"": "too_long"})
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return fail(map[string]string{"": "missing"})
	}

	fields := map[string]string{}
	keys, ok := walkObject(data, maxLen, fields)
	if !ok {
		return fail(map[string]string{"": "invalid"})
	}
	for _, key := range required {
		if !slices.Contains(keys, key) {
			fields[key] = "missing"
		}
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var typeErr *json.UnmarshalTypeError
		if m := unknownField.FindStringSubmatch(err.Error()); m != nil {
			fields[m[1]] = "unknown"
		} else if errors.As(err, &typeErr) && typeErr.Field != "" {
			fields[typeErr.Field] = "invalid"
		} else {
			fields[""] = "invalid"
		}
	}
	if len(fields) > 0 {
		return fail(fields)
	}
	return nil
}

// walkObject reads the top level of data token by token. It returns the keys
// present and false when data is not exactly one object. A repeated key is
// recorded as invalid and a string over its limit as too_long.
func walkObject(data []byte, maxLen map[string]int, fields map[string]string) ([]string, bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, false
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, _ := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, false
		}
		if slices.Contains(keys, key) {
			fields[key] = "invalid"
			continue
		}
		keys = append(keys, key)
		if limit, ok := maxLen[key]; ok && strings.HasPrefix(string(raw), `"`) {
			var s string
			if json.Unmarshal(raw, &s) == nil && utf8.RuneCountInString(s) > limit {
				fields[key] = "too_long"
			}
		}
	}
	if tok, err := dec.Token(); err != nil || tok != json.Delim('}') {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return keys, true
}
