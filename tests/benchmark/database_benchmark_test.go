package benchmark

import (
	"context"
	"fmt"
	"testing"
	"time"

	"digital.vasic.database/pkg/pool"
	"digital.vasic.database/pkg/query"
	"digital.vasic.database/pkg/sqlite"
)

func BenchmarkSQLite_Insert(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	_, _ = client.Exec(ctx,
		"CREATE TABLE bench_insert (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT)")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Exec(ctx,
			"INSERT INTO bench_insert (value) VALUES (?)",
			fmt.Sprintf("bench-value-%d", i))
	}
}

func BenchmarkSQLite_Select(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	_, _ = client.Exec(ctx,
		"CREATE TABLE bench_select (id INTEGER PRIMARY KEY, value TEXT)")

	for i := 0; i < 1000; i++ {
		_, _ = client.Exec(ctx,
			"INSERT INTO bench_select (value) VALUES (?)",
			fmt.Sprintf("value-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := (i % 1000) + 1
		row := client.QueryRow(ctx,
			"SELECT value FROM bench_select WHERE id = ?", id)
		var val string
		_ = row.Scan(&val)
	}
}

func BenchmarkSQLite_Transaction(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	_, _ = client.Exec(ctx,
		"CREATE TABLE bench_tx (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT)")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, err := client.Begin(ctx)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = tx.Exec(ctx,
			"INSERT INTO bench_tx (value) VALUES (?)",
			fmt.Sprintf("tx-value-%d", i))
		_ = tx.Commit(ctx)
	}
}

func BenchmarkQueryBuilder_SimpleSelect(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = query.New().
			Select("id", "name", "email").
			From("users").
			Where(query.Eq("status", "active")).
			OrderBy("created_at DESC").
			Limit(10).
			Build()
	}
}

func BenchmarkQueryBuilder_ComplexQuery(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = query.New().
			Select("u.id", "u.name", "COUNT(o.id)").
			From("users u JOIN orders o ON u.id = o.user_id").
			Where(query.And(
				query.Eq("u.status", "active"),
				query.Gt("o.amount", 100.0),
				query.In("u.role", "admin", "manager", "user"),
			)).
			GroupBy("u.id, u.name").
			Having(query.Gt("COUNT(o.id)", 5)).
			OrderBy("COUNT(o.id) DESC").
			Limit(20).
			Offset(40).
			Build()
	}
}

func BenchmarkPool_AcquireRelease(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	cfg := &pool.PoolConfig{
		MaxSize:             50,
		MinSize:             5,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      5 * time.Second,
	}

	connID := 0
	p, err := pool.NewGenericPool(
		cfg,
		func(_ context.Context) (pool.Conn, error) {
			connID++
			return fmt.Sprintf("conn-%d", connID), nil
		},
		func(_ context.Context, _ pool.Conn) error { return nil },
		func(_ pool.Conn) error { return nil },
	)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, err := p.Acquire(ctx)
		if err != nil {
			b.Fatal(err)
		}
		p.Release(c)
	}
}

func BenchmarkSQLite_HealthCheck(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		b.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.HealthCheck(ctx)
	}
}

func BenchmarkQueryConditions_Build(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark test in short mode")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := query.And(
			query.Eq("a", 1),
			query.Neq("b", 2),
			query.Gt("c", 3),
			query.Lt("d", 4),
			query.Like("e", "%pattern%"),
			query.IsNotNull("f"),
			query.Or(
				query.In("g", 1, 2, 3),
				query.IsNull("h"),
			),
		)
		_, _ = c.Build()
	}
}
