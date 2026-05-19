# digital.vasic.database

Generic, reusable Go module for relational database operations — driver-agnostic
interfaces, PostgreSQL + SQLite adapters, generic connection pooling, version-
tracked schema migrations, generic repository pattern, fluent SQL query
builder, and cross-dialect compatibility helpers.

**Module path:** `digital.vasic.database`
**Go version:** 1.25+
**License:** see repository root

---

## Why this module exists

Most Go applications repeat the same shape of database boilerplate: open a
`*sql.DB`, manage a connection pool, version schema with hand-rolled SQL
files, write CRUD by hand for every entity, manually quote placeholders per
driver, swallow `Begin` / `Commit` / `Rollback` ergonomics. This module
collapses that surface into a small set of injectable interfaces with
production-quality default implementations — so consumers wire it once and
ship features instead of reinventing pools, migrations, and SQL builders.

The library deliberately stays project-not-aware (CONST-051(B)): no
HelixCode-specific paths, hostnames, or runtime assumptions leak in. It is
fully standalone-testable against `:memory:` SQLite, and integrates with real
PostgreSQL when the consumer wires a `Config` pointing at one.

---

## Features

- **Driver-agnostic interfaces** — `Database`, `Tx`, `Row`, `Rows`, `Result`
  in `pkg/database/`; every adapter implements the same contract.
- **PostgreSQL adapter** — `pkg/postgres/` uses `pgx/v5` with `pgxpool`
  connection pooling and lifecycle hooks.
- **SQLite adapter** — `pkg/sqlite/` uses `modernc.org/sqlite` (pure Go, no
  CGO); supports `:memory:`, WAL journaling, configurable busy timeout.
- **Generic connection pool** — `pkg/pool/` provides `Pool[T]` with metrics,
  health-checking, eviction, and bounded acquire timeout.
- **Schema migrations** — `pkg/migration/` runs version-tracked `Up`/`Down`
  migrations idempotently against the configured tracking table (default
  `schema_migrations`).
- **Generic repository pattern** — `pkg/repository/` exposes
  `Repository[T any]` with `Create`, `GetByID`, `Update`, `Delete`, `List`,
  `Count`; backed by a consumer-supplied `EntityMapper[T]`.
- **Fluent SQL query builder** — `pkg/query/` composes `Select`, `From`,
  `Where`, `OrderBy`, `Limit`, `Offset` with type-safe `Condition` helpers
  (`Eq`, `Gt`, `Lt`, `In`, `Like`, `Between`, `IsNull`, `And`, `Or`, …).
- **Cross-dialect connection wrapper** — `pkg/connection/` rewrites placeholders
  (`?` ↔ `$N`), `INSERT OR IGNORE` ↔ `ON CONFLICT DO NOTHING`, boolean
  literals; consumers can write one SQL string that runs on both backends.
- **Cross-dialect helpers** — `pkg/dialect/` enumerates DDL/auto-increment/
  timestamp differences for consumers who prefer explicit branching.
- **GORM adapter** — `pkg/gorm/` wraps `*gorm.DB` with the same pool config,
  health-check, and transaction helpers used by the rest of the module.
- **Transaction helpers** — `pkg/helpers/` ships a safe-transaction wrapper
  that auto-commits or auto-rolls-back based on the supplied function's
  outcome.
- **Shared network-storage types** — `pkg/netstorage/` mirrors the
  Database-KMP Kotlin module so cross-platform consumers share entity
  definitions.

---

## Installation

```bash
go get digital.vasic.database
```

---

## Quick Start

### SQLite (zero-config, no CGO, ideal for tests)

```go
package main

import (
    "context"
    "log"

    "digital.vasic.database/pkg/migration"
    "digital.vasic.database/pkg/sqlite"
)

func main() {
    ctx := context.Background()

    // Use an in-memory database for the demo (production: pass a file path).
    db := sqlite.New(sqlite.DefaultConfig(":memory:"))
    if err := db.Connect(ctx); err != nil {
        log.Fatalf("connect: %v", err)
    }
    defer db.Close()

    // Run versioned migrations.
    runner := migration.NewRunner(db, "")
    if err := runner.Init(ctx); err != nil {
        log.Fatalf("migration init: %v", err)
    }
    err := runner.Apply(ctx, []migration.Migration{
        {
            Version: 1,
            Name:    "create users",
            Up:      "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)",
            Down:    "DROP TABLE users",
        },
    })
    if err != nil {
        log.Fatalf("apply: %v", err)
    }

    // Real INSERT — not a placeholder, not a simulation.
    if _, err := db.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Alice"); err != nil {
        log.Fatalf("insert: %v", err)
    }

    // Real SELECT.
    var name string
    if err := db.QueryRow(ctx, "SELECT name FROM users WHERE id = ?", 1).Scan(&name); err != nil {
        log.Fatalf("query: %v", err)
    }
    log.Printf("user 1 = %s", name) // Alice
}
```

