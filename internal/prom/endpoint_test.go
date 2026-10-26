package prom

import (
	"reflect"
	"testing"
)

// TestCandidates covers every shape of address this package resolves: the API
// roots to try, in order.
func TestCandidates(t *testing.T) {
	cases := []struct {
		name      string
		address   string
		wantRoots []string
	}{
		{
			name:      "direct prometheus base",
			address:   "https://p.example",
			wantRoots: []string{"https://p.example", "https://p.example/api/v1"},
		},
		{
			name:      "trailing slash on the base",
			address:   "https://p.example/",
			wantRoots: []string{"https://p.example", "https://p.example/api/v1"},
		},
		{
			name:      "complete api root",
			address:   "https://p.example/api/v1",
			wantRoots: []string{"https://p.example/api/v1"},
		},
		{
			name:      "complete api root with a trailing slash",
			address:   "https://p.example/api/v1/",
			wantRoots: []string{"https://p.example/api/v1"},
		},
		{
			name:      "two trailing slashes on the base",
			address:   "https://p.example//",
			wantRoots: []string{"https://p.example", "https://p.example/api/v1"},
		},
		{
			name:      "complete api root with two trailing slashes",
			address:   "https://p.example/api/v1//",
			wantRoots: []string{"https://p.example/api/v1"},
		},
		{
			name:    "two slashes in the middle of the path stay",
			address: "https://p.example/a//b/",
			wantRoots: []string{
				"https://p.example/a//b",
				"https://p.example/a//b/api/v1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if roots := Candidates(tc.address); !reflect.DeepEqual(roots, tc.wantRoots) {
				t.Errorf("roots = %#v, want %#v", roots, tc.wantRoots)
			}
		})
	}
}

// TestCandidatesIgnoresGrafanaPaths covers a path that names a datasource or
// a proxy: it gets the roots of any other path.
func TestCandidatesIgnoresGrafanaPaths(t *testing.T) {
	cases := []struct {
		name      string
		address   string
		wantRoots []string
	}{
		{
			name:    "datasource path",
			address: "https://g.example/api/datasources/uid/x/resources",
			wantRoots: []string{
				"https://g.example/api/datasources/uid/x/resources",
				"https://g.example/api/datasources/uid/x/resources/api/v1",
			},
		},
		{
			name:    "proxy path",
			address: "https://g.example/api/datasources/proxy/1",
			wantRoots: []string{
				"https://g.example/api/datasources/proxy/1",
				"https://g.example/api/datasources/proxy/1/api/v1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if roots := Candidates(tc.address); !reflect.DeepEqual(roots, tc.wantRoots) {
				t.Errorf("roots = %#v, want %#v", roots, tc.wantRoots)
			}
		})
	}
}
