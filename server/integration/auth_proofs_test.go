package integration

import (
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"chinesechess/internal/auth"
	"chinesechess/internal/game"
	"chinesechess/internal/pvp"
)

func proofToken(seed byte) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat(string([]byte{seed}), 32)))
}
func authCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *auth.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
func TestAuthPurposeExpiryConcurrencyAndAtomicClaim(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	service, err := auth.New(repo.AuthDB(), auth.Config{PublicURL: "http://127.0.0.1:8765", SMTPAddr: "127.0.0.1:1025", SMTPFrom: "chess@example.test"}, repo.ReloadIdentity)
	if err != nil {
		t.Fatal(err)
	}
	seed := func(token, purpose, email string, expiry int64) {
		t.Helper()
		_, err := conn.Exec(ctx, `INSERT INTO auth_tokens(token_hash,purpose,email,expires_at) VALUES($1,$2,$3,$4)`, auth.Hash(token), purpose, email, expiry)
		if err != nil {
			t.Fatal(err)
		}
	}
	guest := game.ID()
	expired := proofToken(1)
	seed(expired, "register", "expired@example.test", time.Now().Add(-time.Second).UnixMilli())
	_, _, err = service.VerifyRegister(ctx, expired, guest)
	authCode(t, err, "TOKEN_EXPIRED")
	wrong := proofToken(2)
	seed(wrong, "reset", "wrong@example.test", time.Now().Add(time.Minute).UnixMilli())
	_, _, err = service.VerifyRegister(ctx, wrong, guest)
	authCode(t, err, "INVALID_TOKEN")
	token := proofToken(3)
	seed(token, "register", "proof@example.test", time.Now().Add(time.Minute).UnixMilli())
	grants := make(chan string, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); g, _, e := service.VerifyRegister(ctx, token, guest); grants <- g; errs <- e }()
	}
	wg.Wait()
	close(grants)
	close(errs)
	successes := 0
	for e := range errs {
		if e == nil {
			successes++
		} else {
			authCode(t, e, "INVALID_TOKEN")
		}
	}
	if successes != 1 {
		t.Fatal("email proof consumed more than once")
	}
	var grant string
	for g := range grants {
		if g != "" {
			grant = g
		}
	}
	// The grant is bound to the current guest, separately from email ownership.
	_, err = service.CompleteRegister(ctx, grant, game.ID(), "a strong proof password", "proof")
	authCode(t, err, "INVALID_TOKEN")
	session, err := service.CompleteRegister(ctx, grant, guest, "a strong proof password", "proof")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CompleteRegister(ctx, grant, guest, "a strong proof password", "again")
	authCode(t, err, "INVALID_TOKEN")
	if session.User == nil || session.User.ID == guest {
		t.Fatal("account and source identity conflated")
	}
	var hash string
	if err = conn.QueryRow(ctx, `SELECT password_hash FROM auth_accounts WHERE id=$1`, session.Owner).Scan(&hash); err != nil || !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatal("password hash storage")
	}
	// A current PVP cannot become two seats belonging to one account. No link or
	// login session may partially commit on rejection.
	otherGuest := game.ID()
	record := pvp.Record{RedOwner: session.Owner, BlackOwner: otherGuest, Host: session.Owner, State: pvp.State{ID: game.ID(), Version: 1, Fen: game.Initial, Phase: "playing", Outcome: "ongoing", Red: pvp.Seat{Occupied: true}, Black: pvp.Seat{Occupied: true}}}
	if err = repo.SavePVP(ctx, record, 0); err != nil {
		t.Fatal(err)
	}
	_, err = service.Login(ctx, "proof@example.test", "a strong proof password", otherGuest, "test-ip", true)
	authCode(t, err, "MERGE_CONFLICT")
	var linked bool
	if err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_owner_links WHERE owner_key=$1)`, otherGuest).Scan(&linked); err != nil || linked {
		t.Fatal("failed claim partially linked source")
	}
	// Expiration is checked by the server for both absolute and idle lifetime.
	if _, err = conn.Exec(ctx, `UPDATE auth_sessions SET last_seen=$2 WHERE token_hash=$1`, session.Hash, time.Now().Add(-8*24*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Resolve(ctx, session.Raw); !errors.Is(err, auth.ErrSession) {
		t.Fatal("idle session accepted")
	}
}
