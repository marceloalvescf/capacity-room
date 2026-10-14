//go:build postgres

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/audit"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

const resetPassword = "a reset password"

func TestResetPasswordSetsTheHash(t *testing.T) {
	f := newUserFixture(t)
	if _, err := ResetPassword(context.Background(), f.pool, "root", resetPassword, base); err != nil {
		t.Fatal(err)
	}
	row := f.user(t, f.root.ID)
	if match, _, _ := VerifyPassword(context.Background(), resetPassword, row.PasswordHash); !match {
		t.Fatal("the new password does not match")
	}
	if match, _, _ := VerifyPassword(context.Background(), adminPassword, row.PasswordHash); match {
		t.Fatal("the old password still matches")
	}
}

func TestResetPasswordRevokesSessions(t *testing.T) {
	f := newUserFixture(t)
	a, b, other := f.addSession(t, f.root), f.addSession(t, f.root), f.addSession(t, f.eve)
	if _, err := ResetPassword(context.Background(), f.pool, "root", resetPassword, base); err != nil {
		t.Fatal(err)
	}
	requireRevoked(t, f.session(t, a), "password_reset")
	requireRevoked(t, f.session(t, b), "password_reset")
	if s := f.session(t, other); s.RevokedAt != nil || s.RevokedReason != nil {
		t.Fatalf("another user's session was revoked: %+v", s)
	}
}

func TestResetPasswordAudits(t *testing.T) {
	f := newUserFixture(t)
	f.addSession(t, f.root)
	f.addSession(t, f.root)
	if _, err := ResetPassword(context.Background(), f.pool, "root", resetPassword, base); err != nil {
		t.Fatal(err)
	}
	events := f.audits(t, audit.ActionUserPasswordReset)
	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	e := events[0]
	if e.ActorID != nil || e.TargetType == nil || *e.TargetType != "user" ||
		e.TargetID == nil || *e.TargetID != f.root.ID.String() || e.Outcome != audit.OutcomeSucceeded {
		t.Fatalf("event = %+v", e)
	}
	requireMetadata(t, e, `{"sessions_revoked":2}`)
}

func TestResetPasswordKeepsRoleAndActive(t *testing.T) {
	f := newUserFixture(t)
	got, err := ResetPassword(context.Background(), f.pool, "ops", resetPassword, base)
	if err != nil {
		t.Fatal(err)
	}
	row := f.user(t, f.ops.ID)
	if row.Role != RoleOperator || !row.Active || got.Role != RoleOperator || got.ID != f.ops.ID {
		t.Fatalf("row = %+v, got = %+v", row, got)
	}
}

func TestResetPasswordRefusesAnUnknownUser(t *testing.T) {
	f := newUserFixture(t)
	_, err := ResetPassword(context.Background(), f.pool, "nobody", resetPassword, base)
	if !errors.Is(err, ErrNoActiveUser) {
		t.Fatalf("err = %v", err)
	}
	f.requireNoAudit(t)
}

func TestResetPasswordRefusesAnInactiveUser(t *testing.T) {
	f := newUserFixture(t)
	if err := store.UpdateUserFields(context.Background(), f.pool, f.eve.ID, f.eve.Role, f.eve.Language, false, base); err != nil {
		t.Fatal(err)
	}
	_, err := ResetPassword(context.Background(), f.pool, "eve", resetPassword, base)
	if !errors.Is(err, ErrNoActiveUser) {
		t.Fatalf("err = %v", err)
	}
	if got := f.user(t, f.eve.ID).PasswordHash; got != f.eve.PasswordHash {
		t.Fatal("the hash changed")
	}
	f.requireNoAudit(t)
}
