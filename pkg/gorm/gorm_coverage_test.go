package gorm_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/glebarez/sqlite"

	adapter "digital.vasic.database/pkg/gorm"
)

// TestAdapter_HealthCheck_AfterClose tests HealthCheck after the connection
// is closed, which should trigger the error branch in HealthCheck when Ping
// fails.
func TestAdapter_HealthCheck_AfterClose(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	a := adapter.New(db)
	// Close the underlying connection first
	require.NoError(t, a.Close())

	// HealthCheck should now fail because the connection is closed
	err = a.HealthCheck()
	assert.Error(t, err)
}

// TestAdapter_ConfigurePool_AfterClose tests ConfigurePool after the
// connection is closed. Even though sql.DB may still be retrievable,
// we verify the nil config error path.
func TestAdapter_ConfigurePool_AfterClose(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	a := adapter.New(db)
	cfg := adapter.DefaultPoolConfig()

	// ConfigurePool on a valid connection should work
	require.NoError(t, a.ConfigurePool(cfg))

	// Close the connection
	require.NoError(t, a.Close())

	// ConfigurePool with nil should still error with "pool config must not be nil"
	err = a.ConfigurePool(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pool config must not be nil")
}

// TestAdapter_ConfigurePool_WithCustomValues tests ConfigurePool with
// custom values to ensure the full apply path is covered.
func TestAdapter_ConfigurePool_WithCustomValues(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	a := adapter.New(db)
	defer a.Close()

	cfg := &adapter.PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 15 * time.Minute,
		ConnMaxIdleTime: 3 * time.Minute,
	}
	err = a.ConfigurePool(cfg)
	assert.NoError(t, err)
}

// TestAdapter_DB_ReturnsSameInstance tests that DB() returns the same
// gorm.DB instance that was used to create the adapter.
func TestAdapter_DB_ReturnsSameInstance(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	a := adapter.New(db)
	defer a.Close()

	assert.Equal(t, db, a.DB())
}
