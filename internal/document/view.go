package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// View is the report as the web shows it: the same snapshot as the
// document of the PDF, worded by the same helpers, so that the view says
// what the PDF says. A closed set travels as its code, for the frontend to
// word; a text the program composes travels as text in the catalog's
// language; a number travels with its raw value and its text.
type View struct {
	Language        string               `json:"language"`
	MinSeverity     string               `json:"min_severity"`
	Report          ViewReport           `json:"report"`
	Summary         ViewSummary          `json:"summary"`
	Findings        []ViewFinding        `json:"findings"`
	Recommendations []ViewRecommendation `json:"recommendations"`
	Verdict         ViewVerdict          `json:"verdict"`
	Namespaces      []ViewNamespace      `json:"namespaces"`
	Workloads       []ViewWorkload       `json:"workloads"`
	HPAs            []ViewHPA            `json:"hpas"`
	Nodes           []ViewNode           `json:"nodes"`
	Nodepools       []ViewNodepool       `json:"nodepools"`
	Quality         ViewQuality          `json:"quality"`
}

// Value is a number with its text: Raw is nil when the number is absent,
// and Text is then the placeholder the PDF prints for it. Status is the
// mark the PDF prints beside the text, and nil when it prints none; a
// node's request share carries the severity of the node's request finding.
type Value struct {
	Raw    *float64 `json:"raw"`
	Text   string   `json:"text"`
	Status *string  `json:"status"`
}

// Statistic is a series reduced to four numbers, with the state of its
// reduction.
type Statistic struct {
	State   string `json:"state"`
	Average Value  `json:"average"`
	Minimum Value  `json:"minimum"`
	Peak    Value  `json:"peak"`
	Current Value  `json:"current"`
}

// ViewReport identifies the run.
type ViewReport struct {
	ID               string   `json:"id"`
	ClusterName      string   `json:"cluster_name"`
	Profile          string   `json:"profile"`
	Timezone         string   `json:"timezone"`
	GeneratorVersion string   `json:"generator_version"`
	Namespaces       []string `json:"namespaces"`
	EmittedAt        string   `json:"emitted_at"`
	EmittedAtText    string   `json:"emitted_at_text"`
	PeriodStart      string   `json:"period_start"`
	PeriodEnd        string   `json:"period_end"`
	PeriodText       string   `json:"period_text"`
}

// ViewSummary holds the counts of the run and its autoscaling.
type ViewSummary struct {
	Namespaces          Value                  `json:"namespaces"`
	Workloads           Value                  `json:"workloads"`
	HPAs                Value                  `json:"hpas"`
	Nodes               Value                  `json:"nodes"`
	Nodepools           Value                  `json:"nodepools"`
	Findings            Value                  `json:"findings"`
	Recommendations     Value                  `json:"recommendations"`
	Healthy             Value                  `json:"healthy"`
	Attention           Value                  `json:"attention"`
	Critical            Value                  `json:"critical"`
	WorkloadsWithoutHPA Value                  `json:"workloads_without_hpa"`
	Autoscaling         ViewSummaryAutoscaling `json:"autoscaling"`
}

// ViewSummaryAutoscaling is what the run learned about node autoscaling.
type ViewSummaryAutoscaling struct {
	Status            string `json:"status"`
	Source            string `json:"source"`
	Confidence        string `json:"confidence"`
	ManagedNodeGroups Value  `json:"managed_node_groups"`
	CurrentNodes      Value  `json:"current_nodes"`
	MinNodes          Value  `json:"min_nodes"`
	MaxNodes          Value  `json:"max_nodes"`
	ScaleEvents       Value  `json:"scale_events"`
	UnschedulablePods Value  `json:"unschedulable_pods"`
}

