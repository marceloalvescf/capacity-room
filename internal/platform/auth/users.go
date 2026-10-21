package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/audit"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

// Users is the administration of accounts, spec 11.3.
type Users struct {
	Pool *pgxpool.Pool
	Now  func() time.Time

	// beforeWrite runs inside the transaction of Update, after the user is
	// read and before it is written. Tests set it; it is nil otherwise.
	beforeWrite func()
}

// The refusals of the users and account services.
var (
	ErrForbidden       = errors.New("forbidden")
	ErrUserNotFound    = errors.New("user not found")
	ErrUsernameTaken   = store.ErrUsernameTaken
	ErrLastAdmin       = errors.New("last administrator")
	ErrSelfDelete      = errors.New("self delete")
	ErrSelfDeactivate  = errors.New("self deactivate")
	ErrSelfDemote      = errors.New("self demote")
	ErrInvalidRole     = errors.New("invalid role")
	ErrInvalidLanguage = errors.New("invalid language")
)

// Reasons the sessions of an account are revoked.
const (
	revokedPrivilegeChanged = "privilege_changed"
	revokedPasswordChanged  = "password_changed"
	revokedUserDeleted      = "user_deleted"
)

func validRole(role string) bool {
	return role == RoleAdministrator || role == RoleOperator || role == RoleReader
}

func validLanguage(language string) bool {
	return language == "pt-BR" || language == "en"
}

func requireAdministrator(actor Principal) error {
	if actor.Role != RoleAdministrator {
		return ErrForbidden
	}
	return nil
}

// NewUser is the content of an account to create.
type NewUser struct {
	Username string
	Password string
	Role     Role
	// Language is empty when the account follows the browser's language; it
	// then stores en.
	Language string
}

// Create validates the name, the password, the role, and the language (an
// empty one means the account follows the browser),
// hashes, inserts, and audits user.create with {"role": role}. actor must
// be an administrator.
func (u *Users) Create(ctx context.Context, actor Principal, n NewUser) (store.User, error) {
	if err := requireAdministrator(actor); err != nil {
		return store.User{}, err
	}
	name, err := ValidateUsername(n.Username)
	if err != nil {
		return store.User{}, err
	}
	if err := CheckPassword(n.Password); err != nil {
		return store.User{}, err
	}
	if !validRole(n.Role) {
		return store.User{}, ErrInvalidRole
	}
	if n.Language != "" && !validLanguage(n.Language) {
		return store.User{}, ErrInvalidLanguage
	}
	hash, err := HashPassword(ctx, n.Password)
	if err != nil {
		return store.User{}, fmt.Errorf("hash password: %w", err)
	}
	now := u.Now()
	user := store.User{
		ID: uuid.New(), Username: name, PasswordHash: hash, Role: n.Role,
		Language: n.Language, Active: true, CreatedAt: now, UpdatedAt: now,
	}
	if n.Language == "" {
		user.Language = "en"
		user.LanguageFollowsBrowser = true
	}
	err = store.Transact(ctx, u.Pool, func(tx pgx.Tx) error {
		if err := store.InsertUser(ctx, tx, user); err != nil {
			return err
		}
		return audit.Append(ctx, tx, now, audit.Event{
			ActorID:    &actor.ID,
			Action:     audit.ActionUserCreate,
			TargetType: "user",
			TargetID:   user.ID.String(),
			Metadata:   audit.Metadata{"role": user.Role},
		})
	})
	if err != nil {
		return store.User{}, err
	}
	return user, nil
}

// UserChange is the set of fields an administrator changes; a nil field is
// left alone.
type UserChange struct {
	Role     *string
	Active   *bool
	Password *string
	Language *string
	// LanguageFollowsBrowser true unpins the language and keeps the stored one.
	LanguageFollowsBrowser *bool
}

