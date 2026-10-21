//go:build postgres

package api

import (
	"testing"
)

const countAudits = "SELECT count(*) FROM audit_events"

func TestAccountLanguagePinsAndUnpins(t *testing.T) {
	ts := newTestServer(t)
	session, csrf := ts.signIn(t, "eve")
	headers := map[string]string{CSRFHeaderName: csrf}

	w := ts.call(t, "PATCH", "/api/v1/account", `{"language":"en"}`, headers, session)
	if p := decodeStrict[Principal](t, w); w.Code != 200 || p.Language != "en" || p.LanguageFollowsBrowser {
		t.Fatalf("pin: %d %s", w.Code, w.Body.String())
	}
	w = ts.call(t, "PATCH", "/api/v1/account", `{"language_follows_browser":true}`, headers, session)
	if p := decodeStrict[Principal](t, w); w.Code != 200 || p.Language != "en" || !p.LanguageFollowsBrowser {
		t.Fatalf("unpin: %d %s", w.Code, w.Body.String())
	}
	if n := ts.count(t, `SELECT count(*) FROM audit_events WHERE action = 'account.language_changed' AND metadata = '{"language_follows_browser": true}'`); n != 1 {
		t.Fatalf("unpin audit rows = %d, want 1", n)
	}
	w = ts.call(t, "PATCH", "/api/v1/account", `{"language":"pt-BR"}`, headers, session)
	if p := decodeStrict[Principal](t, w); p.Language != "pt-BR" || p.LanguageFollowsBrowser {
		t.Fatalf("pin again: %s", w.Body.String())
	}
}

func TestAccountLanguageFlagRefusals(t *testing.T) {
	ts := newTestServer(t)
	session, csrf := ts.signIn(t, "eve")
	headers := map[string]string{CSRFHeaderName: csrf}
	before := ts.count(t, countAudits)
	for body, code := range map[string]string{
		`{"language_follows_browser":false}`:                "invalid",
		`{"language":"en","language_follows_browser":true}`: "conflict",
		`{"language_follows_browser":"yes"}`:                "invalid",
	} {
		w := ts.call(t, "PATCH", "/api/v1/account", body, headers, session)
		wantProblem(t, w, 400, CodeRequestInvalid, map[string]string{"language_follows_browser": code})
	}
	if after := ts.count(t, countAudits); after != before {
		t.Fatalf("audit rows %d -> %d, want none written", before, after)
	}
	if n := ts.count(t, "SELECT count(*) FROM users WHERE username = 'eve' AND language = 'pt-BR' AND NOT language_follows_browser"); n != 1 {
		t.Fatal("a refused body changed the row")
	}
}

func TestUserLanguagePinsAndUnpins(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	path := usersPath + "/" + ts.eve.ID.String()

	u := decodeUser(t, ts.send(t, "PATCH", path, `{"language":"en"}`, cookie, csrf))
	if u.Language != "en" || u.LanguageFollowsBrowser {
		t.Fatalf("pin: %+v", u)
	}
	u = decodeUser(t, ts.send(t, "PATCH", path, `{"language_follows_browser":true}`, cookie, csrf))
	if u.Language != "en" || !u.LanguageFollowsBrowser {
		t.Fatalf("unpin: %+v", u)
	}
	if n := ts.count(t, `SELECT count(*) FROM audit_events WHERE action = 'user.update' AND metadata -> 'changed' = '{"language_follows_browser": true}'`); n != 1 {
		t.Fatalf("unpin audit rows = %d, want 1", n)
	}

	before := ts.count(t, countAudits)
	wantProblem(t, ts.send(t, "PATCH", path, `{"language_follows_browser":false}`, cookie, csrf), 400, CodeRequestInvalid, map[string]string{"language_follows_browser": "invalid"})
	wantProblem(t, ts.send(t, "PATCH", path, `{"language":"pt-BR","language_follows_browser":true}`, cookie, csrf), 400, CodeRequestInvalid, map[string]string{"language_follows_browser": "conflict"})
	if after := ts.count(t, countAudits); after != before {
		t.Fatalf("audit rows %d -> %d, want none written", before, after)
	}
	if u := decodeUser(t, ts.send(t, "GET", path, "", cookie, csrf)); u.Language != "en" || !u.LanguageFollowsBrowser {
		t.Fatalf("a refused body changed the row: %+v", u)
	}
}

func TestCreateUserWithoutLanguageFollowsTheBrowser(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	u := decodeUser(t, ts.send(t, "POST", usersPath, `{"username":"nina","password":"a-long-enough-password","role":"reader"}`, cookie, csrf))
	if u.Language != "en" || !u.LanguageFollowsBrowser {
		t.Fatalf("no language: %+v", u)
	}
	u = decodeUser(t, ts.send(t, "POST", usersPath, `{"username":"omar","password":"a-long-enough-password","role":"reader","language":"pt-BR"}`, cookie, csrf))
	if u.Language != "pt-BR" || u.LanguageFollowsBrowser {
		t.Fatalf("language given: %+v", u)
	}
	wantProblem(t, ts.send(t, "POST", usersPath, `{"username":"paul","password":"a-long-enough-password","role":"reader","language":""}`, cookie, csrf), 400, CodeRequestInvalid, map[string]string{"language": "invalid"})
}
