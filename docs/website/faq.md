# FAQ

## Why use modernc.org/sqlite instead of mattn/go-sqlite3?

`modernc.org/sqlite` is a pure Go SQLite implementation that requires no CGO. This makes cross-compilation and containerized builds significantly simpler. There is no need to install C compilers or SQLite development libraries in the build environment.

## Is the query builder safe from SQL injection?

Yes. The query builder uses parameterized placeholders (`?`) for all values. Values are never interpolated into the SQL string. The `Build()` method returns the SQL string and arguments separately, and you pass them to `Query()` or `Exec()` which handle parameterization at the driver level.

## Can I use the repository pattern with custom queries?

The `GenericRepository[T]` provides standard CRUD operations (Create, GetByID, Update, Delete, List, Count). For custom queries beyond CRUD, use the underlying `database.Database` interface directly with the query builder. The repository and direct database access can coexist in the same application.

## How does the PostgreSQL connection pool size itself?

The `postgres.DefaultConfig()` factory dynamically sizes the pool based on `runtime.NumCPU()`:

- `MaxConns = min(max(CPU*2 + 1, 10), 50)`
- `MinConns = CPU / 2`

This avoids under-utilization on large machines and resource exhaustion on small ones. Override these values in the `Config` struct if needed.

## Are the adapters thread-safe?

Yes. The PostgreSQL adapter is thread-safe via `pgxpool` (goroutine-safe by design). The SQLite adapter is thread-safe via `database/sql` connection pool. The query builder is NOT thread-safe -- each goroutine should create its own builder instance. The generic repository and migration runner are thread-safe if the underlying `Database` implementation is thread-safe.
