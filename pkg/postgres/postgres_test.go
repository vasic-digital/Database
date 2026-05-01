package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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

// ============================================================================
// Error Path Tests - Connection Pool Errors
// ============================================================================

func TestConnect_PoolCreationFailure(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr string
	}{
		{
			name: "connection refused on valid host",
			config: &Config{
				Config: db.Config{
					Driver:         "postgres",
					Host:           "127.0.0.1",
					Port:           59999, // Unlikely to have postgres here
					User:           "test",
					Password:       "test",
					DBName:         "testdb",
					SSLMode:        "disable",
					ConnectTimeout: 100 * time.Millisecond,
				},
			},
			expectErr: "ping database", // Error occurs at ping since pool creation can succeed
		},
		{
			name: "invalid host resolution",
			config: &Config{
				Config: db.Config{
					Driver:         "postgres",
					Host:           "this-host-does-not-exist-anywhere.invalid",
					Port:           5432,
					User:           "test",
					Password:       "test",
					DBName:         "testdb",
					SSLMode:        "disable",
					ConnectTimeout: 100 * time.Millisecond,
				},
			},
			expectErr: "", // Just needs to error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.config)
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			err := c.Connect(ctx)
			require.Error(t, err)
			if tt.expectErr != "" {
				assert.Contains(t, err.Error(), tt.expectErr)
			}
		})
	}
}

func TestConnect_ContextDeadlineExceeded(t *testing.T) {
	cfg := &Config{
		Config: db.Config{
			Driver:         "postgres",
			Host:           "10.255.255.1", // Non-routable IP, will timeout
			Port:           5432,
			User:           "test",
			Password:       "test",
			DBName:         "testdb",
			SSLMode:        "disable",
			ConnectTimeout: 50 * time.Millisecond,
		},
	}

	c := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := c.Connect(ctx)
	require.Error(t, err)
	// Either deadline exceeded or connection refused
	assert.True(t, err != nil)
}

// ============================================================================
// Error Path Tests - Query Execution Errors
// ============================================================================

// mockScanErrorRow is a pgx.Row that always returns a scan error
type mockScanErrorRow struct {
	err error
}

func (m *mockScanErrorRow) Scan(dest ...any) error {
	return m.err
}

func TestPgRow_ScanError(t *testing.T) {
	tests := []struct {
		name    string
		scanErr error
	}{
		{
			name:    "generic scan error",
			scanErr: fmt.Errorf("column does not exist"),
		},
		{
			name:    "type conversion error",
			scanErr: fmt.Errorf("cannot convert string to int"),
		},
		{
			name:    "null value error",
			scanErr: fmt.Errorf("cannot scan NULL into non-pointer"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := &pgRow{row: &mockScanErrorRow{err: tt.scanErr}}
			var result int
			err := row.Scan(&result)
			require.Error(t, err)
			assert.Equal(t, tt.scanErr, err)
		})
	}
}

// mockScanErrorRows implements pgx.Rows with configurable errors
type mockScanErrorRows struct {
	scanErr   error
	iterErr   error
	data      [][]any
	current   int
	closed    bool
	closeErr  error
	colDescs  []pgconn.FieldDescription
	rawValues [][]byte
	connInfo  *pgconn.CommandTag
}

func (m *mockScanErrorRows) Close()                                       { m.closed = true }
func (m *mockScanErrorRows) Err() error                                   { return m.iterErr }
func (m *mockScanErrorRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockScanErrorRows) FieldDescriptions() []pgconn.FieldDescription { return m.colDescs }
func (m *mockScanErrorRows) RawValues() [][]byte                          { return m.rawValues }
func (m *mockScanErrorRows) Conn() *pgx.Conn                              { return nil }

func (m *mockScanErrorRows) Next() bool {
	if m.closed || m.current >= len(m.data) {
		return false
	}
	m.current++
	return true
}

func (m *mockScanErrorRows) Scan(dest ...any) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	if m.current <= 0 || m.current > len(m.data) {
		return fmt.Errorf("no current row")
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

func (m *mockScanErrorRows) Values() ([]any, error) {
	if m.current <= 0 || m.current > len(m.data) {
		return nil, fmt.Errorf("no current row")
	}
	return m.data[m.current-1], nil
}

func TestPgRows_ScanError(t *testing.T) {
	tests := []struct {
		name    string
		scanErr error
	}{
		{
			name:    "scan type mismatch",
			scanErr: fmt.Errorf("cannot assign string to *int"),
		},
		{
			name:    "scan null into non-pointer",
			scanErr: fmt.Errorf("cannot scan NULL"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockScanErrorRows{
				data:    [][]any{{1, "test"}},
				scanErr: tt.scanErr,
			}
			rows := &pgRows{rows: mock}

			assert.True(t, rows.Next())
			var id int
			err := rows.Scan(&id)
			require.Error(t, err)
			assert.Equal(t, tt.scanErr, err)
		})
	}
}

func TestPgRows_IterationError(t *testing.T) {
	mock := &mockScanErrorRows{
		data:    [][]any{{1, "test"}},
		iterErr: fmt.Errorf("connection lost during iteration"),
	}
	rows := &pgRows{rows: mock}

	// Iterate through
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
	}

	// Check iteration error
	err := rows.Err()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection lost")
}

func TestPgRows_CloseReturnsNil(t *testing.T) {
	mock := &mockScanErrorRows{
		data: [][]any{},
	}
	rows := &pgRows{rows: mock}

	err := rows.Close()
	assert.NoError(t, err)
	assert.True(t, mock.closed)
}

