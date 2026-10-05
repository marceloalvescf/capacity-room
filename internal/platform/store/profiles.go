package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Profile is a row of the profiles table: the identity of a profile and the
// pointer to its current revision.
type Profile struct {
	ID                string
	CurrentRevisionID *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ProfileRevision is a row of the profile_revisions table. A revision never
// changes once written.
type ProfileRevision struct {
	ID             uuid.UUID
	ProfileID      string
	Revision       int
	Fingerprint    string
	DisplayName    string
	Active         bool
	Document       []byte // JSON, as PostgreSQL normalises it in a jsonb column
	CredentialFile string
	CABundleFile   string
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
}

// ErrFingerprintExists is returned by AppendProfileRevision when the profile
// already holds a revision with the same fingerprint.
var ErrFingerprintExists = errors.New("fingerprint exists")

const profileColumns = `id, current_revision_id, created_at, updated_at`

const revisionColumns = `id, profile_id, revision, fingerprint, display_name, active, document, credential_file, ca_bundle_file, created_by, created_at`

func scanProfile(row pgx.Row) (Profile, error) {
	var p Profile
	if err := row.Scan(&p.ID, &p.CurrentRevisionID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return Profile{}, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, nil
}

func scanRevision(row pgx.Row) (ProfileRevision, error) {
	var r ProfileRevision
	err := row.Scan(&r.ID, &r.ProfileID, &r.Revision, &r.Fingerprint, &r.DisplayName, &r.Active,
		&r.Document, &r.CredentialFile, &r.CABundleFile, &r.CreatedBy, &r.CreatedAt)
	if err != nil {
		return ProfileRevision{}, err
	}
	r.CreatedAt = r.CreatedAt.UTC()
	return r, nil
}

// GetOrCreateProfile inserts the identity when absent and returns it locked
// FOR UPDATE for the transaction.
func GetOrCreateProfile(ctx context.Context, tx pgx.Tx, id string, now time.Time) (Profile, error) {
	_, err := tx.Exec(ctx, `
INSERT INTO profiles (id, current_revision_id, created_at, updated_at)
VALUES ($1, NULL, $2, $2)
ON CONFLICT (id) DO NOTHING`, id, now)
	if err != nil {
		return Profile{}, fmt.Errorf("insert profile: %w", err)
	}
	return GetProfileForUpdate(ctx, tx, id)
}

// GetProfileForUpdate reads the identity and locks it for the transaction.
func GetProfileForUpdate(ctx context.Context, tx pgx.Tx, id string) (Profile, error) {
	p, err := scanProfile(tx.QueryRow(ctx, `SELECT `+profileColumns+` FROM profiles WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return p, nil
}

// AppendProfileRevision inserts r as the next revision of its profile and
// returns the row written. The number is worked out inside the insert and
// the Revision of r is ignored.
func AppendProfileRevision(ctx context.Context, tx pgx.Tx, r ProfileRevision) (ProfileRevision, error) {
	got, err := scanRevision(tx.QueryRow(ctx, `
INSERT INTO profile_revisions (`+revisionColumns+`)
VALUES ($1, $2, (SELECT coalesce(max(revision), 0) + 1 FROM profile_revisions WHERE profile_id = $2),
        $3, $4, $5, $6::jsonb, $7, $8, $9, $10)
RETURNING `+revisionColumns,
		r.ID, r.ProfileID, r.Fingerprint, r.DisplayName, r.Active, r.Document,
		r.CredentialFile, r.CABundleFile, r.CreatedBy, r.CreatedAt))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_profile_revisions_profile_fingerprint" {
			return ProfileRevision{}, ErrFingerprintExists
		}
		return ProfileRevision{}, fmt.Errorf("append profile revision: %w", err)
	}
	return got, nil
}

// SetCurrentRevision moves the pointer of the profile, or clears it when
// revisionID is nil, and sets updated_at.
func SetCurrentRevision(ctx context.Context, q Querier, profileID string, revisionID *uuid.UUID, now time.Time) error {
	tag, err := q.Exec(ctx, `UPDATE profiles SET current_revision_id = $2, updated_at = $3 WHERE id = $1`, profileID, revisionID, now)
	if err != nil {
		return fmt.Errorf("set current revision: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func getRevision(ctx context.Context, q Querier, what, sql string, args ...any) (ProfileRevision, error) {
	r, err := scanRevision(q.QueryRow(ctx, sql, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return ProfileRevision{}, ErrNotFound
	}
	if err != nil {
		return ProfileRevision{}, fmt.Errorf("%s: %w", what, err)
	}
	return r, nil
}

// GetProfileRevision reads one revision by id.
func GetProfileRevision(ctx context.Context, q Querier, id uuid.UUID) (ProfileRevision, error) {
	return getRevision(ctx, q, "get profile revision",
		`SELECT `+revisionColumns+` FROM profile_revisions WHERE id = $1`, id)
}

const currentRevisionJoin = ` FROM profile_revisions r JOIN profiles p ON p.current_revision_id = r.id`

const prefixedRevisionColumns = `r.id, r.profile_id, r.revision, r.fingerprint, r.display_name, r.active, r.document, r.credential_file, r.ca_bundle_file, r.created_by, r.created_at`

// GetCurrentRevision reads the current revision of a profile. It returns
// ErrNotFound when the profile is absent or its pointer is null.
func GetCurrentRevision(ctx context.Context, q Querier, profileID string) (ProfileRevision, error) {
	return getRevision(ctx, q, "get current revision",
		`SELECT `+prefixedRevisionColumns+currentRevisionJoin+` WHERE p.id = $1`, profileID)
}

// ListCurrentRevisions reads every current revision in profile id order.
func ListCurrentRevisions(ctx context.Context, q Querier) ([]ProfileRevision, error) {
	rows, err := q.Query(ctx, `SELECT `+prefixedRevisionColumns+currentRevisionJoin+` ORDER BY p.id`)
	if err != nil {
		return nil, fmt.Errorf("list current revisions: %w", err)
	}
	defer rows.Close()
	var out []ProfileRevision
	for rows.Next() {
		r, err := scanRevision(rows)
		if err != nil {
			return nil, fmt.Errorf("list current revisions: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list current revisions: %w", err)
	}
	return out, nil
}

// FindProfileRevisionByFingerprint reads the revision of the profile with
// that fingerprint.
func FindProfileRevisionByFingerprint(ctx context.Context, q Querier, profileID, fingerprint string) (ProfileRevision, error) {
	return getRevision(ctx, q, "find profile revision by fingerprint",
		`SELECT `+revisionColumns+` FROM profile_revisions WHERE profile_id = $1 AND fingerprint = $2`, profileID, fingerprint)
}

// LatestProfileRevision reads the revision with the highest number of the
// profile, whether or not the pointer names it.
func LatestProfileRevision(ctx context.Context, q Querier, profileID string) (ProfileRevision, error) {
	return getRevision(ctx, q, "get latest profile revision",
		`SELECT `+revisionColumns+` FROM profile_revisions WHERE profile_id = $1 ORDER BY revision DESC LIMIT 1`, profileID)
}
