package store

import "context"

// ReloadIdentity publishes a complete committed ownership snapshot. Callers
// serialize claims with HTTP authorization; resource facts remain unchanged.
func (p *Postgres) ReloadIdentity(ctx context.Context) error {
	rows, err := p.pool.Query(ctx, `SELECT owner_key,account_id FROM auth_owner_links`)
	if err != nil {
		return err
	}
	defer rows.Close()
	next := map[string]string{}
	for rows.Next() {
		var source, account string
		if err = rows.Scan(&source, &account); err != nil {
			return err
		}
		next[source] = account
	}
	if err = rows.Err(); err != nil {
		return err
	}
	p.identityMu.Lock()
	p.identities = next
	p.identityMu.Unlock()
	return nil
}

func (p *Postgres) SameOwner(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	p.identityMu.RLock()
	defer p.identityMu.RUnlock()
	ca, cb := a, b
	if v := p.identities[a]; v != "" {
		ca = v
	}
	if v := p.identities[b]; v != "" {
		cb = v
	}
	return ca == cb
}

// SQL authorization accepts a principal's own source or an account's linked
// sources. Revoked guest credentials are rejected by the session layer.
const ownedBy1 = `(owner_key=$1 OR owner_key IN (SELECT owner_key FROM auth_owner_links WHERE account_id=$1))`
const ownedBy2 = `(owner_key=$2 OR owner_key IN (SELECT owner_key FROM auth_owner_links WHERE account_id=$2))`
