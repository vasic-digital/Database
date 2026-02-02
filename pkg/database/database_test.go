package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_DSN(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "full config",
			config: Config{
				Driver:   "postgres",
				Host:     "localhost",
				Port:     5432,
				User:     "app",
				Password: "secret",
				DBName:   "testdb",
				SSLMode:  "require",
			},
			expected: "postgres://app:secret@localhost:5432/testdb?sslmode=require",
		},
		{
			name: "default ssl mode and port",
			config: Config{
				Driver:   "postgres",
				Host:     "db.example.com",
				User:     "admin",
				Password: "pass",
				DBName:   "prod",
			},
			expected: "postgres://admin:pass@db.example.com:5432/prod?sslmode=disable",
		},
		{
			name: "custom port",
			config: Config{
				Driver:   "postgres",
				Host:     "localhost",
				Port:     15432,
				User:     "test",
				Password: "test123",
				DBName:   "test_db",
				SSLMode:  "disable",
			},
			expected: "postgres://test:test123@localhost:15432/test_db?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.DSN()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty driver",
			config:  Config{},
			wantErr: true,
			errMsg:  "driver is required",
		},
		{
			name: "postgres missing host",
			config: Config{
				Driver: "postgres",
				User:   "app",
				DBName: "db",
			},
			wantErr: true,
			errMsg:  "host is required",
		},
		{
			name: "postgres missing user",
			config: Config{
				Driver: "postgres",
				Host:   "localhost",
				DBName: "db",
			},
			wantErr: true,
			errMsg:  "user is required",
		},
		{
			name: "postgres missing dbname",
			config: Config{
				Driver: "postgres",
				Host:   "localhost",
				User:   "app",
			},
			wantErr: true,
			errMsg:  "dbname is required",
		},
		{
			name: "valid postgres",
			config: Config{
				Driver: "postgres",
				Host:   "localhost",
				User:   "app",
				DBName: "testdb",
			},
			wantErr: false,
		},
		{
			name: "sqlite missing dbname",
			config: Config{
				Driver: "sqlite",
			},
			wantErr: true,
			errMsg:  "dbname (file path) is required",
		},
		{
			name: "valid sqlite",
			config: Config{
				Driver: "sqlite",
				DBName: "/tmp/test.db",
			},
			wantErr: false,
		},
		{
			name: "unsupported driver",
			config: Config{
				Driver: "mysql",
			},
			wantErr: true,
			errMsg:  "unsupported driver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	t.Run("zero values are sensible", func(t *testing.T) {
		cfg := Config{
			Driver: "postgres",
			Host:   "localhost",
			User:   "app",
			DBName: "db",
		}

		assert.Equal(t, int32(0), cfg.MaxConns)
		assert.Equal(t, int32(0), cfg.MinConns)
		assert.Equal(t, time.Duration(0), cfg.MaxConnLifetime)
		assert.Equal(t, time.Duration(0), cfg.MaxConnIdleTime)
		assert.Equal(t, time.Duration(0), cfg.ConnectTimeout)

		err := cfg.Validate()
		require.NoError(t, err)
	})
}
