package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Session is a row of the sessions table.
type Session struct {
	ID            uuid.UUID
	TokenHash     []byte
	CSRFHash      []byte
	UserID        uuid.UUID
	CreatedAt     time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	RevokedReason *string
	RotatedFromID *uuid.UUID
}

const sessionColumns = `id, token_hash, csrf_hash, user_id, created_at, expires_at, revoked_at, revoked_reason, rotated_from_id`

func scanSession(row pgx.Row) (Session, error) {
	var s Session
	err := row.Scan(&s.ID, &s.TokenHash, &s.CSRFHash, &s.UserID, &s.CreatedAt, &s.ExpiresAt,
		&s.RevokedAt, &s.RevokedReason, &s.RotatedFromID)
	if err != nil {
		return Session{}, err
	}
	s.CreatedAt = s.CreatedAt.UTC()
	s.ExpiresAt = s.ExpiresAt.UTC()
	if s.RevokedAt != nil {
		t := s.RevokedAt.UTC()
		s.RevokedAt = &t
	}
	return s, nil
}

func interval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}

// InsertSession inserts s.
func InsertSession(ctx context.Context, q Querier, s Session) error {
	_, err := q.Exec(ctx, `
INSERT INTO sessions (id, token_hash, csrf_hash, user_id, created_at, expires_at, revoked_at, revoked_reason, rotated_from_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		s.ID, s.TokenHash, s.CSRFHash, s.UserID, s.CreatedAt, s.ExpiresAt, s.RevokedAt, s.RevokedReason, s.RotatedFromID)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// TouchSession moves expires_at of the active session with this token hash
// to least(created_at + ttl, now + idle) and returns the row after the
// move. ErrNotFound when no active session has the hash.
func TouchSession(ctx context.Context, q Querier, tokenHash []byte, now time.Time, ttl, idle time.Duration) (Session, error) {
	s, err := scanSession(q.QueryRow(ctx, `
UPDATE sessions
   SET expires_at = LEAST(created_at + $3::interval, $2::timestamptz + $4::interval)
 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
RETURNING `+sessionColumns, tokenHash, now, interval(ttl), interval(idle)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("touch session: %w", err)
	}
	return s, nil
}

// FindActiveSessionByTokenHash finds the unrevoked, unexpired session with
// this token hash.
func FindActiveSessionByTokenHash(ctx context.Context, q Querier, tokenHash []byte, now time.Time) (Session, error) {
	s, err := scanSession(q.QueryRow(ctx, `
SELECT `+sessionColumns+` FROM sessions
 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2`, tokenHash, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("find active session by token hash: %w", err)
	}
	return s, nil
}

// FindActiveSessionByID finds the unrevoked, unexpired session with this id.
func FindActiveSessionByID(ctx context.Context, q Querier, id uuid.UUID, now time.Time) (Session, error) {
	s, err := scanSession(q.QueryRow(ctx, `
SELECT `+sessionColumns+` FROM sessions
 WHERE id = $1 AND revoked_at IS NULL AND expires_at > $2`, id, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("find active session by id: %w", err)
	}
	return s, nil
}

// ReplaceSessionCSRF overwrites csrf_hash of the active session with id,
// and returns ErrNotFound when no active session has it.
func ReplaceSessionCSRF(ctx context.Context, q Querier, id uuid.UUID, csrfHash []byte, now time.Time) error {
	tag, err := q.Exec(ctx, `
UPDATE sessions SET csrf_hash = $2
 WHERE id = $1 AND revoked_at IS NULL AND expires_at > $3`, id, csrfHash, now)
	if err != nil {
		return fmt.Errorf("replace session csrf: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeSession sets revoked_at and revoked_reason where revoked_at IS NULL.
// It reports whether a row changed.
func RevokeSession(ctx context.Context, q Querier, id uuid.UUID, reason string, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx, `
UPDATE sessions SET revoked_at = $3, revoked_reason = $2
 WHERE id = $1 AND revoked_at IS NULL`, id, reason, now)
	if err != nil {
		return false, fmt.Errorf("revoke session: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// RevokeSessionsOfUser revokes every unrevoked session of the user and
// returns the count.
func RevokeSessionsOfUser(ctx context.Context, q Querier, userID uuid.UUID, reason string, now time.Time) (int, error) {
	tag, err := q.Exec(ctx, `
UPDATE sessions SET revoked_at = $3, revoked_reason = $2
 WHERE user_id = $1 AND revoked_at IS NULL`, userID, reason, now)
	if err != nil {
		return 0, fmt.Errorf("revoke sessions of user: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
