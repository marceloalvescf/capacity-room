package promtest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Scenario names one of the scenarios and the settings the original
// tests, or this project, ran it with.
type Scenario struct {
	Name   string // file stem
	Folder string // "prometheus" or "prometheus-own"
	// NodeAutoscaling is the value of analysis.node_autoscaling the
	// original tests, or this project, ran the scenario with.
	NodeAutoscaling string
	// Fixture is the file stem of the recorded answers; empty means Name.
	Fixture string
	// Start and End are the wall times of a range scenario, read in
	// America/Sao_Paulo; empty for a scenario that asks about the last N.
	Start, End string
}

// Scenarios holds the five scenarios of the original Python tool, carried
// over byte for byte in testdata/prometheus, and edge-values and
// edge-scaling, written by this project in testdata/prometheus-own. value-edge-cases holds the same
// bytes as direct-complete; only the node autoscaling setting the original
// tests ran it with tells the two apart.
var Scenarios = []Scenario{
	{Name: "direct-complete", Folder: "prometheus", NodeAutoscaling: "auto"},
	{Name: "thanos-multicluster", Folder: "prometheus", NodeAutoscaling: "auto"},
	{Name: "raw-fallback", Folder: "prometheus", NodeAutoscaling: "auto"},
	{Name: "partial-data", Folder: "prometheus", NodeAutoscaling: "auto"},
	{Name: "value-edge-cases", Folder: "prometheus", NodeAutoscaling: "disabled"},
	{Name: "edge-values", Folder: "prometheus-own", NodeAutoscaling: "auto"},
	{Name: "edge-scaling", Folder: "prometheus-own", NodeAutoscaling: "auto"},
}

// RangeScenarios holds the scenarios that ask about a calendar range. Each
// reads the recorded answers of another scenario, so it stays apart from
// Scenarios, whose every entry is compared with the original.
var RangeScenarios = []Scenario{
	{Name: "direct-complete-range", Fixture: "direct-complete", Folder: "prometheus", NodeAutoscaling: "auto", Start: "2026-07-13 00:00", End: "2026-07-14 00:00"},
}

// scenarioPath returns the scenario file of name, under its own folder, found by
// walking up from the working directory to the folder that holds go.mod. A
// range scenario resolves to the file of its fixture.
func scenarioPath(name string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	for _, s := range slices.Concat(Scenarios, RangeScenarios) {
		if s.Name == name {
			return filepath.Join(root, "testdata", s.Folder, s.fixture()+".json"), nil
		}
	}
	return "", fmt.Errorf("no scenario named %q", name)
}

// fixture returns the file stem of the scenario's recorded answers.
func (s Scenario) fixture() string {
	if s.Fixture != "" {
		return s.Fixture
	}
	return s.Name
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

// AddedPath returns the file of the answers this project adds to the
// scenario of name, in testdata/prometheus-added.
func AddedPath(name string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	for _, s := range slices.Concat(Scenarios, RangeScenarios) {
		if s.Name == name {
			return filepath.Join(root, "testdata", "prometheus-added", s.fixture()+".json"), nil
		}
	}
	return "", fmt.Errorf("no scenario named %q", name)
}

// LoadScenario reads the scenario of name: the answers this project adds,
// then the recorded answers, as one fixture.
func LoadScenario(name string) (Fixture, error) {
	added, err := AddedPath(name)
	if err != nil {
		return Fixture{}, err
	}
	recorded, err := scenarioPath(name)
	if err != nil {
		return Fixture{}, err
	}
	a, err := Load(added)
	if err != nil {
		return Fixture{}, err
	}
	r, err := Load(recorded)
	if err != nil {
		return Fixture{}, err
	}
	return Merge(a, r), nil
}
