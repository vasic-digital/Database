package netstorage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr64(v int64) *int64 { return &v }

func TestNewNetworkStorage(t *testing.T) {
	s := NewNetworkStorage("1", "Test", StorageTypeWebDAV, "https://dav.test.com")
	assert.Equal(t, "1", s.ID)
	assert.Equal(t, "Test", s.Name)
	assert.Equal(t, StorageTypeWebDAV, s.Type)
	assert.True(t, s.IsEnabled)
	assert.Equal(t, 100, s.Priority)
	assert.True(t, s.SupportsFolders)
	assert.True(t, s.SupportsMetadata)
	assert.False(t, s.IsOnline)
}

func TestNetworkStorageAvailableSpace(t *testing.T) {
	s := NewNetworkStorage("1", "T", StorageTypeFTP, "ftp://t")
	s.TotalSpace = ptr64(1000)
	s.UsedSpace = ptr64(300)
	avail := s.AvailableSpace()
	require.NotNil(t, avail)
	assert.Equal(t, int64(700), *avail)
}

func TestNetworkStorageAvailableSpaceNil(t *testing.T) {
	s := NewNetworkStorage("1", "T", StorageTypeFTP, "ftp://t")
	assert.Nil(t, s.AvailableSpace())
}

func TestNetworkStorageUsagePercentage(t *testing.T) {
	s := NewNetworkStorage("1", "T", StorageTypeFTP, "ftp://t")
	s.TotalSpace = ptr64(1000)
	s.UsedSpace = ptr64(250)
	pct := s.UsagePercentage()
	require.NotNil(t, pct)
	assert.Equal(t, 0.25, *pct)
}

func TestNetworkStorageIsFull(t *testing.T) {
	s := NewNetworkStorage("1", "T", StorageTypeFTP, "ftp://t")
	s.TotalSpace = ptr64(1000)
	s.UsedSpace = ptr64(1000)
	assert.True(t, s.IsFull())
}

func TestNetworkStorageIsLowOnSpace(t *testing.T) {
	s := NewNetworkStorage("1", "T", StorageTypeFTP, "ftp://t")
	s.TotalSpace = ptr64(1000)
	s.UsedSpace = ptr64(950)
	assert.True(t, s.IsLowOnSpace())
}

func TestNetworkStorageNotLowOnSpace(t *testing.T) {
	s := NewNetworkStorage("1", "T", StorageTypeFTP, "ftp://t")
	s.TotalSpace = ptr64(1000)
	s.UsedSpace = ptr64(500)
	assert.False(t, s.IsLowOnSpace())
}

func TestNewCacheEntry(t *testing.T) {
	e := NewCacheEntry("c1", "d1", "/cache/t", "/t", 1024, 1000)
	assert.Equal(t, "c1", e.ID)
	assert.Equal(t, "d1", e.RemoteDocumentID)
	assert.True(t, e.IsValid)
	assert.False(t, e.IsPinned)
	assert.False(t, e.IsInUse)
	assert.Equal(t, 0, e.AccessCount)
	assert.Equal(t, 100, e.Priority)
}

func TestCacheEntryCanBeEvicted(t *testing.T) {
	e := NewCacheEntry("c1", "d1", "/c/t", "/t", 100, 1000)
	assert.True(t, e.CanBeEvicted())

	e.IsPinned = true
	assert.False(t, e.CanBeEvicted())

	e.IsPinned = false
	e.IsInUse = true
	assert.False(t, e.CanBeEvicted())
}

func TestCacheEntryCompressionRatio(t *testing.T) {
	e := NewCacheEntry("c1", "d1", "/c/t", "/t", 50, 1000)
	e.Compression = "gzip"
	e.OriginalSize = ptr64(100)
	ratio := e.CompressionRatio()
	require.NotNil(t, ratio)
	assert.Equal(t, 0.5, *ratio)
}

func TestCacheEntryCompressionRatioNil(t *testing.T) {
	e := NewCacheEntry("c1", "d1", "/c/t", "/t", 100, 1000)
	assert.Nil(t, e.CompressionRatio())
}

