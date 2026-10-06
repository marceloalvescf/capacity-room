package document

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// goldenNames lists the scenarios of testdata/snapshot/.
var goldenNames = []string{
	"direct-complete", "edge-scaling", "edge-values", "grafana-authenticated",
	"partial-data", "raw-fallback", "thanos-multicluster", "value-edge-cases",
}

func mustView(t *testing.T, s snapshot.Snapshot, lang catalog.Language) View {
	t.Helper()
	view, err := BuildView(s, mustCatalog(t, lang))
	if err != nil {
		t.Fatalf("BuildView() error = %v", err)
	}
	return view
}

// cellText joins the text of the runs of a cell.
func cellText(cell []Run) string {
	var out strings.Builder
	for _, r := range cell {
		if r.text != nil {
			out.WriteString(*r.text)
		}
	}
	return out.String()
}

// pdfTables walks the blocks of a document and calls visit for every table,
// with the name of the namespace whose heading stands above it.
func pdfTables(doc Document, visit func(namespace string, table Table)) {
	namespace := ""
	for _, blk := range doc.Blocks {
		switch x := blk.(type) {
		case Heading:
			if x.Level == 2 && !x.Numbered && len(x.Runs) == 2 && x.Runs[1].kind == "ident" {
				namespace = *x.Runs[1].text
			}
		case Table:
			visit(namespace, x)
		}
	}
}

// detailedDocument builds the PDF of s with the detailed profile.
func detailedDocument(t *testing.T, s snapshot.Snapshot, lang catalog.Language) Document {
	t.Helper()
	return mustBuild(t, s, mustCatalog(t, lang), Options{Profile: "detailed"})
}

func requireText(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, the PDF prints %q", what, got, want)
	}
}

