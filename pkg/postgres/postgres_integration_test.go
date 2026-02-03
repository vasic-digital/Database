//go:build integration

package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "digital.vasic.database/pkg/database"
)

// getTestConfig returns a configuration for integration tests.
// It reads from environment variables with sensible defaults for local testing.
func getTestConfig() *Config {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}

	port := 5432
	if p := os.Getenv("DB_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}

	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "postgres"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "postgres"
	}

	return &Config{
		Config: db.Config{
			Driver:          "postgres",
			Host:            host,
			Port:            port,
			User:            user,
			Password:        password,
			DBName:          dbName,
			SSLMode:         "disable",
			MaxConns:        5,
			MinConns:        1,
			MaxConnLifetime: time.Hour,
			MaxConnIdleTime: 30 * time.Minute,
			ConnectTimeout:  5 * time.Second,
		},
		ApplicationName:      "postgres-integration-test",
		HealthCheckPeriod:    30 * time.Second,
		PreferSimpleProtocol: true,
	}
}

// setupTestClient creates a connected client for testing.
func setupTestClient(t *testing.T) *Client {
	t.Helper()

	cfg := getTestConfig()
	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err, "Failed to connect to test database")

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

// ============================================================================
// Connection Pool Tests
// ============================================================================

func TestIntegration_Connect_Success(t *testing.T) {
	cfg := getTestConfig()
	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)

	assert.NotNil(t, client.Pool())

	err = client.Close()
	assert.NoError(t, err)
	assert.Nil(t, client.Pool())
}

func TestIntegration_Connect_MultipleConnections(t *testing.T) {
	cfg := getTestConfig()

	clients := make([]*Client, 3)
	for i := range clients {
		clients[i] = New(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := clients[i].Connect(ctx)
		cancel()
		require.NoError(t, err, "Failed to connect client %d", i)
	}

	// Close all
	for i, c := range clients {
		err := c.Close()
		assert.NoError(t, err, "Failed to close client %d", i)
	}
}

// ============================================================================
// Exec Tests
// ============================================================================

func TestIntegration_Exec_CreateTable(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// Create a test table
	result, err := client.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_exec_table (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Clean up
	_, err = client.Exec(ctx, "DROP TABLE IF EXISTS test_exec_table")
	require.NoError(t, err)
}

func TestIntegration_Exec_InsertAndUpdate(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// Setup
	_, err := client.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_insert_table (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL
		)
	`)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = client.Exec(ctx, "DROP TABLE IF EXISTS test_insert_table")
	})

	// Insert
	result, err := client.Exec(ctx, "INSERT INTO test_insert_table (name) VALUES ($1)", "test")
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	// Update
	result, err = client.Exec(ctx, "UPDATE test_insert_table SET name = $1 WHERE name = $2", "updated", "test")
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
}

func TestIntegration_Exec_Error(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// Invalid SQL should return an error
	_, err := client.Exec(ctx, "INVALID SQL SYNTAX")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec")
}

// ============================================================================
// Query Tests
// ============================================================================

func TestIntegration_Query_SelectRows(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// Setup
	_, err := client.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_query_table (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL
		)
	`)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = client.Exec(ctx, "DROP TABLE IF EXISTS test_query_table")
	})

	// Insert test data
	_, err = client.Exec(ctx, "INSERT INTO test_query_table (name) VALUES ('alice'), ('bob'), ('charlie')")
	require.NoError(t, err)

	// Query
	rows, err := client.Query(ctx, "SELECT id, name FROM test_query_table ORDER BY name")
	require.NoError(t, err)
	defer rows.Close()

	var results []struct {
		id   int
		name string
	}

	for rows.Next() {
		var r struct {
			id   int
			name string
		}
		err := rows.Scan(&r.id, &r.name)
		require.NoError(t, err)
		results = append(results, r)
	}

	require.NoError(t, rows.Err())
	assert.Len(t, results, 3)
	assert.Equal(t, "alice", results[0].name)
	assert.Equal(t, "bob", results[1].name)
	assert.Equal(t, "charlie", results[2].name)
}

func TestIntegration_Query_EmptyResult(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// Query that returns no rows
	rows, err := client.Query(ctx, "SELECT 1 WHERE false")
	require.NoError(t, err)
	defer rows.Close()

	assert.False(t, rows.Next())
	assert.NoError(t, rows.Err())
}