// ViewFinding is one finding with its sentence, its action, and its
// evidence in text.
type ViewFinding struct {
	Code       string         `json:"code"`
	Severity   string         `json:"severity"`
	ScopeType  string         `json:"scope_type"`
	ScopeKind  string         `json:"scope_kind"`
	ScopeName  string         `json:"scope_name"`
	Confidence string         `json:"confidence"`
	Sentence   string         `json:"sentence"`
	Action     string         `json:"action"`
	Evidence   []ViewEvidence `json:"evidence"`
}

// ViewEvidence is one named value of a finding: the number, boolean, or
// text as stored, and its text.
type ViewEvidence struct {
	Name string `json:"name"`
	Raw  any    `json:"raw"`
	Text string `json:"text"`
}

// ViewRecommendation is one action of the plan, with the scopes it names.
type ViewRecommendation struct {
	Code       string   `json:"code"`
	Severity   string   `json:"severity"`
	ScopeType  string   `json:"scope_type"`
	Confidence string   `json:"confidence"`
	Action     string   `json:"action"`
	Scopes     []string `json:"scopes"`
	ScopeText  string   `json:"scope_text"`
	Finding    *int     `json:"finding"`
}

// ViewNamespace is the row of one namespace.
type ViewNamespace struct {
	Name                 string    `json:"name"`
	Status               string    `json:"status"`
	Pods                 Value     `json:"pods"`
	Approximate          bool      `json:"approximate"`
	CPUUsage             Statistic `json:"cpu_usage"`
	MemoryUsage          Statistic `json:"memory_usage"`
	CPURequests          Statistic `json:"cpu_requests"`
	MemoryRequests       Statistic `json:"memory_requests"`
	CPULimits            Statistic `json:"cpu_limits"`
	MemoryLimits         Statistic `json:"memory_limits"`
	CPURequestPercent    Statistic `json:"cpu_request_percent"`
	MemoryRequestPercent Statistic `json:"memory_request_percent"`
	CPULimitPercent      Statistic `json:"cpu_limit_percent"`
	MemoryLimitPercent   Statistic `json:"memory_limit_percent"`
}

// ViewWorkload is the row of one workload.
type ViewWorkload struct {
	Namespace            string    `json:"namespace"`
	Kind                 string    `json:"kind"`
	Name                 string    `json:"name"`
	Status               string    `json:"status"`
	ObservedUsage        bool      `json:"observed_usage"`
	HPACoverage          string    `json:"hpa_coverage"`
	Pods                 Value     `json:"pods"`
	Approximate          bool      `json:"approximate"`
	CPUUsage             Statistic `json:"cpu_usage"`
	MemoryUsage          Statistic `json:"memory_usage"`
	CPURequests          Statistic `json:"cpu_requests"`
	MemoryRequests       Statistic `json:"memory_requests"`
	CPULimits            Statistic `json:"cpu_limits"`
	MemoryLimits         Statistic `json:"memory_limits"`
	CPURequestPercent    Statistic `json:"cpu_request_percent"`
	MemoryRequestPercent Statistic `json:"memory_request_percent"`
	CPULimitPercent      Statistic `json:"cpu_limit_percent"`
	MemoryLimitPercent   Statistic `json:"memory_limit_percent"`
	Reason               string    `json:"reason"`
}

// ViewHPA is the row of one horizontal pod autoscaler.
type ViewHPA struct {
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	TargetKind string    `json:"target_kind"`
	TargetName string    `json:"target_name"`
	SpecMin    Value     `json:"spec_min"`
	SpecMax    Value     `json:"spec_max"`
	Current    Statistic `json:"current"`
	Desired    Statistic `json:"desired"`
	Situation  string    `json:"situation"`
	// SituationStatus is the mark of the situation cell.
	SituationStatus *string `json:"situation_status"`
}

