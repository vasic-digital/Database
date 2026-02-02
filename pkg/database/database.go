// Package database defines the core interfaces for database operations.
//
// This package provides driver-agnostic abstractions for connecting to,
// querying, and transacting with relational databases. Implementations
// are provided in sibling packages (postgres, sqlite).
//
// # Core Interfaces
//
//   - Database: connection lifecycle, queries, transactions
//   - Tx: transaction commit/rollback with query methods
//   - Row / Rows: result scanning
//
// # Configuration
//
//	cfg := &database.Config{
//	    Driver:   "postgres",
//	    Host:     "localhost",
//	    Port:     5432,
//	    User:     "app",
//	    Password: "secret",
//	    DBName:   "mydb",
//	}
package database

import (
	"context"
	"fmt"
	"time"
)

// Database defines the contract for database operations.
type Database interface {
	// Connect establishes a connection to the database.
	Connect(ctx context.Context) error

	// Close closes the database connection.
	Close() error

	// Exec executes a query that does not return rows.
	Exec(ctx context.Context, query string, args ...any) (Result, error)

	// Query executes a query that returns rows.
	Query(ctx context.Context, query string, args ...any) (Rows, error)

	// QueryRow executes a query that returns at most one row.
	QueryRow(ctx context.Context, query string, args ...any) Row

	// Begin starts a new transaction.
	Begin(ctx context.Context) (Tx, error)

	// HealthCheck verifies the database connection is alive.
	HealthCheck(ctx context.Context) error
}

// Tx represents a database transaction.
type Tx interface {
	// Commit commits the transaction.
	Commit(ctx context.Context) error

	// Rollback aborts the transaction.
	Rollback(ctx context.Context) error

	// Exec executes a query within the transaction.
	Exec(ctx context.Context, query string, args ...any) (Result, error)

	// Query executes a query that returns rows within the transaction.
	Query(ctx context.Context, query string, args ...any) (Rows, error)

	// QueryRow executes a query that returns at most one row within the
	// transaction.
	QueryRow(ctx context.Context, query string, args ...any) Row
}

// Row represents a single result row.
type Row interface {
	// Scan copies columns from the matched row into dest values.
	Scan(dest ...any) error
}

// Rows represents a result set of multiple rows.
type Rows interface {
	// Next advances to the next row, returning false when exhausted.
	Next() bool

	// Scan copies columns from the current row into dest values.
	Scan(dest ...any) error

	// Close releases the resources held by the result set.
	Close() error

	// Err returns any error encountered during iteration.
	Err() error
}

// Result represents the outcome of an Exec operation.
type Result interface {
	// RowsAffected returns the number of rows affected by the query.
	RowsAffected() (int64, error)
}

// Config holds common database configuration parameters.
type Config struct {
	// Driver identifies the database backend ("postgres", "sqlite").
	Driver string

	// Host is the database server hostname or IP.
	Host string

	// Port is the database server port.
	Port int

	// User is the database username.
	User string

	// Password is the database password.
	Password string

	// DBName is the database name.
	DBName string

	// SSLMode controls SSL/TLS behaviour (e.g. "disable", "require").
	SSLMode string

	// MaxConns is the maximum number of open connections.
	MaxConns int32

	// MinConns is the minimum number of idle connections to maintain.
	MinConns int32

	// MaxConnLifetime is the maximum lifetime of a connection.
	MaxConnLifetime time.Duration

	// MaxConnIdleTime is the maximum idle time before a connection is
	// closed.
	MaxConnIdleTime time.Duration

	// ConnectTimeout is the maximum time to wait when establishing a
	// connection.
	ConnectTimeout time.Duration
}

// DSN builds a PostgreSQL-style connection string from the configuration.
func (c *Config) DSN() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	port := c.Port
	if port == 0 {
		port = 5432
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, port, c.DBName, sslMode,
	)
}

// Validate checks that required fields are populated.
func (c *Config) Validate() error {
	if c.Driver == "" {
		return fmt.Errorf("database config: driver is required")
	}
	switch c.Driver {
	case "postgres":
		if c.Host == "" {
			return fmt.Errorf("database config: host is required for postgres")
		}
		if c.User == "" {
			return fmt.Errorf("database config: user is required for postgres")
		}
		if c.DBName == "" {
			return fmt.Errorf("database config: dbname is required for postgres")
		}
	case "sqlite":
		if c.DBName == "" {
			return fmt.Errorf("database config: dbname (file path) is required for sqlite")
		}
	default:
		return fmt.Errorf("database config: unsupported driver %q", c.Driver)
	}
	return nil
}