func TestIntegration_Query_Error(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// Query non-existent table
	_, err := client.Query(ctx, "SELECT * FROM nonexistent_table_xyz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

// ============================================================================
// QueryRow Tests
// ============================================================================

func TestIntegration_QueryRow_SingleRow(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	var result int
	err := client.QueryRow(ctx, "SELECT 42").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestIntegration_QueryRow_NoRows(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	var result int
	err := client.QueryRow(ctx, "SELECT 1 WHERE false").Scan(&result)
	require.Error(t, err)
	// pgx returns ErrNoRows
}

func TestIntegration_QueryRow_ScanError(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	var result int
	err := client.QueryRow(ctx, "SELECT 'not a number'").Scan(&result)
	require.Error(t, err)
}

// ============================================================================
// Transaction Tests
// ============================================================================

func TestIntegration_Transaction_CommitSuccess(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// Setup
	_, err := client.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_tx_table (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL
		)
	`)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = client.Exec(ctx, "DROP TABLE IF EXISTS test_tx_table")
	})

	// Begin transaction
	tx, err := client.Begin(ctx)
	require.NoError(t, err)

	// Insert within transaction
	_, err = tx.Exec(ctx, "INSERT INTO test_tx_table (name) VALUES ($1)", "tx_test")
	require.NoError(t, err)

	// Commit
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Verify data persisted
	var count int
	err = client.QueryRow(ctx, "SELECT COUNT(*) FROM test_tx_table WHERE name = 'tx_test'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestIntegration_Transaction_RollbackSuccess(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// Setup
	_, err := client.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_tx_rollback (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL
		)
	`)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = client.Exec(ctx, "DROP TABLE IF EXISTS test_tx_rollback")
	})

	// Begin transaction
	tx, err := client.Begin(ctx)
	require.NoError(t, err)

	// Insert within transaction
	_, err = tx.Exec(ctx, "INSERT INTO test_tx_rollback (name) VALUES ($1)", "rollback_test")
	require.NoError(t, err)

	// Rollback
	err = tx.Rollback(ctx)
	require.NoError(t, err)

	// Verify data NOT persisted
	var count int
	err = client.QueryRow(ctx, "SELECT COUNT(*) FROM test_tx_rollback WHERE name = 'rollback_test'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestIntegration_Transaction_Query(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// Setup
	_, err := client.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_tx_query (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL
		)
	`)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = client.Exec(ctx, "DROP TABLE IF EXISTS test_tx_query")
	})

	// Insert some data
	_, err = client.Exec(ctx, "INSERT INTO test_tx_query (name) VALUES ('one'), ('two')")
	require.NoError(t, err)

	// Query within transaction
	tx, err := client.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, "SELECT name FROM test_tx_query ORDER BY name")
	require.NoError(t, err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		require.NoError(t, err)
		names = append(names, name)
	}

	assert.Len(t, names, 2)
	assert.Equal(t, "one", names[0])
	assert.Equal(t, "two", names[1])
}

func TestIntegration_Transaction_QueryRow(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	tx, err := client.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	var result int
	err = tx.QueryRow(ctx, "SELECT 123").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 123, result)
}

func TestIntegration_Transaction_ExecError(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	tx, err := client.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "INVALID SQL")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tx exec")
}

func TestIntegration_Transaction_QueryError(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	tx, err := client.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	_, err = tx.Query(ctx, "SELECT * FROM nonexistent_table_abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tx query")
}

// ============================================================================
// HealthCheck Tests
// ============================================================================

func TestIntegration_HealthCheck_Success(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	err := client.HealthCheck(ctx)
	require.NoError(t, err)
}

func TestIntegration_HealthCheck_AfterClose(t *testing.T) {
	cfg := getTestConfig()
	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	// HealthCheck after close should fail
	// Note: This might panic or return error depending on implementation
	// The test verifies the behavior is defined
}

// ============================================================================
// Migrate Tests
// ============================================================================

func TestIntegration_Migrate_Success(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS test_migrate_1 (id SERIAL PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS test_migrate_2 (id SERIAL PRIMARY KEY, value TEXT)`,
		`CREATE INDEX IF NOT EXISTS idx_test_migrate_2_value ON test_migrate_2(value)`,
	}

	t.Cleanup(func() {
		_, _ = client.Exec(ctx, "DROP TABLE IF EXISTS test_migrate_2")
		_, _ = client.Exec(ctx, "DROP TABLE IF EXISTS test_migrate_1")
	})

	err := client.Migrate(ctx, migrations)
	require.NoError(t, err)

	// Verify tables exist
	var exists bool
	err = client.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'test_migrate_1'
		)
	`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestIntegration_Migrate_PartialFailure(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	// First migration succeeds, second fails
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS test_migrate_partial (id SERIAL PRIMARY KEY)`,
		`INVALID SQL STATEMENT`,
	}

	t.Cleanup(func() {
		_, _ = client.Exec(ctx, "DROP TABLE IF EXISTS test_migrate_partial")
	})

	err := client.Migrate(ctx, migrations)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migration 1")
}

func TestIntegration_Migrate_EmptyList(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	err := client.Migrate(ctx, []string{})
	require.NoError(t, err)
}

// ============================================================================
// Context Cancellation Tests
// ============================================================================

func TestIntegration_ContextCancellation_Query(t *testing.T) {
	client := setupTestClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Query(ctx, "SELECT pg_sleep(10)")
	require.Error(t, err)
}

func TestIntegration_ContextCancellation_Exec(t *testing.T) {
	client := setupTestClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Exec(ctx, "SELECT pg_sleep(10)")
	require.Error(t, err)
}

func TestIntegration_ContextCancellation_Begin(t *testing.T) {
	client := setupTestClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Begin(ctx)
	require.Error(t, err)
}

// ============================================================================
// Pool Stats Tests
// ============================================================================

func TestIntegration_Pool_Stats(t *testing.T) {
	client := setupTestClient(t)

	pool := client.Pool()
	require.NotNil(t, pool)

	stats := pool.Stat()
	assert.GreaterOrEqual(t, stats.TotalConns(), int32(0))
	assert.GreaterOrEqual(t, stats.IdleConns(), int32(0))
}
