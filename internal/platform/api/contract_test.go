package api

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func contractPath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "api", "openapi.yaml")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above the working directory")
		}
		dir = parent
	}
}

func difference(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func TestRoutesMatchTheContract(t *testing.T) {
	raw, err := os.ReadFile(contractPath(t))
	if err != nil {
		t.Fatalf("reading the contract: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing the contract: %v", err)
	}
	methods := map[string]bool{"get": true, "post": true, "patch": true, "delete": true, "put": true}
	contract := map[string]bool{}
	for path, item := range doc.Paths {
		for key := range item {
			if methods[key] {
				contract[strings.ToUpper(key)+" /api/v1"+path] = true
			}
		}
	}
	program := map[string]bool{}
	for _, r := range (&Server{}).Routes() {
		program[r.Method+" "+r.Pattern] = true
	}
	if missing := difference(contract, program); len(missing) > 0 {
		t.Errorf("in the contract and not in the routes: %v", missing)
	}
	if extra := difference(program, contract); len(extra) > 0 {
		t.Errorf("in the routes and not in the contract: %v", extra)
	}
}

func TestContractNamesEveryCode(t *testing.T) {
	raw, err := os.ReadFile(contractPath(t))
	if err != nil {
		t.Fatalf("reading the contract: %v", err)
	}
	var doc struct {
		Components struct {
			Schemas struct {
				ErrorCode struct {
					Enum []string `yaml:"enum"`
				} `yaml:"ErrorCode"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing the contract: %v", err)
	}
	contract := map[string]bool{}
	for _, c := range doc.Components.Schemas.ErrorCode.Enum {
		if contract[c] {
			t.Errorf("code %s is named twice in the ErrorCode enum", c)
		}
		contract[c] = true
	}
	program := map[string]bool{}
	for c := range problems {
		program[string(c)] = true
	}
	if missing := difference(program, contract); len(missing) > 0 {
		t.Errorf("in the table and not in the ErrorCode enum: %v", missing)
	}
	if extra := difference(contract, program); len(extra) > 0 {
		t.Errorf("in the ErrorCode enum and not in the table: %v", extra)
	}
}

func TestContractOperationsHaveADefault(t *testing.T) {
	raw, err := os.ReadFile(contractPath(t))
	if err != nil {
		t.Fatalf("reading the contract: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing the contract: %v", err)
	}
	methods := map[string]bool{"get": true, "post": true, "patch": true, "delete": true, "put": true}
	operations := 0
	for path, item := range doc.Paths {
		for method, node := range item {
			if !methods[method] {
				continue
			}
			operations++
			var op struct {
				Responses map[string]struct {
					Ref string `yaml:"$ref"`
				} `yaml:"responses"`
			}
			if err := node.Decode(&op); err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			if got := op.Responses["default"].Ref; got != "#/components/responses/Unexpected" {
				t.Errorf("%s %s: default response %q", method, path, got)
			}
		}
	}
	if operations != len((&Server{}).Routes()) {
		t.Errorf("%d operations in the contract, %d routes", operations, len((&Server{}).Routes()))
	}
}
