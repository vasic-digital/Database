package dialect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	d := New(SQLite)
	assert.Equal(t, SQLite, d.Type)

	d = New(Postgres)
	assert.Equal(t, Postgres, d.Type)
}

func TestRewritePlaceholders_SQLite(t *testing.T) {
	d := New(SQLite)
	q := "SELECT * FROM t WHERE id = ? AND name = ?"
	assert.Equal(t, q, d.RewritePlaceholders(q))
}

func TestRewritePlaceholders_Postgres(t *testing.T) {
	d := New(Postgres)
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "single placeholder",
			input:  "SELECT * FROM t WHERE id = ?",
			expect: "SELECT * FROM t WHERE id = $1",
		},
		{
			name:   "multiple placeholders",
			input:  "SELECT * FROM t WHERE id = ? AND name = ? AND age > ?",
			expect: "SELECT * FROM t WHERE id = $1 AND name = $2 AND age > $3",
		},
		{
			name:   "quoted question mark preserved",
			input:  "SELECT * FROM t WHERE name = 'what?' AND id = ?",
			expect: "SELECT * FROM t WHERE name = 'what?' AND id = $1",
		},
		{
			name:   "no placeholders",
			input:  "SELECT * FROM t",
			expect: "SELECT * FROM t",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, d.RewritePlaceholders(tt.input))
		})
	}
}

func TestRewriteInsertOrIgnore(t *testing.T) {
	tests := []struct {
		name    string
		dialect Type
		input   string
		expect  string
	}{
		{
			name:    "SQLite unchanged",
			dialect: SQLite,
			input:   "INSERT OR IGNORE INTO t (a) VALUES (?)",
			expect:  "INSERT OR IGNORE INTO t (a) VALUES (?)",
		},
		{
			name:    "Postgres rewritten",
			dialect: Postgres,
			input:   "INSERT OR IGNORE INTO t (a) VALUES (?)",
			expect:  "INSERT INTO t (a) VALUES (?) ON CONFLICT DO NOTHING",
		},
		{
			name:    "no match",
			dialect: Postgres,
			input:   "INSERT INTO t (a) VALUES (?)",
			expect:  "INSERT INTO t (a) VALUES (?)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(tt.dialect)
			assert.Equal(t, tt.expect, d.RewriteInsertOrIgnore(tt.input))
		})
	}
}

func TestRewriteInsertOrReplace(t *testing.T) {
	tests := []struct {
		name    string
		dialect Type
		input   string
		expect  string
	}{
		{
			name:    "SQLite unchanged",
			dialect: SQLite,
			input:   "INSERT OR REPLACE INTO t (a) VALUES (?)",
			expect:  "INSERT OR REPLACE INTO t (a) VALUES (?)",
		},
		{
			name:    "Postgres rewritten",
			dialect: Postgres,
			input:   "INSERT OR REPLACE INTO t (a) VALUES (?)",
			expect:  "INSERT INTO t (a) VALUES (?)",
		},
		{
			name:    "no match",
			dialect: Postgres,
			input:   "UPDATE t SET a = ?",
			expect:  "UPDATE t SET a = ?",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(tt.dialect)
			assert.Equal(t, tt.expect, d.RewriteInsertOrReplace(tt.input))
		})
	}
}

func TestAutoIncrement(t *testing.T) {
	assert.Equal(t, "INTEGER PRIMARY KEY AUTOINCREMENT", New(SQLite).AutoIncrement())
	assert.Equal(t, "SERIAL PRIMARY KEY", New(Postgres).AutoIncrement())
}

func TestTimestampType(t *testing.T) {
	assert.Equal(t, "DATETIME", New(SQLite).TimestampType())
	assert.Equal(t, "TIMESTAMP", New(Postgres).TimestampType())
}

func TestBooleanDefault(t *testing.T) {
	assert.Equal(t, "DEFAULT 1", New(SQLite).BooleanDefault(true))
	assert.Equal(t, "DEFAULT 0", New(SQLite).BooleanDefault(false))
	assert.Equal(t, "DEFAULT TRUE", New(Postgres).BooleanDefault(true))
	assert.Equal(t, "DEFAULT FALSE", New(Postgres).BooleanDefault(false))
}

func TestCurrentTimestamp(t *testing.T) {
	assert.Equal(t, "CURRENT_TIMESTAMP", New(SQLite).CurrentTimestamp())
	assert.Equal(t, "CURRENT_TIMESTAMP", New(Postgres).CurrentTimestamp())
}

func TestIsSQLite_IsPostgres(t *testing.T) {
	s := New(SQLite)
	assert.True(t, s.IsSQLite())
	assert.False(t, s.IsPostgres())

	p := New(Postgres)
	assert.True(t, p.IsPostgres())
	assert.False(t, p.IsSQLite())
}

func TestRewriteBooleanLiterals(t *testing.T) {
	d := New(Postgres)
	cols := []string{"is_active", "deleted", "enabled"}

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "true rewrite",
			input:  "SELECT * FROM t WHERE is_active = 1",
			expect: "SELECT * FROM t WHERE is_active = TRUE",
		},
		{
			name:   "false rewrite",
			input:  "SELECT * FROM t WHERE deleted = 0",
			expect: "SELECT * FROM t WHERE deleted = FALSE",
		},
		{
			name:   "multiple columns",
			input:  "SELECT * FROM t WHERE is_active = 1 AND deleted = 0",
			expect: "SELECT * FROM t WHERE is_active = TRUE AND deleted = FALSE",
		},
		{
			name:   "non-boolean column unchanged",
			input:  "SELECT * FROM t WHERE count = 0",
			expect: "SELECT * FROM t WHERE count = 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, d.RewriteBooleanLiterals(tt.input, cols))
		})
	}
}

func TestRewriteBooleanLiterals_SQLite(t *testing.T) {
	d := New(SQLite)
	q := "SELECT * FROM t WHERE is_active = 1"
	assert.Equal(t, q, d.RewriteBooleanLiterals(q, []string{"is_active"}))
}

func TestRewriteBooleanLiterals_EmptyColumns(t *testing.T) {
	d := New(Postgres)
	q := "SELECT * FROM t WHERE is_active = 1"
	assert.Equal(t, q, d.RewriteBooleanLiterals(q, nil))
}

func TestRewriteAll(t *testing.T) {
	d := New(Postgres)
	cols := []string{"is_active"}

	input := "INSERT OR IGNORE INTO t (is_active) VALUES (?) WHERE is_active = 1"
	result := d.RewriteAll(input, cols)

	assert.Contains(t, result, "$1")
	assert.Contains(t, result, "ON CONFLICT DO NOTHING")
	assert.Contains(t, result, "is_active = TRUE")
	assert.NotContains(t, result, "INSERT OR IGNORE")
}

func TestRewriteAll_SQLite(t *testing.T) {
	d := New(SQLite)
	input := "INSERT OR IGNORE INTO t (a) VALUES (?)"
	assert.Equal(t, input, d.RewriteAll(input, nil))
}
