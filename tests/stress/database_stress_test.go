package stress

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.database/pkg/pool"
	"digital.vasic.database/pkg/query"
	"digital.vasic.database/pkg/sqlite"
)

func TestStress_ConcurrentInserts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	client := sqlite.New(&sqlite.Config{
		Path:            ":memory:",
		JournalMode:     "WAL",
		BusyTimeout:     10 * time.Second,
		MaxOpenConns:    1, // SQLite requires serialized writes
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Hour,
	})
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exec(ctx,
		"CREATE TABLE stress_data (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT)")
	require.NoError(t, err)

	// SQLite serializes writes, so we use a mutex for concurrent goroutines
	const goroutines = 50
	const insertsPerGoroutine = 20
	var wg sync.WaitGroup
	var errCount atomic.Int64
	var mu sync.Mutex

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < insertsPerGoroutine; i++ {
				value := fmt.Sprintf("goroutine-%d-item-%d", id, i)
				mu.Lock()
				_, err := client.Exec(ctx,
					"INSERT INTO stress_data (value) VALUES (?)", value)
				mu.Unlock()
				if err != nil {
					errCount.Add(1)
				}
			}
		}(g)
	}

	wg.Wait()
	assert.Equal(t, int64(0), errCount.Load())

	// Verify total count
	row := client.QueryRow(ctx, "SELECT COUNT(*) FROM stress_data")
	var total int
	require.NoError(t, row.Scan(&total))
	assert.Equal(t, goroutines*insertsPerGoroutine, total)
}

func TestStress_ConcurrentReads(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exec(ctx,
		"CREATE TABLE read_test (id INTEGER PRIMARY KEY, value TEXT)")
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		_, err = client.Exec(ctx,
			"INSERT INTO read_test (value) VALUES (?)", fmt.Sprintf("value-%d", i))
		require.NoError(t, err)
	}

	const goroutines = 100
	var wg sync.WaitGroup
	var errCount atomic.Int64

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				targetID := (id*50+i)%100 + 1
				row := client.QueryRow(ctx,
					"SELECT value FROM read_test WHERE id = ?", targetID)
				var val string
				if err := row.Scan(&val); err != nil {
					errCount.Add(1)
				}
			}
		}(g)
	}

	wg.Wait()
	assert.Equal(t, int64(0), errCount.Load())
}

func TestStress_ConcurrentTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	client := sqlite.New(&sqlite.Config{
		Path:            ":memory:",
		JournalMode:     "WAL",
		BusyTimeout:     10 * time.Second,
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Hour,
	})
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.Exec(ctx,
		"CREATE TABLE counters (id INTEGER PRIMARY KEY, count INTEGER)")
	require.NoError(t, err)
	_, err = client.Exec(ctx,
		"INSERT INTO counters (id, count) VALUES (1, 0)")
	require.NoError(t, err)

	const goroutines = 50
	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				mu.Lock()
				tx, err := client.Begin(ctx)
				if err != nil {
					mu.Unlock()
					continue
				}
				_, err = tx.Exec(ctx,
					"UPDATE counters SET count = count + 1 WHERE id = 1")
				if err != nil {
					_ = tx.Rollback(ctx)
					mu.Unlock()
					continue
				}
				_ = tx.Commit(ctx)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	row := client.QueryRow(ctx, "SELECT count FROM counters WHERE id = 1")
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, goroutines*10, count,
		"all increments should have succeeded")
}

func TestStress_ConcurrentPoolAcquireRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	var connID atomic.Int64
	cfg := &pool.PoolConfig{
		MaxSize:             20,
		MinSize:             2,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0, // disable for stress test
		AcquireTimeout:      5 * time.Second,
	}

	p, err := pool.NewGenericPool(
		cfg,
		func(_ context.Context) (pool.Conn, error) {
			id := connID.Add(1)
			return fmt.Sprintf("conn-%d", id), nil
		},
		func(_ context.Context, _ pool.Conn) error { return nil },
		func(_ pool.Conn) error { return nil },
	)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	const goroutines = 100
	var wg sync.WaitGroup
	var errCount atomic.Int64

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				ctx := context.Background()
				c, err := p.Acquire(ctx)
				if err != nil {
					errCount.Add(1)
					continue
				}
				// Simulate work
				time.Sleep(time.Microsecond)
				p.Release(c)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(0), errCount.Load())

	stats := p.Stats()
	assert.True(t, stats.AcquireCount >= int64(goroutines*20))
	assert.Equal(t, int64(0), stats.AcquireErrors)
}

func TestStress_ConcurrentQueryBuilding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const goroutines = 100
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				sql, args := query.New().
					Select("id", "name", "email").
					From("users").
					Where(query.And(
						query.Eq("status", "active"),
						query.Gt("age", 18),
						query.Like("name", fmt.Sprintf("user-%d%%", id)),
					)).
					OrderBy("created_at DESC").
					Limit(10).
					Offset(i * 10).
					Build()

				if sql == "" {
					t.Errorf("query builder returned empty SQL")
				}
				if len(args) == 0 {
					t.Errorf("query builder returned no args")
				}
			}
		}(g)
	}

	wg.Wait()
}

func TestStress_ConcurrentHealthChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	client := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	const goroutines = 80
	var wg sync.WaitGroup
	var errCount atomic.Int64

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if err := client.HealthCheck(ctx); err != nil {
					errCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(0), errCount.Load())
}
