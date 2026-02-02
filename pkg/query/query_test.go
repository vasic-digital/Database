package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuilder_Build(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *Builder
		wantSQL  string
		wantArgs []any
	}{
		{
			name: "simple select all",
			build: func() *Builder {
				return New().From("users")
			},
			wantSQL:  "SELECT * FROM users",
			wantArgs: nil,
		},
		{
			name: "select specific columns",
			build: func() *Builder {
				return New().Select("id", "name", "email").From("users")
			},
			wantSQL:  "SELECT id, name, email FROM users",
			wantArgs: nil,
		},
		{
			name: "with where clause",
			build: func() *Builder {
				return New().Select("id", "name").From("users").
					Where(Eq("status", "active"))
			},
			wantSQL:  "SELECT id, name FROM users WHERE status = ?",
			wantArgs: []any{"active"},
		},
		{
			name: "with multiple where clauses",
			build: func() *Builder {
				return New().Select("*").From("users").
					Where(Eq("status", "active")).
					Where(Gt("age", 18))
			},
			wantSQL:  "SELECT * FROM users WHERE status = ? AND age > ?",
			wantArgs: []any{"active", 18},
		},
		{
			name: "with order by",
			build: func() *Builder {
				return New().Select("*").From("users").
					OrderBy("created_at DESC")
			},
			wantSQL:  "SELECT * FROM users ORDER BY created_at DESC",
			wantArgs: nil,
		},
		{
			name: "with limit",
			build: func() *Builder {
				return New().Select("*").From("users").Limit(10)
			},
			wantSQL:  "SELECT * FROM users LIMIT 10",
			wantArgs: nil,
		},
		{
			name: "with offset",
			build: func() *Builder {
				return New().Select("*").From("users").Offset(20)
			},
			wantSQL:  "SELECT * FROM users OFFSET 20",
			wantArgs: nil,
		},
		{
			name: "full query",
			build: func() *Builder {
				return New().
					Select("id", "name").
					From("users").
					Where(Eq("active", true)).
					Where(Gte("age", 18)).
					OrderBy("name ASC").
					Limit(10).
					Offset(0)
			},
			wantSQL:  "SELECT id, name FROM users WHERE active = ? AND age >= ? ORDER BY name ASC LIMIT 10",
			wantArgs: []any{true, 18},
		},
		{
			name: "with group by and having",
			build: func() *Builder {
				return New().
					Select("department", "COUNT(*) as cnt").
					From("employees").
					GroupBy("department").
					Having(Gt("COUNT(*)", 5))
			},
			wantSQL:  "SELECT department, COUNT(*) as cnt FROM employees GROUP BY department HAVING COUNT(*) > ?",
			wantArgs: []any{5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.build().Build()
			assert.Equal(t, tt.wantSQL, sql)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestConditions(t *testing.T) {
	tests := []struct {
		name     string
		cond     Condition
		wantSQL  string
		wantArgs []any
	}{
		{
			name:     "Eq",
			cond:     Eq("name", "alice"),
			wantSQL:  "name = ?",
			wantArgs: []any{"alice"},
		},
		{
			name:     "Neq",
			cond:     Neq("status", "deleted"),
			wantSQL:  "status != ?",
			wantArgs: []any{"deleted"},
		},
		{
			name:     "Gt",
			cond:     Gt("age", 21),
			wantSQL:  "age > ?",
			wantArgs: []any{21},
		},
		{
			name:     "Gte",
			cond:     Gte("score", 90.5),
			wantSQL:  "score >= ?",
			wantArgs: []any{90.5},
		},
		{
			name:     "Lt",
			cond:     Lt("price", 100),
			wantSQL:  "price < ?",
			wantArgs: []any{100},
		},
		{
			name:     "Lte",
			cond:     Lte("quantity", 0),
			wantSQL:  "quantity <= ?",
			wantArgs: []any{0},
		},
		{
			name:     "Like",
			cond:     Like("name", "%alice%"),
			wantSQL:  "name LIKE ?",
			wantArgs: []any{"%alice%"},
		},
		{
			name:     "IsNull",
			cond:     IsNull("deleted_at"),
			wantSQL:  "deleted_at IS NULL",
			wantArgs: nil,
		},
		{
			name:     "IsNotNull",
			cond:     IsNotNull("email"),
			wantSQL:  "email IS NOT NULL",
			wantArgs: nil,
		},
		{
			name:     "In with values",
			cond:     In("id", 1, 2, 3),
			wantSQL:  "id IN (?, ?, ?)",
			wantArgs: []any{1, 2, 3},
		},
		{
			name:     "In empty",
			cond:     In("id"),
			wantSQL:  "1 = 0",
			wantArgs: nil,
		},
		{
			name: "And composite",
			cond: And(Eq("a", 1), Eq("b", 2)),
			wantSQL:  "(a = ? AND b = ?)",
			wantArgs: []any{1, 2},
		},
		{
			name: "Or composite",
			cond: Or(Eq("status", "active"), Eq("status", "pending")),
			wantSQL:  "(status = ? OR status = ?)",
			wantArgs: []any{"active", "pending"},
		},
		{
			name:     "And single",
			cond:     And(Eq("x", 1)),
			wantSQL:  "x = ?",
			wantArgs: []any{1},
		},
		{
			name:     "And empty",
			cond:     And(),
			wantSQL:  "1 = 1",
			wantArgs: nil,
		},
		{
			name: "nested composite",
			cond: And(
				Eq("active", true),
				Or(Gt("age", 18), Eq("role", "admin")),
			),
			wantSQL:  "(active = ? AND (age > ? OR role = ?))",
			wantArgs: []any{true, 18, "admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.cond.Build()
			assert.Equal(t, tt.wantSQL, sql)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestBuilder_Chaining(t *testing.T) {
	t.Run("builder is reusable after build", func(t *testing.T) {
		b := New().Select("id").From("users").Where(Eq("active", true))

		sql1, args1 := b.Build()
		sql2, args2 := b.Build()

		assert.Equal(t, sql1, sql2)
		assert.Equal(t, args1, args2)
	})

	t.Run("builder methods return same instance", func(t *testing.T) {
		b := New()
		b2 := b.Select("id").From("users").Where(Eq("x", 1)).
			OrderBy("id").Limit(10).Offset(5).GroupBy("x").
			Having(Gt("COUNT(*)", 1))

		assert.Same(t, b, b2)
	})
}
