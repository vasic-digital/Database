package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.database/pkg/database"
	"digital.vasic.database/pkg/migration"
	"digital.vasic.database/pkg/pool"
	"digital.vasic.database/pkg/query"
	"digital.vasic.database/pkg/sqlite"
)

func TestSQLite_ConnectAndQuery_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Create table
	_, err = client.Exec(ctx,
		"CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT)")
	require.NoError(t, err)

	// Insert
	res, err := client.Exec(ctx,
		"INSERT INTO users (name, email) VALUES (?, ?)", "Alice", "alice@example.com")
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	// Query single row
	row := client.QueryRow(ctx, "SELECT name, email FROM users WHERE id = ?", 1)
	var name, email string
	err = row.Scan(&name, &email)
	require.NoError(t, err)
	assert.Equal(t, "Alice", name)
	assert.Equal(t, "alice@example.com", email)

	// Insert more rows
	for i := 2; i <= 5; i++ {
		_, err = client.Exec(ctx,
			"INSERT INTO users (name, email) VALUES (?, ?)",
			"User"+string(rune('0'+i)), "user@example.com")
		require.NoError(t, err)
	}

	// Query multiple rows
	rows, err := client.Query(ctx, "SELECT id, name FROM users ORDER BY id")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		var id int
		var n string
		err = rows.Scan(&id, &n)
		require.NoError(t, err)
		count++
	}
	assert.NoError(t, rows.Err())
	assert.Equal(t, 5, count)
}

func TestSQLite_Transactions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exec(ctx,
		"CREATE TABLE accounts (id INTEGER PRIMARY KEY, balance REAL)")
	require.NoError(t, err)

	_, err = client.Exec(ctx,
		"INSERT INTO accounts (id, balance) VALUES (1, 100.0), (2, 200.0)")
	require.NoError(t, err)

	// Successful transaction: transfer funds
	tx, err := client.Begin(ctx)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, "UPDATE accounts SET balance = balance - 50 WHERE id = 1")
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "UPDATE accounts SET balance = balance + 50 WHERE id = 2")
	require.NoError(t, err)

	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Verify balances
	row := client.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = 1")
	var bal1 float64
	require.NoError(t, row.Scan(&bal1))
	assert.Equal(t, 50.0, bal1)

	row = client.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = 2")
	var bal2 float64
	require.NoError(t, row.Scan(&bal2))
	assert.Equal(t, 250.0, bal2)

	// Rollback transaction
	tx, err = client.Begin(ctx)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, "UPDATE accounts SET balance = 0 WHERE id = 1")
	require.NoError(t, err)

	err = tx.Rollback(ctx)
	require.NoError(t, err)

	// Balance should be unchanged
	row = client.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = 1")
	require.NoError(t, row.Scan(&bal1))
	assert.Equal(t, 50.0, bal1)
}

func TestMigration_ApplyAndRollback_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	runner := migration.NewRunner(client, "test_migrations")

	migrations := []migration.Migration{
		{
			Version: 1,
			Name:    "create_users",
			Up:      "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)",
			Down:    "DROP TABLE users",
		},
		{
			Version: 2,
			Name:    "add_email",
			Up:      "ALTER TABLE users ADD COLUMN email TEXT",
			Down:    "", // SQLite does not support DROP COLUMN easily
		},
	}

	// Apply migrations
	err = runner.Apply(ctx, migrations)
	require.NoError(t, err)

	// Verify migration tracking
	applied, err := runner.Applied(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, applied)

	// Verify schema
	_, err = client.Exec(ctx, "INSERT INTO users (name, email) VALUES ('Test', 'test@test.com')")
	assert.NoError(t, err, "should be able to use email column after migration")

	// Re-apply should be idempotent
	err = runner.Apply(ctx, migrations)
	assert.NoError(t, err)
}

func TestQueryBuilder_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exec(ctx, `
		CREATE TABLE products (
			id INTEGER PRIMARY KEY,
			name TEXT,
			price REAL,
			category TEXT
		)`)
	require.NoError(t, err)

	// Insert test data
	products := []struct {
		name, cat string
		price     float64
	}{
		{"Widget", "A", 9.99},
		{"Gadget", "B", 19.99},
		{"Thingamajig", "A", 29.99},
		{"Doohickey", "C", 4.99},
		{"Gizmo", "B", 14.99},
	}
	for _, p := range products {
		_, err = client.Exec(ctx,
			"INSERT INTO products (name, price, category) VALUES (?, ?, ?)",
			p.name, p.price, p.cat)
		require.NoError(t, err)
	}

	// Build and execute query
	sql, args := query.New().
		Select("name", "price").
		From("products").
		Where(query.Gt("price", 10.0)).
		OrderBy("price ASC").
		Limit(3).
		Build()

	rows, err := client.Query(ctx, sql, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var results []string
	for rows.Next() {
		var name string
		var price float64
		require.NoError(t, rows.Scan(&name, &price))
		results = append(results, name)
	}
	assert.NoError(t, rows.Err())
	assert.Equal(t, 3, len(results))
}

func TestPoolConfig_Validation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Valid config
	cfg := pool.DefaultPoolConfig()
	assert.NoError(t, cfg.Validate())

	// Invalid: negative max size
	badCfg := &pool.PoolConfig{MaxSize: -1, MinSize: 0, MaxLifetime: time.Hour, MaxIdleTime: time.Minute}
	assert.Error(t, badCfg.Validate())

	// Invalid: min > max
	badCfg2 := &pool.PoolConfig{MaxSize: 5, MinSize: 10, MaxLifetime: time.Hour, MaxIdleTime: time.Minute}
	assert.Error(t, badCfg2.Validate())
}

func TestDatabaseConfig_Validation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Valid postgres config
	cfg := &database.Config{
		Driver: "postgres",
		Host:   "localhost",
		Port:   5432,
		User:   "app",
		DBName: "testdb",
	}
	assert.NoError(t, cfg.Validate())

	// Valid sqlite config
	sqliteCfg := &database.Config{
		Driver: "sqlite",
		DBName: ":memory:",
	}
	assert.NoError(t, sqliteCfg.Validate())

	// Invalid: no driver
	assert.Error(t, (&database.Config{}).Validate())

	// Invalid: postgres without host
	assert.Error(t, (&database.Config{Driver: "postgres"}).Validate())

	// Invalid: unsupported driver
	assert.Error(t, (&database.Config{Driver: "mysql"}).Validate())
}
