package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testConn is a simple test connection.
type testConn struct {
	id     int
	closed bool
	mu     sync.Mutex
}

func (tc *testConn) IsClosed() bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.closed
}

func (tc *testConn) SetClosed(closed bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.closed = closed
}

func newFactory() (func() int, ConnFactory) {
	var mu sync.Mutex
	counter := 0
	return func() int {
			mu.Lock()
			defer mu.Unlock()
			return counter
		}, func(ctx context.Context) (Conn, error) {
			mu.Lock()
			counter++
			id := counter
			mu.Unlock()
			return &testConn{id: id}, nil
		}
}

func newFailingFactory(errMsg string) ConnFactory {
	return func(ctx context.Context) (Conn, error) {
		return nil, errors.New(errMsg)
	}
}

func newDelayedFactory(delay time.Duration) ConnFactory {
	var mu sync.Mutex
	counter := 0
	return func(ctx context.Context) (Conn, error) {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		mu.Lock()
		counter++
		id := counter
		mu.Unlock()
		return &testConn{id: id}, nil
	}
}

func okChecker(_ context.Context, _ Conn) error { return nil }

func failChecker(_ context.Context, _ Conn) error {
	return errors.New("unhealthy")
}

func testCloser(conn Conn) error {
	if tc, ok := conn.(*testConn); ok {
		tc.SetClosed(true)
	}
	return nil
}

func failingCloser(_ Conn) error {
	return errors.New("close error")
}

func TestPoolConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  PoolConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: PoolConfig{
				MaxSize:     10,
				MinSize:     2,
				MaxLifetime: time.Hour,
				MaxIdleTime: 30 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "zero max size",
			config: PoolConfig{
				MaxSize:     0,
				MaxLifetime: time.Hour,
				MaxIdleTime: time.Minute,
			},
			wantErr: true,
			errMsg:  "max size must be positive",
		},
		{
			name: "negative max size",
			config: PoolConfig{
				MaxSize:     -5,
				MaxLifetime: time.Hour,
				MaxIdleTime: time.Minute,
			},
			wantErr: true,
			errMsg:  "max size must be positive",
		},
		{
			name: "negative min size",
			config: PoolConfig{
				MaxSize:     10,
				MinSize:     -1,
				MaxLifetime: time.Hour,
				MaxIdleTime: time.Minute,
			},
			wantErr: true,
			errMsg:  "min size must be non-negative",
		},
		{
			name: "min size exceeds max size",
			config: PoolConfig{
				MaxSize:     5,
				MinSize:     10,
				MaxLifetime: time.Hour,
				MaxIdleTime: time.Minute,
			},
			wantErr: true,
			errMsg:  "min size",
		},
		{
			name: "zero max lifetime",
			config: PoolConfig{
				MaxSize:     10,
				MaxLifetime: 0,
				MaxIdleTime: time.Minute,
			},
			wantErr: true,
			errMsg:  "max lifetime must be positive",
		},
		{
			name: "negative max lifetime",
			config: PoolConfig{
				MaxSize:     10,
				MaxLifetime: -time.Hour,
				MaxIdleTime: time.Minute,
			},
			wantErr: true,
			errMsg:  "max lifetime must be positive",
		},
		{
			name: "zero max idle time",
			config: PoolConfig{
				MaxSize:     10,
				MaxLifetime: time.Hour,
				MaxIdleTime: 0,
			},
			wantErr: true,
			errMsg:  "max idle time must be positive",
		},
		{
			name: "negative max idle time",
			config: PoolConfig{
				MaxSize:     10,
				MaxLifetime: time.Hour,
				MaxIdleTime: -time.Minute,
			},
			wantErr: true,
			errMsg:  "max idle time must be positive",
		},
		{
			name: "min size equals max size",
			config: PoolConfig{
				MaxSize:     5,
				MinSize:     5,
				MaxLifetime: time.Hour,
				MaxIdleTime: time.Minute,
			},
			wantErr: false,
		},
		{
			name: "zero min size is valid",
			config: PoolConfig{
				MaxSize:     10,
				MinSize:     0,
				MaxLifetime: time.Hour,
				MaxIdleTime: time.Minute,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()
	assert.Equal(t, 20, cfg.MaxSize)
	assert.Equal(t, 2, cfg.MinSize)
	assert.Equal(t, time.Hour, cfg.MaxLifetime)
	assert.Equal(t, 30*time.Minute, cfg.MaxIdleTime)
	assert.Equal(t, 30*time.Second, cfg.HealthCheckInterval)
	assert.Equal(t, 5*time.Second, cfg.AcquireTimeout)
	assert.NoError(t, cfg.Validate())
}

func TestPoolStats_AverageAcquireTime(t *testing.T) {
	tests := []struct {
		name     string
		stats    PoolStats
		expected time.Duration
	}{
		{
			name:     "zero acquires returns zero",
			stats:    PoolStats{AcquireCount: 0, TotalAcquireTimeUs: 1000},
			expected: 0,
		},
		{
			name:     "calculates average",
			stats:    PoolStats{AcquireCount: 10, TotalAcquireTimeUs: 10000},
			expected: 1000 * time.Microsecond,
		},
		{
			name:     "single acquire",
			stats:    PoolStats{AcquireCount: 1, TotalAcquireTimeUs: 500},
			expected: 500 * time.Microsecond,
		},
		{
			name:     "large values",
			stats:    PoolStats{AcquireCount: 1000000, TotalAcquireTimeUs: 1000000000},
			expected: 1000 * time.Microsecond,
		},
		{
			name:     "zero total time",
			stats:    PoolStats{AcquireCount: 5, TotalAcquireTimeUs: 0},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.stats.AverageAcquireTime())
		})
	}
}

