package game

import (
	"context"
	"testing"
)

func TestIndependentRoomsAcrossAllNativeBudgets(t *testing.T) {
	m := testManager(t, &memoryRepo{rows: map[string][]byte{}}, 2)
	type caseView struct{ owner, id, first string }
	cases := []caseView{}
	for i, p := range Profiles {
		owner := ID()
		v, e := m.Create(context.Background(), owner, "red", p.ID)
		if e != nil {
			t.Fatal(e)
		}
		move := []string{"b0c2", "h0g2", "a3a4"}[i]
		v, e = m.Command(context.Background(), owner, v.ID, Command{ID(), v.Version, "move", move})
		if e != nil {
			t.Fatal(e)
		}
		cases = append(cases, caseView{owner, v.ID, move})
	}
	for _, c := range cases {
		v := waitView(t, m, c.owner, c.id, func(v Snapshot) bool { return v.Phase == "playing" && len(v.Moves) == 2 })
		if v.Moves[0] != c.first {
			t.Fatal("room history leaked", v)
		}
		replay, e := m.position(context.Background(), v.Moves, "")
		if e != nil || replay.Fen != v.Fen {
			t.Fatal("saved board differs from native replay", e)
		}
	}
}
