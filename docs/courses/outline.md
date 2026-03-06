# Course: Building a Database Abstraction Layer in Go

## Module Overview

This course walks through the `digital.vasic.database` module, a layered database abstraction that separates interface contracts from concrete implementations. You will learn how to design driver-agnostic database interfaces, build PostgreSQL and SQLite adapters, implement connection pooling, schema migrations, a generic repository pattern with Go generics, and a fluent query builder.

## Prerequisites

- Intermediate Go knowledge (interfaces, generics, error handling)
- Basic understanding of SQL and relational databases
- Familiarity with PostgreSQL or SQLite concepts
- Go 1.24+ installed

## Lessons

| # | Title | Duration |
|---|-------|----------|
| 1 | Core Interfaces and Configuration | 45 min |
| 2 | Database Adapters: PostgreSQL and SQLite | 50 min |
| 3 | Connection Pooling and Health Checks | 40 min |
| 4 | Schema Migrations with Version Tracking | 40 min |
| 5 | Generic Repository and Query Builder | 50 min |

## Source Files

All source code is under `pkg/` in the Database module:

- `pkg/database/` -- Core interfaces and configuration
- `pkg/postgres/` -- PostgreSQL adapter (pgx/v5)
- `pkg/sqlite/` -- SQLite adapter (modernc.org/sqlite)
- `pkg/pool/` -- Generic connection pool with metrics
- `pkg/migration/` -- Schema migration runner
- `pkg/repository/` -- Generic CRUD repository
- `pkg/query/` -- Fluent SQL query builder
- `pkg/dialect/` -- SQL dialect abstraction
- `pkg/connection/` -- Connection management utilities
- `pkg/helpers/` -- Database helper functions
