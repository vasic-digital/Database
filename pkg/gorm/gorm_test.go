package gorm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/glebarez/sqlite"

	adapter "digital.vasic.database/pkg/gorm"
)

type TestUser struct {
	ID   uint   `gorm:"primarykey"`
	Name string `gorm:"size:100"`
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&TestUser{})
	return db
}

func TestAdapter_HealthCheck(t *testing.T) {
	db := setupTestDB(t)
	a := adapter.New(db)
	err := a.HealthCheck()
	assert.NoError(t, err)
}

func TestAdapter_Create(t *testing.T) {
	db := setupTestDB(t)
	a := adapter.New(db)
	user := &TestUser{Name: "Alice"}
	err := a.DB().Create(user).Error
	require.NoError(t, err)
	assert.NotZero(t, user.ID)
}

func TestAdapter_FindByID(t *testing.T) {
	db := setupTestDB(t)
	a := adapter.New(db)
	user := &TestUser{Name: "Bob"}
	a.DB().Create(user)
	var found TestUser
	err := a.DB().First(&found, user.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "Bob", found.Name)
}

func TestAdapter_Transaction(t *testing.T) {
	db := setupTestDB(t)
	a := adapter.New(db)
	err := a.Transaction(func(tx *gorm.DB) error {
		tx.Create(&TestUser{Name: "Charlie"})
		tx.Create(&TestUser{Name: "Diana"})
		return nil
	})
	require.NoError(t, err)
	var count int64
	a.DB().Model(&TestUser{}).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestAdapter_TransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	a := adapter.New(db)
	err := a.Transaction(func(tx *gorm.DB) error {
		tx.Create(&TestUser{Name: "Eve"})
		return assert.AnError
	})
	assert.Error(t, err)
	var count int64
	a.DB().Model(&TestUser{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestAdapter_ConfigurePool(t *testing.T) {
	db := setupTestDB(t)
	a := adapter.New(db)
	cfg := adapter.DefaultPoolConfig()
	err := a.ConfigurePool(cfg)
	assert.NoError(t, err)
}

func TestAdapter_ConfigurePool_NilConfig(t *testing.T) {
	db := setupTestDB(t)
	a := adapter.New(db)
	err := a.ConfigurePool(nil)
	assert.Error(t, err)
}

func TestAdapter_Close(t *testing.T) {
	db := setupTestDB(t)
	a := adapter.New(db)
	err := a.Close()
	assert.NoError(t, err)
}

func TestDefaultPoolConfig(t *testing.T) {
	cfg := adapter.DefaultPoolConfig()
	assert.Equal(t, 50, cfg.MaxOpenConns)
	assert.Equal(t, 10, cfg.MaxIdleConns)
}