func TestNewGenericPool_Validation(t *testing.T) {
	_, factory := newFactory()

	tests := []struct {
		name    string
		cfg     *PoolConfig
		factory ConnFactory
		checker ConnHealthChecker
		closer  ConnCloser
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config uses defaults",
			cfg:     nil,
			factory: factory,
			checker: okChecker,
			closer:  testCloser,
			wantErr: false,
		},
		{
			name: "invalid config - zero max size",
			cfg: &PoolConfig{
				MaxSize:     0,
				MaxLifetime: time.Hour,
				MaxIdleTime: time.Minute,
			},
			factory: factory,
			checker: okChecker,
			closer:  testCloser,
			wantErr: true,
			errMsg:  "max size",
		},
		{
			name:    "nil factory",
			cfg:     nil,
			factory: nil,
			checker: okChecker,
			closer:  testCloser,
			wantErr: true,
			errMsg:  "factory must not be nil",
		},
		{
			name:    "nil checker",
			cfg:     nil,
			factory: factory,
			checker: nil,
			closer:  testCloser,
			wantErr: true,
			errMsg:  "health checker must not be nil",
		},
		{
			name:    "nil closer",
			cfg:     nil,
			factory: factory,
			checker: okChecker,
			closer:  nil,
			wantErr: true,
			errMsg:  "closer must not be nil",
		},
		{
			name: "valid config with health check disabled",
			cfg: &PoolConfig{
				MaxSize:             10,
				MinSize:             2,
				MaxLifetime:         time.Hour,
				MaxIdleTime:         30 * time.Minute,
				HealthCheckInterval: 0,
				AcquireTimeout:      time.Second,
			},
			factory: factory,
			checker: okChecker,
			closer:  testCloser,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewGenericPool(tt.cfg, tt.factory, tt.checker, tt.closer)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, p)
			_ = p.Close()
		})
	}
}

func TestGenericPool_AcquireRelease(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             1,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0, // Disable for test.
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Acquire a connection.
	conn, err := p.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, conn)

	tc := conn.(*testConn)
	assert.Equal(t, 1, tc.id)

	stats := p.Stats()
	assert.Equal(t, int64(1), stats.AcquiredConns)
	assert.Equal(t, int64(1), stats.AcquireCount)

	// Release the connection.
	p.Release(conn)

	stats = p.Stats()
	assert.Equal(t, int64(1), stats.IdleConns)
}

func TestGenericPool_ReuseConnection(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             1,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Acquire and release.
	conn1, err := p.Acquire(ctx)
	require.NoError(t, err)
	id1 := conn1.(*testConn).id
	p.Release(conn1)

	// Acquire again should reuse.
	conn2, err := p.Acquire(ctx)
	require.NoError(t, err)
	id2 := conn2.(*testConn).id
	p.Release(conn2)

	assert.Equal(t, id1, id2, "should reuse the released connection")
}

