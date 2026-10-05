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

// Querier is what every statement of this package runs on: a pool or a
// transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// User is a row of the users table.
type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Role         string
	Language     string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ErrNotFound is returned by every lookup that finds no row.
var ErrNotFound = errors.New("no such row")

// ErrUsernameTaken is returned by InsertUser when the unique constraint
// refuses the name.
var ErrUsernameTaken = errors.New("username taken")

const userColumns = `id, username, password_hash, role, language, active, created_at, updated_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Language, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	u.UpdatedAt = u.UpdatedAt.UTC()
	return u, nil
}

// InsertUser inserts u.
func InsertUser(ctx context.Context, q Querier, u User) error {
	_, err := q.Exec(ctx, `
INSERT INTO users (id, username, password_hash, role, language, active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		u.ID, u.Username, u.PasswordHash, u.Role, u.Language, u.Active, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_users_username" {
			return ErrUsernameTaken
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// GetUser returns the user with the given id.
func GetUser(ctx context.Context, q Querier, id uuid.UUID) (User, error) {
	return getUser(ctx, q, ``, id)
}

// GetUserForShare reads the user and holds a share lock on the row for the
// transaction, so a concurrent change of the row waits until it commits.
func GetUserForShare(ctx context.Context, tx pgx.Tx, id uuid.UUID) (User, error) {
	return getUser(ctx, tx, ` FOR SHARE`, id)
}

func getUser(ctx context.Context, q Querier, lock string, id uuid.UUID) (User, error) {
	u, err := scanUser(q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`+lock, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

// FindActiveUserByUsername finds the active user of the given stored name.
func FindActiveUserByUsername(ctx context.Context, q Querier, username string) (User, error) {
	u, err := scanUser(q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE username = $1 AND active`, username))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find active user by username: %w", err)
	}
	return u, nil
}

// ListUsers returns every user ordered by username.
func ListUsers(ctx context.Context, q Querier) ([]User, error) {
	rows, err := q.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

// UpdateUserFields writes role, language, active, and updated_at of the
// row with id, and never the hash or the username.
func UpdateUserFields(ctx context.Context, q Querier, id uuid.UUID, role, language string, active bool, now time.Time) error {
	tag, err := q.Exec(ctx, `
UPDATE users SET role = $2, language = $3, active = $4, updated_at = $5 WHERE id = $1`,
		id, role, language, active, now)
	if err != nil {
		return fmt.Errorf("update user fields: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePasswordHash writes newHash and now as password_hash and
// updated_at of the row with id, only when its password_hash is still
// oldHash, and reports whether it wrote.
func UpdatePasswordHash(ctx context.Context, q Querier, id uuid.UUID, oldHash, newHash string, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx, `
UPDATE users SET password_hash = $3, updated_at = $4 WHERE id = $1 AND password_hash = $2`,
		id, oldHash, newHash, now)
	if err != nil {
		return false, fmt.Errorf("update password hash: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// UpdateLanguage writes language and now as language and updated_at of the
// row with id.
func UpdateLanguage(ctx context.Context, q Querier, id uuid.UUID, language string, now time.Time) error {
	tag, err := q.Exec(ctx, `UPDATE users SET language = $2, updated_at = $3 WHERE id = $1`, id, language, now)
	if err != nil {
		return fmt.Errorf("update language: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountActiveAdministrators counts users with role administrator and active.
func CountActiveAdministrators(ctx context.Context, q Querier) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'administrator' AND active`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count active administrators: %w", err)
	}
	return n, nil
}

// LockUsers locks the table IN SHARE ROW EXCLUSIVE MODE for the
// transaction, so that no other transaction changes a user meanwhile.
func LockUsers(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock users: %w", err)
	}
	return nil
}

// LockUsersAndCheckEmpty locks the table as LockUsers does and reports
// whether it holds no row at all.
func LockUsersAndCheckEmpty(ctx context.Context, tx pgx.Tx) (bool, error) {
	if err := LockUsers(ctx, tx); err != nil {
		return false, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false, fmt.Errorf("check users empty: %w", err)
	}
	return !exists, nil
}
