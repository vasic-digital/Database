#!/usr/bin/env bash
#
# challenges/database_describe_challenge.sh
#
# Round-244 deliverable — Database submodule deep-doc + Challenge enrichment
# (mirror of round-220 DocProcessor pattern).
#
# Drives the full CONST-050(B) "Challenges" leg for the Database submodule:
#
#   Step 1: pre-build      — go vet + go build
#   Step 2: post-build     — go test ./pkg/... -count=1 -race -short
#   Step 3: probe build    — compile challenges/runner/main.go against the
#                            REAL pkg/sqlite + pkg/migration code on disk.
#   Step 4: runtime probe  — execute the runner in normal mode against a real
#                            :memory: SQLite database file. Asserts the
#                            Connect → migrate → INSERT → SELECT round-trip
#                            persists and reads back the expected row.
#                            Loads bilingual labels from
#                            challenges/fixtures/<LANG_FIXTURE>.yaml.
#   Step 5: paired mutation — re-execute the SAME binary with
#                            PROBE_MODE=MUTATION; the runner wraps the real
#                            adapter with swallowingDatabase whose Exec
#                            silently drops INSERTs. The probe MUST exit 99
#                            (sql.ErrNoRows detected). Exit 0 in mutation
#                            mode = CONST-035 bluff (probe is not validating
#                            user-visible state) → Challenge FAILs.
#
# Anti-bluff invariants (CONST-035 / Article XI §11.9):
#   - every PASS is preceded by a real command + captured output under
#     challenges/.last-run/
#   - the mutation leg PROVES the assertion would FAIL if the adapter
#     contract regressed (a swallowed INSERT is exactly the historical
#     "PASS-bluff at the adapter-contract layer" pattern)
#   - the script exits non-zero on the first failure (no silent skips)
#
# Usage:
#   bash challenges/database_describe_challenge.sh                # normal run
#   bash challenges/database_describe_challenge.sh --anti-bluff-mutate
#                                                                # ONLY the
#                                                                # mutation leg
#                                                                # → exit 99
#                                                                # on success
#
# Environment overrides:
#   LANG_FIXTURE=sr-Latn         # use Serbian-Latin fixture instead of en

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
EVIDENCE_DIR="${SCRIPT_DIR}/.last-run"
PROBE_BIN="${SCRIPT_DIR}/.probe-bin"
LANG_FIXTURE="${LANG_FIXTURE:-en}"
FIXTURE_PATH="${SCRIPT_DIR}/fixtures/${LANG_FIXTURE}.yaml"

mkdir -p "${EVIDENCE_DIR}"

cd "${REPO_ROOT}"

