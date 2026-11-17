package history

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestParseQuery(t *testing.T) {
	q, err := ParseQuery("", "", 0)
	if err != nil || q.Mode != "all" || q.Limit != 20 || q.Before != nil {
		t.Fatalf("defaults: %#v %v", q, err)
	}
	e := Entry{ID: strings.Repeat("a", 32), Mode: "pvp", Side: "red", UpdatedAt: 123}
	token := Cursor("all", e)
	q, err = ParseQuery("all", token, 50)
	if err != nil || q.Before == nil || q.Before.ID != e.ID || q.Before.Mode != e.Mode || q.Before.Side != "red" || q.Before.UpdatedAt != 123 {
		t.Fatalf("round trip: %#v %v", q, err)
	}
	legacy := `{"v":1,"filter":"all","updatedAt":123,"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mode":"pvp"}`
	if old, err := ParseQuery("all", base64.RawURLEncoding.EncodeToString([]byte(legacy)), 20); err != nil || old.Before.Side != "" {
		t.Fatalf("legacy cursor: %#v %v", old, err)
	}
	malformed := []string{"!", token + "=", token[:5] + "\n" + token[5:], strings.Repeat("a", 513), "e30"}
	for _, raw := range []string{
		`{"v":2,"filter":"all","updatedAt":123,"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mode":"pvp"}`,
		`{"v":1,"filter":"all","updatedAt":-1,"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mode":"pvp"}`,
		`{"v":1,"filter":"all","updatedAt":123,"id":"bad","mode":"pvp"}`,
		`{"v":1,"filter":"all","updatedAt":123,"id":"gggggggggggggggggggggggggggggggg","mode":"pvp"}`,
		`{"v":1,"filter":"all","updatedAt":123,"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mode":"all"}`,
		`{"v":1,"filter":"all","updatedAt":123,"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mode":"pvp","owner":"secret"}`,
		`{"v":1,"v":1,"filter":"all","updatedAt":123,"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mode":"pvp"}`,
		`{"v":1,"filter":"all","updatedAt":123,"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mode":"pvp"} {}`,
		`{"v":1,"filter":"all","id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mode":"pvp"}`,
	} {
		malformed = append(malformed, base64.RawURLEncoding.EncodeToString([]byte(raw)))
	}
	for _, token := range malformed {
		if _, err := ParseQuery("all", token, 1); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("accepted invalid cursor %q: %v", token, err)
		}
	}
	for _, mode := range []string{"ALL", "unknown", " pve"} {
		if _, err := ParseQuery(mode, "", 1); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("accepted mode %q", mode)
		}
	}
	for _, limit := range []int{-1, 51} {
		if _, err := ParseQuery("all", "", limit); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("accepted limit %d", limit)
		}
	}
	if _, err := ParseQuery("pvp", token, 1); !errors.Is(err, ErrInvalidQuery) {
		t.Fatal("accepted cursor from different filter")
	}
	if _, err := ParseQuery("pve", Cursor("pve", e), 1); !errors.Is(err, ErrInvalidQuery) {
		t.Fatal("accepted cursor position from different mode")
	}
}
