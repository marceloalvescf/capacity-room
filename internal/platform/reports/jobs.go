package reports

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
)

// The queue and the priorities of the report jobs. A render runs before the
// next collection, so a collected report's PDFs are not held back.
const (
	queueReports    = "reports"
	priorityRender  = 1
	priorityCollect = 2
)

// CollectArgs and RenderArgs are the River jobs of a report; their Kind is
// the routing key and never changes.
type CollectArgs struct {
	ReportID uuid.UUID `json:"report_id"`
}

// Kind returns the routing key of the collect job.
func (CollectArgs) Kind() string { return "collect" }

// RenderArgs is the River job that renders one language of a report.
type RenderArgs struct {
	ReportID uuid.UUID `json:"report_id"`
	Language string    `json:"language"`
}

// Kind returns the routing key of the render job.
func (RenderArgs) Kind() string { return "render" }

// CollectInsertOpts are the insert options of a collect job.
func CollectInsertOpts(s settings.Settings) *river.InsertOpts {
	return &river.InsertOpts{Queue: queueReports, Priority: priorityCollect, MaxAttempts: s.JobMaxAttempts}
}

// RenderInsertOpts are the insert options of a render job.
func RenderInsertOpts(s settings.Settings) *river.InsertOpts {
	return &river.InsertOpts{Queue: queueReports, Priority: priorityRender, MaxAttempts: s.JobMaxAttempts}
}
