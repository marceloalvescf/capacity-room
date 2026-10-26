package auth

import (
	"context"
	"crypto/hmac"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/audit"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

// Principal is what a session proves: the account as the database holds it
// at this request.
type Principal struct {
	ID       uuid.UUID
	Username string
	Role     string
	Language string
	// LanguageFollowsBrowser is true while the account takes the browser's
	// language, so Language is not the language in use.
	LanguageFollowsBrowser bool
}

// Role is the name of a role as the users table stores it.
type Role = string

// The roles of an account.
const (
	RoleAdministrator Role = "administrator"
	RoleOperator      Role = "operator"
	RoleReader        Role = "reader"
)

// Sessions signs readers in and out and authenticates their requests.
type Sessions struct {
	Pool     *pgxpool.Pool
	Throttle Throttle
	TTL      time.Duration
	Idle     time.Duration
	Now      func() time.Time

	// clients bounds the password work in flight to one sign-in per client.
	clients clientSlots

	// beforeSession runs before the transaction that inserts the session of
	// a sign-in. Tests set it; it is nil otherwise.
	beforeSession func()

	// afterUserRead runs inside that transaction once the user is re-read.
	// Tests set it; it is nil otherwise.
	afterUserRead func()
}

// The refusals of the sessions service.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrCSRF               = errors.New("csrf rejected")
)

// Reasons a session is revoked.
const (
	revokedRotated      = "rotated"
	revokedLogout       = "logout"
	revokedUserInactive = "user_inactive"
)

func principalOf(u store.User) Principal {
	return Principal{ID: u.ID, Username: u.Username, Role: u.Role, Language: u.Language, LanguageFollowsBrowser: u.LanguageFollowsBrowser}
}

// Issued is a new session: the raw tokens, given to the client once.
type Issued struct {
	SessionID uuid.UUID
	Token     string
	CSRFToken string
	ExpiresAt time.Time
	Principal Principal
}

// checkPassword verifies password against stored during the turn of client,
// and returns whether a found user matched and, when the hash needs it, a
// fresh hash of password. The turn comes before any hash slot, so a sign-in
// waiting for it holds no slot, and it ends with the password work.
func (s *Sessions) checkPassword(ctx context.Context, client, password, stored string, found bool) (matched bool, fresh string, err error) {
	release, err := s.clients.hold(ctx, client)
	if err != nil {
		return false, "", fmt.Errorf("verify password: %w", err)
	}
	defer release()
	match, rehash, err := VerifyPassword(ctx, password, stored)
	if err != nil {
		return false, "", fmt.Errorf("verify password: %w", err)
	}
	matched = found && match
	if matched && rehash {
		if fresh, err = HashPassword(ctx, password); err != nil {
			return false, "", fmt.Errorf("hash password: %w", err)
		}
	}
	return matched, fresh, nil
}

// Login runs the sign-in: the throttle, one hash verification, the audit
// row, the rotation of a presented session, the new session. presented is
// the session cookie of the request, or empty.
func (s *Sessions) Login(ctx context.Context, username, password, clientAddress, presented string) (Issued, error) {
	now := s.Now()
	normalized := NormalizeUsername(username)
	key := s.Throttle.Key(normalized, clientAddress)
	if err := s.Throttle.Take(ctx, s.Pool, key, now); err != nil {
		return Issued{}, err
	}

	// The user is read through the pool and the hash runs outside any
	// transaction, so no connection is held while it runs.
	var user store.User
	found := false
	if UsernameMatchesPattern(normalized) {
		u, err := store.FindActiveUserByUsername(ctx, s.Pool, normalized)
		switch {
		case err == nil:
			user, found = u, true
		case !errors.Is(err, store.ErrNotFound):
			return Issued{}, err
		}
	}
	stored := DummyHash()
	if found {
		stored = user.PasswordHash
	}
	matched, fresh, err := s.checkPassword(ctx, clientKey(clientAddress), password, stored, found)
	if err != nil {
		return Issued{}, err
	}
	// verified is the hash the password is known to match: the stored one,
	// or the fresh one once the rehash wrote it. A rehash refused because
	// the hash moved meanwhile is not an error; the session check refuses.
	verified := user.PasswordHash
	err = store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		if matched {
			if fresh == "" {
				return nil
			}
			written, err := store.UpdatePasswordHash(ctx, tx, user.ID, user.PasswordHash, fresh, now)
			if written {
				verified = fresh
			}
			return err
		}
		event := audit.Event{
			Action:   audit.ActionLoginFailed,
			Outcome:  audit.OutcomeFailed,
			Metadata: audit.Metadata{"code": "INVALID_CREDENTIALS"},
		}
		if found {
			event.TargetType, event.TargetID = "user", user.ID.String()
		}
		return audit.Append(ctx, tx, now, event)
	})
	if err != nil {
		return Issued{}, err
	}
	if !matched {
		return Issued{}, ErrInvalidCredentials
	}
	if err := s.Throttle.RecordSuccess(ctx, s.Pool, key, now); err != nil {
		return Issued{}, err
	}

	token, err := NewToken()
	if err != nil {
		return Issued{}, err
	}
	csrf, err := NewToken()
	if err != nil {
		return Issued{}, err
	}
	issued := Issued{
		SessionID: uuid.New(),
		Token:     token,
		CSRFToken: csrf,
		ExpiresAt: now.Add(min(s.TTL, s.Idle)),
		Principal: principalOf(user),
	}
	if s.beforeSession != nil {
		s.beforeSession()
	}
	err = store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		// The account may have changed since the hash was verified: a new
		// password or a deactivation refuses the sign-in. The share lock
		// holds the row until the session exists, so a password change
		// that follows waits and then revokes it.
		current, err := store.GetUserForShare(ctx, tx, user.ID)
		if errors.Is(err, store.ErrNotFound) {
			return ErrInvalidCredentials
		}
		if err != nil {
			return err
		}
		if !current.Active || current.PasswordHash != verified {
			return ErrInvalidCredentials
		}
		if s.afterUserRead != nil {
			s.afterUserRead()
		}
		var parent *uuid.UUID
		if presented != "" {
			old, err := store.FindActiveSessionByTokenHash(ctx, tx, Digest(presented), now)
			switch {
			case err == nil:
				if _, err := store.RevokeSession(ctx, tx, old.ID, revokedRotated, now); err != nil {
					return err
				}
				if old.UserID == user.ID {
					parent = &old.ID
				}
			case !errors.Is(err, store.ErrNotFound):
				return err
			}
		}
		err = store.InsertSession(ctx, tx, store.Session{
			ID:            issued.SessionID,
			TokenHash:     Digest(token),
			CSRFHash:      Digest(csrf),
			UserID:        user.ID,
			CreatedAt:     now,
			ExpiresAt:     issued.ExpiresAt,
			RotatedFromID: parent,
		})
		if err != nil {
			return err
		}
		return audit.Append(ctx, tx, now, audit.Event{
			ActorID:    &user.ID,
			Action:     audit.ActionLoginSucceeded,
			TargetType: "session",
			TargetID:   issued.SessionID.String(),
		})
	})
	if err != nil {
		return Issued{}, err
	}
	return issued, nil
}

