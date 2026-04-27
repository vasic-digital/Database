package database_test

import (
	"testing"

	"digital.vasic.database/pkg/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Config Validate Edge Cases ---

func TestConfig_Validate_EmptyDriver(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "driver is required")
}

func TestConfig_Validate_UnsupportedDriver(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{Driver: "mysql"}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported driver")
}

func TestConfig_Validate_Postgres_MissingHost(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{Driver: "postgres", User: "app", DBName: "db"}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestConfig_Validate_Postgres_MissingUser(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{Driver: "postgres", Host: "localhost", DBName: "db"}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user is required")
}

func TestConfig_Validate_Postgres_MissingDBName(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{Driver: "postgres", Host: "localhost", User: "app"}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dbname is required")
}

func TestConfig_Validate_Postgres_Valid(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{
		Driver: "postgres",
		Host:   "localhost",
		User:   "app",
		DBName: "mydb",
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_SQLite_MissingDBName(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{Driver: "sqlite"}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dbname")
}

func TestConfig_Validate_SQLite_Valid(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{Driver: "sqlite", DBName: ":memory:"}
	err := cfg.Validate()
	assert.NoError(t, err)
}

// --- Config DSN Edge Cases ---

func TestConfig_DSN_DefaultPort(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{
		Host: "localhost",
		User: "admin",
	}
	dsn := cfg.DSN()
	assert.Contains(t, dsn, ":5432/")
	assert.Contains(t, dsn, "sslmode=disable")
}

func TestConfig_DSN_CustomSSLMode(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{
		Host:    "db.example.com",
		Port:    5433,
		User:    "app",
		DBName:  "mydb",
		SSLMode: "require",
	}
	dsn := cfg.DSN()
	assert.Contains(t, dsn, ":5433/")
	assert.Contains(t, dsn, "sslmode=require")
	assert.Contains(t, dsn, "mydb")
}

func TestConfig_DSN_EmptyPassword(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{
		Host:   "localhost",
		User:   "admin",
		DBName: "db",
	}
	dsn := cfg.DSN()
	// DSN should contain user: with empty password
	assert.Contains(t, dsn, "admin:@")
}

func TestConfig_DSN_SpecialCharsInPassword(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{
		Host:     "localhost",
		User:     "admin",
		Password: "p@ss:w0rd/special",
		DBName:   "db",
	}
	dsn := cfg.DSN()
	// Password is included as-is in the DSN
	assert.Contains(t, dsn, "p@ss:w0rd/special")
}

// --- Nil Config Scenarios ---

func TestConfig_Validate_AllEmpty(t *testing.T) {
	t.Parallel()

	cfg := &database.Config{}
	err := cfg.Validate()
	require.Error(t, err)
}
