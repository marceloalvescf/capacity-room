//go:build helm

package chart

import (
	"slices"
	"strings"
	"testing"
)

func TestRoute(t *testing.T) {
	objects := render(t, "capacity-room", "-f", "../../deploy/helm/capacity-room/examples/gateway.yaml")
	route := find(t, objects, "HTTPRoute", "capacity-room")["spec"].(map[string]any)
	backend := route["rules"].([]any)[0].(map[string]any)["backendRefs"].([]any)[0].(map[string]any)
	if backend["name"] != "capacity-room" || backend["port"] != 8080 {
		t.Fatalf("backend %v", backend)
	}
	for _, o := range objects {
		if o.kind() == "Ingress" {
			t.Fatal("an Ingress rendered with the route")
		}
	}
}

func TestIngress(t *testing.T) {
	objects := render(t, "capacity-room", "-f", "../../deploy/helm/capacity-room/examples/ingress.yaml")
	spec := find(t, objects, "Ingress", "capacity-room")["spec"].(map[string]any)
	if spec["ingressClassName"] != "nginx" {
		t.Fatalf("ingressClassName %v", spec["ingressClassName"])
	}
	rule := spec["rules"].([]any)[0].(map[string]any)
	if rule["host"] != "capacityroom.example.com" {
		t.Fatalf("rule %v", rule)
	}
}

func TestRouteAndIngressExcludeEachOther(t *testing.T) {
	both := slices.Concat(required, []string{
		"--set", "route.enabled=true", "--set", "route.parentRefs[0].name=gateway", "--set", "route.hostnames={a.example.com}",
		"--set", "ingress.enabled=true", "--set", "ingress.hosts={a.example.com}"})
	cases := []struct {
		args []string
		want string
	}{
		{both, "'not' failed"},
		{append(required, "--set", "route.enabled=true"), "/route/parentRefs"},
		{append(required, "--set", "ingress.enabled=true"), "/ingress/hosts"},
	}
	for _, c := range cases {
		if stderr := renderError(t, c.args...); !strings.Contains(stderr, c.want) {
			t.Errorf("the refusal does not name %q: %s", c.want, stderr)
		}
	}
}

func TestLintExamples(t *testing.T) {
	for _, file := range []string{"gateway.yaml", "ingress.yaml"} {
		lint(t, "-f", "../../deploy/helm/capacity-room/examples/"+file)
	}
}
