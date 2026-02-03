# API Reference - Database Module

Module: `digital.vasic.database`

---

## Package `database`

Import: `digital.vasic.database/pkg/database`

Core interfaces and configuration for driver-agnostic database operations.

### Interfaces

#### `Database`

```go
type Database interface {
    Connect(ctx context.Context) error
    Close() error
    Exec(ctx context.Context, query string, args ...any) (Result, error)
    Query(ctx context.Context, query string, args ...any) (Rows, error)
    QueryRow(ctx context.Context, query string, args ...any) Row
    Begin(ctx context.Context) (Tx, error)
    HealthCheck(ctx context.Context) error
}
```

The primary contract for all database operations. Implementations: `postgres.Client`, `sqlite.Client`.

| Method | Description |
|--------|-------------|
| `Connect` | Establishes a connection to the database |
| `Close` | Closes the database connection and releases resources |
| `Exec` | Executes a query that does not return rows (INSERT, UPDATE, DELETE) |
| `Query` | Executes a query that returns multiple rows |
| `QueryRow` | Executes a query that returns at most one row |
| `Begin` | Starts a new database transaction |
| `HealthCheck` | Verifies the database connection is alive |

#### `Tx`

```go
type Tx interface {
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
    Exec(ctx context.Context, query string, args ...any) (Result, error)
    Query(ctx context.Context, query string, args ...any) (Rows, error)
    QueryRow(ctx context.Context, query string, args ...any) Row
}
```

Represents a database transaction with commit/rollback and query methods.

#### `Row`

```go
type Row interface {
    Scan(dest ...any) error
}
```

Represents a single result row. `Scan` copies columns into destination values.

#### `Rows`

```go
type Rows interface {
    Next() bool
    Scan(dest ...any) error
    Close() error
    Err() error
}
```

Represents a multi-row result set with iteration.

| Method | Description |
|--------|-------------|
| `Next` | Advances to the next row; returns false when exhausted |
| `Scan` | Copies columns from the current row into destination values |
| `Close` | Releases resources held by the result set |
| `Err` | Returns any error encountered during iteration |

#### `Result`

```go
type Result interface {
    RowsAffected() (int64, error)
}
```

Represents the outcome of an `Exec` operation.

### Types

#### `Config`

```go
type Config struct {
    Driver          string
    Host            string
    Port            int
    User            string
    Password        string
    DBName          string
    SSLMode         string
    MaxConns        int32
    MinConns        int32
    MaxConnLifetime time.Duration
    MaxConnIdleTime time.Duration
    ConnectTimeout  time.Duration
}
```

Common database configuration. Used directly or embedded by adapter configs.

### Methods on `Config`

#### `(*Config) DSN() string`

Builds a PostgreSQL-style connection string: `postgres://user:pass@host:port/dbname?sslmode=mode`. Defaults port to 5432 and sslmode to `"disable"` when not set.

#### `(*Config) Validate() error`

Validates required fields per driver:
- `"postgres"`: requires Host, User, DBName
- `"sqlite"`: requires DBName (file path)
- Returns error for empty Driver or unsupported driver

---

## Package `postgres`

Import: `digital.vasic.database/pkg/postgres`

PostgreSQL implementation using pgx/v5 and pgxpool.

### Types

#### `Client`

```go
type Client struct { /* unexported fields */ }
```

Implements `database.Database` for PostgreSQL. Manages a `pgxpool.Pool` internally.

#### `Config`

```go
type Config struct {
    database.Config                    // Embedded base configuration
    ApplicationName        string      // pg_stat_activity identifier
    HealthCheckPeriod      time.Duration // Pool health check interval
    PreferSimpleProtocol   bool        // Use simple query protocol
    StatementCacheCapacity int         // Prepared statement cache size
}
```

### Functions

#### `DefaultConfig() *Config`