// ViewNode is the row of one node.
type ViewNode struct {
	Name                 string    `json:"name"`
	Pool                 string    `json:"pool"`
	PoolConfidence       string    `json:"pool_confidence"`
	Status               string    `json:"status"`
	CPUCapacity          Value     `json:"cpu_capacity"`
	MemoryCapacity       Value     `json:"memory_capacity"`
	CPUAllocatable       Value     `json:"cpu_allocatable"`
	MemoryAllocatable    Value     `json:"memory_allocatable"`
	CPURequestPercent    Value     `json:"cpu_request_percent"`
	MemoryRequestPercent Value     `json:"memory_request_percent"`
	CPUUsagePercent      Statistic `json:"cpu_usage_percent"`
	MemoryUsagePercent   Statistic `json:"memory_usage_percent"`
	Ready                *bool     `json:"ready"`
	Schedulable          *bool     `json:"schedulable"`
	Reason               string    `json:"reason"`
	// ReasonStatus is the mark of the reason cell, nil when the node has no
	// finding and the cell has none.
	ReasonStatus *string `json:"reason_status"`
}

// ViewNodepool is one node pool and the names of its nodes.
type ViewNodepool struct {
	Name       string   `json:"name"`
	Confidence string   `json:"confidence"`
	Nodes      []string `json:"nodes"`
}

// ViewQuality is what the run could tell about its own coverage.
type ViewQuality struct {
	Capabilities          []ViewCapability `json:"capabilities"`
	Warnings              []ViewMessage    `json:"warnings"`
	Notices               []ViewMessage    `json:"notices"`
	CollectionStartedAt   string           `json:"collection_started_at"`
	CollectionCompletedAt string           `json:"collection_completed_at"`
}

// ViewCapability is one of the nine capabilities, in its effective state,
// with the remark the appendix prints.
type ViewCapability struct {
	Name     string   `json:"name"`
	State    string   `json:"state"`
	Strategy string   `json:"strategy"`
	Evidence []string `json:"evidence"`
	Remark   string   `json:"remark"`
}

// ViewMessage is a warning or a notice with its text.
type ViewMessage struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

// BuildView words a snapshot in the catalog's language for the web,
// narrowed as the PDF of the same report is: findings, recommendations,
// and the rows of namespaces, workloads, and nodes below the minimum
// severity are left out; the minimum is raised to attention for the
// executive profile. HPAs, node pools, the summary, and the quality are
// whole.
func BuildView(s snapshot.Snapshot, c *catalog.Catalog) (View, error) {
	profile, severity, err := resolve(s, Options{})
	if err != nil {
		return View{}, err
	}
	if profile == "executive" {
		severity = raiseToAttention(severity)
	}
	return buildView(s, c, severity)
}

// buildView words a snapshot that passed Check, narrowed by severity.
func buildView(s snapshot.Snapshot, c *catalog.Catalog, severity string) (View, error) {
	b := &builder{cat: c}
	vb := &viewBuilder{b: b, thresholds: thresholdsFromSnapshot(s.Report.Thresholds)}
	minRank := minimumRank(severity)
	findings := shownFindings(s, severity)

	view := View{
		Language:        string(c.Language()),
		MinSeverity:     severity,
		Report:          vb.report(s),
		Summary:         vb.summary(s.Summary),
		Findings:        vb.findings(findings),
		Recommendations: vb.recommendations(planRecommendations(s, executiveMinimum(severity)), findings),
		Verdict:         vb.verdict(decideVerdict(s, findings)),
		Namespaces:      vb.namespaces(s, minRank),
		Workloads:       vb.workloads(s, minRank),
		HPAs:            vb.hpas(s.HPAs),
		Nodes:           vb.nodes(s, minRank),
		Nodepools:       vb.nodepools(s.Nodepools),
		Quality:         vb.quality(s),
	}
	if b.err != nil {
		return View{}, b.err
	}
	return view, nil
}

// EncodeView writes v with two spaces of indentation and one line break at
// the end, without escaping "<", ">", or "&" for HTML. The same view
// always gives the same bytes.
func EncodeView(v View) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// viewBuilder words the parts of a view with the helpers of the document.
type viewBuilder struct {
	b          *builder
	thresholds config.Thresholds
}

