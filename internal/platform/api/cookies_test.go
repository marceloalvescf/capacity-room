package api

import "testing"

func TestSessionCookieShape(t *testing.T) {
	got := sessionCookie("capacityroom_session", "v", 43200).String()
	want := "capacityroom_session=v; Path=/api; Max-Age=43200; HttpOnly; Secure; SameSite=Lax"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSessionCookieWithoutMaxAge(t *testing.T) {
	got := sessionCookie("capacityroom_login_csrf", "v", 0).String()
	want := "capacityroom_login_csrf=v; Path=/api; HttpOnly; Secure; SameSite=Lax"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSessionCookieCleared(t *testing.T) {
	got := sessionCookie("capacityroom_session", "", -1).String()
	want := "capacityroom_session=; Path=/api; Max-Age=0; HttpOnly; Secure; SameSite=Lax"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCookieNames(t *testing.T) {
	if SessionCookie != "capacityroom_session" || LoginCSRFCookie != "capacityroom_login_csrf" {
		t.Fatalf("cookies %q and %q", SessionCookie, LoginCSRFCookie)
	}
}
