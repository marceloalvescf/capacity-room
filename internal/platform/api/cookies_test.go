package api

import "testing"

func TestSessionCookieShape(t *testing.T) {
	got := sessionCookie("kcr_session", "v", 43200).String()
	want := "kcr_session=v; Path=/api; Max-Age=43200; HttpOnly; Secure; SameSite=Lax"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSessionCookieWithoutMaxAge(t *testing.T) {
	got := sessionCookie("kcr_login_csrf", "v", 0).String()
	want := "kcr_login_csrf=v; Path=/api; HttpOnly; Secure; SameSite=Lax"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSessionCookieCleared(t *testing.T) {
	got := sessionCookie("kcr_session", "", -1).String()
	want := "kcr_session=; Path=/api; Max-Age=0; HttpOnly; Secure; SameSite=Lax"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
