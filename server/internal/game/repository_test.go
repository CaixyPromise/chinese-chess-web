package game

import (
	"chinesechess/internal/history"
	"context"
	"encoding/json"
	"sort"
)

func (r *memoryRepo) FindGame(_ context.Context, owner, id string) (Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var record Record
	b, ok := r.rows[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	if err := json.Unmarshal(b, &record); err != nil {
		return Record{}, err
	}
	if record.Owner != owner {
		return Record{}, ErrNotFound
	}
	return record, nil
}

func (r *memoryRepo) RecentGames(_ context.Context, owner string) ([]Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Record{}
	for _, b := range r.rows {
		var record Record
		if err := json.Unmarshal(b, &record); err != nil {
			return nil, err
		}
		if record.Owner == owner {
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

// Production history SQL is covered separately by real PostgreSQL integration tests.
func (r *memoryRepo) History(_ context.Context, _ string, q history.Query) (history.Page, error) {
	if err := q.Validate(); err != nil {
		return history.Page{}, err
	}
	return history.Page{Items: []history.Entry{}}, nil
}