func TestGenericPool_Close(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             1,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)

	ctx := context.Background()

	// Acquire, release, then close.
	conn, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(conn)

	err = p.Close()
	require.NoError(t, err)

	// Verify connection was closed
	tc := conn.(*testConn)
	assert.True(t, tc.IsClosed())

	// Double close is safe.
	err = p.Close()
	require.NoError(t, err)

	// Acquire after close fails.
	_, err = p.Acquire(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestGenericPool_ConcurrentAccess(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             10,
		MinSize:             2,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      2 * time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := p.Acquire(ctx)
			if err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
			p.Release(conn)
		}()
	}

	wg.Wait()

	stats := p.Stats()
	assert.Greater(t, stats.AcquireCount, int64(0))
	assert.LessOrEqual(t, stats.MaxConcurrent, int64(10))
}

func TestGenericPool_Interface(t *testing.T) {
	var _ Pool = (*GenericPool)(nil)
}

func TestGenericPool_AcquireTimeout(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             1, // Only allow 1 connection
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      100 * time.Millisecond,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Acquire the only available connection
	conn1, err := p.Acquire(ctx)
	require.NoError(t, err)

	// Try to acquire another - should timeout
	_, err = p.Acquire(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire")

	stats := p.Stats()
	assert.Equal(t, int64(1), stats.AcquireErrors)

	// Release the connection
	p.Release(conn1)
}

func TestGenericPool_FactoryError(t *testing.T) {
	factory := newFailingFactory("factory error")
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	_, err = p.Acquire(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create connection")
	assert.Contains(t, err.Error(), "factory error")

	stats := p.Stats()
	assert.Equal(t, int64(1), stats.AcquireErrors)
}

func TestGenericPool_MaxLifetimeExpiration(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         1 * time.Millisecond, // Very short lifetime
		MaxIdleTime:         time.Hour,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Acquire and release a connection
	conn1, err := p.Acquire(ctx)
	require.NoError(t, err)
	id1 := conn1.(*testConn).id
	p.Release(conn1)

	// Wait for lifetime to expire
	time.Sleep(10 * time.Millisecond)

	// Acquire again - should get a new connection (old one expired)
	conn2, err := p.Acquire(ctx)
	require.NoError(t, err)
	id2 := conn2.(*testConn).id
	p.Release(conn2)

	// The first connection should have been closed due to lifetime expiration
	assert.NotEqual(t, id1, id2, "should get a new connection after lifetime expiration")
}

func TestGenericPool_MaxIdleTimeExpiration(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         1 * time.Millisecond, // Very short idle time
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Acquire and release a connection
	conn1, err := p.Acquire(ctx)
	require.NoError(t, err)
	id1 := conn1.(*testConn).id
	p.Release(conn1)

	// Wait for idle time to expire
	time.Sleep(10 * time.Millisecond)

	// Acquire again - should get a new connection (old one was idle too long)
	conn2, err := p.Acquire(ctx)
	require.NoError(t, err)
	id2 := conn2.(*testConn).id
	p.Release(conn2)

	assert.NotEqual(t, id1, id2, "should get a new connection after idle expiration")
}

func TestGenericPool_HealthCheckEviction(t *testing.T) {
	_, factory := newFactory()

	// Track health check calls
	var checkCount int64
	checker := func(_ context.Context, _ Conn) error {
		atomic.AddInt64(&checkCount, 1)
		return errors.New("unhealthy")
	}

	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         time.Hour,
		HealthCheckInterval: 50 * time.Millisecond,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, checker, testCloser)
	require.NoError(t, err)

	ctx := context.Background()

	// Acquire and release a connection to put it in idle pool
	conn, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(conn)

	// Wait for health check to run
	time.Sleep(150 * time.Millisecond)

	// Close the pool
	_ = p.Close()

	// Health checker should have been called
	assert.Greater(t, atomic.LoadInt64(&checkCount), int64(0))
}

func TestGenericPool_ReleaseToClosedPool(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)

	ctx := context.Background()

	// Acquire a connection
	conn, err := p.Acquire(ctx)
	require.NoError(t, err)

	// Close the pool while connection is held
	_ = p.Close()

	// Release should close the connection
	p.Release(conn)

	// Verify connection was closed
	tc := conn.(*testConn)
	assert.True(t, tc.IsClosed())
}

