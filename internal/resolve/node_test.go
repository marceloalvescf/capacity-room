package resolve

import "testing"

func TestInstanceHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"10.0.0.1:9100", "10.0.0.1"},
		{"[fd00::1]:9100", "fd00::1"},
		{"Node-A:9100", "node-a"},
		{"user@host:1", "host"},
		{"host", "host"},
		{"", ""},
		{"[abc", ""},
		{"[foo]:9100", ""},
		{"[10.0.0.1]:9100", ""},
		{"[v1.x]:1", "v1.x"},
		{"[::1]x", ""},
		{"ho\tst:1", "host"},
		{"host\n:9100", "host"},
	}
	for _, c := range cases {
		if got := InstanceHost(c.in); got != c.want {
			t.Errorf("InstanceHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIndexDropsASharedValue(t *testing.T) {
	x := NewNodeIndex([]NodeRecord{
		{Name: "a", InternalIP: "10.0.0.1"},
		{Name: "b", InternalIP: "10.0.0.1"},
		{Name: "c", InternalIP: "10.0.0.3"},
	})
	if name, ok := x.Resolve("", "10.0.0.1:9100"); ok {
		t.Errorf("shared IP resolved to %q", name)
	}
	if name, ok := x.Resolve("", "10.0.0.3:9100"); !ok || name != "c" {
		t.Errorf("got %q, %v, want c, true", name, ok)
	}
}

func TestIndexSkipsAValueSharedByThreeNodes(t *testing.T) {
	x := NewNodeIndex([]NodeRecord{
		{Name: "a", InternalIP: "10.0.0.1"},
		{Name: "b", InternalIP: "10.0.0.1"},
		{Name: "c", InternalIP: "10.0.0.1"},
	})
	if name, ok := x.Resolve("", "10.0.0.1:1"); ok {
		t.Errorf("shared IP resolved to %q", name)
	}
}

func TestIndexPrefersTheNodeLabel(t *testing.T) {
	x := NewNodeIndex([]NodeRecord{{Name: "a"}})
	if name, ok := x.Resolve("a", "10.9.9.9:1"); !ok || name != "a" {
		t.Errorf("got %q, %v, want a, true", name, ok)
	}
}

func TestIndexOrderIPHostnameName(t *testing.T) {
	x := NewNodeIndex([]NodeRecord{
		{Name: "a", Hostname: "h"},
		{Name: "h", InternalIP: ""},
	})
	if name, ok := x.Resolve("", "h:1"); !ok || name != "a" {
		t.Errorf("got %q, %v, want a, true", name, ok)
	}
}

func TestIndexDoesNotLowerTheNodeLabel(t *testing.T) {
	x := NewNodeIndex([]NodeRecord{{Name: "Node-A"}})
	if name, ok := x.Resolve("node-a", ""); ok {
		t.Errorf("lower-case label resolved to %q", name)
	}
	if name, ok := x.Resolve("", "Node-A:1"); ok {
		t.Errorf("lowered host resolved to %q", name)
	}
	if name, ok := x.Resolve("Node-A", ""); !ok || name != "Node-A" {
		t.Errorf("got %q, %v, want Node-A, true", name, ok)
	}
}

func TestIndexEmptyHostMatchesNothing(t *testing.T) {
	x := NewNodeIndex([]NodeRecord{{Name: "a"}})
	if name, ok := x.Resolve("", ""); ok {
		t.Errorf("empty instance resolved to %q", name)
	}
}