// textOf writes how a statistic's number is worded: the mark the PDF prints
// beside it, empty when it prints none, and its text.
type textOf func(stat snapshot.Statistics, value *float64) (status, text string)

// quantity words a number by format, as a table cell of a quantity does;
// the PDF marks none.
func (vb *viewBuilder) quantity(format func(float64) string) textOf {
	return func(stat snapshot.Statistics, value *float64) (string, string) {
		return "", quantityText(vb.b, stat, value, format)
	}
}

// ratio words a ratio as the tables of namespaces and workloads do.
func (vb *viewBuilder) ratio() textOf {
	return func(stat snapshot.Statistics, value *float64) (string, string) {
		return ratioText(vb.b, stat, value, vb.thresholds)
	}
}

// usage words a usage percentage as the table of nodes does.
func (vb *viewBuilder) usage() textOf {
	return func(stat snapshot.Statistics, value *float64) (string, string) {
		return usageText(vb.b, stat, value, vb.thresholds)
	}
}

// mark is a status as the view carries it: nil for none.
func mark(status string) *string {
	if status == "" {
		return nil
	}
	return &status
}

// wordedValue words one number of stat by words.
func wordedValue(stat snapshot.Statistics, number *float64, words textOf) Value {
	status, text := words(stat, number)
	return Value{Raw: number, Text: text, Status: mark(status)}
}

// statistic words the four numbers of stat, each by words.
func (vb *viewBuilder) statistic(stat snapshot.Statistics, words textOf) Statistic {
	return Statistic{
		State:   stat.State,
		Average: wordedValue(stat, stat.Average, words),
		Minimum: wordedValue(stat, stat.Minimum, words),
		Peak:    wordedValue(stat, stat.Peak, words),
		Current: wordedValue(stat, stat.Current, words),
	}
}

// replicas words the statistic of a replica count: the average with one
// decimal, the others as counts, as the table of HPAs does.
func (vb *viewBuilder) replicas(stat snapshot.Statistics) Statistic {
	average := vb.quantity(func(v float64) string { return vb.b.cat.Decimal(v, 1) })
	count := vb.quantity(vb.b.cat.Count)
	return Statistic{
		State:   stat.State,
		Average: wordedValue(stat, stat.Average, average),
		Minimum: wordedValue(stat, stat.Minimum, count),
		Peak:    wordedValue(stat, stat.Peak, count),
		Current: wordedValue(stat, stat.Current, count),
	}
}

// number words a number that has no statistic, by format, or as
// unavailable when it is absent.
func (vb *viewBuilder) number(value *float64, format func(float64) string) Value {
	if value == nil {
		return Value{Text: vb.b.text(catalog.PlaceholderUnavailable, nil)}
	}
	return Value{Raw: value, Text: format(*value)}
}

// count words a whole number the run holds.
func (vb *viewBuilder) count(value float64) Value {
	return vb.number(&value, vb.b.cat.Count)
}

// pods words a pod count as the tables do.
func (vb *viewBuilder) pods(pods *float64) Value {
	return Value{Raw: pods, Text: podsText(vb.b, pods)}
}

func (vb *viewBuilder) report(s snapshot.Snapshot) ViewReport {
	return ViewReport{
		ID:               s.Report.ID,
		ClusterName:      s.Scope.ClusterName,
		Profile:          s.Report.Profile,
		Timezone:         s.Report.Timezone,
		GeneratorVersion: s.Report.GeneratorVersion,
		Namespaces:       nonNilStrings(s.Scope.Namespaces),
		EmittedAt:        s.Report.EmittedAt,
		EmittedAtText:    vb.b.cat.DateTime(vb.b.time(s.Report.EmittedAt)),
		PeriodStart:      s.Report.PeriodStart,
		PeriodEnd:        s.Report.PeriodEnd,
		PeriodText:       periodRange(vb.b, s),
	}
}

