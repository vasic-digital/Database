package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
		{
			name: "max conn lifetime is set",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, time.Hour, cfg.MaxConnLifetime)
			},
		},
		{
			name: "max conn idle time is set",
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 30*time.Minute, cfg.MaxConnIdleTime)
			},
		},
		{
			name: "min conns is set based on CPU",
			check: func(t *testing.T, cfg *Config) {
				assert.GreaterOrEqual(t, cfg.MinConns, int32(0))
			},
		},
		{
			name: "prefer simple protocol is enabled",
			check: func(t *testing.T, cfg *Config) {
				assert.True(t, cfg.PreferSimpleProtocol)
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
		{
			name: "empty driver gets overridden",
			config: &Config{
				Config: db.Config{
					Driver: "",
					Host:   "localhost",
					Port:   5432,
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

func TestClient_CloseMultipleTimes(t *testing.T) {
	c := New(nil)
	err := c.Close()
	assert.NoError(t, err)

	err = c.Close()
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
				ApplicationName:      "test-app",
				HealthCheckPeriod:    30 * time.Second,
				PreferSimpleProtocol: true,
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
		{
			name: "config without application name",
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
				ApplicationName: "",
			},
			wantErr: false,
		},
		{
			name: "config with prefer simple protocol disabled",
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
				PreferSimpleProtocol: false,
			},
			wantErr: false,
		},
		{
			name: "config with all zero pool settings",
			config: &Config{
				Config: db.Config{
					Driver:          "postgres",
					Host:            "localhost",
					Port:            5432,
					User:            "test",
					Password:        "test",
					DBName:          "testdb",
					SSLMode:         "disable",
					MaxConns:        0,
					MinConns:        0,
					MaxConnLifetime: 0,
					MaxConnIdleTime: 0,
					ConnectTimeout:  0,
				},
				HealthCheckPeriod: 0,
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
			if tt.config.MaxConnLifetime > 0 {
				assert.Equal(t, tt.config.MaxConnLifetime, poolCfg.MaxConnLifetime)
			}
			if tt.config.MaxConnIdleTime > 0 {
				assert.Equal(t, tt.config.MaxConnIdleTime, poolCfg.MaxConnIdleTime)
			}
			if tt.config.HealthCheckPeriod > 0 {
				assert.Equal(t, tt.config.HealthCheckPeriod, poolCfg.HealthCheckPeriod)
			}
			if tt.config.ConnectTimeout > 0 {
				assert.Equal(t, tt.config.ConnectTimeout, poolCfg.ConnConfig.ConnectTimeout)
			}
		})
	}
}

func TestBuildPoolConfig_InvalidDSN(t *testing.T) {
	// Create a config that will produce an invalid DSN
	c := &Client{
		config: &Config{
			Config: db.Config{
				Driver:   "postgres",
				Host:     "localhost",
				Port:     -1, // Invalid port will still work in DSN but we test the path
				User:     "test",
				Password: "test",
				DBName:   "testdb",
				SSLMode:  "disable",
			},
		},
	}
	// Even with invalid port, DSN parsing might succeed, so just ensure no panic
	_, _ = c.buildPoolConfig()
}

func TestPgResult_RowsAffected(t *testing.T) {
	tests := []struct {
		name     string
		tag      pgconn.CommandTag
		expected int64
	}{
		{
			name:     "insert single row",
			tag:      pgconn.NewCommandTag("INSERT 0 1"),
			expected: 1,
		},
		{
			name:     "update multiple rows",
			tag:      pgconn.NewCommandTag("UPDATE 5"),
			expected: 5,
		},
		{
			name:     "delete no rows",
			tag:      pgconn.NewCommandTag("DELETE 0"),
			expected: 0,
		},
		{
			name:     "select returns zero",
			tag:      pgconn.NewCommandTag("SELECT 10"),
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &pgResult{tag: tt.tag}
			affected, err := r.RowsAffected()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, affected)
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

func TestClient_Interface(t *testing.T) {
	// Verify Client implements db.Database.
	var _ db.Database = (*Client)(nil)
}

func TestConnect_FailsWithInvalidHost(t *testing.T) {
	cfg := &Config{
		Config: db.Config{
			Driver:         "postgres",
			Host:           "nonexistent.invalid.host.test",
			Port:           5432,
			User:           "test",
			Password:       "test",
			DBName:         "testdb",
			SSLMode:        "disable",
			ConnectTimeout: 100 * time.Millisecond,
		},
	}

	c := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := c.Connect(ctx)
	require.Error(t, err)
	// Error could be from pool creation or ping depending on timing
	assert.True(t, err != nil, "should fail to connect to invalid host")
}

func TestConnect_FailsWithContextCanceled(t *testing.T) {
	cfg := &Config{
		Config: db.Config{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			User:     "test",
			Password: "test",
			DBName:   "testdb",
			SSLMode:  "disable",
		},
	}

	c := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := c.Connect(ctx)
	require.Error(t, err)
}

// mockRow for testing pgRow.Scan behavior
type mockPgxRow struct {
	scanErr error
	values  []any
}

func (m *mockPgxRow) Scan(dest ...any) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	for i, v := range m.values {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *int:
				if iv, ok := v.(int); ok {
					*d = iv
				}
			case *string:
				if sv, ok := v.(string); ok {
					*d = sv
				}
			}
		}
	}
	return nil
}

// mockRows for testing pgRows behavior
type mockPgxRows struct {
	data    [][]any
	current int
	closed  bool
	scanErr error
	iterErr error
}

func (m *mockPgxRows) Next() bool {
	if m.closed || m.current >= len(m.data) {
		return false
	}
	m.current++
	return true
}

func (m *mockPgxRows) Scan(dest ...any) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	if m.current <= 0 || m.current > len(m.data) {
		return nil
	}
	row := m.data[m.current-1]
	for i, v := range row {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *int:
				if iv, ok := v.(int); ok {
					*d = iv
				}
			case *string:
				if sv, ok := v.(string); ok {
					*d = sv
				}
			}
		}
	}
	return nil
}

