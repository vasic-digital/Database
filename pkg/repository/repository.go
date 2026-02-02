// Package repository provides a generic repository pattern for CRUD operations
// on database entities.
package repository

import (
	"context"
	"fmt"
	"strings"

	db "digital.vasic.database/pkg/database"
)

// Repository defines generic CRUD operations for an entity type T.
type Repository[T any] interface {
	// Create inserts a new entity.
	Create(ctx context.Context, entity *T) error

	// GetByID retrieves an entity by its primary key.
	GetByID(ctx context.Context, id any) (*T, error)

	// Update modifies an existing entity.
	Update(ctx context.Context, entity *T) error

	// Delete removes an entity by its primary key.
	Delete(ctx context.Context, id any) error

	// List returns entities matching the given options.
	List(ctx context.Context, opts ListOptions) ([]T, error)

	// Count returns the number of entities matching the filter.
	Count(ctx context.Context, opts ListOptions) (int64, error)
}

// ListOptions configures listing/pagination behaviour.
type ListOptions struct {
	// Offset is the number of rows to skip.
	Offset int

	// Limit is the maximum number of rows to return. Zero means no limit.
	Limit int

	// OrderBy is the column to sort by (e.g. "created_at DESC").
	OrderBy string

	// Where is a list of conditions to filter by. Each entry is a
	// WhereClause with a parameterised expression and arguments.
	Where []WhereClause
}

// WhereClause represents a single WHERE condition.
type WhereClause struct {
	// Expr is the SQL expression with parameter placeholders (e.g.
	// "status = ?").
	Expr string

	// Args are the values for the placeholders.
	Args []any
}

// BuildWhereSQL assembles all WhereClause entries into a WHERE fragment and
// a flat args slice. Returns empty string if no clauses are present.
func (o *ListOptions) BuildWhereSQL() (string, []any) {
	if len(o.Where) == 0 {
		return "", nil
	}

	var exprs []string
	var args []any
	for _, w := range o.Where {
		exprs = append(exprs, w.Expr)
		args = append(args, w.Args...)
	}
	return " WHERE " + strings.Join(exprs, " AND "), args
}

// EntityMapper maps between a database row and a Go struct. Implementations
// are provided per-entity type.
type EntityMapper[T any] interface {
	// TableName returns the database table name.
	TableName() string

	// Columns returns the column names for SELECT queries.
	Columns() []string

	// ScanRow scans a database.Row into an entity.
	ScanRow(row db.Row) (*T, error)

	// ScanRows scans the current row from database.Rows into an entity.
	ScanRows(rows db.Rows) (*T, error)

	// InsertSQL returns the INSERT statement and arguments for the entity.
	InsertSQL(entity *T) (string, []any)

	// UpdateSQL returns the UPDATE statement and arguments for the entity.
	UpdateSQL(entity *T) (string, []any)

	// PrimaryKeyColumn returns the name of the primary key column.
	PrimaryKeyColumn() string
}

// GenericRepository is a generic CRUD repository backed by a database.Database
// connection and an EntityMapper.
type GenericRepository[T any] struct {
	DB     db.Database
	Mapper EntityMapper[T]
}

// NewGenericRepository creates a new repository for entity type T.
func NewGenericRepository[T any](
	database db.Database, mapper EntityMapper[T],
) *GenericRepository[T] {
	return &GenericRepository[T]{DB: database, Mapper: mapper}
}

// Create inserts a new entity.
func (r *GenericRepository[T]) Create(ctx context.Context, entity *T) error {
	query, args := r.Mapper.InsertSQL(entity)
	_, err := r.DB.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("create %s: %w", r.Mapper.TableName(), err)
	}
	return nil
}

// GetByID retrieves an entity by primary key.
func (r *GenericRepository[T]) GetByID(ctx context.Context, id any) (*T, error) {
	cols := strings.Join(r.Mapper.Columns(), ", ")
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = ?",
		cols, r.Mapper.TableName(), r.Mapper.PrimaryKeyColumn(),
	)
	row := r.DB.QueryRow(ctx, query, id)
	entity, err := r.Mapper.ScanRow(row)
	if err != nil {
		return nil, fmt.Errorf("get %s by id: %w", r.Mapper.TableName(), err)
	}
	return entity, nil
}

// Update modifies an existing entity.
func (r *GenericRepository[T]) Update(ctx context.Context, entity *T) error {
	query, args := r.Mapper.UpdateSQL(entity)
	_, err := r.DB.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update %s: %w", r.Mapper.TableName(), err)
	}
	return nil
}

// Delete removes an entity by primary key.
func (r *GenericRepository[T]) Delete(ctx context.Context, id any) error {
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = ?",
		r.Mapper.TableName(), r.Mapper.PrimaryKeyColumn(),
	)
	_, err := r.DB.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete %s: %w", r.Mapper.TableName(), err)
	}
	return nil
}

// List returns entities matching the list options.
func (r *GenericRepository[T]) List(
	ctx context.Context, opts ListOptions,
) ([]T, error) {
	cols := strings.Join(r.Mapper.Columns(), ", ")
	query := fmt.Sprintf("SELECT %s FROM %s", cols, r.Mapper.TableName())

	whereSql, whereArgs := opts.BuildWhereSQL()
	query += whereSql

	if opts.OrderBy != "" {
		query += " ORDER BY " + opts.OrderBy
	}
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		if opts.Limit <= 0 {
			// SQLite requires LIMIT before OFFSET; use -1 for unlimited.
			query += " LIMIT -1"
		}
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := r.DB.Query(ctx, query, whereArgs...)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", r.Mapper.TableName(), err)
	}
	defer func() { _ = rows.Close() }()

	var results []T
	for rows.Next() {
		entity, err := r.Mapper.ScanRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", r.Mapper.TableName(), err)
		}
		results = append(results, *entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", r.Mapper.TableName(), err)
	}
	return results, nil
}

// Count returns the number of matching entities.
func (r *GenericRepository[T]) Count(
	ctx context.Context, opts ListOptions,
) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.Mapper.TableName())

	whereSql, whereArgs := opts.BuildWhereSQL()
	query += whereSql

	row := r.DB.QueryRow(ctx, query, whereArgs...)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count %s: %w", r.Mapper.TableName(), err)
	}
	return count, nil
}
