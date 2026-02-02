// Package migration provides schema migration management with version tracking,
// forward application, and rollback support.
package migration

import (
	"context"
	"fmt"
	"sort"
	"time"

	db "digital.vasic.database/pkg/database"
)

// Migration represents a single schema migration.
type Migration struct {
	// Version is a unique, monotonically increasing migration identifier.
	Version int

	// Name is a human-readable description of the migration.
	Name string

	// Up contains the SQL to apply the migration.
	Up string

	// Down contains the SQL to reverse the migration.
	Down string
}

// Runner applies and rolls back migrations against a database.
type Runner struct {
	db    db.Database
	table string
}

// NewRunner creates a new migration runner. The table parameter sets the name
// of the migration tracking table (defaults to "schema_migrations").
func NewRunner(database db.Database, table string) *Runner {
	if table == "" {
		table = "schema_migrations"
	}
	return &Runner{db: database, table: table}
}

// Init creates the migration tracking table if it does not exist.
func (r *Runner) Init(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version    INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TIMESTAMP NOT NULL
		)
	`, r.table)

	_, err := r.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("init migration table: %w", err)
	}
	return nil
}

// Applied returns the set of migration versions that have already been
// applied, sorted ascending.
func (r *Runner) Applied(ctx context.Context) ([]int, error) {
	query := fmt.Sprintf(
		"SELECT version FROM %s ORDER BY version ASC", r.table,
	)

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan migration version: %w", err)
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migrations: %w", err)
	}
	return versions, nil
}

// Apply runs all pending migrations in version order. It first initializes
// the tracking table, then applies each migration whose version has not yet
// been recorded.
func (r *Runner) Apply(ctx context.Context, migrations []Migration) error {
	if err := r.Init(ctx); err != nil {
		return err
	}

	applied, err := r.Applied(ctx)
	if err != nil {
		return err
	}
	appliedSet := make(map[int]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	// Sort migrations by version.
	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	for _, m := range sorted {
		if appliedSet[m.Version] {
			continue
		}
		if err := r.applyOne(ctx, m); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", m.Version, m.Name, err)
		}
	}
	return nil
}

// Rollback reverses all migrations with version >= the given version, in
// reverse order.
func (r *Runner) Rollback(ctx context.Context, version int) error {
	if err := r.Init(ctx); err != nil {
		return err
	}

	applied, err := r.Applied(ctx)
	if err != nil {
		return err
	}

	// Find migrations to roll back (version >= target), sorted descending.
	var toRollback []int
	for _, v := range applied {
		if v >= version {
			toRollback = append(toRollback, v)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(toRollback)))

	// We need the Down SQL, but we only have versions. The caller should
	// provide migrations; for this method we require them as a map.
	return fmt.Errorf(
		"rollback: use RollbackWith to provide migration definitions for %d version(s)",
		len(toRollback),
	)
}

// RollbackWith reverses all applied migrations with version >= target, using
// the provided migration definitions for Down SQL.
func (r *Runner) RollbackWith(
	ctx context.Context, version int, migrations []Migration,
) error {
	if err := r.Init(ctx); err != nil {
		return err
	}

	applied, err := r.Applied(ctx)
	if err != nil {
		return err
	}

	migMap := make(map[int]Migration, len(migrations))
	for _, m := range migrations {
		migMap[m.Version] = m
	}

	// Find versions to roll back.
	var toRollback []int
	for _, v := range applied {
		if v >= version {
			toRollback = append(toRollback, v)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(toRollback)))

	for _, v := range toRollback {
		m, ok := migMap[v]
		if !ok {
			return fmt.Errorf("rollback: no migration definition for version %d", v)
		}
		if m.Down == "" {
			return fmt.Errorf("rollback: migration %d (%s) has no Down SQL", v, m.Name)
		}
		if err := r.rollbackOne(ctx, m); err != nil {
			return fmt.Errorf(
				"rollback migration %d (%s): %w", m.Version, m.Name, err,
			)
		}
	}
	return nil
}

func (r *Runner) applyOne(ctx context.Context, m Migration) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	if _, err := tx.Exec(ctx, m.Up); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("exec up: %w", err)
	}

	insert := fmt.Sprintf(
		"INSERT INTO %s (version, name, applied_at) VALUES (?, ?, ?)",
		r.table,
	)
	if _, err := tx.Exec(ctx, insert, m.Version, m.Name, time.Now().UTC()); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *Runner) rollbackOne(ctx context.Context, m Migration) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	if _, err := tx.Exec(ctx, m.Down); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("exec down: %w", err)
	}

	del := fmt.Sprintf("DELETE FROM %s WHERE version = ?", r.table)
	if _, err := tx.Exec(ctx, del, m.Version); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("remove migration record: %w", err)
	}

	return tx.Commit(ctx)
}