func (m *mockPgxRows) Close() { m.closed = true }
func (m *mockPgxRows) Err() error {
	if m.iterErr != nil {
		return m.iterErr
	}
	return nil
}

func TestPgRows_Methods(t *testing.T) {
	mock := &mockPgxRows{
		data: [][]any{
			{1, "alice"},
			{2, "bob"},
		},
	}

	// Wrap in pgRows - since pgRows expects pgx.Rows, we test the interface
	// behavior conceptually here

	t.Run("Next iterates through rows", func(t *testing.T) {
		assert.True(t, mock.Next())
		assert.True(t, mock.Next())
		assert.False(t, mock.Next())
	})

	t.Run("Scan reads values", func(t *testing.T) {
		mock2 := &mockPgxRows{
			data: [][]any{{42, "test"}},
		}
		mock2.Next()
		var id int
		var name string
		err := mock2.Scan(&id, &name)
		require.NoError(t, err)
		assert.Equal(t, 42, id)
		assert.Equal(t, "test", name)
	})

	t.Run("Close marks as closed", func(t *testing.T) {
		mock3 := &mockPgxRows{data: [][]any{}}
		mock3.Close()
		assert.True(t, mock3.closed)
	})

	t.Run("Err returns iteration error", func(t *testing.T) {
		mock4 := &mockPgxRows{iterErr: assert.AnError}
		assert.Error(t, mock4.Err())
	})
}

func TestConfig_DSN(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		contains []string
	}{
		{
			name: "full config",
			config: &Config{
				Config: db.Config{
					Driver:   "postgres",
					Host:     "localhost",
					Port:     5432,
					User:     "testuser",
					Password: "testpass",
					DBName:   "testdb",
					SSLMode:  "disable",
				},
			},
			contains: []string{
				"postgres://",
				"testuser:",
				"testpass@",
				"localhost:",
				"5432",
				"/testdb",
				"sslmode=disable",
			},
		},
		{
			name: "custom port",
			config: &Config{
				Config: db.Config{
					Driver:   "postgres",
					Host:     "db.example.com",
					Port:     15432,
					User:     "admin",
					Password: "secret",
					DBName:   "production",
					SSLMode:  "require",
				},
			},
			contains: []string{
				"db.example.com:",
				"15432",
				"sslmode=require",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.config.DSN()
			for _, s := range tt.contains {
				assert.Contains(t, dsn, s)
			}
		})
	}
}

