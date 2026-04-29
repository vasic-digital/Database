package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "digital.vasic.database/pkg/database"
	"digital.vasic.database/pkg/sqlite"
)

// testEntity is a simple entity for testing.
type testEntity struct {
	ID   int
	Name string
}

// testMapper implements EntityMapper[testEntity].
type testMapper struct{}

func (m *testMapper) TableName() string          { return "test_entities" }
func (m *testMapper) PrimaryKeyColumn() string   { return "id" }
func (m *testMapper) Columns() []string          { return []string{"id", "name"} }

func (m *testMapper) ScanRow(row db.Row) (*testEntity, error) {
	var e testEntity
	if err := row.Scan(&e.ID, &e.Name); err != nil {
		return nil, err
	}
	return &e, nil
}

func (m *testMapper) ScanRows(rows db.Rows) (*testEntity, error) {
	var e testEntity
	if err := rows.Scan(&e.ID, &e.Name); err != nil {
		return nil, err
	}
	return &e, nil
}

func (m *testMapper) InsertSQL(entity *testEntity) (string, []any) {
	return "INSERT INTO test_entities (id, name) VALUES (?, ?)",
		[]any{entity.ID, entity.Name}
}

func (m *testMapper) UpdateSQL(entity *testEntity) (string, []any) {
	return "UPDATE test_entities SET name = ? WHERE id = ?",
		[]any{entity.Name, entity.ID}
}

func newTestDB(t *testing.T) *sqlite.Client {
	t.Helper()
	c := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := c.Connect(ctx)
	require.NoError(t, err)

	_, err = c.Exec(ctx,
		"CREATE TABLE test_entities (id INTEGER PRIMARY KEY, name TEXT NOT NULL)")
	require.NoError(t, err)

	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestListOptions_BuildWhereSQL(t *testing.T) {
	tests := []struct {
		name         string
		opts         ListOptions
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name:         "no where clauses",
			opts:         ListOptions{},
			expectedSQL:  "",
			expectedArgs: nil,
		},
		{
			name: "single clause",
			opts: ListOptions{
				Where: []WhereClause{
					{Expr: "status = ?", Args: []any{"active"}},
				},
			},
			expectedSQL:  " WHERE status = ?",
			expectedArgs: []any{"active"},
		},
		{
			name: "multiple clauses",
			opts: ListOptions{
				Where: []WhereClause{
					{Expr: "status = ?", Args: []any{"active"}},
					{Expr: "age > ?", Args: []any{18}},
				},
			},
			expectedSQL:  " WHERE status = ? AND age > ?",
			expectedArgs: []any{"active", 18},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.opts.BuildWhereSQL()
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestGenericRepository_Create(t *testing.T) {
	tests := []struct {
		name   string
		entity testEntity
	}{
		{name: "first entity", entity: testEntity{ID: 1, Name: "alice"}},
		{name: "second entity", entity: testEntity{ID: 2, Name: "bob"}},
	}

	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &testMapper{})
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, &tt.entity)
			require.NoError(t, err)
		})
	}
}

func TestGenericRepository_GetByID(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &testMapper{})
	ctx := context.Background()

	// Seed data.
	require.NoError(t, repo.Create(ctx, &testEntity{ID: 1, Name: "alice"}))
	require.NoError(t, repo.Create(ctx, &testEntity{ID: 2, Name: "bob"}))

	tests := []struct {
		name    string
		id      int
		want    *testEntity
		wantErr bool
	}{
		{
			name: "existing entity",
			id:   1,
			want: &testEntity{ID: 1, Name: "alice"},
		},
		{
			name: "another entity",
			id:   2,
			want: &testEntity{ID: 2, Name: "bob"},
		},
		{
			name:    "non-existent entity",
			id:      999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.GetByID(ctx, tt.id)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenericRepository_Update(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &testMapper{})
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &testEntity{ID: 1, Name: "alice"}))

	t.Run("update name", func(t *testing.T) {
		updated := testEntity{ID: 1, Name: "alice_updated"}
		err := repo.Update(ctx, &updated)
		require.NoError(t, err)

		got, err := repo.GetByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "alice_updated", got.Name)
	})
}

