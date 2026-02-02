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
	var _ Repository[testEntity] = (*GenericRepository[testEntity])(nil)
}
