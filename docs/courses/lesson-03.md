# Lesson 3: Connection Pooling and Health Checks

## Learning Objectives

- Implement a semaphore-based connection pool using buffered channels
- Track pool metrics atomically without contention on the hot path
- Build background health checking with idle connection eviction

## Key Concepts

- **Semaphore Pattern**: `GenericPool` uses a buffered channel (`sem chan struct{}`) to enforce `MaxSize`. Sending to the channel acquires a slot; receiving releases one. When the channel is full, `Acquire` blocks.
- **Atomic Metrics**: Pool metrics (`ActiveCount`, `IdleCount`, `TotalCreated`, `TotalClosed`) use `sync/atomic` to avoid locking on the high-frequency hot path.
- **Health Check Goroutine**: A background goroutine periodically snapshots the idle list, health-checks each connection, and retains only healthy ones.

## Code Walkthrough

### Source: `pkg/pool/pool.go`

The pool's core acquisition logic:

1. Send to the semaphore channel (blocks when pool is at capacity)
2. Check the idle list under mutex for a reusable connection
3. If no idle connection, create one via the factory function
4. Return the connection to the caller

Release is the reverse: return the connection to the idle list, then receive from the semaphore channel to free a slot.

The `Close` method stops the health check goroutine, then closes all idle connections. The pool tracks creation and closure counts atomically for observability.

### Source: `pkg/pool/pool_test.go`

Tests cover:
- Basic acquire/release cycle
- Pool exhaustion behavior (blocking when full)
- Concurrent acquire from multiple goroutines
- Health check eviction of unhealthy connections
- Metrics accuracy after multiple operations

### Source: `pkg/connection/connection.go`

Connection management utilities for establishing and configuring database connections, including retry logic and connection string parsing.

## Practice Exercise

1. Read `pkg/pool/pool.go` and identify the semaphore pattern. Explain why a buffered channel is used instead of a `sync.Mutex` with a counter.
2. Write a test that creates a pool with `MaxSize=2`, acquires both connections, then attempts a third acquire in a goroutine. Verify it blocks until one connection is released.
3. Modify the health check logic to add configurable eviction criteria (e.g., maximum connection age). Test that connections older than the threshold are evicted.