func (vb *viewBuilder) summary(m snapshot.Summary) ViewSummary {
	a := m.Autoscaling
	return ViewSummary{
		Namespaces:          vb.count(m.Namespaces),
		Workloads:           vb.count(m.Workloads),
		HPAs:                vb.count(m.HPAs),
		Nodes:               vb.count(m.Nodes),
		Nodepools:           vb.count(m.Nodepools),
		Findings:            vb.count(m.Findings),
		Recommendations:     vb.count(m.Recommendations),
		Healthy:             vb.count(m.Healthy),
		Attention:           vb.count(m.Attention),
		Critical:            vb.count(m.Critical),
		WorkloadsWithoutHPA: vb.count(m.WorkloadsWithoutHPA),
		Autoscaling: ViewSummaryAutoscaling{
			Status:            a.Status,
			Source:            a.Source,
			Confidence:        a.Confidence,
			ManagedNodeGroups: vb.number(a.ManagedNodeGroups, vb.b.cat.Count),
			CurrentNodes:      vb.number(a.CurrentNodes, vb.b.cat.Count),
			MinNodes:          vb.number(a.MinNodes, vb.b.cat.Count),
			MaxNodes:          vb.number(a.MaxNodes, vb.b.cat.Count),
			ScaleEvents:       vb.number(a.ScaleEvents, vb.b.cat.Count),
			UnschedulablePods: vb.number(a.UnschedulablePods, vb.b.cat.Count),
		},
	}
}

// findingDefinition looks up the registry entry of a finding code, and keeps
// an error when the code is not in it.
func findingDefinition(b *builder, code string) (analysis.Definition, bool) {
	definition, ok := analysis.Lookup(code)
	if !ok && b.err == nil {
		b.err = fmt.Errorf("finding %s is not in the registry", code)
	}
	return definition, ok
}

// findingSentence writes the sentence of a finding of definition.
func findingSentence(b *builder, definition analysis.Definition) string {
	return b.text(catalog.FindingSummary(definition.Family, definition.Params["resource"]), nil)
}

func (vb *viewBuilder) findings(findings []snapshot.Finding) []ViewFinding {
	out := make([]ViewFinding, 0, len(findings))
	for _, f := range findings {
		definition, ok := findingDefinition(vb.b, f.Code)
		if !ok {
			return out
		}
		resource := definition.Params["resource"]
		out = append(out, ViewFinding{
			Code:       f.Code,
			Severity:   f.Severity,
			ScopeType:  f.ScopeType,
			ScopeKind:  f.ScopeKind,
			ScopeName:  f.ScopeName,
			Confidence: f.Confidence,
			Sentence:   findingSentence(vb.b, definition),
			Action:     vb.b.text(catalog.FindingAction(definition.Family, resource), nil),
			Evidence:   vb.evidence(f.Evidence),
		})
	}
	return out
}

// evidence words the values of a finding: a name ending in percent as a
// percentage, another whole number as a count, another number with two
// decimals, a boolean as yes or no, and a text as stored.
func (vb *viewBuilder) evidence(evidence []snapshot.Evidence) []ViewEvidence {
	out := make([]ViewEvidence, 0, len(evidence))
	for _, e := range evidence {
		out = append(out, evidenceView(vb.b, e))
	}
	return out
}

// evidenceView words one value of a finding.
func evidenceView(b *builder, e snapshot.Evidence) ViewEvidence {
	item := ViewEvidence{Name: e.Name}
	switch e.Value.Kind {
	case "number":
		item.Raw = e.Value.Number
		switch {
		case strings.HasSuffix(e.Name, "percent"):
			item.Text = b.cat.Percent(e.Value.Number)
		case e.Value.Number == math.Trunc(e.Value.Number):
			item.Text = b.cat.Count(e.Value.Number)
		default:
			item.Text = b.cat.Decimal(e.Value.Number, 2)
		}
	case "boolean":
		item.Raw = e.Value.Boolean
		if e.Value.Boolean {
			item.Text = b.text(catalog.ValueYes, nil)
		} else {
			item.Text = b.text(catalog.ValueNo, nil)
		}
	default:
		item.Raw = e.Value.Text
		item.Text = e.Value.Text
	}
	return item
}

