package chart

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

const ociChart = "oci://ghcr.io/marceloalvescf/charts/capacity-room"

// TestPublishedChartVersionEqualsVERSION requires the OCI chart in both chart
// documents, and every version written beside it, and every tag of the
// published image, to equal VERSION.
func TestPublishedChartVersionEqualsVERSION(t *testing.T) {
	raw, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(string(raw))
	pin := regexp.MustCompile(`--version (\S+)`)
	image := regexp.MustCompile(`alvesmarcelocf/capacity-room:([^\s"'` + "`" + `]+)`)
	documents := []struct {
		path     string
		required bool
	}{
		{"../../docs/operations.md", true},
		{"../../deploy/helm/capacity-room/README.md", true},
		{"../../README.md", false},
	}
	for _, document := range documents {
		data, err := os.ReadFile(document.path)
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for number, line := range strings.Split(string(data), "\n") {
			for _, tag := range image.FindAllStringSubmatch(line, -1) {
				if strings.TrimRight(tag[1], ".,;") != version {
					t.Errorf("%s:%d: image tag %s, want %s", document.path, number+1, tag[1], version)
				}
			}
			if !strings.Contains(line, ociChart) {
				continue
			}
			found++
			match := pin.FindStringSubmatch(line)
			if strings.HasPrefix(line, "helm ") && match == nil {
				t.Errorf("%s:%d: a helm command names the chart with no --version", document.path, number+1)
			}
			if match != nil && strings.Trim(match[1], "`.,;") != version {
				t.Errorf("%s:%d: --version %s, want %s", document.path, number+1, match[1], version)
			}
		}
		if document.required && found == 0 {
			t.Errorf("%s does not name %s", document.path, ociChart)
		}
	}
}
