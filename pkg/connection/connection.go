// Package connection provides a dialect-aware database connection wrapper.
//
// It wraps *sql.DB with automatic query rewriting for cross-database
// compatibility. All Exec, Query, and QueryRow methods transparently
// apply dialect transformations (placeholder rewriting, INSERT OR IGNORE,
// boolean literal conversion) before executing.
//
// Design patterns: Proxy (transparent query rewriting), Abstract Factory
// (dialect-based behavior selection).
package connection

import (
	"context"
	"database/sql"
	"time"

	"digital.vasic.database/pkg/dialect"
)

// Config holds database connection parameters.
type Config struct {
	// Type is the database type: "sqlite" or "postgres".
	Type string

	// MaxOpenConns limits the number of open connections.
	MaxOpenConns int

	// MaxIdleConns limits the number of idle connections.
	MaxIdleConns int

	// ConnMaxLifetime is the maximum lifetime of a connection.
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum idle time of a connection.
	ConnMaxIdleTime time.Duration

	// BusyTimeout is the timeout for context creation.
	BusyTimeout time.Duration

	// BooleanColumns lists known boolean column names for rewriting.
	BooleanColumns []string
}

// DB wraps *sql.DB with dialect-aware query rewriting.
type DB struct {
	*sql.DB
	dialect        *dialect.Dialect
	booleanColumns []string
	busyTimeout    time.Duration
}

// Open creates a new database connection. The caller is responsible for
// registering the appropriate SQL driver (e.g., importing a driver package).
func Open(driverName, dsn string, cfg Config) (*DB, error) {
	sqlDB, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, err
	}

	dt := dialect.SQLite
	if cfg.Type == "postgres" {
		dt = dialect.Postgres
	}

	return &DB{
		DB:             sqlDB,
		dialect:        dialect.New(dt),
		booleanColumns: cfg.BooleanColumns,
		busyTimeout:    cfg.BusyTimeout,
	}, nil
}

// Wrap wraps a raw *sql.DB with dialect awareness.
// Primarily used in tests where the caller provides a pre-opened connection.
func Wrap(sqlDB *sql.DB, dialectType dialect.Type) *DB {
	if sqlDB == nil {
		return nil
	}
	return &DB{
		DB:      sqlDB,
		dialect: dialect.New(dialectType),
	}
}

// Dialect returns the database dialect.
func (db *DB) Dialect() *dialect.Dialect {
	return db.dialect
}

// rewriteQuery applies all dialect-specific transformations.
func (db *DB) rewriteQuery(query string) string {
	return db.dialect.RewriteAll(query, db.booleanColumns)
}

// ExecContext executes a query with dialect rewriting.
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.DB.ExecContext(ctx, db.rewriteQuery(query), args...)
}

// QueryContext executes a query with dialect rewriting.
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, db.rewriteQuery(query), args...)
}

// QueryRowContext executes a single-row query with dialect rewriting.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRowContext(ctx, db.rewriteQuery(query), args...)
}

// Exec executes a query with dialect rewriting (background context).
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.ExecContext(context.Background(), db.rewriteQuery(query), args...)
}

// Query executes a query with dialect rewriting (background context).
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.QueryContext(context.Background(), db.rewriteQuery(query), args...)
}

// QueryRow executes a single-row query with dialect rewriting (background context).
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRowContext(context.Background(), db.rewriteQuery(query), args...)
}

// InsertReturningID executes an INSERT and returns the new row's ID.
// PostgreSQL: appends "RETURNING id" and uses QueryRow.
// SQLite: uses Exec + LastInsertId.
func (db *DB) InsertReturningID(ctx context.Context, query string, args ...interface{}) (int64, error) {
	query = db.rewriteQuery(query)
	if db.dialect.IsPostgres() {
		query += " RETURNING id"
		var id int64
		err := db.DB.QueryRowContext(ctx, query, args...).Scan(&id)
		return id, err
	}
	result, err := db.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// TxInsertReturningID executes an INSERT inside a transaction and returns the ID.
func (db *DB) TxInsertReturningID(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) (int64, error) {
	query = db.rewriteQuery(query)
	if db.dialect.IsPostgres() {
		query += " RETURNING id"
		var id int64
		err := tx.QueryRowContext(ctx, query, args...).Scan(&id)
		return id, err
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// TableExists checks if a table exists in the database.
func (db *DB) TableExists(ctx context.Context, tableName string) (bool, error) {
	if db.dialect.IsPostgres() {
		var exists bool
		err := db.DB.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)",
			tableName).Scan(&exists)
		return exists, err
	}
	var count int
	err := db.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		tableName).Scan(&count)
	return count > 0, err
}

// HealthCheck performs a database health check.
func (db *DB) HealthCheck() error {
	ctx, cancel := db.createContext()
	defer cancel()
	return db.PingContext(ctx)
}

// GetStats returns database connection statistics.
func (db *DB) GetStats() sql.DBStats {
	return db.Stats()
}

// DatabaseType returns "postgres" or "sqlite".
func (db *DB) DatabaseType() string {
	if db.dialect.IsPostgres() {
		return "postgres"
	}
	return "sqlite"
}

func (db *DB) createContext() (context.Context, context.CancelFunc) {
	timeout := db.busyTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}
