package httpapi

import (
	"chinesechess/internal/account"
	"context"
	"errors"
	"net/http"
	"time"
)

type AccountRepository interface {
	AccountOverview(context.Context, string) (account.Overview, error)
	AccountStatistics(context.Context, string, string) (account.Statistics, error)
	AccountActivity(context.Context, string, string, string) (account.Activity, error)
}

func (a *API) accountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/account/overview", a.accountOverview)
	mux.HandleFunc("GET /api/account/statistics", a.accountStatistics)
	mux.HandleFunc("GET /api/account/activity", a.accountActivity)
}

func (a *API) accountRead(w http.ResponseWriter, r *http.Request, read func(context.Context, string) (any, error)) {
	owner := a.requireAccount(w, r)
	if owner == "" {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := read(ctx, owner)
	if errors.Is(err, account.ErrInvalidQuery) {
		respond(w, 400, map[string]string{"code": "INVALID_ACCOUNT_QUERY"})
		return
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 200, result)
}

func (a *API) accountOverview(w http.ResponseWriter, r *http.Request) {
	a.accountRead(w, r, func(ctx context.Context, owner string) (any, error) { return a.accounts.AccountOverview(ctx, owner) })
}
func (a *API) accountStatistics(w http.ResponseWriter, r *http.Request) {
	a.accountRead(w, r, func(ctx context.Context, owner string) (any, error) {
		return a.accounts.AccountStatistics(ctx, owner, r.URL.Query().Get("mode"))
	})
}
func (a *API) accountActivity(w http.ResponseWriter, r *http.Request) {
	a.accountRead(w, r, func(ctx context.Context, owner string) (any, error) {
		return a.accounts.AccountActivity(ctx, owner, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	})
}
