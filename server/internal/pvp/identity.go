package pvp

func (m *Manager) sameOwner(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if repo, ok := m.repo.(interface{ SameOwner(string, string) bool }); ok {
		return repo.SameOwner(a, b)
	}
	return a == b
}
func (m *Manager) ownsSide(r Record, owner, seat string) bool {
	if seat == "red" {
		return m.sameOwner(r.RedOwner, owner)
	}
	return m.sameOwner(r.BlackOwner, owner)
}
func (m *Manager) side(r Record, owner string) string {
	red, black := m.sameOwner(r.RedOwner, owner), m.sameOwner(r.BlackOwner, owner)
	if red && black {
		return "both"
	}
	if red {
		return "red"
	}
	if black {
		return "black"
	}
	return ""
}
func (m *Manager) view(r Record, owner string, now int64) View {
	you := m.side(r, owner)
	if you == "" {
		return View{State: State{ID: r.ID, Version: r.Version, Phase: "left", Moves: []string{}, Legal: []string{}}, ServerNow: now}
	}
	v := View{State: r.State, You: you, ServerNow: now, HostNickname: hostNickname(r), ParticipantNickname: r.ParticipantNickname}
	if you == "both" {
		v.You = ""
		v.ViewChoices = []string{"red", "black"}
	}
	if m.sameOwner(owner, r.Host) {
		v.InvitationStatus = m.invitationStatus(r, r.Invite)
		v.InviteExpiresAt = r.InviteExpiresAt
		if v.InvitationStatus == "available" {
			v.Invite = r.Invite
		}
		if a, ok := r.Applications[r.CurrentApplication]; ok && a.State == "pending" {
			av := applicationView(r, a)
			v.PendingApplication = &av
		}
	}
	return v
}

func hostNickname(r Record) string {
	if r.HostNickname == "" {
		return "房主"
	}
	return r.HostNickname
}
