package api

import (
	"net/http"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
)

// patchAccount changes the language or the password of the signed-in
// reader. A new password revokes every session, so the cookie is cleared.
func (s *Server) patchAccount(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	var body AccountChange
	limits := map[string]int{"password": maxPassword, "current_password": maxPassword, "language": 8}
	if p := DecodeBody(r, &body, nil, limits); p != nil {
		WriteProblem(w, p)
		return
	}
	user, err := s.Account.Change(r.Context(), a.Principal, ClientAddress(r, s.Settings.TrustedProxies), auth.AccountChange{
		CurrentPassword: body.CurrentPassword,
		Password:        body.Password,
		Language:        (*string)(body.Language),
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if body.Password != nil {
		http.SetCookie(w, sessionCookie(SessionCookie, "", -1))
	}
	writeJSON(w, http.StatusOK, Principal{Id: user.ID, Username: user.Username, Role: Role(user.Role), Language: Language(user.Language)})
}
