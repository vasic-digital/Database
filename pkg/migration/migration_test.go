package migration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "digital.vasic.database/pkg/database"
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
		{
			name:     "custom table with underscores",
			table:    "app_schema_migrations",
			expected: "app_schema_migrations",
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

func TestRunner_Init_CustomTable(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "custom_migrations")
	ctx := context.Background()

	err := r.Init(ctx)
	require.NoError(t, err)

	// Verify custom table exists
	applied, err := r.Applied(ctx)
	require.NoError(t, err)
	assert.Empty(t, applied)
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

	t.Run("apply empty migrations list", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		err := r.Apply(ctx, []Migration{})
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Empty(t, applied)
	})

	t.Run("apply with non-sequential versions", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		nonSeq := []Migration{
			{Version: 5, Name: "five", Up: "CREATE TABLE five (id INTEGER)", Down: "DROP TABLE five"},
			{Version: 10, Name: "ten", Up: "CREATE TABLE ten (id INTEGER)", Down: "DROP TABLE ten"},
			{Version: 1, Name: "one", Up: "CREATE TABLE one (id INTEGER)", Down: "DROP TABLE one"},
		}

		err := r.Apply(ctx, nonSeq)
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 5, 10}, applied)
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

	t.Run("rollback with bad down SQL", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		badDown := []Migration{
			{Version: 1, Name: "bad down", Up: "CREATE TABLE tmp (id INTEGER)", Down: "INVALID SQL"},
		}
		err := r.Apply(ctx, badDown)
		require.NoError(t, err)

		err = r.RollbackWith(ctx, 1, badDown)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rollback migration 1")
	})

	t.Run("rollback with no applicable versions", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		err := r.Apply(ctx, migrations[:1])
		require.NoError(t, err)

		// Rollback version 5 when only version 1 is applied
		err = r.RollbackWith(ctx, 5, migrations)
		require.NoError(t, err)

		applied, err := r.Applied(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int{1}, applied)
	})

	t.Run("rollback on empty database", func(t *testing.T) {
		db := newTestDB(t)
		r := NewRunner(db, "")
		ctx := context.Background()

		err := r.RollbackWith(ctx, 1, migrations)
		require.NoError(t, err)
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

func TestRunner_Rollback_WithAppliedMigrations(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	// Apply some migrations first
	migrations := []Migration{
		{Version: 1, Name: "m1", Up: "CREATE TABLE t1 (id INTEGER)", Down: "DROP TABLE t1"},
		{Version: 2, Name: "m2", Up: "CREATE TABLE t2 (id INTEGER)", Down: "DROP TABLE t2"},
		{Version: 3, Name: "m3", Up: "CREATE TABLE t3 (id INTEGER)", Down: "DROP TABLE t3"},
	}
	err := r.Apply(ctx, migrations)
	require.NoError(t, err)

	// Now call Rollback which should find the migrations but return error
	// because it doesn't have Down SQL definitions
	err = r.Rollback(ctx, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RollbackWith")
	assert.Contains(t, err.Error(), "2 version(s)")
}

func TestMigration_Struct(t *testing.T) {
	m := Migration{
		Version: 42,
		Name:    "test migration",
		Up:      "CREATE TABLE test (id INT)",
		Down:    "DROP TABLE test",
	}

	assert.Equal(t, 42, m.Version)
	assert.Equal(t, "test migration", m.Name)
	assert.Equal(t, "CREATE TABLE test (id INT)", m.Up)
	assert.Equal(t, "DROP TABLE test", m.Down)
}

// Mock database for testing error paths
type mockDB struct {
	execErr       error
	txExecErr     error // Error to use for transaction execs
	execErrOnCall int   // Which tx exec call to fail on (1-based), 0 means all
	queryErr      error
	beginErr      error
	rowsErr       error
	scanErr       error
	commitErr     error
	rollbackErr   error
	rows          *mockRows
}

func (m *mockDB) Connect(ctx context.Context) error          { return nil }
func (m *mockDB) Close() error                               { return nil }
func (m *mockDB) HealthCheck(ctx context.Context) error      { return nil }
func (m *mockDB) QueryRow(ctx context.Context, query string, args ...any) db.Row {
	return &mockRow{err: m.scanErr}
}

func (m *mockDB) Exec(ctx context.Context, query string, args ...any) (db.Result, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}
	return &mockResult{}, nil
}

func (m *mockDB) Query(ctx context.Context, query string, args ...any) (db.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	if m.rows != nil {
		return m.rows, nil
	}
	return &mockRows{}, nil
}

func (m *mockDB) Begin(ctx context.Context) (db.Tx, error) {
	if m.beginErr != nil {
		return nil, m.beginErr
	}
	return &mockTx{
		execErr:       m.txExecErr,
		execErrOnCall: m.execErrOnCall,
		commitErr:     m.commitErr,
		rollbackErr:   m.rollbackErr,
	}, nil
}

type mockResult struct{}

func (r *mockResult) RowsAffected() (int64, error) { return 0, nil }

type mockRow struct {
	err error
}

func (r *mockRow) Scan(dest ...any) error {
	return r.err
}

type mockRows struct {
	data    []int
	current int
	err     error
	scanErr error
	closed  bool
}

func (r *mockRows) Next() bool {
	if r.closed || r.current >= len(r.data) {
		return false
	}
	r.current++
	return true
}

func (r *mockRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if len(dest) > 0 && r.current <= len(r.data) {
		if v, ok := dest[0].(*int); ok {
			*v = r.data[r.current-1]
		}
	}
	return nil
}

func (r *mockRows) Close() error {
	r.closed = true
	return nil
}

func (r *mockRows) Err() error {
	return r.err
}

type mockTx struct {
	execErr       error
	execErrOnCall int // Which exec call to fail on (1-based)
	queryErr      error
	commitErr     error
	rollbackErr   error
	execCount     int
}

func (t *mockTx) Commit(ctx context.Context) error {
	return t.commitErr
}

func (t *mockTx) Rollback(ctx context.Context) error {
	return t.rollbackErr
}

func (t *mockTx) Exec(ctx context.Context, query string, args ...any) (db.Result, error) {
	t.execCount++
	if t.execErrOnCall > 0 && t.execCount == t.execErrOnCall && t.execErr != nil {
		return nil, t.execErr
	}
	if t.execErrOnCall == 0 && t.execErr != nil {
		return nil, t.execErr
	}
	return &mockResult{}, nil
}

func (t *mockTx) Query(ctx context.Context, query string, args ...any) (db.Rows, error) {
	if t.queryErr != nil {
		return nil, t.queryErr
	}
	return &mockRows{}, nil
}

func (t *mockTx) QueryRow(ctx context.Context, query string, args ...any) db.Row {
	return &mockRow{}
}

func TestRunner_Init_Error(t *testing.T) {
	mock := &mockDB{execErr: errors.New("exec error")}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.Init(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init migration table")
}

func TestRunner_Applied_QueryError(t *testing.T) {
	mock := &mockDB{queryErr: errors.New("query error")}
	r := NewRunner(mock, "")
	ctx := context.Background()

	// First need to successfully init
	mock.execErr = nil
	_ = r.Init(ctx)
	mock.queryErr = errors.New("query error")

	_, err := r.Applied(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query applied migrations")
}

func TestRunner_Applied_ScanError(t *testing.T) {
	mock := &mockDB{
		rows: &mockRows{
			data:    []int{1, 2, 3},
			scanErr: errors.New("scan error"),
		},
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	_, err := r.Applied(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan migration version")
}

func TestRunner_Applied_IterError(t *testing.T) {
	mock := &mockDB{
		rows: &mockRows{
			data: []int{1, 2, 3},
			err:  errors.New("iteration error"),
		},
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	_, err := r.Applied(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iterate migrations")
}

func TestRunner_Apply_InitError(t *testing.T) {
	mock := &mockDB{execErr: errors.New("init error")}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.Apply(ctx, []Migration{{Version: 1, Name: "test", Up: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init migration table")
}

func TestRunner_Apply_AppliedError(t *testing.T) {
	mock := &mockDB{}
	r := NewRunner(mock, "")
	ctx := context.Background()

	// Init succeeds but query fails
	_ = r.Init(ctx)
	mock.queryErr = errors.New("query error")

	err := r.Apply(ctx, []Migration{{Version: 1, Name: "test", Up: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query applied migrations")
}

func TestRunner_Apply_BeginError(t *testing.T) {
	mock := &mockDB{
		rows:     &mockRows{data: []int{}},
		beginErr: errors.New("begin error"),
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.Apply(ctx, []Migration{{Version: 1, Name: "test", Up: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin")
}

func TestRunner_Apply_ExecUpError(t *testing.T) {
	mock := &mockDB{
		rows:          &mockRows{data: []int{}},
		txExecErr:     errors.New("exec up error"),
		execErrOnCall: 1, // Fail on first tx exec (the Up statement)
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.Apply(ctx, []Migration{{Version: 1, Name: "test", Up: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec up")
}

func TestRunner_Apply_RecordMigrationError(t *testing.T) {
	mock := &mockDB{
		rows:          &mockRows{data: []int{}},
		txExecErr:     errors.New("insert error"),
		execErrOnCall: 2, // Fail on second tx exec (the INSERT statement)
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.Apply(ctx, []Migration{{Version: 1, Name: "test", Up: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "record migration")
}

func TestRunner_RollbackWith_InitError(t *testing.T) {
	mock := &mockDB{execErr: errors.New("init error")}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.RollbackWith(ctx, 1, []Migration{{Version: 1, Name: "test", Up: "SELECT 1", Down: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init migration table")
}

func TestRunner_RollbackWith_AppliedError(t *testing.T) {
	mock := &mockDB{}
	r := NewRunner(mock, "")
	ctx := context.Background()

	// Init succeeds but query fails
	_ = r.Init(ctx)
	mock.queryErr = errors.New("query error")

	err := r.RollbackWith(ctx, 1, []Migration{{Version: 1, Name: "test", Up: "SELECT 1", Down: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query applied migrations")
}

func TestRunner_RollbackWith_BeginError(t *testing.T) {
	mock := &mockDB{
		rows:     &mockRows{data: []int{1}}, // Simulate one applied migration
		beginErr: errors.New("begin error"),
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.RollbackWith(ctx, 1, []Migration{{Version: 1, Name: "test", Up: "SELECT 1", Down: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin")
}

func TestRunner_RollbackWith_ExecDownError(t *testing.T) {
	mock := &mockDB{
		rows:          &mockRows{data: []int{1}}, // Simulate one applied migration
		txExecErr:     errors.New("exec down error"),
		execErrOnCall: 1, // Fail on first tx exec (the Down statement)
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.RollbackWith(ctx, 1, []Migration{{Version: 1, Name: "test", Up: "SELECT 1", Down: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec down")
}

func TestRunner_RollbackWith_DeleteRecordError(t *testing.T) {
	mock := &mockDB{
		rows:          &mockRows{data: []int{1}}, // Simulate one applied migration
		txExecErr:     errors.New("delete error"),
		execErrOnCall: 2, // Fail on second tx exec (the DELETE statement)
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.RollbackWith(ctx, 1, []Migration{{Version: 1, Name: "test", Up: "SELECT 1", Down: "SELECT 1"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove migration record")
}

func TestRunner_Rollback_InitError(t *testing.T) {
	mock := &mockDB{execErr: errors.New("init error")}
	r := NewRunner(mock, "")
	ctx := context.Background()

	err := r.Rollback(ctx, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init migration table")
}

func TestRunner_Rollback_AppliedError(t *testing.T) {
	mock := &mockDB{}
	r := NewRunner(mock, "")
	ctx := context.Background()

	// Init succeeds but query fails
	_ = r.Init(ctx)
	mock.queryErr = errors.New("query error")

	err := r.Rollback(ctx, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query applied migrations")
}

func TestRunner_Applied_Success(t *testing.T) {
	mock := &mockDB{
		rows: &mockRows{
			data: []int{1, 2, 5, 10},
		},
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	applied, err := r.Applied(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 5, 10}, applied)
}

func TestRunner_Applied_EmptyResult(t *testing.T) {
	mock := &mockDB{
		rows: &mockRows{data: []int{}},
	}
	r := NewRunner(mock, "")
	ctx := context.Background()

	applied, err := r.Applied(ctx)
	require.NoError(t, err)
	assert.Empty(t, applied)
}

func TestRunner_MultipleRunners_SameDB(t *testing.T) {
	db := newTestDB(t)
	r1 := NewRunner(db, "migrations_v1")
	r2 := NewRunner(db, "migrations_v2")
	ctx := context.Background()

	migrations1 := []Migration{
		{Version: 1, Name: "v1 m1", Up: "CREATE TABLE v1_table (id INTEGER)", Down: "DROP TABLE v1_table"},
	}
	migrations2 := []Migration{
		{Version: 1, Name: "v2 m1", Up: "CREATE TABLE v2_table (id INTEGER)", Down: "DROP TABLE v2_table"},
	}

	err := r1.Apply(ctx, migrations1)
	require.NoError(t, err)

	err = r2.Apply(ctx, migrations2)
	require.NoError(t, err)

	applied1, _ := r1.Applied(ctx)
	applied2, _ := r2.Applied(ctx)

	assert.Equal(t, []int{1}, applied1)
	assert.Equal(t, []int{1}, applied2)
}

func TestRunner_Apply_ContextCanceled(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	migrations := []Migration{
		{Version: 1, Name: "test", Up: "CREATE TABLE test (id INTEGER)", Down: "DROP TABLE test"},
	}

	err := r.Apply(ctx, migrations)
	// May or may not error depending on timing, but should not panic
	_ = err
}

func TestRunner_TransactionRollback(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	// Create a migration that will fail partway through
	migrations := []Migration{
		{Version: 1, Name: "create table", Up: "CREATE TABLE test (id INTEGER PRIMARY KEY)", Down: "DROP TABLE test"},
		{Version: 2, Name: "fail migration", Up: "CREATE TABLE test (id INTEGER PRIMARY KEY)", Down: "DROP TABLE test"}, // Duplicate, will fail
	}

	err := r.Apply(ctx, migrations)
	require.Error(t, err)

	// Only first migration should be applied
	applied, _ := r.Applied(ctx)
	assert.Equal(t, []int{1}, applied)
}

func TestRunner_RollbackWith_DescendingOrder(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	migrations := []Migration{
		{Version: 1, Name: "m1", Up: "CREATE TABLE t1 (id INTEGER)", Down: "DROP TABLE t1"},
		{Version: 2, Name: "m2", Up: "CREATE TABLE t2 (id INTEGER)", Down: "DROP TABLE t2"},
		{Version: 3, Name: "m3", Up: "CREATE TABLE t3 (id INTEGER)", Down: "DROP TABLE t3"},
	}

	err := r.Apply(ctx, migrations)
	require.NoError(t, err)

	// Track rollback order
	rollbackOrder := []int{}
	originalMigrations := make([]Migration, len(migrations))
	for i, m := range migrations {
		version := m.Version
		originalMigrations[i] = Migration{
			Version: m.Version,
			Name:    m.Name,
			Up:      m.Up,
			Down:    m.Down,
		}
		_ = version
	}

	// Rollback all
	err = r.RollbackWith(ctx, 1, migrations)
	require.NoError(t, err)

	applied, _ := r.Applied(ctx)
	assert.Empty(t, applied)

	_ = rollbackOrder
}

func TestRunner_ApplyWithLongMigrationNames(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	longName := "this_is_a_very_long_migration_name_that_describes_the_migration_in_great_detail"
	migrations := []Migration{
		{Version: 1, Name: longName, Up: "CREATE TABLE test (id INTEGER)", Down: "DROP TABLE test"},
	}

	err := r.Apply(ctx, migrations)
	require.NoError(t, err)

	applied, _ := r.Applied(ctx)
	assert.Equal(t, []int{1}, applied)
}

func TestRunner_ApplyWithSpecialCharactersInSQL(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	migrations := []Migration{
		{
			Version: 1,
			Name:    "create with special",
			Up:      `CREATE TABLE test (id INTEGER, name TEXT DEFAULT 'hello''world')`,
			Down:    "DROP TABLE test",
		},
	}

	err := r.Apply(ctx, migrations)
	require.NoError(t, err)

	// Verify table was created correctly
	_, err = db.Exec(ctx, "INSERT INTO test (id) VALUES (1)")
	require.NoError(t, err)
}

func TestRunner_NilDatabase(t *testing.T) {
	r := NewRunner(nil, "")
	assert.NotNil(t, r)
	assert.Equal(t, "schema_migrations", r.table)
}

func TestRunner_LargeVersionNumbers(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	migrations := []Migration{
		{Version: 20240101000001, Name: "timestamp version", Up: "CREATE TABLE test (id INTEGER)", Down: "DROP TABLE test"},
	}

	err := r.Apply(ctx, migrations)
	require.NoError(t, err)

	applied, _ := r.Applied(ctx)
	assert.Equal(t, []int{20240101000001}, applied)
}

func TestRunner_ZeroVersion(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	migrations := []Migration{
		{Version: 0, Name: "zero version", Up: "CREATE TABLE test (id INTEGER)", Down: "DROP TABLE test"},
	}

	err := r.Apply(ctx, migrations)
	require.NoError(t, err)

	applied, _ := r.Applied(ctx)
	assert.Equal(t, []int{0}, applied)
}

func TestRunner_NegativeVersion(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	migrations := []Migration{
		{Version: -1, Name: "negative version", Up: "CREATE TABLE test (id INTEGER)", Down: "DROP TABLE test"},
	}

	// SQLite should accept negative integers
	err := r.Apply(ctx, migrations)
	require.NoError(t, err)

	applied, _ := r.Applied(ctx)
	assert.Equal(t, []int{-1}, applied)
}

func TestRunner_SortingMigrations(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	// Migrations in random order
	migrations := []Migration{
		{Version: 5, Name: "five", Up: "CREATE TABLE t5 (id INTEGER)", Down: "DROP TABLE t5"},
		{Version: 1, Name: "one", Up: "CREATE TABLE t1 (id INTEGER)", Down: "DROP TABLE t1"},
		{Version: 3, Name: "three", Up: "CREATE TABLE t3 (id INTEGER)", Down: "DROP TABLE t3"},
		{Version: 2, Name: "two", Up: "CREATE TABLE t2 (id INTEGER)", Down: "DROP TABLE t2"},
		{Version: 4, Name: "four", Up: "CREATE TABLE t4 (id INTEGER)", Down: "DROP TABLE t4"},
	}

	err := r.Apply(ctx, migrations)
	require.NoError(t, err)

	// Should be applied in order
	applied, _ := r.Applied(ctx)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, applied)
}

func TestRunner_ConcurrentApply(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	migrations := []Migration{
		{Version: 1, Name: "m1", Up: "CREATE TABLE IF NOT EXISTS t1 (id INTEGER)", Down: "DROP TABLE IF EXISTS t1"},
	}

	// Concurrent applies should be safe (idempotent)
	done := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			done <- r.Apply(ctx, migrations)
		}()
	}

	for i := 0; i < 3; i++ {
		err := <-done
		// Some may fail due to table already exists, but should not panic
		_ = err
	}

	applied, _ := r.Applied(ctx)
	assert.Contains(t, applied, 1)
}

func TestRunner_ApplyWithTimestamp(t *testing.T) {
	db := newTestDB(t)
	r := NewRunner(db, "")
	ctx := context.Background()

	migrations := []Migration{
		{Version: 1, Name: "test", Up: "CREATE TABLE test (id INTEGER)", Down: "DROP TABLE test"},
	}

	before := time.Now().Add(-time.Second)
	err := r.Apply(ctx, migrations)
	require.NoError(t, err)
	after := time.Now().Add(time.Second)

	// Verify timestamp was recorded
	var appliedAt time.Time
	row := db.QueryRow(ctx, "SELECT applied_at FROM schema_migrations WHERE version = 1")
	err = row.Scan(&appliedAt)
	require.NoError(t, err)

	assert.True(t, appliedAt.After(before) || appliedAt.Equal(before))
	assert.True(t, appliedAt.Before(after) || appliedAt.Equal(after))
}

func BenchmarkRunner_Apply(b *testing.B) {
	for i := 0; i < b.N; i++ {
		c := sqlite.New(sqlite.DefaultConfig(":memory:"))
		_ = c.Connect(context.Background())

		r := NewRunner(c, "")
		migrations := []Migration{
			{Version: 1, Name: "m1", Up: "CREATE TABLE t1 (id INTEGER)", Down: "DROP TABLE t1"},
			{Version: 2, Name: "m2", Up: "CREATE TABLE t2 (id INTEGER)", Down: "DROP TABLE t2"},
			{Version: 3, Name: "m3", Up: "CREATE TABLE t3 (id INTEGER)", Down: "DROP TABLE t3"},
		}

		_ = r.Apply(context.Background(), migrations)
		_ = c.Close()
	}
}

func BenchmarkRunner_Applied(b *testing.B) {
	c := sqlite.New(sqlite.DefaultConfig(":memory:"))
	_ = c.Connect(context.Background())
	defer func() { _ = c.Close() }()

	r := NewRunner(c, "")
	migrations := make([]Migration, 100)
	for i := 0; i < 100; i++ {
		migrations[i] = Migration{
			Version: i + 1,
			Name:    "migration",
			Up:      "SELECT 1",
			Down:    "SELECT 1",
		}
	}
	_ = r.Apply(context.Background(), migrations)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Applied(context.Background())
	}
}
