package store

import "context"

// Callers hold the account-admission transaction lock through job creation.
func analysisOwnerBanned(ctx context.Context, db analysisQuerier, owner string) (bool, error) {
	var banned bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_accounts a WHERE a.status='banned' AND (a.id=$1 OR a.id=(SELECT account_id FROM auth_owner_links WHERE owner_key=$1)))`, owner).Scan(&banned)
	return banned, err
}
