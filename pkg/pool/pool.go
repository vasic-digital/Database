// Package pool provides a generic connection pool abstraction with metrics
// tracking, health checking, and configurable lifecycle parameters.
package pool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Conn represents an acquired connection from the pool.
type Conn interface{}

// Pool defines the contract for a connection pool.
type Pool interface {
	// Acquire obtains a connection from the pool.
	Acquire(ctx context.Context) (Conn, error)

	// Release returns a connection to the pool.
	Release(conn Conn)

	// Stats returns current pool statistics.
	Stats() PoolStats

	// Close closes the pool and all connections.
	Close() error
}

// PoolConfig holds connection pool configuration.
type PoolConfig struct {
	// MaxSize is the maximum number of connections.
	MaxSize int

	// MinSize is the minimum number of idle connections to maintain.
	MinSize int

	// MaxLifetime is the maximum lifetime of a connection before it is
	// closed and replaced.
	MaxLifetime time.Duration

	// MaxIdleTime is the maximum time a connection can be idle before it
	// is closed.
	MaxIdleTime time.Duration

	// HealthCheckInterval is the interval between health checks on idle
	// connections.
	HealthCheckInterval time.Duration

	// AcquireTimeout is the maximum time to wait when acquiring a
	// connection.
	AcquireTimeout time.Duration
}

// DefaultPoolConfig returns a PoolConfig with sensible defaults.
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxSize:             20,
		MinSize:             2,
		MaxLifetime:         time.Hour,
		MaxIdleTime:         30 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      5 * time.Second,
	}
}

// Validate checks that the configuration is internally consistent.
func (c *PoolConfig) Validate() error {
	if c.MaxSize <= 0 {
		return fmt.Errorf("pool config: max size must be positive, got %d", c.MaxSize)
	}
	if c.MinSize < 0 {
		return fmt.Errorf("pool config: min size must be non-negative, got %d", c.MinSize)
	}
	if c.MinSize > c.MaxSize {
		return fmt.Errorf(
			"pool config: min size (%d) must not exceed max size (%d)",
			c.MinSize, c.MaxSize,
		)
	}
	if c.MaxLifetime <= 0 {
		return fmt.Errorf("pool config: max lifetime must be positive")
	}
	if c.MaxIdleTime <= 0 {
		return fmt.Errorf("pool config: max idle time must be positive")
	}
	return nil
}

// PoolStats holds runtime statistics for a connection pool.
type PoolStats struct {
	// TotalConns is the total number of connections currently managed.
	TotalConns int64

	// IdleConns is the number of idle connections.
	IdleConns int64

	// AcquiredConns is the number of connections currently in use.
	AcquiredConns int64

	// AcquireCount is the cumulative count of acquire operations.
	AcquireCount int64

	// AcquireErrors is the cumulative count of failed acquire operations.
	AcquireErrors int64

	// MaxConcurrent is the peak concurrent connection usage observed.
	MaxConcurrent int64

	// TotalAcquireTimeUs is the cumulative acquire time in microseconds.
	TotalAcquireTimeUs int64
}

// AverageAcquireTime returns the average acquire latency.
func (s *PoolStats) AverageAcquireTime() time.Duration {
	if s.AcquireCount == 0 {
		return 0
	}
	return time.Duration(s.TotalAcquireTimeUs/s.AcquireCount) * time.Microsecond
}

// ConnFactory creates new connections for the pool.
type ConnFactory func(ctx context.Context) (Conn, error)

// ConnHealthChecker checks whether a connection is still healthy.
type ConnHealthChecker func(ctx context.Context, conn Conn) error

// ConnCloser closes a connection.
type ConnCloser func(conn Conn) error

// managedConn tracks metadata for a pooled connection.
type managedConn struct {
	conn      Conn
	createdAt time.Time
	lastUsed  time.Time
}

// GenericPool is a configurable, goroutine-safe connection pool.
type GenericPool struct {
	config  *PoolConfig
	factory ConnFactory
	checker ConnHealthChecker
	closer  ConnCloser

	mu       sync.Mutex
	idle     []*managedConn
	sem      chan struct{}
	closed   bool
	stopOnce sync.Once
	stopCh   chan struct{}

	// Metrics (atomic).
	acquireCount       int64
	acquireErrors      int64
	totalAcquireTimeUs int64
	maxConcurrent      int64
	currentConcurrent  int64
}

