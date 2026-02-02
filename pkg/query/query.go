// Package query provides a fluent SQL query builder with type-safe conditions.
package query

import (
	"fmt"
	"strings"
)

// Builder constructs SQL SELECT queries fluently.
type Builder struct {
	selectCols []string
	from       string
	conditions []Condition
	orderBy    string
	limit      int
	offset     int
	groupBy    string
	having     []Condition
}

// New creates a new query Builder.
func New() *Builder {
	return &Builder{}
}

// Select sets the columns to select. Pass "*" for all columns.
func (b *Builder) Select(cols ...string) *Builder {
	b.selectCols = cols
	return b
}

// From sets the table name.
func (b *Builder) From(table string) *Builder {
	b.from = table
	return b
}

// Where adds a WHERE condition.
func (b *Builder) Where(c Condition) *Builder {
	b.conditions = append(b.conditions, c)
	return b
}

// OrderBy sets the ORDER BY clause (e.g. "created_at DESC").
func (b *Builder) OrderBy(expr string) *Builder {
	b.orderBy = expr
	return b
}

// Limit sets the LIMIT clause.
func (b *Builder) Limit(n int) *Builder {
	b.limit = n
	return b
}

// Offset sets the OFFSET clause.
func (b *Builder) Offset(n int) *Builder {
	b.offset = n
	return b
}

// GroupBy sets the GROUP BY clause.
func (b *Builder) GroupBy(expr string) *Builder {
	b.groupBy = expr
	return b
}

// Having adds a HAVING condition.
func (b *Builder) Having(c Condition) *Builder {
	b.having = append(b.having, c)
	return b
}

// Build assembles the SQL query string and positional arguments.
func (b *Builder) Build() (string, []any) {
	var sb strings.Builder
	var args []any

	// SELECT.
	sb.WriteString("SELECT ")
	if len(b.selectCols) == 0 {
		sb.WriteString("*")
	} else {
		sb.WriteString(strings.Join(b.selectCols, ", "))
	}

	// FROM.
	if b.from != "" {
		sb.WriteString(" FROM ")
		sb.WriteString(b.from)
	}

	// WHERE.
	if len(b.conditions) > 0 {
		sb.WriteString(" WHERE ")
		for i, c := range b.conditions {
			if i > 0 {
				sb.WriteString(" AND ")
			}
			expr, cArgs := c.Build()
			sb.WriteString(expr)
			args = append(args, cArgs...)
		}
	}

	// GROUP BY.
	if b.groupBy != "" {
		sb.WriteString(" GROUP BY ")
		sb.WriteString(b.groupBy)
	}

	// HAVING.
	if len(b.having) > 0 {
		sb.WriteString(" HAVING ")
		for i, c := range b.having {
			if i > 0 {
				sb.WriteString(" AND ")
			}
			expr, cArgs := c.Build()
			sb.WriteString(expr)
			args = append(args, cArgs...)
		}
	}

	// ORDER BY.
	if b.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(b.orderBy)
	}

	// LIMIT.
	if b.limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", b.limit))
	}

	// OFFSET.
	if b.offset > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", b.offset))
	}

	return sb.String(), args
}

// Condition represents a WHERE or HAVING clause element.
type Condition interface {
	// Build returns the SQL fragment and arguments for this condition.
	Build() (string, []any)
}

// Eq creates a column = value condition.
func Eq(column string, value any) Condition {
	return &simpleCondition{column: column, op: "=", value: value}
}

// Neq creates a column != value condition.
func Neq(column string, value any) Condition {
	return &simpleCondition{column: column, op: "!=", value: value}
}

// Gt creates a column > value condition.
func Gt(column string, value any) Condition {
	return &simpleCondition{column: column, op: ">", value: value}
}

// Gte creates a column >= value condition.
func Gte(column string, value any) Condition {
	return &simpleCondition{column: column, op: ">=", value: value}
}

// Lt creates a column < value condition.
func Lt(column string, value any) Condition {
	return &simpleCondition{column: column, op: "<", value: value}
}

// Lte creates a column <= value condition.
func Lte(column string, value any) Condition {
	return &simpleCondition{column: column, op: "<=", value: value}
}

// Like creates a column LIKE pattern condition.
func Like(column string, pattern string) Condition {
	return &simpleCondition{column: column, op: "LIKE", value: pattern}
}

// IsNull creates a column IS NULL condition.
func IsNull(column string) Condition {
	return &nullCondition{column: column, isNull: true}
}

// IsNotNull creates a column IS NOT NULL condition.
func IsNotNull(column string) Condition {
	return &nullCondition{column: column, isNull: false}
}

// In creates a column IN (values...) condition.
func In(column string, values ...any) Condition {
	return &inCondition{column: column, values: values}
}

// And combines multiple conditions with AND.
func And(conditions ...Condition) Condition {
	return &compositeCondition{op: "AND", conditions: conditions}
}

// Or combines multiple conditions with OR.
func Or(conditions ...Condition) Condition {
	return &compositeCondition{op: "OR", conditions: conditions}
}

// simpleCondition handles binary comparison operators.
type simpleCondition struct {
	column string
	op     string
	value  any
}

func (c *simpleCondition) Build() (string, []any) {
	return fmt.Sprintf("%s %s ?", c.column, c.op), []any{c.value}
}

// nullCondition handles IS NULL / IS NOT NULL.
type nullCondition struct {
	column string
	isNull bool
}

func (c *nullCondition) Build() (string, []any) {
	if c.isNull {
		return fmt.Sprintf("%s IS NULL", c.column), nil
	}
	return fmt.Sprintf("%s IS NOT NULL", c.column), nil
}

// inCondition handles IN clauses.
type inCondition struct {
	column string
	values []any
}

func (c *inCondition) Build() (string, []any) {
	if len(c.values) == 0 {
		return "1 = 0", nil // Always false for empty IN.
	}
	placeholders := make([]string, len(c.values))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	expr := fmt.Sprintf("%s IN (%s)", c.column, strings.Join(placeholders, ", "))
	return expr, c.values
}

// compositeCondition combines conditions with AND or OR.
type compositeCondition struct {
	op         string
	conditions []Condition
}

func (c *compositeCondition) Build() (string, []any) {
	if len(c.conditions) == 0 {
		return "1 = 1", nil
	}
	if len(c.conditions) == 1 {
		return c.conditions[0].Build()
	}

	var parts []string
	var args []any
	for _, cond := range c.conditions {
		expr, cArgs := cond.Build()
		parts = append(parts, expr)
		args = append(args, cArgs...)
	}
	return "(" + strings.Join(parts, " "+c.op+" ") + ")", args
}