// Update applies the fields that are set, in the order of the original:
// the self checks, the load, the moved fields, the last-administrator rule,
// then the password; a role or active change revokes every session as
// privilege_changed, a password change as password_changed. It audits
// user.update with {"changed": {"role": "<new>", ...}} naming the fields
// that moved and their new values for role, active, language, and language_follows_browser only, plus
// {"sessions_revoked": n}; a password change audits user.password_changed
// with {"sessions_revoked": n}. A change that moves nothing writes nothing.
func (u *Users) Update(ctx context.Context, actor Principal, id uuid.UUID, c UserChange) (store.User, error) {
	if err := requireAdministrator(actor); err != nil {
		return store.User{}, err
	}
	if actor.ID == id {
		if c.Active != nil && !*c.Active {
			return store.User{}, ErrSelfDeactivate
		}
		if c.Role != nil && *c.Role != RoleAdministrator {
			return store.User{}, ErrSelfDemote
		}
	}
	if c.Role != nil && !validRole(*c.Role) {
		return store.User{}, ErrInvalidRole
	}
	if c.Language != nil && !validLanguage(*c.Language) {
		return store.User{}, ErrInvalidLanguage
	}

	// The checks that need the row read it through the pool and the hash
	// runs before the transaction, so no connection is held while it runs.
	// The transaction reads the row again under the table lock and applies
	// the same checks to it.
	now := u.Now()
	user, err := store.GetUser(ctx, u.Pool, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, ErrUserNotFound
	}
	if err != nil {
		return store.User{}, err
	}
	changed, privilege := movedFields(user, c)
	if len(changed) == 0 && c.Password == nil {
		return user, nil
	}
	if err := checkLastAdministrator(ctx, u.Pool, user, c, changed, privilege); err != nil {
		return store.User{}, err
	}
	hash := ""
	if c.Password != nil {
		if err := CheckPassword(*c.Password); err != nil {
			return store.User{}, err
		}
		if hash, err = HashPassword(ctx, *c.Password); err != nil {
			return store.User{}, fmt.Errorf("hash password: %w", err)
		}
	}

	out := user
	err = store.Transact(ctx, u.Pool, func(tx pgx.Tx) error {
		if err := store.LockUsers(ctx, tx); err != nil {
			return err
		}
		user, err := store.GetUser(ctx, tx, id)
		if errors.Is(err, store.ErrNotFound) {
			return ErrUserNotFound
		}
		if err != nil {
			return err
		}
		out = user
		changed, privilege = movedFields(user, c)
		if len(changed) == 0 && c.Password == nil {
			return nil
		}
		if err := checkLastAdministrator(ctx, tx, user, c, changed, privilege); err != nil {
			return err
		}

		if u.beforeWrite != nil {
			u.beforeWrite()
		}
		if c.Role != nil {
			user.Role = *c.Role
		}
		if c.Active != nil {
			user.Active = *c.Active
		}
		user.Language, user.LanguageFollowsBrowser = languageAfter(user, c.Language, c.LanguageFollowsBrowser)
		user.UpdatedAt = now
		if err := store.UpdateUserFields(ctx, tx, id, user.Role, user.Language, user.LanguageFollowsBrowser, user.Active, now); err != nil {
			return err
		}
		if c.Password != nil {
			// Written over the hash that was read. The table lock conflicts
			// with the row lock of every UPDATE, so a password writer that
			// started first has committed and one that starts later waits;
			// a miss is a broken invariant.
			written, err := store.UpdatePasswordHash(ctx, tx, id, user.PasswordHash, hash, now)
			if err != nil {
				return err
			}
			if !written {
				return errors.New("password hash moved under the table lock")
			}
			user.PasswordHash = hash
		}
		out = user

		privilegeRevoked := 0
		if privilege {
			privilegeRevoked, err = store.RevokeSessionsOfUser(ctx, tx, id, revokedPrivilegeChanged, now)
			if err != nil {
				return err
			}
		}
		if len(changed) > 0 {
			err := audit.Append(ctx, tx, now, audit.Event{
				ActorID:    &actor.ID,
				Action:     audit.ActionUserUpdate,
				TargetType: "user",
				TargetID:   id.String(),
				Metadata:   audit.Metadata{"changed": changed, "sessions_revoked": privilegeRevoked},
			})
			if err != nil {
				return err
			}
		}
		if c.Password != nil {
			passwordRevoked, err := store.RevokeSessionsOfUser(ctx, tx, id, revokedPasswordChanged, now)
			if err != nil {
				return err
			}
			return audit.Append(ctx, tx, now, audit.Event{
				ActorID:    &actor.ID,
				Action:     audit.ActionUserPasswordChanged,
				TargetType: "user",
				TargetID:   id.String(),
				Metadata:   audit.Metadata{"sessions_revoked": passwordRevoked},
			})
		}
		return nil
	})
	if err != nil {
		return store.User{}, err
	}
	return out, nil
}