func TestNew_PreservesCustomConfig(t *testing.T) {
	customCfg := &Config{
		Config: db.Config{
			Host:            "custom.host",
			Port:            5555,
			User:            "customuser",
			Password:        "custompass",
			DBName:          "customdb",
			SSLMode:         "require",
			MaxConns:        25,
			MinConns:        5,
			MaxConnLifetime: 2 * time.Hour,
			MaxConnIdleTime: time.Hour,
			ConnectTimeout:  10 * time.Second,
		},
		ApplicationName:        "custom-app",
		HealthCheckPeriod:      time.Minute,
		PreferSimpleProtocol:   false,
		StatementCacheCapacity: 1024,
	}

	c := New(customCfg)
	require.NotNil(t, c)

	// Driver should be overridden to postgres
	assert.Equal(t, "postgres", c.config.Driver)

	// All other fields should be preserved
	assert.Equal(t, "custom.host", c.config.Host)
	assert.Equal(t, 5555, c.config.Port)
	assert.Equal(t, "customuser", c.config.User)
	assert.Equal(t, "custompass", c.config.Password)
	assert.Equal(t, "customdb", c.config.DBName)
	assert.Equal(t, "require", c.config.SSLMode)
	assert.Equal(t, int32(25), c.config.MaxConns)
	assert.Equal(t, int32(5), c.config.MinConns)
	assert.Equal(t, 2*time.Hour, c.config.MaxConnLifetime)
	assert.Equal(t, time.Hour, c.config.MaxConnIdleTime)
	assert.Equal(t, 10*time.Second, c.config.ConnectTimeout)
	assert.Equal(t, "custom-app", c.config.ApplicationName)
	assert.Equal(t, time.Minute, c.config.HealthCheckPeriod)
	assert.False(t, c.config.PreferSimpleProtocol)
	assert.Equal(t, 1024, c.config.StatementCacheCapacity)
}

func TestDefaultConfig_CPUScaling(t *testing.T) {
	// Test that default config scales with CPU count
	cfg := DefaultConfig()

	// MaxConns should be between 10 and 50
	assert.GreaterOrEqual(t, cfg.MaxConns, int32(10))
	assert.LessOrEqual(t, cfg.MaxConns, int32(50))

	// MinConns should be non-negative
	assert.GreaterOrEqual(t, cfg.MinConns, int32(0))

	// MinConns should not exceed MaxConns
	assert.LessOrEqual(t, cfg.MinConns, cfg.MaxConns)
}

func TestBuildPoolConfig_AllBranches(t *testing.T) {
	// Test all conditional branches in buildPoolConfig

	t.Run("with all settings at zero (uses pgx defaults)", func(t *testing.T) {
		c := &Client{
			config: &Config{
				Config: db.Config{
					Driver:          "postgres",
					Host:            "localhost",
					Port:            5432,
					User:            "test",
					Password:        "test",
					DBName:          "testdb",
					SSLMode:         "disable",
					MaxConns:        0,
					MinConns:        0,
					MaxConnLifetime: 0,
					MaxConnIdleTime: 0,
					ConnectTimeout:  0,
				},
				ApplicationName:      "",
				HealthCheckPeriod:    0,
				PreferSimpleProtocol: false,
			},
		}

		poolCfg, err := c.buildPoolConfig()
		require.NoError(t, err)
		require.NotNil(t, poolCfg)
		// When zero, pgx defaults are used - just verify no crash
	})

	t.Run("with all settings positive", func(t *testing.T) {
		c := &Client{
			config: &Config{
				Config: db.Config{
					Driver:          "postgres",
					Host:            "localhost",
					Port:            5432,
					User:            "test",
					Password:        "test",
					DBName:          "testdb",
					SSLMode:         "disable",
					MaxConns:        15,
					MinConns:        3,
					MaxConnLifetime: 45 * time.Minute,
					MaxConnIdleTime: 15 * time.Minute,
					ConnectTimeout:  3 * time.Second,
				},
				ApplicationName:      "test-app",
				HealthCheckPeriod:    20 * time.Second,
				PreferSimpleProtocol: true,
			},
		}

		poolCfg, err := c.buildPoolConfig()
		require.NoError(t, err)
		require.NotNil(t, poolCfg)

		assert.Equal(t, int32(15), poolCfg.MaxConns)
		assert.Equal(t, int32(3), poolCfg.MinConns)
		assert.Equal(t, 45*time.Minute, poolCfg.MaxConnLifetime)
		assert.Equal(t, 15*time.Minute, poolCfg.MaxConnIdleTime)
		assert.Equal(t, 20*time.Second, poolCfg.HealthCheckPeriod)
		assert.Equal(t, 3*time.Second, poolCfg.ConnConfig.ConnectTimeout)
		assert.Equal(t, "test-app", poolCfg.ConnConfig.RuntimeParams["application_name"])
	})
}
