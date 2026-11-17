package httpapi

import (
	"chinesechess/internal/game"
	"chinesechess/internal/history"
	"context"
	"net/http"
	"strconv"
	"time"
)

func (a *API) history(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	values := r.URL.Query()
	limit := 0
	if raw := values.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 50 {
			a.fail(w, &game.Error{Code: "INVALID_HISTORY_QUERY"})
			return
		}
		limit = n
	}
	q, err := history.ParseQuery(values.Get("mode"), values.Get("cursor"), limit)
	if err != nil {
		a.fail(w, &game.Error{Code: "INVALID_HISTORY_QUERY"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := a.games.History(ctx, owner, q)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, http.StatusOK, page)
}
