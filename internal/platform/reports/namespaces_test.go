package reports

import (
	"reflect"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
)

func TestSyntaxOfNamespaces(t *testing.T) {
	language := "en"
	cases := []struct {
		name       string
		namespaces []string
		want       map[string]string
	}{
		{"null", nil, nil},
		{"empty list", []string{}, map[string]string{"namespaces": "empty"}},
		{"list", []string{"payments"}, nil},
		{"repeated", []string{"a", "a"}, map[string]string{"namespaces": "repeated"}},
		{"invalid", []string{"Bad_NS"}, map[string]string{"namespaces": "invalid"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := checkSyntax(Selection{ProfileID: "prod", Namespaces: c.namespaces, Language: &language})
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("fields = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPermitsNamespaces(t *testing.T) {
	denied := map[string]string{"namespaces": "not_permitted"}
	cases := []struct {
		name       string
		allowed    []string
		namespaces []string
		want       map[string]string
	}{
		{"null and no limit", nil, nil, nil},
		{"null and a limit", []string{"payments"}, nil, nil},
		{"inside a limit", []string{"payments", "billing"}, []string{"billing"}, nil},
		{"outside a limit", []string{"payments"}, []string{"billing"}, denied},
		{"partly outside a limit", []string{"payments"}, []string{"payments", "billing"}, denied},
		{"any list and no limit", []string{}, []string{"billing", "audit"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := profiles.Defaults()
			d.AllowedNamespaces = c.allowed
			got := permits(Selection{Namespaces: c.namespaces}, Resolved{}, d)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("fields = %v, want %v", got, c.want)
			}
		})
	}
}