Returns a Config with production-ready defaults:
- Host: `"localhost"`, Port: `5432`, SSLMode: `"disable"`
- MaxConns: `min(max(CPU*2+1, 10), 50)`, MinConns: `CPU/2`
- MaxConnLifetime: `1h`, MaxConnIdleTime: `30m`, ConnectTimeout: `5s`
- ApplicationName: `"database-module"`, HealthCheckPeriod: `30s`
- PreferSimpleProtocol: `true`, StatementCacheCapacity: `512`

#### `New(cfg *Config) *Client`

Creates a new PostgreSQL client. If `cfg` is nil, uses `DefaultConfig()`. Sets Driver to `"postgres"`.

### Methods on `Client`

#### `(*Client) Connect(ctx context.Context) error`

Builds a pgxpool config from the DSN, creates the pool, and pings the database. Closes the pool on ping failure.

#### `(*Client) Close() error`

Closes the connection pool and sets it to nil.

#### `(*Client) Exec(ctx context.Context, query string, args ...any) (database.Result, error)`

Executes a non-returning query via the pool.

#### `(*Client) Query(ctx context.Context, query string, args ...any) (database.Rows, error)`

Executes a row-returning query via the pool.

#### `(*Client) QueryRow(ctx context.Context, query string, args ...any) database.Row`

Executes a single-row query via the pool.

#### `(*Client) Begin(ctx context.Context) (database.Tx, error)`

Starts a new transaction via the pool.

#### `(*Client) HealthCheck(ctx context.Context) error`

Pings the database with a 3-second timeout.

#### `(*Client) Pool() *pgxpool.Pool`

Returns the underlying pgxpool.Pool for advanced operations.

#### `(*Client) Migrate(ctx context.Context, migrations []string) error`

Executes a list of SQL migration statements sequentially. Returns error on first failure with migration index.

---

## Package `sqlite`

Import: `digital.vasic.database/pkg/sqlite`

SQLite implementation using modernc.org/sqlite (pure Go, no CGO).

### Types

#### `Client`

```go
type Client struct { /* unexported fields */ }
```

Implements `database.Database` for SQLite. Manages a `*sql.DB` internally.

#### `Config`

```go
type Config struct {
    Path            string        // Database file path or ":memory:"
    JournalMode     string        // Journal mode (default: "WAL")
    BusyTimeout     time.Duration // Lock timeout (default: 5s)
    MaxOpenConns    int           // Max open connections (default: 1)
    MaxIdleConns    int           // Max idle connections (default: 1)
    ConnMaxLifetime time.Duration // Max connection lifetime (default: 1h)
}
```

### Functions

#### `DefaultConfig(path string) *Config`

Returns a Config with production-ready defaults for the given path.

#### `New(cfg *Config) *Client`

Creates a new SQLite client. If `cfg` is nil, uses `DefaultConfig(":memory:")`.

### Methods on `Client`

#### `(*Client) Connect(ctx context.Context) error`

Opens the SQLite database, applies pragmas (journal_mode, busy_timeout, foreign_keys, synchronous), and pings. Closes on failure.

#### `(*Client) Close() error`

Closes the database connection.

#### `(*Client) Exec(ctx context.Context, query string, args ...any) (database.Result, error)`

Executes a non-returning query.

#### `(*Client) Query(ctx context.Context, query string, args ...any) (database.Rows, error)`

Executes a row-returning query.

#### `(*Client) QueryRow(ctx context.Context, query string, args ...any) database.Row`

Executes a single-row query.

#### `(*Client) Begin(ctx context.Context) (database.Tx, error)`

Starts a new transaction.

#### `(*Client) HealthCheck(ctx context.Context) error`

Pings the database with a 3-second timeout. Returns error if not connected.

#### `(*Client) DB() *sql.DB`

Returns the underlying `*sql.DB` for advanced operations.

---

## Package `pool`

Import: `digital.vasic.database/pkg/pool`

Generic connection pool with metrics, health checking, and lifecycle management.

### Interfaces

