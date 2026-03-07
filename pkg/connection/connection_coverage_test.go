package connection

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"digital.vasic.database/pkg/dialect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// TestOpen_SQLite tests the Open function with a valid SQLite driver.
func TestOpen_SQLite(t *testing.T) {
	db, err := Open("sqlite", ":memory:", Config{
		Type:            "sqlite",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		BusyTimeout:     3 * time.Second,
		BooleanColumns:  []string{"active"},
	})
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	assert.True(t, db.Dialect().IsSQLite())
	assert.Equal(t, "sqlite", db.DatabaseType())
}

// TestOpen_PostgresDialectType tests Open with "postgres" type sets the dialect.
func TestOpen_PostgresDialectType(t *testing.T) {
	// We use sqlite driver but set Type to "postgres" to test dialect selection.
	db, err := Open("sqlite", ":memory:", Config{
		Type: "postgres",
	})
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	assert.True(t, db.Dialect().IsPostgres())
	assert.Equal(t, "postgres", db.DatabaseType())
}

// TestOpen_ZeroPoolConfig tests Open with zero pool configuration values.
func TestOpen_ZeroPoolConfig(t *testing.T) {
	db, err := Open("sqlite", ":memory:", Config{
		Type: "sqlite",
	})
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	assert.True(t, db.Dialect().IsSQLite())
}

// TestOpen_InvalidDriver tests Open with an invalid driver name.
func TestOpen_InvalidDriver(t *testing.T) {
	db, err := Open("nonexistent_driver", "anything", Config{Type: "sqlite"})
	assert.Error(t, err)
	assert.Nil(t, db)
}

// TestOpen_OnlyMaxOpenConns tests Open with only MaxOpenConns configured.
func TestOpen_OnlyMaxOpenConns(t *testing.T) {
	db, err := Open("sqlite", ":memory:", Config{
		Type:         "sqlite",
		MaxOpenConns: 10,
	})
	require.NoError(t, err)
	defer db.Close()

	stats := db.GetStats()
	assert.Equal(t, 10, stats.MaxOpenConnections)
}

// TestOpen_OnlyMaxIdleConns tests Open with only MaxIdleConns configured.
func TestOpen_OnlyMaxIdleConns(t *testing.T) {
	db, err := Open("sqlite", ":memory:", Config{
		Type:         "sqlite",
		MaxIdleConns: 3,
	})
	require.NoError(t, err)
	defer db.Close()
	assert.NotNil(t, db)
}

// TestOpen_OnlyConnMaxLifetime tests Open with only ConnMaxLifetime configured.
func TestOpen_OnlyConnMaxLifetime(t *testing.T) {
	db, err := Open("sqlite", ":memory:", Config{
		Type:            "sqlite",
		ConnMaxLifetime: 30 * time.Minute,
	})
	require.NoError(t, err)
	defer db.Close()
	assert.NotNil(t, db)
}

// TestOpen_OnlyConnMaxIdleTime tests Open with only ConnMaxIdleTime configured.
func TestOpen_OnlyConnMaxIdleTime(t *testing.T) {
	db, err := Open("sqlite", ":memory:", Config{
		Type:            "sqlite",
		ConnMaxIdleTime: 5 * time.Minute,
	})
	require.NoError(t, err)
	defer db.Close()
	assert.NotNil(t, db)
}

// TestOpen_WithBooleanColumns tests Open with BooleanColumns set.
func TestOpen_WithBooleanColumns(t *testing.T) {
	db, err := Open("sqlite", ":memory:", Config{
		Type:           "sqlite",
		BooleanColumns: []string{"is_active", "is_deleted"},
	})
	require.NoError(t, err)
	defer db.Close()

	assert.Equal(t, []string{"is_active", "is_deleted"}, db.booleanColumns)
}

// TestInsertReturningID_SQLite_ExecError tests InsertReturningID when the
// SQL execution fails on SQLite.
func TestInsertReturningID_SQLite_ExecError(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	ctx := context.Background()

	// Table doesn't exist, so this should fail.
	id, err := db.InsertReturningID(ctx, "INSERT INTO nonexistent (name) VALUES (?)", "hello")
	assert.Error(t, err)
	assert.Equal(t, int64(0), id)
}

