package store

import (
	"chinesechess/internal/analysis"
	"context"
)

// A historical task keeps its original seat provenance after account linking.
// Batch the projection rather than querying once for every task in a list.
func (p *Postgres) analysisSides(ctx context.Context, jobs []analysis.Job) error {
	ids := []string{}
	seen := map[string]bool{}
	for i := range jobs {
		jobs[i].ViewerSide = ""
		if jobs[i].Source.Mode == "pvp" && !seen[jobs[i].Source.ID] {
			ids = append(ids, jobs[i].Source.ID)
			seen[jobs[i].Source.ID] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := p.pool.Query(ctx, `SELECT id,red_owner,black_owner FROM pvp_rooms WHERE id=ANY($1::text[])`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	type seats struct{ red, black string }
	owners := map[string]seats{}
	for rows.Next() {
		var id string
		var s seats
		if err = rows.Scan(&id, &s.red, &s.black); err != nil {
			return err
		}
		owners[id] = s
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for i := range jobs {
		if jobs[i].Source.Mode != "pvp" {
			continue
		}
		s := owners[jobs[i].Source.ID]
		if jobs[i].Owner == s.red && jobs[i].Owner != s.black {
			jobs[i].ViewerSide = "red"
		}
		if jobs[i].Owner == s.black && jobs[i].Owner != s.red {
			jobs[i].ViewerSide = "black"
		}
	}
	return nil
}
