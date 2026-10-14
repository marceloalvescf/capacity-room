//go:build helm

package chart

import (
	"bytes"
	"os/exec"
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
	t.Logf("%s", out)
}

func TestKubeconform(t *testing.T) {
	kubeconform(t, renderRaw(t, required...))
	kubeconform(t, renderRaw(t, "-f", "../../deploy/helm/capacity-room/examples/gateway.yaml"))
	kubeconform(t, renderRaw(t, "-f", "../../deploy/helm/capacity-room/examples/ingress.yaml"))
}

func TestKubeconformRefusesAWrongManifest(t *testing.T) {
	wrong := []byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: x\nspec:\n  portz: []\n")
	cmd := exec.Command("kubeconform", "-strict",
		"-schema-location", "../../deploy/helm/schemas/kubernetes/{{.ResourceKind}}{{.KindSuffix}}.json", "-")
	cmd.Stdin = bytes.NewReader(wrong)
	if err := cmd.Run(); err == nil {
		t.Fatal("kubeconform accepted a Service with an unknown field")
	}
}
