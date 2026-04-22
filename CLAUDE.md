# CLAUDE.md - Database Module

## Overview

`digital.vasic.database` is a generic, reusable Go module for relational database operations. It provides driver-agnostic interfaces with PostgreSQL and SQLite adapters, connection pooling, schema migrations, a generic repository pattern, and a fluent query builder.

**Module**: `digital.vasic.database` (Go 1.24+)

## Build & Test

```bash
go build ./...
go test ./... -count=1 -race
go test ./... -short              # Unit tests only
go test -tags=integration ./...   # Integration tests (requires PostgreSQL)
go test -bench=. ./...            # Benchmarks
```

## Code Style

- Standard Go conventions, `gofmt` formatting
- Imports grouped: stdlib, third-party, internal (blank line separated)
- Line length <= 100 chars
- Naming: `camelCase` private, `PascalCase` exported, acronyms all-caps
- Errors: always check, wrap with `fmt.Errorf("...: %w", err)`
- Tests: table-driven, `testify`, naming `Test<Struct>_<Method>_<Scenario>`

## Package Structure

| Package | Purpose |
|---------|---------|
| `pkg/database` | Core interfaces (Database, Tx, Row, Rows, Result, Config) |
| `pkg/postgres` | PostgreSQL adapter using pgx/v5 with connection pooling |
| `pkg/sqlite` | SQLite adapter using modernc.org/sqlite (pure Go, no CGO) |
| `pkg/pool` | Generic connection pool with metrics and health checking |
| `pkg/migration` | Schema migration runner with version tracking and rollback |
| `pkg/repository` | Generic repository pattern with CRUD and listing |
| `pkg/query` | Fluent SQL query builder with type-safe conditions |

## Key Interfaces

- `database.Database` -- Connect, Close, Exec, Query, QueryRow, Begin, HealthCheck
- `database.Tx` -- Commit, Rollback, Exec, Query, QueryRow
- `pool.Pool` -- Acquire, Release, Stats, Close
- `repository.Repository[T]` -- Create, GetByID, Update, Delete, List, Count
- `repository.EntityMapper[T]` -- TableName, Columns, ScanRow, InsertSQL, UpdateSQL
- `query.Condition` -- Build() (sql, args)

## Dependencies

- `github.com/jackc/pgx/v5` -- PostgreSQL driver
- `modernc.org/sqlite` -- Pure Go SQLite driver
- `github.com/stretchr/testify` -- Testing assertions


## ⚠️ MANDATORY: NO SUDO OR ROOT EXECUTION

**ALL operations MUST run at local user level ONLY.**

This is a PERMANENT and NON-NEGOTIABLE security constraint:

- **NEVER** use `sudo` in ANY command
- **NEVER** use `su` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Container-Based Solutions
When a build or runtime environment requires system-level dependencies, use containers instead of elevation:

- **Use the `Containers` submodule** (`https://github.com/vasic-digital/Containers`) for containerized build and runtime environments
- **Add the `Containers` submodule as a Git dependency** and configure it for local use within the project
- **Build and run inside containers** to avoid any need for privilege escalation
- **Rootless Podman/Docker** is the preferred container runtime

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo` or `su`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Use the `Containers` submodule for containerized builds
5. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**


