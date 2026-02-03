# User Guide - Database Module

## Overview

`digital.vasic.database` is a generic Go module for relational database operations. It provides driver-agnostic interfaces with concrete PostgreSQL and SQLite adapters, a generic connection pool, schema migration management, a generic repository pattern with Go generics, and a fluent SQL query builder.

## Installation

```bash
go get digital.vasic.database
```

Requires Go 1.24 or later.

## PostgreSQL

### Connecting

```go
package main

import (
    "context"
    "log"

    "digital.vasic.database/pkg/postgres"
)

func main() {
    ctx := context.Background()

    cfg := postgres.DefaultConfig()
    cfg.Host = "localhost"
    cfg.Port = 5432
    cfg.User = "myapp"
    cfg.Password = "secret"
    cfg.DBName = "myapp_db"
    cfg.SSLMode = "disable"

    client := postgres.New(cfg)
    if err := client.Connect(ctx); err != nil {
        log.Fatalf("connect: %v", err)
    }
    defer client.Close()

    // Verify connectivity.
    if err := client.HealthCheck(ctx); err != nil {
        log.Fatalf("health check: %v", err)
    }
}
```

`DefaultConfig()` sets sensible pool sizes based on `runtime.NumCPU()`: MaxConns between 10 and 50, MinConns at half the CPU count, one hour max lifetime, 30-minute idle time, and a 5-second connect timeout.

### Configuration Options

The `postgres.Config` struct embeds `database.Config` and adds PostgreSQL-specific fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ApplicationName` | `string` | `"database-module"` | Visible in `pg_stat_activity` |
| `HealthCheckPeriod` | `time.Duration` | `30s` | Interval between pool health checks |
| `PreferSimpleProtocol` | `bool` | `true` | Use simple query protocol for better performance |
| `StatementCacheCapacity` | `int` | `512` | Prepared statement cache size |

### Executing Queries

```go
// Insert a row.
result, err := client.Exec(ctx,
    "INSERT INTO users (name, email) VALUES ($1, $2)",
    "Alice", "alice@example.com",
)
if err != nil {
    log.Fatal(err)
}
affected, _ := result.RowsAffected()
log.Printf("inserted %d row(s)", affected)

// Query a single row.
var name, email string
row := client.QueryRow(ctx,
    "SELECT name, email FROM users WHERE id = $1", 1,
)
if err := row.Scan(&name, &email); err != nil {
    log.Fatal(err)
}

// Query multiple rows.
rows, err := client.Query(ctx, "SELECT id, name FROM users WHERE active = $1", true)
if err != nil {
    log.Fatal(err)
}
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        log.Fatal(err)
    }
    log.Printf("user %d: %s", id, name)
}
if err := rows.Err(); err != nil {
    log.Fatal(err)
}
```

### Transactions

```go
tx, err := client.Begin(ctx)
if err != nil {
    log.Fatal(err)
}

_, err = tx.Exec(ctx,
    "UPDATE accounts SET balance = balance - $1 WHERE id = $2", 100, 1,
)
if err != nil {
    tx.Rollback(ctx)
    log.Fatal(err)
}

_, err = tx.Exec(ctx,
    "UPDATE accounts SET balance = balance + $1 WHERE id = $2", 100, 2,
)
if err != nil {
    tx.Rollback(ctx)
    log.Fatal(err)
}

if err := tx.Commit(ctx); err != nil {
    log.Fatal(err)
}
```

### Accessing the Underlying Pool

For advanced pgxpool operations:

```go
pgxPool := client.Pool()  // Returns *pgxpool.Pool
stat := pgxPool.Stat()
log.Printf("total conns: %d", stat.TotalConns())
```

### Quick Migrations via Migrate()

The PostgreSQL client has a convenience `Migrate()` method for raw SQL statements:

```go
err := client.Migrate(ctx, []string{
    "CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, name TEXT NOT NULL)",
    "CREATE INDEX IF NOT EXISTS idx_users_name ON users (name)",
})
```

## SQLite

### Connecting

```go
package main

