package profiles

import (
	"reflect"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/config"
)

func sp(s string) *string { return &s }

func validExample() Document {
	return Document{
		DisplayName:             "Production",
		Active:                  true,
		EndpointURL:             "https://metrics.internal",
		TimeoutSeconds:          90,
		TLSVerify:               true,
		CredentialFile:          "prod-token",
		CABundleFile:            "corp-root",
		ClusterName:             "prod-east",
		ClusterLabel:            "cluster",
		FilterMode:              "required",
		NodepoolLabel:           "nodepool",
		NodeAutoscaling:         "auto",
		AllowedNamespaces:       []string{"payments", "checkout"},
		ExcludedContainers:      []string{"istio-proxy"},
		PermittedPeriods:        []string{"24h", "7d"},
		DefaultPeriod:           sp("7d"),
		PermittedTimezones:      []string{"UTC", "America/Sao_Paulo"},
		DefaultTimezone:         sp("UTC"),
		PermittedReportProfiles: []string{"detailed", "executive"},
		DefaultReportProfile:    sp("executive"),
		Thresholds:              config.Default().Thresholds,
	}
}

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.TimeoutSeconds != 90 || !d.TLSVerify ||
		d.ClusterLabel != "cluster" || d.FilterMode != "required" || d.NodeAutoscaling != "auto" {
		t.Fatalf("scalar defaults wrong: %+v", d)
	}
	want := config.Thresholds{
		WarningPercent: 70, CriticalPercent: 85, LowUtilizationPercent: 30,
		AutoscalerNearMaxPercent: 80, NodepoolSpreadPoints: 30, BurstyGapPercent: 100, HPAReplicaDivergence: 1,
	}
	if d.Thresholds != want {
		t.Fatalf("thresholds = %+v", d.Thresholds)
	}
	for name, l := range map[string][]string{
		"allowed": d.AllowedNamespaces, "excluded": d.ExcludedContainers, "periods": d.PermittedPeriods,
		"zones": d.PermittedTimezones, "profiles": d.PermittedReportProfiles,
	} {
		if l == nil || len(l) != 0 {
			t.Fatalf("%s should be an empty list, got %#v", name, l)
		}
	}
	if d.DefaultPeriod != nil || d.DefaultTimezone != nil || d.DefaultReportProfile != nil {
		t.Fatal("defaults must be null")
	}
}

func TestValidateAcceptsTheExample(t *testing.T) {
	if got := validExample().Validate(); got != nil {
		t.Fatalf("problems = %v", got)
	}
}

