//go:build helm

package chart

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
)

func TestNoJobsByDefault(t *testing.T) {
	for _, o := range render(t, "capacity-room", required...) {
		if o.kind() == "Job" {
			t.Fatalf("a Job %s rendered with no admin and no profiles", o.name())
		}
	}
}

func TestAdminJob(t *testing.T) {
	objects := render(t, "capacity-room", append(required,
		"--set", "admin.username=admin", "--set", "admin.existingSecret=admin-secret")...)
	job := find(t, objects, "Job", "capacity-room-admin")
	annotations := job["metadata"].(map[string]any)["annotations"].(map[string]any)
	if annotations["helm.sh/hook"] != "post-install" ||
		annotations["helm.sh/hook-delete-policy"] != "before-hook-creation,hook-succeeded" {
		t.Fatalf("annotations %v", annotations)
	}
	c := job.podSpec()["containers"].([]any)[0].(map[string]any)
	want := []any{"admin", "bootstrap-admin", "--username", "admin", "--language", "en",
		"--password-file", "/opt/capacity-room/admin/password", "--config", "/opt/capacity-room/platform.toml"}
	if !equalLists(c["args"].([]any), want) {
		t.Fatalf("args %v", c["args"])
	}
	if !containsEnv(c["env"].([]any), "CAPACITYROOM_BOOTSTRAP_MODE", "1") {
		t.Fatalf("env %v", c["env"])
	}
	for name, spec := range pods(objects) {
		hasAdmin := false
		for _, v := range spec["volumes"].([]any) {
			if v.(map[string]any)["name"] == "admin" {
				hasAdmin = true
			}
		}
		if hasAdmin != (name == "Job capacity-room-admin") {
			t.Errorf("%s: admin volume %v", name, hasAdmin)
		}
		if name != "Job capacity-room-admin" {
			for _, e := range spec["containers"].([]any)[0].(map[string]any)["env"].([]any) {
				if e.(map[string]any)["name"] == "CAPACITYROOM_BOOTSTRAP_MODE" {
					t.Errorf("%s sets the bootstrap mode", name)
				}
			}
		}
	}
	if out := renderError(t, append(required, "--set", "admin.username=admin")...); !strings.Contains(out, "existingSecret") {
		t.Errorf("a missing existingSecret is refused without naming it:\n%s", out)
	}
	if out := renderError(t, append(required, "--set", "admin.username=admin", "--set", "admin.existingSecret=s", "--set", "admin.passwordKey=a/b")...); !strings.Contains(out, "passwordKey") {
		t.Errorf("a passwordKey with a slash is refused without naming it:\n%s", out)
	}
	if out := renderError(t, append(required, "--set", "admin.username=admin", "--set", "admin.existingSecret=s", "--set", "admin.language=fr")...); !strings.Contains(out, "language") {
		t.Errorf("language fr is refused without naming it:\n%s", out)
	}
}

