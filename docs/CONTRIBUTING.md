# Contributing - Database Module

## Getting Started

1. Clone the repository via SSH:

```bash
git clone <ssh-url>
cd Database
```

2. Ensure you have Go 1.24+ installed:

```bash
go version
```

3. Install dependencies:

```bash
go mod download
```

4. Run tests:

```bash
go test ./... -count=1 -race
```

## Development Workflow

### Branch Naming

Use conventional prefixes:

- `feat/` -- New features (e.g., `feat/mysql-adapter`)
- `fix/` -- Bug fixes (e.g., `fix/pool-deadlock`)
- `chore/` -- Maintenance (e.g., `chore/update-pgx`)
- `docs/` -- Documentation (e.g., `docs/api-reference`)
- `refactor/` -- Code restructuring (e.g., `refactor/migration-runner`)
- `test/` -- Test improvements (e.g., `test/repository-edge-cases`)

### Commit Messages

Follow Conventional Commits:

```
<type>(<scope>): <description>

[optional body]
```

Examples:
```
feat(query): add Between condition constructor
fix(pool): prevent deadlock on concurrent close and acquire
test(repository): add edge case for empty List result
docs(migration): document RollbackWith behavior
```

### Code Quality Checks

Before committing, run:

```bash
go fmt ./...
go vet ./...
go test ./... -count=1 -race
```

If `golangci-lint` is available:

```bash
golangci-lint run ./...
```

## Code Style

### General Rules

- Follow standard Go conventions ([Effective Go](https://go.dev/doc/effective_go))
- Format with `gofmt` (enforced)
- Line length: 100 characters or fewer (readability first)
- Group imports: stdlib, then third-party, then internal (blank line separated)

### Naming

- Private: `camelCase`
- Exported: `PascalCase`
- Constants: `UPPER_SNAKE_CASE`
- Acronyms: all caps (`HTTP`, `URL`, `ID`, `SQL`, `DSN`)
- Receivers: 1-2 letters (`c` for Client, `r` for Runner/Repository, `b` for Builder, `p` for Pool)

### Error Handling

- Always check errors
- Wrap with context: `fmt.Errorf("operation detail: %w", err)`
- Use `defer` for cleanup (e.g., `defer rows.Close()`)
- Discard secondary cleanup errors with `_` when appropriate

### Interfaces

- Keep interfaces small and focused
- Accept interfaces, return structs
- Place interface definitions in the package that uses them, not the package that implements them (except for `pkg/database` which defines shared contracts)

## Testing Standards

### Requirements

- Every exported type, function, and method must have tests
- Use table-driven tests with `testify`
- Test naming: `Test<Struct>_<Method>_<Scenario>`
- Run with `-race` to detect data races
- Run with `-count=1` to disable test caching

### Example Test Structure

```go
func TestBuilder_Build_WithMultipleConditions(t *testing.T) {
    tests := []struct {
        name     string
        setup    func() *query.Builder
        wantSQL  string
        wantArgs []any
    }{
        {
            name: "eq and gt conditions",
            setup: func() *query.Builder {
                return query.New().
                    Select("id").
                    From("users").
                    Where(query.Eq("active", true)).
                    Where(query.Gt("age", 18))
            },
            wantSQL:  "SELECT id FROM users WHERE active = ? AND age > ?",
            wantArgs: []any{true, 18},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            sql, args := tt.setup().Build()
            assert.Equal(t, tt.wantSQL, sql)
            assert.Equal(t, tt.wantArgs, args)
        })
    }
}
```

### Integration Tests

PostgreSQL integration tests require a live instance. Use build tags:

```go
//go:build integration

package postgres_test
```

Run with:

```bash
go test -tags=integration ./pkg/postgres/...
```

SQLite tests use `:memory:` databases and require no external infrastructure.

## Adding a New Database Adapter

1. Create `pkg/<driver>/<driver>.go`
2. Define a `Client` struct implementing `database.Database`
3. Define a `Config` struct with driver-specific options
4. Provide `DefaultConfig()` factory function and `New(cfg) *Client` constructor
5. Wrap driver-specific types into `database.Row`, `database.Rows`, `database.Result`, `database.Tx`
6. Create `pkg/<driver>/<driver>_test.go` with full test coverage
7. Update documentation (API_REFERENCE.md, USER_GUIDE.md, ARCHITECTURE.md)

## Adding a New Query Condition

1. Define a new unexported struct implementing `query.Condition` in `pkg/query/query.go`
2. Add a public constructor function (e.g., `func Between(column string, low, high any) Condition`)
3. Implement the `Build() (string, []any)` method
4. Add table-driven tests in `pkg/query/query_test.go`
5. Update docs/API_REFERENCE.md

## Package Dependencies

When adding code, respect the dependency graph:

- `pkg/database` must not import any other module package
- `pkg/query` must not import any other module package
- `pkg/pool` must not import any other module package
- `pkg/postgres` may only import `pkg/database`
- `pkg/sqlite` may only import `pkg/database`
- `pkg/migration` may only import `pkg/database`
- `pkg/repository` may only import `pkg/database`

No circular dependencies are permitted.

## Reporting Issues

When reporting a bug, include:

1. Go version (`go version`)
2. Module version or commit hash
3. Database driver and version (PostgreSQL version, SQLite version)
4. Minimal reproduction code
5. Expected vs. actual behavior
6. Full error output with stack trace if available
