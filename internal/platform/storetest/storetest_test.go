//go:build postgres

package storetest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestNewRefusesUnsetVariable(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-tags=postgres", "-run", "TestNewProbe", ".")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, dsnVariable+"=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("the child passed, want a failure:\n%s", out)
	}
	if !strings.Contains(string(out), "KCR_TEST_DATABASE_DSN is not set") {
		t.Fatalf("the child output does not name the variable:\n%s", out)
	}
}

// TestNewProbe is selected only by TestNewRefusesUnsetVariable.
func TestNewProbe(t *testing.T) {
	New(t)
}