import (
    "context"
    "log"

    "digital.vasic.database/pkg/sqlite"
)

func main() {
    ctx := context.Background()

    // File-based database.
    cfg := sqlite.DefaultConfig("./app.db")
    client := sqlite.New(cfg)
    if err := client.Connect(ctx); err != nil {
        log.Fatalf("connect: %v", err)
    }
    defer client.Close()

    // In-memory database (for testing).
    memClient := sqlite.New(sqlite.DefaultConfig(":memory:"))
    if err := memClient.Connect(ctx); err != nil {
        log.Fatalf("connect: %v", err)
    }
    defer memClient.Close()
}
```

### Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Path` | `string` | (required) | Database file path, or `":memory:"` |
| `JournalMode` | `string` | `"WAL"` | SQLite journal mode |
| `BusyTimeout` | `time.Duration` | `5s` | Timeout when database is locked |
| `MaxOpenConns` | `int` | `1` | Max open connections (1 for WAL safety) |
| `MaxIdleConns` | `int` | `1` | Max idle connections |
| `ConnMaxLifetime` | `time.Duration` | `1h` | Max connection lifetime |

On `Connect()`, the following PRAGMAs are applied automatically:
- `journal_mode=WAL` (or configured value)
- `busy_timeout=5000` (or configured value)
- `foreign_keys=ON`
- `synchronous=NORMAL`

### Executing Queries

SQLite uses `?` placeholders (standard `database/sql` convention):

```go
_, err := client.Exec(ctx,
    "INSERT INTO users (name, email) VALUES (?, ?)",
    "Bob", "bob@example.com",
)

var count int
row := client.QueryRow(ctx, "SELECT COUNT(*) FROM users")
row.Scan(&count)
```

### Accessing the Underlying *sql.DB

```go
sqlDB := client.DB()  // Returns *sql.DB
stats := sqlDB.Stats()
log.Printf("open connections: %d", stats.OpenConnections)
```

## Connection Pooling

The `pool` package provides a generic, goroutine-safe connection pool that can wrap any resource type.

### Creating a Pool

```go
import "digital.vasic.database/pkg/pool"

cfg := pool.DefaultPoolConfig()
cfg.MaxSize = 30
cfg.MinSize = 5
cfg.MaxLifetime = 2 * time.Hour
cfg.MaxIdleTime = 15 * time.Minute
cfg.HealthCheckInterval = time.Minute
cfg.AcquireTimeout = 10 * time.Second

p, err := pool.NewGenericPool(
    cfg,
    // Factory: creates new connections.
    func(ctx context.Context) (pool.Conn, error) {
        return net.DialTimeout("tcp", "db-host:5432", 5*time.Second)
    },
    // Health checker: validates connection liveness.
    func(ctx context.Context, conn pool.Conn) error {
        c := conn.(net.Conn)
        c.SetDeadline(time.Now().Add(time.Second))
        _, err := c.Write([]byte{0})
        return err
    },
    // Closer: releases the connection.
    func(conn pool.Conn) error {
        return conn.(net.Conn).Close()
    },
)
if err != nil {
    log.Fatal(err)
}
defer p.Close()
```

### Acquiring and Releasing

```go
conn, err := p.Acquire(ctx)
if err != nil {
    log.Fatal(err)
}
defer p.Release(conn)

// Use conn...
```

### Monitoring

```go
stats := p.Stats()
log.Printf("total: %d, idle: %d, acquired: %d",
    stats.TotalConns, stats.IdleConns, stats.AcquiredConns)
log.Printf("acquire count: %d, errors: %d", stats.AcquireCount, stats.AcquireErrors)
log.Printf("peak concurrent: %d", stats.MaxConcurrent)
log.Printf("avg acquire latency: %v", stats.AverageAcquireTime())
```

### Pool Lifecycle

- Idle connections exceeding `MaxLifetime` or `MaxIdleTime` are evicted on acquire
- A background goroutine runs at `HealthCheckInterval` to evict unhealthy idle connections
- `Close()` stops the health check loop and closes all idle connections

## Migrations

