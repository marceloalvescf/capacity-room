package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/audit"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
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
	// LanguageFollowsBrowser true unpins the language and keeps the stored one.
	LanguageFollowsBrowser *bool
}

// Change applies a language change and, when Password is set, verifies
// CurrentPassword, checks the policy, writes the hash, revokes every
// session of the reader as password_changed, and audits both actions.
// Before CurrentPassword is verified, an attempt is taken from the sign-in
// throttle under the reader's name and clientAddress: a blocked key returns
// *ErrThrottled, a wrong current password returns ErrInvalidCredentials
// and stays counted, and a right one resets the throttle.
func (a *Account) Change(ctx context.Context, p Principal, clientAddress string, c AccountChange) (store.User, error) {
	if c.Language != nil && !validLanguage(*c.Language) {
		return store.User{}, ErrInvalidLanguage
	}
	if c.Password != nil && c.CurrentPassword == nil {
		return store.User{}, ErrInvalidCredentials
	}

	now := a.Now()
	// The user is read through the pool and the hashes run outside any
	// transaction, so no connection is held while they run.
	user, err := store.GetUser(ctx, a.Pool, p.ID)
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, ErrUserNotFound
	}
	if err != nil {
		return store.User{}, err
	}
	language, follows := languageAfter(user, c.Language, c.LanguageFollowsBrowser)
	languageMoved := language != user.Language || follows != user.LanguageFollowsBrowser
	if !languageMoved && c.Password == nil {
		return user, nil
	}
	fresh := ""
	if c.Password != nil {
		key := a.Sessions.Throttle.Key(p.Username, clientAddress)
		if err := a.Sessions.Throttle.Take(ctx, a.Pool, key, now); err != nil {
			return store.User{}, err
		}
		match, _, err := VerifyPassword(ctx, *c.CurrentPassword, user.PasswordHash)
		if err != nil {
			return store.User{}, fmt.Errorf("verify password: %w", err)
		}
		if !match {
			return store.User{}, ErrInvalidCredentials
		}
		if err := a.Sessions.Throttle.RecordSuccess(ctx, a.Pool, key, now); err != nil {
			return store.User{}, err
		}
		if err := CheckPassword(*c.Password); err != nil {
			return store.User{}, err
		}
		hash, err := HashPassword(ctx, *c.Password)
		if err != nil {
			return store.User{}, fmt.Errorf("hash password: %w", err)
		}
		fresh = hash
	}

	// Each change writes its column alone. The password is written only
	// over the hash that was verified: when another change wrote first, the
	// current password given here is no longer known to be right.
	err = store.Transact(ctx, a.Pool, func(tx pgx.Tx) error {
		if c.Password != nil {
			written, err := store.UpdatePasswordHash(ctx, tx, p.ID, user.PasswordHash, fresh, now)
			if err != nil {
				return err
			}
			if !written {
				return ErrInvalidCredentials
			}
		}
		if languageMoved {
			if err := store.UpdateLanguagePin(ctx, tx, p.ID, language, follows, now); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return ErrUserNotFound
				}
				return err
			}
			detail := audit.Metadata{"language": language}
			if c.Language == nil {
				detail = audit.Metadata{"language_follows_browser": follows}
			}
			err := audit.Append(ctx, tx, now, audit.Event{
				ActorID:    &p.ID,
				Action:     audit.ActionAccountLanguageChanged,
				TargetType: "user",
				TargetID:   p.ID.String(),
				Metadata:   detail,
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
	if err != nil {
		return store.User{}, err
	}
	if c.Password != nil {
		user.PasswordHash = fresh
	}
	if languageMoved {
		user.Language, user.LanguageFollowsBrowser = language, follows
	}
	user.UpdatedAt = now
	return user, nil
}
