//go:build postgres

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/capacity-room/internal/platform/audit"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

var now = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func TestAppendWritesRow(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	actor := store.User{ID: uuid.New(), Username: "amy", PasswordHash: "x", Role: "administrator", Language: "en", Active: true, CreatedAt: now, UpdatedAt: now}
	if err := store.InsertUser(ctx, pool, actor); err != nil {
		t.Fatal(err)
	}
	err := audit.Append(ctx, pool, now, audit.Event{
		ActorID:    &actor.ID,
		Action:     audit.ActionUserCreate,
		TargetType: "user",
		TargetID:   "42",
		Outcome:    audit.OutcomeFailed,
		Metadata:   audit.Metadata{"role": "reader"},
	})
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.ListAuditEvents(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d rows", len(events))
	}
	e := events[0]
	if e.ActorID == nil || *e.ActorID != actor.ID || !e.OccurredAt.Equal(now) || e.Action != "user.create" ||
		e.TargetType == nil || *e.TargetType != "user" || e.TargetID == nil || *e.TargetID != "42" ||
		e.Outcome != "failed" || string(e.Metadata) != `{"role": "reader"}` {
		t.Fatalf("got %+v", e)
	}
}

func TestAppendDefaultsOutcome(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	if err := audit.Append(ctx, pool, now, audit.Event{Action: audit.ActionBootstrap}); err != nil {
		t.Fatal(err)
	}
	events, err := store.ListAuditEvents(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Outcome != "succeeded" || events[0].TargetType != nil || events[0].TargetID != nil ||
		events[0].ActorID != nil || string(events[0].Metadata) != `{}` {
		t.Fatalf("got %+v", events)
	}
}

func TestAppendRefusedMetadataWritesNothing(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	err := audit.Append(ctx, pool, now, audit.Event{Action: audit.ActionLoginFailed, Metadata: audit.Metadata{"password": "x"}})
	if err == nil {
		t.Fatal("want an error")
	}
	events, err := store.ListAuditEvents(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("got %d rows", len(events))
	}
}
