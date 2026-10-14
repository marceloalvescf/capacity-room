package collect

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/discovery"
)

// TestRegistriesOfCodes transcribes the shape of the registries: the first seven
// warning codes, the first two notice codes, and a "count" parameter for each
// notice.
func TestRegistriesOfCodes(t *testing.T) {
	wantWarnings := []string{
		WarningNamespaceUsageIncomplete,
		WarningNamespaceRequestsLimitsIncomplete,
		WarningNamespaceRequestsLimitsUnavailable,
		WarningWorkloadUsageIncomplete,
		WarningWorkloadRequestsLimitsIncomplete,
		WarningWorkloadRequestsLimitsUnavailable,
		WarningWorkloadHistoryCurrentAssociation,
	}
	if len(WarningCodes) < len(wantWarnings) {
		t.Fatalf("len(WarningCodes) = %d, want at least %d", len(WarningCodes), len(wantWarnings))
	}
	for i, code := range wantWarnings {
		if WarningCodes[i] != code {
			t.Errorf("WarningCodes[%d] = %q, want %q", i, WarningCodes[i], code)
		}
	}

	wantNotices := []string{
		NoticeWorkloadsOmittedNoUsage,
		NoticeJobsOmitted,
	}
	if len(NoticeCodes) < len(wantNotices) {
		t.Fatalf("len(NoticeCodes) = %d, want at least %d", len(NoticeCodes), len(wantNotices))
	}
	for i, code := range wantNotices {
		if NoticeCodes[i] != code {
			t.Errorf("NoticeCodes[%d] = %q, want %q", i, NoticeCodes[i], code)
		}
	}

	for _, code := range wantNotices {
		params, ok := NoticeParams[code]
		if !ok {
			t.Fatalf("NoticeParams has no entry for %q", code)
		}
		if len(params) != 1 || params[0] != "count" {
			t.Errorf("NoticeParams[%q] = %v, want [\"count\"]", code, params)
		}
	}
}

// TestNoticeAndWarningCodesGrew holds the eleven warnings and three notices,
// and the count parameter of the newest notice.
func TestNoticeAndWarningCodesGrew(t *testing.T) {
	if len(WarningCodes) != 11 {
		t.Fatalf("len(WarningCodes) = %d, want 11", len(WarningCodes))
	}
	if WarningCodes[7] != "HPA_AMBIGUOUS_SPEC" || WarningCodes[8] != "HPA_AMBIGUOUS_TARGET" {
		t.Errorf("WarningCodes[7:9] = %v, want HPA_AMBIGUOUS_SPEC, HPA_AMBIGUOUS_TARGET", WarningCodes[7:9])
	}
	if WarningCodes[9] != "NODE_IDENTITY_UNAVAILABLE" || WarningCodes[10] != "NODE_CAPACITY_EVIDENCE_INCOMPLETE" {
		t.Errorf("WarningCodes[9:] = %v, want NODE_IDENTITY_UNAVAILABLE, NODE_CAPACITY_EVIDENCE_INCOMPLETE", WarningCodes[9:])
	}
	if len(NoticeCodes) != 3 {
		t.Fatalf("len(NoticeCodes) = %d, want 3", len(NoticeCodes))
	}
	if NoticeCodes[2] != "WORKLOADS_OMITTED_SCALE_ZERO" {
		t.Errorf("NoticeCodes[2] = %q, want WORKLOADS_OMITTED_SCALE_ZERO", NoticeCodes[2])
	}
	params := NoticeParams["WORKLOADS_OMITTED_SCALE_ZERO"]
	if len(params) != 1 || params[0] != "count" {
		t.Errorf("NoticeParams = %v, want [count]", params)
	}
}

// allInput holds every capability the five collectors read, all available.
func allInput() Input {
	in := nodeInput()
	in.Capabilities = append(in.Capabilities,
		workloadCapability(discovery.StateAvailable, discovery.StrategyRawMetrics),
		hpaCapability("hpa_history", discovery.StateAvailable, nil, nil),
		hpaCapability("workloads_without_hpa", discovery.StateAvailable, labelsEvidence, nil),
	)
	in.Analysis.NodeAutoscaling = AutoscalingUnknown
	return in
}

// queriesOf returns the queries that run sends to a fresh querier.
func queriesOf(t *testing.T, run func(Querier) error) []string {
	t.Helper()
	q := &scriptedQuerier{}
	if err := run(q); err != nil {
		t.Fatalf("collector: %v", err)
	}
	return q.queries
}

func TestAllRunsTheFiveInOrder(t *testing.T) {
	in := allInput()
	ctx := context.Background()

	namespaces := queriesOf(t, func(q Querier) error { _, err := Namespaces(ctx, q, in); return err })
	workloads := queriesOf(t, func(q Querier) error { _, err := Workloads(ctx, q, in); return err })
	hpas := queriesOf(t, func(q Querier) error { _, _, err := HPAs(ctx, q, in); return err })
	nodes := queriesOf(t, func(q Querier) error { _, err := Nodes(ctx, q, in); return err })
	autoscaling := queriesOf(t, func(q Querier) error { _, err := Autoscaling(ctx, q, in); return err })

	if len(hpas) != 7 {
		t.Fatalf("the HPA collector sent %d queries, want 7", len(hpas))
	}
	if len(autoscaling) != 6 {
		t.Fatalf("the autoscaling collector sent %d queries, want 6", len(autoscaling))
	}
	if len(namespaces) == 0 || len(workloads) == 0 || len(nodes) == 0 {
		t.Fatalf("a collector sent nothing: namespaces %d, workloads %d, nodes %d", len(namespaces), len(workloads), len(nodes))
	}

	var want []string
	for _, group := range [][]string{namespaces, workloads, hpas, nodes, autoscaling} {
		want = append(want, group...)
	}

	q := &scriptedQuerier{}
	if _, err := All(ctx, q, in); err != nil {
		t.Fatalf("All: %v", err)
	}
	if !reflect.DeepEqual(q.queries, want) {
		t.Errorf("All sent\n%q\nwant\n%q", q.queries, want)
	}
}

