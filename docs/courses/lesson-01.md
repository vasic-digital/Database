# Lesson 1: Core Interfaces and Configuration

## Learning Objectives

- Understand the interface-driven design that decouples business logic from database drivers
- Define the five core interfaces: `Database`, `Tx`, `Row`, `Rows`, and `Result`
- Build a validated configuration struct with DSN generation

## Key Concepts

- **Interface-Driven Core**: The `database` package contains zero concrete implementations. It defines only interfaces and a configuration struct, allowing adapter packages to implement the contract independently.
- **Driver Agnosticism**: Business logic depends on the `Database` interface, never on PostgreSQL or SQLite directly. This enables testing with in-memory SQLite while running PostgreSQL in production.
- **Config Validation**: The `Config` struct includes a `Validate()` method that enforces driver-specific requirements at construction time, following the fail-fast principle.

## Code Walkthrough

### Source: `pkg/database/database.go`

The `Database` interface defines the full lifecycle contract:

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

Notice that every method accepting context enables cancellation and timeout propagation. The `Tx` interface mirrors the query methods of `Database` but adds `Commit` and `Rollback`.

The supporting result interfaces (`Row`, `Rows`, `Result`) follow Go's standard library patterns -- `Rows` has `Next()`, `Scan()`, `Close()`, and `Err()`, matching what developers expect from `database/sql`.

### Configuration and DSN

The `Config` struct covers all common database parameters (host, port, credentials, pool sizes, timeouts). The `DSN()` method generates a PostgreSQL connection string, while `Validate()` checks driver-specific requirements:

```go
func (c *Config) Validate() error {
    if c.Driver == "" {
        return fmt.Errorf("database config: driver is required")
    }
    switch c.Driver {
    case "postgres":
        // requires host, user, dbname
    case "sqlite":
        // requires dbname (file path)
    default:
        return fmt.Errorf("database config: unsupported driver %q", c.Driver)
    }
    return nil
}
```

This pattern ensures invalid configurations are caught before any connection attempt.

### Source: `pkg/database/database_test.go`

Tests verify config validation, DSN generation, and interface contract expectations using table-driven patterns.

## Practice Exercise

1. Read `pkg/database/database.go` and sketch the interface dependency graph on paper.
2. Write a mock implementation of the `Database` interface that records all method calls and returns predefined results. Use this mock to test a hypothetical `UserService` that calls `Query` and `Exec`.
3. Add a new driver case to `Validate()` (e.g., `"mysql"`) that requires `Host`, `User`, `Password`, and `DBName`. Write table-driven tests for all validation paths.