func TestGenericPool_CloseWithError(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, failingCloser)
	require.NoError(t, err)

	ctx := context.Background()

	// Acquire and release to put connection in idle pool
	conn, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(conn)

	// Close should return the closer error
	err = p.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close error")
}

func TestGenericPool_StatsAccuracy(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             10,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Initial stats
	stats := p.Stats()
	assert.Equal(t, int64(0), stats.TotalConns)
	assert.Equal(t, int64(0), stats.IdleConns)
	assert.Equal(t, int64(0), stats.AcquiredConns)
	assert.Equal(t, int64(0), stats.AcquireCount)

	// Acquire connections
	conns := make([]Conn, 5)
	for i := 0; i < 5; i++ {
		var err error
		conns[i], err = p.Acquire(ctx)
		require.NoError(t, err)
	}

	stats = p.Stats()
	assert.Equal(t, int64(5), stats.AcquiredConns)
	assert.Equal(t, int64(5), stats.AcquireCount)

	// Release all
	for _, conn := range conns {
		p.Release(conn)
	}

	stats = p.Stats()
	assert.Equal(t, int64(5), stats.IdleConns)
	assert.Equal(t, int64(5), stats.AcquireCount)
}

func TestGenericPool_MaxConcurrentTracking(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             10,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      2 * time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Acquire 5 connections concurrently, hold them, then release
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := p.Acquire(ctx)
			if err != nil {
				return
			}
			time.Sleep(50 * time.Millisecond) // Hold the connection
			p.Release(conn)
		}()
	}

	wg.Wait()

	stats := p.Stats()
	assert.GreaterOrEqual(t, stats.MaxConcurrent, int64(1))
	assert.LessOrEqual(t, stats.MaxConcurrent, int64(5))
}

func TestGenericPool_AcquireWithCanceledContext(t *testing.T) {
	factory := newDelayedFactory(500 * time.Millisecond)
	cfg := &PoolConfig{
		MaxSize:             1,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      2 * time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = p.Acquire(ctx)
	require.Error(t, err)

	stats := p.Stats()
	assert.Equal(t, int64(1), stats.AcquireErrors)
}

func TestGenericPool_ZeroAcquireTimeout(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      0, // Zero timeout should use default of 5 seconds
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	conn, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(conn)
}

func TestGenericPool_EvictUnhealthy_EmptyPool(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 50 * time.Millisecond,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)

	// Wait for health check to run on empty pool
	time.Sleep(100 * time.Millisecond)

	_ = p.Close()
}

func TestGenericPool_EvictUnhealthy_ClosedPool(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 50 * time.Millisecond,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)

	ctx := context.Background()

	// Add a connection to idle pool
	conn, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(conn)

	// Close the pool
	_ = p.Close()

	// Health check should handle closed pool gracefully
	time.Sleep(100 * time.Millisecond)
}