func TestNewNetworkOperation(t *testing.T) {
	op := NewNetworkOperation(1, OperationTypeUpload, "/test.txt", 1000)
	assert.Equal(t, int64(1), op.ID)
	assert.Equal(t, OperationTypeUpload, op.Type)
	assert.Equal(t, OperationStatusPending, op.Status)
	assert.Equal(t, 3, op.MaxRetries)
	assert.True(t, op.CanPause)
	assert.True(t, op.CanCancel)
	assert.False(t, op.IsPaused)
}

func TestNetworkOperationIsRunning(t *testing.T) {
	op := NewNetworkOperation(1, OperationTypeUpload, "/t", 1000)
	op.Status = OperationStatusInProgress
	assert.True(t, op.IsRunning())

	op.IsPaused = true
	assert.False(t, op.IsRunning())
}

func TestNetworkOperationCanRetry(t *testing.T) {
	op := NewNetworkOperation(1, OperationTypeUpload, "/t", 1000)
	op.Status = OperationStatusFailed
	op.RetryCount = 1
	assert.True(t, op.CanRetry())

	op.RetryCount = 3
	assert.False(t, op.CanRetry())
}

func TestNetworkOperationProgressPercentage(t *testing.T) {
	op := NewNetworkOperation(1, OperationTypeUpload, "/t", 1000)
	op.Progress = 0.75
	assert.Equal(t, 75, op.ProgressPercentage())
}

func TestNetworkOperationDuration(t *testing.T) {
	op := NewNetworkOperation(1, OperationTypeUpload, "/t", 1000)
	started := int64(2000)
	completed := int64(5000)
	op.StartedAt = &started
	op.CompletedAt = &completed
	d := op.Duration()
	require.NotNil(t, d)
	assert.Equal(t, int64(3000), *d)
}

func TestNetworkOperationDurationNil(t *testing.T) {
	op := NewNetworkOperation(1, OperationTypeUpload, "/t", 1000)
	assert.Nil(t, op.Duration())
}

func TestNetworkOperationIsCompleted(t *testing.T) {
	op := NewNetworkOperation(1, OperationTypeUpload, "/t", 1000)
	op.Status = OperationStatusCompleted
	assert.True(t, op.IsCompleted())
}

func TestNetworkOperationHasFailed(t *testing.T) {
	op := NewNetworkOperation(1, OperationTypeUpload, "/t", 1000)
	op.Status = OperationStatusFailed
	assert.True(t, op.HasFailed())
}

func TestStorageTypes(t *testing.T) {
	types := []StorageType{
		StorageTypeWebDAV, StorageTypeFTP, StorageTypeSFTP, StorageTypeSMB,
		StorageTypeGoogleDrive, StorageTypeDropbox, StorageTypeOneDrive, StorageTypeGit,
	}
	assert.Len(t, types, 8)
}

func TestSyncStatuses(t *testing.T) {
	statuses := []SyncStatus{
		SyncStatusUnknown, SyncStatusSynced, SyncStatusPendingUpload,
		SyncStatusPendingDownload, SyncStatusSyncing, SyncStatusSyncError,
		SyncStatusNotSynced, SyncStatusQueued, SyncStatusConflict,
		SyncStatusUploading, SyncStatusDownloading,
	}
	assert.Len(t, statuses, 11)
}

func TestOperationTypes(t *testing.T) {
	types := []OperationType{
		OperationTypeUpload, OperationTypeDownload, OperationTypeDelete,
		OperationTypeCreateFolder, OperationTypeRename, OperationTypeMove,
		OperationTypeCopy, OperationTypeSync, OperationTypeSearch,
	}
	assert.Len(t, types, 9)
}

func TestOperationStatuses(t *testing.T) {
	statuses := []OperationStatus{
		OperationStatusPending, OperationStatusInProgress, OperationStatusCompleted,
		OperationStatusFailed, OperationStatusPaused, OperationStatusCancelled,
	}
	assert.Len(t, statuses, 6)
}

func TestDocumentPermissions(t *testing.T) {
	perms := []DocumentPermission{
		PermissionRead, PermissionWrite, PermissionDelete, PermissionCreate,
		PermissionRename, PermissionMove, PermissionCopy, PermissionShare,
		PermissionManagePermissions, PermissionViewMetadata, PermissionModifyMetadata,
		PermissionExecute, PermissionDownload, PermissionUpload, PermissionSync,
		PermissionAdmin,
	}
	assert.Len(t, perms, 16)
}
