package discovery

import (
	"context"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// Preflight is what every run shares before it collects: the client bound
// to the root that answered, the scope, and the capabilities.
type Preflight struct {
	Client       *prom.Client
	Scope        Scope
	Capabilities []Capability
}

// RunPreflight connects with opts, resolves the scope of cluster at at,
// detects the capabilities for analysis at at, and checks they are
// sufficient. It reads the clock nowhere.
func RunPreflight(ctx context.Context, opts prom.Options, cluster config.Cluster, analysis config.Analysis, at time.Time) (Preflight, error) {
	client, err := prom.Connect(ctx, opts)
	if err != nil {
		return Preflight{}, err
	}
	scope, err := ResolveScope(ctx, client, cluster, at)
	if err != nil {
		return Preflight{}, err
	}
	capabilities, err := Detect(ctx, client, scope, analysis, at)
	if err != nil {
		return Preflight{}, err
	}
	if err := CheckSufficient(capabilities); err != nil {
		return Preflight{}, err
	}

	return Preflight{Client: client, Scope: scope, Capabilities: capabilities}, nil
}
