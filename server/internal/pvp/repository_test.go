package pvp

import (
	"chinesechess/internal/game"
	"context"
	"encoding/json"
	"sort"
)

func (r *repository) FindPVP(_ context.Context, owner, id string) (Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.rows[id]
	if !ok {
		return Record{}, game.ErrNotFound
	}
	var record Record
	if err := json.Unmarshal(b, &record); err != nil {
		return Record{}, err
	}
	if side(record, owner) == "" {
		return Record{}, game.ErrNotFound
	}
	return record, nil
}

func (r *repository) RecentPVP(_ context.Context, owner string) ([]Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Record{}
	for _, b := range r.rows {
		var record Record
		if err := json.Unmarshal(b, &record); err != nil {
			return nil, err
		}
		if side(record, owner) != "" {
			out = append(out, record)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Updated == out[j].Updated {
			return out[i].ID > out[j].ID
		}
		return out[i].Updated > out[j].Updated
	})
	if len(out) > 8 {
		out = out[:8]
	}
	return out, nil
}

func (r *repository) FindPVPInvitation(_ context.Context, code string) (Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.rows {
		var record Record
		if err := json.Unmarshal(b, &record); err != nil {
			return Record{}, err
		}
		_, old := record.PreviousInvites[code]
		if record.Invite == code || old {
			return record, nil
		}
	}
	return Record{}, game.ErrNotFound
}

// Legacy memory fixtures intentionally have no identity-link support.
func side(r Record, owner string) string {
	if r.RedOwner == owner {
		return "red"
	}
	if r.BlackOwner == owner {
		return "black"
	}
	return ""
}