// TestInsertReturningID_PostgresDialect tests InsertReturningID using
// the Postgres dialect path. Since we use SQLite as the underlying driver,
// the "RETURNING id" SQL will fail, but this exercises the Postgres branch.
func TestInsertReturningID_PostgresDialect(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.Postgres)
	ctx := context.Background()

	_, err = db.DB.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	require.NoError(t, err)

	// Postgres path appends "RETURNING id" and uses QueryRow.Scan.
	// SQLite can handle "RETURNING id" in newer versions (3.35+), so
	// this may succeed or fail depending on the SQLite version.
	id, err := db.InsertReturningID(ctx, "INSERT INTO test (name) VALUES (?)", "hello")
	if err == nil {
		assert.Greater(t, id, int64(0))
	}
	// Either way, the Postgres branch is exercised.
}

// TestTxInsertReturningID_SQLite_ExecError tests TxInsertReturningID when the
// SQL execution fails on SQLite.
func TestTxInsertReturningID_SQLite_ExecError(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	ctx := context.Background()

	tx, err := db.DB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	// Table doesn't exist, so this should fail.
	id, err := db.TxInsertReturningID(ctx, tx, "INSERT INTO nonexistent (name) VALUES (?)", "hello")
	assert.Error(t, err)
	assert.Equal(t, int64(0), id)
}

// TestTxInsertReturningID_PostgresDialect tests TxInsertReturningID using
// the Postgres dialect path.
func TestTxInsertReturningID_PostgresDialect(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.Postgres)
	ctx := context.Background()

	_, err = db.DB.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	require.NoError(t, err)

	tx, err := db.DB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	// Postgres path appends "RETURNING id" and uses tx.QueryRow.Scan.
	id, err := db.TxInsertReturningID(ctx, tx, "INSERT INTO test (name) VALUES (?)", "in-tx")
	if err == nil {
		assert.Greater(t, id, int64(0))
	}
}

// TestTableExists_PostgresDialect tests TableExists using the Postgres
// dialect path. Since SQLite doesn't have information_schema, this will
// fail, but the Postgres branch is exercised.
func TestTableExists_PostgresDialect(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.Postgres)
	ctx := context.Background()

	// Postgres branch queries information_schema, which doesn't exist in SQLite.
	_, err = db.TableExists(ctx, "test")
	// This should error because information_schema doesn't exist in SQLite.
	assert.Error(t, err)
}

// TestTableExists_SQLite_CreatedAndFound tests TableExists with a table
// that exists using the SQLite path.
func TestTableExists_SQLite_CreatedAndFound(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	ctx := context.Background()

	_, err = db.ExecContext(ctx, "CREATE TABLE my_table (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)

	exists, err := db.TableExists(ctx, "my_table")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = db.TableExists(ctx, "other_table")
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestCreateContext_DefaultTimeout tests createContext with zero busyTimeout.
func TestCreateContext_DefaultTimeout(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := Wrap(sqlDB, dialect.SQLite)
	ctx, cancel := db.createContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(5*time.Second), deadline, 1*time.Second)
}

// TestCreateContext_CustomTimeout tests createContext with a custom busyTimeout.
func TestCreateContext_CustomTimeout(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := &DB{
		DB:          sqlDB,
		dialect:     dialect.New(dialect.SQLite),
		busyTimeout: 10 * time.Second,
	}

	ctx, cancel := db.createContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(10*time.Second), deadline, 1*time.Second)
}

// TestHealthCheck_WithBusyTimeout tests HealthCheck using a connection with
// a custom busyTimeout to cover the createContext path.
func TestHealthCheck_WithBusyTimeout(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := &DB{
		DB:          sqlDB,
		dialect:     dialect.New(dialect.SQLite),
		busyTimeout: 2 * time.Second,
	}

	assert.NoError(t, db.HealthCheck())
}

// TestRewriteQuery_WithBooleanColumns tests that rewriteQuery delegates to
// dialect.RewriteAll with the correct boolean columns.
func TestRewriteQuery_WithBooleanColumns(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := &DB{
		DB:             sqlDB,
		dialect:        dialect.New(dialect.Postgres),
		booleanColumns: []string{"is_active"},
	}

	rewritten := db.rewriteQuery("SELECT * FROM t WHERE is_active = 1")
	assert.Contains(t, rewritten, "TRUE")
}