func (vb *viewBuilder) recommendations(recommendations []snapshot.Recommendation, findings []snapshot.Finding) []ViewRecommendation {
	out := make([]ViewRecommendation, 0, len(recommendations))
	for _, r := range recommendations {
		definition, ok := analysis.Lookup(r.Code)
		if !ok {
			if vb.b.err == nil {
				vb.b.err = fmt.Errorf("recommendation %s is not in the registry", r.Code)
			}
			return out
		}
		out = append(out, ViewRecommendation{
			Code:       r.Code,
			Severity:   r.Severity,
			ScopeType:  r.ScopeType,
			Confidence: r.Confidence,
			Action:     vb.b.text(catalog.FindingAction(definition.Family, definition.Params["resource"]), nil),
			Scopes:     nonNilStrings(r.Scopes),
			ScopeText:  *recommendationScope(vb.b, r).text,
			Finding:    recommendationFinding(r, findings),
		})
	}
	return out
}

func (vb *viewBuilder) namespaces(s snapshot.Snapshot, minRank int) []ViewNamespace {
	cpu, memory := vb.quantity(vb.b.cat.CPU), vb.quantity(vb.b.cat.Memory)
	ratio := vb.ratio()
	out := make([]ViewNamespace, 0, len(s.Namespaces))
	for _, n := range s.Namespaces {
		status := scopeSeverity(s.Findings, analysis.ScopeNamespace, n.Name, "")
		if analysis.Rank(status) < minRank {
			continue
		}
		out = append(out, ViewNamespace{
			Name:                 n.Name,
			Status:               status,
			Pods:                 vb.pods(n.Pods),
			Approximate:          n.Approximate,
			CPUUsage:             vb.statistic(n.CPUUsage, cpu),
			MemoryUsage:          vb.statistic(n.MemoryUsage, memory),
			CPURequests:          vb.statistic(n.CPURequests, cpu),
			MemoryRequests:       vb.statistic(n.MemoryRequests, memory),
			CPULimits:            vb.statistic(n.CPULimits, cpu),
			MemoryLimits:         vb.statistic(n.MemoryLimits, memory),
			CPURequestPercent:    vb.statistic(n.CPURequestPercent, ratio),
			MemoryRequestPercent: vb.statistic(n.MemoryRequestPercent, ratio),
			CPULimitPercent:      vb.statistic(n.CPULimitPercent, ratio),
			MemoryLimitPercent:   vb.statistic(n.MemoryLimitPercent, ratio),
		})
	}
	return out
}

// workloadKey names a workload by namespace, kind, and name.
type workloadKey struct{ namespace, kind, name string }

