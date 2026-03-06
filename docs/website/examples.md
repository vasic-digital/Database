# Examples

## Fluent Query Builder with Conditions

Build complex queries safely using the query builder's method chaining and typed conditions:

```go
package main

import (
    "fmt"
    "digital.vasic.database/pkg/query"
)

func main() {
    // Simple select with conditions
    sql, args := query.New().
        Select("id", "title", "year").
        From("movies").
        Where(query.Eq("genre", "sci-fi")).
        Where(query.Gt("year", 2000)).
        OrderBy("year DESC").
        Limit(20).
        Build()

    fmt.Println(sql)  // SELECT id, title, year FROM movies WHERE genre = ? AND year > ? ORDER BY year DESC LIMIT 20
    fmt.Println(args) // ["sci-fi", 2000]

    // Insert query
    insertSQL, insertArgs := query.New().
        InsertInto("movies").
        Columns("title", "genre", "year").
        Values("Dune", "sci-fi", 2021).
        Build()

    fmt.Println(insertSQL)  // INSERT INTO movies (title, genre, year) VALUES (?, ?, ?)
    fmt.Println(insertArgs) // ["Dune", "sci-fi", 2021]
}
```

## Transactional Operations

Use transactions for atomic multi-step database operations:

```go
package main

import (
    "fmt"
    "digital.vasic.database/pkg/sqlite"
)

func main() {
    client := sqlite.NewClient(sqlite.DefaultConfig("app.db"))
    client.Connect()
    defer client.Close()

    // Begin a transaction
    tx, err := client.Begin()
    if err != nil {
        panic(err)
    }

    // Execute multiple operations atomically
    _, err = tx.Exec("INSERT INTO accounts (name, balance) VALUES (?, ?)", "Alice", 1000)
    if err != nil {
        tx.Rollback()
        panic(err)
    }

    _, err = tx.Exec("INSERT INTO accounts (name, balance) VALUES (?, ?)", "Bob", 500)
    if err != nil {
        tx.Rollback()
        panic(err)
    }

    // Transfer funds
    _, err = tx.Exec("UPDATE accounts SET balance = balance - ? WHERE name = ?", 200, "Alice")
    if err != nil {
        tx.Rollback()
        panic(err)
    }

    _, err = tx.Exec("UPDATE accounts SET balance = balance + ? WHERE name = ?", 200, "Bob")
    if err != nil {
        tx.Rollback()
        panic(err)
    }

    // Commit all changes
    if err := tx.Commit(); err != nil {
        panic(err)
    }

    fmt.Println("Transfer complete")
}
```

## Connection Pool with Health Checking

Use the generic connection pool for managing database connections with automatic health checking:

```go
package main

import (
    "fmt"
    "digital.vasic.database/pkg/pool"
)

func main() {
    cfg := &pool.Config{
        MaxSize:       10,
        MinIdle:       2,
        MaxIdleTime:   300, // seconds
        HealthCheck:   true,
        CheckInterval: 30, // seconds
    }

    p := pool.NewGenericPool(cfg, func() (interface{}, error) {
        // Factory function: create a new connection
        return createDatabaseConnection()
    }, func(conn interface{}) error {
        // Health check function
        return conn.(Pingable).Ping()
    })

    // Acquire a connection
    conn, err := p.Acquire()
    if err != nil {
        panic(err)
    }

    // Use the connection...
    fmt.Println("Got connection from pool")

    // Return it to the pool
    p.Release(conn)

    // Check pool statistics
    stats := p.Stats()
    fmt.Printf("Active: %d, Idle: %d, Total: %d\n",
        stats.ActiveCount, stats.IdleCount, stats.TotalCount)

    // Shut down the pool
    p.Close()
}
```
