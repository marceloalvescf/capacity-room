package api

import (
	"maps"
	"net/http/httptest"
	"strings"
	"testing"
)

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func decode(body string, required []string, maxLen map[string]int) (loginBody, *Problem) {
	var v loginBody
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	return v, DecodeBody(r, &v, required, maxLen)
}

func TestDecodeBodyRefusals(t *testing.T) {
	req := []string{"username", "password"}
	cases := []struct {
		name   string
		body   string
		maxLen map[string]int
		want   map[string]string
	}{
		{"unknown key", `{"username":"a","password":"b","extra":1}`, nil, map[string]string{"extra": "unknown"}},
		{"repeated key", `{"username":"a","username":"b","password":"c"}`, nil, map[string]string{"username": "invalid"}},
		{"repeated key by case", `{"username":"a","Username":"b","password":"c"}`, nil, map[string]string{"username": "invalid"}},
		{"string too long by case", `{"Username":"abcd","password":"b"}`, map[string]int{"username": 3}, map[string]string{"username": "too_long"}},
		{"wrong type", `{"username":1,"password":"b"}`, nil, map[string]string{"username": "invalid"}},
		{"missing", `{"username":"a"}`, nil, map[string]string{"password": "missing"}},
		{"two documents", `{} {}`, nil, map[string]string{"": "invalid"}},
		{"too long body", `{"username":"` + strings.Repeat("a", 65<<10) + `"}`, nil, map[string]string{"": "too_long"}},
		{"empty", ``, nil, map[string]string{"": "missing"}},
		{"null", `null`, nil, map[string]string{"": "invalid"}},
		{"string too long", `{"username":"abcd","password":"b"}`, map[string]int{"username": 3}, map[string]string{"username": "too_long"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, p := decode(c.body, req, c.maxLen)
			if p == nil || p.Code != CodeRequestInvalid {
				t.Fatalf("problem %v", p)
			}
			if !maps.Equal(p.Fields, c.want) {
				t.Fatalf("fields %v, want %v", p.Fields, c.want)
			}
		})
	}
}

func TestDecodeBodyAccepts(t *testing.T) {
	v, p := decode(`{"username":"alice","password":"secret"}`, []string{"username", "password"}, map[string]int{"username": 10})
	if p != nil {
		t.Fatalf("problem %v", p)
	}
	if v.Username != "alice" || v.Password != "secret" {
		t.Fatalf("got %+v", v)
	}
}