func TestAllStopsAtTheFirstError(t *testing.T) {
	in := allInput()
	ctx := context.Background()

	before := 0
	for _, run := range []func(Querier) error{
		func(q Querier) error { _, err := Namespaces(ctx, q, in); return err },
		func(q Querier) error { _, err := Workloads(ctx, q, in); return err },
		func(q Querier) error { _, _, err := HPAs(ctx, q, in); return err },
	} {
		before += len(queriesOf(t, run))
	}

	boom := errors.New("boom")
	q := &scriptedQuerier{failAfter: before + 1, failErr: boom}
	_, err := All(ctx, q, in)
	if !errors.Is(err, boom) {
		t.Fatalf("All error = %v, want %v", err, boom)
	}
	if len(q.queries) != before+1 {
		t.Errorf("All sent %d queries, want %d", len(q.queries), before+1)
	}
	for _, query := range q.queries {
		if strings.Contains(query, "cluster_autoscaler") {
			t.Errorf("All sent an autoscaling query after the node collector failed: %q", query)
		}
	}
}

func TestWarningsKeepTheResultOrder(t *testing.T) {
	message := func(code string) []Message { return []Message{{Code: code, Params: map[string]float64{}}} }
	c := Collected{
		Namespaces: Result[Namespace]{Warnings: message("W_NAMESPACE"), Notices: message("N_NAMESPACE")},
		Workloads:  Result[Workload]{Warnings: append(message("W_WORKLOAD"), message("W_NAMESPACE")...), Notices: message("N_WORKLOAD")},
		HPAs:       Result[HPA]{Warnings: message("W_HPA"), Notices: message("N_HPA")},
		Coverage:   Result[WorkloadRef]{Warnings: message("W_COVERAGE"), Notices: message("N_COVERAGE")},
		Nodes:      Result[Node]{Warnings: message("W_NODE"), Notices: message("N_NODE")},
	}
	codes := func(messages []Message) []string {
		var out []string
		for _, m := range messages {
			out = append(out, m.Code)
		}
		return out
	}
	wantWarnings := []string{"W_NAMESPACE", "W_WORKLOAD", "W_HPA", "W_COVERAGE", "W_NODE"}
	if got := codes(c.Warnings()); !reflect.DeepEqual(got, wantWarnings) {
		t.Errorf("Warnings() = %v, want %v", got, wantWarnings)
	}
	wantNotices := []string{"N_NAMESPACE", "N_WORKLOAD", "N_HPA", "N_COVERAGE", "N_NODE"}
	if got := codes(c.Notices()); !reflect.DeepEqual(got, wantNotices) {
		t.Errorf("Notices() = %v, want %v", got, wantNotices)
	}
}

func emptyCollected() Collected {
	return Collected{
		Namespaces:  Result[Namespace]{State: discovery.StateUnavailable},
		Workloads:   Result[Workload]{State: discovery.StateUnavailable},
		HPAs:        Result[HPA]{State: discovery.StateUnavailable},
		Coverage:    Result[WorkloadRef]{State: discovery.StateUnavailable},
		Nodes:       Result[Node]{State: discovery.StateUnavailable},
		Autoscaling: AutoscalingEvidence{Status: AutoscalingUnknown},
	}
}

func TestCheckUsefulPasses(t *testing.T) {
	c := emptyCollected()
	c.Namespaces = Result[Namespace]{State: discovery.StateAvailable, Rows: []Namespace{{Name: "payments"}}}
	if err := CheckUseful(c); err != nil {
		t.Errorf("CheckUseful = %v, want nil", err)
	}
}

func TestCheckUsefulFails(t *testing.T) {
	if err := CheckUseful(emptyCollected()); !errors.Is(err, ErrNoUsefulSections) {
		t.Errorf("CheckUseful = %v, want ErrNoUsefulSections", err)
	}
	if got := ErrNoUsefulSections.Error(); got != "no useful report sections were collected" {
		t.Errorf("ErrNoUsefulSections = %q", got)
	}
}

func TestCheckUsefulSavedByAutoscaling(t *testing.T) {
	c := emptyCollected()
	c.Autoscaling.Status = AutoscalingDisabled
	if err := CheckUseful(c); err != nil {
		t.Errorf("CheckUseful = %v, want nil", err)
	}
}

func TestCheckUsefulIgnoresUnavailableRows(t *testing.T) {
	c := emptyCollected()
	c.Namespaces = Result[Namespace]{State: discovery.StateUnavailable, Rows: []Namespace{{Name: "payments"}}}
	c.Nodes = Result[Node]{State: discovery.StateUnavailable, Rows: []Node{{Name: "node-1"}}}
	if err := CheckUseful(c); !errors.Is(err, ErrNoUsefulSections) {
		t.Errorf("CheckUseful = %v, want ErrNoUsefulSections", err)
	}
}
