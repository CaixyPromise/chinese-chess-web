package httpapi

import (
	"chinesechess/internal/auth"
	"chinesechess/internal/game"
	"net/http"
)

const cookieName = "chess_guest"

func owner(r *http.Request) string {
	if id, ok := identity(r); ok {
		return id.session.Owner
	}
	c, err := r.Cookie(cookieName)
	if err != nil || !game.ValidID(c.Value) {
		return ""
	}
	return c.Value
}
func (a *API) session(w http.ResponseWriter, r *http.Request) {
	id := owner(r)
	if id == "" {
		id = game.ID()
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: id, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: 60 * 60 * 24 * 30})
	}
	recent, err := a.games.Recent(r.Context(), id)
	if err != nil {
		a.fail(w, err)
		return
	}
	body := map[string]any{"profiles": game.Profiles, "recent": recent}
	if id, ok := identity(r); ok {
		state := map[string]any{"kind": "guest", "key": auth.Hash("identity:" + id.session.Owner)}
		if id.session.User != nil {
			state["kind"] = "account"
			state["user"] = id.session.User
		}
		body["identity"] = state
		body["csrfToken"] = id.session.CSRF()
		if id.session.ReturnTo != "" {
			body["returnTo"] = id.session.ReturnTo
		}
	}
	if a.rooms != nil {
		rooms, err := a.rooms.Recent(r.Context(), id)
		if err != nil {
			a.fail(w, err)
			return
		}
		body["rooms"] = rooms
	}
	respond(w, 200, body)
}
func requireOwner(w http.ResponseWriter, r *http.Request) string {
	id := owner(r)
	if id == "" {
		respond(w, 401, map[string]string{"code": "SESSION_REQUIRED"})
	}
	return id
}
