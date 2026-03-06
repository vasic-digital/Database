// Package dialect provides cross-database SQL compatibility helpers.
//
// It supports SQLite and PostgreSQL dialects, automatically rewriting
// queries for placeholder syntax, INSERT OR IGNORE, boolean literals,
// and DDL differences (auto-increment, timestamp types).
//
// Design patterns: Strategy (SQLite vs PostgreSQL behavior).
package dialect

import (
	"fmt"
	"regexp"
	"strings"
)

// Type identifies the SQL dialect in use.
type Type string

const (
	SQLite   Type = "sqlite"
	Postgres Type = "postgres"
)

// Dialect provides helpers for cross-database SQL compatibility.
type Dialect struct {
	Type Type
}

// New creates a new Dialect for the given type.
func New(t Type) *Dialect {
	return &Dialect{Type: t}
}

// RewritePlaceholders converts ? placeholders to $1, $2, ... for PostgreSQL.
// SQLite queries are returned unchanged. Placeholders inside single-quoted
// strings are left untouched.
func (d *Dialect) RewritePlaceholders(query string) string {
	if d.Type != Postgres {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 32)
	n := 0
	inSingleQuote := false
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '\'' {
			inSingleQuote = !inSingleQuote
			b.WriteByte(ch)
			continue
		}
		if ch == '?' && !inSingleQuote {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// RewriteInsertOrIgnore converts "INSERT OR IGNORE INTO ..." to
// "INSERT INTO ... ON CONFLICT DO NOTHING" for PostgreSQL.
func (d *Dialect) RewriteInsertOrIgnore(query string) string {
	if d.Type != Postgres {
		return query
	}
	upper := strings.ToUpper(query)
	if idx := strings.Index(upper, "INSERT OR IGNORE INTO"); idx != -1 {
		prefix := query[:idx]
		rest := query[idx+len("INSERT OR IGNORE INTO"):]
		return prefix + "INSERT INTO" + rest + " ON CONFLICT DO NOTHING"
	}
	return query
}

// RewriteInsertOrReplace converts "INSERT OR REPLACE INTO ..." to
// PostgreSQL-compatible syntax.
func (d *Dialect) RewriteInsertOrReplace(query string) string {
	if d.Type != Postgres {
		return query
	}
	upper := strings.ToUpper(query)
	if idx := strings.Index(upper, "INSERT OR REPLACE INTO"); idx != -1 {
		prefix := query[:idx]
		rest := query[idx+len("INSERT OR REPLACE INTO"):]
		return prefix + "INSERT INTO" + rest
	}
	return query
}

// AutoIncrement returns the auto-increment primary key clause.
func (d *Dialect) AutoIncrement() string {
	if d.Type == Postgres {
		return "SERIAL PRIMARY KEY"
	}
	return "INTEGER PRIMARY KEY AUTOINCREMENT"
}

// TimestampType returns the column type for timestamps.
func (d *Dialect) TimestampType() string {
	if d.Type == Postgres {
		return "TIMESTAMP"
	}
	return "DATETIME"
}

// BooleanDefault returns the default boolean value syntax.
func (d *Dialect) BooleanDefault(val bool) string {
	if d.Type == Postgres {
		if val {
			return "DEFAULT TRUE"
		}
		return "DEFAULT FALSE"
	}
	if val {
		return "DEFAULT 1"
	}
	return "DEFAULT 0"
}

// CurrentTimestamp returns the current timestamp expression.
func (d *Dialect) CurrentTimestamp() string {
	return "CURRENT_TIMESTAMP"
}

// IsSQLite returns true if the dialect is SQLite.
func (d *Dialect) IsSQLite() bool {
	return d.Type == SQLite
}

// IsPostgres returns true if the dialect is PostgreSQL.
func (d *Dialect) IsPostgres() bool {
	return d.Type == Postgres
}

// RewriteBooleanLiterals converts "column = 0" to "column = FALSE" and
// "column = 1" to "column = TRUE" for the specified boolean columns in
// PostgreSQL. SQLite queries are returned unchanged.
func (d *Dialect) RewriteBooleanLiterals(query string, boolColumns []string) string {
	if d.Type != Postgres || len(boolColumns) == 0 {
		return query
	}
	pattern := regexp.MustCompile(
		`(?i)\b(` + strings.Join(boolColumns, "|") + `)\s*=\s*([01])\b`)
	return pattern.ReplaceAllStringFunc(query, func(match string) string {
		if strings.HasSuffix(strings.TrimSpace(match), "1") {
			return pattern.ReplaceAllString(match, "${1} = TRUE")
		}
		return pattern.ReplaceAllString(match, "${1} = FALSE")
	})
}

// RewriteAll applies all dialect-specific query transformations:
// placeholder rewriting, INSERT OR IGNORE, INSERT OR REPLACE,
// and boolean literal rewriting.
func (d *Dialect) RewriteAll(query string, boolColumns []string) string {
	query = d.RewritePlaceholders(query)
	if d.IsPostgres() {
		query = d.RewriteInsertOrIgnore(query)
		query = d.RewriteInsertOrReplace(query)
		query = d.RewriteBooleanLiterals(query, boolColumns)
	}
	return query
}
