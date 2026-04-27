# Architecture -- Database

## Purpose

Generic, reusable Go module for relational database operations. Provides driver-agnostic interfaces with PostgreSQL (pgx/v5) and SQLite (pure Go, no CGO) adapters, generic connection pooling with metrics, version-tracked schema migrations with rollback, a generic CRUD repository pattern using Go generics, and a fluent SQL query builder with type-safe conditions.

## Structure

```
pkg/
  database/    Core interfaces: Database, Tx, Row, Rows, Result, Config
  postgres/    PostgreSQL adapter using pgx/v5 with connection pooling (pgxpool)
  sqlite/      SQLite adapter using modernc.org/sqlite (pure Go, no CGO required)
  pool/        Generic connection pool with metrics, health checking, and eviction
  migration/   Schema migration runner with version tracking and rollback support
  repository/  Generic repository pattern: Repository[T] with CRUD and listing
  query/       Fluent SQL query builder with type-safe conditions (Eq, Gt, In, Like, etc.)
```

## Key Components

- **`database.Database`** -- Core interface: Connect, Close, Exec, Query, QueryRow, Begin, HealthCheck
- **`database.Tx`** -- Transaction interface: Commit, Rollback, Exec, Query, QueryRow
- **`postgres.DB`** / **`sqlite.DB`** -- Concrete adapter implementations
- **`pool.Pool`** -- Generic connection pool with Acquire, Release, Stats, Close, and automatic eviction
- **`migration.Runner`** -- Version-tracked migration execution with Apply and Rollback
- **`repository.Repository[T]`** -- Generic CRUD: Create, GetByID, Update, Delete, List, Count
- **`repository.EntityMapper[T]`** -- Maps entities to SQL: TableName, Columns, ScanRow, InsertSQL, UpdateSQL
- **`query.Builder`** -- Fluent API: Select, From, Where, OrderBy, Limit, Offset with type-safe Condition builders

## Data Flow

```
Repository[T].Create(entity) -> EntityMapper[T].InsertSQL() -> Database.Exec(sql, args)
Repository[T].GetByID(id)    -> query.Builder.Select().Where(Eq("id", id)) -> Database.QueryRow()
Repository[T].List(opts)     -> query.Builder.Select().OrderBy().Limit().Offset() -> Database.Query()

Migration: Runner.Apply(migrations) -> for each version:
    check schema_migrations table -> execute Up SQL -> record version
```

## Dependencies

- `github.com/jackc/pgx/v5` -- PostgreSQL driver with connection pooling
- `modernc.org/sqlite` -- Pure Go SQLite driver (no CGO)
- `github.com/stretchr/testify` -- Test assertions

## Testing Strategy

Table-driven tests with `testify` and race detection. SQLite used for all unit tests (in-memory mode). Integration tests require a running PostgreSQL instance. Tests cover CRUD operations, migration apply/rollback, query builder SQL generation, connection pool lifecycle, and concurrent access patterns.
