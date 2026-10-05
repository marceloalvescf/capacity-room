//go:build postgres

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

func TestInsertAuditEventAndList(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	u := newUser("amy", "administrator", true)
	mustInsertUser(t, pool, u)

	targetType, targetID := "user", "42"
	// jsonb normalises text, so the metadata is written in the form jsonb
	// prints: keys sorted, one space after each colon and comma.
	first := store.AuditRow{ActorID: &u.ID, OccurredAt: now, Action: "user.create", TargetType: &targetType, TargetID: &targetID, Outcome: "succeeded", Metadata: []byte(`{"a": 1, "b": "x"}`)}
	second := store.AuditRow{OccurredAt: now.Add(time.Minute), Action: "login", Outcome: "failed", Metadata: []byte(`{}`)}
	for _, r := range []store.AuditRow{first, second} {
		if err := store.InsertAuditEvent(ctx, pool, r); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.ListAuditEvents(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].ID >= events[1].ID {
		t.Fatalf("got %+v", events)
	}
	e1, e2 := events[0], events[1]
	if e1.ActorID == nil || *e1.ActorID != u.ID || !e1.OccurredAt.Equal(time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)) ||
		e1.Action != "user.create" || e1.TargetType == nil || *e1.TargetType != "user" ||
		e1.TargetID == nil || *e1.TargetID != "42" || e1.Outcome != "succeeded" ||
		string(e1.Metadata) != `{"a": 1, "b": "x"}` || string(e2.Metadata) != `{}` {
		t.Fatalf("first %+v", e1)
	}
	if e2.ActorID != nil || e2.TargetType != nil || e2.TargetID != nil || e2.Action != "login" ||
		e2.Outcome != "failed" || !e2.OccurredAt.Equal(time.Date(2026, 10, 5, 12, 1, 0, 0, time.UTC)) {
		t.Fatalf("second %+v", e2)
	}
}

func TestAuditActorMustExist(t *testing.T) {
	pool := storetest.New(t)
	actor := uuid.New()
	err := store.InsertAuditEvent(context.Background(), pool, store.AuditRow{ActorID: &actor, OccurredAt: now, Action: "x", Outcome: "succeeded", Metadata: []byte(`{}`)})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("got %v, want SQLSTATE 23503", err)
	}
}
