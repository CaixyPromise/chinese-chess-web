package httpapi

import (
	"chinesechess/internal/game"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		respond(w, 415, map[string]string{"code": "INVALID_REQUEST"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		respond(w, 400, map[string]string{"code": "INVALID_REQUEST"})
		return false
	}
	if d.Decode(&struct{}{}) != io.EOF {
		respond(w, 400, map[string]string{"code": "INVALID_REQUEST"})
		return false
	}
	return true
}
func (a *API) create(w http.ResponseWriter, r *http.Request) {
	id := requireOwner(w, r)
	if id == "" {
		return
	}
	var body struct {
		Human      string `json:"human"`
		Difficulty string `json:"difficulty"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	v, err := a.games.Create(ctx, id, body.Human, body.Difficulty)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 201, v)
}
func (a *API) snapshot(w http.ResponseWriter, r *http.Request) {
	id := requireOwner(w, r)
	if id == "" {
		return
	}
	v, err := a.games.Get(id, r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 200, v)
}
func (a *API) command(w http.ResponseWriter, r *http.Request) {
	id := requireOwner(w, r)
	if id == "" {
		return
	}
	var body game.Command
	if !decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	v, err := a.games.Command(ctx, id, r.PathValue("id"), body)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 200, v)
}