func TestGenericRepository_Delete(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &testMapper{})
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &testEntity{ID: 1, Name: "alice"}))

	t.Run("delete existing", func(t *testing.T) {
		err := repo.Delete(ctx, 1)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, 1)
		require.Error(t, err)
	})
}

func TestGenericRepository_List(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &testMapper{})
	ctx := context.Background()

	// Seed data.
	for i := 1; i <= 5; i++ {
		require.NoError(t, repo.Create(ctx,
			&testEntity{ID: i, Name: fmt.Sprintf("user_%d", i)}))
	}

	tests := []struct {
		name      string
		opts      ListOptions
		wantCount int
		wantFirst string
	}{
		{
			name:      "all entities",
			opts:      ListOptions{},
			wantCount: 5,
			wantFirst: "user_1",
		},
		{
			name:      "with limit",
			opts:      ListOptions{Limit: 2, OrderBy: "id ASC"},
			wantCount: 2,
			wantFirst: "user_1",
		},
		{
			name:      "with offset",
			opts:      ListOptions{Offset: 3, OrderBy: "id ASC"},
			wantCount: 2,
			wantFirst: "user_4",
		},
		{
			name: "with where clause",
			opts: ListOptions{
				Where: []WhereClause{
					{Expr: "id > ?", Args: []any{3}},
				},
				OrderBy: "id ASC",
			},
			wantCount: 2,
			wantFirst: "user_4",
		},
		{
			name:      "order by desc",
			opts:      ListOptions{Limit: 1, OrderBy: "id DESC"},
			wantCount: 1,
			wantFirst: "user_5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := repo.List(ctx, tt.opts)
			require.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantFirst, results[0].Name)
			}
		})
	}
}

