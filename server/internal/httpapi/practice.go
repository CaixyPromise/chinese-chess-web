package httpapi

import (
	"chinesechess/internal/game"
	"context"
	"net/http"
	"time"
)

func (a *API) createPractice(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	var body struct {
		SnapshotID      string `json:"snapshotId,omitempty"`
		SourceMode      string `json:"sourceMode"`
		SourceID        string `json:"sourceId"`
		ExpectedVersion int64  `json:"expectedVersion"`
		Ply             *int   `json:"ply"`
		Human           string `json:"human"`
		Difficulty      string `json:"difficulty"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Ply == nil || *body.Ply < 0 {
		a.fail(w, &game.Error{Code: "INVALID_REQUEST"})
		return
	}
	var version int64
	var moves []string
	var rules string
	group := body.SourceMode + ":" + body.SourceID
	switch body.SourceMode {
	case "pve":
		v, err := a.games.Get(owner, body.SourceID)
		if err != nil {
			a.fail(w, err)
			return
		}
		version, moves, rules = v.Version, v.Moves, v.Rules
		if v.Practice != nil && v.Practice.SourceGroupID != "" {
			group = v.Practice.SourceGroupID
		}
	case "pvp":
		if a.rooms == nil {
			a.fail(w, game.ErrNotFound)
			return
		}
		v, err := a.rooms.Get(owner, body.SourceID)
		if err != nil {
			a.fail(w, err)
			return
		}
		if v.Phase != "finished" {
			a.fail(w, &game.Error{Code: "ANALYSIS_NOT_AVAILABLE"})
			return
		}
		version, moves, rules = v.Version, v.Moves, v.Rules
	default:
		a.fail(w, &game.Error{Code: "INVALID_REQUEST"})
		return
	}
	if body.SnapshotID != "" {
		if a.analyses == nil {
			a.fail(w, &game.Error{Code: "INVALID_REQUEST"})
			return
		}
		frozen, err := a.analyses.Frozen(r.Context(), owner, body.SnapshotID)
		if err != nil {
			a.fail(w, err)
			return
		}
		if frozen.Mode != body.SourceMode || frozen.ID != body.SourceID || frozen.Version != body.ExpectedVersion {
			a.fail(w, &game.Error{Code: "INVALID_REQUEST"})
			return
		}
		if frozen.Mode == "pvp" && frozen.Phase != "finished" {
			a.fail(w, &game.Error{Code: "ANALYSIS_NOT_AVAILABLE"})
			return
		}
		version, moves, rules, group = frozen.Version, frozen.Moves, frozen.Rules, frozen.GroupID
	}
	if version != body.ExpectedVersion {
		a.fail(w, &game.Error{Code: "VERSION_CONFLICT"})
		return
	}
	if *body.Ply > len(moves) {
		a.fail(w, &game.Error{Code: "INVALID_REQUEST"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	v, err := a.games.CreatePractice(ctx, owner, body.Human, body.Difficulty, body.SourceMode, body.SourceID, version, moves[:*body.Ply], rules, group)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 201, v)
}
