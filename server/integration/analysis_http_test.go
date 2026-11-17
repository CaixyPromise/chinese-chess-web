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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAnalysisHTTPAutomaticTrainingAndPracticeLineage(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	root, _ := filepath.Abs("../..")
	binary := filepath.Join(root, "build/engine/xiangqi-engine")
	rules := engine.NewPool(binary, 2, 8)
	bots := engine.NewPool(binary, 1, 0)
	teacher := engine.NewPool(binary, 1, 4)
	t.Cleanup(rules.Close)
	t.Cleanup(bots.Close)
	t.Cleanup(teacher.Close)
	owner, outsider := game.ID(), game.ID()
	moves := []string{"a0a2"}
	position, err := chess.Replay(ctx, rules, moves, "")
	if err != nil {
		t.Fatal(err)
	}
	source := game.Record{Snapshot: game.Snapshot{ID: game.ID(), Version: 2, Fen: position.Fen, Moves: moves, Legal: []string{}, Human: "red", Difficulty: "easy", Rules: chess.CasualRules, Phase: "finished", Outcome: "black_win", Reason: "resign", InCheck: position.InCheck}, Owner: owner, Receipts: map[string]game.Receipt{}}
	if err = repo.Save(ctx, source, 0); err != nil {
		t.Fatal(err)
	}
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
	call := func(who, method, path string, body any, status int, code string) []byte {
		t.Helper()
		var payload []byte
		if body != nil {
			payload, _ = json.Marshal(body)
		}
		req, e := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(payload))
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
		data, e := io.ReadAll(resp.Body)
		if e != nil || resp.StatusCode != status {
			t.Fatalf("%s %s: status %d body %s err %v", method, path, resp.StatusCode, data, e)
		}
		if code != "" {
			var v map[string]string
			if json.Unmarshal(data, &v) != nil || v["code"] != code {
				t.Fatalf("expected %s: %s", code, data)
			}
		}
		return data
	}
	listPath := fmt.Sprintf("/api/analyses?sourceMode=pve&sourceId=%s&sourceVersion=%d", source.ID, source.Version)
	var auto analysis.Job
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var jobs []analysis.Job
		if err = json.Unmarshal(call(owner, "GET", listPath, nil, 200, ""), &jobs); err != nil {
			t.Fatal(err)
		}
		if len(jobs) > 1 {
			t.Fatalf("duplicate automatic jobs: %+v", jobs)
		}
		if len(jobs) == 1 {
			auto = jobs[0]
			if auto.State == "failed" {
				t.Fatalf("automatic analysis failed: %+v", auto)
			}
			if auto.State == "completed" {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if auto.State != "completed" || !auto.Automatic || auto.Total != 2 || auto.Completed != 2 {
		t.Fatalf("automatic task: %+v", auto)
	}
	manualRequest := map[string]any{"sourceMode": "pve", "sourceId": source.ID, "expectedVersion": source.Version, "scope": "game", "profile": "standard", "ply": 0}
	var manual analysis.Job
	if err = json.Unmarshal(call(owner, "POST", "/api/analyses", manualRequest, 202, ""), &manual); err != nil || manual.ID != auto.ID {
		t.Fatalf("manual did not reuse automatic task: %+v %v", manual, err)
	}
	var jobs []analysis.Job
	if err = json.Unmarshal(call(owner, "GET", listPath, nil, 200, ""), &jobs); err != nil || len(jobs) != 1 {
		t.Fatalf("duplicate jobs: %+v %v", jobs, err)
	}
	var page analysis.Page
	if err = json.Unmarshal(call(owner, "GET", "/api/analyses/"+auto.ID+"/positions", nil, 200, ""), &page); err != nil || len(page.Items) != 2 {
		t.Fatalf("positions: %+v %v", page, err)
	}
	for i, sample := range page.Items {
		want, e := chess.Replay(ctx, rules, moves[:i], "")
		if e != nil {
			t.Fatal(e)
		}
		turn := "red"
		if i%2 == 1 {
			turn = "black"
		}
		if sample.SchemaVersion != 1 || sample.AnalysisVersion == "" || len(sample.SampleID) != 64 || len(sample.PositionID) != 64 || sample.GroupID != "pve:"+source.ID || sample.SourceID != source.ID || sample.SourceVersion != source.Version || sample.SourceMode != "pve" || sample.Origin != "game" || sample.Ply != i || sample.Fen != want.Fen || sample.Turn != turn || !reflect.DeepEqual(sample.History, moves[:i]) || sample.Rules != chess.CasualRules || sample.Teacher != "0.5.2" || sample.Budget != (analysis.Budget{Depth: 6, Millis: 750, HashMiB: 128}) || sample.ScorePerspective != "red" || sample.ScoreUnits != "legacy-native" || !sample.Final || sample.Outcome != "black_win" || sample.EndReason != "resign" {
			t.Fatalf("sample metadata: %+v", sample)
		}
		if !sample.Best.Reliable || sample.Best.Depth < 1 || sample.Best.Depth > 6 || sample.Best.NativePerspective != "side_to_move" || sample.Best.Producer != "search" || sample.Best.Nodes == 0 || sample.Best.CompletedNanos == 0 || len(sample.Candidates) != 1 || sample.Candidates[0].Role != "teacher_recommended" || sample.Candidates[0].Move != sample.Best.Move {
			t.Fatalf("teacher output: %+v", sample)
		}
		wantScore := sample.Best.NativeScore
		if turn == "black" {
			wantScore = -wantScore
		}
		if sample.Best.Score != wantScore {
			t.Fatalf("score perspective: %+v", sample)
		}
		legal := false
		for _, move := range want.Legal {
			if move == sample.Best.Move {
				legal = true
			}
		}
		if !legal {
			t.Fatalf("illegal recommendation: %+v", sample)
		}
		if i == 0 && (sample.Actual == nil || sample.ActualMove != moves[0] || sample.Actual.Move != moves[0] || sample.Best.Move == moves[0]) {
			t.Fatalf("actual and recommendation: %+v", sample)
		}
		if sample.Actual != nil {
			actualScore := sample.Actual.NativeScore
			if turn == "black" {
				actualScore = -actualScore
			}
			if !sample.Actual.Reliable || sample.Actual.Depth != sample.Best.Depth || sample.Actual.NativePerspective != "side_to_move" || sample.Actual.Score != actualScore || len(sample.Actual.PV) == 0 || sample.Actual.PV[0] != sample.ActualMove {
				t.Fatalf("actual evaluation: %+v", sample)
			}
		}
		if i == 1 && (sample.Actual != nil || sample.ActualMove != "") {
			t.Fatalf("unexpected actual after final ply: %+v", sample)
		}
	}
	if page.Items[0].SampleID == page.Items[1].SampleID || page.Items[0].PositionID == page.Items[1].PositionID {
		t.Fatal("distinct positions share IDs")
	}
	call(outsider, "GET", listPath, nil, 404, "GAME_NOT_FOUND")
	call(outsider, "GET", "/api/analyses/"+auto.ID, nil, 404, "GAME_NOT_FOUND")
	call(outsider, "GET", "/api/analyses/"+auto.ID+"/positions", nil, 404, "GAME_NOT_FOUND")
	call(outsider, "POST", "/api/analyses/"+auto.ID+"/cancel", map[string]any{}, 404, "GAME_NOT_FOUND")
	liveRoom, err := rooms.Create(ctx, owner, "red")
	if err != nil {
		t.Fatal(err)
	}
	call(owner, "POST", "/api/analyses", map[string]any{"sourceMode": "pvp", "sourceId": liveRoom.ID, "expectedVersion": liveRoom.Version, "scope": "position", "profile": "standard", "ply": 0}, 400, "ANALYSIS_NOT_AVAILABLE")
	practice := func(id string, version int64) game.Snapshot {
		t.Helper()
		var v game.Snapshot
		data := call(owner, "POST", "/api/games/practice", map[string]any{"sourceMode": "pve", "sourceId": id, "expectedVersion": version, "ply": 0, "human": "red", "difficulty": "easy"}, 201, "")
		if err = json.Unmarshal(data, &v); err != nil || v.Practice == nil || v.Practice.SourceGroupID != "pve:"+source.ID || v.Phase != "playing" {
			t.Fatalf("practice lineage: %s %v", data, err)
		}
		return v
	}
	first := practice(source.ID, source.Version)
	second := practice(first.ID, first.Version)
	if first.ID == second.ID || second.Practice.SourceID != first.ID {
		t.Fatalf("practice identity: %+v %+v", first, second)
	}
	var active analysis.Job
	data := call(owner, "POST", "/api/analyses", map[string]any{"sourceMode": "pve", "sourceId": second.ID, "expectedVersion": second.Version, "scope": "position", "profile": "standard", "ply": 0}, 202, "")
	if err = json.Unmarshal(data, &active); err != nil {
		t.Fatal(err)
	}
	for time.Now().Before(deadline) {
		data = call(owner, "GET", "/api/analyses/"+active.ID, nil, 200, "")
		if err = json.Unmarshal(data, &active); err != nil {
			t.Fatal(err)
		}
		if active.State == "failed" {
			t.Fatalf("active analysis failed: %s", data)
		}
		if active.State == "completed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if active.State != "completed" {
		t.Fatalf("active analysis timed out: %+v", active)
	}
	data = call(owner, "GET", "/api/analyses/"+active.ID+"/positions", nil, 200, "")
	page = analysis.Page{}
	if err = json.Unmarshal(data, &page); err != nil || len(page.Items) != 1 || page.Items[0].Final || page.Items[0].Outcome != "ongoing" || page.Items[0].EndReason != "" || page.Items[0].GroupID != "pve:"+source.ID || page.Items[0].Origin != "practice" {
		t.Fatalf("active sample: %s %v", data, err)
	}
	autoPosition := firstPosition(t, call(owner, "GET", "/api/analyses/"+auto.ID+"/positions", nil, 200, ""))
	if page.Items[0].PositionID == "" || page.Items[0].SampleID == "" || !strings.EqualFold(page.Items[0].PositionID, autoPosition.PositionID) || page.Items[0].SampleID == autoPosition.SampleID {
		t.Fatal("identical prefix must share position identity")
	}
	unchanged, err := repo.FindGame(ctx, owner, source.ID)
	if err != nil || unchanged.Version != source.Version || unchanged.Fen != source.Fen || !reflect.DeepEqual(unchanged.Moves, source.Moves) || unchanged.Outcome != source.Outcome || unchanged.Reason != source.Reason {
		t.Fatalf("source changed: %+v %v", unchanged, err)
	}
}

func firstPosition(t *testing.T, data []byte) analysis.Position {
	t.Helper()
	var page analysis.Page
	if err := json.Unmarshal(data, &page); err != nil || len(page.Items) == 0 {
		t.Fatalf("positions: %s %v", data, err)
	}
	return page.Items[0]
}
