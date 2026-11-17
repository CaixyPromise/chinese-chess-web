package integration

import (
	"chinesechess/internal/account"
	"chinesechess/internal/game"
	"testing"
	"time"
)

func TestAccountActivitySingaporeBoundaryAndLinkedEventDedupe(t *testing.T) {
	ctx, repo, db := analysisDatabase(t)
	owner, a, b, other := game.ID(), game.ID(), game.ID(), game.ID()
	if _, err := db.Exec(ctx, `INSERT INTO auth_owner_links VALUES($1,$3,1),($2,$3,1),($3,$3,1)`, a, b, owner); err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct{ key, who, kind, at, origin string }{
		{"outside", a, "game", "2026-10-31T15:59:59Z", ""},
		{"same-game", a, "game", "2026-10-31T16:00:00Z", ""},
		{"same-game", b, "game", "2026-10-31T16:00:00Z", ""},
		{"analysis", a, "analysis", "2026-11-01T15:59:59Z", "manual"},
		{"practice", a, "practice", "2026-11-01T16:00:00Z", ""},
		{"private", other, "game", "2026-11-01T16:00:00Z", ""},
	} {
		at, err := time.Parse(time.RFC3339, event.at)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO account_activity(event_key,owner_key,kind,source_mode,source_id,occurred_at,origin) VALUES($1,$2,$3,'pvp','fixture',$4,$5)`, event.key, event.who, event.kind, at.UnixMilli(), event.origin); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.AccountActivity(ctx, owner, "2026-11-01", "2026-11-02")
	if err != nil {
		t.Fatal(err)
	}
	if got.TimeZone != account.TimeZone || len(got.Days) != 2 || got.Totals != (account.Totals{Game: 1, Practice: 1, Manual: 1}) {
		t.Fatalf("calendar aggregation %+v", got)
	}
	if got.Days[0].Date != "2026-11-01" || got.Days[0].Game != 1 || got.Days[0].Manual != 1 || got.Days[0].Practice != 0 {
		t.Fatal("first local day", got.Days[0])
	}
	if got.Days[1].Date != "2026-11-02" || got.Days[1].Game != 0 || got.Days[1].Practice != 1 {
		t.Fatal("second local day", got.Days[1])
	}
	for _, day := range got.Days {
		if !day.Partial {
			t.Fatal("historical creation coverage overstated", day)
		}
	}
}
