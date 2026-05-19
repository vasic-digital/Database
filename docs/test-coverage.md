# Database — Test-Type Coverage Matrix

**Authority**: CONST-050(B) "100%-Test-Type-Coverage" mandate (cascaded from
HelixConstitution submodule §11.4.27).
**Scope**: this document is the Database submodule's coverage ledger. It
enumerates every test type CONST-050(B) recognises and records the current
status against the module's surface (`pkg/database/`, `pkg/sqlite/`,
`pkg/postgres/`, `pkg/migration/`, `pkg/repository/`, `pkg/query/`,
`pkg/pool/`, `pkg/connection/`, `pkg/dialect/`, `pkg/gorm/`, `pkg/helpers/`,
`pkg/netstorage/`).

A row may be `covered`, `planned`, or `n/a (out of scope for a library of this
shape)`. `n/a` rows MUST justify themselves — silent omission is a CONST-048
violation per §11.4.25.

---

## Coverage Ledger

| Test type        | Status   | Artefact / location                                                                                              | Notes |
|------------------|----------|------------------------------------------------------------------------------------------------------------------|-------|
| Unit             | covered  | `pkg/database/database_test.go` + `database_edge_test.go`, `pkg/sqlite/sqlite_test.go`, `pkg/migration/migration_test.go`, `pkg/repository/repository_test.go`, `pkg/query/query_test.go`, `pkg/pool/pool_test.go`, `pkg/connection/*_test.go`, `pkg/dialect/*_test.go`, `pkg/gorm/*_test.go`, `pkg/helpers/*_test.go`, `pkg/netstorage/*_test.go` | Mocks permitted per CONST-050(A); race-detector enforced; `Config.Validate` + `Config.DSN` exercised for every supported driver branch. |
| Integration      | covered  | `tests/integration/` (`-tags=integration` for PG-requiring leg)                                                  | SQLite leg uses `:memory:` real driver — no mocks. PG leg requires `POSTGRES_URL`; skipped with `SKIP-OK` marker otherwise per CONST-035 skip-bluff rule. |
| E2E              | covered  | `tests/e2e/` + `challenges/database_describe_challenge.sh` (round-244)                                           | Bash-orchestrated full round-trip — vet + build + unit + runtime probe (SQLite Connect → migration Apply → repository Create → query Scan → describe) + paired anti-bluff mutation re-installing a swallow-Exec adapter and asserting the probe FAILs with rc=99. |
| Full automation  | planned  | recommend: re-run the Challenge under every supported Go minor (1.25, 1.26) on each host platform (linux/darwin/windows) | CONST-048 coverage matrix dimension is feature × platform × invariant; Database is pure Go (modernc SQLite + pgx/v5) so platform coverage = Go-supported set. |
| Security         | covered  | `tests/security/`                                                                                                | Threat model: SQL injection via `Query`/`Exec` (verifies positional placeholder enforcement); `Config.Password` not echoed in `Config.DSN()` error paths; no credential in `*_test.go` fixtures (CONST-042 / CONST-053). |
| DDoS             | n/a      | —                                                                                                                | Database is an in-process library — no network surface, no request fan-in. The consuming service (e.g. an HTTP API in front of PostgreSQL) exposes the DDoS surface, not this module. |
| Scaling          | covered  | `pkg/pool/pool_test.go` concurrent-acquire tests                                                                 | Verifies `Pool[T]` bounded behaviour under N concurrent acquirers (N ∈ {1, 10, 100}); pool's `MaxConns` cap holds. |
| Chaos            | planned  | recommend: chaos-style assertion that a mid-transaction process kill leaves the database recoverable; Connect failure with pgx network partition; SQLite WAL recovery after simulated power loss | Failure-injection scope: network partition (postgres), file deletion mid-Connect (sqlite), context cancellation propagation through `pgx.QueryRow`. |
| Stress           | covered  | `tests/stress/`                                                                                                  | Sustained `Exec` + `Query` load above the typical tier (counts: 10k+); verifies pool eviction + connection re-use are correct under heat. |
| Performance      | covered (micro) / planned (macro) | `tests/benchmark/` (`go test -bench`)                                                              | Per-query latency + per-Tx commit latency reported with `b.ReportAllocs()`. Macro tier (vs SLO baseline) lives in the consuming application (CONST-051(B)). |
| Benchmarking     | covered (micro) / planned (macro) | `tests/benchmark/`                                                                                 | Historical p95-drift detection planned in consumer release-gate sweep. |
| UI               | n/a      | —                                                                                                                | Database ships no UI. |
| UX               | planned  | recommend: when `pkg/repository` describe-style methods gain locale-aware output, the bilingual round-trip pattern from `challenges/fixtures/{en,sr-Latn}.yaml` will exercise the path | Currently the module's user-facing strings are limited to error messages (English-only); CONST-046 work to dynamically generate locale-aware describe output is a future round. |
| Challenges       | covered  | `challenges/database_describe_challenge.sh` + `challenges/runner/main.go` + `challenges/fixtures/{en,sr-Latn}.yaml` (round-244)  | Incorporates the `vasic-digital/Challenges` pattern; captures stdout/stderr as wire evidence per §11.4.2; paired mutation per §1.1 / CONST-055 meta-test. Pre-round-244 the module also shipped `database_compile_challenge.sh`, `database_functionality_challenge.sh`, `database_unit_challenge.sh`, plus the chaos / ddos / scaling / stress / ui / ux Challenges from the CONST-050(B) round-215 cascade — all retained. |
| HelixQA          | planned  | recommend: register Database as a target in HelixQA's autonomous QA bank                                         | HelixQA submodule (`HelixDevelopment/HelixQA`) is incorporated at HelixCode root per CONST-050; Database enrolment is a HelixCode-meta-repo task, not a Database-internal task. |

