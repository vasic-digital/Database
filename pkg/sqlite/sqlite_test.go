package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "digital.vasic.database/pkg/database"
)

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "path is set",
			path: "/tmp/test.db",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "/tmp/test.db", cfg.Path)
			},
		},
		{
			name: "journal mode is WAL",
			path: ":memory:",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "WAL", cfg.JournalMode)
			},
		},
		{
			name: "busy timeout is set",
			path: ":memory:",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 5*time.Second, cfg.BusyTimeout)
			},
		},
		{
			name: "max open conns defaults to 1",
			path: ":memory:",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 1, cfg.MaxOpenConns)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig(tt.path)
			tt.check(t, cfg)
		})
	}
}

func TestNew_NilConfigUsesMemory(t *testing.T) {
	c := New(nil)
	require.NotNil(t, c)
	assert.Equal(t, ":memory:", c.config.Path)
}

func TestClient_ConnectAndClose_InMemory(t *testing.T) {
	c := New(DefaultConfig(":memory:"))
	ctx := context.Background()

	err := c.Connect(ctx)
	require.NoError(t, err)
	assert.NotNil(t, c.DB())

	err = c.Close()
	require.NoError(t, err)
	assert.Nil(t, c.db)
}

func TestClient_ConnectAndClose_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	c := New(DefaultConfig(path))
	ctx := context.Background()

	err := c.Connect(ctx)
	require.NoError(t, err)

	// Verify file was created.
	_, err = os.Stat(path)
	require.NoError(t, err)

	err = c.Close()
	require.NoError(t, err)
}

func TestClient_CloseWithoutConnect(t *testing.T) {
	c := New(nil)
	err := c.Close()
	assert.NoError(t, err)
}

func TestClient_HealthCheck(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
		wantErr   bool
	}{
		{
			name:      "connected database passes health check",
			connected: true,
			wantErr:   false,
		},
		{
			name:      "disconnected database fails health check",
			connected: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(DefaultConfig(":memory:"))
			ctx := context.Background()

			if tt.connected {
				err := c.Connect(ctx)
				require.NoError(t, err)
				defer func() { _ = c.Close() }()
			}

			err := c.HealthCheck(ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_ExecAndQuery(t *testing.T) {
	c := New(DefaultConfig(":memory:"))
	ctx := context.Background()

	err := c.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	// Create table.
	_, err = c.Exec(ctx,
		"CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT NOT NULL)")
	require.NoError(t, err)

	// Insert rows.
	tests := []struct {
		name string
		id   int
		val  string
	}{
		{name: "first", id: 1, val: "alice"},
		{name: "second", id: 2, val: "bob"},
		{name: "third", id: 3, val: "charlie"},
	}

	for _, tt := range tests {
		t.Run("insert_"+tt.name, func(t *testing.T) {
			res, err := c.Exec(ctx,
				"INSERT INTO test (id, name) VALUES (?, ?)", tt.id, tt.val)
			require.NoError(t, err)

			n, err := res.RowsAffected()
			require.NoError(t, err)
			assert.Equal(t, int64(1), n)
		})
	}

	// Query all rows.
	t.Run("query all", func(t *testing.T) {
		rows, err := c.Query(ctx, "SELECT id, name FROM test ORDER BY id")
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()

		var count int
		for rows.Next() {
			var id int
			var name string
			err := rows.Scan(&id, &name)
			require.NoError(t, err)
			assert.Equal(t, tests[count].id, id)
			assert.Equal(t, tests[count].val, name)
			count++
		}
		assert.NoError(t, rows.Err())
		assert.Equal(t, 3, count)
	})

	// QueryRow.
	t.Run("query row", func(t *testing.T) {
		row := c.QueryRow(ctx, "SELECT name FROM test WHERE id = ?", 2)
		var name string
		err := row.Scan(&name)
		require.NoError(t, err)
		assert.Equal(t, "bob", name)
	})
}

func TestClient_Transaction(t *testing.T) {
	c := New(DefaultConfig(":memory:"))
	ctx := context.Background()

	err := c.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	_, err = c.Exec(ctx,
		"CREATE TABLE txtest (id INTEGER PRIMARY KEY, val TEXT)")
	require.NoError(t, err)

	t.Run("commit", func(t *testing.T) {
		tx, err := c.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.Exec(ctx,
			"INSERT INTO txtest (id, val) VALUES (?, ?)", 1, "committed")
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		// Verify committed.
		row := c.QueryRow(ctx, "SELECT val FROM txtest WHERE id = ?", 1)
		var val string
		err = row.Scan(&val)
		require.NoError(t, err)
		assert.Equal(t, "committed", val)
	})

	t.Run("rollback", func(t *testing.T) {
		tx, err := c.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.Exec(ctx,
			"INSERT INTO txtest (id, val) VALUES (?, ?)", 2, "rolled_back")
		require.NoError(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		// Verify not present.
		row := c.QueryRow(ctx, "SELECT val FROM txtest WHERE id = ?", 2)
		var val string
		err = row.Scan(&val)
		assert.Error(t, err) // no rows
	})

	t.Run("tx query", func(t *testing.T) {
		tx, err := c.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		rows, err := tx.Query(ctx, "SELECT id, val FROM txtest")
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()

		var count int
		for rows.Next() {
			count++
			var id int
			var val string
			err := rows.Scan(&id, &val)
			require.NoError(t, err)
		}
		assert.NoError(t, rows.Err())
		assert.Equal(t, 1, count) // only the committed row
	})

	t.Run("tx query row", func(t *testing.T) {
		tx, err := c.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		row := tx.QueryRow(ctx, "SELECT val FROM txtest WHERE id = ?", 1)
		var val string
		err = row.Scan(&val)
		require.NoError(t, err)
		assert.Equal(t, "committed", val)
	})
}

func TestClient_InterfaceCompliance(t *testing.T) {
	// Ensure Client implements database.Database.
	var _ db.Database = (*Client)(nil)

	// Ensure sqlTx implements database.Tx.
	var _ db.Tx = (*sqlTx)(nil)

	// Ensure sqlRow implements database.Row.
	var _ db.Row = (*sqlRow)(nil)

	// Ensure sqlRows implements database.Rows.
	var _ db.Rows = (*sqlRows)(nil)

	// Ensure sqlResult implements database.Result.
	var _ db.Result = (*sqlResult)(nil)
}

func TestClient_JournalMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected string
	}{
		{name: "WAL", mode: "WAL", expected: "WAL"},
		{name: "DELETE", mode: "DELETE", expected: "DELETE"},
		{name: "empty defaults to WAL", mode: "", expected: "WAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{config: &Config{JournalMode: tt.mode}}
			assert.Equal(t, tt.expected, c.journalMode())
		})
	}
}

func TestClient_BusyTimeoutMs(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected int64
	}{
		{name: "5 seconds", timeout: 5 * time.Second, expected: 5000},
		{name: "1 second", timeout: time.Second, expected: 1000},
		{name: "zero defaults to 5000", timeout: 0, expected: 5000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{config: &Config{BusyTimeout: tt.timeout}}
			assert.Equal(t, tt.expected, c.busyTimeoutMs())
		})
	}
}
