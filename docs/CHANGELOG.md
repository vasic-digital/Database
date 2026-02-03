# Changelog

All notable changes to the `digital.vasic.database` module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-03

### Added

- **pkg/database**: Core interfaces (`Database`, `Tx`, `Row`, `Rows`, `Result`) and `Config` struct with `DSN()` and `Validate()` methods.
- **pkg/postgres**: PostgreSQL adapter using `pgx/v5` and `pgxpool`. `Client` implements `database.Database` with connection pooling, health checks, and configurable pool parameters. `DefaultConfig()` auto-sizes pool based on CPU count. `Pool()` exposes underlying `pgxpool.Pool`. `Migrate()` convenience method for raw SQL migrations.
- **pkg/sqlite**: SQLite adapter using `modernc.org/sqlite` (pure Go, no CGO). `Client` implements `database.Database` with WAL journal mode, foreign keys, busy timeout, and synchronous pragmas applied on connect. `DefaultConfig(path)` factory. `DB()` exposes underlying `*sql.DB`.
- **pkg/pool**: Generic connection pool (`GenericPool`) with semaphore-based concurrency control, configurable lifecycle (MaxSize, MinSize, MaxLifetime, MaxIdleTime), background health check goroutine, idle connection eviction, and atomic metrics tracking (`PoolStats`). `ConnFactory`, `ConnHealthChecker`, `ConnCloser` function types.
- **pkg/migration**: Schema migration runner (`Runner`) with version-tracked `schema_migrations` table, forward application (`Apply`), rollback (`RollbackWith`), version listing (`Applied`), and table initialization (`Init`). Each migration runs in a transaction.
- **pkg/repository**: Generic repository pattern using Go generics. `Repository[T]` interface with `Create`, `GetByID`, `Update`, `Delete`, `List`, `Count`. `GenericRepository[T]` implementation. `EntityMapper[T]` interface for per-entity SQL mapping. `ListOptions` with `WhereClause`, pagination, and ordering.
- **pkg/query**: Fluent SQL query builder (`Builder`) with `Select`, `From`, `Where`, `OrderBy`, `Limit`, `Offset`, `GroupBy`, `Having`, and `Build`. `Condition` interface with 12 built-in constructors: `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Like`, `IsNull`, `IsNotNull`, `In`, `And`, `Or`.
- Unit tests for all packages with table-driven tests using testify.
