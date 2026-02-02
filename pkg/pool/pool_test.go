package pool

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testConn is a simple test connection.
type testConn struct {
	id     int
	closed bool
}

func newFactory() (int, ConnFactory) {
	var mu sync.Mutex
	counter := 0
	return counter, func(ctx context.Context) (Conn, error) {
		mu.Lock()
		counter++
		id := counter
		mu.Unlock()
		return &testConn{id: id}, nil
	}
}

func okChecker(_ context.Context, _ Conn) error { return nil }

func failChecker(_ context.Context, _ Conn) error {
	return fmt.Errorf("unhealthy")
}

func testCloser(conn Conn) error {
	if tc, ok := conn.(*testConn); ok {
		tc.closed = true
	}
	return nil
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
			name: "zero max idle time",
			config: PoolConfig{
				MaxSize:     10,
				MaxLifetime: time.Hour,
				MaxIdleTime: 0,
			},
			wantErr: true,
			errMsg:  "max idle time must be positive",
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
			name: "invalid config",
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
