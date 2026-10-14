//go:build postgres

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
)

const usersPath = "/api/v1/admin/users"

// send sends a mutating request with the session and the CSRF token.
func (ts *testServer) send(t *testing.T, method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	return ts.call(t, method, path, body, map[string]string{CSRFHeaderName: csrf}, cookie)
}

func decodeUser(t *testing.T, w *httptest.ResponseRecorder) User {
	t.Helper()
	var u User
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return u
}

func TestUsersListAsAdministrator(t *testing.T) {
	ts := newTestServer(t)
	cookie, _ := ts.signIn(t, "root")
	w := ts.call(t, "GET", usersPath, "", nil, cookie)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "password") {
		t.Fatalf("body holds a password key: %s", w.Body.String())
	}
	var list UserList
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Users) != 2 || list.Users[0].Username != "eve" || list.Users[1].Username != "root" {
		t.Fatalf("users %+v", list.Users)
	}
	if list.Users[1].Role != "administrator" || !list.Users[1].Active || list.Users[1].CreatedAt.Location().String() != "UTC" {
		t.Fatalf("root %+v", list.Users[1])
	}
}

func TestUsersListAsReader(t *testing.T) {
	ts := newTestServer(t)
	cookie, _ := ts.signIn(t, "eve")
	wantProblem(t, ts.call(t, "GET", usersPath, "", nil, cookie), 403, CodeForbidden, nil)
}

func TestUsersListWithoutSession(t *testing.T) {
	ts := newTestServer(t)
	wantProblem(t, ts.call(t, "GET", usersPath, "", nil), 401, CodeUnauthenticated, nil)
}

func TestUsersCreate(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	w := ts.send(t, "POST", usersPath, `{"username":"kim","password":"kim-password-1","role":"operator","language":"en"}`, cookie, csrf)
	if w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	created := decodeUser(t, w)
	if created.Username != "kim" || created.Role != "operator" || created.Language != "en" || !created.Active || created.Id == uuid.Nil {
		t.Fatalf("created %+v", created)
	}
	if strings.Contains(w.Body.String(), "password") {
		t.Fatalf("body holds a password key: %s", w.Body.String())
	}
	got := ts.call(t, "GET", usersPath+"/"+created.Id.String(), "", nil, cookie)
	if got.Code != 200 || decodeUser(t, got) != created {
		t.Fatalf("get: %d %s", got.Code, got.Body.String())
	}
}

func TestUsersCreateRefusals(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	cases := []struct {
		name   string
		body   string
		status int
		code   Code
		fields map[string]string
	}{
		{"missing role", `{"username":"a","password":"a-password-12","language":"en"}`, 400, CodeRequestInvalid, map[string]string{"role": "missing"}},
		{"role boss", `{"username":"a","password":"a-password-12","role":"boss","language":"en"}`, 400, CodeRequestInvalid, map[string]string{"role": "invalid"}},
		{"short password", `{"username":"a","password":"short","role":"reader","language":"en"}`, 422, CodePasswordTooShort, nil},
		{"name .x", `{"username":".x","password":"a-password-12","role":"reader","language":"en"}`, 422, CodeInvalidUsername, nil},
		{"name taken", `{"username":"eve","password":"a-password-12","role":"reader","language":"en"}`, 409, CodeUsernameTaken, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantProblem(t, ts.send(t, "POST", usersPath, c.body, cookie, csrf), c.status, c.code, c.fields)
		})
	}
}

func TestUsersCreateRequiresCSRF(t *testing.T) {
	ts := newTestServer(t)
	cookie, _ := ts.signIn(t, "root")
	w := ts.call(t, "POST", usersPath, `{"username":"kim","password":"kim-password-1","role":"operator","language":"en"}`, nil, cookie)
	wantProblem(t, w, 403, CodeCSRFRejected, nil)
	if n := ts.count(t, "SELECT count(*) FROM users WHERE username = 'kim'"); n != 0 {
		t.Fatalf("%d users created", n)
	}
}

func TestUsersPatchRole(t *testing.T) {
	ts := newTestServer(t)
	eveCookie, _ := ts.signIn(t, "eve")
	cookie, csrf := ts.signIn(t, "root")
	w := ts.send(t, "PATCH", usersPath+"/"+ts.eve.ID.String(), `{"role":"operator"}`, cookie, csrf)
	if w.Code != 200 || decodeUser(t, w).Role != "operator" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	wantProblem(t, ts.call(t, "GET", usersPath, "", nil, eveCookie), 401, CodeUnauthenticated, nil)
}

