package collect

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// Querier is what a collector needs of the client.
type Querier interface {
	Query(ctx context.Context, query string, at time.Time) ([]prom.Sample, error)
	QueryRange(ctx context.Context, query string, start, end time.Time, step string) ([]prom.Series, error)
}

// Input is what every collector reads.
type Input struct {
	Scope        discovery.Scope
	Cluster      config.Cluster
	Analysis     config.Analysis
	Capabilities []discovery.Capability
	Window       Window
}

// Resources holds the ten statistics of a row.
type Resources struct {
	CPUUsage             Statistics
	MemoryUsage          Statistics
	CPURequests          Statistics
	MemoryRequests       Statistics
	CPULimits            Statistics
	MemoryLimits         Statistics
	CPURequestPercent    Statistics
	MemoryRequestPercent Statistics
	CPULimitPercent      Statistics
	MemoryLimitPercent   Statistics
}

// Message is a warning or a notice: a code and its named numbers.
type Message struct {
	Code   string
	Params map[string]float64 // never nil
}

// Result is what a collector returns.
type Result[T any] struct {
	State    string    // a state of discovery
	Strategy string    // a strategy of discovery
	Rows     []T       // sorted, never nil
	Warnings []Message // never nil
	Notices  []Message // never nil
}

// The warning and notice codes of both collectors of this package.
const (
	WarningNamespaceUsageIncomplete           = "NAMESPACE_USAGE_INCOMPLETE"
	WarningNamespaceRequestsLimitsIncomplete  = "NAMESPACE_REQUESTS_LIMITS_INCOMPLETE"
	WarningNamespaceRequestsLimitsUnavailable = "NAMESPACE_REQUESTS_LIMITS_UNAVAILABLE"
	WarningWorkloadUsageIncomplete            = "WORKLOAD_USAGE_INCOMPLETE"
	WarningWorkloadRequestsLimitsIncomplete   = "WORKLOAD_REQUESTS_LIMITS_INCOMPLETE"
	WarningWorkloadRequestsLimitsUnavailable  = "WORKLOAD_REQUESTS_LIMITS_UNAVAILABLE"
	WarningWorkloadHistoryCurrentAssociation  = "WORKLOAD_HISTORY_CURRENT_ASSOCIATION"
	NoticeWorkloadsOmittedNoUsage             = "WORKLOADS_OMITTED_NO_USAGE"
	NoticeJobsOmitted                         = "JOBS_OMITTED"
	WarningHPAAmbiguousSpec                   = "HPA_AMBIGUOUS_SPEC"
	WarningHPAAmbiguousTarget                 = "HPA_AMBIGUOUS_TARGET"
	NoticeWorkloadsOmittedScaleZero           = "WORKLOADS_OMITTED_SCALE_ZERO"
	WarningNodeIdentityUnavailable            = "NODE_IDENTITY_UNAVAILABLE"
	WarningNodeCapacityEvidenceIncomplete     = "NODE_CAPACITY_EVIDENCE_INCOMPLETE"
)

// WarningCodes lists every warning code, in the order above.
var WarningCodes = []string{
	WarningNamespaceUsageIncomplete,
	WarningNamespaceRequestsLimitsIncomplete,
	WarningNamespaceRequestsLimitsUnavailable,
	WarningWorkloadUsageIncomplete,
	WarningWorkloadRequestsLimitsIncomplete,
	WarningWorkloadRequestsLimitsUnavailable,
	WarningWorkloadHistoryCurrentAssociation,
	WarningHPAAmbiguousSpec,
	WarningHPAAmbiguousTarget,
	WarningNodeIdentityUnavailable,
	WarningNodeCapacityEvidenceIncomplete,
}

// NoticeCodes lists every notice code, in the order above.
var NoticeCodes = []string{
	NoticeWorkloadsOmittedNoUsage,
	NoticeJobsOmitted,
	NoticeWorkloadsOmittedScaleZero,
}

// NoticeParams names the parameters of each notice: count, for all three.
var NoticeParams = map[string][]string{
	NoticeWorkloadsOmittedNoUsage:   {"count"},
	NoticeJobsOmitted:               {"count"},
	NoticeWorkloadsOmittedScaleZero: {"count"},
}