// Authenticated is a request's session after the slide.
type Authenticated struct {
	SessionID uuid.UUID
	ExpiresAt time.Time
	Principal Principal
}

// Authenticate slides the session of the cookie and re-reads the account.
// A missing, expired, or revoked session, and an inactive account, answer
// ErrUnauthenticated; an inactive account revokes the session first.
func (s *Sessions) Authenticate(ctx context.Context, token string) (Authenticated, error) {
	now := s.Now()
	var out Authenticated
	err := store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		session, err := store.TouchSession(ctx, tx, Digest(token), now, s.TTL, s.Idle)
		if errors.Is(err, store.ErrNotFound) {
			return ErrUnauthenticated
		}
		if err != nil {
			return err
		}
		user, err := store.GetUser(ctx, tx, session.UserID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if err != nil || !user.Active {
			if _, err := store.RevokeSession(ctx, tx, session.ID, revokedUserInactive, now); err != nil {
				return err
			}
			return nil
		}
		out = Authenticated{SessionID: session.ID, ExpiresAt: session.ExpiresAt, Principal: principalOf(user)}
		return nil
	})
	if err != nil {
		return Authenticated{}, err
	}
	if out.SessionID == uuid.Nil {
		return Authenticated{}, ErrUnauthenticated
	}
	return out, nil
}

// VerifyCSRF compares the header token with the digest of the session row.
func (s *Sessions) VerifyCSRF(ctx context.Context, sessionID uuid.UUID, headerToken string) error {
	if headerToken == "" {
		return ErrCSRF
	}
	row, err := store.FindActiveSessionByID(ctx, s.Pool, sessionID, s.Now())
	if errors.Is(err, store.ErrNotFound) {
		return ErrCSRF
	}
	if err != nil {
		return err
	}
	if !hmac.Equal(Digest(headerToken), row.CSRFHash) {
		return ErrCSRF
	}
	return nil
}

// RotateCSRF issues a new CSRF token for the session and stores its digest.
func (s *Sessions) RotateCSRF(ctx context.Context, sessionID uuid.UUID) (string, error) {
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	if err := store.ReplaceSessionCSRF(ctx, s.Pool, sessionID, Digest(token), s.Now()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", ErrUnauthenticated
		}
		return "", err
	}
	return token, nil
}

// Logout revokes the session of the token as "logout" and writes the audit
// row. An unknown token is not an error.
func (s *Sessions) Logout(ctx context.Context, token string) error {
	now := s.Now()
	return store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		session, err := store.FindActiveSessionByTokenHash(ctx, tx, Digest(token), now)
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		revoked, err := store.RevokeSession(ctx, tx, session.ID, revokedLogout, now)
		if err != nil || !revoked {
			return err
		}
		return audit.Append(ctx, tx, now, audit.Event{
			ActorID:    &session.UserID,
			Action:     audit.ActionLogout,
			TargetType: "session",
			TargetID:   session.ID.String(),
		})
	})
}

// NewSessions makes the sign-in service. It makes the dummy hash now, so
// the first sign-in of an unknown user costs one verification like any
// other.
func NewSessions(pool *pgxpool.Pool, throttle Throttle, ttl, idle time.Duration, now func() time.Time) *Sessions {
	DummyHash()
	return &Sessions{Pool: pool, Throttle: throttle, TTL: ttl, Idle: idle, Now: now}
}
