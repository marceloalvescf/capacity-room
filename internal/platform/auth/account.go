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

// Account is the reader's own account, spec 11.10 PATCH /account.
type Account struct {
	Pool     *pgxpool.Pool
	Sessions *Sessions
	Now      func() time.Time
}

// AccountChange is what a reader changes in their own account; a nil field
// is left alone.
type AccountChange struct {
	CurrentPassword *string
	Password        *string
	Language        *string
}

// Change applies a language change and, when Password is set, verifies
// CurrentPassword, checks the policy, writes the hash, revokes every
// session of the reader as password_changed, and audits both actions. A
// wrong current password returns ErrInvalidCredentials and counts a
// failure of the throttle under the reader's name and clientAddress.
func (a *Account) Change(ctx context.Context, p Principal, clientAddress string, c AccountChange) (store.User, error) {
	if c.Language != nil && !validLanguage(*c.Language) {
		return store.User{}, ErrInvalidLanguage
	}
	if c.Password != nil && c.CurrentPassword == nil {
		return store.User{}, ErrInvalidCredentials
	}

	now := a.Now()
	var out store.User
	wrongCurrent := false
	err := store.Transact(ctx, a.Pool, func(tx pgx.Tx) error {
		user, err := store.GetUser(ctx, tx, p.ID)
		if errors.Is(err, store.ErrNotFound) {
			return ErrUserNotFound
		}
		if err != nil {
			return err
		}
		out = user

		languageMoved := c.Language != nil && *c.Language != user.Language
		if !languageMoved && c.Password == nil {
			return nil
		}
		if c.Password != nil {
			if match, _ := VerifyPassword(*c.CurrentPassword, user.PasswordHash); !match {
				wrongCurrent = true
				return ErrInvalidCredentials
			}
			if err := CheckPassword(*c.Password); err != nil {
				return err
			}
			hash, err := HashPassword(*c.Password)
			if err != nil {
				return fmt.Errorf("hash password: %w", err)
			}
			user.PasswordHash = hash
		}
		if languageMoved {
			user.Language = *c.Language
		}
		user.UpdatedAt = now
		if err := store.UpdateUser(ctx, tx, user); err != nil {
			return err
		}
		out = user

		if languageMoved {
			err := audit.Append(ctx, tx, now, audit.Event{
				ActorID:    &p.ID,
				Action:     audit.ActionAccountLanguageChanged,
				TargetType: "user",
				TargetID:   p.ID.String(),
				Metadata:   audit.Metadata{"language": user.Language},
			})
			if err != nil {
				return err
			}
		}
		if c.Password != nil {
			revoked, err := store.RevokeSessionsOfUser(ctx, tx, p.ID, revokedPasswordChanged, now)
			if err != nil {
				return err
			}
			return audit.Append(ctx, tx, now, audit.Event{
				ActorID:    &p.ID,
				Action:     audit.ActionAccountPasswordChanged,
				TargetType: "user",
				TargetID:   p.ID.String(),
				Metadata:   audit.Metadata{"sessions_revoked": revoked},
			})
		}
		return nil
	})
	if wrongCurrent {
		key := a.Sessions.Throttle.Key(p.Username, clientAddress)
		if err := a.Sessions.Throttle.RecordFailure(ctx, a.Pool, key, now); err != nil {
			return store.User{}, err
		}
	}
	if err != nil {
		return store.User{}, err
	}
	return out, nil
}
