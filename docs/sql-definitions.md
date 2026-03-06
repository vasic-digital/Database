# SQL Definitions

## Migration Tracking Table

Created automatically by `migration.Runner.Init()`:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TIMESTAMP NOT NULL
);
```

The table name is configurable via `NewRunner(db, "custom_table_name")`.

## Dialect Transformations

The `dialect` package rewrites SQL queries transparently. Below are the transformation rules.

### Placeholder Rewriting (PostgreSQL)

| SQLite | PostgreSQL |
|--------|-----------|
| `SELECT * FROM users WHERE id = ? AND active = ?` | `SELECT * FROM users WHERE id = $1 AND active = $2` |

Placeholders inside single-quoted strings are not rewritten.

### INSERT OR IGNORE (PostgreSQL)

| SQLite | PostgreSQL |
|--------|-----------|
| `INSERT OR IGNORE INTO tags (name) VALUES (?)` | `INSERT INTO tags (name) VALUES ($1) ON CONFLICT DO NOTHING` |

### Boolean Literals (PostgreSQL)

For registered boolean columns:

| SQLite | PostgreSQL |
|--------|-----------|
| `WHERE active = 1` | `WHERE active = TRUE` |
| `WHERE active = 0` | `WHERE active = FALSE` |

### DDL Helpers

| Method | SQLite | PostgreSQL |
|--------|--------|-----------|
| `AutoIncrement()` | `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` |
| `TimestampType()` | `DATETIME` | `TIMESTAMP` |
| `BooleanDefault(true)` | `DEFAULT 1` | `DEFAULT TRUE` |
| `BooleanDefault(false)` | `DEFAULT 0` | `DEFAULT FALSE` |
| `CurrentTimestamp()` | `CURRENT_TIMESTAMP` | `CURRENT_TIMESTAMP` |

## Query Builder Output

The `query` package generates parameterized SQL with `?` placeholders. Apply `dialect.RewriteAll()` before executing against PostgreSQL.

```go
q, args := query.New().
    Select("id", "title", "year").
    From("media_items").
    Where(query.Eq("type", "movie")).
    Where(query.Gte("year", 2000)).
    OrderBy("title ASC").
    Limit(20).
    Build()
// q    = "SELECT id, title, year FROM media_items WHERE type = ? AND year >= ? ORDER BY title ASC LIMIT 20"
// args = ["movie", 2000]
```

## Connection Pool Metrics

The `pool` package tracks these internal counters (no table — in-memory):

| Metric | Description |
|--------|-------------|
| `TotalConnections` | Current number of open connections |
| `IdleConnections` | Connections available for reuse |
| `ActiveConnections` | Connections currently in use |
| `WaitCount` | Total times a caller waited for a connection |
| `MaxLifetimeClosures` | Connections closed due to max lifetime |
| `MaxIdleClosures` | Connections closed due to idle timeout |

## Repository Pattern

The `repository` package works with any table via `EntityMapper[T]`:

```go
type EntityMapper[T any] interface {
    TableName() string
    Columns() []string
    ScanRow(row database.Row) (T, error)
    InsertSQL() string
    UpdateSQL() string
}
```

The generated SQL follows the pattern:
- **Create**: Uses `InsertSQL()` (custom INSERT statement)
- **GetByID**: `SELECT {columns} FROM {table} WHERE id = ?`
- **Update**: Uses `UpdateSQL()` (custom UPDATE statement)
- **Delete**: `DELETE FROM {table} WHERE id = ?`
- **List**: `SELECT {columns} FROM {table} LIMIT ? OFFSET ?`
- **Count**: `SELECT COUNT(*) FROM {table}`
