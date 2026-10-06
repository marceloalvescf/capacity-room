package worker

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

// Deps are what every handler of the worker reads.
type Deps struct {
	Pool      *pgxpool.Pool
	Profiles  *profiles.Service
	Settings  settings.Settings
	Toolchain typst.Toolchain
	TempDir   string
	Version   string
	Now       func() time.Time
	Logger    *slog.Logger // writes through the run's redactor
}

// ErrStaleAttempt is store.ErrStaleAttempt, returned when a newer attempt owns the report.
var ErrStaleAttempt = store.ErrStaleAttempt
