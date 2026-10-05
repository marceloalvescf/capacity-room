//go:build postgres

package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

type accountFixture struct {
	*userFixture
	account *Account
}

func newAccountFixture(t *testing.T) *accountFixture {
	t.Helper()
	f := newUserFixture(t)
	now := func() time.Time { return base.Add(time.Minute) }
	sessions := &Sessions{
		Pool:     f.pool,
		Throttle: Throttle{Secret: []byte("0123456789abcdef0123456789abcdef")},
		TTL:      12 * time.Hour,
		Idle:     time.Hour,
		Now:      now,
	}
	return &accountFixture{userFixture: f, account: &Account{Pool: f.pool, Sessions: sessions, Now: now}}
}

func (f *accountFixture) failures(t *testing.T, name string) int {
	t.Helper()
	row, err := store.GetThrottle(context.Background(), f.pool, f.account.Sessions.Throttle.Key(name, testAddress))
	if errors.Is(err, store.ErrNotFound) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return row.Failures
}

func TestAccountChangeLanguage(t *testing.T) {
	f := newAccountFixture(t)
	s := f.addSession(t, f.eve)
	got, err := f.account.Change(context.Background(), f.actor(f.eve), testAddress, AccountChange{Language: str("en")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "en" || f.user(t, f.eve.ID).Language != "en" {
		t.Fatalf("row %+v", got)
	}
	if f.session(t, s).RevokedAt != nil {
		t.Fatal("session revoked")
	}
	events := f.audits(t, audit.ActionAccountLanguageChanged)
	if len(events) != 1 || *events[0].ActorID != f.eve.ID || *events[0].TargetType != "user" || *events[0].TargetID != f.eve.ID.String() {
		t.Fatalf("events %+v", events)
	}
	requireMetadata(t, events[0], `{"language":"en"}`)
}

func TestAccountChangePassword(t *testing.T) {
	f := newAccountFixture(t)
	s1, s2 := f.addSession(t, f.eve), f.addSession(t, f.eve)
	got, err := f.account.Change(context.Background(), f.actor(f.eve), testAddress,
		AccountChange{CurrentPassword: str(readerPassword), Password: str(newPassword)})
	if err != nil {
		t.Fatal(err)
	}
	row := f.user(t, f.eve.ID)
	if row.PasswordHash == f.eve.PasswordHash || got.PasswordHash != row.PasswordHash {
		t.Fatal("hash not replaced")
	}
	if match, _ := VerifyPassword(newPassword, row.PasswordHash); !match {
		t.Fatal("new hash does not match")
	}
	requireRevoked(t, f.session(t, s1), "password_changed")
	requireRevoked(t, f.session(t, s2), "password_changed")
	events := f.audits(t, audit.ActionAccountPasswordChanged)
	if len(events) != 1 || *events[0].ActorID != f.eve.ID || *events[0].TargetID != f.eve.ID.String() {
		t.Fatalf("events %+v", events)
	}
	requireMetadata(t, events[0], `{"sessions_revoked":2}`)
}

func TestAccountChangePasswordAndLanguageTogether(t *testing.T) {
	f := newAccountFixture(t)
	_, err := f.account.Change(context.Background(), f.actor(f.eve), testAddress,
		AccountChange{CurrentPassword: str(readerPassword), Password: str(newPassword), Language: str("en")})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.audits(t, audit.ActionAccountLanguageChanged)) != 1 || len(f.audits(t, audit.ActionAccountPasswordChanged)) != 1 {
		t.Fatalf("events %+v", f.audits(t, ""))
	}
	if f.user(t, f.eve.ID).Language != "en" {
		t.Fatal("language not set")
	}
}

func TestAccountWrongCurrentPassword(t *testing.T) {
	f := newAccountFixture(t)
	_, err := f.account.Change(context.Background(), f.actor(f.eve), testAddress,
		AccountChange{CurrentPassword: str("not the password"), Password: str(newPassword)})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err %v", err)
	}
	if f.user(t, f.eve.ID).PasswordHash != f.eve.PasswordHash {
		t.Fatal("hash changed")
	}
	if n := f.failures(t, "eve"); n != 1 {
		t.Fatalf("failures %d", n)
	}
	f.requireNoAudit(t)
}

func TestAccountPasswordWithoutCurrent(t *testing.T) {
	f := newAccountFixture(t)
	_, err := f.account.Change(context.Background(), f.actor(f.eve), testAddress, AccountChange{Password: str(newPassword)})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err %v", err)
	}
	if n := f.failures(t, "eve"); n != 0 {
		t.Fatalf("failures %d", n)
	}
}

func TestAccountChangeValidates(t *testing.T) {
	f := newAccountFixture(t)
	ctx, actor := context.Background(), f.actor(f.eve)
	_, err := f.account.Change(ctx, actor, testAddress, AccountChange{Language: str("fr")})
	if !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("err %v", err)
	}
	_, err = f.account.Change(ctx, actor, testAddress, AccountChange{CurrentPassword: str(readerPassword), Password: str("short")})
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("err %v", err)
	}
	f.requireNoAudit(t)
}

func TestAccountNoChange(t *testing.T) {
	f := newAccountFixture(t)
	got, err := f.account.Change(context.Background(), f.actor(f.eve), testAddress, AccountChange{})
	if err != nil || got.ID != f.eve.ID || !got.UpdatedAt.Equal(base) {
		t.Fatalf("row %+v %v", got, err)
	}
	f.requireNoAudit(t)
}