func TestViewSaysWhatThePDFSays(t *testing.T) {
	var namespaces, workloads, hpas, nodes, reasons, recs int
	for _, name := range goldenNames {
		for _, lang := range catalog.Languages() {
			s := goldenSnapshot(t, name)
			s.Report.Profile = "detailed"
			view := mustView(t, s, lang)
			doc := detailedDocument(t, s, lang)
			where := name + "/" + string(lang)
			cat := mustCatalog(t, lang)

			viewNamespaces := map[string]ViewNamespace{}
			for _, n := range view.Namespaces {
				viewNamespaces[n.Name] = n
			}
			viewWorkloads := map[[2]string]ViewWorkload{}
			for _, w := range view.Workloads {
				if w.ObservedUsage {
					viewWorkloads[[2]string{w.Namespace, w.Name}] = w
				}
			}
			viewHPAs := map[[2]string]ViewHPA{}
			for _, h := range view.HPAs {
				viewHPAs[[2]string{h.Namespace, h.Name}] = h
			}
			viewNodes := map[string]ViewNode{}
			for _, n := range view.Nodes {
				viewNodes[n.Name] = n
			}

			seenNamespaces, seenWorkloads, seenNodes := map[string]bool{}, map[[2]string]bool{}, map[string]bool{}
			pdfTables(doc, func(namespace string, table Table) {
				switch table.ID {
				case "namespace-capacity":
					n, ok := viewNamespaces[namespace]
					if !ok {
						t.Errorf("%s: the PDF prints namespace %q, the view lacks it", where, namespace)
						return
					}
					seenNamespaces[namespace] = true
					namespaces++
					cpu, mem := table.Rows[0], table.Rows[1]
					requireText(t, where+" cpu usage average", n.CPUUsage.Average.Text, cellText(cpu[1]))
					requireText(t, where+" cpu usage peak", n.CPUUsage.Peak.Text, cellText(cpu[2]))
					requireText(t, where+" cpu requests average", n.CPURequests.Average.Text, cellText(cpu[3]))
					requireText(t, where+" cpu limits average", n.CPULimits.Average.Text, cellText(cpu[4]))
					requireText(t, where+" cpu request percent peak", n.CPURequestPercent.Peak.Text, cellText(cpu[5]))
					requireText(t, where+" cpu limit percent peak", n.CPULimitPercent.Peak.Text, cellText(cpu[6]))
					requireText(t, where+" memory usage average", n.MemoryUsage.Average.Text, cellText(mem[1]))
					requireText(t, where+" memory usage peak", n.MemoryUsage.Peak.Text, cellText(mem[2]))
					requireText(t, where+" memory requests average", n.MemoryRequests.Average.Text, cellText(mem[3]))
					requireText(t, where+" memory limits average", n.MemoryLimits.Average.Text, cellText(mem[4]))
					requireText(t, where+" memory request percent peak", n.MemoryRequestPercent.Peak.Text, cellText(mem[5]))
					requireText(t, where+" memory limit percent peak", n.MemoryLimitPercent.Peak.Text, cellText(mem[6]))
				case "workload-resources":
					for _, row := range table.Rows {
						key := [2]string{namespace, cellText(row[0])}
						w, ok := viewWorkloads[key]
						if !ok {
							t.Errorf("%s: the PDF prints workload %v, the view lacks it", where, key)
							continue
						}
						seenWorkloads[key] = true
						workloads++
						requireText(t, where+" workload pods", w.Pods.Text, cellText(row[1]))
						requireText(t, where+" workload cpu usage average", w.CPUUsage.Average.Text, cellText(row[2]))
						requireText(t, where+" workload cpu usage peak", w.CPUUsage.Peak.Text, cellText(row[3]))
						requireText(t, where+" workload cpu request percent", w.CPURequestPercent.Peak.Text, cellText(row[4]))
						requireText(t, where+" workload cpu limit percent", w.CPULimitPercent.Peak.Text, cellText(row[5]))
						requireText(t, where+" workload memory usage average", w.MemoryUsage.Average.Text, cellText(row[6]))
						requireText(t, where+" workload memory usage peak", w.MemoryUsage.Peak.Text, cellText(row[7]))
						requireText(t, where+" workload memory request percent", w.MemoryRequestPercent.Peak.Text, cellText(row[8]))
						requireText(t, where+" workload memory limit percent", w.MemoryLimitPercent.Peak.Text, cellText(row[9]))
					}
				case "hpa-observed":
					for _, row := range table.Rows {
						h, ok := viewHPAs[[2]string{namespace, cellText(row[0])}]
						if !ok {
							t.Errorf("%s: the PDF prints HPA %s/%s, the view lacks it", where, namespace, cellText(row[0]))
							continue
						}
						hpas++
						requireText(t, where+" hpa spec min", h.SpecMin.Text, cellText(row[1]))
						requireText(t, where+" hpa spec max", h.SpecMax.Text, cellText(row[2]))
						requireText(t, where+" hpa current average", h.Current.Average.Text, cellText(row[3]))
						requireText(t, where+" hpa minimum reached", h.Current.Minimum.Text, cellText(row[4]))
						requireText(t, where+" hpa maximum reached", h.Current.Peak.Text, cellText(row[5]))
						requireText(t, where+" hpa situation", h.Situation, cellText(row[6]))
					}
				case "node-detail":
					for _, row := range table.Rows {
						n, ok := viewNodes[cellText(row[0])]
						if !ok {
							t.Errorf("%s: the PDF prints node %q, the view lacks it", where, cellText(row[0]))
							continue
						}
						seenNodes[n.Name] = true
						nodes++
						requireText(t, where+" node cpu average", n.CPUUsagePercent.Average.Text, cellText(row[1][:1]))
						requireText(t, where+" node cpu peak", n.CPUUsagePercent.Peak.Text, cellText(row[1][2:]))
						requireText(t, where+" node memory average", n.MemoryUsagePercent.Average.Text, cellText(row[2][:1]))
						requireText(t, where+" node memory peak", n.MemoryUsagePercent.Peak.Text, cellText(row[2][2:]))
						requireText(t, where+" node reason", n.Reason, cellText(row[3]))
					}
				case "action-plan":
					plan := 0
					for _, row := range table.Rows {
						if plan >= len(view.Recommendations) {
							break
						}
						r := view.Recommendations[plan]
						plan++
						recs++
						shared := 0
						for _, other := range view.Recommendations {
							if other.Action == r.Action {
								shared++
							}
						}
						wantAction := r.Action
						if shared > 1 {
							wantAction = catText(t, cat, catalog.SectionsExecutiveActionQualified, catalog.Params{
								"Action":    r.Action,
								"Qualifier": catText(t, cat, catalog.ScopeQualifier(r.ScopeType), nil),
							})
						}
						requireText(t, where+" action", wantAction, cellText(row[2]))
						requireText(t, where+" scope", r.ScopeText, cellText(row[3]))
					}
					if len(table.Rows) != len(view.Recommendations) {
						t.Errorf("%s: the action plan has %d rows, the view %d recommendations", where, len(table.Rows), len(view.Recommendations))
					}
				case "priority-workloads":
					for _, row := range table.Rows {
						reasons++
						matched := false
						for _, w := range view.Workloads {
							if w.Namespace == cellText(row[0]) && (w.Name == cellText(row[1]) || w.Name+" ("+w.Kind+")" == cellText(row[1])) {
								matched = true
								requireText(t, where+" workload reason", w.Reason, cellText(row[3]))
							}
						}
						if !matched {
							t.Errorf("%s: the PDF prints priority workload %v, the view lacks it", where, rowString(row))
						}
					}
				}
			})

			for name := range viewNamespaces {
				if !seenNamespaces[name] {
					t.Errorf("%s: the view holds namespace %q, the PDF does not print it", where, name)
				}
			}
			for key := range viewWorkloads {
				if !seenWorkloads[key] {
					t.Errorf("%s: the view holds workload %v, the PDF does not print it", where, key)
				}
			}
			for name := range viewNodes {
				if !seenNodes[name] {
					t.Errorf("%s: the view holds node %q, the PDF does not print it", where, name)
				}
			}
			for _, w := range view.Workloads {
				if w.Reason != "" && w.Status == "healthy" {
					t.Errorf("%s: healthy workload %s/%s has the reason %q", where, w.Namespace, w.Name, w.Reason)
				}
			}
		}
	}
	for what, n := range map[string]int{"namespace": namespaces, "workload": workloads, "HPA": hpas, "node": nodes, "reason": reasons, "recommendation": recs} {
		if n == 0 {
			t.Errorf("no %s was compared: the test checks nothing", what)
		}
	}
}

