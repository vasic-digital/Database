# Database Module

`digital.vasic.database` is a generic, reusable Go module for relational database operations. It provides driver-agnostic interfaces with PostgreSQL and SQLite adapters, connection pooling, schema migrations, a generic repository pattern, and a fluent query builder.

## Key Features

- **Interface-driven core** -- Five core interfaces (`Database`, `Tx`, `Row`, `Rows`, `Result`) with no concrete implementations in the base package
- **PostgreSQL adapter** -- Production-ready adapter using `pgx/v5` with connection pooling sized by CPU count
- **SQLite adapter** -- Pure Go adapter using `modernc.org/sqlite` (no CGO required)
- **Generic repository** -- Type-safe CRUD operations via `GenericRepository[T]` using Go generics
- **Fluent query builder** -- SQL construction with method chaining and parameterized placeholders
- **Connection pooling** -- Semaphore-based pool with health checking and idle eviction
- **Schema migrations** -- Transactional migration runner with version tracking and rollback

## Package Overview

| Package | Purpose |
|---------|---------|
| `pkg/database` | Core interfaces and configuration |
| `pkg/postgres` | PostgreSQL adapter (pgx/v5) |
| `pkg/sqlite` | SQLite adapter (modernc.org/sqlite) |
| `pkg/pool` | Generic connection pool with metrics |
| `pkg/migration` | Schema migration runner |
| `pkg/repository` | Generic CRUD repository |
| `pkg/query` | Fluent SQL query builder |
| `pkg/connection` | Connection management utilities |
| `pkg/dialect` | SQL dialect abstraction |
| `pkg/helpers` | Database helper functions |

## Installation

```bash
go get digital.vasic.database
```

Requires Go 1.24 or later.

## Dependencies

| Dependency | Purpose |
|-----------|---------|
| `github.com/jackc/pgx/v5` | PostgreSQL driver and connection pool |
| `modernc.org/sqlite` | Pure Go SQLite driver (no CGO) |