func TestUsersPatchSelfDemote(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	id := ts.userID(t, "root")
	wantProblem(t, ts.send(t, "PATCH", usersPath+"/"+id, `{"role":"reader"}`, cookie, csrf), 409, CodeSelfDemote, nil)
}

func TestUsersPatchLastAdmin(t *testing.T) {
	ts := newTestServer(t)
	two := seed(t, ts.pool, ts.now, "two", rootPassword, auth.RoleAdministrator)
	if _, err := ts.pool.Exec(context.Background(), "UPDATE users SET active = false WHERE id = $1", two.ID); err != nil {
		t.Fatal(err)
	}
	cookie, csrf := ts.signIn(t, "root")
	w := ts.send(t, "PATCH", usersPath+"/"+two.ID.String(), `{"active":false}`, cookie, csrf)
	if w.Code != 200 || decodeUser(t, w).Active {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	wantProblem(t, ts.send(t, "PATCH", usersPath+"/"+ts.userID(t, "root"), `{"role":"reader"}`, cookie, csrf), 409, CodeSelfDemote, nil)
}

func TestUsersPatchLastAdminByAnother(t *testing.T) {
	ts := newTestServer(t)
	seed(t, ts.pool, ts.now, "two", rootPassword, auth.RoleAdministrator)
	rootCookie, _ := ts.signIn(t, "root")
	cookie, csrf := ts.signIn(t, "two")
	w := ts.send(t, "PATCH", usersPath+"/"+ts.userID(t, "root"), `{"role":"reader"}`, cookie, csrf)
	if w.Code != 200 || decodeUser(t, w).Role != "reader" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	wantProblem(t, ts.call(t, "GET", usersPath, "", nil, rootCookie), 401, CodeUnauthenticated, nil)
}

func TestUsersDelete(t *testing.T) {
	ts := newTestServer(t)
	eveCookie, _ := ts.signIn(t, "eve")
	cookie, csrf := ts.signIn(t, "root")
	w := ts.send(t, "DELETE", usersPath+"/"+ts.eve.ID.String(), "", cookie, csrf)
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	got := ts.call(t, "GET", usersPath+"/"+ts.eve.ID.String(), "", nil, cookie)
	if got.Code != 200 || decodeUser(t, got).Active {
		t.Fatalf("eve: %d %s", got.Code, got.Body.String())
	}
	wantProblem(t, ts.call(t, "GET", usersPath, "", nil, eveCookie), 401, CodeUnauthenticated, nil)
}

func TestUsersDeleteSelf(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	wantProblem(t, ts.send(t, "DELETE", usersPath+"/"+ts.userID(t, "root"), "", cookie, csrf), 409, CodeSelfDelete, nil)
}

func TestUsersUnknownID(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		path := usersPath + "/" + id
		wantProblem(t, ts.call(t, "GET", path, "", nil, cookie), 404, CodeUserNotFound, nil)
		wantProblem(t, ts.send(t, "PATCH", path, `{"language":"en"}`, cookie, csrf), 404, CodeUserNotFound, nil)
		wantProblem(t, ts.send(t, "DELETE", path, "", cookie, csrf), 404, CodeUserNotFound, nil)
	}
}

func TestUsersPatchNoOp(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	before := ts.count(t, "SELECT count(*) FROM audit_events")
	w := ts.send(t, "PATCH", usersPath+"/"+ts.eve.ID.String(), `{}`, cookie, csrf)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if u := decodeUser(t, w); u.Role != "reader" || u.Language != "pt-BR" || !u.Active {
		t.Fatalf("user %+v", u)
	}
	if after := ts.count(t, "SELECT count(*) FROM audit_events"); after != before {
		t.Fatalf("audit rows %d, want %d", after, before)
	}
}

func (ts *testServer) userID(t *testing.T, name string) string {
	t.Helper()
	var id uuid.UUID
	if err := ts.pool.QueryRow(context.Background(), "SELECT id FROM users WHERE username = $1", name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id.String()
}

func TestUsersPatchJudgesBodyBeforeID(t *testing.T) {
	ts := newTestServer(t)
	cookie, csrf := ts.signIn(t, "root")
	path := usersPath + "/not-a-uuid"
	wantProblem(t, ts.send(t, "PATCH", path, `{"nope":1}`, cookie, csrf), 400, CodeRequestInvalid, map[string]string{"nope": "unknown"})
	wantProblem(t, ts.send(t, "PATCH", path, `{"language":"en"}`, cookie, csrf), 404, CodeUserNotFound, nil)
}