func TestViewNarrowsAsThePDF(t *testing.T) {
	cases := []struct {
		name, profile, severity string
		wantMin                 string
		findings, recs          int
	}{
		{"all", "detailed", "all", "all", 26, 10},
		{"attention", "detailed", "attention", "attention", 19, 10},
		{"critical", "detailed", "critical", "critical", 9, 7},
		{"executive at all", "executive", "all", "attention", 19, 10},
		{"executive at critical", "executive", "critical", "critical", 9, 7},
	}
	for _, tc := range cases {
		for _, lang := range catalog.Languages() {
			t.Run(tc.name+"/"+string(lang), func(t *testing.T) {
				s := goldenSnapshot(t, "edge-scaling")
				s.Report.Profile, s.Report.MinSeverity = tc.profile, tc.severity
				view := mustView(t, s, lang)

				if view.MinSeverity != tc.wantMin {
					t.Errorf("min_severity = %q, want %q", view.MinSeverity, tc.wantMin)
				}
				if len(view.Findings) != tc.findings {
					t.Errorf("findings = %d, want %d", len(view.Findings), tc.findings)
				}
				if len(view.Recommendations) != tc.recs {
					t.Errorf("recommendations = %d, want %d", len(view.Recommendations), tc.recs)
				}

				// The rows are those the detailed PDF of the same
				// effective minimum prints.
				effective := s
				effective.Report.Profile, effective.Report.MinSeverity = "detailed", tc.wantMin
				doc := detailedDocument(t, effective, lang)
				var pdfNamespaces, pdfWorkloads, pdfNodes int
				pdfTables(doc, func(_ string, table Table) {
					switch table.ID {
					case "namespace-capacity":
						pdfNamespaces++
					case "workload-resources":
						pdfWorkloads += len(table.Rows)
					case "node-detail":
						pdfNodes += len(table.Rows)
					}
				})
				observed := 0
				for _, w := range view.Workloads {
					if w.ObservedUsage {
						observed++
					}
				}
				if len(view.Namespaces) != pdfNamespaces || observed != pdfWorkloads || len(view.Nodes) != pdfNodes {
					t.Errorf("namespaces, workloads, nodes = %d, %d, %d; the PDF prints %d, %d, %d",
						len(view.Namespaces), observed, len(view.Nodes), pdfNamespaces, pdfWorkloads, pdfNodes)
				}

				floor := analysis.Rank(tc.wantMin)
				if tc.wantMin == "all" {
					floor = analysis.Rank(analysis.SeverityHealthy)
				}
				for _, f := range view.Findings {
					if analysis.Rank(f.Severity) < floor {
						t.Errorf("finding %s of severity %s is below %s", f.Code, f.Severity, tc.wantMin)
					}
				}
				for _, r := range view.Recommendations {
					if analysis.Rank(r.Severity) < floor {
						t.Errorf("recommendation %s of severity %s is below %s", r.Code, r.Severity, tc.wantMin)
					}
				}
				if tc.profile == "executive" {
					for _, w := range view.Workloads {
						if w.Status == "healthy" {
							t.Errorf("the executive view keeps healthy workload %s/%s", w.Namespace, w.Name)
						}
					}
					for _, n := range view.Nodes {
						if n.Status == "healthy" {
							t.Errorf("the executive view keeps healthy node %s", n.Name)
						}
					}
					for _, n := range view.Namespaces {
						if n.Status == "healthy" {
							t.Errorf("the executive view keeps healthy namespace %s", n.Name)
						}
					}
				}

				// The action plan of the executive PDF lists the
				// recommendations of workloads, HPAs, namespaces, and the
				// cluster that the view keeps.
				executive := mustBuild(t, s, mustCatalog(t, lang), Options{Profile: "executive"})
				plan := 0
				pdfTables(executive, func(_ string, table Table) {
					if table.ID == "action-plan" {
						plan = len(table.Rows)
					}
				})
				want := 0
				for _, r := range view.Recommendations {
					if !strings.HasPrefix(r.Code, "NODE_") && !strings.HasPrefix(r.Code, "NODEPOOL_") {
						want++
					}
				}
				if plan != want {
					t.Errorf("the action plan has %d rows, the view %d recommendations outside nodes", plan, want)
				}
			})
		}
	}
}

