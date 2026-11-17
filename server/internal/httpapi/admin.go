package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"chinesechess/internal/admin"
	"chinesechess/internal/auth"
)

func (a *API) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/users", a.adminUsers)
	mux.HandleFunc("GET /api/admin/users/{id}", a.adminUser)
	mux.HandleFunc("GET /api/admin/sessions", a.adminSessions)
	mux.HandleFunc("GET /api/admin/tasks", a.adminTasks)
	mux.HandleFunc("GET /api/admin/tasks/{id}", a.adminTask)
	mux.HandleFunc("GET /api/admin/audit", a.adminAudits)
	mux.HandleFunc("GET /api/admin/audit/{id}", a.adminAudit)
	mux.HandleFunc("POST /api/admin/reauth", a.adminReauth)
	for route, action := range map[string]string{"users/{id}/ban": "user.ban", "users/{id}/unban": "user.unban", "users/{id}/revoke-sessions": "user.revoke-sessions", "sessions/{id}/revoke": "session.revoke", "tasks/{id}/pause": "task.pause"} {
		mux.HandleFunc("POST /api/admin/"+route, func(w http.ResponseWriter, r *http.Request) { a.adminAction(w, r, action) })
	}
}
func (a *API) adminIdentity(w http.ResponseWriter, r *http.Request) (auth.Session, bool) {
	id, ok := identity(r)
	if !ok {
		respond(w, 403, map[string]string{"code": "FORBIDDEN"})
		return auth.Session{}, false
	}
	if e := a.admin.Allowed(r.Context(), id.session.Owner, id.session.Hash); e != nil {
		a.adminError(w, r, e)
		return auth.Session{}, false
	}
	return id.session, true
}
func (a *API) adminError(w http.ResponseWriter, r *http.Request, e error) {
	var problem *auth.Error
	if errors.As(e, &problem) {
		id, _ := identity(r)
		if problem.Status == 401 || problem.Status == 403 || problem.Status == 428 {
			a.admin.Security(r.Context(), id.session.Owner, problem.Code)
		}
		respond(w, problem.Status, map[string]string{"code": problem.Code})
		return
	}
	a.log.Warn("administration request failed")
	respond(w, 503, map[string]string{"code": "UNAVAILABLE"})
}
func adminQuery(r *http.Request) admin.Query {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	return admin.Query{Q: q.Get("q"), Status: q.Get("status"), UserID: q.Get("userId"), Cursor: q.Get("cursor"), Limit: limit}
}
func (a *API) adminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminIdentity(w, r); !ok {
		return
	}
	x, e := a.admin.Users(r.Context(), adminQuery(r))
	if e != nil {
		a.adminError(w, r, e)
		return
	}
	respond(w, 200, x)
}
func (a *API) adminUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.adminIdentity(w, r)
	if !ok {
		return
	}
	x, e := a.admin.User(r.Context(), actor.Owner, r.PathValue("id"))
	if e != nil {
		a.adminError(w, r, e)
		return
	}
	respond(w, 200, x)
}
func (a *API) adminSessions(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminIdentity(w, r); !ok {
		return
	}
	x, e := a.admin.Sessions(r.Context(), adminQuery(r))
	if e != nil {
		a.adminError(w, r, e)
		return
	}
	respond(w, 200, x)
}
func (a *API) adminTasks(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminIdentity(w, r); !ok {
		return
	}
	x, e := a.admin.Tasks(r.Context(), adminQuery(r))
	if e != nil {
		a.adminError(w, r, e)
		return
	}
	respond(w, 200, x)
}
func (a *API) adminTask(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminIdentity(w, r); !ok {
		return
	}
	x, e := a.admin.Task(r.Context(), r.PathValue("id"))
	if e != nil {
		a.adminError(w, r, e)
		return
	}
	respond(w, 200, x)
}
func (a *API) adminAudits(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminIdentity(w, r); !ok {
		return
	}
	x, e := a.admin.Audits(r.Context(), adminQuery(r))
	if e != nil {
		a.adminError(w, r, e)
		return
	}
	respond(w, 200, x)
}
func (a *API) adminAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminIdentity(w, r); !ok {
		return
	}
	x, e := a.admin.Audit(r.Context(), r.PathValue("id"))
	if e != nil {
		a.adminError(w, r, e)
		return
	}
	respond(w, 200, x)
}
func (a *API) adminReauth(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.adminIdentity(w, r)
	if !ok {
		return
	}
	var b struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &b) {
		return
	}
	expiry, e := a.auth.ConfirmAdmin(r.Context(), actor, b.Password, a.clientIP(r))
	if e != nil {
		a.adminError(w, r, e)
		return
	}
	respond(w, 200, map[string]int64{"expiresAt": expiry})
}
func (a *API) adminAction(w http.ResponseWriter, r *http.Request, action string) {
	actor, ok := a.adminIdentity(w, r)
	if !ok {
		return
	}
	var b admin.Action
	if !decode(w, r, &b) {
		return
	}
	x, e := a.admin.Act(r.Context(), actor, action, r.PathValue("id"), b)
	if e != nil {
		a.adminError(w, r, e)
		return
	}
	a.auth.CloseSessions(x.Hashes)
	if a.analyses != nil && len(x.TaskIDs) > 0 {
		if e = a.analyses.AdminStopCommitted(r.Context(), x.TaskIDs); e != nil {
			a.log.Warn("committed administration task stop awaits reconciliation")
		}
	}
	respond(w, 200, x)
}