#### `Conn`

```go
type Conn interface{}
```

Empty interface representing an acquired connection. Callers type-assert to their concrete connection type.

#### `Pool`

```go
type Pool interface {
    Acquire(ctx context.Context) (Conn, error)
    Release(conn Conn)
    Stats() PoolStats
    Close() error
}
```

Contract for connection pool operations. Implementation: `GenericPool`.

### Types

#### `PoolConfig`

```go
type PoolConfig struct {
    MaxSize             int           // Max connections (required, > 0)
    MinSize             int           // Min idle connections (>= 0, <= MaxSize)
    MaxLifetime         time.Duration // Max connection lifetime (required, > 0)
    MaxIdleTime         time.Duration // Max idle time (required, > 0)
    HealthCheckInterval time.Duration // Health check interval (0 disables)
    AcquireTimeout      time.Duration // Acquire wait timeout
}
```

#### `PoolStats`

```go
type PoolStats struct {
    TotalConns         int64
    IdleConns          int64
    AcquiredConns      int64
    AcquireCount       int64
    AcquireErrors      int64
    MaxConcurrent      int64
    TotalAcquireTimeUs int64
}
```

#### `GenericPool`

```go
type GenericPool struct { /* unexported fields */ }
```

Goroutine-safe connection pool with semaphore-based concurrency control.

### Function Types

#### `ConnFactory`

```go
type ConnFactory func(ctx context.Context) (Conn, error)
```

Creates new connections for the pool.

#### `ConnHealthChecker`

```go
type ConnHealthChecker func(ctx context.Context, conn Conn) error
```

Checks whether a connection is still healthy. Return nil for healthy.

#### `ConnCloser`

```go
type ConnCloser func(conn Conn) error
```

Closes a connection.

### Functions

#### `DefaultPoolConfig() *PoolConfig`

Returns defaults: MaxSize 20, MinSize 2, MaxLifetime 1h, MaxIdleTime 30m, HealthCheckInterval 30s, AcquireTimeout 5s.

#### `NewGenericPool(cfg *PoolConfig, factory ConnFactory, checker ConnHealthChecker, closer ConnCloser) (*GenericPool, error)`

Creates a new pool. Validates config and requires non-nil factory, checker, and closer. Starts background health check goroutine if `HealthCheckInterval > 0`.

### Methods on `PoolConfig`

#### `(*PoolConfig) Validate() error`

Validates: MaxSize > 0, MinSize >= 0, MinSize <= MaxSize, MaxLifetime > 0, MaxIdleTime > 0.

### Methods on `PoolStats`

#### `(*PoolStats) AverageAcquireTime() time.Duration`

Returns `TotalAcquireTimeUs / AcquireCount` as a `time.Duration`. Returns 0 if no acquires.

### Methods on `GenericPool`

#### `(*GenericPool) Acquire(ctx context.Context) (Conn, error)`

Obtains a connection. Waits for a semaphore slot (up to AcquireTimeout), tries idle reuse (checking lifetime and idle time), or creates via factory. Tracks metrics atomically.

#### `(*GenericPool) Release(conn Conn)`

Returns a connection to the idle list. If pool is closed, closes the connection instead.

#### `(*GenericPool) Stats() PoolStats`

Returns a snapshot of current pool statistics.

#### `(*GenericPool) Close() error`

Stops the health check goroutine, closes all idle connections. Returns the first error encountered.

---

## Package `migration`

Import: `digital.vasic.database/pkg/migration`

Schema migration runner with version tracking, forward application, and rollback.

### Types

#### `Migration`

```go
type Migration struct {
    Version int    // Unique, monotonically increasing identifier
    Name    string // Human-readable description
    Up      string // SQL to apply the migration
    Down    string // SQL to reverse the migration
}
```

#### `Runner`

```go
type Runner struct { /* unexported fields */ }
```

Applies and rolls back migrations against a `database.Database`.

### Functions

