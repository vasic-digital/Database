#!/usr/bin/env bash
# database_functionality_challenge.sh - Validates Database module core functionality
# Checks connection pool, migrations, repository pattern, query builder, PostgreSQL, SQLite
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MODULE_NAME="Database"

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

echo "=== ${MODULE_NAME} Functionality Challenge ==="
echo ""

# --- Section 1: Required packages ---
echo "Section 1: Required packages (7)"

for pkg in database migration pool postgres query repository sqlite; do
    echo "Test: Package pkg/${pkg} exists"
    if [ -d "${MODULE_DIR}/pkg/${pkg}" ]; then
        pass "Package pkg/${pkg} exists"
    else
        fail "Package pkg/${pkg} missing"
    fi
done

# --- Section 2: Core database interface ---
echo ""
echo "Section 2: Core database interface"

echo "Test: Database interface exists"
if grep -q "type Database interface" "${MODULE_DIR}/pkg/database/"*.go 2>/dev/null; then
    pass "Database interface exists"
else
    fail "Database interface missing"
fi

echo "Test: Tx interface exists"
if grep -q "type Tx interface" "${MODULE_DIR}/pkg/database/"*.go 2>/dev/null; then
    pass "Tx (transaction) interface exists"
else
    fail "Tx (transaction) interface missing"
fi

echo "Test: Row interface exists"
if grep -q "type Row interface" "${MODULE_DIR}/pkg/database/"*.go 2>/dev/null; then
    pass "Row interface exists"
else
    fail "Row interface missing"
fi

echo "Test: Rows interface exists"
if grep -q "type Rows interface" "${MODULE_DIR}/pkg/database/"*.go 2>/dev/null; then
    pass "Rows interface exists"
else
    fail "Rows interface missing"
fi

echo "Test: Result interface exists"
if grep -q "type Result interface" "${MODULE_DIR}/pkg/database/"*.go 2>/dev/null; then
    pass "Result interface exists"
else
    fail "Result interface missing"
fi

echo "Test: Database Config struct exists"
if grep -q "type Config struct" "${MODULE_DIR}/pkg/database/"*.go 2>/dev/null; then
    pass "Database Config struct exists"
else
    fail "Database Config struct missing"
fi

# --- Section 3: Connection pool ---
echo ""
echo "Section 3: Connection pool"

echo "Test: Pool interface exists"
if grep -q "type Pool interface" "${MODULE_DIR}/pkg/pool/"*.go 2>/dev/null; then
    pass "Pool interface exists"
else
    fail "Pool interface missing"
fi

echo "Test: GenericPool struct exists"
if grep -q "type GenericPool struct" "${MODULE_DIR}/pkg/pool/"*.go 2>/dev/null; then
    pass "GenericPool struct exists"
else
    fail "GenericPool struct missing"
fi

echo "Test: PoolConfig struct exists"
if grep -q "type PoolConfig struct" "${MODULE_DIR}/pkg/pool/"*.go 2>/dev/null; then
    pass "PoolConfig struct exists"
else
    fail "PoolConfig struct missing"
fi

echo "Test: PoolStats struct exists"
if grep -q "type PoolStats struct" "${MODULE_DIR}/pkg/pool/"*.go 2>/dev/null; then
    pass "PoolStats struct exists"
else
    fail "PoolStats struct missing"
fi

# --- Section 4: Migrations ---
echo ""
echo "Section 4: Migrations"

echo "Test: Migration struct exists"
if grep -q "type Migration struct" "${MODULE_DIR}/pkg/migration/"*.go 2>/dev/null; then
    pass "Migration struct exists"
else
    fail "Migration struct missing"
fi

echo "Test: Runner struct exists"
if grep -q "type Runner struct" "${MODULE_DIR}/pkg/migration/"*.go 2>/dev/null; then
    pass "Migration Runner struct exists"
else
    fail "Migration Runner struct missing"
fi

# --- Section 5: Query builder ---
echo ""
echo "Section 5: Query builder"

echo "Test: Builder struct exists"
if grep -q "type Builder struct" "${MODULE_DIR}/pkg/query/"*.go 2>/dev/null; then
    pass "Query Builder struct exists"
else
    fail "Query Builder struct missing"
fi

echo "Test: Condition interface exists"
if grep -q "type Condition interface" "${MODULE_DIR}/pkg/query/"*.go 2>/dev/null; then
    pass "Condition interface exists"
else
    fail "Condition interface missing"
fi

# --- Section 6: Repository pattern ---
echo ""
echo "Section 6: Repository pattern"

echo "Test: ListOptions struct exists"
if grep -q "type ListOptions struct" "${MODULE_DIR}/pkg/repository/"*.go 2>/dev/null; then
    pass "ListOptions struct exists"
else
    fail "ListOptions struct missing"
fi

echo "Test: WhereClause struct exists"
if grep -q "type WhereClause struct" "${MODULE_DIR}/pkg/repository/"*.go 2>/dev/null; then
    pass "WhereClause struct exists"
else
    fail "WhereClause struct missing"
fi

# --- Section 7: PostgreSQL client ---
echo ""
echo "Section 7: PostgreSQL client"

echo "Test: PostgreSQL Client struct exists"
if grep -q "type Client struct" "${MODULE_DIR}/pkg/postgres/"*.go 2>/dev/null; then
    pass "PostgreSQL Client struct exists"
else
    fail "PostgreSQL Client struct missing"
fi

echo "Test: PostgreSQL Config struct exists"
if grep -q "type Config struct" "${MODULE_DIR}/pkg/postgres/"*.go 2>/dev/null; then
    pass "PostgreSQL Config struct exists"
else
    fail "PostgreSQL Config struct missing"
fi

# --- Section 8: SQLite client ---
echo ""
echo "Section 8: SQLite client"

echo "Test: SQLite Client struct exists"
if grep -q "type Client struct" "${MODULE_DIR}/pkg/sqlite/"*.go 2>/dev/null; then
    pass "SQLite Client struct exists"
else
    fail "SQLite Client struct missing"
fi

echo "Test: SQLite Config struct exists"
if grep -q "type Config struct" "${MODULE_DIR}/pkg/sqlite/"*.go 2>/dev/null; then
    pass "SQLite Config struct exists"
else
    fail "SQLite Config struct missing"
fi

# --- Section 9: Source structure completeness ---
echo ""
echo "Section 9: Source structure"

echo "Test: Each package has non-test Go source files"
all_have_source=true
for pkg in database migration pool postgres query repository sqlite; do
    non_test=$(find "${MODULE_DIR}/pkg/${pkg}" -name "*.go" ! -name "*_test.go" -type f 2>/dev/null | wc -l)
    if [ "$non_test" -eq 0 ]; then
        fail "Package pkg/${pkg} has no non-test Go files"
        all_have_source=false
    fi
done
if [ "$all_have_source" = true ]; then
    pass "All packages have non-test Go source files"
fi

echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="
[ "${FAIL}" -eq 0 ] && exit 0 || exit 1
