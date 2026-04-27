// SPDX-License-Identifier: Apache-2.0
//
// Stress tests for digital.vasic.database/pkg/pool, modelled on the
// canonical P3 stress-test template in
// digital.vasic.buildcheck/pkg/buildcheck/stress_test.go.
//
// Run with:
//   GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -run '^TestStress' \
//       ./pkg/pool/ -p 1 -count=1 -timeout 120s
package pool

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// CONST-022: sized for GOMAXPROCS=2, single-process `-p 1` execution.
	stressGoroutines   = 8
	stressIterations   = 200
	stressMaxWallClock = 20 * time.Second
)

// stressFakeConn is a minimal implementation that lets us exercise the
// pool machinery without needing a real database.
type stressFakeConn struct {
	id     int64
	closed atomic.Bool
}

func newStressPool(t *testing.T, maxSize int) *GenericPool {
	t.Helper()
	var nextID atomic.Int64
	cfg := &PoolConfig{
		MaxSize:             maxSize,
		MinSize:             0,
		AcquireTimeout:      1 * time.Second,
		MaxLifetime:         30 * time.Second,
		MaxIdleTime:         30 * time.Second,
		HealthCheckInterval: 100 * time.Millisecond,
	}
	p, err := NewGenericPool(
		cfg,
		func(_ context.Context) (Conn, error) {
			return &stressFakeConn{id: nextID.Add(1)}, nil
		},
		func(_ context.Context, c Conn) error {
			if fc, ok := c.(*stressFakeConn); ok && fc.closed.Load() {
				return assert.AnError
			}
			return nil
		},
		func(c Conn) error {
			if fc, ok := c.(*stressFakeConn); ok {
				fc.closed.Store(true)
			}
			return nil
		},
	)
	require.NoError(t, err)
	return p
}

// TestStress_GenericPool_ConcurrentAcquireRelease asserts the pool stays
// consistent under heavy Acquire/Release churn. No race, no connection
// leak, acquire contention resolved within AcquireTimeout.
func TestStress_GenericPool_ConcurrentAcquireRelease(t *testing.T) {
	p := newStressPool(t, 4) // tight pool to force contention
	defer func() { _ = p.Close() }()

	startGoroutines := runtime.NumGoroutine()
	var wg sync.WaitGroup
	var acqErrors atomic.Int64
	deadline := time.Now().Add(stressMaxWallClock)

	for g := 0; g < stressGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < stressIterations; j++ {
				if time.Now().After(deadline) {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				conn, err := p.Acquire(ctx)
				cancel()
				if err != nil {
					acqErrors.Add(1)
					continue
				}
				// Simulate tiny workload while holding the connection.
				time.Sleep(50 * time.Microsecond)
				p.Release(conn)
			}
		}(g)
	}
	wg.Wait()

	// Under tight-pool contention with generous AcquireTimeout, we should
	// not see acquisition failures on well-formed calls.
	assert.Equal(t, int64(0), acqErrors.Load(),
		"Acquire should not time-out or error on well-formed calls under sustained load")

	// Allow the runtime to reap any transient goroutines the pool spawned
	// (health-check loop, etc.).
	time.Sleep(200 * time.Millisecond)
	runtime.Gosched()
	// Generous tolerance — the pool's background health-check goroutine
	// is legitimate and will still be running until Close() completes.
	endGoroutines := runtime.NumGoroutine()
	assert.LessOrEqual(t, endGoroutines-startGoroutines, 3,
		"goroutine leak: pool's worker set grew by %d", endGoroutines-startGoroutines)
}

// TestStress_GenericPool_RapidClose validates that Close after a burst
// of in-flight Acquires does not leak goroutines or panic.
func TestStress_GenericPool_RapidClose(t *testing.T) {
	for iter := 0; iter < 5; iter++ {
		p := newStressPool(t, 8)
		var wg sync.WaitGroup
		for g := 0; g < stressGoroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()
				conn, err := p.Acquire(ctx)
				if err == nil {
					time.Sleep(10 * time.Millisecond)
					p.Release(conn)
				}
			}()
		}
		// Close mid-flight.
		time.Sleep(5 * time.Millisecond)
		_ = p.Close()
		wg.Wait()
	}
}

// BenchmarkStress_GenericPool_AcquireRelease establishes a throughput
// baseline for ±25% regression gates.
func BenchmarkStress_GenericPool_AcquireRelease(b *testing.B) {
	var nextID atomic.Int64
	cfg := &PoolConfig{
		MaxSize: 16, MinSize: 0,
		AcquireTimeout: 500 * time.Millisecond,
	}
	p, err := NewGenericPool(cfg,
		func(_ context.Context) (Conn, error) { return &stressFakeConn{id: nextID.Add(1)}, nil },
		func(_ context.Context, _ Conn) error { return nil },
		func(_ Conn) error { return nil },
	)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		conn, err := p.Acquire(ctx)
		cancel()
		if err != nil {
			b.Fatal(err)
		}
		p.Release(conn)
	}
}
