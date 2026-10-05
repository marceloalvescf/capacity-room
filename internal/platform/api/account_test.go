//go:build postgres

package api

import (
	"encoding/json"
	"testing"
)

func TestAccountLanguage(t *testing.T) {
	ts := newTestServer(t)
	session, csrf := ts.signIn(t, "eve")
	w := ts.call(t, "PATCH", "/api/v1/account", `{"language":"en"}`, map[string]string{CSRFHeader: csrf}, session)
	var p Principal
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || p.Language != "en" || p.Username != "eve" || p.Role != "reader" || p.ID != ts.eve.ID.String() {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if len(w.Result().Cookies()) != 0 {
		t.Fatalf("Set-Cookie %q", w.Header().Values("Set-Cookie"))
	}
	if n := ts.count(t, "SELECT count(*) FROM users WHERE username = 'eve' AND language = 'en'"); n != 1 {
		t.Fatal("language not stored")
	}
}

func TestAccountPassword(t *testing.T) {
	ts := newTestServer(t)
	session, csrf := ts.signIn(t, "eve")
	body := `{"current_password":"` + evePassword + `","password":"a-brand-new-password"}`
	w := ts.call(t, "PATCH", "/api/v1/account", body, map[string]string{CSRFHeader: csrf}, session)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	cleared := cookieNamed(w, SessionCookie)
	if cleared == nil || cleared.MaxAge != -1 {
		t.Fatalf("Set-Cookie %q", w.Header().Values("Set-Cookie"))
	}
	wantProblem(t, ts.call(t, "POST", "/api/v1/auth/logout", "", map[string]string{CSRFHeader: csrf}, session), 401, CodeUnauthenticated, nil)
	if w := ts.login(t, "eve", "a-brand-new-password"); w.Code != 200 {
		t.Fatalf("sign in with the new password: %d", w.Code)
	}
}

func TestAccountWrongCurrent(t *testing.T) {
	ts := newTestServer(t)
	session, csrf := ts.signIn(t, "eve")
	body := `{"current_password":"not-the-password","password":"a-brand-new-password"}`
	w := ts.call(t, "PATCH", "/api/v1/account", body, map[string]string{CSRFHeader: csrf}, session)
	wantProblem(t, w, 401, CodeInvalidCredentials, nil)
}

func TestAccountShortPassword(t *testing.T) {
	ts := newTestServer(t)
	session, csrf := ts.signIn(t, "eve")
	body := `{"current_password":"` + evePassword + `","password":"abc"}`
	w := ts.call(t, "PATCH", "/api/v1/account", body, map[string]string{CSRFHeader: csrf}, session)
	wantProblem(t, w, 422, CodePasswordTooShort, nil)
}

func TestAccountInvalidLanguage(t *testing.T) {
	ts := newTestServer(t)
	session, csrf := ts.signIn(t, "eve")
	w := ts.call(t, "PATCH", "/api/v1/account", `{"language":"fr"}`, map[string]string{CSRFHeader: csrf}, session)
	wantProblem(t, w, 400, CodeRequestInvalid, map[string]string{"language": "invalid"})
}

func TestAccountRequiresCSRF(t *testing.T) {
	ts := newTestServer(t)
	session, _ := ts.signIn(t, "eve")
	w := ts.call(t, "PATCH", "/api/v1/account", `{"language":"en"}`, nil, session)
	wantProblem(t, w, 403, CodeCSRFRejected, nil)
}