// NewGenericPool creates a new pool. factory, checker, and closer must not be
// nil.
func NewGenericPool(
	cfg *PoolConfig,
	factory ConnFactory,
	checker ConnHealthChecker,
	closer ConnCloser,
) (*GenericPool, error) {
	if cfg == nil {
		cfg = DefaultPoolConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, fmt.Errorf("pool: factory must not be nil")
	}
	if checker == nil {
		return nil, fmt.Errorf("pool: health checker must not be nil")
	}
	if closer == nil {
		return nil, fmt.Errorf("pool: closer must not be nil")
	}

	p := &GenericPool{
		config:  cfg,
		factory: factory,
		checker: checker,
		closer:  closer,
		idle:    make([]*managedConn, 0, cfg.MaxSize),
		sem:     make(chan struct{}, cfg.MaxSize),
		stopCh:  make(chan struct{}),
	}

	if cfg.HealthCheckInterval > 0 {
		go p.healthCheckLoop()
	}

	return p, nil
}

// Acquire obtains a connection from the pool.
func (p *GenericPool) Acquire(ctx context.Context) (Conn, error) {
	start := time.Now()
	atomic.AddInt64(&p.acquireCount, 1)

	timeout := p.config.AcquireTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for a slot.
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		atomic.AddInt64(&p.acquireErrors, 1)
		return nil, fmt.Errorf("acquire: %w", ctx.Err())
	}

	// Try to reuse an idle connection.
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		<-p.sem
		atomic.AddInt64(&p.acquireErrors, 1)
		return nil, fmt.Errorf("pool is closed")
	}

	now := time.Now()
	for len(p.idle) > 0 {
		mc := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]

		// Check lifetime.
		if now.Sub(mc.createdAt) > p.config.MaxLifetime {
			_ = p.closer(mc.conn)
			continue
		}
		// Check idle time.
		if now.Sub(mc.lastUsed) > p.config.MaxIdleTime {
			_ = p.closer(mc.conn)
			continue
		}

		mc.lastUsed = now
		p.mu.Unlock()
		p.trackAcquire(start)
		return mc.conn, nil
	}
	p.mu.Unlock()

	// Create a new connection.
	conn, err := p.factory(ctx)
	if err != nil {
		<-p.sem
		atomic.AddInt64(&p.acquireErrors, 1)
		return nil, fmt.Errorf("create connection: %w", err)
	}

	p.trackAcquire(start)
	return conn, nil
}

// Release returns a connection to the pool.
func (p *GenericPool) Release(conn Conn) {
	cur := atomic.AddInt64(&p.currentConcurrent, -1)
	_ = cur

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		_ = p.closer(conn)
		<-p.sem
		return
	}

	mc := &managedConn{
		conn:      conn,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}
	p.idle = append(p.idle, mc)
	<-p.sem
}

// Stats returns current pool statistics.
func (p *GenericPool) Stats() PoolStats {
	p.mu.Lock()
	idleCount := int64(len(p.idle))
	p.mu.Unlock()

	acquired := atomic.LoadInt64(&p.currentConcurrent)
	return PoolStats{
		TotalConns:         idleCount + acquired,
		IdleConns:          idleCount,
		AcquiredConns:      acquired,
		AcquireCount:       atomic.LoadInt64(&p.acquireCount),
		AcquireErrors:      atomic.LoadInt64(&p.acquireErrors),
		MaxConcurrent:      atomic.LoadInt64(&p.maxConcurrent),
		TotalAcquireTimeUs: atomic.LoadInt64(&p.totalAcquireTimeUs),
	}
}

// Close shuts down the pool and closes all connections.
func (p *GenericPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	idle := p.idle
	p.idle = nil
	p.mu.Unlock()

	p.stopOnce.Do(func() { close(p.stopCh) })

	var firstErr error
	for _, mc := range idle {
		if err := p.closer(mc.conn); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (p *GenericPool) trackAcquire(start time.Time) {
	elapsed := time.Since(start).Microseconds()
	atomic.AddInt64(&p.totalAcquireTimeUs, elapsed)

	cur := atomic.AddInt64(&p.currentConcurrent, 1)
	for {
		max := atomic.LoadInt64(&p.maxConcurrent)
		if cur <= max || atomic.CompareAndSwapInt64(&p.maxConcurrent, max, cur) {
			break
		}
	}
}

func (p *GenericPool) healthCheckLoop() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.evictUnhealthy()
		case <-p.stopCh:
			return
		}
	}
}

func (p *GenericPool) evictUnhealthy() {
	p.mu.Lock()
	if p.closed || len(p.idle) == 0 {
		p.mu.Unlock()
		return
	}

	// Take a snapshot of idle connections.
	conns := make([]*managedConn, len(p.idle))
	copy(conns, p.idle)
	p.idle = p.idle[:0]
	p.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var healthy []*managedConn
	for _, mc := range conns {
		if err := p.checker(ctx, mc.conn); err != nil {
			_ = p.closer(mc.conn)
			continue
		}
		healthy = append(healthy, mc)
	}

	p.mu.Lock()
	p.idle = append(p.idle, healthy...)
	p.mu.Unlock()
}
