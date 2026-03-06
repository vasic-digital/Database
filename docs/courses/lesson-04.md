# Lesson 4: Schema Migrations with Version Tracking

## Learning Objectives

- Build a transactional migration runner that guarantees atomic schema changes
- Implement version tracking with a `schema_migrations` table
- Support both forward (Up) and rollback (Down) migrations

## Key Concepts

- **Transactional Migrations**: Each migration runs inside a database transaction. The schema change and the tracking record insert/delete are committed together, ensuring a migration is either fully applied or not applied at all.
- **Version Tracking**: The `schema_migrations` table uses the migration version as the primary key. The runner queries this table to determine which migrations have been applied.
- **Bidirectional Support**: Each migration defines both `Up` (apply) and `Down` (rollback) SQL, enabling controlled rollback of schema changes.

## Code Walkthrough

### Source: `pkg/migration/migration.go`

The migration runner follows this sequence for each migration:

1. Begin a database transaction
2. Execute the Up (or Down) SQL
3. Insert (or delete) the migration tracking record
4. Commit the transaction (or rollback on error)

Migrations are defined as structs with `Version`, `Description`, `Up`, and `Down` fields. The runner sorts migrations by version and applies them in order, skipping already-applied versions.

The runner uses the `database.Database` interface, making it backend-agnostic. It works with both PostgreSQL and SQLite adapters.

### Source: `pkg/migration/migration_test.go`

Tests cover:
- Fresh migration run (all migrations applied in order)
- Idempotent re-run (already-applied migrations are skipped)
- Rollback of specific versions
- Transaction rollback on SQL error (partial migration prevented)
- Version ordering verification

## Practice Exercise

1. Read `pkg/migration/migration.go` and trace the apply-and-track sequence for a single migration. Identify where the transaction boundary is.
2. Define three migrations (create table, add column, add index) and run them against an in-memory SQLite database. Verify the `schema_migrations` table contains the correct versions.
3. Write a test that intentionally introduces a SQL error in a migration's Up field. Verify that neither the schema change nor the tracking record is applied (transaction atomicity).
