package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
)

func TestRequireRoleWithoutCookie(t *testing.T) {
	s := &Server{}
	h := s.requireRole([]auth.Role{auth.RoleAdministrator}, func(http.ResponseWriter, *http.Request, auth.Authenticated) {
		t.Error("the handler ran without a session")
	})
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", "/api/v1/probe", nil))
	if w.Code != 401 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if c := w.Result().Cookies(); len(c) != 1 || c[0].Name != SessionCookie || c[0].MaxAge != -1 {
		t.Fatalf("Set-Cookie %q", w.Header().Values("Set-Cookie"))
	}
}