func TestValidateEachField(t *testing.T) {
	long := strings.Repeat("a", 254)
	cases := []struct {
		name   string
		mutate func(*Document)
		field  string
		word   string
	}{
		{"display empty", func(d *Document) { d.DisplayName = "  " }, "display_name", "empty"},
		{"display long", func(d *Document) { d.DisplayName = strings.Repeat("a", 129) }, "display_name", "invalid"},
		{"display control", func(d *Document) { d.DisplayName = "a\nb" }, "display_name", "invalid"},
		{"url missing", func(d *Document) { d.EndpointURL = "" }, "endpoint_url", "missing"},
		{"timeout zero", func(d *Document) { d.TimeoutSeconds = 0 }, "timeout_seconds", "invalid"},
		{"timeout big", func(d *Document) { d.TimeoutSeconds = 601 }, "timeout_seconds", "invalid"},
		{"timeout nan", func(d *Document) { d.TimeoutSeconds = nan() }, "timeout_seconds", "invalid"},
		{"credential name", func(d *Document) { d.CredentialFile = "../x" }, "credential_file", "invalid"},
		{"ca name", func(d *Document) { d.CABundleFile = "Corp Root" }, "ca_bundle_file", "invalid"},
		{"ca incoherent", func(d *Document) { d.TLSVerify = false }, "ca_bundle_file", "incoherent"},
		{"cluster empty", func(d *Document) { d.ClusterName = " " }, "cluster_name", "empty"},
		{"cluster control", func(d *Document) { d.ClusterName = "a\tb" }, "cluster_name", "invalid"},
		{"label invalid", func(d *Document) { d.ClusterLabel = "1bad" }, "cluster_label", "invalid"},
		{"filter invalid", func(d *Document) { d.FilterMode = "x" }, "filter_mode", "invalid"},
		{"nodepool invalid", func(d *Document) { d.NodepoolLabel = "a-b" }, "nodepool_label", "invalid"},
		{"autoscaling invalid", func(d *Document) { d.NodeAutoscaling = "x" }, "node_autoscaling", "invalid"},
		{"namespaces empty", func(d *Document) { d.AllowedNamespaces = nil }, "allowed_namespaces", "empty"},
		{"namespace invalid", func(d *Document) { d.AllowedNamespaces = []string{"Bad_NS"} }, "allowed_namespaces", "invalid"},
		{"namespace repeated", func(d *Document) { d.AllowedNamespaces = []string{"a", "a"} }, "allowed_namespaces", "repeated"},
		{"container invalid", func(d *Document) { d.ExcludedContainers = []string{"-x"} }, "excluded_containers", "invalid"},
		{"container long", func(d *Document) { d.ExcludedContainers = []string{long} }, "excluded_containers", "invalid"},
		{"container repeated", func(d *Document) { d.ExcludedContainers = []string{"a", "a"} }, "excluded_containers", "repeated"},
		{"period invalid", func(d *Document) { d.PermittedPeriods = []string{"7d", "x"} }, "permitted_periods", "invalid"},
		{"period repeated", func(d *Document) { d.PermittedPeriods = []string{"7d", "7d"} }, "permitted_periods", "repeated"},
		{"default period invalid", func(d *Document) { d.DefaultPeriod = sp("x") }, "default_period", "invalid"},
		{"default period not permitted", func(d *Document) { d.DefaultPeriod = sp("30d") }, "default_period", "not_permitted"},
		{"zone invalid", func(d *Document) { d.PermittedTimezones = []string{"UTC", "Mars/Base"} }, "permitted_timezones", "invalid"},
		{"zone repeated", func(d *Document) { d.PermittedTimezones = []string{"UTC", "UTC"} }, "permitted_timezones", "repeated"},
		{"default zone invalid", func(d *Document) { d.DefaultTimezone = sp("Mars/Base") }, "default_timezone", "invalid"},
		{"default zone not permitted", func(d *Document) { d.DefaultTimezone = sp("Europe/Paris") }, "default_timezone", "not_permitted"},
		{"profile invalid", func(d *Document) { d.PermittedReportProfiles = []string{"executive", "full"} }, "permitted_report_profiles", "invalid"},
		{"profile repeated", func(d *Document) { d.PermittedReportProfiles = []string{"executive", "executive"} }, "permitted_report_profiles", "repeated"},
		{"default profile invalid", func(d *Document) { d.DefaultReportProfile = sp("full") }, "default_report_profile", "invalid"},
		{"default profile not permitted", func(d *Document) {
			d.PermittedReportProfiles = []string{"detailed"}
		}, "default_report_profile", "not_permitted"},
		{"thresholds", func(d *Document) { d.Thresholds.WarningPercent = 90 }, "thresholds", "invalid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := validExample()
			c.mutate(&d)
			want := []Problem{{Field: c.field, Word: c.word}}
			if got := d.Validate(); !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

func TestValidateReportsEveryField(t *testing.T) {
	d := validExample()
	d.Thresholds.CriticalPercent = 10
	d.DisplayName = ""
	want := []Problem{{"display_name", "empty"}, {"thresholds", "invalid"}}
	if got := d.Validate(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestValidateReferenceNames(t *testing.T) {
	for _, bad := range []string{"../corp-root", "/etc/ssl/certs/corp.pem", "corp root", "Corp-Root", ".", strings.Repeat("a", 64), "a..b", ""} {
		if ValidReferenceName(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
	for _, good := range []string{"corp-root", "a.b_c-d", strings.Repeat("a", 63)} {
		if !ValidReferenceName(good) {
			t.Errorf("%q refused", good)
		}
	}
}

func TestDocumentDefaultsWithinPermitted(t *testing.T) {
	cases := []struct {
		permitted []string
		def       *string
		want      []Problem
	}{
		{[]string{"7d"}, nil, []Problem{{"default_period", "not_permitted"}}},
		{[]string{"7d"}, sp("7d"), nil},
		{[]string{}, nil, nil},
	}
	for i, c := range cases {
		d := validExample()
		d.PermittedPeriods, d.DefaultPeriod = c.permitted, c.def
		if got := d.Validate(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("case %d: got %v, want %v", i, got, c.want)
		}
	}
}

func TestResolvedDefaults(t *testing.T) {
	p, z, r := Defaults().ResolvedDefaults()
	if p != "24h" || z != "America/Sao_Paulo" || r != "detailed" {
		t.Fatalf("fallbacks = %q %q %q", p, z, r)
	}
	p, z, r = validExample().ResolvedDefaults()
	if p != "7d" || z != "UTC" || r != "executive" {
		t.Fatalf("set = %q %q %q", p, z, r)
	}
}

func TestRunConfig(t *testing.T) {
	d := validExample()
	d.TimeoutSeconds = 45
	d.NodeAutoscaling = "enabled"
	d.NodepoolLabel = "pool"
	got := d.RunConfig("7d", "", "", "UTC", "executive", []string{"payments"})
	want := config.Default()
	want.Metrics = config.Metrics{URL: "https://metrics.internal", TLSVerify: true, TimeoutSeconds: 45}
	want.Cluster = config.Cluster{Name: "prod-east", Namespaces: []string{"payments"}, Label: "cluster", FilterMode: "required", NodepoolLabel: "pool"}
	want.Analysis = config.Analysis{Period: "7d", Timezone: "UTC", NodeAutoscaling: "enabled", ExcludedContainers: []string{"istio-proxy"}}
	want.Report.Profile = "executive"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if got.Metrics.TokenEnv != "" {
		t.Fatal("token env must stay empty")
	}
}

func nan() float64 { var z float64; return z / z }

func TestRunConfigCarriesARange(t *testing.T) {
	got := validExample().RunConfig("", "2026-09-01 00:00", "2026-10-01 00:00", "UTC", "executive", []string{"payments"})
	a := got.Analysis
	if a.Period != "" || a.Start != "2026-09-01 00:00" || a.End != "2026-10-01 00:00" || a.Timezone != "UTC" {
		t.Fatalf("analysis = %+v", a)
	}
}
