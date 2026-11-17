package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode"

	"chinesechess/internal/game"
	"github.com/jackc/pgx/v5"
)

type RegistrationContext struct{ GuestOwner, ReturnTo string }
type Registration struct {
	Grant    string `json:"-"`
	Email    string `json:"email"`
	ReturnTo string `json:"returnTo,omitempty"`
}

// Return paths name existing local task routes. Path encodings, fragments and
// unrecognized query controls are rejected, rather than decoded a second time
// by the browser into a different navigation target.
func NormalizeReturnTo(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	invalid := func() (string, error) { return "", problem("INVALID_RETURN_PATH", 400) }
	if len(raw) > 4096 || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\\#") {
		return invalid()
	}
	for _, r := range raw {
		if unicode.IsControl(r) {
			return invalid()
		}
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.RawPath != "" || strings.Contains(u.Path, "%") {
		return invalid()
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	valid := u.Path == "/" || u.Path == "/account" || u.Path == "/history" || u.Path == "/pvp"
	if len(parts) == 2 && (parts[0] == "games" || parts[0] == "rooms" || parts[0] == "join") && game.ValidID(parts[1]) {
		valid = true
	}
	if len(parts) == 3 && parts[0] == "replay" && (parts[1] == "pve" || parts[1] == "pvp") && game.ValidID(parts[2]) {
		valid = true
	}
	if !valid {
		return invalid()
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return invalid()
	}
	for key, values := range q {
		if key != "side" && key != "mode" && key != "cursor" && key != "prev" && key != "from" {
			return invalid()
		}
		for _, v := range values {
			for _, r := range v {
				if unicode.IsControl(r) || r == '\\' {
					return invalid()
				}
			}
			switch key {
			case "side":
				if v != "red" && v != "black" {
					return invalid()
				}
			case "mode":
				if v != "all" && v != "pve" && v != "pvp" {
					return invalid()
				}
			case "from":
				if v != "/history" && !strings.HasPrefix(v, "/history?") {
					return invalid()
				}
				if _, e := NormalizeReturnTo(v); e != nil {
					return invalid()
				}
			}
		}
	}
	result := u.Path
	if query := q.Encode(); query != "" {
		result += "?" + query
	}
	return result, nil
}
func (s *Service) RegistrationStatus(ctx context.Context, grant, guest string) (Registration, error) {
	if !validToken(grant) {
		return Registration{}, problem("INVALID_TOKEN", 400)
	}
	var out Registration
	var bound string
	var expiry int64
	err := s.db.QueryRow(ctx, `SELECT email,guest_owner,expires_at,return_path FROM auth_registration_grants WHERE token_hash=$1 AND used_at=0`, Hash(grant)).Scan(&out.Email, &bound, &expiry, &out.ReturnTo)
	if errors.Is(err, pgx.ErrNoRows) {
		return Registration{}, problem("INVALID_TOKEN", 400)
	}
	if err != nil {
		return Registration{}, err
	}
	if bound != guest {
		return Registration{}, problem("INVALID_TOKEN", 400)
	}
	if expiry <= now() {
		return Registration{}, problem("TOKEN_EXPIRED", 400)
	}
	return out, nil
}
