package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "digital.vasic.database/pkg/database"
)

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "driver is postgres",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "postgres", cfg.Driver)
			},
		},
		{
			name: "host defaults to localhost",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "localhost", cfg.Host)
			},
		},
		{
			name: "port defaults to 5432",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 5432, cfg.Port)
			},
		},
		{
			name: "max conns is at least 10",
			check: func(t *testing.T, cfg *Config) {
				assert.GreaterOrEqual(t, cfg.MaxConns, int32(10))
			},
		},
		{
			name: "max conns is at most 50",
			check: func(t *testing.T, cfg *Config) {
				assert.LessOrEqual(t, cfg.MaxConns, int32(50))
			},
		},
		{
			name: "ssl mode defaults to disable",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "disable", cfg.SSLMode)
			},
		},
		{
			name: "connect timeout is set",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 5*time.Second, cfg.ConnectTimeout)
			},
		},
		{
			name: "health check period is set",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 30*time.Second, cfg.HealthCheckPeriod)
			},
		},
		{
			name: "application name is set",
			check: func(t *testing.T, cfg *Config) {
				assert.NotEmpty(t, cfg.ApplicationName)
			},
		},
		{
			name: "statement cache capacity is positive",
			check: func(t *testing.T, cfg *Config) {
				assert.Greater(t, cfg.StatementCacheCapacity, 0)
			},
		},
	}

	cfg := DefaultConfig()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, cfg)
		})
	}
}

func TestNew_SetsDriverToPostgres(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
		},
		{
			name: "custom config overrides driver",
			config: &Config{
				Config: db.Config{
					Driver: "something",
					Host:   "db.test",
					Port:   5433,
					User:   "user",
					DBName: "db",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.config)
			require.NotNil(t, c)
			assert.Equal(t, "postgres", c.config.Driver)
		})
	}
}

func TestClient_PoolNilBeforeConnect(t *testing.T) {
	c := New(nil)
	assert.Nil(t, c.Pool())
}

func TestClient_CloseWithoutConnect(t *testing.T) {
	c := New(nil)
	err := c.Close()
	assert.NoError(t, err)
}

func TestBuildPoolConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config builds successfully",
			config: &Config{
				Config: db.Config{
					Driver:          "postgres",
					Host:            "localhost",
					Port:            5432,
					User:            "test",
					Password:        "test",
					DBName:          "testdb",
					SSLMode:         "disable",
					MaxConns:        10,
					MinConns:        2,
					MaxConnLifetime: time.Hour,
					MaxConnIdleTime: 30 * time.Minute,
					ConnectTimeout:  5 * time.Second,
				},
				ApplicationName:   "test-app",
				HealthCheckPeriod: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "zero values uses pgx defaults",
			config: &Config{
				Config: db.Config{
					Driver:   "postgres",
					Host:     "localhost",
					Port:     5432,
					User:     "test",
					Password: "test",
					DBName:   "testdb",
					SSLMode:  "disable",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{config: tt.config}
			poolCfg, err := c.buildPoolConfig()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, poolCfg)

			if tt.config.MaxConns > 0 {
				assert.Equal(t, tt.config.MaxConns, poolCfg.MaxConns)
			}
			if tt.config.MinConns > 0 {
				assert.Equal(t, tt.config.MinConns, poolCfg.MinConns)
			}
			if tt.config.ApplicationName != "" {
				assert.Equal(t,
					tt.config.ApplicationName,
					poolCfg.ConnConfig.RuntimeParams["application_name"],
				)
			}
		})
	}
}

func TestPgResult_Interface(t *testing.T) {
	// Verify pgResult implements db.Result through the real pgconn type.
	var _ db.Result = (*pgResult)(nil)
}

func TestPgRow_Interface(t *testing.T) {
	var _ db.Row = (*pgRow)(nil)
}

func TestPgRows_Interface(t *testing.T) {
	var _ db.Rows = (*pgRows)(nil)
}

func TestPgTx_Interface(t *testing.T) {
	var _ db.Tx = (*pgTx)(nil)
}
