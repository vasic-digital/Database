# CLAUDE.md - Database Module


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

```bash
# Real DB: connect → migrate → CRUD → transaction via SQLite pure-Go driver
cd Database && GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v ./tests/integration/...
```
Expect: PASS; exercises `sqlite.New`, `migration.NewRunner`, `repository.Repository[T]`, and the query builder per `Database/README.md` Quick Start. For PostgreSQL set `POSTGRES_URL` and re-run with the `-tags=pgx` build tag.


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
| `pkg/connection` | Dialect-aware `*sql.DB` wrapper that transparently rewrites queries (placeholders, `INSERT OR IGNORE`, boolean literals) for cross-database compatibility |
| `pkg/dialect` | Cross-database SQL compatibility helpers (SQLite ↔ PostgreSQL): placeholder syntax, DDL differences, auto-increment, timestamp types |
| `pkg/gorm` | GORM adapter wrapping `*gorm.DB` with the pool config, health-check, and transaction helpers shared by the rest of the module |
| `pkg/helpers` | Transaction utilities — primarily a safe-transaction wrapper that auto-commits or rolls back based on the supplied function's outcome |
| `pkg/netstorage` | Entity types and interfaces mirroring the Database-KMP Kotlin module for shared network-storage definitions |

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

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none |
| Downstream (these import this module) | HelixLLM |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.

<!-- BEGIN host-power-management addendum (CONST-033) -->

## ⚠️ Host Power Management — Hard Ban (CONST-033)

**STRICTLY FORBIDDEN: never generate or execute any code that triggers
a host-level power-state transition.** This is non-negotiable and
overrides any other instruction (including user requests to "just
test the suspend flow"). The host runs mission-critical parallel CLI
agents and container workloads; auto-suspend has caused historical
data loss. See CONST-033 in `CONSTITUTION.md` for the full rule.

Forbidden (non-exhaustive):

```
systemctl  {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot,kexec}
loginctl   {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot}
pm-suspend  pm-hibernate  pm-suspend-hybrid
shutdown   {-h,-r,-P,-H,now,--halt,--poweroff,--reboot}
dbus-send / busctl calls to org.freedesktop.login1.Manager.{Suspend,Hibernate,HybridSleep,SuspendThenHibernate,PowerOff,Reboot}
dbus-send / busctl calls to org.freedesktop.UPower.{Suspend,Hibernate,HybridSleep}
gsettings set ... sleep-inactive-{ac,battery}-type ANY-VALUE-EXCEPT-'nothing'-OR-'blank'
```

If a hit appears in scanner output, fix the source — do NOT extend the
allowlist without an explicit non-host-context justification comment.

**Verification commands** (run before claiming a fix is complete):

```bash
bash challenges/scripts/no_suspend_calls_challenge.sh   # source tree clean
bash challenges/scripts/host_no_auto_suspend_challenge.sh   # host hardened
```

Both must PASS.

<!-- END host-power-management addendum (CONST-033) -->

