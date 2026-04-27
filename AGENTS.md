# AGENTS.md - Database Module Multi-Agent Coordination

## Module Identity

- **Module**: `digital.vasic.database`
- **Purpose**: Generic, reusable Go module for relational database operations
- **Language**: Go 1.24+
- **Packages**: database, postgres, sqlite, pool, migration, repository, query

## Agent Responsibilities

### Database Core Agent

**Owns**: `pkg/database/`

- Maintains the driver-agnostic interface contracts (`Database`, `Tx`, `Row`, `Rows`, `Result`)
- Maintains `Config` struct and its `DSN()` / `Validate()` methods
- Any change to core interfaces requires coordination with all adapter agents (PostgreSQL, SQLite)
- Must not import any driver-specific packages

### PostgreSQL Agent

**Owns**: `pkg/postgres/`

- Maintains `Client` (implements `database.Database`) backed by `pgx/v5` and `pgxpool`
- Manages `Config` (embeds `database.Config`), `DefaultConfig()`, pool configuration
- Wraps `pgconn.CommandTag`, `pgx.Row`, `pgx.Rows`, `pgx.Tx` into database interfaces
- Exposes `Pool()` for advanced pgxpool access and `Migrate()` for raw SQL migrations
- Integration tests require a live PostgreSQL instance

### SQLite Agent

**Owns**: `pkg/sqlite/`

- Maintains `Client` (implements `database.Database`) backed by `modernc.org/sqlite`
- Manages `Config`, `DefaultConfig(path)`, PRAGMA application (WAL, foreign_keys, synchronous, busy_timeout)
- Wraps `sql.Result`, `*sql.Row`, `*sql.Rows`, `*sql.Tx` into database interfaces
- Exposes `DB()` for advanced `*sql.DB` access
- Pure Go -- no CGO dependency

### Pool Agent

**Owns**: `pkg/pool/`

- Maintains `GenericPool` with semaphore-based concurrency control
- Manages `PoolConfig`, `PoolStats`, `DefaultPoolConfig()`
- Implements connection lifecycle: acquire (idle reuse or factory), release, eviction
- Background health check loop with `ConnHealthChecker` callback
- Tracks metrics atomically: acquire count, errors, latency, peak concurrency
- Functional dependencies: `ConnFactory`, `ConnHealthChecker`, `ConnCloser`

### Migration Agent

**Owns**: `pkg/migration/`

- Maintains `Runner` with version-tracked schema migration management
- Manages `Migration` struct (Version, Name, Up, Down SQL)
- Operations: `Init` (create tracking table), `Applied` (list versions), `Apply` (forward), `Rollback` / `RollbackWith` (reverse)
- Each migration runs in a transaction (apply + record / rollback + delete)
- Depends on `database.Database` interface only

### Repository Agent

**Owns**: `pkg/repository/`

- Maintains `GenericRepository[T]` implementing `Repository[T]` interface
- Manages `EntityMapper[T]` contract for per-entity table/column/scan/SQL mapping
- CRUD operations: `Create`, `GetByID`, `Update`, `Delete`, `List`, `Count`
- `ListOptions` with `WhereClause`, pagination (`Offset`, `Limit`), ordering
- Depends on `database.Database` interface only

### Query Agent

**Owns**: `pkg/query/`

- Maintains `Builder` with fluent method chaining (Select, From, Where, OrderBy, Limit, Offset, GroupBy, Having)
- Maintains `Condition` interface and built-in implementations:
  - Comparison: `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Like`
  - Null: `IsNull`, `IsNotNull`
  - Set: `In`
  - Composite: `And`, `Or`
- `Build()` produces parameterized SQL with `?` placeholders
- Zero external dependencies -- standalone SQL generation

## Coordination Rules

### Interface Changes

Any modification to interfaces in `pkg/database/` (Database, Tx, Row, Rows, Result) requires:
1. Database Core Agent updates the interface definition
2. PostgreSQL Agent updates `Client` and wrapper types to satisfy the new contract
3. SQLite Agent updates `Client` and wrapper types to satisfy the new contract
4. Repository Agent and Migration Agent review for downstream impact

