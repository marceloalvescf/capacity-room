package prom

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
)

// TestScenarioProbe checks that Connect succeeds against every recorded
// scenario of the original Python tool, served by promtest, using exactly one
// scripted request (the smoke test).
func TestScenarioProbe(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			fixture, err := promtest.LoadScenario(scenario.Name)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			server := promtest.NewServer(fixture)
			ts := httptest.NewServer(server)
			defer ts.Close()

			sleep, _ := recordingSleep()
			opts := Options{
				Metrics: config.Metrics{
					URL:            ts.URL + "/api/v1",
					TLSVerify:      true,
					TimeoutSeconds: 5,
				},
				Sleep: sleep,
			}

			if _, err := Connect(context.Background(), opts); err != nil {
				t.Fatalf("Connect: %v", err)
			}

			seen := server.Seen()
			if len(seen) != 1 {
				t.Fatalf("len(Seen()) = %d, want 1", len(seen))
			}
			if !seen[0].Matched {
				t.Error("the one request the server saw did not match a scripted request")
			}
		})
	}
}
