package integration

import (
	"bytes"
	"chinesechess/internal/analysis"
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"chinesechess/internal/httpapi"
	"chinesechess/internal/pvp"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestReplayHTTPFrozenFrameAfterUndo(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	root, _ := filepath.Abs("../..")
	binary := filepath.Join(root, "build/engine/xiangqi-engine")
	rules := engine.NewPool(binary, 2, 8)
	bots := engine.NewPool(binary, 1, 0)
	teacher := engine.NewPool(binary, 1, 4)
	t.Cleanup(rules.Close)
	t.Cleanup(bots.Close)
	t.Cleanup(teacher.Close)
	games, err := game.New(repo, rules, bots, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(games.Close)
	rooms, err := pvp.New(repo, rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rooms.Close)
	analyses, err := analysis.New(repo, teacher, rules, httpapi.AnalysisLoader(games, rooms))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(analyses.Close)
	server := httptest.NewServer(httpapi.NewWithAnalysis(games, rooms, analyses, root, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(server.Close)
	owner, outsider := game.ID(), game.ID()
	call := func(who, method, path string, body any, status int, code string) []byte {
		t.Helper()
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		req, e := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(data))
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "chess_guest", Value: who})
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		data, e = io.ReadAll(resp.Body)
		if e != nil || resp.StatusCode != status {
			t.Fatalf("%s %s: status=%d body=%s error=%v", method, path, resp.StatusCode, data, e)
		}
		if code != "" {
			var failure map[string]string
			if json.Unmarshal(data, &failure) != nil || failure["code"] != code {
				t.Fatalf("expected %s: %s", code, data)
			}
		}
		return data
	}
	live, err := games.Create(ctx, owner, "red", "easy")
	if err != nil {
		t.Fatal(err)
	}
	live, err = games.Command(ctx, owner, live.ID, game.Command{RequestID: game.ID(), Version: live.Version, Type: "move", Move: "b0c2"})
	if err != nil || live.Phase != "queued" {
		t.Fatalf("source move: %+v %v", live, err)
	}
	var frame struct {
		SnapshotID string        `json:"snapshotId"`
		Game       game.Snapshot `json:"game"`
	}
	data := call(owner, "GET", "/api/replay/pve/"+live.ID, nil, 200, "")
	if err = json.Unmarshal(data, &frame); err != nil || !game.ValidID(frame.SnapshotID) || !reflect.DeepEqual(frame.Game, live) {
		t.Fatalf("frame: %s %v", data, err)
	}
	call(outsider, "GET", "/api/replay/pve/"+live.ID, nil, 404, "GAME_NOT_FOUND")
	current, err := games.Command(ctx, owner, live.ID, game.Command{RequestID: game.ID(), Version: live.Version, Type: "undo"})
	if err != nil || current.Version <= live.Version || len(current.Moves) != 0 {
		t.Fatalf("undo: %+v %v", current, err)
	}
	analysisRequest := func(id string, version int64, snapshot string) map[string]any {
		request := map[string]any{"sourceMode": "pve", "sourceId": id, "expectedVersion": version, "scope": "position", "profile": "standard", "ply": 1}
		if snapshot != "" {
			request["snapshotId"] = snapshot
		}
		return request
	}
	practiceRequest := func(id string, version int64, snapshot string) map[string]any {
		request := map[string]any{"sourceMode": "pve", "sourceId": id, "expectedVersion": version, "ply": 1, "human": "black", "difficulty": "easy"}
		if snapshot != "" {
			request["snapshotId"] = snapshot
		}
		return request
	}
	call(owner, "POST", "/api/analyses", analysisRequest(live.ID, live.Version, ""), 409, "VERSION_CONFLICT")
	call(owner, "POST", "/api/games/practice", practiceRequest(live.ID, live.Version, ""), 409, "VERSION_CONFLICT")
	var job analysis.Job
	data = call(owner, "POST", "/api/analyses", analysisRequest(live.ID, live.Version, frame.SnapshotID), 202, "")
	if err = json.Unmarshal(data, &job); err != nil || job.Source.ID != live.ID || job.Source.Version != live.Version || job.Source.Fen != live.Fen || !reflect.DeepEqual(job.Source.Moves, live.Moves) {
		t.Fatalf("frozen analysis: %s %v", data, err)
	}
	var practice game.Snapshot
	data = call(owner, "POST", "/api/games/practice", practiceRequest(live.ID, live.Version, frame.SnapshotID), 201, "")
	if err = json.Unmarshal(data, &practice); err != nil || practice.ID == live.ID || practice.Practice == nil || practice.Practice.SourceVersion != live.Version || practice.Practice.Ply != 1 || practice.Fen != live.Fen || practice.Rules != live.Rules || !reflect.DeepEqual(practice.Moves, live.Moves) {
		t.Fatalf("frozen practice: %s %v", data, err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		data = call(owner, "GET", "/api/analyses/"+job.ID, nil, 200, "")
		if err = json.Unmarshal(data, &job); err != nil {
			t.Fatal(err)
		}
		if job.State == "failed" {
			t.Fatalf("native frozen analysis failed: %s", data)
		}
		if job.State == "completed" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if job.State != "completed" {
		t.Fatalf("frozen analysis timeout: %+v", job)
	}
	var page analysis.Page
	data = call(owner, "GET", "/api/analyses/"+job.ID+"/positions", nil, 200, "")
	if err = json.Unmarshal(data, &page); err != nil || len(page.Items) != 1 || page.Items[0].Ply != 1 || page.Items[0].Fen != live.Fen || page.Items[0].SourceVersion != live.Version || !reflect.DeepEqual(page.Items[0].History, live.Moves) || page.Items[0].Rules != chess.CasualRules {
		t.Fatalf("frozen native result: %s %v", data, err)
	}
	other, err := games.Create(ctx, owner, "red", "easy")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := games.Create(ctx, outsider, "red", "easy")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/analyses", "/api/games/practice"} {
		request := analysisRequest
		if path == "/api/games/practice" {
			request = practiceRequest
		}
		call(owner, "POST", path, request(other.ID, other.Version, frame.SnapshotID), 400, "INVALID_REQUEST")
		call(owner, "POST", path, request(live.ID, current.Version, frame.SnapshotID), 400, "INVALID_REQUEST")
		call(outsider, "POST", path, request(foreign.ID, foreign.Version, frame.SnapshotID), 404, "GAME_NOT_FOUND")
	}
	room, err := rooms.Create(ctx, owner, "red")
	if err != nil {
		t.Fatal(err)
	}
	var pvpFrame struct {
		SnapshotID string   `json:"snapshotId"`
		Game       pvp.View `json:"game"`
	}
	data = call(owner, "GET", "/api/replay/pvp/"+room.ID, nil, 200, "")
	if err = json.Unmarshal(data, &pvpFrame); err != nil || !game.ValidID(pvpFrame.SnapshotID) || pvpFrame.Game.ID != room.ID || pvpFrame.Game.Version != room.Version {
		t.Fatalf("pvp frame: %s %v", data, err)
	}
	call(owner, "POST", "/api/analyses", map[string]any{"sourceMode": "pvp", "sourceId": room.ID, "expectedVersion": room.Version, "snapshotId": pvpFrame.SnapshotID, "scope": "position", "profile": "standard", "ply": 0}, 400, "ANALYSIS_NOT_AVAILABLE")
	call(owner, "POST", "/api/games/practice", map[string]any{"sourceMode": "pvp", "sourceId": room.ID, "expectedVersion": room.Version, "snapshotId": pvpFrame.SnapshotID, "ply": 0, "human": "red", "difficulty": "easy"}, 400, "ANALYSIS_NOT_AVAILABLE")
	unchanged, err := games.Get(owner, live.ID)
	if err != nil || !reflect.DeepEqual(unchanged, current) {
		t.Fatalf("replay actions changed source: %+v %v", unchanged, err)
	}
}