---

## Anti-Bluff Posture

Every `covered` row above carries captured runtime evidence:

- **Unit**: `GOMAXPROCS=2 go test ./pkg/... -count=1 -race -short` exits 0; per-package timing logged; `pkg/database/database_edge_test.go` covers Config validation edges (missing driver, missing host for postgres, missing dbname for sqlite).
- **E2E (Challenge)**: `challenges/database_describe_challenge.sh` writes
  `challenges/.last-run/` artefacts containing stdout + stderr + assertion
  log + mutation-rejection proof. The mutation leg deliberately swaps in a
  `swallowingDatabase` whose `Exec` returns success without persisting any
  row; the probe MUST FAIL because the subsequent `QueryRow` returns
  `sql.ErrNoRows` — proving the probe would catch a real regression in the
  adapter contract.
- **Performance (micro)**: `go test -bench=. -benchmem ./tests/benchmark` produces ns/op + allocs/op numbers; future macro-benchmarks will diff against historical baseline.

Rows marked `planned` are **deliverables for future rounds**, NOT bluffs —
CONST-048 (Six Invariants) tolerates documented gaps in the ledger only when
the gap is explicit, dated, and owner-assigned. This document is the explicit
register; future rounds will flip rows from `planned` to `covered` with the
matching artefact.

---

## Four-Layer Floor (CONST-048 invariant 6)

Per §1 of the constitution, every test artefact MUST sit on the four-layer floor:

| Layer       | Database artefact today                                                                                |
|-------------|--------------------------------------------------------------------------------------------------------|
| Pre-build   | `go vet ./...`, `go build ./...` — invoked by `challenges/database_describe_challenge.sh` step 1       |
| Post-build  | `go test ./pkg/... -count=1 -race -short` — invoked by Challenge step 2                                |
| Runtime     | Driver-contract probe (Connect → migrate → insert → query → describe) — Challenge step 4               |
| Paired mut. | re-install swallowing `Database` adapter via Go program, assert probe FAILs with rc=99 — Challenge step 5 |

A future round that adds a new test type to a `covered` row MUST extend the
Challenge to keep the four-layer floor intact.

---

## Bilingual fixtures

`challenges/fixtures/en.yaml` and `challenges/fixtures/sr-Latn.yaml` carry the
human-readable labels the Challenge runner emits when describing the SQLite
driver round-trip. Round-244 introduces them so the parent project's
CONST-046 i18n posture stays demonstrable at the Challenge-output layer:
swapping `LANG_FIXTURE=sr-Latn` at probe-run time produces a Cyrillic-romanised
describe output without touching production code — proving the locale
boundary is honest, not hardcoded.

---

## Owner / Cadence

- **Owner**: Database submodule maintainer (vasic-digital). HelixCode
  consumers MAY contribute upstream but MUST NOT inject HelixCode-specific
  context (CONST-051(B)).
- **Cadence**: ledger reviewed at every governance-cascade round; planned →
  covered transitions land as their own commits with verbatim mandate quotes
  per CONST-049 §11.4.17.

---

## Round-244 deliverable

This document was introduced in **round-244** (mirror of round-220
DocProcessor enrichment) as part of the §11.4 deep-doc + Challenge
enrichment sweep. The verbatim 2026-05-19 operator mandate driving the cycle:

> "all existing tests and Challenges do work in anti-bluff manner — they MUST
> confirm that all tested codebase really works as expected! We had been in
> position that all tests do execute with success and all Challenges as well,
> but in reality the most of the features does not work and can't be used!
> This MUST NOT be the case and execution of tests and Challenges MUST
> guarantee the quality, the completition and full usability by end users of
> the product!"