#### `NewRunner(database database.Database, table string) *Runner`

Creates a new runner. If `table` is empty, defaults to `"schema_migrations"`.

### Methods on `Runner`

#### `(*Runner) Init(ctx context.Context) error`

Creates the migration tracking table (`version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TIMESTAMP NOT NULL`) if it does not exist.

#### `(*Runner) Applied(ctx context.Context) ([]int, error)`

Returns applied migration versions sorted ascending.

#### `(*Runner) Apply(ctx context.Context, migrations []Migration) error`

Initializes the tracking table, sorts migrations by version, and applies each pending migration in a transaction (executes Up SQL, records version).

#### `(*Runner) Rollback(ctx context.Context, version int) error`

Returns an error instructing to use `RollbackWith` instead (requires migration definitions for Down SQL).

#### `(*Runner) RollbackWith(ctx context.Context, version int, migrations []Migration) error`

Rolls back all applied migrations with `version >= target` in reverse order. Each rollback runs in a transaction (executes Down SQL, deletes tracking record). Returns error if a migration definition is missing or has no Down SQL.

---

## Package `repository`

Import: `digital.vasic.database/pkg/repository`

Generic repository pattern for CRUD operations using Go generics.

### Interfaces

#### `Repository[T any]`

```go
type Repository[T any] interface {
    Create(ctx context.Context, entity *T) error
    GetByID(ctx context.Context, id any) (*T, error)
    Update(ctx context.Context, entity *T) error
    Delete(ctx context.Context, id any) error
    List(ctx context.Context, opts ListOptions) ([]T, error)
    Count(ctx context.Context, opts ListOptions) (int64, error)
}
```

Generic CRUD contract. Implementation: `GenericRepository[T]`.

#### `EntityMapper[T any]`

```go
type EntityMapper[T any] interface {
    TableName() string
    Columns() []string
    ScanRow(row database.Row) (*T, error)
    ScanRows(rows database.Rows) (*T, error)
    InsertSQL(entity *T) (string, []any)
    UpdateSQL(entity *T) (string, []any)
    PrimaryKeyColumn() string
}
```

Maps between database rows and Go structs. One implementation per entity type.

| Method | Description |
|--------|-------------|
| `TableName` | Returns the database table name |
| `Columns` | Returns column names for SELECT queries |
| `ScanRow` | Scans a single `database.Row` into an entity |
| `ScanRows` | Scans the current row from `database.Rows` into an entity |
| `InsertSQL` | Returns the INSERT statement and arguments |
| `UpdateSQL` | Returns the UPDATE statement and arguments |
| `PrimaryKeyColumn` | Returns the primary key column name |

### Types

#### `ListOptions`

```go
type ListOptions struct {
    Offset  int           // Rows to skip
    Limit   int           // Max rows to return (0 = no limit)
    OrderBy string        // Sort expression (e.g. "created_at DESC")
    Where   []WhereClause // Filter conditions
}
```

#### `WhereClause`

```go
type WhereClause struct {
    Expr string // SQL expression with ? placeholders
    Args []any  // Values for the placeholders
}
```

#### `GenericRepository[T any]`

```go
type GenericRepository[T any] struct {
    DB     database.Database
    Mapper EntityMapper[T]
}
```

### Functions

#### `NewGenericRepository[T any](database database.Database, mapper EntityMapper[T]) *GenericRepository[T]`

Creates a new repository for entity type T.

### Methods on `ListOptions`

#### `(*ListOptions) BuildWhereSQL() (string, []any)`

Assembles all WhereClause entries into a `" WHERE expr1 AND expr2"` fragment and a flat args slice. Returns empty string if no clauses.

### Methods on `GenericRepository[T]`

#### `(*GenericRepository[T]) Create(ctx context.Context, entity *T) error`

Inserts a new entity using `Mapper.InsertSQL()`.

#### `(*GenericRepository[T]) GetByID(ctx context.Context, id any) (*T, error)`

