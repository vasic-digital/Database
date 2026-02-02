// Package postgres provides a PostgreSQL implementation of the database.Database
// interface using pgx/v5 and pgxpool for connection pooling.
package postgres

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	db "digital.vasic.database/pkg/database"
)

// Client implements database.Database for PostgreSQL.
type Client struct {
	pool   *pgxpool.Pool
	config *Config
}

// Config holds PostgreSQL-specific configuration.
type Config struct {
	// Base configuration.
	db.Config

	// ApplicationName identifies the connection in pg_stat_activity.
	ApplicationName string

	// HealthCheckPeriod is the interval between automatic health checks.
	HealthCheckPeriod time.Duration

	// PreferSimpleProtocol uses the simple query protocol for better
	// performance with simple queries.
	PreferSimpleProtocol bool

	// StatementCacheCapacity controls the prepared statement cache size.
	StatementCacheCapacity int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	cpuCount := int32(runtime.NumCPU())
	maxConns := cpuCount*2 + 1
	if maxConns < 10 {
		maxConns = 10
	}
	if maxConns > 50 {
		maxConns = 50
	}

	return &Config{
		Config: db.Config{
			Driver:          "postgres",
			Host:            "localhost",
			Port:            5432,
			SSLMode:         "disable",
			MaxConns:        maxConns,
			MinConns:        cpuCount / 2,
			MaxConnLifetime: time.Hour,
			MaxConnIdleTime: 30 * time.Minute,
			ConnectTimeout:  5 * time.Second,
		},
		ApplicationName:        "database-module",
		HealthCheckPeriod:      30 * time.Second,
		PreferSimpleProtocol:   true,
		StatementCacheCapacity: 512,
	}
}

// New creates a new PostgreSQL client. Call Connect to establish the
// connection pool.
func New(cfg *Config) *Client {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg.Driver = "postgres"
	return &Client{config: cfg}
}

// Connect establishes the pgxpool connection pool.
func (c *Client) Connect(ctx context.Context) error {
	poolCfg, err := c.buildPoolConfig()
	if err != nil {
		return fmt.Errorf("build pool config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	c.pool = pool
	return nil
}

// Close closes the connection pool.
func (c *Client) Close() error {
	if c.pool != nil {
		c.pool.Close()
		c.pool = nil
	}
	return nil
}

// Exec executes a query that does not return rows.
func (c *Client) Exec(
	ctx context.Context, query string, args ...any,
) (db.Result, error) {
	tag, err := c.pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec: %w", err)
	}
	return &pgResult{tag: tag}, nil
}

// Query executes a query that returns rows.
func (c *Client) Query(
	ctx context.Context, query string, args ...any,
) (db.Rows, error) {
	rows, err := c.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return &pgRows{rows: rows}, nil
}

// QueryRow executes a query expected to return at most one row.
func (c *Client) QueryRow(
	ctx context.Context, query string, args ...any,
) db.Row {
	return &pgRow{row: c.pool.QueryRow(ctx, query, args...)}
}

// Begin starts a new transaction.
func (c *Client) Begin(ctx context.Context) (db.Tx, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return &pgTx{tx: tx}, nil
}

// HealthCheck pings the database with a short timeout.
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.pool.Ping(ctx)
}

// Pool returns the underlying pgxpool.Pool for advanced operations.
func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}

// Migrate applies a list of SQL migration statements sequentially.
func (c *Client) Migrate(ctx context.Context, migrations []string) error {
	for i, m := range migrations {
		if _, err := c.pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	return nil
}

// buildPoolConfig translates our Config into a pgxpool.Config.
func (c *Client) buildPoolConfig() (*pgxpool.Config, error) {
	dsn := c.config.DSN()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	if c.config.MaxConns > 0 {
		cfg.MaxConns = c.config.MaxConns
	}
	if c.config.MinConns > 0 {
		cfg.MinConns = c.config.MinConns
	}
	if c.config.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = c.config.MaxConnLifetime
	}
	if c.config.MaxConnIdleTime > 0 {
		cfg.MaxConnIdleTime = c.config.MaxConnIdleTime
	}
	if c.config.HealthCheckPeriod > 0 {
		cfg.HealthCheckPeriod = c.config.HealthCheckPeriod
	}
	if c.config.ConnectTimeout > 0 {
		cfg.ConnConfig.ConnectTimeout = c.config.ConnectTimeout
	}
	if c.config.ApplicationName != "" {
		cfg.ConnConfig.RuntimeParams["application_name"] = c.config.ApplicationName
	}
	if c.config.PreferSimpleProtocol {
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}

	return cfg, nil
}

// pgResult wraps pgconn.CommandTag to implement database.Result.
type pgResult struct {
	tag pgconn.CommandTag
}

func (r *pgResult) RowsAffected() (int64, error) {
	return r.tag.RowsAffected(), nil
}

// pgRow wraps pgx.Row to implement database.Row.
type pgRow struct {
	row pgx.Row
}

func (r *pgRow) Scan(dest ...any) error {
	return r.row.Scan(dest...)
}

// pgRows wraps pgx.Rows to implement database.Rows.
type pgRows struct {
	rows pgx.Rows
}

func (r *pgRows) Next() bool        { return r.rows.Next() }
func (r *pgRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *pgRows) Close() error      { r.rows.Close(); return nil }
func (r *pgRows) Err() error        { return r.rows.Err() }

// pgTx wraps pgx.Tx to implement database.Tx.
type pgTx struct {
	tx pgx.Tx
}

func (t *pgTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *pgTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

func (t *pgTx) Exec(
	ctx context.Context, query string, args ...any,
) (db.Result, error) {
	tag, err := t.tx.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("tx exec: %w", err)
	}
	return &pgResult{tag: tag}, nil
}

func (t *pgTx) Query(
	ctx context.Context, query string, args ...any,
) (db.Rows, error) {
	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("tx query: %w", err)
	}
	return &pgRows{rows: rows}, nil
}

func (t *pgTx) QueryRow(
	ctx context.Context, query string, args ...any,
) db.Row {
	return &pgRow{row: t.tx.QueryRow(ctx, query, args...)}
}
