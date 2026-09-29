package collect

import (
	"context"
	"fmt"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
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
}

// NoticeCodes lists every notice code, in the order above.
var NoticeCodes = []string{
	NoticeWorkloadsOmittedNoUsage,
	NoticeJobsOmitted,
}

// NoticeParams names the parameters of each notice: count, for both.
var NoticeParams = map[string][]string{
	NoticeWorkloadsOmittedNoUsage: {"count"},
	NoticeJobsOmitted:             {"count"},
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
