package integration

import (
	"net/url"
	"strings"
)

// pgx.ConnConfig.ConnString returns its original input, not edited RuntimeParams.
func schemaDSN(dsn, schema string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, _ := url.Parse(dsn)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		return u.String()
	}
	return dsn + " search_path=" + schema
}