### Defining Migrations

```go
import "digital.vasic.database/pkg/migration"

var migrations = []migration.Migration{
    {
        Version: 1,
        Name:    "create users table",
        Up:      "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT UNIQUE)",
        Down:    "DROP TABLE users",
    },
    {
        Version: 2,
        Name:    "add created_at column",
        Up:      "ALTER TABLE users ADD COLUMN created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP",
        Down:    "ALTER TABLE users DROP COLUMN created_at",
    },
    {
        Version: 3,
        Name:    "create posts table",
        Up: `CREATE TABLE posts (
            id INTEGER PRIMARY KEY,
            user_id INTEGER NOT NULL REFERENCES users(id),
            title TEXT NOT NULL,
            body TEXT
        )`,
        Down: "DROP TABLE posts",
    },
}
```

### Applying Migrations

```go
runner := migration.NewRunner(db, "")  // Uses default table "schema_migrations"

// Apply all pending migrations in version order.
if err := runner.Apply(ctx, migrations); err != nil {
    log.Fatal(err)
}

// Check which versions have been applied.
applied, err := runner.Applied(ctx)
// applied: []int{1, 2, 3}
```

### Rolling Back

```go
// Roll back all migrations with version >= 2, in reverse order.
err := runner.RollbackWith(ctx, 2, migrations)
// This executes migration 3's Down, then migration 2's Down.
```

### Custom Tracking Table

```go
runner := migration.NewRunner(db, "my_migrations")
```

### How It Works

1. `Init()` creates the tracking table (`schema_migrations` by default) with columns: `version INTEGER PRIMARY KEY`, `name TEXT`, `applied_at TIMESTAMP`
2. `Apply()` queries applied versions, sorts provided migrations by version, and runs each pending migration's `Up` SQL inside a transaction
3. `RollbackWith()` finds applied versions >= target, runs each `Down` SQL in reverse order inside a transaction, and deletes the tracking record

## Repository Pattern

### Defining an Entity Mapper

```go
import (
    "fmt"

    db "digital.vasic.database/pkg/database"
    "digital.vasic.database/pkg/repository"
)

type User struct {
    ID    int
    Name  string
    Email string
}

type UserMapper struct{}

func (m *UserMapper) TableName() string          { return "users" }
func (m *UserMapper) Columns() []string          { return []string{"id", "name", "email"} }
func (m *UserMapper) PrimaryKeyColumn() string   { return "id" }

func (m *UserMapper) ScanRow(row db.Row) (*User, error) {
    var u User
    if err := row.Scan(&u.ID, &u.Name, &u.Email); err != nil {
        return nil, err
    }
    return &u, nil
}

func (m *UserMapper) ScanRows(rows db.Rows) (*User, error) {
    var u User
    if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
        return nil, err
    }
    return &u, nil
}

func (m *UserMapper) InsertSQL(u *User) (string, []any) {
    return "INSERT INTO users (name, email) VALUES (?, ?)",
        []any{u.Name, u.Email}
}

func (m *UserMapper) UpdateSQL(u *User) (string, []any) {
    return "UPDATE users SET name = ?, email = ? WHERE id = ?",
        []any{u.Name, u.Email, u.ID}
}
```

### Using the Repository

```go
repo := repository.NewGenericRepository[User](dbClient, &UserMapper{})

// Create.
err := repo.Create(ctx, &User{Name: "Alice", Email: "alice@example.com"})

// Read.
user, err := repo.GetByID(ctx, 1)

// Update.
user.Name = "Alice Smith"
err = repo.Update(ctx, user)

// Delete.
err = repo.Delete(ctx, 1)
```

### Listing with Filters

```go
users, err := repo.List(ctx, repository.ListOptions{
    Where: []repository.WhereClause{
        {Expr: "email LIKE ?", Args: []any{"%@example.com"}},
    },
    OrderBy: "name ASC",
    Limit:   10,
    Offset:  0,
})

count, err := repo.Count(ctx, repository.ListOptions{
    Where: []repository.WhereClause{
        {Expr: "active = ?", Args: []any{true}},
    },
})
```

