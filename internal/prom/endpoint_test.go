package prom

import (
	"reflect"
	"testing"
)

// TestCandidates covers every row of the brief's table of roots: the API
// roots to try, in order, and the endpoint kind.
func TestCandidates(t *testing.T) {
	cases := []struct {
		name         string
		address      string
		endpointType string
		wantRoots    []string
		wantKind     string
	}{
		{
			name:         "direct prometheus base",
			address:      "https://p.example",
			endpointType: "prometheus",
			wantRoots:    []string{"https://p.example", "https://p.example/api/v1"},
			wantKind:     "prometheus",
		},
		{
			name:         "trailing slash on the base",
			address:      "https://p.example/",
			endpointType: "auto",
			wantRoots:    []string{"https://p.example", "https://p.example/api/v1"},
			wantKind:     "prometheus",
		},
		{
			name:         "complete api root",
			address:      "https://p.example/api/v1",
			endpointType: "auto",
			wantRoots:    []string{"https://p.example/api/v1"},
			wantKind:     "prometheus",
		},
		{
			name:         "complete api root with a trailing slash",
			address:      "https://p.example/api/v1/",
			endpointType: "auto",
			wantRoots:    []string{"https://p.example/api/v1"},
			wantKind:     "prometheus",
		},
		{
			name:         "grafana datasource resource proxy",
			address:      "https://g.example/api/datasources/uid/x/resources/api/v1",
			endpointType: "auto",
			wantRoots:    []string{"https://g.example/api/datasources/uid/x/resources/api/v1"},
			wantKind:     "grafana",
		},
		{
			name:         "grafana proxy id",
			address:      "https://g.example/api/datasources/proxy/1",
			endpointType: "auto",
			wantRoots: []string{
				"https://g.example/api/datasources/proxy/1",
				"https://g.example/api/datasources/proxy/1/api/v1",
			},
			wantKind: "grafana",
		},
		{
			name:         "path merely containing proxy is not auto-detected",
			address:      "https://g.example/myproxything",
			endpointType: "auto",
			wantRoots: []string{
				"https://g.example/myproxything",
				"https://g.example/myproxything/api/v1",
			},
			wantKind: "prometheus",
		},
		{
			name:         "path merely containing proxy, forced grafana",
			address:      "https://g.example/myproxything",
			endpointType: "grafana",
			wantRoots: []string{
				"https://g.example/myproxything",
				"https://g.example/myproxything/api/v1",
			},
			wantKind: "grafana",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roots, kind := Candidates(tc.address, tc.endpointType)
			if !reflect.DeepEqual(roots, tc.wantRoots) {
				t.Errorf("roots = %#v, want %#v", roots, tc.wantRoots)
			}
			if kind != tc.wantKind {
				t.Errorf("kind = %q, want %q", kind, tc.wantKind)
			}
		})
	}
}
