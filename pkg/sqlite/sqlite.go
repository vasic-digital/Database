// Package sqlite provides a SQLite implementation of the database.Database
// interface using modernc.org/sqlite (pure Go, no CGO).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver

	db "digital.vasic.database/pkg/database"
)

// SQLOpener defines an interface for opening SQL database connections.
// This allows for dependency injection during testing.
type SQLOpener interface {
	Open(driverName, dataSourceName string) (*sql.DB, error)
}

// DefaultSQLOpener is the default implementation using database/sql.
type DefaultSQLOpener struct{}

// Open opens a database connection using the standard sql.Open.
func (d DefaultSQLOpener) Open(driverName, dataSourceName string) (*sql.DB, error) {
	return sql.Open(driverName, dataSourceName)
}

// defaultOpener is the package-level opener used by Connect.
var defaultOpener SQLOpener = DefaultSQLOpener{}

// Client implements database.Database for SQLite.
type Client struct {
	db     *sql.DB
	config *Config
	opener SQLOpener // injected for testing; if nil, uses defaultOpener
}

// Config holds SQLite-specific configuration.
type Config struct {
	// Path is the database file path. Use ":memory:" for in-memory.
	Path string

	// JournalMode controls the journal mode (e.g. "WAL", "DELETE").
	JournalMode string

	// BusyTimeout is the timeout in milliseconds when the database is
	// locked.
	BusyTimeout time.Duration

	// MaxOpenConns limits the number of open connections.
	MaxOpenConns int

	// MaxIdleConns limits the number of idle connections.
	MaxIdleConns int

	// ConnMaxLifetime is the maximum lifetime of a connection.
	ConnMaxLifetime time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(path string) *Config {
	return &Config{
		Path:            path,
		JournalMode:     "WAL",
		BusyTimeout:     5 * time.Second,
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Hour,
	}
}

// New creates a new SQLite client.
func New(cfg *Config) *Client {
	if cfg == nil {
		cfg = DefaultConfig(":memory:")
	}
	return &Client{config: cfg, opener: nil}
}

// WithOpener sets a custom SQLOpener for the client (used for testing).
func (c *Client) WithOpener(opener SQLOpener) *Client {
	c.opener = opener
	return c
}

// Connect opens the SQLite database and applies pragmas.
func (c *Client) Connect(ctx context.Context) error {
	dsn := c.config.Path
	if dsn == "" {
		dsn = ":memory:"
	}

	opener := c.opener
	if opener == nil {
		opener = defaultOpener
	}

	sdb, err := opener.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	if c.config.MaxOpenConns > 0 {
		sdb.SetMaxOpenConns(c.config.MaxOpenConns)
	}
	if c.config.MaxIdleConns > 0 {
		sdb.SetMaxIdleConns(c.config.MaxIdleConns)
	}
	if c.config.ConnMaxLifetime > 0 {
		sdb.SetConnMaxLifetime(c.config.ConnMaxLifetime)
	}

	// Apply pragmas.
	pragmas := []string{
		fmt.Sprintf("PRAGMA journal_mode=%s", c.journalMode()),
		fmt.Sprintf("PRAGMA busy_timeout=%d", c.busyTimeoutMs()),
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := sdb.ExecContext(ctx, p); err != nil {
			_ = sdb.Close()
			return fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	if err := sdb.PingContext(ctx); err != nil {
		_ = sdb.Close()
		return fmt.Errorf("ping sqlite: %w", err)
	}

	c.db = sdb
	return nil
}

// Close closes the database connection.
func (c *Client) Close() error {
	if c.db != nil {
		err := c.db.Close()
		c.db = nil
		return err
	}
	return nil
}

// Exec executes a query that does not return rows.
func (c *Client) Exec(
	ctx context.Context, query string, args ...any,
) (db.Result, error) {
	res, err := c.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec: %w", err)
	}
	return &sqlResult{result: res}, nil
}

// Query executes a query that returns rows.
func (c *Client) Query(
	ctx context.Context, query string, args ...any,
) (db.Rows, error) {
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return &sqlRows{rows: rows}, nil
}

// QueryRow executes a query that returns at most one row.
func (c *Client) QueryRow(
	ctx context.Context, query string, args ...any,
) db.Row {
	return &sqlRow{row: c.db.QueryRowContext(ctx, query, args...)}
}

// Begin starts a new transaction.
func (c *Client) Begin(ctx context.Context) (db.Tx, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return &sqlTx{tx: tx}, nil
}

// HealthCheck pings the database.
func (c *Client) HealthCheck(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("sqlite: database not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.db.PingContext(ctx)
}

// DB returns the underlying *sql.DB for advanced operations.
func (c *Client) DB() *sql.DB {
	return c.db
}

func (c *Client) journalMode() string {
	if c.config.JournalMode != "" {
		return c.config.JournalMode
	}
	return "WAL"
}

func (c *Client) busyTimeoutMs() int64 {
	if c.config.BusyTimeout > 0 {
		return c.config.BusyTimeout.Milliseconds()
	}
	return 5000
}

// sqlResult wraps sql.Result to implement database.Result.
type sqlResult struct {
	result sql.Result
}

func (r *sqlResult) RowsAffected() (int64, error) {
	return r.result.RowsAffected()
}

// sqlRow wraps *sql.Row to implement database.Row.
type sqlRow struct {
	row *sql.Row
}

func (r *sqlRow) Scan(dest ...any) error {
	return r.row.Scan(dest...)
}

// sqlRows wraps *sql.Rows to implement database.Rows.
type sqlRows struct {
	rows *sql.Rows
}

func (r *sqlRows) Next() bool           { return r.rows.Next() }
func (r *sqlRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *sqlRows) Close() error          { return r.rows.Close() }
func (r *sqlRows) Err() error            { return r.rows.Err() }

// sqlTx wraps *sql.Tx to implement database.Tx.
type sqlTx struct {
	tx *sql.Tx
}

func (t *sqlTx) Commit(_ context.Context) error {
	return t.tx.Commit()
}

func (t *sqlTx) Rollback(_ context.Context) error {
	return t.tx.Rollback()
}

func (t *sqlTx) Exec(
	ctx context.Context, query string, args ...any,
) (db.Result, error) {
	res, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("tx exec: %w", err)
	}
	return &sqlResult{result: res}, nil
}

func (t *sqlTx) Query(
	ctx context.Context, query string, args ...any,
) (db.Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("tx query: %w", err)
	}
	return &sqlRows{rows: rows}, nil
}

func (t *sqlTx) QueryRow(
	ctx context.Context, query string, args ...any,
) db.Row {
	return &sqlRow{row: t.tx.QueryRowContext(ctx, query, args...)}
}
