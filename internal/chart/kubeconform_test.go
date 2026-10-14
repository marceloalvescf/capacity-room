//go:build helm

package chart

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"
)

// renderRaw runs helm template with args and returns its output whole.
func renderRaw(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("helm", append([]string{"template", "capacity-room", chartDir, "--namespace", "capacity-room"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm template: %v: %s", err, stderr.String())
	}
	return stdout.Bytes()
}

// summaryLine captures the Valid, Invalid, Errors, and Skipped counts of
// kubeconform's summary.
var summaryLine = regexp.MustCompile(`Valid: (\d+), Invalid: (\d+), Errors: (\d+), Skipped: (\d+)`)

// kubeconform checks manifests against the schemas kept in the repository
// alone; a kind with no schema there fails.
func kubeconform(t *testing.T, manifests ...[]byte) {
	t.Helper()
	cmd := exec.Command("kubeconform", "-strict", "-summary",
		"-schema-location", "../../deploy/helm/schemas/kubernetes/{{.ResourceKind}}{{.KindSuffix}}.json",
		"-schema-location", "../../deploy/helm/schemas/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
		"-")
	cmd.Stdin = bytes.NewReader(bytes.Join(manifests, []byte("\n---\n")))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kubeconform: %v\n%s", err, out)
	}
	summary := summaryLine.FindSubmatch(out)
	if summary == nil || string(summary[1]) == "0" || string(summary[2]) != "0" || string(summary[3]) != "0" || string(summary[4]) != "0" {
		t.Fatalf("kubeconform validated nothing, or not everything:\n%s", out)
	}
	t.Logf("%s", out)
}

func TestKubeconform(t *testing.T) {
	kubeconform(t, renderRaw(t, required...))
	kubeconform(t, renderRaw(t, "-f", "../../deploy/helm/capacity-room/examples/gateway.yaml"))
	kubeconform(t, renderRaw(t, "-f", "../../deploy/helm/capacity-room/examples/ingress.yaml"))
}

func TestKubeconformRefusesAWrongManifest(t *testing.T) {
	wrong := []byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: x\nspec:\n  portz: []\n")
	cmd := exec.Command("kubeconform", "-strict", "-summary",
		"-schema-location", "../../deploy/helm/schemas/kubernetes/{{.ResourceKind}}{{.KindSuffix}}.json", "-")
	cmd.Stdin = bytes.NewReader(wrong)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("kubeconform accepted a Service with an unknown field")
	}
	if !bytes.Contains(out, []byte("additional properties 'portz' not allowed")) {
		t.Fatalf("kubeconform refused it for another reason:\n%s", out)
	}
}
