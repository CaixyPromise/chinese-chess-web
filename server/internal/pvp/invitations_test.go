package pvp

import (
	"chinesechess/internal/game"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func approve(t *testing.T, m *Manager, host string, a ApplicationView) View {
	t.Helper()
	v, e := m.Get(host, a.RoomID)
	if e != nil {
		t.Fatal(e)
	}
	v, e = m.Command(context.Background(), host, a.RoomID, Command{RequestID: game.ID(), Version: v.Version, Type: "approve_join", ApplicationID: a.ApplicationID})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestInvitationRequiresApprovalAndKeepsPreviewPrivate(t *testing.T) {
	m, _, _ := setup(t)
	host, guest, third := game.ID(), game.ID(), game.ID()
	v, e := m.Create(context.Background(), host, "black", "房主<甲>")
	if e != nil {
		t.Fatal(e)
	}
	preview, e := m.Preview(guest, v.Invite)
	if e != nil || preview.HostNickname != "房主<甲>" || preview.HostSide != "black" || preview.Status != "available" {
		t.Fatal(preview, e)
	}
	a, e := m.ApplyJoin(guest, v.Invite, "小乙", game.ID())
	if e != nil || a.State != "pending" {
		t.Fatal(a, e)
	}
	before, _ := m.Get(host, v.ID)
	if before.PendingApplication == nil || before.Red.Occupied || before.TurnStarted != 0 {
		t.Fatal("approval bypass", before)
	}
	for _, who := range []string{guest, third} {
		if _, e = m.Get(who, v.ID); !errors.Is(e, game.ErrNotFound) {
			t.Fatal("private room leaked", e)
		}
		if _, _, _, e = m.Watch(who, v.ID); !errors.Is(e, game.ErrNotFound) {
			t.Fatal("private stream leaked", e)
		}
		if _, e = m.Command(context.Background(), who, v.ID, Command{RequestID: game.ID(), Version: a.Version, Type: "ready"}); !errors.Is(e, game.ErrNotFound) {
			t.Fatal("unapproved ready", e)
		}
	}
	other, _ := m.Preview(third, v.Invite)
	if other.Application != nil {
		t.Fatal("other application leaked")
	}
	same, _ := m.ApplyJoin(guest, v.Invite, "小乙", game.ID())
	if same.ApplicationID != a.ApplicationID || same.Version != a.Version {
		t.Fatal("duplicate application mutated", same)
	}
	approved := approve(t, m, host, a)
	guestView, e := m.Get(guest, v.ID)
	if e != nil || guestView.You != "red" || guestView.PendingApplication != nil || guestView.Invite != "" || approved.TurnStarted != 0 {
		t.Fatal(guestView, e)
	}
	command(t, m, guest, v.ID, "ready", "")
	playing := command(t, m, host, v.ID, "ready", "")
	if playing.Phase != "playing" || playing.TurnStarted == 0 {
		t.Fatal(playing)
	}
	preview, _ = m.Preview(third, v.Invite)
	if preview.Status != "started" {
		t.Fatal(preview)
	}
}
func TestWithdrawApproveAndRejectRaceOnlyOneTransition(t *testing.T) {
	for _, action := range []string{"withdraw", "reject_join"} {
		t.Run(action, func(t *testing.T) {
			m, _, _ := setup(t)
			host, guest := game.ID(), game.ID()
			v, _ := m.Create(context.Background(), host, "red")
			a, _ := m.ApplyJoin(guest, v.Invite, "来宾", game.ID())
			var wg sync.WaitGroup
			var wins atomic.Int64
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, e := m.Command(context.Background(), host, v.ID, Command{RequestID: game.ID(), Version: a.Version, Type: "approve_join", ApplicationID: a.ApplicationID})
				if e == nil {
					wins.Add(1)
				}
			}()
			go func() {
				defer wg.Done()
				var e error
				if action == "withdraw" {
					_, e = m.WithdrawApplication(guest, v.Invite, a.ApplicationID, game.ID())
				} else {
					_, e = m.Command(context.Background(), host, v.ID, Command{RequestID: game.ID(), Version: a.Version, Type: action, ApplicationID: a.ApplicationID})
				}
				if e == nil {
					wins.Add(1)
				}
			}()
			wg.Wait()
			if wins.Load() != 1 {
				t.Fatal("transitions", wins.Load())
			}
			p, _ := m.Preview(guest, v.Invite)
			if p.Application == nil {
				t.Fatal(p)
			}
			_, e := m.Get(guest, v.ID)
			if (p.Application.State == "approved") != (e == nil) {
				t.Fatal("seat/state split", p, e)
			}
		})
	}
}
func TestInvitationExpiryRotationAndLegacyLifetimePersist(t *testing.T) {
	m, clock, repo := setup(t)
	host, guest := game.ID(), game.ID()
	v, _ := m.Create(context.Background(), host, "red")
	a, _ := m.ApplyJoin(guest, v.Invite, "来宾", game.ID())
	clock.Add(ApplicationLifetime)
	p, _ := m.Preview(guest, v.Invite)
	if p.Application.State != "expired" {
		t.Fatal(p)
	}
	_, e := m.Command(context.Background(), host, v.ID, Command{RequestID: game.ID(), Version: a.Version, Type: "approve_join", ApplicationID: a.ApplicationID})
	code(t, e, "VERSION_CONFLICT")
	v = command(t, m, host, v.ID, "rotate_invite", "")
	old := p
	old, _ = m.Preview(guest, oldCode(repo, v.ID, v.Invite))
	if old.Status != "revoked" {
		t.Fatal(old)
	}
	clock.Add(InviteLifetime)
	p, _ = m.Preview(guest, v.Invite)
	if p.Status != "expired" {
		t.Fatal(p)
	}
	// Simulate a real pre-upgrade waiting record and prove first-use time survives restart.
	fresh, _ := m.Create(context.Background(), game.ID(), "red")
	g := m.invitationRoom(fresh.Invite)
	g.mu.Lock()
	r := g.record
	r.InviteExpiresAt = 0
	g.record = r
	g.mu.Unlock()
	repo.mu.Lock()
	repo.rows[r.ID], _ = json.Marshal(r)
	repo.mu.Unlock()
	p, e = m.Preview(guest, fresh.Invite)
	if e != nil {
		t.Fatal(e)
	}
	expires := p.ExpiresAt
	clock.Add(10000)
	recovered, e := New(repo, m.rules, clock.Load)
	if e != nil {
		t.Fatal(e)
	}
	defer recovered.Close()
	p, e = recovered.Preview(guest, fresh.Invite)
	if e != nil || p.ExpiresAt != expires {
		t.Fatal("renewed legacy invitation", p, e)
	}
}
func oldCode(repo *repository, id, newCode string) string {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	var r Record
	_ = json.Unmarshal(repo.rows[id], &r)
	for c := range r.PreviousInvites {
		if c != newCode {
			return c
		}
	}
	return ""
}
func TestRotateDoesNotReplaceApprovedSeatAndLimitsNewApplications(t *testing.T) {
	m, clock, _ := setup(t)
	host, guest, third := game.ID(), game.ID(), game.ID()
	v, _ := m.Create(context.Background(), host, "red")
	a, _ := m.ApplyJoin(guest, v.Invite, "来宾", game.ID())
	approve(t, m, host, a)
	v = command(t, m, host, v.ID, "rotate_invite", "")
	if _, e := m.ApplyJoin(third, v.Invite, "第三人", game.ID()); e == nil {
		t.Fatal("approved seat replaced")
	}
	if current, _ := m.Get(guest, v.ID); current.You != "black" {
		t.Fatal(current)
	}
	for i := 0; i < 4; i++ {
		room, _ := m.Create(context.Background(), game.ID(), "red")
		a, e := m.ApplyJoinFrom(third, room.Invite, "来宾", game.ID(), "127.0.0.1")
		if i == 3 {
			code(t, e, "RATE_LIMITED")
		} else {
			if e != nil {
				t.Fatal(e)
			}
			if _, e = m.WithdrawApplication(third, room.Invite, a.ApplicationID, game.ID()); e != nil {
				t.Fatal(e)
			}
		}
	}
	clock.Add(60000)
	room, _ := m.Create(context.Background(), game.ID(), "red")
	if _, e := m.ApplyJoinFrom(third, room.Invite, "来宾", game.ID(), "127.0.0.1"); e != nil {
		t.Fatal(e)
	}
	for _, nickname := range []string{"", "\nfoo", "\t", "abcdefghijklmnopqrstuvwxyz012345678901234"} {
		if _, e := m.ApplyJoin(game.ID(), room.Invite, nickname, game.ID()); e == nil {
			t.Fatalf("accepted nickname %q", nickname)
		}
	}
}

func TestInvitationCancellationSurvivesEvictionAndRejectionIsRecoverable(t *testing.T) {
	m, _, _ := setup(t)
	host, guest, third := game.ID(), game.ID(), game.ID()
	v, _ := m.Create(context.Background(), host, "red")
	invite := v.Invite
	a, e := m.ApplyJoin(guest, invite, "访客", game.ID())
	if e != nil {
		t.Fatal(e)
	}
	v, e = m.Command(context.Background(), host, v.ID, Command{RequestID: game.ID(), Version: a.Version, Type: "reject_join", ApplicationID: a.ApplicationID})
	if e != nil {
		t.Fatal(e)
	}
	p, e := m.Preview(guest, invite)
	if e != nil || p.Application.State != "rejected" {
		t.Fatal(p, e)
	}
	next, e := m.ApplyJoin(third, invite, "朋友", game.ID())
	if e != nil || next.State != "pending" {
		t.Fatal(next, e)
	}
	v = command(t, m, host, v.ID, "leave", "")
	if len(m.roomList()) != 0 {
		t.Fatal("cancelled room retained")
	}
	p, e = m.Preview(third, invite)
	if e != nil || p.Status != "cancelled" || p.Application.State != "cancelled" {
		t.Fatal("archived cancellation lost", p, e)
	}
	if len(m.roomList()) != 0 {
		t.Fatal("preview retained finished room")
	}
}
func TestApplicationIPLimitCannotUseFreshGuestToBypass(t *testing.T) {
	m, _, _ := setup(t)
	for i := 0; i < 11; i++ {
		v, e := m.Create(context.Background(), game.ID(), "red")
		if e != nil {
			t.Fatal(e)
		}
		_, e = m.ApplyJoinFrom(game.ID(), v.Invite, "访客", game.ID(), "192.0.2.1")
		if i < 10 && e != nil {
			t.Fatal(e)
		}
		if i == 10 {
			code(t, e, "RATE_LIMITED")
		}
	}
}

func TestInvitationDeadlineExpiresPendingAndPublishesOnce(t *testing.T) {
	m, clock, _ := setup(t)
	host, guest := game.ID(), game.ID()
	v, _ := m.Create(context.Background(), host, "red")
	clock.Add(InviteLifetime - 1000)
	a, e := m.ApplyJoin(guest, v.Invite, "访客", game.ID())
	if e != nil {
		t.Fatal(e)
	}
	_, updates, stop, e := m.Watch(host, v.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer stop()
	clock.Add(1000)
	p, e := m.Preview(guest, v.Invite)
	if e != nil || p.Status != "expired" || p.Application.State != "expired" {
		t.Fatal(p, e)
	}
	update := <-updates
	if update.InvitationStatus != "expired" || update.PendingApplication != nil || update.Version <= a.Version {
		t.Fatal("deadline not published", update)
	}
	again, e := m.Get(host, v.ID)
	if e != nil || again.Version != update.Version {
		t.Fatal("expiry committed twice", again, e)
	}
}