func TestViewKeepsTheWholeOfTheRest(t *testing.T) {
	type rest struct {
		Summary   ViewSummary
		HPAs      []ViewHPA
		Nodepools []ViewNodepool
		Quality   ViewQuality
	}
	for _, lang := range catalog.Languages() {
		var first *rest
		for _, setting := range [][2]string{
			{"detailed", "all"}, {"detailed", "attention"}, {"detailed", "critical"},
			{"executive", "all"}, {"executive", "critical"},
		} {
			s := goldenSnapshot(t, "edge-scaling")
			s.Report.Profile, s.Report.MinSeverity = setting[0], setting[1]
			view := mustView(t, s, lang)
			got := rest{view.Summary, view.HPAs, view.Nodepools, view.Quality}
			if len(got.HPAs) != len(s.HPAs) || len(got.Nodepools) != len(s.Nodepools) || len(got.Quality.Capabilities) != len(s.Quality.Capabilities) {
				t.Errorf("%v: HPAs, node pools, capabilities = %d, %d, %d; the snapshot holds %d, %d, %d", setting,
					len(got.HPAs), len(got.Nodepools), len(got.Quality.Capabilities),
					len(s.HPAs), len(s.Nodepools), len(s.Quality.Capabilities))
			}
			if first == nil {
				first = &got
			} else if !reflect.DeepEqual(*first, got) {
				t.Errorf("%v: HPAs, node pools, summary, or quality differ from those at detailed all", setting)
			}
		}
	}
}

// registryFindings returns one finding of every code of the registry, with
// a value of the kind of each of its evidences.
func registryFindings() []snapshot.Finding {
	var out []snapshot.Finding
	for _, d := range analysis.Registry {
		f := snapshot.Finding{
			Code: d.Code, Severity: d.Severities[len(d.Severities)-1], ScopeType: d.ScopeType,
			ScopeName: "scope", Confidence: "exact", Evidence: []snapshot.Evidence{},
		}
		for _, spec := range d.Evidence {
			e := snapshot.Evidence{Name: spec.Name}
			switch spec.Kind {
			case analysis.KindBoolean:
				e.Value = snapshot.Value{Kind: "boolean", Boolean: true}
			case analysis.KindText:
				e.Value = snapshot.Value{Kind: "text", Text: "stored"}
			default:
				e.Value = snapshot.Value{Kind: "number", Number: 4}
			}
			f.Evidence = append(f.Evidence, e)
		}
		out = append(out, f)
	}
	return out
}

