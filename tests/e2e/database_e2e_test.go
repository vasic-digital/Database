package e2e

import (
	"context"
	"fmt"
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

func TestEndToEnd_FullDatabaseWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Phase 1: Run migrations
	runner := migration.NewRunner(client, "migrations")
	migrations := []migration.Migration{
		{
			Version: 1,
			Name:    "create_tasks",
			Up: `CREATE TABLE tasks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				title TEXT NOT NULL,
				status TEXT DEFAULT 'pending',
				priority INTEGER DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
			Down: "DROP TABLE tasks",
		},
	}
	err = runner.Apply(ctx, migrations)
	require.NoError(t, err)

	// Phase 2: Insert data
	for i := 1; i <= 20; i++ {
		status := "pending"
		if i%3 == 0 {
			status = "done"
		}
		_, err = client.Exec(ctx,
			"INSERT INTO tasks (title, status, priority) VALUES (?, ?, ?)",
			fmt.Sprintf("Task %d", i), status, i%5)
		require.NoError(t, err)
	}

	// Phase 3: Query with builder
	sql, args := query.New().
		Select("id", "title", "status").
		From("tasks").
		Where(query.Eq("status", "pending")).
		OrderBy("priority DESC").
		Limit(5).
		Build()

	rows, err := client.Query(ctx, sql, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		var id int
		var title, status string
		require.NoError(t, rows.Scan(&id, &title, &status))
		assert.Equal(t, "pending", status)
		count++
	}
	assert.NoError(t, rows.Err())
	assert.True(t, count > 0 && count <= 5)

	// Phase 4: Transaction -- bulk update
	tx, err := client.Begin(ctx)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, "UPDATE tasks SET status = 'in_progress' WHERE status = 'pending' AND priority > 2")
	require.NoError(t, err)
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Phase 5: Verify update
	row := client.QueryRow(ctx, "SELECT COUNT(*) FROM tasks WHERE status = 'in_progress'")
	var inProgress int
	require.NoError(t, row.Scan(&inProgress))
	assert.True(t, inProgress > 0)

	// Phase 6: Health check
	err = client.HealthCheck(ctx)
	assert.NoError(t, err)

	// Phase 7: Rollback migration
	err = runner.RollbackWith(ctx, 1, migrations)
	require.NoError(t, err)

	// Table should be gone
	_, err = client.Query(ctx, "SELECT * FROM tasks")
	assert.Error(t, err, "table should not exist after rollback")
}

func TestEndToEnd_QueryBuilder_ComplexQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exec(ctx, `
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY,
			customer TEXT,
			amount REAL,
			status TEXT,
			created_at TEXT
		)`)
	require.NoError(t, err)

	for i := 1; i <= 50; i++ {
		customer := fmt.Sprintf("customer-%d", i%5)
		amount := float64(i) * 10.50
		status := "completed"
		if i%4 == 0 {
			status = "pending"
		}
		_, err = client.Exec(ctx,
			"INSERT INTO orders (customer, amount, status, created_at) VALUES (?, ?, ?, ?)",
			customer, amount, status, "2024-01-01")
		require.NoError(t, err)
	}

	// Complex query: OR conditions
	sql, args := query.New().
		Select("*").
		From("orders").
		Where(query.Or(
			query.Eq("status", "pending"),
			query.Gt("amount", 400.0),
		)).
		OrderBy("amount DESC").
		Limit(10).
		Build()

	rows, err := client.Query(ctx, sql, args...)
	require.NoError(t, err)
	count := 0
	for rows.Next() {
		count++
		var id int
		var customer, status, created string
		var amount float64
		_ = rows.Scan(&id, &customer, &amount, &status, &created)
	}
	_ = rows.Close()
	assert.True(t, count > 0)

	// IN query
	sql, args = query.New().
		Select("customer", "amount").
		From("orders").
		Where(query.In("customer", "customer-0", "customer-1")).
		Build()

	rows, err = client.Query(ctx, sql, args...)
	require.NoError(t, err)
	count = 0
	for rows.Next() {
		count++
		var customer string
		var amount float64
		_ = rows.Scan(&customer, &amount)
	}
	_ = rows.Close()
	assert.True(t, count > 0)

	// NULL check
	sql, args = query.New().
		Select("COUNT(*)").
		From("orders").
		Where(query.IsNotNull("amount")).
		Build()

	row := client.QueryRow(ctx, sql, args...)
	var total int
	require.NoError(t, row.Scan(&total))
	assert.Equal(t, 50, total)
}

func TestEndToEnd_ConnectionPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	cfg := &pool.PoolConfig{
		MaxSize:             10,
		MinSize:             1,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: time.Minute,
		AcquireTimeout:      5 * time.Second,
	}
	require.NoError(t, cfg.Validate())

	connCount := 0
	p, err := pool.NewGenericPool(
		cfg,
		func(_ context.Context) (pool.Conn, error) {
			connCount++
			return fmt.Sprintf("conn-%d", connCount), nil
		},
		func(_ context.Context, _ pool.Conn) error { return nil },
		func(_ pool.Conn) error { return nil },
	)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Acquire and release connections
	var conns []pool.Conn
	for i := 0; i < 5; i++ {
		c, err := p.Acquire(ctx)
		require.NoError(t, err)
		conns = append(conns, c)
	}

	stats := p.Stats()
	assert.Equal(t, int64(5), stats.AcquireCount)

	// Release all
	for _, c := range conns {
		p.Release(c)
	}

	// Re-acquire should reuse connections
	c, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(c)
}

func TestEndToEnd_ConfigDSN(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	cfg := &database.Config{
		Driver:   "postgres",
		Host:     "db.example.com",
		Port:     5433,
		User:     "myuser",
		Password: "mypass",
		DBName:   "mydb",
		SSLMode:  "require",
	}

	dsn := cfg.DSN()
	assert.Contains(t, dsn, "postgres://myuser:mypass@db.example.com:5433/mydb")
	assert.Contains(t, dsn, "sslmode=require")

	// Default port and SSL mode
	cfg2 := &database.Config{
		Driver: "postgres",
		Host:   "localhost",
		User:   "user",
		DBName: "db",
	}
	dsn2 := cfg2.DSN()
	assert.Contains(t, dsn2, ":5432/")
	assert.Contains(t, dsn2, "sslmode=disable")
}

func TestEndToEnd_PoolStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	stats := pool.PoolStats{
		AcquireCount:       100,
		TotalAcquireTimeUs: 50000,
	}

	avg := stats.AverageAcquireTime()
	assert.Equal(t, 500*time.Microsecond, avg)

	// Zero acquire count
	emptyStats := pool.PoolStats{}
	assert.Equal(t, time.Duration(0), emptyStats.AverageAcquireTime())
}
