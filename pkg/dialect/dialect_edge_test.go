package dialect_test

import (
	"testing"

	"digital.vasic.database/pkg/dialect"
	"github.com/stretchr/testify/assert"
)

// --- Empty SQL ---

func TestRewritePlaceholders_EmptySQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dt      dialect.Type
		input   string
		expect  string
	}{
		{"sqlite_empty", dialect.SQLite, "", ""},
		{"postgres_empty", dialect.Postgres, "", ""},
		{"sqlite_whitespace", dialect.SQLite, "   ", "   "},
		{"postgres_whitespace", dialect.Postgres, "   ", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := dialect.New(tt.dt)
			assert.Equal(t, tt.expect, d.RewritePlaceholders(tt.input))
		})
	}
}

// --- SQL With No Placeholders ---

func TestRewritePlaceholders_NoPlaceholders(t *testing.T) {
	t.Parallel()

	d := dialect.New(dialect.Postgres)
	tests := []struct {
		name  string
		input string
	}{
		{"simple_select", "SELECT 1"},
		{"select_with_join", "SELECT a.id FROM a JOIN b ON a.id = b.a_id"},
		{"delete_no_where", "DELETE FROM t"},
		{"create_table", "CREATE TABLE t (id INT PRIMARY KEY)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := d.RewritePlaceholders(tt.input)
			assert.Equal(t, tt.input, result)
		})
	}
}

// --- Mixed Placeholder Styles ---

func TestRewritePlaceholders_MixedStyles(t *testing.T) {
	t.Parallel()

	d := dialect.New(dialect.Postgres)
	// If the query already has $1-style placeholders mixed with ?,
	// the rewriter should still rewrite the ? placeholders.
	input := "SELECT * FROM t WHERE id = $1 AND name = ?"
	result := d.RewritePlaceholders(input)
	// ? should become $1, making it $1 ... $1 (both = $1 since it's the
	// first ? placeholder encountered).
	assert.Equal(t, "SELECT * FROM t WHERE id = $1 AND name = $1", result)
}

// --- Many Placeholders ---

func TestRewritePlaceholders_ManyPlaceholders(t *testing.T) {
	t.Parallel()

	d := dialect.New(dialect.Postgres)
	// 100 placeholders
	input := "INSERT INTO t VALUES (?" +
		func() string {
			s := ""
			for i := 1; i < 100; i++ {
				s += ", ?"
			}
			return s
		}() + ")"

	result := d.RewritePlaceholders(input)
	assert.Contains(t, result, "$1")
	assert.Contains(t, result, "$100")
	assert.NotContains(t, result, "?")
}

// --- Placeholders Inside Quotes ---

func TestRewritePlaceholders_QuotedQuestionMarks(t *testing.T) {
	t.Parallel()

	d := dialect.New(dialect.Postgres)
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			"single_quoted_question",
			"SELECT * FROM t WHERE name = '?' AND id = ?",
			"SELECT * FROM t WHERE name = '?' AND id = $1",
		},
		{
			"multiple_quoted_segments",
			"SELECT '?' as q1, '?' as q2, id = ?",
			"SELECT '?' as q1, '?' as q2, id = $1",
		},
		{
			"empty_quotes",
			"SELECT * FROM t WHERE name = '' AND id = ?",
			"SELECT * FROM t WHERE name = '' AND id = $1",
		},
		{
			"only_quoted_question",
			"SELECT 'is this a question?'",
			"SELECT 'is this a question?'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expect, d.RewritePlaceholders(tt.input))
		})
	}
}

// --- RewriteInsertOrIgnore Edge Cases ---

func TestRewriteInsertOrIgnore_EdgeCases(t *testing.T) {
	t.Parallel()

	d := dialect.New(dialect.Postgres)
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			"lowercase_insert_or_ignore",
			"insert or ignore into t (a) values (?)",
			"INSERT INTO t (a) values (?) ON CONFLICT DO NOTHING",
		},
		{
			"mixed_case",
			"Insert Or Ignore Into t (a) VALUES (?)",
			"INSERT INTO t (a) VALUES (?) ON CONFLICT DO NOTHING",
		},
		{
			"empty_string",
			"",
			"",
		},
		{
			"insert_without_ignore",
			"INSERT INTO t (a) VALUES (?)",
			"INSERT INTO t (a) VALUES (?)",
		},
		{
			"update_statement",
			"UPDATE t SET a = ? WHERE id = ?",
			"UPDATE t SET a = ? WHERE id = ?",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expect, d.RewriteInsertOrIgnore(tt.input))
		})
	}
}

