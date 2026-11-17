package account

import (
	"testing"
	"time"
)

func TestCalendarBoundariesAndPerspective(t *testing.T) {
	now := time.Date(2026, 11, 16, 16, 1, 0, 0, time.UTC)
	start, end, err := Range("", "", now)
	if err != nil || start.Format(time.DateOnly) != "2025-11-18" || end.Format(time.DateOnly) != "2026-11-18" {
		t.Fatalf("calendar: %v %v %v", start, end, err)
	}
	for _, dates := range [][2]string{{"2026-11-18", "2026-11-18"}, {"2025-01-01", "2026-11-17"}, {"2026-02-30", "2026-11-17"}, {"2026-11-17", ""}, {"2026-11-17", "2026-11-16"}} {
		if _, _, err = Range(dates[0], dates[1], now); err == nil {
			t.Fatal("accepted invalid calendar", dates)
		}
	}
	var c Counts
	Add(&c, "red_win", "black")
	Add(&c, "draw", "black")
	Add(&c, "black_win", "black")
	Add(&c, "ongoing", "black")
	Add(&c, "red_win", "")
	if c != (Counts{Win: 1, Draw: 1, Loss: 1, Total: 3}) {
		t.Fatal(c)
	}
}