## Query Builder

### Basic Queries

```go
import "digital.vasic.database/pkg/query"

sql, args := query.New().
    Select("id", "name", "email").
    From("users").
    Where(query.Eq("active", true)).
    OrderBy("name ASC").
    Limit(20).
    Offset(40).
    Build()
// sql:  "SELECT id, name, email FROM users WHERE active = ? ORDER BY name ASC LIMIT 20 OFFSET 40"
// args: [true]
```

### Condition Types

```go
// Comparison operators.
query.Eq("status", "active")      // status = ?
query.Neq("role", "admin")        // role != ?
query.Gt("age", 18)               // age > ?
query.Gte("score", 90)            // score >= ?
query.Lt("price", 100)            // price < ?
query.Lte("quantity", 0)          // quantity <= ?
query.Like("name", "%alice%")     // name LIKE ?

// Null checks.
query.IsNull("deleted_at")        // deleted_at IS NULL
query.IsNotNull("verified_at")    // verified_at IS NOT NULL

// Set membership.
query.In("status", "active", "pending", "review")
// status IN (?, ?, ?)

// Composite conditions.
query.And(
    query.Gte("age", 18),
    query.Lt("age", 65),
)
// (age >= ? AND age < ?)

query.Or(
    query.Eq("role", "admin"),
    query.Eq("role", "superadmin"),
)
// (role = ? OR role = ?)
```

### Complex Queries

```go
sql, args := query.New().
    Select("department", "COUNT(*) as cnt").
    From("employees").
    Where(query.IsNotNull("hired_at")).
    Where(query.Gte("salary", 50000)).
    GroupBy("department").
    Having(query.Gt("COUNT(*)", 5)).
    OrderBy("cnt DESC").
    Limit(10).
    Build()
// SELECT department, COUNT(*) as cnt FROM employees
//   WHERE hired_at IS NOT NULL AND salary >= ?
//   GROUP BY department HAVING COUNT(*) > ?
//   ORDER BY cnt DESC LIMIT 10
// args: [50000, 5]
```

### Nested Conditions

```go
sql, args := query.New().
    Select("*").
    From("products").
    Where(query.And(
        query.Or(
            query.Eq("category", "electronics"),
            query.Eq("category", "books"),
        ),
        query.Lt("price", 50),
        query.IsNotNull("in_stock_since"),
    )).
    Build()
// SELECT * FROM products WHERE
//   ((category = ? OR category = ?) AND price < ? AND in_stock_since IS NOT NULL)
// args: ["electronics", "books", 50]
```

## Driver-Agnostic Code

All packages except `postgres` and `sqlite` depend only on the `database.Database` interface. Write driver-agnostic business logic by accepting the interface:

```go
import db "digital.vasic.database/pkg/database"

func CountUsers(ctx context.Context, database db.Database) (int64, error) {
    var count int64
    row := database.QueryRow(ctx, "SELECT COUNT(*) FROM users")
    if err := row.Scan(&count); err != nil {
        return 0, fmt.Errorf("count users: %w", err)
    }
    return count, nil
}
```

Swap PostgreSQL or SQLite at the call site:

```go
// Production: PostgreSQL.
pgClient := postgres.New(pgCfg)
pgClient.Connect(ctx)
count, _ := CountUsers(ctx, pgClient)

// Tests: SQLite in-memory.
sqliteClient := sqlite.New(sqlite.DefaultConfig(":memory:"))
sqliteClient.Connect(ctx)
count, _ = CountUsers(ctx, sqliteClient)
```

## Testing

```bash
# All tests with race detection.
go test ./... -count=1 -race

# Unit tests only (short mode).
go test ./... -short

# Integration tests (requires live PostgreSQL).
go test -tags=integration ./...

# Benchmarks.
go test -bench=. ./...

# Single package.
go test -v ./pkg/query/...
```

SQLite tests run without external dependencies since `modernc.org/sqlite` is a pure Go driver. PostgreSQL integration tests require a running server and appropriate environment variables (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`).
