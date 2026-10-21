package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

var userLimits = map[string]int{"username": maxUsername, "password": maxPassword, "role": 32, "language": 8}

func userOf(u store.User) User {
	return User{
		Id: u.ID, Username: u.Username, Role: Role(u.Role), Language: Language(u.Language), LanguageFollowsBrowser: u.LanguageFollowsBrowser, Active: u.Active,
		CreatedAt: u.CreatedAt.UTC(), UpdatedAt: u.UpdatedAt.UTC(),
	}
}

// userID reads the id of the path and answers 404 USER_NOT_FOUND when it is
// not a uuid.
func userID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		WriteProblem(w, &Problem{Code: CodeUserNotFound})
		return uuid.UUID{}, false
	}
	return id, true
}

// adminOnly wraps a handler that needs an administrator.
func (s *Server) adminOnly(next SessionHandler) http.HandlerFunc {
	return s.requireRole([]auth.Role{auth.RoleAdministrator}, next)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	users, err := s.Users.List(r.Context(), a.Principal)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	list := UserList{Users: make([]User, 0, len(users))}
	for _, u := range users {
		list.Users = append(list.Users, userOf(u))
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	var body NewUser
	if p := DecodeBody(r, &body, []string{"username", "password", "role"}, userLimits); p != nil {
		WriteProblem(w, p)
		return
	}
	language := ""
	if body.Language != nil {
		if language = string(*body.Language); language == "" {
			WriteProblem(w, &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"language": "invalid"}})
			return
		}
	}
	user, err := s.Users.Create(r.Context(), a.Principal, auth.NewUser{
		Username: body.Username, Password: body.Password, Role: string(body.Role), Language: language,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, userOf(user))
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	user, err := s.Users.Get(r.Context(), a.Principal, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, userOf(user))
}

func (s *Server) patchUser(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	var body UserChange
	if p := DecodeBody(r, &body, nil, userLimits); p != nil {
		WriteProblem(w, p)
		return
	}
	id, ok := userID(w, r)
	if !ok {
		return
	}
	if p := languageFlagProblem(body.Language != nil, body.LanguageFollowsBrowser); p != nil {
		WriteProblem(w, p)
		return
	}
	user, err := s.Users.Update(r.Context(), a.Principal, id, auth.UserChange{
		Role: (*string)(body.Role), Active: body.Active, Password: body.Password, Language: (*string)(body.Language),
		LanguageFollowsBrowser: body.LanguageFollowsBrowser,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, userOf(user))
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	if err := s.Users.Delete(r.Context(), a.Principal, id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
