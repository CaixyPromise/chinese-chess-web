package integration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"chinesechess/internal/admin"
	"chinesechess/internal/auth"
	"chinesechess/internal/game"
	"chinesechess/internal/store"
)

const adminPassword = "a strong admin chess password"

func seedAdminAccount(t *testing.T, service *auth.Service, repo *store.Postgres, role string) auth.Session {
	t.Helper()
	ctx := context.Background()
	guest := game.ID()
	grant := base64.RawURLEncoding.EncodeToString([]byte(game.ID()))
	email := game.ID() + "@example.test"
	// A grant represents an already verified email ownership proof, independent
	// from SMTP transport (covered by the existing mail integration suite).
	_, e := repo.AuthDB().Exec(ctx, `INSERT INTO auth_registration_grants(token_hash,email,guest_owner,expires_at) VALUES($1,$2,$3,$4)`, auth.Hash(grant), email, guest, time.Now().Add(time.Minute).UnixMilli())
	if e != nil {
		t.Fatal(e)
	}
	session, e := service.CompleteRegister(ctx, grant, guest, adminPassword, "管理员测试")
	if e != nil {
		t.Fatal(e)
	}
	if role == "admin" {
		_, e = admin.New(repo.AuthDB()).SetRole(ctx, session.Owner, "admin", admin.Action{Reason: "本地初始化", RequestID: game.ID(), Version: 1})
		if e != nil {
			t.Fatal(e)
		}
	}
	return session
}
func loginAdminBrowser(t *testing.T, b *authBrowser, session auth.Session) {
	t.Helper()
	b.call(http.MethodPost, "/api/auth/login", map[string]any{"email": session.User.Email, "password": adminPassword}, 200)
}
func TestAdminHTTPPermissionsReauthenticationAndPrivateProjection(t *testing.T) {
	service, a, b, repo := setupAuthHTTP(t)
	operator := seedAdminAccount(t, service, repo, "admin")
	target := seedAdminAccount(t, service, repo, "user")
	loginAdminBrowser(t, a, operator)
	loginAdminBrowser(t, b, target)
	for _, crossSite := range []bool{false, true} {
		var before, after int
		if e := repo.AuthDB().QueryRow(context.Background(), `SELECT count(*) FROM admin_security_events WHERE code='FORBIDDEN' AND actor_id=''`).Scan(&before); e != nil {
			t.Fatal(e)
		}
		req, _ := http.NewRequest("POST", a.base+"/api/admin/reauth", strings.NewReader(`{"password":"never-record-request-body"}`))
		req.Header.Set("X-CSRF-Token", a.csrf)
		if crossSite {
			req.Header.Set("Origin", a.base)
			req.Header.Set("Sec-Fetch-Site", "cross-site")
		} else {
			req.Header.Set("Origin", "https://untrusted.example.test")
		}
		response, e := a.client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != 403 {
			t.Fatal("cross-origin request admitted")
		}
		if e = repo.AuthDB().QueryRow(context.Background(), `SELECT count(*) FROM admin_security_events WHERE code='FORBIDDEN' AND actor_id=''`).Scan(&after); e != nil || after != before+1 {
			t.Fatalf("denied origin not recorded: %d -> %d: %v", before, after, e)
		}
	}

	b.call("GET", "/api/admin/users", nil, 403)
	b.call("GET", "/api/admin/users/"+game.ID(), nil, 403)
	users := a.call("GET", "/api/admin/users?limit=1", nil, 200)
	items := users["items"].([]any)
	if len(items) != 1 || !strings.Contains(items[0].(map[string]any)["email"].(string), "***") {
		t.Fatal("user list leaked full email")
	}
	if users["nextCursor"] == "" {
		t.Fatal("missing keyset cursor")
	}
	a.call("GET", "/api/admin/users?cursor=invalid", nil, 400)
	detail := a.call("GET", "/api/admin/users/"+target.Owner, nil, 200)
	if detail["email"] != target.User.Email {
		t.Fatal("detail did not expose requested identity")
	}
	action := map[string]any{"reason": "处理异常账号", "requestId": game.ID(), "version": 1}
	a.call("POST", "/api/admin/users/"+target.Owner+"/ban", action, 428)
	a.call("POST", "/api/admin/reauth", map[string]any{"password": "wrong"}, 401)
	a.call("POST", "/api/admin/reauth", map[string]any{"password": adminPassword}, 200)
	sessions := a.call("GET", "/api/admin/sessions?userId="+target.Owner, nil, 200)
	raw, _ := json.Marshal(sessions)
	if strings.Contains(string(raw), "token") || strings.Contains(string(raw), target.Hash) {
		t.Fatal("credential material in session projection")
	}
	// Reauthentication is bound to the exact session, including after login again.
	a.call("POST", "/api/auth/login", map[string]any{"email": operator.User.Email, "password": adminPassword}, 200)
	a.call("POST", "/api/admin/users/"+target.Owner+"/ban", action, 428)
	a.call("POST", "/api/admin/reauth", map[string]any{"password": adminPassword}, 200)
	protected := map[string]any{"reason": "保护管理员", "requestId": game.ID(), "version": 2}
	a.call("POST", "/api/admin/users/"+operator.Owner+"/ban", protected, 409)
	// Expiry removes permission without changing the browser principal.
	if _, e := repo.AuthDB().Exec(context.Background(), `UPDATE auth_sessions SET admin_confirmed_at=$2 WHERE owner_key=$1`, operator.Owner, time.Now().Add(-11*time.Minute).UnixMilli()); e != nil {
		t.Fatal(e)
	}
	a.call("POST", "/api/admin/users/"+target.Owner+"/ban", action, 428)
	a.call("POST", "/api/admin/reauth", map[string]any{"password": adminPassword}, 200)
	// Revoke the user's exact display session; a second identical request is safe.
	sessionItems := sessions["items"].([]any)
	var live map[string]any
	for _, item := range sessionItems {
		row := item.(map[string]any)
		if row["revokedAt"].(float64) == 0 {
			live = row
			break
		}
	}
	if live == nil {
		t.Fatal("no active target session")
	}
	revoke := map[string]any{"reason": "撤销单一会话", "requestId": game.ID(), "version": live["version"]}
	a.call("POST", "/api/admin/sessions/"+live["id"].(string)+"/revoke", revoke, 200)
	a.call("POST", "/api/admin/sessions/"+live["id"].(string)+"/revoke", revoke, 200)
	audits := a.call("GET", "/api/admin/audit?q=user.view", nil, 200)
	auditItems := audits["items"].([]any)
	if len(auditItems) != 1 {
		t.Fatal("sensitive user detail was not audited")
	}
	auditRaw, _ := json.Marshal(auditItems)
	if strings.Contains(string(auditRaw), target.User.Email) {
		t.Fatal("audit stores unnecessary full email")
	}

	// A current role change invalidates already established browser authorization.
	_, e := repo.AuthDB().Exec(context.Background(), `UPDATE auth_accounts SET role='user' WHERE id=$1`, operator.Owner)
	if e != nil {
		t.Fatal(e)
	}
	a.call("GET", "/api/admin/users", nil, 403)
}
func TestAdminBanAtomicAliasesTokensTasksAndIdempotency(t *testing.T) {
	service, a, b, repo := setupAuthHTTP(t)
	operator := seedAdminAccount(t, service, repo, "admin")
	target := seedAdminAccount(t, service, repo, "user")
	loginAdminBrowser(t, a, operator)
	loginAdminBrowser(t, b, target)
	a.call("POST", "/api/admin/reauth", map[string]any{"password": adminPassword}, 200)
	ctx := context.Background()
	var alias string
	if e := repo.AuthDB().QueryRow(ctx, `SELECT owner_key FROM auth_owner_links WHERE account_id=$1 AND owner_key<>$1 LIMIT 1`, target.Owner).Scan(&alias); e != nil {
		t.Fatal(e)
	}
	job := analysisJob(alias)
	job.State = "running"
	job.Budget.Millis = 100
	if _, e := repo.CreateAnalysis(ctx, job); e != nil {
		t.Fatal(e)
	}
	projection := a.call("GET", "/api/admin/tasks/"+job.ID, nil, 200)
	projectionRaw, _ := json.Marshal(projection)
	for _, private := range []string{"source", "moves", "fen", "positions", "sampleId", job.Source.ID} {
		if strings.Contains(string(projectionRaw), private) {
			t.Fatalf("task projection leaked %s", private)
		}
	}

	reset := proofToken(252)
	_, e := repo.AuthDB().Exec(ctx, `INSERT INTO auth_tokens(token_hash,purpose,email,account_id,expires_at) VALUES($1,'reset',$2,$3,$4)`, auth.Hash(reset), target.User.Email, target.Owner, time.Now().Add(time.Hour).UnixMilli())
	if e != nil {
		t.Fatal(e)
	}
	action := map[string]any{"reason": "异常任务封禁", "requestId": game.ID(), "version": 1}
	result := a.call("POST", "/api/admin/users/"+target.Owner+"/ban", action, 200)
	again := a.call("POST", "/api/admin/users/"+target.Owner+"/ban", action, 200)
	if result["version"] != again["version"] {
		t.Fatal("idempotency changed state")
	}
	b.call("GET", "/api/admin/users", nil, 401)
	_, e = service.Login(ctx, target.User.Email, adminPassword, "", "test", false)
	authCode(t, e, "INVALID_CREDENTIALS")
	if _, e = service.NewSession(ctx, alias); !errors.Is(e, auth.ErrSession) {
		t.Fatalf("linked guest revived: %v", e)
	}
	e = service.ResetPassword(ctx, reset, adminPassword)
	authCode(t, e, "INVALID_TOKEN")
	persisted, e := repo.FindAnalysis(ctx, target.Owner, job.ID)
	if e != nil || persisted.State != "paused" || persisted.Generation != 2 {
		t.Fatalf("alias task not paused: %+v %v", persisted, e)
	}
	assertAnalysisConflict(t, repo.SaveAnalysis(ctx, job, 1, "running"))
	var count int
	if e = repo.AuthDB().QueryRow(ctx, `SELECT count(*) FROM admin_audit WHERE actor_id=$1 AND request_id=$2`, operator.Owner, action["requestId"]).Scan(&count); e != nil || count != 1 {
		t.Fatal("repeated action audit was not exactly once")
	}
	// Unban does not resurrect credentials revoked by ban.
	a.call("POST", "/api/admin/users/"+target.Owner+"/unban", map[string]any{"reason": "完成复核", "requestId": game.ID(), "version": 2}, 200)
	b.call("GET", "/api/admin/users", nil, 401)
}
func TestAdminAuditRollbackAndConcurrentVersions(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	service, e := auth.New(repo.AuthDB(), auth.Config{PublicURL: "http://127.0.0.1:8765", SMTPAddr: "127.0.0.1:1025", SMTPFrom: "chess@example.test"}, repo.ReloadIdentity)
	if e != nil {
		t.Fatal(e)
	}
	actor := seedAdminAccount(t, service, repo, "admin")
	target := seedAdminAccount(t, service, repo, "user")
	if _, e = service.ConfirmAdmin(ctx, actor, adminPassword, "test"); e != nil {
		t.Fatal(e)
	}
	svc := admin.New(repo.AuthDB())
	job := analysisJob(target.Owner)
	if _, e = repo.CreateAnalysis(ctx, job); e != nil {
		t.Fatal(e)
	}
	_, e = conn.Exec(ctx, `CREATE FUNCTION reject_admin_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$; CREATE TRIGGER reject_admin_audit BEFORE INSERT ON admin_audit FOR EACH ROW EXECUTE FUNCTION reject_admin_audit()`)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Act(ctx, actor, "task.pause", job.ID, admin.Action{Reason: "暂停异常任务", RequestID: game.ID(), Version: 1}); e == nil {
		t.Fatal("audit failure accepted action")
	}
	persisted, e := repo.FindAnalysis(ctx, target.Owner, job.ID)
	if e != nil || persisted.State != "queued" || persisted.Generation != 1 {
		t.Fatalf("audit rollback lost: %+v %v", persisted, e)
	}
	if _, e = conn.Exec(ctx, `DROP TRIGGER reject_admin_audit ON admin_audit; DROP FUNCTION reject_admin_audit()`); e != nil {
		t.Fatal(e)
	}
	paused, e := svc.Act(ctx, actor, "task.pause", job.ID, admin.Action{Reason: "审计恢复后暂停", RequestID: game.ID(), Version: 1})
	if e != nil || paused.Status != "paused" || paused.Version != 2 {
		t.Fatalf("queued pause: %+v %v", paused, e)
	}
	assertAnalysisConflict(t, repo.SaveAnalysis(ctx, job, 1, "queued"))
	completed := analysisJob(target.Owner)
	completed.ID = game.ID()
	completed.Digest = game.ID()
	completed.State = "completed"
	completed.Generation = 4
	if _, e = repo.CreateAnalysis(ctx, completed); e != nil {
		t.Fatal(e)
	}
	natural, e := svc.Act(ctx, actor, "task.pause", completed.ID, admin.Action{Reason: "完成竞争检查", RequestID: game.ID(), Version: 1})
	if e != nil || natural.Status != "completed" || natural.Version != 4 {
		t.Fatalf("natural completion overwritten: %+v %v", natural, e)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Act(ctx, actor, "user.revoke-sessions", target.Owner, admin.Action{Reason: "并发撤销", RequestID: game.ID(), Version: 1})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			authCode(t, err, "VERSION_CONFLICT")
		}
	}
	if success != 1 {
		t.Fatalf("expected one committed version, got %d", success)
	}
}
func TestAdminConcurrentLastAdminProtectionAndAudit(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	service, e := auth.New(repo.AuthDB(), auth.Config{PublicURL: "http://127.0.0.1:8765", SMTPAddr: "127.0.0.1:1025", SMTPFrom: "chess@example.test"}, repo.ReloadIdentity)
	if e != nil {
		t.Fatal(e)
	}
	first := seedAdminAccount(t, service, repo, "admin")
	second := seedAdminAccount(t, service, repo, "admin")
	svc := admin.New(repo.AuthDB())
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []string{first.Owner, second.Owner} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := svc.SetRole(ctx, id, "user", admin.Action{Reason: "本地撤销角色", RequestID: game.ID(), Version: 2})
			errs <- err
		}(id)
	}
	wg.Wait()
	close(errs)
	ok := 0
	for err := range errs {
		if err == nil {
			ok++
		} else {
			authCode(t, err, "LAST_ADMIN")
		}
	}
	if ok != 1 {
		t.Fatalf("expected one revoked role, got %d", ok)
	}
	var count int
	if e = repo.AuthDB().QueryRow(ctx, `SELECT count(*) FROM auth_accounts WHERE role='admin' AND status='active'`).Scan(&count); e != nil || count != 1 {
		t.Fatal(fmt.Sprintf("last admin count %d: %v", count, e))
	}
}

func TestAdminCLIRoleRequestIdempotency(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	service, e := auth.New(repo.AuthDB(), auth.Config{PublicURL: "http://127.0.0.1:8765", SMTPAddr: "127.0.0.1:1025", SMTPFrom: "chess@example.test"}, repo.ReloadIdentity)
	if e != nil {
		t.Fatal(e)
	}
	target := seedAdminAccount(t, service, repo, "user")
	svc := admin.New(repo.AuthDB())
	request := admin.Action{Reason: "本地角色提升", RequestID: game.ID(), Version: 1}
	first, e := svc.SetRole(ctx, target.Owner, "admin", request)
	if e != nil {
		t.Fatal(e)
	}
	// A CLI retry fetches the now-current target version before confirmation.
	request.Version = 2
	again, e := svc.SetRole(ctx, target.Owner, "admin", request)
	if e != nil || first.Version != again.Version {
		t.Fatalf("CLI request replay: %+v %v", again, e)
	}
	request.Reason = "different intent"
	_, e = svc.SetRole(ctx, target.Owner, "admin", request)
	authCode(t, e, "REQUEST_CONFLICT")
}
