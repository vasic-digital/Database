# Lesson 5: Generic Repository and Query Builder

## Learning Objectives

- Build a type-safe generic repository using Go generics that handles CRUD without code generation
- Design a fluent SQL query builder with method chaining and parameterized placeholders
- Understand the `EntityMapper[T]` bridge between generic repository and entity-specific mappings

## Key Concepts

- **Repository Pattern with Generics**: `GenericRepository[T]` provides `Create`, `GetByID`, `Update`, `Delete`, `List`, and `Count` methods for any type `T`. It delegates SQL generation and row scanning to an `EntityMapper[T]`.
- **EntityMapper Bridge**: The mapper is responsible for table name, column definitions, row scanning, INSERT/UPDATE SQL generation, and primary key identification. The repository never needs to know the schema.
- **Fluent Query Builder**: `query.Builder` uses method chaining to construct SQL queries. Each method returns `*Builder`, and `Build()` produces the final SQL string and arguments slice. Parameterized placeholders prevent SQL injection.

## Code Walkthrough

### Source: `pkg/repository/repository.go`

The generic repository delegates all schema-specific work to the mapper:

```go
type GenericRepository[T any] struct {
    DB     database.Database
    Mapper EntityMapper[T]
}
```

The `EntityMapper[T]` interface defines:

- `TableName()` -- returns the SQL table name
- `Columns()` -- returns the column list
- `ScanRow(Row) (T, error)` -- scans a single row into T
- `InsertSQL(T) (string, []any)` -- generates INSERT statement
- `UpdateSQL(T) (string, []any)` -- generates UPDATE statement
- `PrimaryKeyColumn()` -- identifies the PK column

This separation means adding a new entity requires only writing a mapper, not modifying repository code.

### Source: `pkg/query/query.go`

The query builder provides a fluent interface:

```go
sql, args := query.New().
    Select("id", "name", "email").
    From("users").
    Where(query.Eq("active", true)).
    OrderBy("name", "ASC").
    Limit(10).
    Build()
```

Each condition function (`Eq`, `NotEq`, `Gt`, `Lt`, `In`, `Like`) returns a `Condition` that produces a parameterized placeholder (`?`) and adds the value to the arguments slice. The builder is stateless after `Build()` and can be reused.

### Source: `pkg/query/query_test.go` and `pkg/repository/repository_test.go`

Tests verify:
- Query builder produces correct SQL with proper placeholder ordering
- Conditions handle edge cases (nil values, empty slices for IN)
- Repository CRUD operations work end-to-end with a mock mapper
- List with pagination and filtering generates correct queries

## Practice Exercise

1. Implement an `EntityMapper` for a `Product` struct with fields `ID`, `Name`, `Price`, and `CategoryID`. Define all required methods.
2. Use the query builder to construct a complex query: SELECT with multiple WHERE conditions (AND/OR), ORDER BY, LIMIT, and OFFSET. Print the generated SQL and arguments.
3. Write a test that creates a `GenericRepository[Product]` with your mapper, inserts three products via an in-memory SQLite database, then lists them with a price filter using the query builder.