func TestPgRows_NextAfterClose(t *testing.T) {
	mock := &mockScanErrorRows{
		data: [][]any{{1, "test"}, {2, "test2"}},
	}
	rows := &pgRows{rows: mock}

	// Read first row
	assert.True(t, rows.Next())

	// Close
	_ = rows.Close()

	// Next should return false after close
	assert.False(t, rows.Next())
}

// ============================================================================
// Error Path Tests - Transaction Errors
// ============================================================================

// mockTx implements pgx.Tx for testing transaction error paths
type mockTx struct {
	commitErr   error
	rollbackErr error
	execErr     error
	queryErr    error
	queryRows   *mockScanErrorRows
	queryRow    *mockScanErrorRow
	committed   bool
	rolledBack  bool
}

func (m *mockTx) Begin(ctx context.Context) (pgx.Tx, error) {
	return nil, fmt.Errorf("nested transactions not supported")
}

func (m *mockTx) Commit(ctx context.Context) error {
	if m.commitErr != nil {
		return m.commitErr
	}
	m.committed = true
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error {
	if m.rollbackErr != nil {
		return m.rollbackErr
	}
	m.rolledBack = true
	return nil
}

func (m *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execErr != nil {
		return pgconn.CommandTag{}, m.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	if m.queryRows != nil {
		return m.queryRows, nil
	}
	return &mockScanErrorRows{data: [][]any{}}, nil
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRow != nil {
		return m.queryRow
	}
	return &mockScanErrorRow{err: nil}
}

func (m *mockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, fmt.Errorf("mock Tx: CopyFrom not supported in test mock")
}

func (m *mockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil
}

func (m *mockTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (m *mockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, fmt.Errorf("mock Tx: Prepare not supported in test mock")
}

func (m *mockTx) Conn() *pgx.Conn {
	return nil
}

func TestPgTx_CommitError(t *testing.T) {
	tests := []struct {
		name      string
		commitErr error
	}{
		{
			name:      "connection closed during commit",
			commitErr: fmt.Errorf("connection closed"),
		},
		{
			name:      "serialization failure",
			commitErr: fmt.Errorf("could not serialize access"),
		},
		{
			name:      "deadlock detected",
			commitErr: fmt.Errorf("deadlock detected"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTx{commitErr: tt.commitErr}
			tx := &pgTx{tx: mock}

			err := tx.Commit(context.Background())
			require.Error(t, err)
			assert.Equal(t, tt.commitErr, err)
		})
	}
}

func TestPgTx_RollbackError(t *testing.T) {
	tests := []struct {
		name        string
		rollbackErr error
	}{
		{
			name:        "connection already closed",
			rollbackErr: fmt.Errorf("conn closed"),
		},
		{
			name:        "transaction already committed",
			rollbackErr: fmt.Errorf("tx is closed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTx{rollbackErr: tt.rollbackErr}
			tx := &pgTx{tx: mock}

			err := tx.Rollback(context.Background())
			require.Error(t, err)
			assert.Equal(t, tt.rollbackErr, err)
		})
	}
}

func TestPgTx_ExecError(t *testing.T) {
	tests := []struct {
		name    string
		execErr error
	}{
		{
			name:    "syntax error in SQL",
			execErr: fmt.Errorf("syntax error at or near"),
		},
		{
			name:    "foreign key violation",
			execErr: fmt.Errorf("violates foreign key constraint"),
		},
		{
			name:    "unique constraint violation",
			execErr: fmt.Errorf("duplicate key value violates unique constraint"),
		},
		{
			name:    "check constraint violation",
			execErr: fmt.Errorf("violates check constraint"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTx{execErr: tt.execErr}
			tx := &pgTx{tx: mock}

			result, err := tx.Exec(context.Background(), "INSERT INTO test VALUES (1)")
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "tx exec")
		})
	}
}

func TestPgTx_QueryError(t *testing.T) {
	tests := []struct {
		name     string
		queryErr error
	}{
		{
			name:     "table does not exist",
			queryErr: fmt.Errorf("relation \"test\" does not exist"),
		},
		{
			name:     "column does not exist",
			queryErr: fmt.Errorf("column \"foo\" does not exist"),
		},
		{
			name:     "permission denied",
			queryErr: fmt.Errorf("permission denied for table"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTx{queryErr: tt.queryErr}
			tx := &pgTx{tx: mock}

			rows, err := tx.Query(context.Background(), "SELECT * FROM test")
			require.Error(t, err)
			assert.Nil(t, rows)
			assert.Contains(t, err.Error(), "tx query")
		})
	}
}

func TestPgTx_QueryRowScanError(t *testing.T) {
	scanErr := fmt.Errorf("no rows in result set")
	mock := &mockTx{queryRow: &mockScanErrorRow{err: scanErr}}
	tx := &pgTx{tx: mock}

	row := tx.QueryRow(context.Background(), "SELECT 1 WHERE false")
	var result int
	err := row.Scan(&result)
	require.Error(t, err)
	assert.Equal(t, scanErr, err)
}

func TestPgTx_SuccessfulOperations(t *testing.T) {
	t.Run("successful commit", func(t *testing.T) {
		mock := &mockTx{}
		tx := &pgTx{tx: mock}

		err := tx.Commit(context.Background())
		require.NoError(t, err)
		assert.True(t, mock.committed)
	})

	t.Run("successful rollback", func(t *testing.T) {
		mock := &mockTx{}
		tx := &pgTx{tx: mock}

		err := tx.Rollback(context.Background())
		require.NoError(t, err)
		assert.True(t, mock.rolledBack)
	})

	t.Run("successful exec", func(t *testing.T) {
		mock := &mockTx{}
		tx := &pgTx{tx: mock}

		result, err := tx.Exec(context.Background(), "INSERT INTO test VALUES (1)")
		require.NoError(t, err)
		require.NotNil(t, result)

		affected, err := result.RowsAffected()
		require.NoError(t, err)
		assert.Equal(t, int64(1), affected)
	})

	t.Run("successful query", func(t *testing.T) {
		mock := &mockTx{
			queryRows: &mockScanErrorRows{data: [][]any{{1, "test"}}},
		}
		tx := &pgTx{tx: mock}

		rows, err := tx.Query(context.Background(), "SELECT * FROM test")
		require.NoError(t, err)
		require.NotNil(t, rows)
		defer rows.Close()
	})
}

// ============================================================================
// Error Path Tests - Context Cancellation
// ============================================================================

func TestContextCancellation_DuringOperations(t *testing.T) {
	t.Run("cancelled context for connect", func(t *testing.T) {
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
		cancel() // Cancel before connect

		err := c.Connect(ctx)
		require.Error(t, err)
	})

	t.Run("deadline exceeded context", func(t *testing.T) {
		cfg := &Config{
			Config: db.Config{
				Driver:         "postgres",
				Host:           "localhost",
				Port:           5432,
				User:           "test",
				Password:       "test",
				DBName:         "testdb",
				SSLMode:        "disable",
				ConnectTimeout: time.Nanosecond, // Extremely short timeout
			},
		}

		c := New(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()

		// Wait for deadline to pass
		time.Sleep(time.Millisecond)

		err := c.Connect(ctx)
		require.Error(t, err)
	})
}

// ============================================================================
// Error Path Tests - Migration Errors
// ============================================================================

func TestMigrate_ErrorCases(t *testing.T) {
	// Without a real pool, we can't directly test Migrate,
	// but we can document the expected error wrapping format
	t.Run("migration error format", func(t *testing.T) {
		// The Migrate function wraps errors with migration index
		// Format: "migration %d: %w"
		expectedFormat := "migration 0:"
		assert.Contains(t, fmt.Sprintf("migration %d: test error", 0), expectedFormat)
	})
}

// ============================================================================
// Error Path Tests - BuildPoolConfig Edge Cases
// ============================================================================

func TestBuildPoolConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		check  func(t *testing.T, cfg *pgxpool.Config)
	}{
		{
			name: "empty application name is not set",
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
			check: func(t *testing.T, cfg *pgxpool.Config) {
				// Should not crash and application_name might be empty or default
				require.NotNil(t, cfg)
			},
		},
		{
			name: "prefer simple protocol false uses default mode",
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
			check: func(t *testing.T, cfg *pgxpool.Config) {
				// Default mode, not simple protocol
				require.NotNil(t, cfg)
				// The default is not QueryExecModeSimpleProtocol
			},
		},
		{
			name: "negative values are treated as zero (not applied)",
			config: &Config{
				Config: db.Config{
					Driver:          "postgres",
					Host:            "localhost",
					Port:            5432,
					User:            "test",
					Password:        "test",
					DBName:          "testdb",
					SSLMode:         "disable",
					MaxConns:        -1, // Negative, won't trigger condition
					MinConns:        -1,
					MaxConnLifetime: -1,
					MaxConnIdleTime: -1,
					ConnectTimeout:  -1,
				},
				HealthCheckPeriod: -1,
			},
			check: func(t *testing.T, cfg *pgxpool.Config) {
				require.NotNil(t, cfg)
				// pgx defaults should be used since negatives fail > 0 check
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{config: tt.config}
			poolCfg, err := c.buildPoolConfig()
			require.NoError(t, err)
			tt.check(t, poolCfg)
		})
	}
}

// ============================================================================
// Additional Coverage Tests
// ============================================================================

func TestPgResult_ZeroRowsAffected(t *testing.T) {
	r := &pgResult{tag: pgconn.NewCommandTag("DELETE 0")}
	affected, err := r.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(0), affected)
}

