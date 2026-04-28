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



## Sixth Law — Real User Verification (Anti-Pseudo-Test Rule)

> Inherits from the root project's Anti-Bluff Testing Pact and the cross-project
> universal mandate (CONST-035). Submodule rules below are additive, never
> relaxing.

A test that passes while the feature it covers is broken for end users is the
most expensive kind of test in this codebase — it converts unknown breakage into
believed safety. This has happened in consuming projects before: tests and
Integration Challenge Tests executed green while large parts of the product
were unusable on a real device. That outcome is a constitutional failure, not a
coverage failure, and it MUST NOT recur in any module that depends on or is
depended on by this one.

Every test added MUST satisfy ALL of the following. Violation of any of them is
a release blocker, irrespective of coverage metrics, CI status, reviewer
sign-off, or schedule pressure.

1. **Same surfaces the user touches.** The test must traverse the production
   code path the user's action triggers, end to end, with no shortcut that
   bypasses real wiring.

2. **Provably falsifiable on real defects.** Before merging, the author MUST
   run the test once with the underlying feature deliberately broken (throw
   inside the function, return the wrong row, return the wrong status) and
   confirm the test fails with a clear assertion message. The PR description
   MUST state which deliberate break was used and what failure the test
   produced. A test that cannot be made to fail by breaking the thing it claims
   to verify is a bluff test by definition.

3. **Primary assertion on user-visible state.** The chief failure signal MUST
   be on something a real consumer could see or measure: rendered output,
   persisted database row, HTTP response body / status / header, file written
   to disk, packet on the wire. "Mock was invoked N times" is a permitted
   secondary assertion, never the primary one.

4. **Integration / Challenge tests are the load-bearing acceptance gate.** A
   green Challenge Test means a real consumer can complete the flow against
   real services — not "the wiring compiles". A feature for which a Challenge
   Test cannot be written is, by definition, not shippable.

5. **CI green is necessary, not sufficient.** Before any release tag is cut, a
   human (or a scripted black-box runner) MUST have exercised the feature
   end-to-end and observed the user-visible outcome.

6. **Inheritance.** This rule applies recursively to every consumer of this
   submodule. Consumer constitutions MAY add stricter rules but MUST NOT relax
   this one.

<!-- BEGIN anti-bluff-testing addendum (Article XI) -->

## Article XI — Anti-Bluff Testing (MANDATORY)

**Inherited from the umbrella project's Constitution Article XI.
Tests and Challenges that pass without exercising real end-user
behaviour are forbidden in this submodule too.**

Every test, every Challenge, every HelixQA bank entry MUST:

1. **Assert on a concrete end-user-visible outcome** — rendered DOM,
   DB rows that a real query would return, files on disk, media that
   actually plays, search results that actually contain expected
   items. Not "no error" or "200 OK".
2. **Run against the real system below the assertion.** Mocks/stubs
   are permitted ONLY in unit tests (`*_test.go` under `go test
   -short` or language equivalent). Integration / E2E / Challenge /
   HelixQA tests use real containers, real databases, real
   renderers. Unreachable real-system → skip with `SKIP-OK:
   #<ticket>`, never silently pass.
3. **Include a matching negative.** Every positive assertion is
   paired with an assertion that fails when the feature is broken.
4. **Emit copy-pasteable evidence** — body, screenshot, frame, DB
   row, log excerpt. Boolean pass/fail is insufficient.
5. **Verify "fails when feature is removed."** Author runs locally
   with the feature commented out; the test MUST FAIL. If it still
   passes, it's a bluff — delete and rewrite.
6. **No blind shells.** No `&& echo PASS`, `|| true`, `tee` exit
   laundering, `if [ -f file ]` without content assertion.

**Challenges in this submodule** must replay the user journey
end-to-end through the umbrella project's deliverables — never via
raw `curl` or third-party scripts. Sub-1-second Challenges almost
always indicate a bluff.

**HelixQA banks** declare executable actions
(`adb_shell:`, `playwright:`, `http:`, `assertVisible:`,
`assertNotVisible:`), never prose. Stagnation guard from Article I
§1.3 applies — frame N+1 identical to frame N for >10 s = FAIL.

**PR requirement:** every PR adding/modifying a test or Challenge in
this submodule MUST include a fenced `## Anti-Bluff Verification`
block with: (a) command run, (b) pasted output, (c) proof the test
fails when the feature is broken (second run with feature
commented-out showing FAIL).

**Cross-reference:** umbrella `CONSTITUTION.md` Article XI
(§§ 11.1 — 11.8).

<!-- END anti-bluff-testing addendum (Article XI) -->
