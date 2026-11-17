package integration

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAuthRegistrationContextNewTabAndCrossBrowser(t *testing.T) {
	_, a, b, _ := setupAuthHTTP(t)
	id := strings.Repeat("a", 32)
	target := "/replay/pvp/" + id + "?side=black"
	original := a.call("GET", "/api/session", nil, 200)["identity"].(map[string]any)["key"]
	email := strings.Repeat("context", 1) + fmt.Sprint(time.Now().UnixNano()) + "@example.test"
	a.call("POST", "/api/auth/register/start", map[string]any{"email": email, "returnTo": target}, 202)
	token := mailToken(t, email)
	// A new tab shares cookies, not sessionStorage. Only the server knows where
	// registration started; verification restores the task and selected seat.
	verified := a.call("POST", "/api/auth/register/verify", map[string]any{"token": token}, 200)
	if verified["returnTo"] != target {
		t.Fatal("same browser lost registration return path")
	}
	status := a.call("GET", "/api/auth/register/status", nil, 200)
	if status["returnTo"] != target || status["email"] != email {
		t.Fatal("registration refresh lost grant context")
	}
	complete := a.call("POST", "/api/auth/register/complete", map[string]any{"password": "registration context password", "nickname": "new tab"}, 200)
	if complete["returnTo"] != target {
		t.Fatal("registration completion lost task")
	}
	identity := complete["identity"].(map[string]any)
	if identity["key"] == original || len(identity["key"].(string)) != 64 {
		t.Fatal("identity boundary fingerprint did not change")
	}
	// A fresh guest identity is distinguished from the prior guest as well.
	a.call("POST", "/api/auth/logout", map[string]any{}, 200)
	if a.call("GET", "/api/session", nil, 200)["identity"].(map[string]any)["key"] == original {
		t.Fatal("new guest reused claimed guest identity key")
	}
	email2 := fmt.Sprint(time.Now().UnixNano()) + "@example.test"
	a.call("POST", "/api/auth/register/start", map[string]any{"email": email2, "returnTo": target}, 202)
	otherToken := mailToken(t, email2)
	different := b.call("POST", "/api/auth/register/verify", map[string]any{"token": otherToken}, 200)
	if _, exists := different["returnTo"]; exists {
		t.Fatal("another browser received source guest task context")
	}
	different = b.call("GET", "/api/auth/register/status", nil, 200)
	if _, exists := different["returnTo"]; exists {
		t.Fatal("another browser grant persisted source task context")
	}
	b.call("POST", "/api/auth/register/complete", map[string]any{"password": "a second context password", "nickname": "cross browser"}, 200)
	for _, unsafe := range []string{"https://outside.example/", "//outside.example/", "/\\outside.example/", "/%2f%2foutside.example", "/account/login?returnTo=https://outside.example", "/history?mode=%0aevil", "/replay/pvp/" + id + "?side=black#token=secret"} {
		a.call("POST", "/api/auth/register/start", map[string]any{"email": fmt.Sprint(time.Now().UnixNano()) + "@example.test", "returnTo": unsafe}, 400)
	}
}
func TestAuthSlowBodiesStayOutsideIdentityGate(t *testing.T) {
	_, a, b, _ := setupAuthHTTP(t)
	for _, path := range []string{"/api/auth/register/start", "/api/games"} {
		t.Run(path, func(t *testing.T) {
			game := b.call("POST", "/api/games", map[string]any{"human": "red", "difficulty": "easy"}, 201)
			endpoint, _ := url.Parse(a.base)
			conn, err := net.Dial("tcp", endpoint.Host)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			cookies := []string{}
			for _, c := range a.client.Jar.Cookies(endpoint) {
				cookies = append(cookies, c.Name+"="+c.Value)
			}
			_, err = fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: %s\r\nOrigin: %s\r\nContent-Type: application/json\r\nX-CSRF-Token: %s\r\nCookie: %s\r\nContent-Length: 128\r\n\r\n{\"email\":\"unfinished", path, endpoint.Host, a.base, a.csrf, strings.Join(cookies, "; "))
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(75 * time.Millisecond)
			// These operations use different lock modes (read, domain mutation and
			// auth write). None may wait for the attacker's five-second body timeout.
			b.client.Timeout = time.Second
			defer func() { b.client.Timeout = 0 }()
			b.call("GET", "/api/session", nil, 200)
			b.call("POST", "/api/games/"+game["id"].(string)+"/commands", map[string]any{"requestId": "slow-body-move", "expectedVersion": game["version"], "type": "move", "move": "b0c2"}, 200)
			b.call("POST", "/api/auth/logout", map[string]any{}, 200)
			_ = conn.SetReadDeadline(time.Now().Add(7 * time.Second))
			response, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusRequestTimeout {
				t.Fatalf("incomplete body status %d", response.StatusCode)
			}
		})
	}
}

func TestAuthSMTPWaitDoesNotHoldIdentityGate(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, a, b, _ := setupAuthHTTP(t, listener.Addr().String())
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, e := listener.Accept()
		if e == nil {
			accepted <- conn
		}
	}()
	done := make(chan error, 1)
	go func() {
		payload := fmt.Sprintf(`{"email":"smtpwait%d@example.test"}`, time.Now().UnixNano())
		request, _ := http.NewRequest("POST", a.base+"/api/auth/register/start", strings.NewReader(payload))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", a.base)
		request.Header.Set("X-CSRF-Token", a.csrf)
		response, e := a.client.Do(request)
		if e == nil {
			response.Body.Close()
			if response.StatusCode != 202 {
				e = fmt.Errorf("SMTP recovery status %d", response.StatusCode)
			}
		}
		done <- e
	}()
	var smtp net.Conn
	select {
	case smtp = <-accepted:
	case <-time.After(3 * time.Second):
		t.Fatal("SMTP connection did not start")
	}
	defer smtp.Close()
	b.client.Timeout = time.Second
	b.call("GET", "/api/session", nil, 200)
	b.call("POST", "/api/auth/logout", map[string]any{}, 200)
	smtp.Close()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SMTP request did not finish")
	}
}
func TestAuthRechecksSessionAfterBodyBuffered(t *testing.T) {
	_, a, _, _ := setupAuthHTTP(t)
	endpoint, _ := url.Parse(a.base)
	conn, err := net.Dial("tcp", endpoint.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cookies := []string{}
	for _, c := range a.client.Jar.Cookies(endpoint) {
		cookies = append(cookies, c.Name+"="+c.Value)
	}
	body := `{"human":"red","difficulty":"easy"}`
	fmt.Fprintf(conn, "POST /api/games HTTP/1.1\r\nHost: %s\r\nOrigin: %s\r\nContent-Type: application/json\r\nX-CSRF-Token: %s\r\nCookie: %s\r\nContent-Length: %d\r\n\r\n%s", endpoint.Host, a.base, a.csrf, strings.Join(cookies, "; "), len(body), body[:8])
	time.Sleep(40 * time.Millisecond)
	a.client.Timeout = time.Second
	a.call("POST", "/api/auth/logout", map[string]any{}, 200)
	if _, err = fmt.Fprint(conn, body[8:]); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatalf("request authenticated before its body completed: %d", response.StatusCode)
	}
}