func TestGenericPool_MultipleExpiredConnections(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             10,
		MinSize:             0,
		MaxLifetime:         1 * time.Millisecond, // Very short
		MaxIdleTime:         time.Hour,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Acquire and release multiple connections
	for i := 0; i < 5; i++ {
		conn, err := p.Acquire(ctx)
		require.NoError(t, err)
		p.Release(conn)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Acquire should skip all expired connections
	conn, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(conn)
}

func TestPoolStats_AllFields(t *testing.T) {
	stats := PoolStats{
		TotalConns:         10,
		IdleConns:          3,
		AcquiredConns:      7,
		AcquireCount:       100,
		AcquireErrors:      5,
		MaxConcurrent:      8,
		TotalAcquireTimeUs: 50000,
	}

	assert.Equal(t, int64(10), stats.TotalConns)
	assert.Equal(t, int64(3), stats.IdleConns)
	assert.Equal(t, int64(7), stats.AcquiredConns)
	assert.Equal(t, int64(100), stats.AcquireCount)
	assert.Equal(t, int64(5), stats.AcquireErrors)
	assert.Equal(t, int64(8), stats.MaxConcurrent)
	assert.Equal(t, int64(50000), stats.TotalAcquireTimeUs)
	assert.Equal(t, 500*time.Microsecond, stats.AverageAcquireTime())
}

func TestGenericPool_HealthyConnectionsSurvive(t *testing.T) {
	getCount, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         time.Hour,
		HealthCheckInterval: 50 * time.Millisecond,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Add connections to idle pool
	conn, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(conn)

	// Record factory count after first connection
	countAfterFirst := getCount()

	// Wait for health check to run
	time.Sleep(100 * time.Millisecond)

	// Acquire should reuse the healthy connection (no new factory calls)
	conn2, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(conn2)

	// Verify no new connections were created (factory count unchanged)
	// This confirms healthy connections survive health checks
	countAfterSecond := getCount()
	assert.Equal(t, countAfterFirst, countAfterSecond, "healthy connection should be reused, not recreated")
}

func TestGenericPool_LIFO_Order(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             10,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         time.Hour,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Acquire multiple connections
	conn1, _ := p.Acquire(ctx)
	conn2, _ := p.Acquire(ctx)
	conn3, _ := p.Acquire(ctx)

	id1 := conn1.(*testConn).id
	id2 := conn2.(*testConn).id
	id3 := conn3.(*testConn).id

	// Release in order: 1, 2, 3
	p.Release(conn1)
	p.Release(conn2)
	p.Release(conn3)

	// Acquire should return in LIFO order: 3, 2, 1
	got1, _ := p.Acquire(ctx)
	got2, _ := p.Acquire(ctx)
	got3, _ := p.Acquire(ctx)

	assert.Equal(t, id3, got1.(*testConn).id)
	assert.Equal(t, id2, got2.(*testConn).id)
	assert.Equal(t, id1, got3.(*testConn).id)

	p.Release(got1)
	p.Release(got2)
	p.Release(got3)
}

func TestGenericPool_TotalAcquireTime(t *testing.T) {
	factory := newDelayedFactory(10 * time.Millisecond)
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      2 * time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	conn, err := p.Acquire(ctx)
	require.NoError(t, err)
	p.Release(conn)

	stats := p.Stats()
	// Acquire should have taken at least 10ms (10000 us)
	assert.Greater(t, stats.TotalAcquireTimeUs, int64(5000))
}

func TestNewGenericPool_WithHealthCheckEnabled(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 100 * time.Millisecond, // Enable health check
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)

	// Let health check loop run
	time.Sleep(50 * time.Millisecond)

	_ = p.Close()
}

func TestGenericPool_StopChannelClosed(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 50 * time.Millisecond,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)

	// Wait a bit for health check loop to start
	time.Sleep(30 * time.Millisecond)

	// Close should stop the health check loop
	err = p.Close()
	require.NoError(t, err)

	// Double close should not panic
	err = p.Close()
	require.NoError(t, err)
}

func TestManagedConn_Fields(t *testing.T) {
	conn := &testConn{id: 42}
	now := time.Now()

	mc := &managedConn{
		conn:      conn,
		createdAt: now,
		lastUsed:  now.Add(time.Second),
	}

	assert.Equal(t, conn, mc.conn)
	assert.Equal(t, now, mc.createdAt)
	assert.Equal(t, now.Add(time.Second), mc.lastUsed)
}

func TestGenericPool_AcquireUpdatesLastUsed(t *testing.T) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             5,
		MinSize:             0,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         time.Hour,
		HealthCheckInterval: 0,
		AcquireTimeout:      time.Second,
	}

	p, err := NewGenericPool(cfg, factory, okChecker, testCloser)
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	// Acquire and release
	conn1, _ := p.Acquire(ctx)
	p.Release(conn1)

	// Small delay
	time.Sleep(5 * time.Millisecond)

	// Acquire again - should update lastUsed
	conn2, _ := p.Acquire(ctx)
	assert.Equal(t, conn1, conn2) // Same connection
	p.Release(conn2)
}

func BenchmarkGenericPool_AcquireRelease(b *testing.B) {
	_, factory := newFactory()
	cfg := &PoolConfig{
		MaxSize:             100,
		MinSize:             10,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 0,
		AcquireTimeout:      5 * time.Second,
	}

	p, _ := NewGenericPool(cfg, factory, okChecker, testCloser)
	defer func() { _ = p.Close() }()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := p.Acquire(ctx)
			if err != nil {
				b.Fatal(err)
			}
			p.Release(conn)
		}
	})
}

func BenchmarkPoolStats_AverageAcquireTime(b *testing.B) {
	stats := PoolStats{
		AcquireCount:       1000000,
		TotalAcquireTimeUs: 50000000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stats.AverageAcquireTime()
	}
}
