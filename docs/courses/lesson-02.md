# Lesson 2: Database Adapters -- PostgreSQL and SQLite

## Learning Objectives

- Implement the Adapter pattern to wrap third-party database drivers behind a unified interface
- Understand the PostgreSQL adapter's pgx/v5 wrapping chain and connection pool sizing
- Understand the SQLite adapter's pure-Go approach with `modernc.org/sqlite`

## Key Concepts

- **Adapter Pattern**: Each driver package (`postgres`, `sqlite`) provides a `Client` struct that implements `database.Database`. Internally, each wraps driver-specific types into the shared interfaces.
- **Wrapping Chain**: PostgreSQL wraps `pgconn.CommandTag` into `pgResult`, `pgx.Row` into `pgRow`, `pgx.Rows` into `pgRows`, and `pgx.Tx` into `pgTx`. SQLite wraps `sql.Result`, `*sql.Row`, `*sql.Rows`, and `*sql.Tx` into their respective interface wrappers.
- **Factory Configuration**: `postgres.DefaultConfig()` dynamically sizes the connection pool based on `runtime.NumCPU()` -- `MaxConns = min(max(CPU*2 + 1, 10), 50)`.
- **Pure Go SQLite**: Using `modernc.org/sqlite` eliminates CGO, simplifying cross-compilation and container builds.

## Code Walkthrough

### Source: `pkg/postgres/postgres.go`

The PostgreSQL client wraps `pgxpool.Pool` and translates every pgx-specific type to the module's interfaces:

```go
type Client struct {
    pool   *pgxpool.Pool
    config *database.Config
}
```

The `Connect` method builds a `pgxpool.Config` from the module's `Config`, sets pool sizes, timeouts, and health check period, then calls `pgxpool.NewWithConfig`. The wrapping chain translates pgx results:

- `pgResult` wraps `pgconn.CommandTag` to satisfy `database.Result`
- `pgRow` wraps `pgx.Row` to satisfy `database.Row`
- `pgRows` wraps `pgx.Rows` to satisfy `database.Rows`
- `pgTx` wraps `pgx.Tx` to satisfy `database.Tx`

### Source: `pkg/sqlite/sqlite.go`

The SQLite client wraps the standard `database/sql` package with the pure-Go `modernc.org/sqlite` driver:

```go
type Client struct {
    db     *sql.DB
    config *database.Config
}
```

SQLite uses `MaxOpenConns=1` for WAL mode safety. The wrapping chain is simpler since `database/sql` types are closer to the module interfaces.

### Source: `pkg/dialect/dialect.go`

The dialect package handles SQL differences between PostgreSQL and SQLite, such as placeholder rewriting (`?` to `$1, $2, ...`) and boolean literal translation.

### Source: `pkg/postgres/postgres_test.go` and `pkg/sqlite/sqlite_test.go`

Unit tests verify connection configuration, health checks, and the wrapping chain. Integration tests (tagged) verify actual database operations.

## Practice Exercise

1. Read both `pkg/postgres/postgres.go` and `pkg/sqlite/sqlite.go`. List every struct that wraps a third-party type and the interface it satisfies.
2. Trace the `Query` method from `Client.Query` through the wrapping chain to the underlying driver call and back. Draw the call sequence.
3. Create a test that uses `sqlite.Client` with an in-memory database (`:memory:`) to execute a CREATE TABLE, INSERT, and SELECT, verifying the results through the `database.Rows` interface.
