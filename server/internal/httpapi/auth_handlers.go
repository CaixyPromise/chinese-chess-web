package httpapi

import (
	"context"
	"net/http"

	"chinesechess/internal/auth"
	"chinesechess/internal/game"
)

func (a *API) authRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register/start", a.registerStart)
	mux.HandleFunc("POST /api/auth/register/verify", a.registerVerify)
	mux.HandleFunc("POST /api/auth/register/complete", a.registerComplete)
	mux.HandleFunc("GET /api/auth/register/status", a.registerStatus)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("POST /api/auth/password/forgot", a.passwordForgot)
	mux.HandleFunc("POST /api/auth/password/reset", a.passwordReset)
	mux.HandleFunc("POST /api/auth/password/change", a.passwordChange)
}
func (a *API) startMail(w http.ResponseWriter, r *http.Request, purpose string) {
	var b struct {
		Email    string `json:"email"`
		ReturnTo string `json:"returnTo,omitempty"`
	}
	if !decode(w, r, &b) {
		return
	}
	id, _ := identity(r)
	task := auth.RegistrationContext{}
	if purpose == "register" {
		task.GuestOwner = id.session.Owner
		task.ReturnTo = b.ReturnTo
	}
	if err := a.auth.StartMail(r.Context(), b.Email, purpose, a.clientIP(r), task); err != nil {
		a.authFail(w, err)
		return
	}
	respond(w, 202, map[string]bool{"accepted": true})
}
func (a *API) registerStart(w http.ResponseWriter, r *http.Request)  { a.startMail(w, r, "register") }
func (a *API) passwordForgot(w http.ResponseWriter, r *http.Request) { a.startMail(w, r, "reset") }
func (a *API) heldGuest(r *http.Request) (string, error) {
	guest := ""
	if c, e := r.Cookie(cookieName); e == nil && game.ValidID(c.Value) {
		guest = c.Value
	}
	ok, err := a.auth.GuestAvailable(r.Context(), guest)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return guest, nil
}
func (a *API) registerVerify(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &b) {
		return
	}
	id, _ := identity(r)
	if id.session.User != nil {
		respond(w, 409, map[string]string{"code": "ALREADY_SIGNED_IN"})
		return
	}
	registration, err := a.auth.VerifyRegistration(r.Context(), b.Token, id.session.Owner)
	if err != nil {
		a.authFail(w, err)
		return
	}
	a.setCookie(w, r, grantCookie, registration.Grant, 15*60)
	respond(w, 200, registration)
}
func (a *API) registerComplete(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Password string `json:"password"`
		Nickname string `json:"nickname"`
	}
	if !decode(w, r, &b) {
		return
	}
	c, e := r.Cookie(grantCookie)
	if e != nil {
		respond(w, 400, map[string]string{"code": "INVALID_TOKEN"})
		return
	}
	id, _ := identity(r)
	if id.session.User != nil {
		respond(w, 409, map[string]string{"code": "ALREADY_SIGNED_IN"})
		return
	}
	session, err := a.auth.CompleteRegister(r.Context(), c.Value, id.session.Owner, b.Password, b.Nickname)
	if err != nil {
		a.authFail(w, err)
		return
	}
	a.setCookie(w, r, grantCookie, "", -1)
	a.switchSession(w, r, session)
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		SaveGuest bool   `json:"saveGuest"`
	}
	if !decode(w, r, &b) {
		return
	}
	guest, err := a.heldGuest(r)
	if err != nil {
		a.authFail(w, err)
		return
	}
	if b.SaveGuest {
		if c, e := r.Cookie(cookieName); e == nil && game.ValidID(c.Value) {
			guest = c.Value
		}
	}
	session, err := a.auth.Login(r.Context(), b.Email, b.Password, guest, a.clientIP(r), b.SaveGuest)
	if err != nil {
		a.authFail(w, err)
		return
	}
	a.switchSession(w, r, session)
}
func (a *API) switchSession(w http.ResponseWriter, r *http.Request, session auth.Session) {
	previous, ok := identity(r)
	if ok {
		// An unclaimed guest remains dormant behind its independent guest cookie;
		// the active opaque session is always revoked at the identity boundary.
		if err := a.auth.Revoke(r.Context(), previous.session.Hash); err != nil {
			a.authFail(w, err)
			return
		}
	}
	a.setCookie(w, r, sessionCookie, session.Raw, 30*24*60*60)
	r = r.WithContext(context.WithValue(r.Context(), authContextKey{}, requestIdentity{session: session, gated: true}))
	a.session(w, r)
}
func (a *API) guestSession(w http.ResponseWriter, r *http.Request) {
	guest, err := a.heldGuest(r)
	if err != nil {
		a.authFail(w, err)
		return
	}
	if guest == "" {
		guest = game.ID()
		a.setCookie(w, r, cookieName, guest, 30*24*60*60)
	}
	session, err := a.auth.NewSession(r.Context(), guest)
	if err != nil {
		a.authFail(w, err)
		return
	}
	a.switchSession(w, r, session)
}
func (a *API) logout(w http.ResponseWriter, r *http.Request) { a.guestSession(w, r) }
func (a *API) passwordReset(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &b) {
		return
	}
	if err := a.auth.ResetPassword(r.Context(), b.Token, b.Password); err != nil {
		a.authFail(w, err)
		return
	}
	a.guestSession(w, r)
}
func (a *API) passwordChange(w http.ResponseWriter, r *http.Request) {
	account := requireAccount(w, r)
	if account == "" {
		return
	}
	var b struct {
		CurrentPassword string `json:"currentPassword"`
		Password        string `json:"password"`
	}
	if !decode(w, r, &b) {
		return
	}
	if err := a.auth.ChangePassword(r.Context(), account, b.CurrentPassword, b.Password); err != nil {
		a.authFail(w, err)
		return
	}
	a.guestSession(w, r)
}

func (a *API) registerStatus(w http.ResponseWriter, r *http.Request) {
	id, _ := identity(r)
	if id.session.User != nil {
		respond(w, 409, map[string]string{"code": "ALREADY_SIGNED_IN"})
		return
	}
	c, err := r.Cookie(grantCookie)
	if err != nil {
		respond(w, 400, map[string]string{"code": "INVALID_TOKEN"})
		return
	}
	registration, err := a.auth.RegistrationStatus(r.Context(), c.Value, id.session.Owner)
	if err != nil {
		a.authFail(w, err)
		return
	}
	respond(w, 200, registration)
}
