package resolve

import (
	"reflect"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/discovery"
)

func TestConfidencesEqualDiscovery(t *testing.T) {
	if ConfidenceExact != discovery.ConfidenceExact ||
		ConfidenceInferred != discovery.ConfidenceInferred ||
		ConfidenceUnknown != discovery.ConfidenceUnknown {
		t.Error("resolve confidences differ from discovery")
	}
}

func checkPool(t *testing.T, pool, source, conf, wantPool, wantSource, wantConf string) {
	t.Helper()
	if pool != wantPool || source != wantSource || conf != wantConf {
		t.Errorf("got (%q, %q, %q), want (%q, %q, %q)", pool, source, conf, wantPool, wantSource, wantConf)
	}
}

func TestNodePoolConfiguredLabelWins(t *testing.T) {
	p, s, c := NodePool("n", map[string]string{"label_pool": "x", "label_agentpool": "y"}, "label_pool", nil)
	checkPool(t, p, s, c, "x", "label_pool", "exact")
}

func TestNodePoolKnownLabelsInOrder(t *testing.T) {
	p, s, c := NodePool("n", map[string]string{
		"label_agentpool":                      "b",
		"label_kubernetes_azure_com_agentpool": "a",
	}, "", nil)
	checkPool(t, p, s, c, "a", "label_kubernetes_azure_com_agentpool", "exact")
}

func TestNodePoolAKSName(t *testing.T) {
	p, s, c := NodePool("aks-nodepool1-12345678-vmss000000", nil, "", nil)
	checkPool(t, p, s, c, "nodepool1", "aks-node-name", "inferred")
}

func TestNodePoolAKSNameNeedsThePrefix(t *testing.T) {
	p, s, c := NodePool("xaks-nodepool1-1-vmss", nil, "", nil)
	checkPool(t, p, s, c, "unknown", "none", "unknown")
}

func TestNodePoolOKEName(t *testing.T) {
	p, s, c := NodePool("oke-cabc-nxyz-swxyz-0", nil, "", nil)
	checkPool(t, p, s, c, "pool-nxyz", "oke-node-name", "inferred")
}

func TestNodePoolOKENameNeedsFourParts(t *testing.T) {
	p, s, c := NodePool("oke-a-b", nil, "", nil)
	checkPool(t, p, s, c, "unknown", "none", "unknown")
}

func TestNodePoolSharedName(t *testing.T) {
	labels := map[string]string{"label_name": "workers"}
	p, s, c := NodePool("n", labels, "", map[string]bool{"workers": true})
	checkPool(t, p, s, c, "workers", "label_name", "inferred")
	p, s, c = NodePool("n", labels, "", map[string]bool{})
	checkPool(t, p, s, c, "unknown", "none", "unknown")
}

func TestNodePoolUnknown(t *testing.T) {
	p, s, c := NodePool("plain", nil, "", nil)
	checkPool(t, p, s, c, "unknown", "none", "unknown")
}

func TestSharedNameLabels(t *testing.T) {
	got := SharedNameLabels(map[string]map[string]string{
		"a": {"label_name": "w"},
		"b": {"label_name": "w"},
		"c": {"label_name": "v"},
		"d": {},
	})
	if want := map[string]bool{"w": true}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