// --- RewriteInsertOrReplace Edge Cases ---

func TestRewriteInsertOrReplace_EdgeCases(t *testing.T) {
	t.Parallel()

	d := dialect.New(dialect.Postgres)
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			"empty_string",
			"",
			"",
		},
		{
			"no_match",
			"SELECT 1",
			"SELECT 1",
		},
		{
			"lowercase_or_replace",
			"insert or replace into t (a) values (?)",
			"INSERT INTO t (a) values (?)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := d.RewriteInsertOrReplace(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// --- RewriteBooleanLiterals Edge Cases ---

func TestRewriteBooleanLiterals_EdgeCases(t *testing.T) {
	t.Parallel()

	d := dialect.New(dialect.Postgres)

	tests := []struct {
		name    string
		query   string
		columns []string
		expect  string
	}{
		{
			"nil_columns",
			"SELECT * FROM t WHERE active = 1",
			nil,
			"SELECT * FROM t WHERE active = 1",
		},
		{
			"empty_columns",
			"SELECT * FROM t WHERE active = 1",
			[]string{},
			"SELECT * FROM t WHERE active = 1",
		},
		{
			"column_not_in_query",
			"SELECT * FROM t WHERE name = 'test'",
			[]string{"active"},
			"SELECT * FROM t WHERE name = 'test'",
		},
		{
			"value_not_0_or_1",
			"SELECT * FROM t WHERE active = 2",
			[]string{"active"},
			"SELECT * FROM t WHERE active = 2",
		},
		{
			"empty_query",
			"",
			[]string{"active"},
			"",
		},
		{
			"column_appears_in_value",
			"SELECT * FROM t WHERE description = 'active = 1'",
			[]string{"active"},
			// The regex might match inside a string literal -- documenting behavior
			"SELECT * FROM t WHERE description = 'active = TRUE'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := d.RewriteBooleanLiterals(tt.query, tt.columns)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// --- RewriteAll Combined Edge Cases ---

func TestRewriteAll_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dt      dialect.Type
		query   string
		columns []string
	}{
		{
			"sqlite_passthrough",
			dialect.SQLite,
			"INSERT OR IGNORE INTO t (active) VALUES (?) WHERE active = 1",
			[]string{"active"},
		},
		{
			"empty_everything",
			dialect.Postgres,
			"",
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := dialect.New(tt.dt)
			// Should not panic on any input
			result := d.RewriteAll(tt.query, tt.columns)
			_ = result
		})
	}
}

// --- Unknown Dialect Type ---

func TestDialect_UnknownType(t *testing.T) {
	t.Parallel()

	d := dialect.New(dialect.Type("mysql"))

	// Unknown dialect should behave like SQLite (passthrough)
	assert.False(t, d.IsSQLite())
	assert.False(t, d.IsPostgres())

	query := "SELECT * FROM t WHERE id = ? AND active = 1"
	assert.Equal(t, query, d.RewritePlaceholders(query))
	assert.Equal(t, query, d.RewriteBooleanLiterals(query, []string{"active"}))
}

// --- AutoIncrement / TimestampType for unknown dialect ---

func TestDialect_DDL_Defaults(t *testing.T) {
	t.Parallel()

	d := dialect.New(dialect.Type("unknown"))

	// Non-postgres types should return SQLite-style defaults
	assert.Equal(t, "INTEGER PRIMARY KEY AUTOINCREMENT", d.AutoIncrement())
	assert.Equal(t, "DATETIME", d.TimestampType())
	assert.Equal(t, "DEFAULT 1", d.BooleanDefault(true))
	assert.Equal(t, "DEFAULT 0", d.BooleanDefault(false))
	assert.Equal(t, "CURRENT_TIMESTAMP", d.CurrentTimestamp())
}
