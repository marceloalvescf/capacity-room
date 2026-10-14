//go:build postgres

package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

func newRevision(profileID, fingerprint string) store.ProfileRevision {
	return store.ProfileRevision{
		ID:             uuid.New(),
		ProfileID:      profileID,
		Fingerprint:    fingerprint,
		DisplayName:    "Name " + fingerprint,
		Active:         true,
		Document:       []byte(`{"b":1,"a":[1,2]}`),
		CredentialFile: "cred-" + fingerprint,
		CABundleFile:   "ca-" + fingerprint,
		CreatedAt:      now,
	}
}

// appendRevision creates the identity when absent and appends a revision in
// one transaction.
func appendRevision(t *testing.T, pool *pgxpool.Pool, profileID, fingerprint string) store.ProfileRevision {
	t.Helper()
	ctx := context.Background()
	var got store.ProfileRevision
	err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		if _, err := store.GetOrCreateProfile(ctx, tx, profileID, now); err != nil {
			return err
		}
		var err error
		got, err = store.AppendProfileRevision(ctx, tx, newRevision(profileID, fingerprint))
		return err
	})
	if err != nil {
		t.Fatalf("append revision: %v", err)
	}
	return got
}

func TestGetOrCreateProfileIsIdempotentAndLocks(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()

	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Rollback(ctx) }()
	created, err := store.GetOrCreateProfile(ctx, first, "amy", now)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "amy" || created.CurrentRevisionID != nil || !created.CreatedAt.Equal(now) || !created.UpdatedAt.Equal(now) {
		t.Fatalf("got %+v", created)
	}
	again, err := store.GetOrCreateProfile(ctx, first, "amy", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !again.CreatedAt.Equal(now) {
		t.Fatalf("second call rewrote the row: %+v", again)
	}

	expectBlocksUntilCommit(t, first, func() error {
		return store.Transact(ctx, pool, func(tx pgx.Tx) error {
			_, err := store.GetOrCreateProfile(ctx, tx, "amy", now)
			return err
		})
	})

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("%d rows, %v, want 1", n, err)
	}
}

func TestGetProfileForUpdate(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		if _, err := store.GetProfileForUpdate(ctx, tx, "amy"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("absent: got %v, want ErrNotFound", err)
		}
		if _, err := store.GetOrCreateProfile(ctx, tx, "amy", now); err != nil {
			return err
		}
		got, err := store.GetProfileForUpdate(ctx, tx, "amy")
		if err != nil || got.ID != "amy" {
			t.Fatalf("got %+v, %v", got, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func sameJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("document %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)
	if !bytes.Equal(gb, wb) {
		t.Fatalf("document %s, want %s", got, want)
	}
}

func TestAppendRevisionNumbersInsideTheInsert(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()

	one := appendRevision(t, pool, "amy", "fp1")
	two := appendRevision(t, pool, "amy", "fp2")
	if one.Revision != 1 || two.Revision != 2 {
		t.Fatalf("revisions %d, %d, want 1, 2", one.Revision, two.Revision)
	}
	if two.ProfileID != "amy" || two.Fingerprint != "fp2" || two.DisplayName != "Name fp2" || !two.Active ||
		two.CredentialFile != "cred-fp2" || two.CABundleFile != "ca-fp2" || two.CreatedBy != nil ||
		!two.CreatedAt.Equal(now) || two.CreatedAt.Location() != time.UTC {
		t.Fatalf("got %+v", two)
	}
	sameJSON(t, two.Document, `{"a":[1,2],"b":1}`)

	if err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		return store.SetCurrentRevision(ctx, tx, "amy", &two.ID, now)
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		return store.SetCurrentRevision(ctx, tx, "amy", nil, now)
	}); err != nil {
		t.Fatal(err)
	}
	three := appendRevision(t, pool, "amy", "fp3")
	if three.Revision != 3 {
		t.Fatalf("revision %d, want 3", three.Revision)
	}
	other := appendRevision(t, pool, "kim", "fp1")
	if other.Revision != 1 {
		t.Fatalf("another profile: revision %d, want 1", other.Revision)
	}
}

func TestAppendRevisionRefusesARepeatedFingerprint(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	appendRevision(t, pool, "amy", "fp1")
	err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		_, err := store.AppendProfileRevision(ctx, tx, newRevision("amy", "fp1"))
		return err
	})
	if !errors.Is(err, store.ErrFingerprintExists) {
		t.Fatalf("got %v, want ErrFingerprintExists", err)
	}
}

func appendOnce(ctx context.Context, pool *pgxpool.Pool, lockIdentity bool, fingerprint string) (int, error) {
	var number int
	err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		if lockIdentity {
			if _, err := store.GetOrCreateProfile(ctx, tx, "amy", now); err != nil {
				return err
			}
		}
		got, err := store.AppendProfileRevision(ctx, tx, newRevision("amy", fingerprint))
		number = got.Revision
		return err
	})
	return number, err
}