func (vb *viewBuilder) workloads(s snapshot.Snapshot, minRank int) []ViewWorkload {
	// The reason of a workload is the one the executive priority tables
	// print, so it follows their minimum, raised to attention.
	summaries, err := workloadSummaries(s, max(minRank, minimumRank(analysis.SeverityAttention)))
	if err != nil && vb.b.err == nil {
		vb.b.err = err
	}
	reasons := make(map[workloadKey]string, len(summaries))
	for _, w := range summaries {
		reasons[workloadKey{w.Namespace, w.Kind, w.Name}] = workloadReason(vb.b, w.Findings)
	}

	cpu, memory := vb.quantity(vb.b.cat.CPU), vb.quantity(vb.b.cat.Memory)
	ratio := vb.ratio()
	out := make([]ViewWorkload, 0, len(s.Workloads))
	for _, w := range s.Workloads {
		status := scopeSeverity(s.Findings, analysis.ScopeWorkload, w.Namespace+"/"+w.Name, w.Kind)
		if analysis.Rank(status) < minRank {
			continue
		}
		out = append(out, ViewWorkload{
			Namespace:            w.Namespace,
			Kind:                 w.Kind,
			Name:                 w.Name,
			Status:               status,
			ObservedUsage:        w.ObservedUsage,
			HPACoverage:          w.HPACoverage,
			Pods:                 vb.pods(w.Pods),
			Approximate:          w.Approximate,
			CPUUsage:             vb.statistic(w.CPUUsage, cpu),
			MemoryUsage:          vb.statistic(w.MemoryUsage, memory),
			CPURequests:          vb.statistic(w.CPURequests, cpu),
			MemoryRequests:       vb.statistic(w.MemoryRequests, memory),
			CPULimits:            vb.statistic(w.CPULimits, cpu),
			MemoryLimits:         vb.statistic(w.MemoryLimits, memory),
			CPURequestPercent:    vb.statistic(w.CPURequestPercent, ratio),
			MemoryRequestPercent: vb.statistic(w.MemoryRequestPercent, ratio),
			CPULimitPercent:      vb.statistic(w.CPULimitPercent, ratio),
			MemoryLimitPercent:   vb.statistic(w.MemoryLimitPercent, ratio),
			Reason:               reasons[workloadKey{w.Namespace, w.Kind, w.Name}],
		})
	}
	return out
}

func (vb *viewBuilder) hpas(hpas []snapshot.HPA) []ViewHPA {
	out := make([]ViewHPA, 0, len(hpas))
	for _, h := range hpas {
		situationStatus, situation := hpaSituationText(vb.b, h)
		out = append(out, ViewHPA{
			Namespace:       h.Namespace,
			Name:            h.Name,
			TargetKind:      h.TargetKind,
			TargetName:      h.TargetName,
			SpecMin:         vb.pods(h.SpecMin),
			SpecMax:         vb.pods(h.SpecMax),
			Current:         vb.replicas(h.Current),
			Desired:         vb.replicas(h.Desired),
			Situation:       situation,
			SituationStatus: mark(situationStatus),
		})
	}
	return out
}

func (vb *viewBuilder) nodes(s snapshot.Snapshot, minRank int) []ViewNode {
	usage := vb.usage()
	out := make([]ViewNode, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		status := scopeSeverity(s.Findings, analysis.ScopeNode, n.Name, "")
		if analysis.Rank(status) < minRank {
			continue
		}
		reasonStatus, reason := nodeReasonText(vb.b, n.Name, s.Findings)
		out = append(out, ViewNode{
			Name:                 n.Name,
			Pool:                 n.Pool,
			PoolConfidence:       n.PoolConfidence,
			Status:               status,
			CPUCapacity:          vb.number(n.CPUCapacity, vb.b.cat.CPU),
			MemoryCapacity:       vb.number(n.MemoryCapacity, vb.b.cat.Memory),
			CPUAllocatable:       vb.number(n.CPUAllocatable, vb.b.cat.CPU),
			MemoryAllocatable:    vb.number(n.MemoryAllocatable, vb.b.cat.Memory),
			CPURequestPercent:    requestShare(vb.number(n.CPURequestPercent, vb.b.cat.Percent), s.Findings, n.Name, "NODE_CPU_REQUEST_HIGH"),
			MemoryRequestPercent: requestShare(vb.number(n.MemoryRequestPercent, vb.b.cat.Percent), s.Findings, n.Name, "NODE_MEMORY_REQUEST_HIGH"),
			CPUUsagePercent:      vb.statistic(n.CPUUsagePercent, usage),
			MemoryUsagePercent:   vb.statistic(n.MemoryUsagePercent, usage),
			Ready:                n.Ready,
			Schedulable:          n.Schedulable,
			Reason:               reason,
			ReasonStatus:         mark(reasonStatus),
		})
	}
	return out
}

// requestShare marks a node's request share with the severity of the
// node's finding of code, and leaves it unmarked when the node has none.
func requestShare(value Value, findings []snapshot.Finding, node, code string) Value {
	for _, f := range findings {
		if f.Code == code && f.ScopeType == analysis.ScopeNode && f.ScopeName == node {
			value.Status = mark(f.Severity)
			break
		}
	}
	return value
}