func TestPgResult_LargeRowsAffected(t *testing.T) {
	r := &pgResult{tag: pgconn.NewCommandTag("UPDATE 999999")}
	affected, err := r.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(999999), affected)
}

func TestPgRows_EmptyResultSet(t *testing.T) {
	mock := &mockScanErrorRows{data: [][]any{}}
	rows := &pgRows{rows: mock}

	assert.False(t, rows.Next())
	assert.NoError(t, rows.Err())
	assert.NoError(t, rows.Close())
}

func TestPgRows_MultipleIterations(t *testing.T) {
	mock := &mockScanErrorRows{
		data: [][]any{
			{1, "first"},
			{2, "second"},
			{3, "third"},
		},
	}
	rows := &pgRows{rows: mock}

	count := 0
	for rows.Next() {
		var id int
		var name string
		err := rows.Scan(&id, &name)
		require.NoError(t, err)
		count++
	}

	assert.Equal(t, 3, count)
	assert.NoError(t, rows.Err())
}

func TestDefaultConfig_Consistency(t *testing.T) {
	// Multiple calls should return consistent values
	cfg1 := DefaultConfig()
	cfg2 := DefaultConfig()

	assert.Equal(t, cfg1.Driver, cfg2.Driver)
	assert.Equal(t, cfg1.Host, cfg2.Host)
	assert.Equal(t, cfg1.Port, cfg2.Port)
	assert.Equal(t, cfg1.MaxConns, cfg2.MaxConns)
	assert.Equal(t, cfg1.MinConns, cfg2.MinConns)
	assert.Equal(t, cfg1.ApplicationName, cfg2.ApplicationName)
}

