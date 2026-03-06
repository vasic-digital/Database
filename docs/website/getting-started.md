# Getting Started

## Installation

```bash
go get digital.vasic.database
```

## Connect to SQLite

```go
package main

import (
    "fmt"
    "digital.vasic.database/pkg/sqlite"
)

func main() {
    cfg := sqlite.DefaultConfig("app.db")
    client := sqlite.NewClient(cfg)

    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Close()

    if err := client.HealthCheck(); err != nil {
        panic(err)
    }
    fmt.Println("Connected to SQLite")
}
```

## Connect to PostgreSQL

```go
package main

import (
    "fmt"
    "digital.vasic.database/pkg/postgres"
)

func main() {
    cfg := postgres.DefaultConfig()
    cfg.Host = "localhost"
    cfg.Port = 5432
    cfg.Database = "myapp"
    cfg.User = "postgres"
    cfg.Password = "secret"

    client := postgres.NewClient(cfg)
    if err := client.Connect(); err != nil {
        panic(err)
    }
    defer client.Close()

    fmt.Println("Connected to PostgreSQL")
}
```

## Run Queries

All adapters implement the `database.Database` interface, so queries work identically regardless of backend:

```go
// Execute a statement
_, err := client.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, active BOOLEAN)")

// Query rows
rows, err := client.Query("SELECT id, name FROM users WHERE active = ?", true)
defer rows.Close()
for rows.Next() {
    var id int
    var name string
    rows.Scan(&id, &name)
    fmt.Printf("User %d: %s\n", id, name)
}

// Query a single row
row := client.QueryRow("SELECT name FROM users WHERE id = ?", 1)
var name string
row.Scan(&name)
```

## Build Queries with the Query Builder

```go
import "digital.vasic.database/pkg/query"

sql, args := query.New().
    Select("id", "name", "email").
    From("users").
    Where(query.Eq("active", true)).
    OrderBy("name ASC").
    Limit(10).
    Build()

rows, err := client.Query(sql, args...)
```

## Run Migrations

```go
import "digital.vasic.database/pkg/migration"

runner := migration.NewRunner(client)
runner.Add(migration.Migration{
    Version: 1,
    Name:    "create_users",
    Up:      "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)",
    Down:    "DROP TABLE users",
})

if err := runner.MigrateUp(); err != nil {
    panic(err)
}
```

## Use the Generic Repository

```go
import (
    "digital.vasic.database/pkg/repository"
    "digital.vasic.database/pkg/database"
)

type User struct {
    ID   int
    Name string
}

// Implement the EntityMapper[User] interface for your type,
// then create a repository:
repo := repository.NewGenericRepository[User](client, userMapper)

// CRUD operations
err := repo.Create(&User{Name: "Alice"})
user, err := repo.GetByID(1)
users, err := repo.List()
count, err := repo.Count()
```
