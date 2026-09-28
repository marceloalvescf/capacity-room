package guard

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const baseGoMod = `module example.com/widget

go 1.27

require github.com/spf13/cobra v1.10.2
`

func TestForbiddenModulesClean(t *testing.T) {
	got, err := ForbiddenModules([]byte(baseGoMod))
	if err != nil {
		t.Fatalf("ForbiddenModules returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ForbiddenModules = %v, want empty", got)
	}
}

func TestForbiddenModulesDirect(t *testing.T) {
	gomod := baseGoMod + "require k8s.io/client-go v0.30.0\n"

	got, err := ForbiddenModules([]byte(gomod))
	if err != nil {
		t.Fatalf("ForbiddenModules returned error: %v", err)
	}
	want := []string{"k8s.io/client-go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ForbiddenModules = %v, want %v", got, want)
	}
}

func TestForbiddenModulesIndirect(t *testing.T) {
	gomod := baseGoMod + "require github.com/aws/aws-sdk-go-v2 v1.0.0 // indirect\n"

	got, err := ForbiddenModules([]byte(gomod))
	if err != nil {
		t.Fatalf("ForbiddenModules returned error: %v", err)
	}
	want := []string{"github.com/aws/aws-sdk-go-v2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ForbiddenModules = %v, want %v", got, want)
	}
}

func TestForbiddenModulesReplace(t *testing.T) {
	gomod := baseGoMod + "require example.com/x v1.0.0\n\nreplace example.com/x => sigs.k8s.io/yaml v1.0.0\n"

	got, err := ForbiddenModules([]byte(gomod))
	if err != nil {
		t.Fatalf("ForbiddenModules returned error: %v", err)
	}
	want := []string{"sigs.k8s.io/yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ForbiddenModules = %v, want %v", got, want)
	}
}

func TestForbiddenModulesSegmentBoundary(t *testing.T) {
	gomod := baseGoMod + "require k8s.iox/thing v1.0.0\n"

	got, err := ForbiddenModules([]byte(gomod))
	if err != nil {
		t.Fatalf("ForbiddenModules returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ForbiddenModules = %v, want empty", got)
	}
}

func TestForbiddenModulesMalformed(t *testing.T) {
	_, err := ForbiddenModules([]byte("requir x"))
	if err == nil {
		t.Fatalf("ForbiddenModules returned no error, want error")
	}
}

func TestRepositoryHasNoForbiddenModule(t *testing.T) {
	root := moduleRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}

	got, err := ForbiddenModules(raw)
	if err != nil {
		t.Fatalf("ForbiddenModules returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ForbiddenModules = %v, want empty", got)
	}
}