### PostgreSQL (production)

```go
import (
    "digital.vasic.database/pkg/database"
    "digital.vasic.database/pkg/postgres"
)

cfg := &database.Config{
    Driver:   "postgres",
    Host:     "db.example.internal",
    Port:     5432,
    User:     "app",
    Password: os.Getenv("DB_PASSWORD"), // no hardcoded credentials (CONST-046 / CONST-042)
    DBName:   "production",
    SSLMode:  "require",
    MaxConns: 50,
    MinConns: 5,
}
db := postgres.New(cfg)
if err := db.Connect(ctx); err != nil { /* ... */ }
```

The rest of the API (migrations, repositories, query builder, transactions)
is **identical** to the SQLite path — driver-agnostic by design.

---

## Anti-Bluff Posture

This module follows the parent project's anti-bluff mandate (Article XI §11.9,
CONST-035, CONST-050(B)) and the Round-244 enrichment cycle:

- **No simulated code paths.** Every public function actually executes its
  documented behaviour against the configured driver — no `"for now"` stubs,
  no `"TODO implement"` returns, no `placeholder` payloads.
- **Captured runtime evidence.** Every PASS in the Challenge suite is
  preceded by a real command + captured stdout/stderr written to
  `challenges/.last-run/`.
- **Paired mutation.** The Challenge suite includes an `--anti-bluff-mutate`
  leg that re-installs a deliberately-broken adapter (a SQLite `Database`
  whose `Exec` silently swallows the row) and asserts the probe **FAILs**
  with exit code 99 — proving the probe would catch a real regression.
- **Real backing store, not a mock.** The Challenge's runtime probe uses a
  real SQLite database file (not `database/sql` test doubles) and asserts
  end-to-end CRUD round-trip through migration → insert → query → describe.

See `docs/test-coverage.md` for the full CONST-050(B) coverage ledger.

---

## Testing

```bash
# Fast unit suite (mocks permitted per CONST-050(A))
GOMAXPROCS=2 go test ./pkg/... -count=1 -race -short

# Full local sweep including tests/{integration,e2e,security,benchmark,stress}
GOMAXPROCS=2 go test ./... -count=1 -race

# Coverage report
go test -coverprofile=cover.out ./pkg/...
go tool cover -html=cover.out

# Anti-bluff Challenge (executes full probe + paired mutation)
bash challenges/database_describe_challenge.sh
bash challenges/database_describe_challenge.sh --anti-bluff-mutate  # must exit 99
```

The Challenge writes evidence to `challenges/.last-run/`:

| File                          | Content                                        |
|-------------------------------|------------------------------------------------|
| `01-vet.log`                  | `go vet ./...` output                          |
| `02-build.log`                | `go build ./...` output                        |
| `03-test.log`                 | `go test ./pkg/... -count=1 -race` output      |
| `04-probe-build.log`          | Challenge runner compilation output            |
| `05-probe-normal.log`         | Runtime probe stdout/stderr (PASS expected)    |
| `06-probe-mutation.log`       | Mutation leg stderr (FAIL with rc=99 expected) |

---

## Repository Layout

```
pkg/database/    Core interfaces (Database, Tx, Row, Rows, Result, Config)
pkg/postgres/    pgx/v5 adapter + pgxpool connection pooling
pkg/sqlite/      modernc.org/sqlite adapter (pure Go)
pkg/pool/        Generic connection pool with metrics + eviction
pkg/migration/   Version-tracked migration runner with Up/Down
pkg/repository/  Repository[T] with CRUD + listing
pkg/query/       Fluent SQL builder with type-safe conditions
pkg/connection/  Dialect-aware *sql.DB wrapper (placeholder/DDL rewriting)
pkg/dialect/     Cross-database SQL compatibility helpers
pkg/gorm/        GORM adapter (pool config + health-check + transactions)
pkg/helpers/     Transaction utilities (safe Tx wrapper)
pkg/netstorage/  Shared entity types (mirrors Database-KMP)

challenges/      Anti-bluff Challenge scripts + runner + fixtures
docs/            Architecture, API reference, user guide, test-coverage ledger
tests/           Integration, e2e, security, benchmark, stress suites
```

See `docs/ARCHITECTURE.md` for the data-flow + interface diagrams and
`docs/API_REFERENCE.md` for the full public-surface catalog.

---

## Bilingual fixtures

`challenges/fixtures/{en.yaml,sr-Latn.yaml}` carry the human-readable strings
the Challenge runner emits when describing each driver. They exist so the
parent project's CONST-046 i18n posture stays demonstrable at the
Challenge-output layer — the same probe can be rerun with a different
language fixture to produce a localised describe output without touching
production code.

---

## License

See repository root (`LICENSE`).