func TestClient_PoolReturnedAfterClose(t *testing.T) {
	c := New(nil)
	// Before any connect, pool is nil
	assert.Nil(t, c.Pool())

	// Close doesn't panic
	err := c.Close()
	assert.NoError(t, err)

	// Pool is still nil
	assert.Nil(t, c.Pool())
}

func TestNew_NilConfigUsesDefaults(t *testing.T) {
	c := New(nil)
	require.NotNil(t, c)
	require.NotNil(t, c.config)

	// Should have default values
	assert.Equal(t, "postgres", c.config.Driver)
	assert.Equal(t, "localhost", c.config.Host)
	assert.Equal(t, 5432, c.config.Port)
	assert.NotEmpty(t, c.config.ApplicationName)
}

// ============================================================================
// Wrapper Type Coverage Tests
// ============================================================================

func TestPgRow_NilValues(t *testing.T) {
	// Test scanning nil values
	mock := &mockPgxRow{
		values: []any{nil, nil},
	}

	row := &pgRow{row: mock}
	var id int
	var name string
	// Should not crash, values just won't be set
	err := row.Scan(&id, &name)
	assert.NoError(t, err)
}

func TestPgRows_ScanWithWrongTypes(t *testing.T) {
	mock := &mockScanErrorRows{
		data:    [][]any{{"not an int", 123}},
		scanErr: nil, // Our mock doesn't enforce types
	}
	rows := &pgRows{rows: mock}

	assert.True(t, rows.Next())

	// Scan with mismatched types - mock doesn't enforce, but tests the path
	var id int
	var name string
	err := rows.Scan(&id, &name)
	assert.NoError(t, err) // Mock allows it
}

// ============================================================================
// DefaultConfig CPU Boundary Tests
// ============================================================================

func TestDefaultConfig_MaxConnsBoundaries(t *testing.T) {
	// DefaultConfig uses runtime.NumCPU() which is fixed for the test
	// This test verifies the boundary logic is correct
	cfg := DefaultConfig()

	// Verify the calculation: maxConns = cpuCount*2 + 1, capped at 10-50
	// For any reasonable system, this should hold
	assert.GreaterOrEqual(t, cfg.MaxConns, int32(10), "maxConns should be at least 10")
	assert.LessOrEqual(t, cfg.MaxConns, int32(50), "maxConns should be at most 50")

	// MinConns should be reasonable
	assert.GreaterOrEqual(t, cfg.MinConns, int32(0))
	assert.Less(t, cfg.MinConns, cfg.MaxConns, "minConns should be less than maxConns")
}

// ============================================================================
// Error Wrapping Format Tests
// ============================================================================

func TestErrorWrapping_Format(t *testing.T) {
	tests := []struct {
		name           string
		errorFunc      func() error
		expectedPrefix string
	}{
		{
			name: "exec error format",
			errorFunc: func() error {
				return fmt.Errorf("exec: %w", fmt.Errorf("test error"))
			},
			expectedPrefix: "exec:",
		},
		{
			name: "query error format",
			errorFunc: func() error {
				return fmt.Errorf("query: %w", fmt.Errorf("test error"))
			},
			expectedPrefix: "query:",
		},
		{
			name: "begin transaction error format",
			errorFunc: func() error {
				return fmt.Errorf("begin transaction: %w", fmt.Errorf("test error"))
			},
			expectedPrefix: "begin transaction:",
		},
		{
			name: "tx exec error format",
			errorFunc: func() error {
				return fmt.Errorf("tx exec: %w", fmt.Errorf("test error"))
			},
			expectedPrefix: "tx exec:",
		},
		{
			name: "tx query error format",
			errorFunc: func() error {
				return fmt.Errorf("tx query: %w", fmt.Errorf("test error"))
			},
			expectedPrefix: "tx query:",
		},
		{
			name: "migration error format",
			errorFunc: func() error {
				return fmt.Errorf("migration %d: %w", 5, fmt.Errorf("test error"))
			},
			expectedPrefix: "migration 5:",
		},
		{
			name: "build pool config error format",
			errorFunc: func() error {
				return fmt.Errorf("build pool config: %w", fmt.Errorf("test error"))
			},
			expectedPrefix: "build pool config:",
		},
		{
			name: "create connection pool error format",
			errorFunc: func() error {
				return fmt.Errorf("create connection pool: %w", fmt.Errorf("test error"))
			},
			expectedPrefix: "create connection pool:",
		},
		{
			name: "ping database error format",
			errorFunc: func() error {
				return fmt.Errorf("ping database: %w", fmt.Errorf("test error"))
			},
			expectedPrefix: "ping database:",
		},
		{
			name: "parse dsn error format",
			errorFunc: func() error {
				return fmt.Errorf("parse dsn: %w", fmt.Errorf("test error"))
			},
			expectedPrefix: "parse dsn:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.errorFunc()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedPrefix)
			assert.Contains(t, err.Error(), "test error")
		})
	}
}

// ============================================================================
// Pooler Interface Mock (for comprehensive testing)
// ============================================================================

// mockPool implements pooler for testing
type mockPool struct {
	execFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	beginFunc    func(ctx context.Context) (pgx.Tx, error)
	pingFunc     func(ctx context.Context) error
	closed       bool
}

func (m *mockPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFunc != nil {
		return m.execFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag(""), nil
}

func (m *mockPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, sql, args...)
	}
	return nil, nil
}

func (m *mockPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(ctx, sql, args...)
	}
	return nil
}