func (vb *viewBuilder) nodepools(pools []snapshot.NodePool) []ViewNodepool {
	out := make([]ViewNodepool, 0, len(pools))
	for _, p := range pools {
		out = append(out, ViewNodepool{Name: p.Name, Confidence: p.Confidence, Nodes: nonNilStrings(p.Nodes)})
	}
	return out
}

func (vb *viewBuilder) quality(s snapshot.Snapshot) ViewQuality {
	capabilities := effectiveCapabilities(s)
	quality := ViewQuality{
		Capabilities:          make([]ViewCapability, 0, len(capabilities)),
		Warnings:              make([]ViewMessage, 0, len(s.Quality.Warnings)),
		Notices:               make([]ViewMessage, 0, len(s.Quality.Notices)),
		CollectionStartedAt:   s.Quality.CollectionStartedAt,
		CollectionCompletedAt: s.Quality.CollectionCompletedAt,
	}
	for _, c := range capabilities {
		quality.Capabilities = append(quality.Capabilities, ViewCapability{
			Name:     c.Name,
			State:    c.State,
			Strategy: c.Strategy,
			Evidence: nonNilStrings(c.Evidence),
			Remark:   appendixRemark(vb.b, c.Reasons),
		})
	}
	for _, w := range s.Quality.Warnings {
		quality.Warnings = append(quality.Warnings, ViewMessage{Code: w.Code, Text: warningText(vb.b, w.Code)})
	}
	for _, n := range s.Quality.Notices {
		quality.Notices = append(quality.Notices, ViewMessage{Code: n.Code, Text: noticeText(vb.b, n)})
	}
	return quality
}

// nonNilStrings returns values, or an empty, non-nil slice in its place, so
// that a list is never written as null.
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return slices.Clone(values)
}

// ViewVerdict holds the decisions of the verdict of the overview, as
// indices into the lists of the view.
type ViewVerdict struct {
	ClusterFinding      *int           `json:"cluster_finding"`
	NodeEvidenceWarning *int           `json:"node_evidence_warning"`
	Gauge               *ViewGauge     `json:"gauge"`
	Pressure            []ViewPressure `json:"pressure"`
}

// ViewGauge is the depth gauge: whole nodes and the hazard of its last band.
type ViewGauge struct {
	Current int     `json:"current"`
	Max     int     `json:"max"`
	Min     *int    `json:"min"`
	Hazard  *string `json:"hazard"`
}

// ViewPressure is one scope type under pressure.
type ViewPressure struct {
	ScopeType string `json:"scope_type"`
	Severity  string `json:"severity"`
	Finding   int    `json:"finding"`
	Critical  int    `json:"critical"`
	Attention int    `json:"attention"`
}

// verdict carries the decisions of the verdict into the view.
func (vb *viewBuilder) verdict(facts verdictFacts) ViewVerdict {
	v := ViewVerdict{
		ClusterFinding:      indexOrNil(facts.ClusterFinding),
		NodeEvidenceWarning: indexOrNil(facts.EvidenceWarning),
		Pressure:            make([]ViewPressure, 0, len(facts.Pressure)),
	}
	if g := facts.Gauge; g != nil {
		v.Gauge = &ViewGauge{Current: g.Current, Max: g.Max, Min: g.Min}
		if g.Hazard != "" {
			v.Gauge.Hazard = &g.Hazard
		}
	}
	for _, p := range facts.Pressure {
		v.Pressure = append(v.Pressure, ViewPressure{
			ScopeType: p.ScopeType, Severity: p.Severity, Finding: p.Lead,
			Critical: p.Critical, Attention: p.Attention,
		})
	}
	return v
}

func indexOrNil(i int) *int {
	if i < 0 {
		return nil
	}
	return &i
}
