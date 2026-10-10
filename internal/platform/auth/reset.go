package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

var (
	// ErrNoActiveUser is what a reset finds when no active user has the name.
	ErrNoActiveUser = errors.New("no active user has that username")
	// ErrPasswordRaced is what a reset finds when the password changed after
	// it was read.
	ErrPasswordRaced = errors.New("the password changed during the reset")
)

// revokedPasswordReset is the reason of the sessions a reset ends.
const revokedPasswordReset = "password_reset"

// ResetPassword sets the password of the active user named username, ends
// every session of that user, and records the reset with no actor. It is the
// way back in for an administrator who lost the password, run on the host.
func ResetPassword(ctx context.Context, pool *pgxpool.Pool, username, password string, now time.Time) (store.User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return store.User{}, fmt.Errorf("hash password: %w", err)
	}
	var out store.User
	err = store.Transact(ctx, pool, func(tx pgx.Tx) error {
		user, err := store.FindActiveUserByUsername(ctx, tx, username)
		if errors.Is(err, store.ErrNotFound) {
			return ErrNoActiveUser
		}
		if err != nil {
			return err
		}
		written, err := store.UpdatePasswordHash(ctx, tx, user.ID, user.PasswordHash, hash, now)
		if err != nil {
			return err
		}
		if !written {
			return ErrPasswordRaced
		}
		user.PasswordHash = hash
		user.UpdatedAt = now
		out = user
		revoked, err := store.RevokeSessionsOfUser(ctx, tx, user.ID, revokedPasswordReset, now)
		if err != nil {
			return err
		}
		return audit.Append(ctx, tx, now, audit.Event{
			Action:     audit.ActionUserPasswordReset,
			TargetType: "user",
			TargetID:   user.ID.String(),
			Metadata:   audit.Metadata{"sessions_revoked": revoked},
		})
	})
	if err != nil {
		return store.User{}, err
	}
	return out, nil
}
