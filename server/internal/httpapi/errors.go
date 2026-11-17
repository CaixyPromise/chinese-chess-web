package httpapi

import (
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"context"
	"errors"
	"net/http"
)

func (a *API) fail(w http.ResponseWriter, err error) {
	status, code := 503, "UNAVAILABLE"
	var problem *game.Error
	if errors.As(err, &problem) {
		code = problem.Code
		status = 400
		if code == "RATE_LIMITED" {
			status = 429
		}
		if code == "VERSION_CONFLICT" || code == "REQUEST_CONFLICT" {
			status = 409
		}
	}
	if errors.Is(err, game.ErrNotFound) {
		status, code = 404, "GAME_NOT_FOUND"
	}
	if errors.Is(err, engine.ErrBusy) {
		status, code = 429, "BUSY"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		status, code = 504, "TIMEOUT"
	}
	a.log.Warn("request failed", "code", code, "error", err)
	respond(w, status, map[string]string{"code": code})
}