func TestViewFindingTexts(t *testing.T) {
	for _, lang := range catalog.Languages() {
		vb := &viewBuilder{b: &builder{cat: mustCatalog(t, lang)}}
		got := vb.findings(registryFindings(), 0)
		if vb.b.err != nil {
			t.Fatalf("%s: %v", lang, vb.b.err)
		}
		if len(got) != len(analysis.Registry) {
			t.Fatalf("%s: %d findings, the registry holds %d codes", lang, len(got), len(analysis.Registry))
		}
		for _, f := range got {
			if f.Sentence == "" || f.Action == "" {
				t.Errorf("%s: %s has sentence %q and action %q", lang, f.Code, f.Sentence, f.Action)
			}
		}
	}

	evidence := []snapshot.Evidence{
		{Name: "peak_percent", Value: snapshot.Value{Kind: "number", Number: 87.5}},
		{Name: "pods", Value: snapshot.Value{Kind: "number", Number: 3}},
		{Name: "gap", Value: snapshot.Value{Kind: "number", Number: 2.5}},
		{Name: "node_pressure", Value: snapshot.Value{Kind: "boolean", Boolean: true}},
		{Name: "idle", Value: snapshot.Value{Kind: "boolean", Boolean: false}},
		{Name: "resources", Value: snapshot.Value{Kind: "text", Text: "cpu, memory"}},
	}
	want := map[catalog.Language][]string{
		catalog.English:          {"87.5%", "3", "2.5", "yes", "no", "cpu, memory"},
		catalog.PortugueseBrazil: {"87,5%", "3", "2,5", "sim", "não", "cpu, memory"},
	}
	wantRaw := []any{87.5, 3.0, 2.5, true, false, "cpu, memory"}
	for lang, texts := range want {
		vb := &viewBuilder{b: &builder{cat: mustCatalog(t, lang)}}
		got := vb.evidence(evidence)
		for i, e := range got {
			if e.Text != texts[i] || !reflect.DeepEqual(e.Raw, wantRaw[i]) {
				t.Errorf("%s: evidence %s = raw %v, text %q; want raw %v, text %q", lang, e.Name, e.Raw, e.Text, wantRaw[i], texts[i])
			}
		}
	}
}

func TestViewMissingNumbers(t *testing.T) {
	for _, lang := range catalog.Languages() {
		cat := mustCatalog(t, lang)
		unavailable, noBase := catText(t, cat, catalog.PlaceholderUnavailable, nil), catText(t, cat, catalog.PlaceholderNoBase, nil)
		s := goldenSnapshot(t, "edge-values")
		view := mustView(t, s, lang)

		var absent int
		check := func(where string, stat Statistic, ratio bool) {
			for _, v := range []Value{stat.Average, stat.Minimum, stat.Peak, stat.Current} {
				if v.Raw != nil {
					continue
				}
				absent++
				want := unavailable
				if ratio && stat.State == "no_base" {
					want = noBase
				}
				if v.Text != want {
					t.Errorf("%s/%s: absent number has text %q, want %q", lang, where, v.Text, want)
				}
			}
		}
		for _, n := range view.Namespaces {
			check(n.Name+" cpu usage", n.CPUUsage, false)
			check(n.Name+" cpu request percent", n.CPURequestPercent, true)
			check(n.Name+" memory limit percent", n.MemoryLimitPercent, true)
		}
		for _, w := range view.Workloads {
			check(w.Name+" cpu usage", w.CPUUsage, false)
			check(w.Name+" cpu request percent", w.CPURequestPercent, true)
			check(w.Name+" memory limit percent", w.MemoryLimitPercent, true)
			if w.Pods.Raw == nil && w.Pods.Text != unavailable {
				t.Errorf("%s/%s: absent pods have text %q", lang, w.Name, w.Pods.Text)
			}
		}
		for _, h := range view.HPAs {
			check(h.Name+" current", h.Current, false)
			check(h.Name+" desired", h.Desired, false)
		}
		for _, n := range view.Nodes {
			check(n.Name+" cpu usage", n.CPUUsagePercent, false)
			for _, v := range []Value{n.CPUCapacity, n.MemoryCapacity, n.CPUAllocatable, n.MemoryAllocatable, n.CPURequestPercent, n.MemoryRequestPercent} {
				if v.Raw == nil && v.Text != unavailable {
					t.Errorf("%s/%s: absent node number has text %q", lang, n.Name, v.Text)
				}
			}
		}
		if absent == 0 {
			t.Errorf("%s: edge-values holds no absent number: the test checks nothing", lang)
		}
	}
}

