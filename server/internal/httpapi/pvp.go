package httpapi

import (
	"chinesechess/internal/pvp"
	"context"
	"net/http"
	"time"
)

func (a *API) pvpRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/rooms", a.createRoom)
	mux.HandleFunc("POST /api/invitations/preview", a.invitationPreview)
	mux.HandleFunc("POST /api/invitations/withdraw", a.withdrawInvitation)
	mux.HandleFunc("POST /api/rooms/join", a.joinRoom)
	mux.HandleFunc("GET /api/rooms/{id}", a.roomView)
	mux.HandleFunc("POST /api/rooms/{id}/commands", a.roomCommand)
	mux.HandleFunc("GET /api/rooms/{id}/socket", a.roomSocket)
}
func (a *API) createRoom(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	var b struct {
		Side     string `json:"side"`
		Nickname string `json:"nickname"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.Nickname == "" {
		if id, ok := identity(r); ok && id.session.User != nil {
			b.Nickname = id.session.User.Nickname
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	v, e := a.rooms.Create(ctx, owner, b.Side, b.Nickname)
	if e != nil {
		a.fail(w, e)
		return
	}
	respond(w, 201, v)
}
func (a *API) joinRoom(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	var b struct {
		Code      string `json:"code"`
		Nickname  string `json:"nickname"`
		RequestID string `json:"requestId"`
	}
	if !decode(w, r, &b) {
		return
	}
	ip := a.clientIP(r)
	v, e := a.rooms.ApplyJoinFrom(owner, b.Code, b.Nickname, b.RequestID, ip)
	if e != nil {
		a.fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) roomView(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	v, e := a.rooms.GetSide(owner, r.PathValue("id"), r.URL.Query().Get("side"))
	if e != nil {
		a.fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) roomCommand(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	var b pvp.Command
	if !decode(w, r, &b) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	v, e := a.rooms.Command(ctx, owner, r.PathValue("id"), b)
	if e != nil {
		a.fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) roomSocket(w http.ResponseWriter, r *http.Request) {
	r, release, closeSocket, ok := a.socketIdentity(w, r)
	if !ok {
		return
	}
	defer closeSocket()
	defer release()
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	initial, updates, unwatch, e := a.rooms.WatchSide(owner, r.PathValue("id"), r.URL.Query().Get("side"))
	if e != nil {
		a.fail(w, e)
		return
	}
	defer unwatch()
	release()
	stream(w, r, a.rooms.Context(), initial, updates)
}
