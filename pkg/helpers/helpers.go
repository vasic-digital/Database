// Package helpers provides database transaction utilities.
//
// It includes a safe transaction wrapper that automatically handles
// commit/rollback based on the outcome of the provided function.
package helpers

import (
	"context"
	"database/sql"
	"fmt"
)

// TxFunc is a function that executes within a transaction.
type TxFunc func(tx *sql.Tx) error

// WithTransaction executes fn within a database transaction.
// If fn returns an error, the transaction is rolled back.
// Otherwise, it is committed.
func WithTransaction(ctx context.Context, db *sql.DB, fn TxFunc) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}

// WithTransactionOpts executes fn within a transaction with the given options.
func WithTransactionOpts(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn TxFunc) error {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}
