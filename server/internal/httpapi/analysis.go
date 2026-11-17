package httpapi

import (
	"chinesechess/internal/analysis"
	"chinesechess/internal/game"
	"chinesechess/internal/pvp"
	"context"
	"net/http"
	"strconv"
	"time"
)

func AnalysisLoader(games *game.Manager, rooms *pvp.Manager) analysis.Loader {
	return func(ctx context.Context, owner, mode, id string) (analysis.Source, error) {
		if err := ctx.Err(); err != nil {
			return analysis.Source{}, err
		}
		switch mode {
		case "pve":
			v, err := games.Get(owner, id)
			if err != nil {
				return analysis.Source{}, err
			}
			return gameSource(v), nil
		case "pvp":
			if rooms == nil {
				return analysis.Source{}, game.ErrNotFound
			}
			v, err := rooms.Get(owner, id)
			if err != nil {
				return analysis.Source{}, err
			}
			return roomSource(v), nil
		default:
			return analysis.Source{}, &game.Error{Code: "INVALID_REQUEST"}
		}
	}
}

func (a *API) analysisRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/analyses", a.createAnalysis)
	mux.HandleFunc("GET /api/analyses", a.listAnalyses)
	mux.HandleFunc("GET /api/analyses/{id}", a.analysisView)
	mux.HandleFunc("GET /api/analyses/{id}/positions", a.analysisPositions)
	mux.HandleFunc("POST /api/analyses/{id}/cancel", a.analysisControl)
	mux.HandleFunc("POST /api/analyses/{id}/resume", a.analysisControl)
}

func (a *API) createAnalysis(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	var request analysis.Request
	if !decode(w, r, &request) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	j, err := a.analyses.Create(ctx, owner, request)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, http.StatusAccepted, j)
}

func (a *API) listAnalyses(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	q := r.URL.Query()
	version, err := strconv.ParseInt(q.Get("sourceVersion"), 10, 64)
	if err != nil {
		a.fail(w, &game.Error{Code: "INVALID_REQUEST"})
		return
	}
	jobs, err := a.analyses.List(r.Context(), owner, q.Get("sourceMode"), q.Get("sourceId"), version)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 200, jobs)
}

func (a *API) analysisView(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	j, err := a.analyses.Get(r.Context(), owner, r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 200, j)
}

func (a *API) analysisPositions(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	cursor, limit := 0, 100
	var err error
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		cursor, err = strconv.Atoi(raw)
	}
	if raw := r.URL.Query().Get("limit"); raw != "" && err == nil {
		limit, err = strconv.Atoi(raw)
	}
	if err != nil {
		a.fail(w, &game.Error{Code: "INVALID_REQUEST"})
		return
	}
	page, err := a.analyses.Positions(r.Context(), owner, r.PathValue("id"), cursor, limit)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 200, page)
}

func (a *API) analysisControl(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	action := "cancel"
	if r.URL.Path[len(r.URL.Path)-6:] == "resume" {
		action = "resume"
	}
	j, err := a.analyses.Control(r.Context(), owner, r.PathValue("id"), action)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 200, j)
}
