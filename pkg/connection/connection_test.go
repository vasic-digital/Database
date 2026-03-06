package connection

import (
	"context"
	"database/sql"
	"testing"

	"digital.vasic.database/pkg/dialect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestWrap_NilDB(t *testing.T) {
	assert.Nil(t, Wrap(nil, dialect.SQLite))
}

func TestWrap_SQLite(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	require.NotNil(t, db)
	assert.True(t, db.Dialect().IsSQLite())
	assert.Equal(t, "sqlite", db.DatabaseType())
}

func TestWrap_PostgresDialect(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.Postgres)
	require.NotNil(t, db)
	assert.True(t, db.Dialect().IsPostgres())
	assert.Equal(t, "postgres", db.DatabaseType())
}

func TestDB_ExecAndQuery_SQLite(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	ctx := context.Background()

	_, err = db.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", "hello")
	require.NoError(t, err)

	var name string
	err = db.QueryRowContext(ctx, "SELECT name FROM test WHERE id = ?", 1).Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "hello", name)
}

func TestDB_Exec_BackgroundContext(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO test (name) VALUES (?)", "world")
	require.NoError(t, err)

	var name string
	err = db.QueryRow("SELECT name FROM test WHERE id = ?", 1).Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "world", name)
}

func TestDB_Query_BackgroundContext(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, val TEXT)")
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO test (val) VALUES (?)", "a")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test (val) VALUES (?)", "b")
	require.NoError(t, err)

	rows, err := db.Query("SELECT val FROM test ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()

	var vals []string
	for rows.Next() {
		var v string
		require.NoError(t, rows.Scan(&v))
		vals = append(vals, v)
	}
	assert.Equal(t, []string{"a", "b"}, vals)
}

func TestDB_InsertReturningID_SQLite(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	ctx := context.Background()

	_, err = db.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	require.NoError(t, err)

	id, err := db.InsertReturningID(ctx, "INSERT INTO test (name) VALUES (?)", "hello")
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	id, err = db.InsertReturningID(ctx, "INSERT INTO test (name) VALUES (?)", "world")
	require.NoError(t, err)
	assert.Equal(t, int64(2), id)
}

func TestDB_TxInsertReturningID_SQLite(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	ctx := context.Background()

	_, err = db.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	require.NoError(t, err)

	tx, err := db.DB.BeginTx(ctx, nil)
	require.NoError(t, err)

	id, err := db.TxInsertReturningID(ctx, tx, "INSERT INTO test (name) VALUES (?)", "in-tx")
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	require.NoError(t, tx.Commit())

	var name string
	err = db.QueryRowContext(ctx, "SELECT name FROM test WHERE id = ?", 1).Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "in-tx", name)
}

func TestDB_TableExists_SQLite(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	ctx := context.Background()

	exists, err := db.TableExists(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = db.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)

	exists, err = db.TableExists(ctx, "test")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestDB_HealthCheck(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	assert.NoError(t, db.HealthCheck())
}

func TestDB_GetStats(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	stats := db.GetStats()
	assert.Equal(t, 0, stats.InUse)
}

func TestDB_QueryContext_Rows(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	ctx := context.Background()

	_, err = db.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	for _, name := range []string{"alice", "bob", "charlie"} {
		_, err = db.ExecContext(ctx, "INSERT INTO test (name) VALUES (?)", name)
		require.NoError(t, err)
	}

	rows, err := db.QueryContext(ctx, "SELECT name FROM test ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{"alice", "bob", "charlie"}, names)
}
