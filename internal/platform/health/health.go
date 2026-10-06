package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

// readyTimeout bounds the whole of a readiness answer.
const readyTimeout = 2 * time.Second

// Check is one named readiness check.
type Check struct {
	Name string
	Run  func(ctx context.Context) bool
}

type readyBody struct {
	Status string          `json:"status"`
	Checks map[string]bool `json:"checks"`
}

// readOnly reports whether r is a GET or a HEAD, the two methods the routes take.
func readOnly(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

// Handler answers GET /health/live with 200 {"status":"live"} and GET
// /health/ready with 200 or 503 and the result of every check, run together
// within two seconds; a check that has not answered by then counts as
// failed. Any other request is 404 {"status":"not_found"}.
func Handler(checks []Check) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case readOnly(r) && r.URL.Path == "/health/live":
			writeJSON(w, r, http.StatusOK, map[string]string{"status": "live"})
		case readOnly(r) && r.URL.Path == "/health/ready":
			ready(w, r, checks)
		default:
			writeJSON(w, r, http.StatusNotFound, map[string]string{"status": "not_found"})
		}
	})
}

func ready(w http.ResponseWriter, r *http.Request, checks []Check) {
	ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
	defer cancel()

	type result struct {
		name string
		ok   bool
	}
	results := make(chan result, len(checks))
	for _, c := range checks {
		go func() { results <- result{c.Name, c.Run(ctx)} }()
	}
	body := readyBody{Status: "ready", Checks: make(map[string]bool, len(checks))}
	for _, c := range checks {
		body.Checks[c.Name] = false
	}
	for range checks {
		select {
		case res := <-results:
			body.Checks[res.name] = res.ok
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	status := http.StatusOK
	for _, ok := range body.Checks {
		if !ok {
			body.Status, status = "not_ready", http.StatusServiceUnavailable
			break
		}
	}
	writeJSON(w, r, status, body)
}

// Database passes when the database answers SELECT 1.
func Database(pool *pgxpool.Pool) Check {
	return Check{Name: "database", Run: func(ctx context.Context) bool {
		if pool == nil {
			return false
		}
		var one int
		return pool.QueryRow(ctx, "SELECT 1").Scan(&one) == nil
	}}
}

// Migrations passes when no migration is pending.
func Migrations(pool *pgxpool.Pool) Check {
	return Check{Name: "migrations", Run: func(ctx context.Context) bool {
		if pool == nil {
			return false
		}
		pending, err := store.HasPending(ctx, pool)
		return err == nil && !pending
	}}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(append(body, '\n'))
}
