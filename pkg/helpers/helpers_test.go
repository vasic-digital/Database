package helpers

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	return db
}

func TestWithTransaction_Commit(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	err := WithTransaction(context.Background(), db, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test (name) VALUES (?)", "committed")
		return err
	})
	require.NoError(t, err)

	var name string
	err = db.QueryRow("SELECT name FROM test WHERE id = 1").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "committed", name)
}

func TestWithTransaction_Rollback(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	testErr := errors.New("test error")
	err := WithTransaction(context.Background(), db, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test (name) VALUES (?)", "should-rollback")
		require.NoError(t, err)
		return testErr
	})
	assert.ErrorIs(t, err, testErr)

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestWithTransactionOpts_ReadOnly(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	_, err := db.Exec("INSERT INTO test (name) VALUES (?)", "existing")
	require.NoError(t, err)

	var name string
	err = WithTransactionOpts(context.Background(), db, &sql.TxOptions{ReadOnly: true}, func(tx *sql.Tx) error {
		return tx.QueryRow("SELECT name FROM test WHERE id = 1").Scan(&name)
	})
	require.NoError(t, err)
	assert.Equal(t, "existing", name)
}

func TestWithTransaction_ContextCancelled(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := WithTransaction(ctx, db, func(tx *sql.Tx) error {
		return nil
	})
	assert.Error(t, err)
}
