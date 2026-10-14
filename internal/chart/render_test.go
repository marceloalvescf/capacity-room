//go:build helm

package chart

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// chartDir is the chart, from this package's folder.
const chartDir = "../../deploy/helm/capacity-room"

// required names the two Secrets the schema requires.
var required = []string{"--set", "database.existingSecret=db", "--set", "serverSecret.existingSecret=server"}

// object is one rendered manifest. It is decoded as a plain map, so that the
// maps inside it are plain maps too.
type object map[string]any

func (o object) kind() string { return o["kind"].(string) }

func (o object) name() string { return o["metadata"].(map[string]any)["name"].(string) }

// podSpec returns the pod template's spec of a Deployment or a Job.
func (o object) podSpec() map[string]any {
	template := o["spec"].(map[string]any)["template"].(map[string]any)
	return template["spec"].(map[string]any)
}

// render runs helm template for release with args and returns its manifests.
// A failed render fails the test.
func render(t *testing.T, release string, args ...string) []object {
	t.Helper()
	cmd := exec.Command("helm", append([]string{"template", release, chartDir, "--namespace", "capacity-room"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm template: %v: %s", err, stderr.String())
	}
	var objects []object
	decoder := yaml.NewDecoder(&stdout)
	for {
		var o map[string]any
		err := decoder.Decode(&o)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if o != nil {
			objects = append(objects, object(o))
		}
	}
	return objects
}

// renderError runs helm template with args, expects it to fail, and returns
// its standard error.
func renderError(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("helm", append([]string{"template", "capacity-room", chartDir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("helm template %s succeeded, want a refusal", strings.Join(args, " "))
	}
	return stderr.String()
}

// find returns the object of kind and name, failing the test when there is
// none.
func find(t *testing.T, objects []object, kind, name string) object {
	t.Helper()
	for _, o := range objects {
		if o.kind() == kind && o.name() == name {
			return o
		}
	}
	t.Fatalf("no %s %s among the rendered objects", kind, name)
	return nil
}