func equalLists(got, want []any) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestProfilesRoundTrip renders testdata/profiles.yaml, whose display name
// holds quotes, a backslash, accents, <, and &, and reads the rendered file
// with the import's own parser.
func TestProfilesRoundTrip(t *testing.T) {
	objects := render(t, "capacity-room", append(required, "-f", "testdata/profiles.yaml")...)
	job := find(t, objects, "Job", "capacity-room-profiles")
	annotations := job["metadata"].(map[string]any)["annotations"].(map[string]any)
	if annotations["helm.sh/hook"] != "post-install,post-upgrade" {
		t.Fatalf("annotations %v", annotations)
	}
	data := find(t, objects, "ConfigMap", "capacity-room")["data"].(map[string]any)
	path := filepath.Join(t.TempDir(), "profiles.toml")
	if err := os.WriteFile(path, []byte(data["profiles.toml"].(string)), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded := loadSettings(t, objects)
	entries, err := profiles.ReadImportFile(path, netpolicy.Policy{
		Hostnames: loaded.AllowedEndpointHostnames, Networks: loaded.AllowedEndpointCIDRs,
	})
	if err != nil {
		t.Fatalf("ReadImportFile: %v\n%s", err, data["profiles.toml"])
	}
	if len(entries) != 2 || entries[0].ID != "prod" || entries[1].ID != "staging" {
		t.Fatalf("entries %+v", entries)
	}
	const name = `Prod "east" \ ação <b> & co`
	if entries[0].Document.DisplayName != name {
		t.Fatalf("display name %q, want %q", entries[0].Document.DisplayName, name)
	}
	if entries[1].Document.Thresholds.WarningPercent != 70 {
		t.Fatalf("thresholds %+v", entries[1].Document.Thresholds)
	}
}

func TestKubeconformJobs(t *testing.T) {
	kubeconform(t, renderRaw(t, append(required, "-f", "testdata/profiles.yaml",
		"--set", "admin.username=admin", "--set", "admin.existingSecret=admin-secret")...))
	kubeconform(t, renderRaw(t, "-f", "../../deploy/helm/capacity-room/examples/gateway.yaml"))
}

// jobContainers returns the container of the admin Job and of the profiles
// Job rendered with args.
func jobContainers(t *testing.T, args ...string) []map[string]any {
	t.Helper()
	objects := render(t, "capacity-room", append(append(required,
		"-f", "testdata/profiles.yaml",
		"--set", "admin.username=admin", "--set", "admin.existingSecret=admin-secret"), args...)...)
	var containers []map[string]any
	for _, name := range []string{"capacity-room-admin", "capacity-room-profiles"} {
		job := find(t, objects, "Job", name)
		containers = append(containers, job.podSpec()["containers"].([]any)[0].(map[string]any))
	}
	return containers
}

func TestJobsHaveResources(t *testing.T) {
	for _, c := range jobContainers(t) {
		r := c["resources"].(map[string]any)
		requests := r["requests"].(map[string]any)
		limits := r["limits"].(map[string]any)
		if requests["cpu"] != "50m" || requests["memory"] != "128Mi" || len(requests) != 2 {
			t.Errorf("%v: requests %v", c["name"], requests)
		}
		if limits["memory"] != "256Mi" || len(limits) != 1 {
			t.Errorf("%v: limits %v", c["name"], limits)
		}
	}
}

func TestJobResourcesCanBeSet(t *testing.T) {
	for _, c := range jobContainers(t, "--set", "jobs.resources.limits.cpu=200m") {
		limits := c["resources"].(map[string]any)["limits"].(map[string]any)
		if limits["cpu"] != "200m" || limits["memory"] != "256Mi" {
			t.Errorf("%v: limits %v", c["name"], limits)
		}
	}
}

func TestUnknownJobsKeyIsRefused(t *testing.T) {
	if out := renderError(t, append(required, "--set", "jobs.limits=1")...); !strings.Contains(out, "at '/jobs': additional properties 'limits' not allowed") {
		t.Errorf("an unknown key under jobs is not refused:\n%s", out)
	}
}

// notes returns the notes Helm prints after an install of the chart with
// args, from a dry run that reaches no cluster.
func notes(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("helm", append([]string{"install", "capacity-room", chartDir,
		"--namespace", "capacity-room", "--dry-run=client"}, append(required, args...)...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm install --dry-run=client: %v: %s", err, stderr.String())
	}
	_, after, found := strings.Cut(stdout.String(), "NOTES:\n")
	if !found {
		t.Fatalf("no notes in the dry run:\n%s", stdout.String())
	}
	return after
}

func TestNotesWarnWhenNoAdministratorIsMade(t *testing.T) {
	const warning = "No administrator is created"
	if out := notes(t); !strings.Contains(out, warning) {
		t.Errorf("notes with admin.username empty lack the warning:\n%s", out)
	}
	out := notes(t, "--set", "admin.username=admin", "--set", "admin.existingSecret=admin-secret")
	if strings.Contains(out, warning) {
		t.Errorf("notes with admin.username set carry the warning:\n%s", out)
	}
}
