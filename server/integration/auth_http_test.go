package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chinesechess/internal/auth"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"chinesechess/internal/httpapi"
	"chinesechess/internal/store"
	"github.com/coder/websocket"
)

type authBrowser struct {
	client     *http.Client
	base, csrf string
	t          *testing.T
}

func (b *authBrowser) call(method, path string, body any, status int) map[string]any {
	b.t.Helper()
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, b.base+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if method == "POST" {
		req.Header.Set("X-CSRF-Token", b.csrf)
		req.Header.Set("Origin", b.base)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode != status {
		b.t.Fatalf("%s %s: status=%d expected=%d code=%v", method, path, resp.StatusCode, status, out["code"])
	}
	if csrf, ok := out["csrfToken"].(string); ok {
		b.csrf = csrf
	}
	return out
}
func mailToken(t *testing.T, email string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:8025/api/v1/messages")
		if err != nil {
			t.Fatal("Mailpit required on 8025")
		}
		var list struct {
			Messages []struct {
				ID string
				To []struct{ Address string }
			}
		}
		err = json.NewDecoder(resp.Body).Decode(&list)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range list.Messages {
			for _, to := range m.To {
				if to.Address == email {
					resp, err = http.Get("http://127.0.0.1:8025/api/v1/message/" + m.ID)
					if err != nil {
						t.Fatal(err)
					}
					var message struct{ Text string }
					err = json.NewDecoder(resp.Body).Decode(&message)
					resp.Body.Close()
					if err != nil {
						t.Fatal(err)
					}
					for _, line := range strings.Split(message.Text, "\n") {
						line = strings.TrimSpace(line)
						if strings.HasPrefix(line, "http") {
							u, e := url.Parse(line)
							if e != nil {
								continue
							}
							q, _ := url.ParseQuery(u.Fragment)
							if token := q.Get("token"); token != "" {
								return token
							}
						}
					}
				}
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatal("expected test mailbox message")
	return ""
}
func setupAuthHTTP(t *testing.T, smtpAddress ...string) (*auth.Service, *authBrowser, *authBrowser, *store.Postgres) {
	t.Helper()
	ctx, repo, _ := analysisDatabase(t)
	root, _ := filepath.Abs("../..")
	pool := engine.NewPool(filepath.Join(root, "build/engine/xiangqi-engine"), 1, 4)
	t.Cleanup(pool.Close)
	games, err := game.New(repo, pool, pool, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(games.Close)
	server := httptest.NewUnstartedServer(nil)
	base := "http://" + server.Listener.Addr().String()
	smtpAddr := "127.0.0.1:1025"
	if len(smtpAddress) > 0 {
		smtpAddr = smtpAddress[0]
	}
	service, err := auth.New(repo.AuthDB(), auth.Config{PublicURL: base, SMTPAddr: smtpAddr, SMTPFrom: "chess@example.test"}, repo.ReloadIdentity)
	if err != nil {
		t.Fatal(err)
	}
	server.Config.Handler = httpapi.NewWithAuth(games, nil, nil, service, nil, root, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.Start()
	t.Cleanup(server.Close)
	newBrowser := func() *authBrowser {
		jar, _ := cookiejar.New(nil)
		return &authBrowser{client: &http.Client{Jar: jar}, base: base, t: t}
	}
	a, b := newBrowser(), newBrowser()
	a.call("GET", "/api/session", nil, 200)
	b.call("GET", "/api/session", nil, 200)
	_ = ctx
	return service, a, b, repo
}
func TestAuthRealSMTPRegistrationSessionsAndRecovery(t *testing.T) {
	_, a, b, _ := setupAuthHTTP(t)
	email := game.ID() + "@example.test"
	password := "a strong password for chess"
	guest := a.call("POST", "/api/games", map[string]any{"human": "red", "difficulty": "easy"}, 201)
	gameID := guest["id"].(string)
	// No request with a guessed legacy bearer may authenticate in production.
	noSession := &authBrowser{client: &http.Client{}, base: a.base, t: t}
	noSession.call("GET", "/api/games/"+gameID, nil, 401)
	a.call("POST", "/api/auth/register/start", map[string]any{"email": email}, 202)
	token := mailToken(t, email)
	a.call("POST", "/api/auth/register/verify", map[string]any{"token": token}, 200)
	before := a.call("GET", "/api/session", nil, 200)
	if before["identity"].(map[string]any)["kind"] != "guest" {
		t.Fatal("email proof became login")
	}
	a.call("POST", "/api/auth/register/verify", map[string]any{"token": token}, 400)
	account := a.call("POST", "/api/auth/register/complete", map[string]any{"password": password, "nickname": "棋友甲"}, 200)
	if account["identity"].(map[string]any)["kind"] != "account" {
		t.Fatal("registration did not log in")
	}
	a.call("GET", "/api/games/"+gameID, nil, 200)
	a.call("POST", "/api/auth/register/complete", map[string]any{"password": password, "nickname": "again"}, 400)
	b.call("POST", "/api/auth/login", map[string]any{"email": email, "password": "wrong"}, 401)
	b.call("POST", "/api/auth/login", map[string]any{"email": email, "password": password}, 200)
	b.call("GET", "/api/games/"+gameID, nil, 200)
	// Server-side session revocation closes an already authorized WebSocket.
	u, _ := url.Parse(a.base)
	headers := http.Header{"Origin": []string{a.base}}
	for _, c := range a.client.Jar.Cookies(u) {
		headers.Add("Cookie", c.Name+"="+c.Value)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(a.base, "http")+"/api/games/"+gameID+"/socket", &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if _, _, err = conn.Read(ctx); err != nil {
		t.Fatal(err)
	}
	a.call("POST", "/api/auth/logout", map[string]any{}, 200)
	if _, _, err = conn.Read(ctx); err == nil {
		t.Fatal("logout left socket open")
	}
	a.call("GET", "/api/games/"+gameID, nil, 404)
	// Registration and reset share the documented email cooldown.
	time.Sleep(61 * time.Second)
	// Real recovery mail and a one-use reset invalidate other devices as well.
	a.call("POST", "/api/auth/password/forgot", map[string]any{"email": email}, 202)
	reset := mailToken(t, email)
	next := "another long strong password"
	a.call("POST", "/api/auth/password/reset", map[string]any{"token": reset, "password": next}, 200)
	b.call("GET", "/api/games/"+gameID, nil, 401)
	a.call("POST", "/api/auth/password/reset", map[string]any{"token": reset, "password": next}, 400)
	a.call("POST", "/api/auth/login", map[string]any{"email": email, "password": password}, 401)
	a.call("POST", "/api/auth/login", map[string]any{"email": email, "password": next}, 200)
	a.call("POST", "/api/auth/password/change", map[string]any{"currentPassword": next, "password": "a third strong chess password"}, 200)
	if a.call("GET", "/api/session", nil, 200)["identity"].(map[string]any)["kind"] != "guest" {
		t.Fatal("change password must return to login")
	}
}
func TestAuthCrossBrowserRegistrationAndDefaultNoMerge(t *testing.T) {
	_, a, b, _ := setupAuthHTTP(t)
	email := game.ID() + "@example.test"
	password := "the guest does not merge"
	ga := a.call("POST", "/api/games", map[string]any{"human": "red", "difficulty": "easy"}, 201)
	id := ga["id"].(string)
	a.call("POST", "/api/auth/register/start", map[string]any{"email": email}, 202)
	token := mailToken(t, email)
	b.call("POST", "/api/auth/register/verify", map[string]any{"token": token}, 200)
	b.call("POST", "/api/auth/register/complete", map[string]any{"password": password, "nickname": "Cross browser"}, 200)
	b.call("GET", "/api/games/"+id, nil, 404)
	a.call("GET", "/api/games/"+id, nil, 200)
	a.call("POST", "/api/auth/login", map[string]any{"email": email, "password": password}, 200)
	a.call("GET", "/api/games/"+id, nil, 404)
	a.call("POST", "/api/auth/logout", map[string]any{}, 200)
	a.call("GET", "/api/games/"+id, nil, 200)
	a.call("POST", "/api/auth/login", map[string]any{"email": email, "password": password, "saveGuest": true}, 200)
	a.call("GET", "/api/games/"+id, nil, 200)
	b.call("GET", "/api/games/"+id, nil, 200)
	a.call("POST", "/api/auth/logout", map[string]any{}, 200)
	a.call("GET", "/api/games/"+id, nil, 404)
}
func TestAuthCSRFAndMailLimits(t *testing.T) {
	service, a, _, _ := setupAuthHTTP(t)
	_ = service
	saved := a.csrf
	a.csrf = "wrong"
	a.call("POST", "/api/auth/register/start", map[string]any{"email": game.ID() + "@example.test"}, 403)
	a.csrf = saved
	email := game.ID() + "@example.test"
	a.call("POST", "/api/auth/register/start", map[string]any{"email": email}, 202)
	a.call("POST", "/api/auth/register/start", map[string]any{"email": email}, 429)
}

func TestAuthSMTPFailureDoesNotEnumerateAndRecovery(t *testing.T) {
	_, a, _, repo := setupAuthHTTP(t, "127.0.0.1:1")
	ctx := context.Background()
	existingRegister := game.ID() + "@example.test"
	existingReset := game.ID() + "@example.test"
	for _, email := range []string{existingRegister, existingReset} {
		if _, err := repo.AuthDB().Exec(ctx, `INSERT INTO auth_accounts(id,email,password_hash,nickname,created_at) VALUES($1,$2,'unused-test-fixture','fixture',1)`, game.ID(), email); err != nil {
			t.Fatal(err)
		}
	}
	fresh := game.ID() + "@example.test"
	for _, c := range []struct{ path, email string }{
		{"/api/auth/register/start", existingRegister}, {"/api/auth/register/start", fresh},
		{"/api/auth/password/forgot", game.ID() + "@example.test"}, {"/api/auth/password/forgot", existingReset},
	} {
		body := a.call("POST", c.path, map[string]any{"email": c.email}, 202)
		if body["accepted"] != true || len(body) != 1 {
			t.Fatal("email state leaked through response")
		}
	}
	// A failed SMTP handoff leaves no usable email token. Application restart
	// with working SMTP can issue and deliver a fresh proof without a new queue.
	var usable int
	if err := repo.AuthDB().QueryRow(ctx, `SELECT count(*) FROM auth_tokens WHERE used_at=0`).Scan(&usable); err != nil || usable != 0 {
		t.Fatal("undelivered proof remained usable")
	}
	good, err := auth.New(repo.AuthDB(), auth.Config{PublicURL: a.base, SMTPAddr: "127.0.0.1:1025", SMTPFrom: "chess@example.test"}, repo.ReloadIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if err = good.StartMail(ctx, fresh, "register", "recovered-process"); err != nil {
		t.Fatal(err)
	}
	token := mailToken(t, fresh)
	if _, _, err = good.VerifyRegister(ctx, token, game.ID()); err != nil {
		t.Fatal("SMTP recovery did not deliver usable proof")
	}
}