// findCapability returns the capability named name from capabilities. A
// caller that omits it from Input.Capabilities gets an error naming it,
// since that is a fault of the caller, not a state to report.
func findCapability(capabilities []discovery.Capability, name string) (discovery.Capability, error) {
	for _, capability := range capabilities {
		if capability.Name == name {
			return capability, nil
		}
	}
	return discovery.Capability{}, fmt.Errorf("collect: input capabilities do not name %q", name)
}

// Collected is the result of every collector of a run.
type Collected struct {
	Namespaces  Result[Namespace]
	Workloads   Result[Workload]
	HPAs        Result[HPA]
	Coverage    Result[WorkloadRef]
	Nodes       Result[Node]
	Autoscaling AutoscalingEvidence
}

// ErrNoUsefulSections is the second rule of insufficient capabilities: no
// collector returned a row and the autoscaling status is unknown.
var ErrNoUsefulSections = errors.New("no useful report sections were collected")

// All runs the collectors in the order of the original: namespaces,
// workloads, HPAs with their coverage, nodes, then the autoscaling. It stops
// at the first error and returns it.
func All(ctx context.Context, q Querier, in Input) (Collected, error) {
	namespaces, err := Namespaces(ctx, q, in)
	if err != nil {
		return Collected{}, err
	}
	workloads, err := Workloads(ctx, q, in)
	if err != nil {
		return Collected{}, err
	}
	hpas, coverage, err := HPAs(ctx, q, in)
	if err != nil {
		return Collected{}, err
	}
	nodes, err := Nodes(ctx, q, in)
	if err != nil {
		return Collected{}, err
	}
	autoscaling, err := Autoscaling(ctx, q, in)
	if err != nil {
		return Collected{}, err
	}
	return Collected{
		Namespaces:  namespaces,
		Workloads:   workloads,
		HPAs:        hpas,
		Coverage:    coverage,
		Nodes:       nodes,
		Autoscaling: autoscaling,
	}, nil
}

// CheckUseful reports ErrNoUsefulSections when no result of the five holds a
// row with a state other than unavailable and the autoscaling status is
// unknown.
func CheckUseful(c Collected) error {
	if collected(c.Namespaces) || collected(c.Workloads) || collected(c.HPAs) ||
		collected(c.Coverage) || collected(c.Nodes) || c.Autoscaling.Status != AutoscalingUnknown {
		return nil
	}
	return ErrNoUsefulSections
}

// collected reports whether result holds a row in a state other than
// unavailable.
func collected[T any](result Result[T]) bool {
	return result.State != discovery.StateUnavailable && len(result.Rows) > 0
}

// Warnings joins the warnings of the five results, in the order namespace,
// workload, hpa, hpa coverage, node, each message held once.
func (c Collected) Warnings() []Message {
	return dedupMessages(c.Namespaces.Warnings, c.Workloads.Warnings, c.HPAs.Warnings, c.Coverage.Warnings, c.Nodes.Warnings)
}

// Notices joins the notices of the five results, in the order namespace,
// workload, hpa, hpa coverage, node, each message held once.
func (c Collected) Notices() []Message {
	return dedupMessages(c.Namespaces.Notices, c.Workloads.Notices, c.HPAs.Notices, c.Coverage.Notices, c.Nodes.Notices)
}

// dedupMessages concatenates groups in the order given, keeping the first
// occurrence of a message and dropping a later one equal to it by code and
// parameters.
func dedupMessages(groups ...[]Message) []Message {
	joined := []Message{}
	for _, group := range groups {
		for _, message := range group {
			if !holdsMessage(joined, message) {
				joined = append(joined, message)
			}
		}
	}
	return joined
}

// holdsMessage reports whether messages holds one equal to message by code
// and parameters.
func holdsMessage(messages []Message, message Message) bool {
	for _, existing := range messages {
		if existing.Code == message.Code && paramsEqual(existing.Params, message.Params) {
			return true
		}
	}
	return false
}

// paramsEqual reports whether a and b hold the same keys, each with the
// same value.
func paramsEqual(a, b map[string]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, ok := b[key]; !ok || other != value {
			return false
		}
	}
	return true
}