func TestViewIsDeterministic(t *testing.T) {
	for _, name := range goldenNames {
		s := goldenSnapshot(t, name)
		first, err := EncodeView(mustView(t, s, catalog.English))
		if err != nil {
			t.Fatalf("EncodeView() error = %v", err)
		}
		second, err := EncodeView(mustView(t, s, catalog.English))
		if err != nil {
			t.Fatalf("EncodeView() error = %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("%s: two views of one snapshot differ", name)
		}
		if !json.Valid(first) || bytes.Contains(first, []byte(`<`)) {
			t.Errorf("%s: the encoding is not plain JSON", name)
		}
	}
}

// TestViewListsAreNeverNull builds the view past Check, from rows whose
// lists are nil, and requires every list to encode as [].
func TestViewListsAreNeverNull(t *testing.T) {
	s := baseSnapshot(t)
	s.Scope.Namespaces = nil
	s.Nodes = []snapshot.Node{{Name: "n1", Pool: "p", PoolConfidence: "exact", CPUUsagePercent: snapshot.Statistics{State: stateUnavailable}, MemoryUsagePercent: snapshot.Statistics{State: stateUnavailable}}}
	s.Nodepools = []snapshot.NodePool{{Name: "p", Confidence: "exact"}}
	s.Findings = []snapshot.Finding{{Code: "NAMESPACE_CPU_LIMIT_CRITICAL", Severity: "critical", ScopeType: "namespace", ScopeName: "payments", Confidence: "exact"}}
	s.Recommendations = []snapshot.Recommendation{{Code: "NAMESPACE_CPU_LIMIT_CRITICAL", Severity: "critical", ScopeType: "namespace", Confidence: "exact"}}
	for i := range s.Quality.Capabilities {
		s.Quality.Capabilities[i].Evidence = nil
	}
	s.Quality.Warnings, s.Quality.Notices = nil, nil
	view, err := buildView(s, mustCatalog(t, catalog.English), "all")
	if err != nil {
		t.Fatalf("buildView() error = %v", err)
	}
	data, err := EncodeView(view)
	if err != nil {
		t.Fatalf("EncodeView() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	first := func(key string) map[string]any { return decoded[key].([]any)[0].(map[string]any) }
	quality := decoded["quality"].(map[string]any)
	for what, v := range map[string]any{
		"report.namespaces":        decoded["report"].(map[string]any)["namespaces"],
		"workloads":                decoded["workloads"],
		"hpas":                     decoded["hpas"],
		"nodepools[].nodes":        first("nodepools")["nodes"],
		"recommendations[].scopes": first("recommendations")["scopes"],
		"findings[].evidence":      first("findings")["evidence"],
		"capabilities[].evidence":  quality["capabilities"].([]any)[0].(map[string]any)["evidence"],
		"quality.warnings":         quality["warnings"],
		"quality.notices":          quality["notices"],
	} {
		if list, ok := v.([]any); !ok || len(list) != 0 {
			t.Errorf("%s = %v, want an empty list", what, v)
		}
	}
}

// A recommendation is never healthy in a snapshot that passes Check, so the
// view is built past Check: the plan of the PDF raises the minimum to
// attention in both profiles, and the view must list what it lists.
func TestViewRaisesThePlanMinimum(t *testing.T) {
	for _, lang := range catalog.Languages() {
		cat := mustCatalog(t, lang)
		s := baseSnapshot(t)
		s.Recommendations = []snapshot.Recommendation{{
			Code: "NAMESPACE_CPU_LIMIT_HEALTHY", Severity: "healthy", ScopeType: "namespace",
			Scopes: []string{"payments"}, Confidence: "exact",
		}}
		view, err := buildView(s, cat, "all")
		if err != nil {
			t.Fatalf("buildView() error = %v", err)
		}
		b := &builder{cat: cat}
		blocks := actionPlan(b, s, executiveMinimum("all"))
		plan := 0
		for _, blk := range blocks {
			if table, ok := blk.(Table); ok {
				plan = len(table.Rows)
			}
		}
		if plan != 0 || len(view.Recommendations) != plan {
			t.Errorf("%s: the plan has %d rows, the view %d recommendations", lang, plan, len(view.Recommendations))
		}
	}
}

func TestViewRefusesWhatCheckRefuses(t *testing.T) {
	s := goldenSnapshot(t, "direct-complete")
	s.Report.ID = ""
	want := s.Check()
	if want == nil {
		t.Fatal("Check() accepted a snapshot with no report id")
	}
	_, err := BuildView(s, mustCatalog(t, catalog.English))
	if err == nil || err.Error() != want.Error() {
		t.Errorf("BuildView() error = %v, want %v", err, want)
	}

	s = goldenSnapshot(t, "direct-complete")
	s.Report.MinSeverity = "healthy"
	if _, err := BuildView(s, mustCatalog(t, catalog.English)); err == nil {
		t.Error("BuildView() accepted an unknown minimum severity")
	}
}