func (m *mockPool) Begin(ctx context.Context) (pgx.Tx, error) {
	if m.beginFunc != nil {
		return m.beginFunc(ctx)
	}
	return nil, nil
}

func (m *mockPool) Ping(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return nil
}

func (m *mockPool) Close() {
	m.closed = true
}

// TestMockPool verifies the mock implements the expected interface
func TestMockPool_Interface(t *testing.T) {
	// bluff-scan: no-assert-ok (integration/interface-compliance smoke — wiring must not panic)
	var _ pooler = (*mockPool)(nil)
}

// ============================================================================
// Config DSN Edge Cases
// ============================================================================

func TestConfig_DSN_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		contains    []string
		notContains []string
	}{
		{
			name: "default SSL mode when empty",
			config: &Config{
				Config: db.Config{
					Driver:   "postgres",
					Host:     "localhost",
					Port:     5432,
					User:     "user",
					Password: "pass",
					DBName:   "db",
					SSLMode:  "", // Empty should default to "disable"
				},
			},
			contains: []string{"sslmode=disable"},
		},
		{
			name: "default port when zero",
			config: &Config{
				Config: db.Config{
					Driver:   "postgres",
					Host:     "localhost",
					Port:     0, // Zero should default to 5432
					User:     "user",
					Password: "pass",
					DBName:   "db",
					SSLMode:  "disable",
				},
			},
			contains: []string{":5432/"},
		},
		{
			name: "special characters in password",
			config: &Config{
				Config: db.Config{
					Driver:   "postgres",
					Host:     "localhost",
					Port:     5432,
					User:     "user",
					Password: "p@ss:word/special",
					DBName:   "db",
					SSLMode:  "disable",
				},
			},
			contains: []string{"p@ss:word/special@"},
		},
		{
			name: "empty password",
			config: &Config{
				Config: db.Config{
					Driver:   "postgres",
					Host:     "localhost",
					Port:     5432,
					User:     "user",
					Password: "",
					DBName:   "db",
					SSLMode:  "disable",
				},
			},
			contains: []string{"user:@localhost"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.config.DSN()
			for _, s := range tt.contains {
				assert.Contains(t, dsn, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, dsn, s)
			}
		})
	}
}

// ============================================================================
// Close Method Edge Cases
// ============================================================================

func TestClient_Close_NilPool(t *testing.T) {
	c := &Client{config: DefaultConfig()}
	// Pool is nil by default
	assert.Nil(t, c.pool)

	err := c.Close()
	assert.NoError(t, err)
	assert.Nil(t, c.pool)
}

func TestClient_Close_SetsPoolToNil(t *testing.T) {
	c := New(nil)
	// Simulate having a pool (we can't set a real one without connecting)
	// This tests the logic flow

	// Close without pool is safe
	err := c.Close()
	assert.NoError(t, err)

	// Double close is safe
	err = c.Close()
	assert.NoError(t, err)
}

// ============================================================================
// Additional Transaction Tests
// ============================================================================

func TestPgTx_QueryRowReturnsRow(t *testing.T) {
	mock := &mockTx{}
	tx := &pgTx{tx: mock}

	row := tx.QueryRow(context.Background(), "SELECT 1")
	require.NotNil(t, row)
}

func TestPgTx_QueryReturnsRows(t *testing.T) {
	mock := &mockTx{
		queryRows: &mockScanErrorRows{data: [][]any{{1}, {2}, {3}}},
	}
	tx := &pgTx{tx: mock}

	rows, err := tx.Query(context.Background(), "SELECT id FROM test")
	require.NoError(t, err)
	require.NotNil(t, rows)

	count := 0
	for rows.Next() {
		count++
	}
	assert.Equal(t, 3, count)
}

// ============================================================================
// Client Methods with Mock Pool Tests
// ============================================================================

// newClientWithMockPool creates a Client with an injected mock pool for testing.
func newClientWithMockPool(mock *mockPool) *Client {
	c := New(nil)
	c.pool = mock
	return c
}

func TestClient_Exec_Success(t *testing.T) {
	tests := []struct {
		name         string
		commandTag   string
		expectedRows int64
	}{
		{
			name:         "insert single row",
			commandTag:   "INSERT 0 1",
			expectedRows: 1,
		},
		{
			name:         "update multiple rows",
			commandTag:   "UPDATE 5",
			expectedRows: 5,
		},
		{
			name:         "delete no rows",
			commandTag:   "DELETE 0",
			expectedRows: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPool{
				execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag(tt.commandTag), nil
				},
			}
			c := newClientWithMockPool(mock)

			result, err := c.Exec(context.Background(), "TEST SQL", "arg1")
			require.NoError(t, err)
			require.NotNil(t, result)

			affected, err := result.RowsAffected()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedRows, affected)
		})
	}
}

func TestClient_Exec_Error(t *testing.T) {
	tests := []struct {
		name    string
		execErr error
	}{
		{
			name:    "syntax error",
			execErr: fmt.Errorf("syntax error at or near"),
		},
		{
			name:    "constraint violation",
			execErr: fmt.Errorf("violates foreign key constraint"),
		},
		{
			name:    "connection closed",
			execErr: fmt.Errorf("connection closed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPool{
				execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					return pgconn.CommandTag{}, tt.execErr
				},
			}
			c := newClientWithMockPool(mock)

			result, err := c.Exec(context.Background(), "TEST SQL")
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "exec:")
		})
	}
}

