package utils

import (
	"net/url"
	"strings"
)

// SQLiteDSN builds a modernc.org/sqlite DSN that applies each pragma on every
// connection the database/sql pool opens. Two pitfalls make this helper
// necessary:
//
//   - modernc.org/sqlite silently ignores mattn/go-sqlite3 style parameters
//     such as "_busy_timeout=5000"; it only honors "_pragma=name(value)".
//   - Per-connection pragmas (busy_timeout, foreign_keys, ...) issued via
//     db.Exec reach a single pooled connection, leaving every other
//     connection on SQLite defaults.
//
// Pragmas are given in SQLite call syntax, e.g. "busy_timeout(5000)".
func SQLiteDSN(dbPath string, pragmas ...string) string {
	if len(pragmas) == 0 {
		return dbPath
	}

	var dsn strings.Builder
	dsn.WriteString(dbPath)
	separator := "?"
	for _, pragma := range pragmas {
		dsn.WriteString(separator)
		dsn.WriteString("_pragma=")
		dsn.WriteString(url.QueryEscape(pragma))
		separator = "&"
	}
	return dsn.String()
}