func TestGenericRepository_Count(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &testMapper{})
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		require.NoError(t, repo.Create(ctx,
			&testEntity{ID: i, Name: fmt.Sprintf("user_%d", i)}))
	}

	tests := []struct {
		name string
		opts ListOptions
		want int64
	}{
		{
			name: "count all",
			opts: ListOptions{},
			want: 5,
		},
		{
			name: "count with where",
			opts: ListOptions{
				Where: []WhereClause{
					{Expr: "id > ?", Args: []any{3}},
				},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.Count(ctx, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenericRepository_Interface(t *testing.T) {
	// bluff-scan: no-assert-ok (feature/interface smoke — wiring must not panic)
	var _ Repository[testEntity] = (*GenericRepository[testEntity])(nil)
}

// errorMapper is a test mapper that returns invalid SQL to trigger errors.
type errorMapper struct{}

func (m *errorMapper) TableName() string        { return "nonexistent_table" }
func (m *errorMapper) PrimaryKeyColumn() string { return "id" }
func (m *errorMapper) Columns() []string        { return []string{"id", "name"} }

func (m *errorMapper) ScanRow(row db.Row) (*testEntity, error) {
	var e testEntity
	if err := row.Scan(&e.ID, &e.Name); err != nil {
		return nil, err
	}
	return &e, nil
}

func (m *errorMapper) ScanRows(rows db.Rows) (*testEntity, error) {
	var e testEntity
	if err := rows.Scan(&e.ID, &e.Name); err != nil {
		return nil, err
	}
	return &e, nil
}

func (m *errorMapper) InsertSQL(entity *testEntity) (string, []any) {
	return "INSERT INTO nonexistent_table (id, name) VALUES (?, ?)",
		[]any{entity.ID, entity.Name}
}

func (m *errorMapper) UpdateSQL(entity *testEntity) (string, []any) {
	return "UPDATE nonexistent_table SET name = ? WHERE id = ?",
		[]any{entity.Name, entity.ID}
}

func TestGenericRepository_Create_Error(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &errorMapper{})
	ctx := context.Background()

	err := repo.Create(ctx, &testEntity{ID: 1, Name: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create nonexistent_table:")
}

func TestGenericRepository_Update_Error(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &errorMapper{})
	ctx := context.Background()

	err := repo.Update(ctx, &testEntity{ID: 1, Name: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update nonexistent_table:")
}

func TestGenericRepository_Delete_Error(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &errorMapper{})
	ctx := context.Background()

	err := repo.Delete(ctx, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete nonexistent_table:")
}

func TestGenericRepository_List_Error(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &errorMapper{})
	ctx := context.Background()

	_, err := repo.List(ctx, ListOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list nonexistent_table:")
}

func TestGenericRepository_Count_Error(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &errorMapper{})
	ctx := context.Background()

	_, err := repo.Count(ctx, ListOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count nonexistent_table:")
}

// scanErrorMapper returns valid SQL but causes scan errors.
type scanErrorMapper struct{}

func (m *scanErrorMapper) TableName() string        { return "scan_error_table" }
func (m *scanErrorMapper) PrimaryKeyColumn() string { return "id" }
func (m *scanErrorMapper) Columns() []string        { return []string{"id", "name"} }

func (m *scanErrorMapper) ScanRow(row db.Row) (*testEntity, error) {
	// Intentionally scan wrong types to cause error.
	var wrongType1 string
	var wrongType2 int
	if err := row.Scan(&wrongType1, &wrongType2); err != nil {
		return nil, err
	}
	return &testEntity{}, nil
}

func (m *scanErrorMapper) ScanRows(rows db.Rows) (*testEntity, error) {
	// Intentionally return an error.
	return nil, fmt.Errorf("intentional scan error")
}

func (m *scanErrorMapper) InsertSQL(entity *testEntity) (string, []any) {
	return "INSERT INTO scan_error_table (id, name) VALUES (?, ?)",
		[]any{entity.ID, entity.Name}
}

func (m *scanErrorMapper) UpdateSQL(entity *testEntity) (string, []any) {
	return "UPDATE scan_error_table SET name = ? WHERE id = ?",
		[]any{entity.Name, entity.ID}
}

func TestGenericRepository_List_ScanError(t *testing.T) {
	c := sqlite.New(sqlite.DefaultConfig(":memory:"))
	ctx := context.Background()

	err := c.Connect(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	// Create a table with test data.
	_, err = c.Exec(ctx,
		"CREATE TABLE scan_error_table (id INTEGER PRIMARY KEY, name TEXT NOT NULL)")
	require.NoError(t, err)

	_, err = c.Exec(ctx,
		"INSERT INTO scan_error_table (id, name) VALUES (1, 'test')")
	require.NoError(t, err)

	repo := NewGenericRepository[testEntity](c, &scanErrorMapper{})

	_, err = repo.List(ctx, ListOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan scan_error_table:")
}

// rowsErrorMapper simulates rows.Err() returning an error.
type rowsErrorMapper struct{}

func (m *rowsErrorMapper) TableName() string        { return "test_entities" }
func (m *rowsErrorMapper) PrimaryKeyColumn() string { return "id" }
func (m *rowsErrorMapper) Columns() []string        { return []string{"id", "name"} }

func (m *rowsErrorMapper) ScanRow(row db.Row) (*testEntity, error) {
	var e testEntity
	if err := row.Scan(&e.ID, &e.Name); err != nil {
		return nil, err
	}
	return &e, nil
}

func (m *rowsErrorMapper) ScanRows(rows db.Rows) (*testEntity, error) {
	var e testEntity
	if err := rows.Scan(&e.ID, &e.Name); err != nil {
		return nil, err
	}
	return &e, nil
}

func (m *rowsErrorMapper) InsertSQL(entity *testEntity) (string, []any) {
	return "INSERT INTO test_entities (id, name) VALUES (?, ?)",
		[]any{entity.ID, entity.Name}
}

func (m *rowsErrorMapper) UpdateSQL(entity *testEntity) (string, []any) {
	return "UPDATE test_entities SET name = ? WHERE id = ?",
		[]any{entity.Name, entity.ID}
}

func TestGenericRepository_List_OffsetWithoutLimit(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &testMapper{})
	ctx := context.Background()

	// Seed data.
	for i := 1; i <= 5; i++ {
		require.NoError(t, repo.Create(ctx,
			&testEntity{ID: i, Name: fmt.Sprintf("user_%d", i)}))
	}

	// Test with offset but no limit (should add LIMIT -1).
	results, err := repo.List(ctx, ListOptions{
		Offset:  2,
		OrderBy: "id ASC",
	})
	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, "user_3", results[0].Name)
}

func TestGenericRepository_List_OffsetWithLimit(t *testing.T) {
	sdb := newTestDB(t)
	repo := NewGenericRepository[testEntity](sdb, &testMapper{})
	ctx := context.Background()

	// Seed data.
	for i := 1; i <= 10; i++ {
		require.NoError(t, repo.Create(ctx,
			&testEntity{ID: i, Name: fmt.Sprintf("user_%d", i)}))
	}

	// Test with both offset and limit.
	results, err := repo.List(ctx, ListOptions{
		Offset:  3,
		Limit:   2,
		OrderBy: "id ASC",
	})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "user_4", results[0].Name)
	assert.Equal(t, "user_5", results[1].Name)
}

// mockRows implements db.Rows and returns an error from Err().
type mockRows struct {
	data    []testEntity
	index   int
	iterErr error
}

func (m *mockRows) Next() bool {
	m.index++
	return m.index <= len(m.data)
}

func (m *mockRows) Scan(dest ...any) error {
	if m.index > 0 && m.index <= len(m.data) {
		*(dest[0].(*int)) = m.data[m.index-1].ID
		*(dest[1].(*string)) = m.data[m.index-1].Name
		return nil
	}
	return fmt.Errorf("no data")
}

func (m *mockRows) Close() error { return nil }
func (m *mockRows) Err() error   { return m.iterErr }

// mockResult implements db.Result.
type mockResult struct {
	affected int64
}

func (m *mockResult) RowsAffected() (int64, error) { return m.affected, nil }

// mockRow implements db.Row.
type mockRow struct {
	err error
}

func (m *mockRow) Scan(dest ...any) error { return m.err }

// mockDB implements db.Database with controllable behavior.
type mockDB struct {
	queryRows   db.Rows
	queryErr    error
	execResult  db.Result
	execErr     error
	queryRowRow db.Row
}

func (m *mockDB) Connect(ctx context.Context) error { return nil }
func (m *mockDB) Close() error                      { return nil }
func (m *mockDB) Exec(ctx context.Context, query string, args ...any) (db.Result, error) {
	return m.execResult, m.execErr
}
func (m *mockDB) Query(ctx context.Context, query string, args ...any) (db.Rows, error) {
	return m.queryRows, m.queryErr
}
func (m *mockDB) QueryRow(ctx context.Context, query string, args ...any) db.Row {
	return m.queryRowRow
}
func (m *mockDB) Begin(ctx context.Context) (db.Tx, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockDB) HealthCheck(ctx context.Context) error { return nil }

func TestGenericRepository_List_RowsErr(t *testing.T) {
	iterErr := fmt.Errorf("iteration error")

	mdb := &mockDB{
		queryRows: &mockRows{
			data:    []testEntity{{ID: 1, Name: "test"}},
			index:   0,
			iterErr: iterErr,
		},
	}

	repo := NewGenericRepository[testEntity](mdb, &testMapper{})
	ctx := context.Background()

	_, err := repo.List(ctx, ListOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iterate test_entities:")
}