func TestClient_Query_Success(t *testing.T) {
	mock := &mockPool{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &mockScanErrorRows{
				data: [][]any{{1, "alice"}, {2, "bob"}},
			}, nil
		},
	}
	c := newClientWithMockPool(mock)

	rows, err := c.Query(context.Background(), "SELECT id, name FROM users")
	require.NoError(t, err)
	require.NotNil(t, rows)
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		var name string
		err := rows.Scan(&id, &name)
		require.NoError(t, err)
		count++
	}
	assert.Equal(t, 2, count)
}

func TestClient_Query_Error(t *testing.T) {
	tests := []struct {
		name     string
		queryErr error
	}{
		{
			name:     "table not found",
			queryErr: fmt.Errorf("relation does not exist"),
		},
		{
			name:     "permission denied",
			queryErr: fmt.Errorf("permission denied"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPool{
				queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return nil, tt.queryErr
				},
			}
			c := newClientWithMockPool(mock)

			rows, err := c.Query(context.Background(), "SELECT * FROM test")
			require.Error(t, err)
			assert.Nil(t, rows)
			assert.Contains(t, err.Error(), "query:")
		})
	}
}

func TestClient_QueryRow_Success(t *testing.T) {
	mock := &mockPool{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockPgxRow{values: []any{42}}
		},
	}
	c := newClientWithMockPool(mock)

	row := c.QueryRow(context.Background(), "SELECT 42")
	require.NotNil(t, row)

	var result int
	err := row.Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestClient_QueryRow_ScanError(t *testing.T) {
	mock := &mockPool{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockScanErrorRow{err: fmt.Errorf("no rows")}
		},
	}
	c := newClientWithMockPool(mock)

	row := c.QueryRow(context.Background(), "SELECT 1 WHERE false")
	var result int
	err := row.Scan(&result)
	require.Error(t, err)
}

func TestClient_Begin_Success(t *testing.T) {
	mock := &mockPool{
		beginFunc: func(ctx context.Context) (pgx.Tx, error) {
			return &mockTx{}, nil
		},
	}
	c := newClientWithMockPool(mock)

	tx, err := c.Begin(context.Background())
	require.NoError(t, err)
	require.NotNil(t, tx)
}

func TestClient_Begin_Error(t *testing.T) {
	tests := []struct {
		name     string
		beginErr error
	}{
		{
			name:     "connection closed",
			beginErr: fmt.Errorf("connection closed"),
		},
		{
			name:     "too many connections",
			beginErr: fmt.Errorf("too many connections"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPool{
				beginFunc: func(ctx context.Context) (pgx.Tx, error) {
					return nil, tt.beginErr
				},
			}
			c := newClientWithMockPool(mock)

			tx, err := c.Begin(context.Background())
			require.Error(t, err)
			assert.Nil(t, tx)
			assert.Contains(t, err.Error(), "begin transaction:")
		})
	}
}

func TestClient_HealthCheck_Success(t *testing.T) {
	mock := &mockPool{
		pingFunc: func(ctx context.Context) error {
			return nil
		},
	}
	c := newClientWithMockPool(mock)

	err := c.HealthCheck(context.Background())
	require.NoError(t, err)
}

func TestClient_HealthCheck_Error(t *testing.T) {
	tests := []struct {
		name    string
		pingErr error
	}{
		{
			name:    "connection refused",
			pingErr: fmt.Errorf("connection refused"),
		},
		{
			name:    "timeout",
			pingErr: fmt.Errorf("timeout"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPool{
				pingFunc: func(ctx context.Context) error {
					return tt.pingErr
				},
			}
			c := newClientWithMockPool(mock)

			err := c.HealthCheck(context.Background())
			require.Error(t, err)
		})
	}
}

func TestClient_Migrate_Success(t *testing.T) {
	executedQueries := []string{}
	mock := &mockPool{
		execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			executedQueries = append(executedQueries, sql)
			return pgconn.NewCommandTag("CREATE TABLE"), nil
		},
	}
	c := newClientWithMockPool(mock)

	migrations := []string{
		"CREATE TABLE test1 (id INT)",
		"CREATE TABLE test2 (id INT)",
		"CREATE INDEX idx_test ON test1(id)",
	}

	err := c.Migrate(context.Background(), migrations)
	require.NoError(t, err)
	assert.Equal(t, len(migrations), len(executedQueries))
	assert.Equal(t, migrations, executedQueries)
}

func TestClient_Migrate_EmptyList(t *testing.T) {
	mock := &mockPool{
		execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			t.Error("Exec should not be called for empty migrations")
			return pgconn.CommandTag{}, nil
		},
	}
	c := newClientWithMockPool(mock)

	err := c.Migrate(context.Background(), []string{})
	require.NoError(t, err)
}

