# Database

Generic, reusable Go module for relational database operations.

## Features

- **Driver-agnostic interfaces** -- `Database`, `Tx`, `Row`, `Rows`, `Result`
- **PostgreSQL adapter** -- pgx/v5 with connection pooling (pgxpool)
- **SQLite adapter** -- Pure Go via modernc.org/sqlite (no CGO required)
- **Connection pooling** -- Generic pool with metrics, health checking, eviction
- **Schema migrations** -- Version-tracked migrations with apply and rollback
- **Repository pattern** -- Generic CRUD with generics (`Repository[T]`)
- **Query builder** -- Fluent API with type-safe conditions (Eq, Gt, In, Like, etc.)

## Installation

```bash
go get digital.vasic.database
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "digital.vasic.database/pkg/sqlite"
    "digital.vasic.database/pkg/migration"
)

func main() {
    ctx := context.Background()

    // Create and connect to SQLite.
    db := sqlite.New(sqlite.DefaultConfig("app.db"))
    if err := db.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Run migrations.
    runner := migration.NewRunner(db, "")
    err := runner.Apply(ctx, []migration.Migration{
        {
            Version: 1,
            Name:    "create users",
            Up:      "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)",
            Down:    "DROP TABLE users",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Execute queries.
    db.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "Alice")
}
```

## Testing

```bash
go test ./... -count=1 -race
```

## License

See repository root.
