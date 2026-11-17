package store

import "github.com/jackc/pgx/v5/pgxpool"

// AuthDB shares the transaction pool; authentication owns its separate tables.
func (p *Postgres) AuthDB() *pgxpool.Pool { return p.pool }