Retrieves an entity by primary key using `SELECT columns FROM table WHERE pk = ?`.

#### `(*GenericRepository[T]) Update(ctx context.Context, entity *T) error`

Modifies an existing entity using `Mapper.UpdateSQL()`.

#### `(*GenericRepository[T]) Delete(ctx context.Context, id any) error`

Removes an entity by primary key using `DELETE FROM table WHERE pk = ?`.

#### `(*GenericRepository[T]) List(ctx context.Context, opts ListOptions) ([]T, error)`

Returns entities matching the options. Builds SELECT with WHERE, ORDER BY, LIMIT, OFFSET. Uses `LIMIT -1` before OFFSET when no explicit limit is set (SQLite compatibility).

#### `(*GenericRepository[T]) Count(ctx context.Context, opts ListOptions) (int64, error)`

Returns the count of matching entities using `SELECT COUNT(*)`.

---

## Package `query`

Import: `digital.vasic.database/pkg/query`

Fluent SQL query builder with type-safe conditions.

### Interfaces

#### `Condition`

```go
type Condition interface {
    Build() (string, []any)
}
```

Represents a WHERE or HAVING clause element. Returns the SQL fragment and positional arguments.

### Types

#### `Builder`

```go
type Builder struct { /* unexported fields */ }
```

Constructs SQL SELECT queries fluently via method chaining.

### Functions

#### `New() *Builder`

Creates a new empty query Builder.

#### Condition Constructors

| Function | Signature | SQL Output |
|----------|-----------|------------|
| `Eq` | `Eq(column string, value any) Condition` | `column = ?` |
| `Neq` | `Neq(column string, value any) Condition` | `column != ?` |
| `Gt` | `Gt(column string, value any) Condition` | `column > ?` |
| `Gte` | `Gte(column string, value any) Condition` | `column >= ?` |
| `Lt` | `Lt(column string, value any) Condition` | `column < ?` |
| `Lte` | `Lte(column string, value any) Condition` | `column <= ?` |
| `Like` | `Like(column string, pattern string) Condition` | `column LIKE ?` |
| `IsNull` | `IsNull(column string) Condition` | `column IS NULL` |
| `IsNotNull` | `IsNotNull(column string) Condition` | `column IS NOT NULL` |
| `In` | `In(column string, values ...any) Condition` | `column IN (?, ?, ...)` |
| `And` | `And(conditions ...Condition) Condition` | `(c1 AND c2 AND ...)` |
| `Or` | `Or(conditions ...Condition) Condition` | `(c1 OR c2 OR ...)` |

**Special cases:**
- `In` with zero values produces `1 = 0` (always false)
- `And`/`Or` with zero conditions produces `1 = 1` (always true)
- `And`/`Or` with one condition delegates directly to that condition

### Methods on `Builder`

All chainable methods return `*Builder`.

#### `(*Builder) Select(cols ...string) *Builder`

Sets the columns to select. Defaults to `*` if not called.

#### `(*Builder) From(table string) *Builder`

Sets the table name for the FROM clause.

#### `(*Builder) Where(c Condition) *Builder`

Adds a WHERE condition. Multiple calls are combined with AND.

#### `(*Builder) OrderBy(expr string) *Builder`

Sets the ORDER BY clause (e.g., `"created_at DESC"`).

#### `(*Builder) Limit(n int) *Builder`

Sets the LIMIT clause.

#### `(*Builder) Offset(n int) *Builder`

Sets the OFFSET clause.

#### `(*Builder) GroupBy(expr string) *Builder`

Sets the GROUP BY clause.

#### `(*Builder) Having(c Condition) *Builder`

Adds a HAVING condition. Multiple calls are combined with AND.

#### `(*Builder) Build() (string, []any)`

Assembles the complete SQL string and positional arguments. Clause order: SELECT, FROM, WHERE, GROUP BY, HAVING, ORDER BY, LIMIT, OFFSET.