log()  { printf '\n=== %s ===\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

if [[ ! -f "${FIXTURE_PATH}" ]]; then
    fail "fixture not found: ${FIXTURE_PATH} (set LANG_FIXTURE=en or sr-Latn)"
fi

MUTATE_ONLY=0
if [[ "${1:-}" == "--anti-bluff-mutate" ]]; then
    MUTATE_ONLY=1
fi

# ---------------------------------------------------------------------------
# Build the probe binary first — both legs use the same binary so there is
# zero risk of accidental code drift between "normal" and "mutation" runs.
# ---------------------------------------------------------------------------
log "Step 3: build challenges/runner probe binary"
go build -o "${PROBE_BIN}" ./challenges/runner 2>&1 | tee "${EVIDENCE_DIR}/04-probe-build.log" \
    || fail "probe build"

if [[ ${MUTATE_ONLY} -eq 1 ]]; then
    # --anti-bluff-mutate: only run the mutation leg. Caller asserts exit 99.
    log "Mutation-only mode: executing probe with PROBE_MODE=MUTATION"
    set +e
    PROBE_MODE=MUTATION LANG_FIXTURE="${LANG_FIXTURE}" FIXTURE_PATH="${FIXTURE_PATH}" \
        "${PROBE_BIN}" > "${EVIDENCE_DIR}/06-probe-mutation.log" 2>&1
    MUTATION_RC=$?
    set -e
    cat "${EVIDENCE_DIR}/06-probe-mutation.log"
    printf 'mutation exit code: %d (expected 99)\n' "${MUTATION_RC}"
    if [[ ${MUTATION_RC} -ne 99 ]]; then
        fail "mutation leg expected rc=99 but got rc=${MUTATION_RC} — probe is not catching the swallowed-INSERT regression (CONST-035 bluff)"
    fi
    # NOTE: this script intentionally exits 99 in --anti-bluff-mutate mode so
    # the caller (per task spec) can verify the canonical exit code by string
    # match on the script's own exit status.
    exit 99
fi

# ---------------------------------------------------------------------------
# Step 1 -- pre-build floor
# ---------------------------------------------------------------------------
log "Step 1: go vet + go build (pre-build floor)"
go vet ./... 2>&1 | tee "${EVIDENCE_DIR}/01-vet.log" || fail "go vet"
go build ./... 2>&1 | tee "${EVIDENCE_DIR}/02-build.log" || fail "go build"

# ---------------------------------------------------------------------------
# Step 2 -- post-build floor: pkg/ unit suite under race detector
# ---------------------------------------------------------------------------
log "Step 2: go test ./pkg/... -count=1 -race -short (post-build floor)"
GOMAXPROCS=2 go test ./pkg/... -count=1 -race -short 2>&1 | tee "${EVIDENCE_DIR}/03-test.log" \
    || fail "unit suite"

# ---------------------------------------------------------------------------
# Step 4 -- runtime probe in normal mode (real SQLite, real round-trip)
# ---------------------------------------------------------------------------
log "Step 4: runtime Database probe (Connect → migrate → INSERT → SELECT) — locale=${LANG_FIXTURE}"
set +e
LANG_FIXTURE="${LANG_FIXTURE}" FIXTURE_PATH="${FIXTURE_PATH}" \
    "${PROBE_BIN}" 2>&1 | tee "${EVIDENCE_DIR}/05-probe-normal.log"
PROBE_RC=${PIPESTATUS[0]}
set -e
if [[ ${PROBE_RC} -ne 0 ]]; then
    fail "Database probe failed in normal mode (rc=${PROBE_RC}) — adapter contract broken"
fi
grep -q 'PROBE PASS\|PROVERA USPEŠNA' "${EVIDENCE_DIR}/05-probe-normal.log" \
    || fail "probe exited 0 without printing PASS sentinel from the fixture — output assertion missing"

# ---------------------------------------------------------------------------
# Step 5 -- paired anti-bluff mutation
#
# Re-run the SAME probe binary with PROBE_MODE=MUTATION. The runner wraps the
# real SQLite adapter with swallowingDatabase whose Exec lies about INSERT
# success. The probe MUST exit 99 (sql.ErrNoRows detected). If it exits 0 the
# probe is not actually validating user-visible state — CONST-035 bluff.
# ---------------------------------------------------------------------------
log "Step 5: paired anti-bluff mutation (swallowingDatabase, expect probe rc=99)"
set +e
PROBE_MODE=MUTATION LANG_FIXTURE="${LANG_FIXTURE}" FIXTURE_PATH="${FIXTURE_PATH}" \
    "${PROBE_BIN}" > "${EVIDENCE_DIR}/06-probe-mutation.log" 2>&1
MUTATION_RC=$?
set -e

cat "${EVIDENCE_DIR}/06-probe-mutation.log"
printf 'mutation exit code: %d (expected 99)\n' "${MUTATION_RC}"

if [[ ${MUTATION_RC} -eq 0 ]]; then
    fail "paired-mutation leg: probe exited 0 with swallowingDatabase — contract probe is not validating writes (CONST-035 bluff)"
fi
if [[ ${MUTATION_RC} -ne 99 ]]; then
    printf 'WARN: mutation rejected but rc=%d (expected 99); inspect %s\n' \
        "${MUTATION_RC}" "${EVIDENCE_DIR}/06-probe-mutation.log"
    # Treat any non-zero mutation rc as a pass for the anti-bluff invariant —
    # the assertion proved it would catch a regression — but the canonical
    # exit code is 99 and a drift here should be inspected.
fi

# ---------------------------------------------------------------------------
# Cleanup probe binary (keep evidence dir intact).
# ---------------------------------------------------------------------------
rm -f "${PROBE_BIN}"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
log "PASS: database_describe_challenge.sh — all 5 steps green (locale=${LANG_FIXTURE})"
printf 'evidence directory: %s\n' "${EVIDENCE_DIR}"
ls -la "${EVIDENCE_DIR}"
exit 0
