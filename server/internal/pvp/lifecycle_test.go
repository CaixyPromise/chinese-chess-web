package pvp

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/game"
	"context"
	"errors"
	"testing"
	"time"
)

func TestPVPArchiveSeparatedFromActiveRoomsAndSeats(t *testing.T) {
	m, clock, repo := setup(t)
	owner, other := game.ID(), game.ID()
	var archived Record
	for i := 0; i < 300; i++ {
		archived = Record{State: State{ID: game.ID(), Version: 1, Fen: chess.Initial, Phase: "finished", Outcome: "draw", Reason: "agreement", Moves: []string{}, Legal: []string{}}, Host: owner, RedOwner: owner, BlackOwner: other, Invite: game.ID(), Receipts: map[string]Receipt{}}
		if err := repo.SavePVP(context.Background(), archived, 0); err != nil {
			t.Fatal(err)
		}
	}
	recovered, err := New(repo, m.rules, clock.Load)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(recovered.Close)
	if len(recovered.roomList()) != 0 {
		t.Fatal("history eagerly loaded")
	}
	if _, err = recovered.Get(game.ID(), archived.ID); !errors.Is(err, game.ErrNotFound) {
		t.Fatal("foreign archive", err)
	}
	for _, seat := range []string{owner, other} {
		v, e := recovered.Get(seat, archived.ID)
		if e != nil || v.Rules != RulesVersion || len(recovered.roomList()) != 0 {
			t.Fatal("archive read", v, e)
		}
	}
	for i := 0; i < 8; i++ {
		if _, err = recovered.Create(context.Background(), owner, "red"); err != nil {
			t.Fatal(err)
		}
	}
	_, err = recovered.Create(context.Background(), owner, "red")
	code(t, err, "ACTIVE_GAME_LIMIT")
	if len(recovered.roomList()) != 8 {
		t.Fatal("history counted as active")
	}
}

func TestTickerAloneFinishesRoomWithoutGetOrCommand(t *testing.T) {
	m, clock, _ := setup(t)
	v, red, _ := started(t, m)
	_, updates, stop, err := m.Watch(red, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	clock.Add(InitialMillis + 1)
	select {
	case terminal := <-updates:
		if terminal.Phase != "finished" || terminal.Reason != "timeout" || terminal.Outcome != "black_win" {
			t.Fatal("ticker result", terminal)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ticker did not finish room")
	}
	stop()
	if len(m.roomList()) != 0 {
		t.Fatal("ticker finished room not released")
	}
}

func TestPVPFinishedRoomIsEvictedAfterLastSubscriber(t *testing.T) {
	m, _, _ := setup(t)
	v, red, _ := started(t, m)
	_, ch, unwatch, err := m.Watch(red, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unwatch()
	v = command(t, m, red, v.ID, "resign", "")
	if received := <-ch; received.Phase != "finished" || received.Version != v.Version {
		t.Fatal("last update", received)
	}
	if len(m.roomList()) != 1 {
		t.Fatal("live subscription detached")
	}
	unwatch()
	if len(m.roomList()) != 0 {
		t.Fatal("finished subscriber retained")
	}
	if _, err = m.Get(red, v.ID); err != nil || len(m.roomList()) != 0 {
		t.Fatal("history lookup", err)
	}
}