### New Adapter

Adding a new database adapter (e.g., MySQL):
1. Database Core Agent verifies no interface changes are needed
2. New adapter agent creates `pkg/<driver>/` implementing `database.Database`
3. Tests must cover Connect, Close, Exec, Query, QueryRow, Begin, HealthCheck

### Repository Mapper

Adding a new entity type:
1. Repository Agent defines the `EntityMapper[T]` implementation
2. No changes required to `GenericRepository` -- it is generic over `T`

### Migration Schema

Adding new migrations:
1. Migration Agent manages the `Migration` structs
2. The `Runner` auto-initializes the tracking table (`schema_migrations`)
3. Each migration version must be unique and monotonically increasing

## Testing Boundaries

| Agent | Unit Tests | Integration Tests | Requirements |
|-------|-----------|-------------------|--------------|
| Database Core | Interface mocks, Config validation | None | None |
| PostgreSQL | Config defaults, DSN building | Full CRUD, transactions | Live PostgreSQL |
| SQLite | Config defaults, PRAGMA verification | Full CRUD, transactions, in-memory | None (pure Go) |
| Pool | Config validation, stats tracking | Acquire/release cycles, eviction | None |
| Migration | Version sorting, applied tracking | Apply/rollback sequences | database.Database impl |
| Repository | WhereClause building | Full CRUD with ListOptions | database.Database impl |
| Query | SQL generation, condition building | None | None |

## Communication Protocol

Agents coordinate through:
- **Interface contracts**: `pkg/database/` defines the shared API surface
- **Go module versioning**: Breaking changes require a major version bump
- **Test coverage**: Every exported function must have table-driven tests with `testify`
- **Error wrapping**: All errors use `fmt.Errorf("context: %w", err)` for traceability


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


### ⚠️⚠️⚠️ ABSOLUTELY MANDATORY: ZERO UNFINISHED WORK POLICY

NO unfinished work, TODOs, or known issues may remain in the codebase. EVER.

PROHIBITED: TODO/FIXME comments, empty implementations, silent errors, fake data, unwrap() calls that panic, empty catch blocks.

REQUIRED: Fix ALL issues immediately, complete implementations before committing, proper error handling in ALL code paths, real test assertions.

Quality Principle: If it is not finished, it does not ship. If it ships, it is finished.

<!-- BEGIN host-power-management addendum (CONST-033) -->

## Host Power Management — Hard Ban (CONST-033)

**You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition.** This rule applies to:

- Every shell command you run via the Bash tool.
- Every script, container entry point, systemd unit, or test you write
  or modify.
- Every CLI suggestion, snippet, or example you emit.

**Forbidden invocations** (non-exhaustive — see CONST-033 in
`CONSTITUTION.md` for the full list):

- `systemctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot|kexec`
- `loginctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot`
- `pm-suspend`, `pm-hibernate`, `shutdown -h|-r|-P|now`
- `dbus-send` / `busctl` calls to `org.freedesktop.login1.Manager.Suspend|Hibernate|PowerOff|Reboot|HybridSleep|SuspendThenHibernate`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to anything but `'nothing'` or `'blank'`

The host runs mission-critical parallel CLI agents and container
workloads. Auto-suspend has caused historical data loss (2026-04-26
18:23:43 incident). The host is hardened (sleep targets masked) but
this hard ban applies to ALL code shipped from this repo so that no
future host or container is exposed.

**Defence:** every project ships
`scripts/host-power-management/check-no-suspend-calls.sh` (static
scanner) and
`challenges/scripts/no_suspend_calls_challenge.sh` (challenge wrapper).
Both MUST be wired into the project's CI / `run_all_challenges.sh`.

**Full background:** `docs/HOST_POWER_MANAGEMENT.md` and `CONSTITUTION.md` (CONST-033).

<!-- END host-power-management addendum (CONST-033) -->

