package document

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
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

// cellStatus is the status of the status run of a cell, or nil when the cell
// has none.
func cellStatus(cell []Run) *string {
	for _, r := range cell {
		if r.kind == "status" {
			status := r.status
			return &status
		}
	}
	return nil
}

func statusText(status *string) string {
	if status == nil {
		return "null"
	}
	return *status
}

func requireStatus(t *testing.T, what string, got *string, cell []Run) {
	t.Helper()
	if want := cellStatus(cell); statusText(got) != statusText(want) {
		t.Errorf("%s status = %s, the PDF prints %s", what, statusText(got), statusText(want))
	}
}

// requireValue compares the text and the mark of a value with a cell.
func requireValue(t *testing.T, what string, got Value, cell []Run) {
	t.Helper()
	requireText(t, what, got.Text, cellText(cell))
	requireStatus(t, what, got.Status, cell)
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
					requireValue(t, where+" cpu usage average", n.CPUUsage.Average, cpu[1])
					requireValue(t, where+" cpu usage peak", n.CPUUsage.Peak, cpu[2])
					requireValue(t, where+" cpu requests average", n.CPURequests.Average, cpu[3])
					requireValue(t, where+" cpu limits average", n.CPULimits.Average, cpu[4])
					requireValue(t, where+" cpu request percent peak", n.CPURequestPercent.Peak, cpu[5])
					requireValue(t, where+" cpu limit percent peak", n.CPULimitPercent.Peak, cpu[6])
					requireValue(t, where+" memory usage average", n.MemoryUsage.Average, mem[1])
					requireValue(t, where+" memory usage peak", n.MemoryUsage.Peak, mem[2])
					requireValue(t, where+" memory requests average", n.MemoryRequests.Average, mem[3])
					requireValue(t, where+" memory limits average", n.MemoryLimits.Average, mem[4])
					requireValue(t, where+" memory request percent peak", n.MemoryRequestPercent.Peak, mem[5])
					requireValue(t, where+" memory limit percent peak", n.MemoryLimitPercent.Peak, mem[6])
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
						requireValue(t, where+" workload pods", w.Pods, row[1])
						requireValue(t, where+" workload cpu usage average", w.CPUUsage.Average, row[2])
						requireValue(t, where+" workload cpu usage peak", w.CPUUsage.Peak, row[3])
						requireValue(t, where+" workload cpu request percent", w.CPURequestPercent.Peak, row[4])
						requireValue(t, where+" workload cpu limit percent", w.CPULimitPercent.Peak, row[5])
						requireValue(t, where+" workload memory usage average", w.MemoryUsage.Average, row[6])
						requireValue(t, where+" workload memory usage peak", w.MemoryUsage.Peak, row[7])
						requireValue(t, where+" workload memory request percent", w.MemoryRequestPercent.Peak, row[8])
						requireValue(t, where+" workload memory limit percent", w.MemoryLimitPercent.Peak, row[9])
					}
				case "hpa-observed":
					for _, row := range table.Rows {
						h, ok := viewHPAs[[2]string{namespace, cellText(row[0])}]
						if !ok {
							t.Errorf("%s: the PDF prints HPA %s/%s, the view lacks it", where, namespace, cellText(row[0]))
							continue
						}
						hpas++
						requireValue(t, where+" hpa spec min", h.SpecMin, row[1])
						requireValue(t, where+" hpa spec max", h.SpecMax, row[2])
						requireValue(t, where+" hpa current average", h.Current.Average, row[3])
						requireValue(t, where+" hpa minimum reached", h.Current.Minimum, row[4])
						requireValue(t, where+" hpa maximum reached", h.Current.Peak, row[5])
						requireText(t, where+" hpa situation", h.Situation, cellText(row[6]))
						requireStatus(t, where+" hpa situation", h.SituationStatus, row[6])
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
						requireValue(t, where+" node cpu average", n.CPUUsagePercent.Average, row[1][:1])
						requireValue(t, where+" node cpu peak", n.CPUUsagePercent.Peak, row[1][2:])
						requireValue(t, where+" node memory average", n.MemoryUsagePercent.Average, row[2][:1])
						requireValue(t, where+" node memory peak", n.MemoryUsagePercent.Peak, row[2][2:])
						requireText(t, where+" node reason", n.Reason, cellText(row[3]))
						requireStatus(t, where+" node reason", n.ReasonStatus, row[3])
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
						requireText(t, where+" action", r.Action, cellText(row[2]))
						requireText(t, where+" scope", catText(t, mustCatalog(t, lang), catalog.Scope(r.ScopeType), nil)+" "+r.ScopeText, cellText(row[3]))
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
		{"all", "detailed", "all", "all", 28, 10},
		{"attention", "detailed", "attention", "attention", 21, 10},
		{"critical", "detailed", "critical", "critical", 9, 7},
		{"executive at all", "executive", "all", "attention", 21, 10},
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
		got := vb.findings(registryFindings())
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

// marks lists the status of the four numbers of a statistic, "null" for none.
func marks(s Statistic) [4]string {
	return [4]string{statusText(s.Average.Status), statusText(s.Minimum.Status), statusText(s.Peak.Status), statusText(s.Current.Status)}
}

func TestViewMarksEveryNumberAsThePDFDoes(t *testing.T) {
	direct := mustView(t, goldenSnapshot(t, "direct-complete"), catalog.English)
	ns := direct.Namespaces[0]
	for name, c := range map[string]struct {
		got  [4]string
		want [4]string
	}{
		"cpu request percent":    {marks(ns.CPURequestPercent), [4]string{"healthy", "healthy", "attention", "attention"}},
		"memory request percent": {marks(ns.MemoryRequestPercent), [4]string{"critical", "critical", "critical", "critical"}},
		"cpu limit percent":      {marks(ns.CPULimitPercent), [4]string{"healthy", "healthy", "healthy", "healthy"}},
		"cpu usage":              {marks(ns.CPUUsage), [4]string{"null", "null", "null", "null"}},
		"memory requests":        {marks(ns.MemoryRequests), [4]string{"null", "null", "null", "null"}},
		"node cpu usage":         {marks(direct.Nodes[0].CPUUsagePercent), [4]string{"healthy", "healthy", "healthy", "healthy"}},
		"hpa current":            {marks(direct.HPAs[0].Current), [4]string{"null", "null", "null", "null"}},
	} {
		if c.got != c.want {
			t.Errorf("%s: marks %v, want %v", name, c.got, c.want)
		}
	}
	unmeasured := direct.Workloads[1]
	if got, want := marks(unmeasured.CPURequestPercent), [4]string{"unavailable", "unavailable", "unavailable", "unavailable"}; got != want {
		t.Errorf("unmeasured ratio: marks %v, want %v", got, want)
	}
	if got, want := marks(unmeasured.CPUUsage), [4]string{"null", "null", "null", "null"}; got != want {
		t.Errorf("unmeasured usage: marks %v, want %v", got, want)
	}

	nodes := direct.Nodes[0]
	for name, status := range map[string]*string{
		"pods": ns.Pods.Status, "cpu capacity": nodes.CPUCapacity.Status, "cpu request percent": nodes.CPURequestPercent.Status,
		"summary findings": direct.Summary.Findings.Status, "spec max": direct.HPAs[0].SpecMax.Status,
		"managed node groups": direct.Summary.Autoscaling.ManagedNodeGroups.Status,
	} {
		if status != nil {
			t.Errorf("%s carries the mark %q, the PDF prints none", name, *status)
		}
	}
	if got := statusText(direct.HPAs[0].SituationStatus); got != "healthy" {
		t.Errorf("hpa situation mark %s, want healthy", got)
	}
	if got := statusText(nodes.ReasonStatus); got != "healthy" {
		t.Errorf("node reason mark %s, want healthy", got)
	}

	edge := mustView(t, goldenSnapshot(t, "edge-values"), catalog.English)
	if got, want := marks(edge.Namespaces[0].CPURequestPercent), [4]string{"undefined", "undefined", "undefined", "undefined"}; got != want {
		t.Errorf("a ratio without a base: marks %v, want %v", got, want)
	}
}

func TestViewMarksANodeRequestShareWithItsFinding(t *testing.T) {
	for _, n := range mustView(t, goldenSnapshot(t, "direct-complete"), catalog.English).Nodes {
		if n.CPURequestPercent.Status != nil || n.MemoryRequestPercent.Status != nil {
			t.Errorf("%s has no request finding, marks %v and %v, want none", n.Name, n.CPURequestPercent.Status, n.MemoryRequestPercent.Status)
		}
	}

	marked := 0
	for _, n := range mustView(t, goldenSnapshot(t, "edge-scaling"), catalog.English).Nodes {
		if n.Name != "pool-a-1" {
			continue
		}
		marked++
		if got := statusText(n.CPURequestPercent.Status); got != "critical" {
			t.Errorf("pool-a-1 cpu request percent mark %s, want critical", got)
		}
		if got := statusText(n.MemoryRequestPercent.Status); got != "null" {
			t.Errorf("pool-a-1 memory request percent mark %s, want null", got)
		}
	}
	if marked != 1 {
		t.Fatalf("edge-scaling holds %d nodes named pool-a-1, want 1", marked)
	}
}

func TestViewVerdictPointsAtWhatThePDFWords(t *testing.T) {
	for _, name := range goldenNames {
		s := goldenSnapshot(t, name)
		view := mustView(t, s, catalog.English)
		pdf := englishVerdict(t, s, view.MinSeverity)
		where := name
		v := view.Verdict

		switch {
		case v.ClusterFinding == nil && pdf.Growth.Finding != nil, v.ClusterFinding != nil && pdf.Growth.Finding == nil:
			t.Fatalf("%s: cluster finding %v, PDF finding %v", where, v.ClusterFinding, pdf.Growth.Finding)
		case v.ClusterFinding != nil:
			if got := view.Findings[*v.ClusterFinding].Sentence; got != pdf.Growth.Finding.Sentence {
				t.Errorf("%s: cluster finding %q, PDF %q", where, got, pdf.Growth.Finding.Sentence)
			}
		}

		if len(v.Pressure) != len(pdf.Pressure.Rows) {
			t.Fatalf("%s: %d pressure rows, PDF %d", where, len(v.Pressure), len(pdf.Pressure.Rows))
		}
		for i, p := range v.Pressure {
			row := pdf.Pressure.Rows[i]
			if got := view.Findings[p.Finding].Sentence; got != row.Sentence {
				t.Errorf("%s: pressure %d sentence %q, PDF %q", where, i, got, row.Sentence)
			}
			if p.Severity != row.Status {
				t.Errorf("%s: pressure %d severity %q, PDF %q", where, i, p.Severity, row.Status)
			}
			counts := map[string]string{}
			for _, c := range row.Counts {
				counts[c.Status] = c.Count
			}
			if got := strconv.Itoa(p.Critical); got != zero(counts["critical"]) {
				t.Errorf("%s: pressure %d critical %s, PDF %s", where, i, got, counts["critical"])
			}
			if got := strconv.Itoa(p.Attention); got != zero(counts["attention"]) {
				t.Errorf("%s: pressure %d attention %s, PDF %s", where, i, got, counts["attention"])
			}
		}

		g := pdf.Growth.Gauge
		switch {
		case (v.Gauge == nil) != (g == nil):
			t.Fatalf("%s: gauge %v, PDF gauge %v", where, v.Gauge, g)
		case g != nil:
			if v.Gauge.Current != g.Current || v.Gauge.Max != g.Max || !reflect.DeepEqual(v.Gauge.Min, g.Min) || (v.Gauge.Hazard != nil) != g.Hazard {
				t.Errorf("%s: gauge %+v, PDF %+v", where, *v.Gauge, *g)
			}
		}

		switch {
		case (v.NodeEvidenceWarning == nil) != (pdf.Growth.Warning == nil):
			t.Fatalf("%s: evidence warning %v, PDF %v", where, v.NodeEvidenceWarning, pdf.Growth.Warning)
		case v.NodeEvidenceWarning != nil:
			if got := view.Quality.Warnings[*v.NodeEvidenceWarning].Text; got != *pdf.Growth.Warning {
				t.Errorf("%s: evidence warning %q, PDF %q", where, got, *pdf.Growth.Warning)
			}
		}
	}
}

func zero(count string) string {
	if count == "" {
		return "0"
	}
	return count
}

func TestViewVerdictOfEdgeScaling(t *testing.T) {
	v := mustView(t, goldenSnapshot(t, "edge-scaling"), catalog.English).Verdict
	want := ViewVerdict{
		ClusterFinding:      intPtr(10),
		NodeEvidenceWarning: intPtr(1),
		Gauge:               &ViewGauge{Current: 3, Max: 3, Min: intPtr(2), Hazard: strPtr("attention")},
		Pressure: []ViewPressure{
			{ScopeType: "node", Severity: "critical", Finding: 0, Critical: 2, Attention: 1},
			{ScopeType: "namespace", Severity: "critical", Finding: 1, Critical: 1, Attention: 0},
			{ScopeType: "workload", Severity: "critical", Finding: 5, Critical: 1, Attention: 1},
			{ScopeType: "hpa", Severity: "critical", Finding: 7, Critical: 1, Attention: 1},
			{ScopeType: "nodepool", Severity: "attention", Finding: 14, Critical: 0, Attention: 2},
		},
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Verdict = %+v, want %+v", v, want)
	}
}

func TestViewVerdictOfTheExecutiveProfile(t *testing.T) {
	s := goldenSnapshot(t, "edge-scaling")
	s.Report.Profile = "executive"
	view := mustView(t, s, catalog.English)
	v := view.Verdict
	if view.MinSeverity != "attention" {
		t.Fatalf("MinSeverity = %q, want attention", view.MinSeverity)
	}
	pdf := englishVerdict(t, s, view.MinSeverity)
	if v.ClusterFinding == nil || pdf.Growth.Finding == nil {
		t.Fatalf("cluster finding %v, PDF %v, want both", v.ClusterFinding, pdf.Growth.Finding)
	}
	if f := view.Findings[*v.ClusterFinding]; f.ScopeType != "cluster" || f.Sentence != pdf.Growth.Finding.Sentence {
		t.Errorf("cluster finding is %s %q, PDF %q", f.ScopeType, f.Sentence, pdf.Growth.Finding.Sentence)
	}
	if len(v.Pressure) != len(pdf.Pressure.Rows) {
		t.Fatalf("%d pressure rows, PDF %d", len(v.Pressure), len(pdf.Pressure.Rows))
	}
	for i, p := range v.Pressure {
		f := view.Findings[p.Finding]
		if f.ScopeType != p.ScopeType || f.Severity != p.Severity || p.Severity == "healthy" || f.Sentence != pdf.Pressure.Rows[i].Sentence {
			t.Errorf("pressure %d points at %s %s %q, row is %s %s, PDF %q", i, f.ScopeType, f.Severity, f.Sentence, p.ScopeType, p.Severity, pdf.Pressure.Rows[i].Sentence)
		}
	}
}

func TestViewVerdictWithoutAutoscaling(t *testing.T) {
	s := goldenSnapshot(t, "edge-scaling")
	s.Summary.Autoscaling.Status = "disabled"
	if g := viewOf(t, s).Verdict.Gauge; g != nil {
		t.Errorf("Gauge = %+v, want none", g)
	}
}

func TestViewRecommendationFinding(t *testing.T) {
	s := goldenSnapshot(t, "edge-scaling")
	view := mustView(t, s, catalog.English)
	pointed := 0
	for _, r := range view.Recommendations {
		if r.Finding == nil {
			continue
		}
		pointed++
		f := view.Findings[*r.Finding]
		if f.Code != r.Code || f.ScopeType != r.ScopeType || f.ScopeName != r.Scopes[0] {
			t.Errorf("recommendation %s on %v points at %s on %s %s", r.Code, r.Scopes, f.Code, f.ScopeType, f.ScopeName)
		}
	}
	if pointed == 0 {
		t.Error("no recommendation points at a finding")
	}

	r := snapshot.Recommendation{Code: "NAMESPACE_CPU_LIMIT_CRITICAL", ScopeType: "namespace", Scopes: []string{"absent"}}
	if got := recommendationFinding(r, s.Findings); got != nil {
		t.Errorf("recommendationFinding() = %d, want none", *got)
	}
	r.Scopes = nil
	if got := recommendationFinding(r, s.Findings); got != nil {
		t.Errorf("recommendationFinding() without scopes = %d, want none", *got)
	}
}

// viewOf builds the view past Check, so a summary Check refuses can be tried.
func viewOf(t *testing.T, s snapshot.Snapshot) View {
	t.Helper()
	view, err := buildView(s, mustCatalog(t, catalog.English), "all")
	if err != nil {
		t.Fatalf("buildView() error = %v", err)
	}
	return view
}

func TestViewVerdictPressureIsNeverNull(t *testing.T) {
	s := goldenSnapshot(t, "edge-scaling")
	s.Findings = nil
	data, err := EncodeView(viewOf(t, s))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Verdict struct {
			Pressure []any `json:"pressure"`
		} `json:"verdict"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Verdict.Pressure == nil || len(decoded.Verdict.Pressure) != 0 {
		t.Errorf("verdict.pressure = %v, want an empty list", decoded.Verdict.Pressure)
	}
}

func TestNodeStateStatus(t *testing.T) {
	yes, no := boolPtr(true), boolPtr(false)
	tests := []struct {
		name               string
		ready, schedulable *bool
		findings           []snapshot.Finding
		want               string
		wantView           string
	}{
		{"unknown", nil, nil, nil, "unavailable", "unavailable"},
		{"not ready", no, yes, []snapshot.Finding{nodeFinding("NODE_NOT_READY", analysis.SeverityAttention, "n1")}, "attention", "attention"},
		{"cordoned", yes, no, []snapshot.Finding{nodeFinding("NODE_UNSCHEDULABLE", analysis.SeverityAttention, "n1")}, "attention", "attention"},
		{"fine", yes, yes, nil, "", "null"},
		{"usage finding only", yes, yes, []snapshot.Finding{nodeFinding("NODE_LOW_UTILIZATION", analysis.SeverityAttention, "n1", number("maximum_average_percent", 10))}, "", "null"},
		{"stored without the finding", no, yes, nil, "", "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := testNode("n1", "apps", valueStat(10, 20), valueStat(10, 20))
			n.Ready, n.Schedulable = tt.ready, tt.schedulable
			if got := nodeStateStatus(n, tt.findings); got != tt.want {
				t.Errorf("nodeStateStatus() = %q, want %q", got, tt.want)
			}
			view := mustView(t, nodeViewSnapshot(t, n, tt.findings), catalog.English)
			if len(view.Nodes) != 1 {
				t.Fatalf("view holds %d nodes, want 1", len(view.Nodes))
			}
			if got := statusText(view.Nodes[0].StateStatus); got != tt.wantView {
				t.Errorf("state_status = %s, want %s", got, tt.wantView)
			}
		})
	}
}

func TestExecutiveCountsANodeNotReadyWithoutAnAction(t *testing.T) {
	n := testNode("n1", "apps", valueStat(10, 20), valueStat(10, 20))
	n.Ready = boolPtr(false)
	s := nodeViewSnapshot(t, n, []snapshot.Finding{nodeFinding("NODE_NOT_READY", analysis.SeverityAttention, "n1")})
	s.Report.Profile = "executive"
	s.Recommendations = deriveRecommendations(s.Findings)

	view := mustView(t, s, catalog.English)
	found := false
	for _, p := range view.Verdict.Pressure {
		if p.ScopeType == "node" {
			found = true
			if p.Severity != "attention" || p.Attention != 1 || p.Critical != 0 {
				t.Errorf("node pressure = %s with %d critical and %d attention, want attention with 0 and 1", p.Severity, p.Critical, p.Attention)
			}
		}
	}
	if !found {
		t.Error("the node scope has no pressure row")
	}
	pdf := englishVerdict(t, s, view.MinSeverity)
	if len(pdf.Pressure.Rows) != len(view.Verdict.Pressure) {
		t.Errorf("PDF has %d pressure rows, view %d", len(pdf.Pressure.Rows), len(view.Verdict.Pressure))
	}
	pdfNodes := 0
	for _, row := range pdf.Pressure.Rows {
		if row.Scope != "Nodes" {
			continue
		}
		pdfNodes++
		if want := []VerdictCount{{"attention", "Attention", "1"}}; row.Status != "attention" || !reflect.DeepEqual(row.Counts, want) {
			t.Errorf("PDF node pressure = %s %+v, want attention %+v", row.Status, row.Counts, want)
		}
	}
	if pdfNodes != 1 {
		t.Errorf("PDF has %d node pressure rows, want 1", pdfNodes)
	}
	if (pdf.Plan.Table != nil && len(pdf.Plan.Table.Rows) != 0) || pdf.Plan.Empty == nil {
		t.Errorf("PDF plan = %+v, want no action for a node's readiness", pdf.Plan)
	}
	for _, r := range view.Recommendations {
		if strings.HasPrefix(r.Code, "NODE_") {
			t.Errorf("view recommendation %s, want none for a node's readiness", r.Code)
		}
	}
}

// nodeViewSnapshot returns a snapshot of one node with the counts of its
// summary agreeing with what it holds.
func nodeViewSnapshot(t *testing.T, n snapshot.Node, findings []snapshot.Finding) snapshot.Snapshot {
	t.Helper()
	s := poolSnapshot(t, []snapshot.Node{n}, findings...)
	s.Summary.Nodes = 1
	s.Summary.Nodepools = 1
	s.Summary.Findings = float64(len(findings))
	for _, f := range findings {
		if f.Severity == analysis.SeverityAttention {
			s.Summary.Attention++
		}
	}
	s.Recommendations = deriveRecommendations(findings)
	s.Summary.Recommendations = float64(len(s.Recommendations))
	return s
}
