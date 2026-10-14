//go:build helm

package chart

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/marceloalvescf/capacity-room/internal/platform/settings"
)

func TestChartVersionEqualsVERSION(t *testing.T) {
	raw, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(raw))
	data, err := os.ReadFile(filepath.Join(chartDir, "Chart.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var chart struct {
		Version    string `yaml:"version"`
		AppVersion string `yaml:"appVersion"`
	}
	if err := yaml.Unmarshal(data, &chart); err != nil {
		t.Fatal(err)
	}
	if chart.Version != want || chart.AppVersion != want {
		t.Fatalf("Chart.yaml version %q, appVersion %q, want both %q", chart.Version, chart.AppVersion, want)
	}
}

func TestLint(t *testing.T) {
	lint(t, required...)
}

// loadSettings writes the platform.toml of the rendered ConfigMap into a
// folder and loads it as serve would.
func loadSettings(t *testing.T, objects []object) settings.Settings {
	t.Helper()
	data := find(t, objects, "ConfigMap", "capacity-room")["data"].(map[string]any)
	path := filepath.Join(t.TempDir(), "platform.toml")
	if err := os.WriteFile(path, []byte(data["platform.toml"].(string)), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := settings.Load(path, func(key string) string {
		if key == "CAPACITYROOM_DATABASE_DSN" {
			return "postgres://user:password@db:5432/capacityroom"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("settings.Load: %v\n%s", err, data["platform.toml"])
	}
	return loaded
}

func TestDefaultSettingsLoad(t *testing.T) {
	loaded := loadSettings(t, render(t, "capacity-room", required...))
	if loaded.ServerSecretPath != "/opt/capacity-room/secret/server-secret" ||
		loaded.CredentialDir != "/opt/capacity-room/credentials" || loaded.CABundleDir != "/opt/capacity-room/ca" {
		t.Fatalf("paths: %q %q %q", loaded.ServerSecretPath, loaded.CredentialDir, loaded.CABundleDir)
	}
	if loaded.WorkerSlots != 2 || loaded.WorkerDrain != 30*time.Second {
		t.Fatalf("defaults: slots %d, drain %v", loaded.WorkerSlots, loaded.WorkerDrain)
	}
}

// TestEverySettingLoads sets the fifteen keys at their largest allowed value,
// from a values file and again from --set, so that a number Helm reads as a
// float still reaches the program as an integer.
func TestEverySettingLoads(t *testing.T) {
	fromFile := loadSettings(t, render(t, "capacity-room", append(required, "-f", "testdata/every-setting.yaml")...))
	fromSet := loadSettings(t, render(t, "capacity-room", append(required,
		"--set", "platform.session_ttl_seconds=2592000",
		"--set", "platform.session_idle_seconds=2592000",
		"--set", "platform.worker_slots=16",
		"--set", "platform.worker_drain_seconds=3600",
		"--set", "platform.collect_timeout_seconds=3600",
		"--set", "platform.render_timeout_seconds=3600",
		"--set", "platform.job_max_attempts=10",
		"--set", "platform.report_retention_days=365",
		"--set", "platform.expired_report_days=365",
		"--set", "platform.admission_limit=1000",
		"--set", "platform.admission_profile_limit=1000",
		"--set", "platform.admission_retry_seconds=300",
		"--set", "platform.trusted_proxies={10.244.0.0/16}",
		"--set", "platform.allowed_endpoint_hostnames={prometheus.monitoring.svc.cluster.local}",
		"--set", "platform.allowed_endpoint_cidrs={10.96.0.0/12}",
	)...))
	if !reflect.DeepEqual(fromFile, fromSet) {
		t.Fatalf("the file and --set give different settings:\n%+v\n%+v", fromFile, fromSet)
	}
	if fromFile.SessionTTL != 2592000*time.Second || fromFile.WorkerSlots != 16 || fromFile.AdmissionLimit != 1000 {
		t.Fatalf("settings = %+v", fromFile)
	}
}

func TestChartOwnedKeysAreRefused(t *testing.T) {
	for _, key := range []string{"server_secret_path", "credential_dir", "ca_bundle_dir", "unknown_key"} {
		if stderr := renderError(t, append(required, "--set", "platform."+key+"=/x")...); !strings.Contains(stderr, key) {
			t.Errorf("the refusal of %s does not name it: %s", key, stderr)
		}
	}
}

func TestRequiredValues(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--set", "serverSecret.existingSecret=server"}, "existingSecret"},
		{[]string{"--set", "database.existingSecret=db"}, "existingSecret"},
		{append(required, "--set", "unknown=1"), "unknown"},
		{append(required, "--set", "worker.resources.limits.memory=4G"), "memory"},
	}
	for _, c := range cases {
		if stderr := renderError(t, c.args...); !strings.Contains(stderr, c.want) {
			t.Errorf("the refusal does not name %q: %s", c.want, stderr)
		}
	}
}

// pods returns the pod spec of every Deployment and Job rendered.
func pods(objects []object) map[string]map[string]any {
	specs := map[string]map[string]any{}
	for _, o := range objects {
		if o.kind() == "Deployment" || o.kind() == "Job" {
			specs[o.kind()+" "+o.name()] = o.podSpec()
		}
	}
	return specs
}

func TestPodSecurity(t *testing.T) {
	for name, spec := range pods(render(t, "capacity-room", required...)) {
		if spec["automountServiceAccountToken"] != false {
			t.Errorf("%s mounts a service account token", name)
		}
		pod := spec["securityContext"].(map[string]any)
		if pod["runAsNonRoot"] != true || pod["runAsUser"] != 10001 || pod["runAsGroup"] != 10001 ||
			pod["seccompProfile"].(map[string]any)["type"] != "RuntimeDefault" {
			t.Errorf("%s: pod security context %v", name, pod)
		}
		for _, c := range spec["containers"].([]any) {
			sc := c.(map[string]any)["securityContext"].(map[string]any)
			drop := sc["capabilities"].(map[string]any)["drop"].([]any)
			if sc["readOnlyRootFilesystem"] != true || sc["allowPrivilegeEscalation"] != false ||
				len(drop) != 1 || drop[0] != "ALL" {
				t.Errorf("%s: container security context %v", name, sc)
			}
		}
	}
}

func TestDeployments(t *testing.T) {
	objects := render(t, "capacity-room", required...)
	loaded := loadSettings(t, objects)
	for _, name := range []string{"capacity-room-serve", "capacity-room-worker"} {
		spec := find(t, objects, "Deployment", name)["spec"].(map[string]any)
		if spec["replicas"] != 1 || spec["strategy"].(map[string]any)["type"] != "Recreate" {
			t.Errorf("%s: replicas %v, strategy %v", name, spec["replicas"], spec["strategy"])
		}
	}
	serve := find(t, objects, "Deployment", "capacity-room-serve").podSpec()
	if serve["terminationGracePeriodSeconds"] != 45 {
		t.Errorf("serve grace %v, want 45", serve["terminationGracePeriodSeconds"])
	}
	worker := find(t, objects, "Deployment", "capacity-room-worker").podSpec()
	if got, want := worker["terminationGracePeriodSeconds"], int(2*loaded.WorkerDrain/time.Second+15); got != want {
		t.Errorf("worker grace %v, want %d from the program's default drain", got, want)
	}
	drained := render(t, "capacity-room", append(required, "--set", "platform.worker_drain_seconds=60")...)
	if got := find(t, drained, "Deployment", "capacity-room-worker").podSpec()["terminationGracePeriodSeconds"]; got != 135 {
		t.Errorf("worker grace %v with a drain of 60, want 135", got)
	}
	env := worker["containers"].([]any)[0].(map[string]any)["env"].([]any)
	if !containsEnv(env, "GOMEMLIMIT", "3072MiB") {
		t.Errorf("worker env %v, want GOMEMLIMIT 3072MiB for a limit of 4Gi", env)
	}
}

func containsEnv(env []any, name, value string) bool {
	for _, e := range env {
		m := e.(map[string]any)
		if m["name"] == name && m["value"] == value {
			return true
		}
	}
	return false
}

func TestSettingsChangeRecreatesThePods(t *testing.T) {
	checksum := func(objects []object, name string) string {
		template := find(t, objects, "Deployment", name)["spec"].(map[string]any)["template"].(map[string]any)
		return template["metadata"].(map[string]any)["annotations"].(map[string]any)["checksum/settings"].(string)
	}
	before := render(t, "capacity-room", required...)
	after := render(t, "capacity-room", append(required, "--set", "platform.worker_slots=4")...)
	for _, name := range []string{"capacity-room-serve", "capacity-room-worker"} {
		if checksum(before, name) == checksum(after, name) {
			t.Errorf("the checksum of %s did not change with the settings", name)
		}
	}
}

func TestMounts(t *testing.T) {
	objects := render(t, "capacity-room", append(required,
		"--set", "credentials.existingSecret=creds", "--set", "caBundles.existingConfigMap=cas")...)
	want := map[string]bool{
		"/tmp": false, "/opt/capacity-room/platform.toml": true, "/opt/capacity-room/secret": true,
		"/opt/capacity-room/credentials": true, "/opt/capacity-room/ca": true,
	}
	for name, spec := range pods(objects) {
		c := spec["containers"].([]any)[0].(map[string]any)
		got := map[string]bool{}
		for _, m := range c["volumeMounts"].([]any) {
			mount := m.(map[string]any)
			got[mount["mountPath"].(string)] = mount["readOnly"] == true
		}
		for path, readOnly := range want {
			if r, ok := got[path]; !ok || r != readOnly {
				t.Errorf("%s: mount %s missing or readOnly %v", name, path, r)
			}
		}
		for path := range got {
			if strings.HasPrefix(path, "/etc") {
				t.Errorf("%s mounts %s", name, path)
			}
		}
		dsn := c["env"].([]any)[0].(map[string]any)
		ref := dsn["valueFrom"].(map[string]any)["secretKeyRef"].(map[string]any)
		if dsn["name"] != "CAPACITYROOM_DATABASE_DSN" || ref["name"] != "db" || ref["key"] != "uri" {
			t.Errorf("%s: DSN env %v", name, dsn)
		}
	}
}

var dnsLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func TestLongReleaseName(t *testing.T) {
	release := strings.Repeat("a", 52) + "b"
	for _, o := range render(t, release, required...) {
		if name := o.name(); len(name) > 63 || !dnsLabel.MatchString(name) {
			t.Errorf("%s name %q (%s characters) is not a DNS label", o.kind(), name, strconv.Itoa(len(name)))
		}
	}
}
