# Architecture - Database Module

## Overview

The `digital.vasic.database` module follows a layered architecture that separates interface contracts from concrete implementations. The module is organized into seven packages, each with a single responsibility.

```
pkg/
  database/    Core interfaces and configuration (no external deps)
  postgres/    PostgreSQL adapter (pgx/v5, pgxpool)
  sqlite/      SQLite adapter (modernc.org/sqlite, database/sql)
  pool/        Generic connection pool with metrics
  migration/   Schema migration runner with version tracking
  repository/  Generic CRUD repository using Go generics
  query/       Fluent SQL query builder (zero deps)
```

## Design Decisions

### 1. Interface-Driven Core

The `database` package defines five interfaces (`Database`, `Tx`, `Row`, `Rows`, `Result`) and one config struct. No concrete implementations exist in this package. This allows:

- Adapter packages to implement the same contract independently
- Business logic to depend on abstractions, not drivers
- Easy testing with in-memory SQLite instead of PostgreSQL

The `Database` interface covers the full lifecycle: `Connect`, `Close`, `Exec`, `Query`, `QueryRow`, `Begin`, `HealthCheck`.

### 2. Adapter Pattern for Database Drivers

Each driver package (`postgres`, `sqlite`) provides a `Client` struct that implements `database.Database`. Internally, each wraps driver-specific types into the shared interfaces:

**PostgreSQL adapter wrapping chain:**
- `pgconn.CommandTag` -> `pgResult` (implements `database.Result`)
- `pgx.Row` -> `pgRow` (implements `database.Row`)
- `pgx.Rows` -> `pgRows` (implements `database.Rows`)
- `pgx.Tx` -> `pgTx` (implements `database.Tx`)

**SQLite adapter wrapping chain:**
- `sql.Result` -> `sqlResult` (implements `database.Result`)
- `*sql.Row` -> `sqlRow` (implements `database.Row`)
- `*sql.Rows` -> `sqlRows` (implements `database.Rows`)
- `*sql.Tx` -> `sqlTx` (implements `database.Tx`)

This is the classic Adapter pattern: each wrapper adapts a third-party type to the module's internal interface.

### 3. Factory Pattern for Configuration

Both `postgres.DefaultConfig()` and `sqlite.DefaultConfig(path)` serve as factory functions that produce fully configured structs with production-ready defaults.

The PostgreSQL factory dynamically sizes the connection pool based on `runtime.NumCPU()`:
- `MaxConns = min(max(CPU*2 + 1, 10), 50)`
- `MinConns = CPU / 2`

This avoids both under-utilization on large machines and resource exhaustion on small ones.

### 4. Builder Pattern for Query Construction

The `query.Builder` uses method chaining to construct SQL queries fluently:

```go
query.New().Select("id", "name").From("users").Where(query.Eq("active", true)).Build()
```

Each method returns `*Builder`, enabling the chain. `Build()` is the terminal method that produces the SQL string and arguments slice. This pattern:

- Makes query construction readable and self-documenting
- Prevents SQL injection through parameterized placeholders (`?`)
- Keeps the builder stateless after `Build()` -- the builder can be reused

### 5. Repository Pattern with Generics

`GenericRepository[T]` provides type-safe CRUD without code generation. The `EntityMapper[T]` interface acts as a bridge between the generic repository and entity-specific database mappings:

```
Repository[T] interface
    |
    v
GenericRepository[T] struct
    |
    +-- DB: database.Database (for queries)
    +-- Mapper: EntityMapper[T] (for SQL generation and scanning)
```

The mapper is responsible for:
- Table name and column definitions
- Row scanning (both single and multi-row)
- INSERT and UPDATE SQL generation
- Primary key column identification

This separation means `GenericRepository` never needs to know the schema -- it delegates all SQL generation and scanning to the mapper.

### 6. Semaphore-Based Connection Pool

`GenericPool` uses a buffered channel (`sem chan struct{}`) as a semaphore to enforce `MaxSize`. This is simpler and more Go-idiomatic than a mutex-guarded counter:

- `Acquire`: send to channel (blocks when full), then check idle list or create via factory
- `Release`: return to idle list, receive from channel (frees a slot)
- `Close`: stop health check goroutine, close all idle connections

The pool tracks metrics atomically (`sync/atomic`) to avoid contention on the hot path. A background goroutine periodically evicts unhealthy connections by taking a snapshot of the idle list, health-checking each, and retaining only healthy ones.

### 7. Transactional Migration Runner

Each migration (apply or rollback) executes inside a database transaction:
1. Begin transaction
2. Execute the Up/Down SQL
3. Insert/delete the tracking record
4. Commit (or rollback on error)

This guarantees that a migration is either fully applied (schema change + tracking record) or not applied at all. The tracking table (`schema_migrations`) uses the migration version as the primary key.

## Dependency Graph

```
query (standalone, zero internal deps)

database (standalone, zero internal deps)
   ^
   |
   +-- postgres (imports database)
   |
   +-- sqlite (imports database)
   |
   +-- migration (imports database)
   |
   +-- repository (imports database)

pool (standalone, zero internal deps)
```

Key observations:
- `query` and `pool` are fully independent -- they import no other module packages
- `postgres`, `sqlite`, `migration`, and `repository` all depend on `database` only
- There are no circular dependencies
- No package depends on a concrete adapter (`postgres` or `sqlite`)

## Error Handling Strategy

All errors follow Go's error wrapping convention with `fmt.Errorf("context: %w", err)`:

- Each layer adds context about what operation failed
- Error chains enable `errors.Is()` and `errors.As()` for specific error detection
- Deferred cleanup (e.g., `tx.Rollback()`, `rows.Close()`) uses `_` to discard secondary errors

Examples of the wrapping chain:
```
"create users: exec: connection refused"
"apply migration 3 (create posts): exec up: syntax error"
"list users: query: context deadline exceeded"
```

## Thread Safety

- `postgres.Client`: thread-safe via `pgxpool` (goroutine-safe by design)
- `sqlite.Client`: thread-safe via `database/sql` connection pool (MaxOpenConns=1 for WAL mode safety)
- `GenericPool`: thread-safe via `sync.Mutex` for idle list and `sync/atomic` for metrics
- `query.Builder`: NOT thread-safe -- each goroutine should create its own builder
- `GenericRepository`: thread-safe if the underlying `database.Database` is thread-safe
- `migration.Runner`: thread-safe if the underlying `database.Database` is thread-safe (but concurrent migration applies should be avoided)

## External Dependencies

| Dependency | Version | Used By | Purpose |
|-----------|---------|---------|---------|
| `github.com/jackc/pgx/v5` | v5.7.5 | `postgres` | PostgreSQL driver and connection pool |
| `modernc.org/sqlite` | v1.37.1 | `sqlite` | Pure Go SQLite driver (no CGO) |
| `github.com/stretchr/testify` | v1.11.1 | tests only | Test assertions |

The choice of `modernc.org/sqlite` over `mattn/go-sqlite3` is deliberate: it requires no CGO, making cross-compilation and containerized builds simpler.
