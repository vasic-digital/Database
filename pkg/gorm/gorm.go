// Package gorm provides a GORM adapter for the database module, wrapping a
// *gorm.DB instance with connection pool configuration, health checking, and
// transaction helpers. This adapter provides generic database access for
// any backend where GORM is the primary ORM.
package gorm

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PoolConfig holds connection pool tuning parameters for the underlying
// database/sql connection pool managed by GORM.
type PoolConfig struct {
	// MaxOpenConns is the maximum number of open connections to the
	// database.
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections in the pool.
	MaxIdleConns int

	// ConnMaxLifetime is the maximum amount of time a connection may be
	// reused.
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum amount of time a connection may be
	// idle before being closed.
	ConnMaxIdleTime time.Duration
}

// DefaultPoolConfig returns a PoolConfig with sensible production defaults.
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// Adapter wraps a *gorm.DB and provides convenience methods for health
// checking, transaction management, and connection pool configuration.
type Adapter struct {
	db *gorm.DB
}

// New creates a new Adapter wrapping the given *gorm.DB. The caller is
// responsible for opening the GORM connection with the appropriate driver.
func New(db *gorm.DB) *Adapter {
	if db == nil {
		return nil
	}
	return &Adapter{db: db}
}

// DB returns the underlying *gorm.DB for direct use.
func (a *Adapter) DB() *gorm.DB {
	return a.db
}

// HealthCheck verifies that the underlying database connection is alive by
// pinging it.
func (a *Adapter) HealthCheck() error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return fmt.Errorf("gorm adapter: get sql.DB: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("gorm adapter: ping: %w", err)
	}
	return nil
}

// Transaction executes fn within a database transaction. If fn returns an
// error the transaction is rolled back; otherwise it is committed.
func (a *Adapter) Transaction(fn func(tx *gorm.DB) error) error {
	return a.db.Transaction(fn)
}

// ConfigurePool applies the given PoolConfig to the underlying database/sql
// connection pool.
func (a *Adapter) ConfigurePool(cfg *PoolConfig) error {
	if cfg == nil {
		return fmt.Errorf("gorm adapter: pool config must not be nil")
	}
	sqlDB, err := a.db.DB()
	if err != nil {
		return fmt.Errorf("gorm adapter: get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	return nil
}

// Close closes the underlying database connection.
func (a *Adapter) Close() error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return fmt.Errorf("gorm adapter: get sql.DB: %w", err)
	}
	return sqlDB.Close()
}
