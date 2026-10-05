package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

const readyTimeout = 2 * time.Second

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
}

type readyChecks struct {
	Database   bool `json:"database"`
	Migrations bool `json:"migrations"`
}

type readyBody struct {
	Status string      `json:"status"`
	Checks readyChecks `json:"checks"`
}

// ready answers 200 when the database answers and no migration is pending.
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	var checks readyChecks
	if s.Pool != nil {
		ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
		defer cancel()
		var one int
		if err := s.Pool.QueryRow(ctx, "SELECT 1").Scan(&one); err == nil {
			checks.Database = true
			pending, err := store.HasPending(ctx, s.Pool)
			checks.Migrations = err == nil && !pending
		}
	}
	if checks.Database && checks.Migrations {
		writeJSON(w, http.StatusOK, readyBody{"ready", checks})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, readyBody{"unavailable", checks})
}
