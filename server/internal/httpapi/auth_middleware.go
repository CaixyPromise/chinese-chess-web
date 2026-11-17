package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"chinesechess/internal/auth"
	"chinesechess/internal/game"
)

const sessionCookie = "chess_session"
const grantCookie = "chess_registration"

type authContextKey struct{}
type requestIdentity struct {
	session auth.Session
	gated   bool
}

func identity(r *http.Request) (requestIdentity, bool) {
	v, ok := r.Context().Value(authContextKey{}).(requestIdentity)
	return v, ok
}
func (a *API) authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	var session auth.Session
	c, err := r.Cookie(sessionCookie)
	if err == nil {
		session, err = a.auth.Resolve(r.Context(), c.Value)
	} else {
		err = auth.ErrSession
	}
	if errors.Is(err, auth.ErrSession) && r.Method == "GET" && r.URL.Path == "/api/session" {
		guest := owner(r)
		available, e := a.auth.GuestAvailable(r.Context(), guest)
		if e != nil {
			return r, e
		}
		if !available || !game.ValidID(guest) {
			guest = game.ID()
			a.setCookie(w, r, cookieName, guest, 30*24*60*60)
		}
		session, err = a.auth.NewSession(r.Context(), guest)
		if err == nil {
			a.setCookie(w, r, sessionCookie, session.Raw, 30*24*60*60)
		}
	}
	if err != nil {
		return r, err
	}
	return r.WithContext(context.WithValue(r.Context(), authContextKey{}, requestIdentity{session: session, gated: true})), nil
}
func (a *API) setCookie(w http.ResponseWriter, r *http.Request, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.auth.PublicOrigin(), "https://"), SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (a *API) protected(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		// Socket handlers own the short gate themselves. It must never be retained
		// through streaming, otherwise logout could not acquire its write lock.
		socket := strings.HasSuffix(r.URL.Path, "/socket")
		if r.Method == "POST" || socket {
			origin := r.Header.Get("Origin")
			if origin != "" && origin != a.auth.PublicOrigin() {
				if strings.HasPrefix(r.URL.Path, "/api/admin/") {
					a.admin.Security(r.Context(), "", "FORBIDDEN")
				}
				respond(w, 403, map[string]string{"code": "FORBIDDEN"})
				return
			}
			if r.Method == "POST" && r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				if strings.HasPrefix(r.URL.Path, "/api/admin/") {
					a.admin.Security(r.Context(), "", "FORBIDDEN")
				}
				respond(w, 403, map[string]string{"code": "FORBIDDEN"})
				return
			}
		}
		if socket {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == "POST" && !readAuthBody(w, r) {
			return
		}
		write := r.Method == "POST" && (strings.HasPrefix(r.URL.Path, "/api/auth/") || strings.HasPrefix(r.URL.Path, "/api/admin/"))
		mailRequest := r.Method == "POST" && (r.URL.Path == "/api/auth/register/start" || r.URL.Path == "/api/auth/password/forgot")
		var once sync.Once
		release := func() {
			once.Do(func() {
				if write {
					a.identityGate.Unlock()
				} else {
					a.identityGate.RUnlock()
				}
			})
		}
		if write {
			a.identityGate.Lock()
		} else {
			a.identityGate.RLock()
		}
		defer release()
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		verified, err := a.authenticate(w, r)
		if err != nil {
			if errors.Is(err, auth.ErrSession) {
				if strings.HasPrefix(r.URL.Path, "/api/admin/") {
					a.admin.Security(r.Context(), "", "SESSION_REQUIRED")
				}
				respond(w, 401, map[string]string{"code": "SESSION_REQUIRED"})
			} else {
				a.authFail(w, err)
			}
			return
		}
		if r.Method == "POST" {
			id, _ := identity(verified)
			want := id.session.CSRF()
			got := r.Header.Get("X-CSRF-Token")
			if len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
				if strings.HasPrefix(r.URL.Path, "/api/admin/") {
					a.admin.Security(r.Context(), id.session.Owner, "CSRF_INVALID")
				}
				respond(w, 403, map[string]string{"code": "CSRF_INVALID"})
				return
			}
		}
		// Email issuance changes no principal or domain admission state. Its SMTP
		// connection must not serialize unrelated games behind an external service.
		if mailRequest {
			release()
		}
		next.ServeHTTP(w, verified)
	})
}
func requireAccount(w http.ResponseWriter, r *http.Request) string {
	if id, ok := identity(r); ok && id.session.User != nil {
		return id.session.User.ID
	}
	respond(w, 401, map[string]string{"code": "ACCOUNT_REQUIRED"})
	return ""
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func (a *API) authFail(w http.ResponseWriter, err error) {
	var e *auth.Error
	if errors.As(err, &e) {
		respond(w, e.Status, map[string]string{"code": e.Code})
		return
	}
	// Never log raw transport/database input, credentials, addresses or tokens.
	a.log.Warn("authentication request failed")
	respond(w, 503, map[string]string{"code": "UNAVAILABLE"})
}

// Kept here so the API owns a single lock shared by identity mutations and all
// game/domain routes, including invitation approval.
type identityGate = sync.RWMutex

func (a *API) socketIdentity(w http.ResponseWriter, r *http.Request) (*http.Request, func(), func(), bool) {
	noop := func() {}
	if a.auth == nil {
		return r, noop, noop, true
	}
	a.identityGate.RLock()
	var once sync.Once
	release := func() { once.Do(a.identityGate.RUnlock) }
	if r.Header.Get("Origin") != a.auth.PublicOrigin() {
		release()
		respond(w, 403, map[string]string{"code": "FORBIDDEN"})
		return r, noop, noop, false
	}
	verified, err := a.authenticate(w, r)
	if err != nil {
		release()
		respond(w, 401, map[string]string{"code": "SESSION_REQUIRED"})
		return r, noop, noop, false
	}
	id, _ := identity(verified)
	ctx, closeSocket := a.auth.SocketContext(verified.Context(), id.session.Hash)
	return verified.WithContext(ctx), release, closeSocket, true
}

func (a *API) requireAccount(w http.ResponseWriter, r *http.Request) string {
	return requireAccount(w, r)
}

func (a *API) clientIP(r *http.Request) string {
	host := clientIP(r)
	if a.auth == nil {
		return host
	}
	return a.auth.ClientAddress(host, r.Header.Get("X-Real-IP"))
}
