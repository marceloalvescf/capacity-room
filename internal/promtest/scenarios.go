package promtest

import (
	"fmt"
	"os"
	"path/filepath"
)

// Scenario names one of the six scenarios and the settings the original
// tests ran it with.
type Scenario struct {
	Name         string // file stem
	APIRoot      string // path to add to the server address
	EndpointType string
	Token        string // empty when the scenario needs none
	// NodeAutoscaling is the value of analysis.node_autoscaling the
	// original tests ran the scenario with.
	NodeAutoscaling string
}

// Scenarios is the six scenarios of the original Python tool, carried over
// byte for byte in testdata/prometheus. value-edge-cases holds the same
// bytes as direct-complete; only the node autoscaling setting the original
// tests ran it with tells the two apart.
var Scenarios = []Scenario{
	{Name: "direct-complete", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "auto"},
	{Name: "grafana-authenticated", APIRoot: "/api/datasources/uid/prom/resources/api/v1", EndpointType: "grafana", Token: "fixture-token", NodeAutoscaling: "auto"},
	{Name: "thanos-multicluster", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "auto"},
	{Name: "raw-fallback", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "auto"},
	{Name: "partial-data", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "auto"},
	{Name: "value-edge-cases", APIRoot: "/api/v1", EndpointType: "prometheus", Token: "", NodeAutoscaling: "disabled"},
}

// Path returns the scenario file of name under testdata/prometheus, found
// by walking up from the working directory to the folder that holds
// go.mod.
func Path(name string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "testdata", "prometheus", name+".json"), nil
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
