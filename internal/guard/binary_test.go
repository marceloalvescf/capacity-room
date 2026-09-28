package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

// TestProductionBinaryHoldsNoTestSupport confirms the shipped binary never
// pulls in test-only support code. internal/promtest and
// internal/goldentest exist to help tests build fixtures and pages;
// cmd/fake-prometheus and cmd/update-testdata are tools built for the same
// purpose. None of them may reach cmd/k8s-capacity-report through its
// import graph. internal/goldentest and cmd/update-testdata do not exist
// yet; the guard is written ahead of them so that neither package can slip
// in unnoticed once it does.
func TestProductionBinaryHoldsNoTestSupport(t *testing.T) {
	root := moduleRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	module := modfile.ModulePath(raw)
	if module == "" {
		t.Fatalf("could not read module path from go.mod")
	}

	cmd := exec.Command("go", "list", "-deps", "./cmd/k8s-capacity-report")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps ./cmd/k8s-capacity-report: %v\n%s", err, out)
	}

	forbidden := []string{
		module + "/internal/promtest",
		module + "/internal/goldentest",
		module + "/cmd/fake-prometheus",
		module + "/cmd/update-testdata",
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		for _, pkg := range forbidden {
			if line == pkg || strings.HasPrefix(line, pkg+"/") {
				t.Fatalf("cmd/k8s-capacity-report depends on %s", line)
			}
		}
	}
}

// TestProductionBinaryEmbedsZones confirms the shipped binary embeds the
// IANA time zone database (via internal/config's blank import of
// time/tzdata), so analysis.timezone resolves the same way on every
// machine, whether or not the host itself carries zoneinfo files.
func TestProductionBinaryEmbedsZones(t *testing.T) {
	root := moduleRoot(t)

	cmd := exec.Command("go", "list", "-deps", "./cmd/k8s-capacity-report")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps ./cmd/k8s-capacity-report: %v\n%s", err, out)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "time/tzdata" {
			return
		}
	}
	t.Fatalf("cmd/k8s-capacity-report does not depend on time/tzdata")
}

// TestFakePrometheusBuilds confirms cmd/fake-prometheus builds on its own,
// as the tool kept out of the production binary above.
func TestFakePrometheusBuilds(t *testing.T) {
	root := moduleRoot(t)
	out := filepath.Join(t.TempDir(), "fake-prometheus")

	cmd := exec.Command("go", "build", "-o", out, "./cmd/fake-prometheus")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build -o %s ./cmd/fake-prometheus: %v\n%s", out, err, output)
	}
}
