package prom

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
)

// TestScenarioProbe checks that Connect succeeds against every recorded
// scenario of the original Python tool, served by promtest, binding to the
// scenario's own endpoint kind and using exactly one scripted request (the
// smoke test).
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
					URL:            ts.URL + scenario.APIRoot,
					EndpointType:   scenario.EndpointType,
					TLSVerify:      true,
					TimeoutSeconds: 5,
				},
				Token: scenario.Token,
				Sleep: sleep,
			}

			client, err := Connect(context.Background(), opts)
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}
			if client.Kind() != scenario.EndpointType {
				t.Errorf("Kind() = %q, want %q", client.Kind(), scenario.EndpointType)
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

// TestScenarioGrafanaNeedsToken checks that the grafana-authenticated
// scenario, which the fixture scripts to require a bearer token, fails to
// connect without one, and that no request was recorded as a match.
func TestScenarioGrafanaNeedsToken(t *testing.T) {
	fixture, err := promtest.LoadScenario("grafana-authenticated")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	server := promtest.NewServer(fixture)
	ts := httptest.NewServer(server)
	defer ts.Close()

	var scenario promtest.Scenario
	for _, sc := range promtest.Scenarios {
		if sc.Name == "grafana-authenticated" {
			scenario = sc
		}
	}

	sleep, _ := recordingSleep()
	opts := Options{
		Metrics: config.Metrics{
			URL:            ts.URL + scenario.APIRoot,
			EndpointType:   scenario.EndpointType,
			TLSVerify:      true,
			TimeoutSeconds: 5,
		},
		// Token deliberately left empty.
		Sleep: sleep,
	}

	_, err = Connect(context.Background(), opts)
	if err == nil {
		t.Fatal("Connect: want an error, got nil")
	}

	for _, s := range server.Seen() {
		if s.Matched {
			t.Errorf("a request was recorded as a success: %+v", s)
		}
	}
}