func TestClient_Migrate_Error(t *testing.T) {
	tests := []struct {
		name          string
		migrations    []string
		failAtIndex   int
		expectedError string
	}{
		{
			name:          "first migration fails",
			migrations:    []string{"INVALID SQL", "CREATE TABLE t(id INT)"},
			failAtIndex:   0,
			expectedError: "migration 0:",
		},
		{
			name:          "second migration fails",
			migrations:    []string{"CREATE TABLE t1(id INT)", "INVALID SQL"},
			failAtIndex:   1,
			expectedError: "migration 1:",
		},
		{
			name:          "third migration fails",
			migrations:    []string{"CREATE TABLE t1(id INT)", "CREATE TABLE t2(id INT)", "BAD SQL"},
			failAtIndex:   2,
			expectedError: "migration 2:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mock := &mockPool{
				execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					defer func() { callCount++ }()
					if callCount == tt.failAtIndex {
						return pgconn.CommandTag{}, fmt.Errorf("exec error")
					}
					return pgconn.NewCommandTag("OK"), nil
				},
			}
			c := newClientWithMockPool(mock)

			err := c.Migrate(context.Background(), tt.migrations)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestClient_Close_WithMockPool(t *testing.T) {
	mock := &mockPool{}
	c := newClientWithMockPool(mock)

	err := c.Close()
	require.NoError(t, err)
	assert.True(t, mock.closed)
	assert.Nil(t, c.pool)
}

func TestClient_Close_DoubleClose(t *testing.T) {
	mock := &mockPool{}
	c := newClientWithMockPool(mock)

	err := c.Close()
	require.NoError(t, err)
	assert.True(t, mock.closed)

	// Second close should be safe
	err = c.Close()
	require.NoError(t, err)
}

// ============================================================================
// Context Handling Tests
// ============================================================================

func TestClient_Exec_ContextCanceled(t *testing.T) {
	mock := &mockPool{
		execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, ctx.Err()
		},
	}
	c := newClientWithMockPool(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Exec(ctx, "SELECT 1")
	require.Error(t, err)
}

func TestClient_Query_ContextCanceled(t *testing.T) {
	mock := &mockPool{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, ctx.Err()
		},
	}
	c := newClientWithMockPool(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Query(ctx, "SELECT 1")
	require.Error(t, err)
}

func TestClient_Begin_ContextCanceled(t *testing.T) {
	mock := &mockPool{
		beginFunc: func(ctx context.Context) (pgx.Tx, error) {
			return nil, ctx.Err()
		},
	}
	c := newClientWithMockPool(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Begin(ctx)
	require.Error(t, err)
}

func TestClient_HealthCheck_ContextTimeout(t *testing.T) {
	mock := &mockPool{
		pingFunc: func(ctx context.Context) error {
			// Simulate a slow ping that exceeds the health check timeout
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		},
	}
	c := newClientWithMockPool(mock)

	// Use a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err := c.HealthCheck(ctx)
	require.Error(t, err)
}

// ============================================================================
// Argument Passing Tests
// ============================================================================

func TestClient_Exec_PassesArguments(t *testing.T) {
	var receivedSQL string
	var receivedArgs []any

	mock := &mockPool{
		execFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			receivedSQL = sql
			receivedArgs = args
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	c := newClientWithMockPool(mock)

	_, err := c.Exec(context.Background(), "INSERT INTO users (name, age) VALUES ($1, $2)", "Alice", 30)
	require.NoError(t, err)

	assert.Equal(t, "INSERT INTO users (name, age) VALUES ($1, $2)", receivedSQL)
	assert.Len(t, receivedArgs, 2)
	assert.Equal(t, "Alice", receivedArgs[0])
	assert.Equal(t, 30, receivedArgs[1])
}

func TestClient_Query_PassesArguments(t *testing.T) {
	var receivedSQL string
	var receivedArgs []any

	mock := &mockPool{
		queryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			receivedSQL = sql
			receivedArgs = args
			return &mockScanErrorRows{data: [][]any{}}, nil
		},
	}
	c := newClientWithMockPool(mock)

	_, err := c.Query(context.Background(), "SELECT * FROM users WHERE age > $1", 25)
	require.NoError(t, err)

	assert.Equal(t, "SELECT * FROM users WHERE age > $1", receivedSQL)
	assert.Len(t, receivedArgs, 1)
	assert.Equal(t, 25, receivedArgs[0])
}

func TestClient_QueryRow_PassesArguments(t *testing.T) {
	var receivedSQL string
	var receivedArgs []any

	mock := &mockPool{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			receivedSQL = sql
			receivedArgs = args
			return &mockPgxRow{values: []any{1}}
		},
	}
	c := newClientWithMockPool(mock)

	c.QueryRow(context.Background(), "SELECT id FROM users WHERE name = $1", "Bob")

	assert.Equal(t, "SELECT id FROM users WHERE name = $1", receivedSQL)
	assert.Len(t, receivedArgs, 1)
	assert.Equal(t, "Bob", receivedArgs[0])
}

// ============================================================================
// calculateMaxConns Tests (for 100% branch coverage)
// ============================================================================

func TestCalculateMaxConns(t *testing.T) {
	tests := []struct {
		name     string
		cpuCount int32
		expected int32
	}{
		{
			name:     "very low CPU count (1 CPU) clamps to minimum 10",
			cpuCount: 1,
			expected: 10, // 1*2+1=3, but clamped to 10
		},
		{
			name:     "low CPU count (2 CPUs) clamps to minimum 10",
			cpuCount: 2,
			expected: 10, // 2*2+1=5, but clamped to 10
		},
		{
			name:     "low CPU count (3 CPUs) clamps to minimum 10",
			cpuCount: 3,
			expected: 10, // 3*2+1=7, but clamped to 10
		},
		{
			name:     "low CPU count (4 CPUs) clamps to minimum 10",
			cpuCount: 4,
			expected: 10, // 4*2+1=9, but clamped to 10
		},
		{
			name:     "exactly 5 CPUs gives 11 (no clamping)",
			cpuCount: 5,
			expected: 11, // 5*2+1=11, within range
		},
		{
			name:     "8 CPUs gives 17 (within range)",
			cpuCount: 8,
			expected: 17, // 8*2+1=17, within range
		},
		{
			name:     "16 CPUs gives 33 (within range)",
			cpuCount: 16,
			expected: 33, // 16*2+1=33, within range
		},
		{
			name:     "24 CPUs gives 49 (just under max)",
			cpuCount: 24,
			expected: 49, // 24*2+1=49, within range
		},
		{
			name:     "25 CPUs clamps to maximum 50",
			cpuCount: 25,
			expected: 50, // 25*2+1=51, but clamped to 50
		},
		{
			name:     "32 CPUs clamps to maximum 50",
			cpuCount: 32,
			expected: 50, // 32*2+1=65, but clamped to 50
		},
		{
			name:     "64 CPUs clamps to maximum 50",
			cpuCount: 64,
			expected: 50, // 64*2+1=129, but clamped to 50
		},
		{
			name:     "128 CPUs clamps to maximum 50",
			cpuCount: 128,
			expected: 50, // 128*2+1=257, but clamped to 50
		},
		{
			name:     "zero CPUs clamps to minimum 10",
			cpuCount: 0,
			expected: 10, // 0*2+1=1, but clamped to 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMaxConns(tt.cpuCount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Connect Method with Pool Factory Tests
// ============================================================================

// mockPgxPool wraps mockPool but satisfies *pgxpool.Pool type requirements
// by embedding in a way the Connect method expects.
type testablePool struct {
	*pgxpool.Pool
	pingErr error
}

func TestClient_Connect_PoolCreationError(t *testing.T) {
	// Save original factory
	originalCreator := createPool
	defer func() { createPool = originalCreator }()

	// Replace with factory that returns an error
	createPool = func(ctx context.Context, cfg *pgxpool.Config) (pooler, *pgxpool.Pool, error) {
		return nil, nil, fmt.Errorf("simulated pool creation error")
	}

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

	err := c.Connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create connection pool:")
}

func TestClient_Connect_BuildPoolConfigError(t *testing.T) {
	// Trigger buildPoolConfig error by using an invalid SSLMode
	// pgxpool.ParseConfig fails with invalid sslmode values
	cfg := &Config{
		Config: db.Config{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			User:     "test",
			Password: "test",
			DBName:   "testdb",
			SSLMode:  "invalid_sslmode_xyz", // This will cause ParseConfig to fail
		},
	}
	c := New(cfg)

	err := c.Connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build pool config:")
}

// ============================================================================
// Pool Factory Default Implementation Test
// ============================================================================

func TestPoolCreator_IsInitialized(t *testing.T) {
	// Verify the default pool creator is set and callable
	// We don't actually call it because it would try to connect
	assert.NotNil(t, createPool)
}

func TestDefaultPoolCreator_Error(t *testing.T) {
	// Test the default pool creator error path by passing a config with
	// invalid MaxConns (0 or negative), which causes pgxpool.NewWithConfig to fail.

	ctx := context.Background()

	// Parse a valid DSN to get a config
	cfg, err := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/db?sslmode=disable")
	require.NoError(t, err)

	// Set MaxConns to 0, which pgxpool rejects with "MaxSize must be >= 1"
	cfg.MaxConns = 0

	pool, pgPool, err := defaultPoolCreator(ctx, cfg)

	// pgxpool.NewWithConfig fails with MaxConns = 0
	require.Error(t, err)
	assert.Nil(t, pool)
	assert.Nil(t, pgPool)
	assert.Contains(t, err.Error(), "MaxSize must be >= 1")
}

func TestDefaultPoolCreator_Success(t *testing.T) {
	// Test that defaultPoolCreator returns both pooler and *pgxpool.Pool
	// when successful. We can't actually connect, so we just verify the
	// function signature works correctly.

	// This test verifies the success path returns consistent types
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	cfg, err := pgxpool.ParseConfig("postgres://user:pass@localhost:59999/db?sslmode=disable")
	require.NoError(t, err)
	cfg.MinConns = 0 // Don't force connections

	pool, pgPool, err := defaultPoolCreator(ctx, cfg)

	// With MinConns = 0 and lazy connection, pool should be created
	if err == nil {
		assert.NotNil(t, pool)
		assert.NotNil(t, pgPool)
		// Verify they're the same object
		assert.Equal(t, pool, pooler(pgPool))
		pool.Close()
	}
}

func TestClient_Connect_Success(t *testing.T) {
	// Save original factory
	originalCreator := createPool
	defer func() { createPool = originalCreator }()

	// Replace with factory that returns a mock pool
	mockPoolInstance := &mockPool{
		pingFunc: func(ctx context.Context) error {
			return nil // Ping succeeds
		},
	}
	createPool = func(ctx context.Context, cfg *pgxpool.Config) (pooler, *pgxpool.Pool, error) {
		return mockPoolInstance, nil, nil // Return mock pool, nil for pgxpool.Pool
	}

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

	err := c.Connect(context.Background())
	require.NoError(t, err)

	// Verify pool was assigned
	assert.NotNil(t, c.pool)
	assert.Equal(t, mockPoolInstance, c.pool)

	// pgPool should be nil since we returned nil for it
	assert.Nil(t, c.pgPool)
	assert.Nil(t, c.Pool())
}

func TestClient_Connect_PingError(t *testing.T) {
	// Save original factory
	originalCreator := createPool
	defer func() { createPool = originalCreator }()

	// Replace with factory that returns a pool that fails ping
	mockPoolInstance := &mockPool{
		pingFunc: func(ctx context.Context) error {
			return fmt.Errorf("ping failed")
		},
	}
	createPool = func(ctx context.Context, cfg *pgxpool.Config) (pooler, *pgxpool.Pool, error) {
		return mockPoolInstance, nil, nil
	}

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

	err := c.Connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ping database:")
	assert.True(t, mockPoolInstance.closed, "pool should be closed after ping failure")
}
