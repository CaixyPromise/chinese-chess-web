package chess

import "strings"

const LegacyRules = "legacy-java-v1"
const CasualRules = "casual-xiangqi-v1"

func SupportedRules(version string) bool { return version == LegacyRules || version == CasualRules }
func PositionKey(fen string) string {
	parts := strings.Fields(fen)
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:2], " ")
}

// CasualOutcome is shared by PVE and PVP; basic terminal positions take priority at the caller.
// Keys include the initial position; checks describe each move. Games start with red.
func CasualOutcome(fen string, plies int, keys []string, checks []bool) (outcome, reason string) {
	fields := strings.Fields(fen)
	if len(fields) == 0 {
		return "ongoing", ""
	}
	if !strings.ContainsAny(fields[0], "rRnNcCpP") {
		return "draw", "insufficient_material"
	}
	if len(keys) != plies+1 || len(checks) != plies || PositionKey(fen) != keys[len(keys)-1] {
		return "ongoing", ""
	}
	current := keys[len(keys)-1]
	matches := []int{}
	for i, key := range keys {
		if key == current {
			matches = append(matches, i)
		}
	}
	if len(matches) < 3 {
		return "ongoing", ""
	}
	start := matches[len(matches)-3]
	all, seen := [2]bool{true, true}, [2]bool{}
	for ply := start; ply < plies; ply++ {
		side := ply % 2
		seen[side] = true
		all[side] = all[side] && checks[ply]
	}
	if seen[0] && seen[1] && all[0] != all[1] {
		if all[0] {
			return "black_win", "perpetual_check"
		}
		return "red_win", "perpetual_check"
	}
	return "draw", "repetition"
}
