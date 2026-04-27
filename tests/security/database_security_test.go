package security

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.database/pkg/database"
	"digital.vasic.database/pkg/pool"
	"digital.vasic.database/pkg/query"
	"digital.vasic.database/pkg/sqlite"
)

func TestSecurity_SQLInjection_Parameterized(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exec(ctx,
		"CREATE TABLE secrets (id INTEGER PRIMARY KEY, value TEXT)")
	require.NoError(t, err)

	_, err = client.Exec(ctx,
		"INSERT INTO secrets (value) VALUES (?)", "top-secret-data")
	require.NoError(t, err)

	// SQL injection attempt via parameterized query should be safe
	maliciousInputs := []string{
		"'; DROP TABLE secrets; --",
		"1 OR 1=1",
		"' UNION SELECT * FROM secrets --",
		"1; DELETE FROM secrets",
		"Robert'); DROP TABLE secrets;--",
	}

	for _, input := range maliciousInputs {
		row := client.QueryRow(ctx,
			"SELECT value FROM secrets WHERE id = ?", input)
		var value string
		err = row.Scan(&value)
		// Should fail to find anything, not execute malicious SQL
		assert.Error(t, err, "malicious input %q should not return results", input)
	}

	// Verify table still exists with original data
	row := client.QueryRow(ctx, "SELECT value FROM secrets WHERE id = 1")
	var val string
	require.NoError(t, row.Scan(&val))
	assert.Equal(t, "top-secret-data", val)
}

func TestSecurity_QueryBuilder_InjectionResistance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exec(ctx,
		"CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)

	_, err = client.Exec(ctx,
		"INSERT INTO items (name) VALUES (?)", "safe-item")
	require.NoError(t, err)

	// Query builder with malicious input in condition values
	maliciousValues := []string{
		"'; DROP TABLE items; --",
		"1 OR 1=1",
		"' UNION SELECT * FROM sqlite_master --",
	}

	for _, val := range maliciousValues {
		sql, args := query.New().
			Select("*").
			From("items").
			Where(query.Eq("name", val)).
			Build()

		rows, err := client.Query(ctx, sql, args...)
		if err != nil {
			continue // Query failed safely
		}
		count := 0
		for rows.Next() {
			count++
		}
		_ = rows.Close()
		assert.Equal(t, 0, count, "malicious value %q should return no results", val)
	}

	// Original data should still be intact
	row := client.QueryRow(ctx, "SELECT name FROM items WHERE id = 1")
	var name string
	require.NoError(t, row.Scan(&name))
	assert.Equal(t, "safe-item", name)
}

func TestSecurity_NilClient_Operations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	// Health check before connect
	err := client.HealthCheck(ctx)
	assert.Error(t, err, "health check before connect should fail")

	// Close without connect should not panic
	err = client.Close()
	assert.NoError(t, err)
}

func TestSecurity_ConfigValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	invalidConfigs := []struct {
		name string
		cfg  *database.Config
	}{
		{"empty", &database.Config{}},
		{"no driver", &database.Config{Host: "localhost"}},
		{"postgres no host", &database.Config{Driver: "postgres"}},
		{"postgres no user", &database.Config{Driver: "postgres", Host: "localhost"}},
		{"postgres no db", &database.Config{Driver: "postgres", Host: "localhost", User: "user"}},
		{"sqlite no path", &database.Config{Driver: "sqlite"}},
		{"unsupported driver", &database.Config{Driver: "oracle"}},
	}

	for _, tc := range invalidConfigs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			assert.Error(t, err, "config should be invalid: %s", tc.name)
		})
	}
}

func TestSecurity_PoolNilCallbacks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	cfg := pool.DefaultPoolConfig()

	// Nil factory
	_, err := pool.NewGenericPool(cfg, nil, func(_ context.Context, _ pool.Conn) error { return nil }, func(_ pool.Conn) error { return nil })
	assert.Error(t, err, "nil factory should be rejected")

	// Nil checker
	_, err = pool.NewGenericPool(cfg, func(_ context.Context) (pool.Conn, error) { return "c", nil }, nil, func(_ pool.Conn) error { return nil })
	assert.Error(t, err, "nil checker should be rejected")

	// Nil closer
	_, err = pool.NewGenericPool(cfg, func(_ context.Context) (pool.Conn, error) { return "c", nil }, func(_ context.Context, _ pool.Conn) error { return nil }, nil)
	assert.Error(t, err, "nil closer should be rejected")
}

func TestSecurity_PoolConfigValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	tests := []struct {
		name string
		cfg  pool.PoolConfig
	}{
		{"zero max", pool.PoolConfig{MaxSize: 0, MinSize: 0, MaxLifetime: time.Hour, MaxIdleTime: time.Minute}},
		{"negative max", pool.PoolConfig{MaxSize: -5, MinSize: 0, MaxLifetime: time.Hour, MaxIdleTime: time.Minute}},
		{"min > max", pool.PoolConfig{MaxSize: 5, MinSize: 10, MaxLifetime: time.Hour, MaxIdleTime: time.Minute}},
		{"negative min", pool.PoolConfig{MaxSize: 10, MinSize: -1, MaxLifetime: time.Hour, MaxIdleTime: time.Minute}},
		{"zero lifetime", pool.PoolConfig{MaxSize: 10, MinSize: 1, MaxLifetime: 0, MaxIdleTime: time.Minute}},
		{"zero idle time", pool.PoolConfig{MaxSize: 10, MinSize: 1, MaxLifetime: time.Hour, MaxIdleTime: 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			assert.Error(t, err, "pool config should be invalid: %s", tc.name)
		})
	}
}

func TestSecurity_LargeStringValues(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exec(ctx, "CREATE TABLE docs (id INTEGER PRIMARY KEY, content TEXT)")
	require.NoError(t, err)

	// Insert very large string
	largeContent := strings.Repeat("A", 1024*1024) // 1MB
	_, err = client.Exec(ctx, "INSERT INTO docs (content) VALUES (?)", largeContent)
	require.NoError(t, err)

	row := client.QueryRow(ctx, "SELECT content FROM docs WHERE id = 1")
	var retrieved string
	require.NoError(t, row.Scan(&retrieved))
	assert.Equal(t, len(largeContent), len(retrieved))
}

func TestSecurity_QueryBuilder_EmptyIn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security test in short mode")  // SKIP-OK: #short-mode
	}

	// Empty IN clause should produce a safe always-false condition
	sql, args := query.New().
		Select("*").
		From("users").
		Where(query.In("id")).
		Build()

	assert.Contains(t, sql, "1 = 0")
	assert.Len(t, args, 0)
}
