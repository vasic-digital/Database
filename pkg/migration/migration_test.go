package migration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.database/pkg/sqlite"
)

func newTestDB(t *testing.T) *sqlite.Client {
	t.Helper()
	c := sqlite.New(sqlite.DefaultConfig(":memory:"))
	err := c.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestNewRunner_DefaultTable(t *testing.T) {
	tests := []struct {
		name     string
		table    string
		expected string
	}{
		{
			name:     "empty uses default",
			table:    "",
			expected: "schema_migrations",
		},
		{
			name:     "custom table name",
			table:    "my_migrations",
			expected: "my_migrations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRunner(nil, tt.table)
			assert.Equal(t, tt.expected, r.table)
		})
	}
}

func TestRunner_Init(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	err := r.Init(ctx)
	require.NoError(t, err)

	// Verify table exists by querying it.
	applied, err := r.Applied(ctx)
	require.NoError(t, err)
	assert.Empty(t, applied)

	// Double init is safe.
	err = r.Init(ctx)
	require.NoError(t, err)
}

func TestRunner_Apply(t *testing.T) {
	migrations := []Migration{
		{
			Version: 1,
			Name:    "create users",
			Up:      "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)",
			Down:    "DROP TABLE users",
		},
		{
			Version: 2,
			Name:    "create posts",
			Up:      "CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER, title TEXT)",
			Down:    "DROP TABLE posts",
		},
	}

	t.Run("apply all migrations", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		err := r.Apply(ctx, migrations)
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, applied)

		// Verify tables exist.
		_, err = db.Exec(ctx, "INSERT INTO users (id, name) VALUES (1, 'alice')")
		require.NoError(t, err)

		_, err = db.Exec(ctx,
			"INSERT INTO posts (id, user_id, title) VALUES (1, 1, 'hello')")
		require.NoError(t, err)
	})

	t.Run("apply is idempotent", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		err := r.Apply(ctx, migrations)
		require.NoError(t, err)

		// Apply again should be a no-op.
		err = r.Apply(ctx, migrations)
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, applied)
	})

	t.Run("apply incremental", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		// Apply first migration.
		err := r.Apply(ctx, migrations[:1])
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int{1}, applied)

		// Apply all migrations.
		err = r.Apply(ctx, migrations)
		require.NoError(t, err)

		applied, err = r.Applied(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, applied)
	})

	t.Run("apply out of order", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		// Provide migrations in reverse order.
		reversed := []Migration{migrations[1], migrations[0]}
		err := r.Apply(ctx, reversed)
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, applied)
	})

	t.Run("apply with bad SQL fails", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		bad := []Migration{
			{Version: 1, Name: "bad", Up: "INVALID SQL STATEMENT"},
		}
		err := r.Apply(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "apply migration 1")
	})
}

func TestRunner_RollbackWith(t *testing.T) {
	migrations := []Migration{
		{
			Version: 1,
			Name:    "create accounts",
			Up:      "CREATE TABLE accounts (id INTEGER PRIMARY KEY, email TEXT)",
			Down:    "DROP TABLE accounts",
		},
		{
			Version: 2,
			Name:    "create orders",
			Up:      "CREATE TABLE orders (id INTEGER PRIMARY KEY, account_id INTEGER)",
			Down:    "DROP TABLE orders",
		},
		{
			Version: 3,
			Name:    "create items",
			Up:      "CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)",
			Down:    "DROP TABLE items",
		},
	}

	t.Run("rollback latest", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		err := r.Apply(ctx, migrations)
		require.NoError(t, err)

		err = r.RollbackWith(ctx, 3, migrations)
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, applied)
	})

	t.Run("rollback multiple", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		err := r.Apply(ctx, migrations)
		require.NoError(t, err)

		err = r.RollbackWith(ctx, 2, migrations)
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int{1}, applied)
	})

	t.Run("rollback all", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		err := r.Apply(ctx, migrations)
		require.NoError(t, err)

		err = r.RollbackWith(ctx, 1, migrations)
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Empty(t, applied)
	})

	t.Run("rollback missing definition", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		err := r.Apply(ctx, migrations)
		require.NoError(t, err)

		// Provide incomplete migration list.
		err = r.RollbackWith(ctx, 2, migrations[:1])
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no migration definition")
	})

	t.Run("rollback missing down SQL", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		noDown := []Migration{
			{Version: 1, Name: "no down", Up: "CREATE TABLE tmp (id INTEGER)", Down: ""},
		}
		err := r.Apply(ctx, noDown)
		require.NoError(t, err)

		err = r.RollbackWith(ctx, 1, noDown)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no Down SQL")
	})
}

func TestRunner_Rollback_WithoutDefinitions(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	err := r.Init(ctx)
	require.NoError(t, err)

	err = r.Rollback(ctx, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RollbackWith")
}
