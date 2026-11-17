package chess

import "testing"

func TestCasualRepetitionAndCheckingResponsibility(t *testing.T) {
	key := PositionKey(Initial)
	keys := []string{key, "b", "c", "d", key, "b", "c", "d", key}
	for _, tc := range []struct {
		name            string
		checks          []bool
		outcome, reason string
	}{
		{"quiet repetition", make([]bool, 8), "draw", "repetition"},
		{"red continuous checking", []bool{true, false, true, false, true, false, true, false}, "black_win", "perpetual_check"},
		{"black continuous checking", []bool{false, true, false, true, false, true, false, true}, "red_win", "perpetual_check"},
		{"interrupted checking", []bool{true, false, true, false, false, false, true, false}, "draw", "repetition"},
		{"both checking", []bool{true, true, true, true, true, true, true, true}, "draw", "repetition"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome, reason := CasualOutcome(Initial, 8, keys, tc.checks)
			if outcome != tc.outcome || reason != tc.reason {
				t.Fatal(outcome, reason)
			}
		})
	}
	if outcome, _ := CasualOutcome(Initial, 4, keys[:5], make([]bool, 4)); outcome != "ongoing" {
		t.Fatal("only two occurrences")
	}
	if outcome, _ := CasualOutcome(Initial, 8, keys, make([]bool, 7)); outcome != "ongoing" {
		t.Fatal("incomplete history")
	}
	if outcome, _ := CasualOutcome(Initial, 8, append([]string{"other"}, keys[1:]...), make([]bool, 8)); outcome != "ongoing" {
		t.Fatal("position identity")
	}
}

func TestCasualNoAttackersAndPositionIdentity(t *testing.T) {
	fen := "3k5/9/9/9/9/9/9/9/9/5K3 r - - 0 1"
	if outcome, reason := CasualOutcome(fen, 0, nil, nil); outcome != "draw" || reason != "insufficient_material" {
		t.Fatal(outcome, reason)
	}
	if PositionKey(Initial) == PositionKey("rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR b - - 0 1") {
		t.Fatal("turn missing")
	}
	if PositionKey(Initial) != PositionKey("rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 99 42") {
		t.Fatal("clock changes identity")
	}
}
