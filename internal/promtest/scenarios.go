package promtest

import (
	"fmt"
	"os"
	"path/filepath"
)

// Scenario names one of the seven scenarios and the settings the original
// tests, or this project, ran it with.
type Scenario struct {
	Name         string // file stem
	Folder       string // "prometheus" or "prometheus-own"
	APIRoot      string // path to add to the server address
	EndpointType string
	Token        string // empty when the scenario needs none
	// NodeAutoscaling is the value of analysis.node_autoscaling the
	// original tests, or this project, ran the scenario with.
	NodeAutoscaling string
}

// Scenarios holds the six scenarios of the original Python tool, carried
// over byte for byte in testdata/prometheus, and edge-values, written by
// this project in testdata/prometheus-own. value-edge-cases holds the same
// bytes as direct-complete; only the node autoscaling setting the original
// tests ran it with tells the two apart.
var Scenarios = []Scenario{
	{Name: "direct-complete", Folder: "prometheus", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "auto"},
	{Name: "grafana-authenticated", Folder: "prometheus", APIRoot: "/api/datasources/uid/prom/resources/api/v1", EndpointType: "grafana", Token: "fixture-token", NodeAutoscaling: "auto"},
	{Name: "thanos-multicluster", Folder: "prometheus", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "auto"},
	{Name: "raw-fallback", Folder: "prometheus", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "auto"},
	{Name: "partial-data", Folder: "prometheus", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "auto"},
	{Name: "value-edge-cases", Folder: "prometheus", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "disabled"},
	{Name: "edge-values", Folder: "prometheus-own", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "auto"},
}

// Path returns the scenario file of name, under its own folder, found by
// walking up from the working directory to the folder that holds go.mod.
func Path(name string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	for _, s := range Scenarios {
		if s.Name == name {
			return filepath.Join(root, "testdata", s.Folder, name+".json"), nil
		}
	}
	return "", fmt.Errorf("no scenario named %q", name)
}

// moduleRoot walks up from the working directory to the folder that holds
// go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
