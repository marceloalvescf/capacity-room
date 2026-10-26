//go:build postgres

package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

func TestHomeNeedsASession(t *testing.T) {
	ts := newTestServer(t)
	wantProblem(t, ts.as(t, nil, "GET", "/api/v1/home", "", nil), 401, "UNAUTHENTICATED", nil)
}

func TestHomeAnswersEveryActiveProfile(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	ctx := context.Background()

	d := profiles.Defaults()
	d.DisplayName = "Retired"
	d.Active = false
	d.EndpointURL = "https://metrics.test/api/v1"
	d.ClusterName = "retired"
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Transact(ctx, ts.pool, func(tx pgx.Tx) error {
		if _, err := store.GetOrCreateProfile(ctx, tx, "retired", ts.now); err != nil {
			return err
		}
		rev, err := store.AppendProfileRevision(ctx, tx, store.ProfileRevision{
			ID: uuid.New(), ProfileID: "retired", Fingerprint: "retired", DisplayName: d.DisplayName,
			Active: false, Document: raw, CreatedAt: ts.now,
		})
		if err != nil {
			return err
		}
		return store.SetCurrentRevision(ctx, tx, "retired", &rev.ID, ts.now)
	})
	if err != nil {
		t.Fatal(err)
	}
	id := ts.seedReport(t, stored{owner: c.ops.user.ID})

	w := ts.as(t, &c.eve, "GET", "/api/v1/home", "", nil)
	if w.Code != 200 {
		t.Fatalf("home: %d %s", w.Code, w.Body.String())
	}
	home := decodeStrict[Home](t, w)
	if len(home.Clusters) != 1 {
		t.Fatalf("clusters %+v", home.Clusters)
	}
	cl := home.Clusters[0]
	if cl.ProfileId != "prod" || cl.DisplayName != "Production" {
		t.Fatalf("cluster %+v", cl)
	}
	if cl.Latest == nil || cl.Latest.Id != id || cl.Newest == nil || cl.Newest.Id != id {
		t.Fatalf("latest %+v newest %+v, want %s", cl.Latest, cl.Newest, id)
	}
}
