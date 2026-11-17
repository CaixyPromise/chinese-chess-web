package httpapi

import (
	"chinesechess/internal/analysis"
	"chinesechess/internal/game"
	"chinesechess/internal/pvp"
	"context"
	"net/http"
	"time"
)

func gameSource(v game.Snapshot) analysis.Source {
	s := analysis.Source{Mode: "pve", ID: v.ID, Version: v.Version, Fen: v.Fen, Moves: v.Moves, Rules: v.Rules, Phase: v.Phase, Outcome: v.Outcome, Reason: v.Reason, GroupID: "pve:" + v.ID, Origin: "game"}
	if v.Practice != nil {
		s.Origin = "practice"
		s.GroupID = v.Practice.SourceGroupID
		if s.GroupID == "" {
			s.GroupID = v.Practice.SourceMode + ":" + v.Practice.SourceID
		}
	}
	if s.Moves == nil {
		s.Moves = []string{}
	}
	return s
}

func roomSource(v pvp.View) analysis.Source {
	s := analysis.Source{Mode: "pvp", ID: v.ID, Version: v.Version, Fen: v.Fen, Moves: v.Moves, Rules: v.Rules, Phase: v.Phase, Outcome: v.Outcome, Reason: v.Reason, GroupID: "pvp:" + v.ID, Origin: "game"}
	if s.Moves == nil {
		s.Moves = []string{}
	}
	return s
}

// Capture once: the persisted source and displayed game always describe the
// same committed version even if a robot moves while storage is completing.
func (a *API) replaySnapshot(w http.ResponseWriter, r *http.Request) {
	owner := requireOwner(w, r)
	if owner == "" {
		return
	}
	var displayed any
	var source analysis.Source
	switch r.PathValue("mode") {
	case "pve":
		v, err := a.games.Get(owner, r.PathValue("id"))
		if err != nil {
			a.fail(w, err)
			return
		}
		displayed, source = v, gameSource(v)
	case "pvp":
		if a.rooms == nil {
			a.fail(w, game.ErrNotFound)
			return
		}
		v, err := a.rooms.GetSide(owner, r.PathValue("id"), r.URL.Query().Get("side"))
		if err != nil {
			a.fail(w, err)
			return
		}
		displayed, source = v, roomSource(v)
	default:
		a.fail(w, &game.Error{Code: "INVALID_REQUEST"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	id, err := a.analyses.Freeze(ctx, owner, source)
	if err != nil {
		a.fail(w, err)
		return
	}
	respond(w, 200, map[string]any{"snapshotId": id, "game": displayed})
}