func TestAppendRevisionRace(t *testing.T) {
	ctx := context.Background()

	t.Run("serialized on the identity lock", func(t *testing.T) {
		pool := storetest.New(t)
		type result struct {
			number int
			err    error
		}
		results := make(chan result, 2)
		for _, fingerprint := range []string{"fp1", "fp2"} {
			go func() {
				n, err := appendOnce(ctx, pool, true, fingerprint)
				results <- result{n, err}
			}()
		}
		seen := map[int]bool{}
		for range 2 {
			r := <-results
			if r.err != nil {
				t.Fatal(r.err)
			}
			seen[r.number] = true
		}
		if !seen[1] || !seen[2] {
			t.Fatalf("numbers %v, want 1 and 2", seen)
		}
	})

	t.Run("without the identity lock", func(t *testing.T) {
		pool := storetest.New(t)
		if _, err := pool.Exec(ctx, insertProfile, "amy", nil); err != nil {
			t.Fatal(err)
		}
		errs := make(chan error, 2)
		for _, fingerprint := range []string{"fp1", "fp2"} {
			go func() {
				_, err := appendOnce(ctx, pool, false, fingerprint)
				errs <- err
			}()
		}
		failures := 0
		for range 2 {
			if err := <-errs; err != nil {
				failures++
				if code, _ := sqlState(t, err); code != "23505" {
					t.Fatalf("SQLSTATE = %s, want 23505", code)
				}
			}
		}
		rows, err := pool.Query(ctx, `SELECT revision FROM profile_revisions WHERE profile_id = 'amy' ORDER BY revision`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var numbers []int
		for rows.Next() {
			var n int
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
			numbers = append(numbers, n)
		}
		want := []int{1, 2}
		if failures == 1 {
			want = []int{1}
		}
		if !slices.Equal(numbers, want) {
			t.Fatalf("%d failures, rows %v, want %v", failures, numbers, want)
		}
		t.Logf("failures = %d, numbers = %v", failures, numbers)
	})
}

func TestSetCurrentAndGetCurrent(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	rev := appendRevision(t, pool, "amy", "fp1")

	if _, err := store.GetCurrentRevision(ctx, pool, "amy"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("null pointer: got %v, want ErrNotFound", err)
	}
	if _, err := store.GetCurrentRevision(ctx, pool, "absent"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("absent profile: got %v, want ErrNotFound", err)
	}

	later := now.Add(time.Hour)
	if err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		return store.SetCurrentRevision(ctx, tx, "amy", &rev.ID, later)
	}); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetCurrentRevision(ctx, pool, "amy")
	if err != nil || got.ID != rev.ID || got.Revision != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	profile, err := store.GetProfileRevision(ctx, pool, rev.ID)
	if err != nil || profile.Fingerprint != "fp1" {
		t.Fatalf("got %+v, %v", profile, err)
	}
	if _, err := store.GetProfileRevision(ctx, pool, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("absent revision: got %v, want ErrNotFound", err)
	}
	var updated time.Time
	if err := pool.QueryRow(ctx, `SELECT updated_at FROM profiles WHERE id = 'amy'`).Scan(&updated); err != nil || !updated.Equal(later) {
		t.Fatalf("updated_at = %v, %v, want %v", updated, err, later)
	}

	if err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		return store.SetCurrentRevision(ctx, tx, "amy", nil, later)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetCurrentRevision(ctx, pool, "amy"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cleared: got %v, want ErrNotFound", err)
	}
	if err := store.SetCurrentRevision(ctx, pool, "absent", nil, later); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("absent profile: got %v, want ErrNotFound", err)
	}
}

func TestListCurrentOrdersByID(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	zed := appendRevision(t, pool, "zed", "fp1")
	amy := appendRevision(t, pool, "amy", "fp1")
	appendRevision(t, pool, "kim", "fp1")
	if err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		if err := store.SetCurrentRevision(ctx, tx, "zed", &zed.ID, now); err != nil {
			return err
		}
		return store.SetCurrentRevision(ctx, tx, "amy", &amy.ID, now)
	}); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListCurrentRevisions(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ProfileID != "amy" || got[1].ProfileID != "zed" {
		t.Fatalf("got %+v", got)
	}
}

func TestFindByFingerprint(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	rev := appendRevision(t, pool, "amy", "fp1")
	got, err := store.FindProfileRevisionByFingerprint(ctx, pool, "amy", "fp1")
	if err != nil || got.ID != rev.ID {
		t.Fatalf("got %+v, %v", got, err)
	}
	for _, c := range [][2]string{{"amy", "other"}, {"kim", "fp1"}} {
		if _, err := store.FindProfileRevisionByFingerprint(ctx, pool, c[0], c[1]); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("%v: got %v, want ErrNotFound", c, err)
		}
	}
}

func TestLatestProfileRevision(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	if _, err := store.LatestProfileRevision(ctx, pool, "amy"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("none: got %v, want ErrNotFound", err)
	}
	appendRevision(t, pool, "amy", "fp1")
	appendRevision(t, pool, "amy", "fp2")
	three := appendRevision(t, pool, "amy", "fp3")
	got, err := store.LatestProfileRevision(ctx, pool, "amy")
	if err != nil || got.ID != three.ID || got.Revision != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestGetProfileForShareBlocksAnUpdate(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	appendRevision(t, pool, "amy", "fp1")

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	got, err := store.GetProfileForShare(ctx, tx, "amy")
	if err != nil || got.ID != "amy" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := store.GetProfileForShare(ctx, tx, "absent"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("absent: %v", err)
	}
	expectBlocksUntilCommit(t, tx, func() error {
		_, err := pool.Exec(ctx, `UPDATE profiles SET updated_at = $2 WHERE id = $1`, "amy", now.Add(time.Hour))
		return err
	})
}