// movedFields returns the fields of c that differ from user, with their new
// values, and whether role or active is among them.
func movedFields(user store.User, c UserChange) (changed audit.Metadata, privilege bool) {
	changed = audit.Metadata{}
	if c.Role != nil && *c.Role != user.Role {
		changed["role"] = *c.Role
		privilege = true
	}
	if c.Active != nil && *c.Active != user.Active {
		changed["active"] = *c.Active
		privilege = true
	}
	language, follows := languageAfter(user, c.Language, c.LanguageFollowsBrowser)
	if language != user.Language {
		changed["language"] = language
	}
	if follows != user.LanguageFollowsBrowser {
		changed["language_follows_browser"] = follows
	}
	return changed, privilege
}

// checkLastAdministrator returns ErrLastAdmin when the change demotes or
// deactivates the only active administrator.
func checkLastAdministrator(ctx context.Context, q store.Querier, user store.User, c UserChange, changed audit.Metadata, privilege bool) error {
	if !privilege || user.Role != RoleAdministrator || !user.Active {
		return nil
	}
	_, demoted := changed["role"]
	_, deactivated := changed["active"]
	if !demoted && (!deactivated || *c.Active) {
		return nil
	}
	n, err := store.CountActiveAdministrators(ctx, q)
	if err != nil {
		return err
	}
	if n <= 1 {
		return ErrLastAdmin
	}
	return nil
}

// Delete deactivates the user and revokes every session as user_deleted,
// auditing user.delete with {"sessions_revoked": n}.
func (u *Users) Delete(ctx context.Context, actor Principal, id uuid.UUID) error {
	if err := requireAdministrator(actor); err != nil {
		return err
	}
	if actor.ID == id {
		return ErrSelfDelete
	}
	now := u.Now()
	return store.Transact(ctx, u.Pool, func(tx pgx.Tx) error {
		if err := store.LockUsers(ctx, tx); err != nil {
			return err
		}
		user, err := store.GetUser(ctx, tx, id)
		if errors.Is(err, store.ErrNotFound) {
			return ErrUserNotFound
		}
		if err != nil {
			return err
		}
		if user.Role == RoleAdministrator && user.Active {
			n, err := store.CountActiveAdministrators(ctx, tx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return ErrLastAdmin
			}
		}
		if err := store.UpdateUserFields(ctx, tx, id, user.Role, user.Language, user.LanguageFollowsBrowser, false, now); err != nil {
			return err
		}
		revoked, err := store.RevokeSessionsOfUser(ctx, tx, id, revokedUserDeleted, now)
		if err != nil {
			return err
		}
		return audit.Append(ctx, tx, now, audit.Event{
			ActorID:    &actor.ID,
			Action:     audit.ActionUserDelete,
			TargetType: "user",
			TargetID:   id.String(),
			Metadata:   audit.Metadata{"sessions_revoked": revoked},
		})
	})
}

// Get returns one account; actor must be an administrator.
func (u *Users) Get(ctx context.Context, actor Principal, id uuid.UUID) (store.User, error) {
	if err := requireAdministrator(actor); err != nil {
		return store.User{}, err
	}
	user, err := store.GetUser(ctx, u.Pool, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, ErrUserNotFound
	}
	return user, err
}

// List returns every account in name order; actor must be an administrator.
func (u *Users) List(ctx context.Context, actor Principal) ([]store.User, error) {
	if err := requireAdministrator(actor); err != nil {
		return nil, err
	}
	return store.ListUsers(ctx, u.Pool)
}

// languageAfter returns the language and the follow flag of user once a
// pinned language and a follow flag are applied: a pinned language clears
// the flag, and a flag that is set then has the last word.
func languageAfter(user store.User, language *string, follows *bool) (string, bool) {
	outLanguage, outFollows := user.Language, user.LanguageFollowsBrowser
	if language != nil {
		outLanguage, outFollows = *language, false
	}
	if follows != nil {
		outFollows = *follows
	}
	return outLanguage, outFollows
}
