// Package netstorage provides entity types and interfaces for network storage
// database operations. This mirrors the Database-KMP Kotlin module.
package netstorage

import "context"

// StorageType represents a storage protocol type.
type StorageType string

const (
	StorageTypeWebDAV      StorageType = "WEBDAV"
	StorageTypeFTP         StorageType = "FTP"
	StorageTypeSFTP        StorageType = "SFTP"
	StorageTypeSMB         StorageType = "SMB"
	StorageTypeGoogleDrive StorageType = "GOOGLE_DRIVE"
	StorageTypeDropbox     StorageType = "DROPBOX"
	StorageTypeOneDrive    StorageType = "ONEDRIVE"
	StorageTypeGit         StorageType = "GIT"
)

// SyncStatus represents synchronization status.
type SyncStatus string

const (
	SyncStatusUnknown         SyncStatus = "UNKNOWN"
	SyncStatusSynced          SyncStatus = "SYNCED"
	SyncStatusPendingUpload   SyncStatus = "PENDING_UPLOAD"
	SyncStatusPendingDownload SyncStatus = "PENDING_DOWNLOAD"
	SyncStatusSyncing         SyncStatus = "SYNCING"
	SyncStatusSyncError       SyncStatus = "SYNC_ERROR"
	SyncStatusNotSynced       SyncStatus = "NOT_SYNCED"
	SyncStatusQueued          SyncStatus = "QUEUED"
	SyncStatusConflict        SyncStatus = "CONFLICT"
	SyncStatusUploading       SyncStatus = "UPLOADING"
	SyncStatusDownloading     SyncStatus = "DOWNLOADING"
)

// OperationType represents types of network operations.
type OperationType string

const (
	OperationTypeUpload       OperationType = "UPLOAD"
	OperationTypeDownload     OperationType = "DOWNLOAD"
	OperationTypeDelete       OperationType = "DELETE"
	OperationTypeCreateFolder OperationType = "CREATE_FOLDER"
	OperationTypeRename       OperationType = "RENAME"
	OperationTypeMove         OperationType = "MOVE"
	OperationTypeCopy         OperationType = "COPY"
	OperationTypeSync         OperationType = "SYNC"
	OperationTypeSearch       OperationType = "SEARCH"
)

// OperationStatus represents the status of a network operation.
type OperationStatus string

const (
	OperationStatusPending    OperationStatus = "PENDING"
	OperationStatusInProgress OperationStatus = "IN_PROGRESS"
	OperationStatusCompleted  OperationStatus = "COMPLETED"
	OperationStatusFailed     OperationStatus = "FAILED"
	OperationStatusPaused     OperationStatus = "PAUSED"
	OperationStatusCancelled  OperationStatus = "CANCELLED"
)

// DocumentPermission represents permissions on documents.
type DocumentPermission string

const (
	PermissionRead              DocumentPermission = "READ"
	PermissionWrite             DocumentPermission = "WRITE"
	PermissionDelete            DocumentPermission = "DELETE"
	PermissionCreate            DocumentPermission = "CREATE"
	PermissionRename            DocumentPermission = "RENAME"
	PermissionMove              DocumentPermission = "MOVE"
	PermissionCopy              DocumentPermission = "COPY"
	PermissionShare             DocumentPermission = "SHARE"
	PermissionManagePermissions DocumentPermission = "MANAGE_PERMISSIONS"
	PermissionViewMetadata      DocumentPermission = "VIEW_METADATA"
	PermissionModifyMetadata    DocumentPermission = "MODIFY_METADATA"
	PermissionExecute           DocumentPermission = "EXECUTE"
	PermissionDownload          DocumentPermission = "DOWNLOAD"
	PermissionUpload            DocumentPermission = "UPLOAD"
	PermissionSync              DocumentPermission = "SYNC"
	PermissionAdmin             DocumentPermission = "ADMIN"
)

// NetworkStorage represents a network storage location.
type NetworkStorage struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Type                StorageType       `json:"type"`
	Location            string            `json:"location"`
	TotalSpace          *int64            `json:"totalSpace,omitempty"`
	UsedSpace           *int64            `json:"usedSpace,omitempty"`
	IsOnline            bool              `json:"isOnline"`
	LastSync            *int64            `json:"lastSync,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	IsEnabled           bool              `json:"isEnabled"`
	Priority            int               `json:"priority"`
	SupportsFolders     bool              `json:"supportsFolders"`
	SupportsMetadata    bool              `json:"supportsMetadata"`
	MaxFileSize         *int64            `json:"maxFileSize,omitempty"`
	SupportedExtensions []string          `json:"supportedExtensions,omitempty"`
}

// NewNetworkStorage creates a NetworkStorage with defaults.
func NewNetworkStorage(id, name string, storageType StorageType, location string) *NetworkStorage {
	return &NetworkStorage{
		ID:               id,
		Name:             name,
		Type:             storageType,
		Location:         location,
		IsEnabled:        true,
		Priority:         100,
		SupportsFolders:  true,
		SupportsMetadata: true,
	}
}

// AvailableSpace returns available space or nil.
func (s *NetworkStorage) AvailableSpace() *int64 {
	if s.TotalSpace != nil && s.UsedSpace != nil {
		avail := *s.TotalSpace - *s.UsedSpace
		return &avail
	}
	return nil
}

// UsagePercentage returns usage as 0.0-1.0 or nil.
func (s *NetworkStorage) UsagePercentage() *float64 {
	if s.TotalSpace != nil && s.UsedSpace != nil && *s.TotalSpace > 0 {
		pct := float64(*s.UsedSpace) / float64(*s.TotalSpace)
		return &pct
	}
	return nil
}

// IsFull returns true if no space available.
func (s *NetworkStorage) IsFull() bool {
	avail := s.AvailableSpace()
	return avail != nil && *avail == 0
}

// IsLowOnSpace returns true if >90% used.
func (s *NetworkStorage) IsLowOnSpace() bool {
	pct := s.UsagePercentage()
	return pct != nil && *pct > 0.9
}

// NetworkDocument represents a file or folder on network storage.
type NetworkDocument struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Path        string               `json:"path"`
	IsFolder    bool                 `json:"isFolder"`
	Size        int64                `json:"size"`
	LastModified *int64              `json:"lastModified,omitempty"`
	SyncStatus  SyncStatus           `json:"syncStatus"`
	DocumentID  string               `json:"documentId,omitempty"`
	ContentType string               `json:"contentType,omitempty"`
	Extension   string               `json:"extension"`
	ParentPath  string               `json:"parentPath"`
	IsReadOnly  bool                 `json:"isReadOnly"`
	IsHidden    bool                 `json:"isHidden"`
	Metadata    map[string]string    `json:"metadata,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	Owner       string               `json:"owner,omitempty"`
	Permissions []DocumentPermission `json:"permissions,omitempty"`
	StorageID   string               `json:"storageId"`
	Author      string               `json:"author,omitempty"`
}

// CacheEntry represents a locally cached copy of a remote file.
type CacheEntry struct {
	ID               string            `json:"id"`
	RemoteDocumentID string            `json:"remoteDocumentId"`
	LocalPath        string            `json:"localPath"`
	RemotePath       string            `json:"remotePath"`
	Size             int64             `json:"size"`
	CreatedAt        int64             `json:"createdAt"`
	LastAccessed     int64             `json:"lastAccessed"`
	LastModified     int64             `json:"lastModified"`
	ExpiresAt        *int64            `json:"expiresAt,omitempty"`
	IsValid          bool              `json:"isValid"`
	IsPinned         bool              `json:"isPinned"`
	IsInUse          bool              `json:"isInUse"`
	AccessCount      int               `json:"accessCount"`
	ContentType      string            `json:"contentType,omitempty"`
	Checksum         string            `json:"checksum,omitempty"`
	Compression      string            `json:"compression,omitempty"`
	OriginalSize     *int64            `json:"originalSize,omitempty"`
	Priority         int               `json:"priority"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
}

// NewCacheEntry creates a CacheEntry with defaults.
func NewCacheEntry(id, remoteDocID, localPath, remotePath string, size, createdAt int64) *CacheEntry {
	return &CacheEntry{
		ID:               id,
		RemoteDocumentID: remoteDocID,
		LocalPath:        localPath,
		RemotePath:       remotePath,
		Size:             size,
		CreatedAt:        createdAt,
		LastAccessed:     createdAt,
		LastModified:     createdAt,
		IsValid:          true,
		Priority:         100,
	}
}

// CanBeEvicted returns true if not pinned and not in use.
func (c *CacheEntry) CanBeEvicted() bool {
	return !c.IsPinned && !c.IsInUse
}

// CompressionRatio returns the compression ratio or nil.
func (c *CacheEntry) CompressionRatio() *float64 {
	if c.Compression != "" && c.OriginalSize != nil && c.Size > 0 {
		ratio := float64(c.Size) / float64(*c.OriginalSize)
		return &ratio
	}
	return nil
}

// NetworkOperation represents an in-progress network operation.
type NetworkOperation struct {
	ID                     int64             `json:"id"`
	Type                   OperationType     `json:"type"`
	RemotePath             string            `json:"remotePath"`
	LocalPath              string            `json:"localPath,omitempty"`
	Status                 OperationStatus   `json:"status"`
	Progress               float64           `json:"progress"`
	TotalSize              int64             `json:"totalSize"`
	BytesTransferred       int64             `json:"bytesTransferred"`
	CreatedAt              int64             `json:"createdAt"`
	StartedAt              *int64            `json:"startedAt,omitempty"`
	CompletedAt            *int64            `json:"completedAt,omitempty"`
	Error                  string            `json:"error,omitempty"`
	RetryCount             int               `json:"retryCount"`
	MaxRetries             int               `json:"maxRetries"`
	Priority               int               `json:"priority"`
	CanPause               bool              `json:"canPause"`
	CanCancel              bool              `json:"canCancel"`
	IsPaused               bool              `json:"isPaused"`
	EstimatedTimeRemaining *int64            `json:"estimatedTimeRemaining,omitempty"`
	TransferSpeed          *int64            `json:"transferSpeed,omitempty"`
	Metadata               map[string]string `json:"metadata,omitempty"`
}

// NewNetworkOperation creates a NetworkOperation with defaults.
func NewNetworkOperation(id int64, opType OperationType, remotePath string, createdAt int64) *NetworkOperation {
	return &NetworkOperation{
		ID:         id,
		Type:       opType,
		RemotePath: remotePath,
		Status:     OperationStatusPending,
		CreatedAt:  createdAt,
		MaxRetries: 3,
		Priority:   100,
		CanPause:   true,
		CanCancel:  true,
	}
}

// IsRunning returns true if in progress and not paused.
func (op *NetworkOperation) IsRunning() bool {
	return op.Status == OperationStatusInProgress && !op.IsPaused
}

// IsCompleted returns true if completed.
func (op *NetworkOperation) IsCompleted() bool {
	return op.Status == OperationStatusCompleted
}

// HasFailed returns true if failed.
func (op *NetworkOperation) HasFailed() bool {
	return op.Status == OperationStatusFailed
}

// CanRetry returns true if failed and retries remain.
func (op *NetworkOperation) CanRetry() bool {
	return op.HasFailed() && op.RetryCount < op.MaxRetries
}

// ProgressPercentage returns progress as 0-100.
func (op *NetworkOperation) ProgressPercentage() int {
	return int(op.Progress * 100)
}

// Duration returns duration in millis or nil.
func (op *NetworkOperation) Duration() *int64 {
	if op.CompletedAt != nil && op.StartedAt != nil {
		d := *op.CompletedAt - *op.StartedAt
		return &d
	}
	return nil
}

// NetworkStorageDB defines the interface for network storage database operations.
type NetworkStorageDB interface {
	Initialize(ctx context.Context) error
	Close() error

	// Storage
	InsertStorage(ctx context.Context, storage *NetworkStorage) error
	UpdateStorage(ctx context.Context, storage *NetworkStorage) error
	GetStorage(ctx context.Context, id string) (*NetworkStorage, error)
	GetAllStorage(ctx context.Context) ([]*NetworkStorage, error)
	DeleteStorage(ctx context.Context, id string) error

	// Documents
	InsertDocument(ctx context.Context, doc *NetworkDocument) error
	UpdateDocument(ctx context.Context, doc *NetworkDocument) error
	GetDocument(ctx context.Context, id string) (*NetworkDocument, error)
	GetDocumentsByStorage(ctx context.Context, storageID string) ([]*NetworkDocument, error)
	GetDocumentsByPath(ctx context.Context, path string) ([]*NetworkDocument, error)
	DeleteDocument(ctx context.Context, id string) error

	// Cache
	InsertCacheEntry(ctx context.Context, entry *CacheEntry) error
	UpdateCacheEntry(ctx context.Context, entry *CacheEntry) error
	GetCacheEntry(ctx context.Context, id string) (*CacheEntry, error)
	GetCacheEntriesByDocument(ctx context.Context, docID string) ([]*CacheEntry, error)
	GetAllCacheEntries(ctx context.Context) ([]*CacheEntry, error)
	DeleteCacheEntry(ctx context.Context, id string) error
	DeleteExpiredCacheEntries(ctx context.Context) (int, error)
	GetCacheUsage(ctx context.Context) (int64, error)

	// Operations
	InsertOperation(ctx context.Context, op *NetworkOperation) error
	UpdateOperation(ctx context.Context, op *NetworkOperation) error
	GetOperation(ctx context.Context, id int64) (*NetworkOperation, error)
	GetActiveOperations(ctx context.Context) ([]*NetworkOperation, error)
	DeleteOperation(ctx context.Context, id int64) error
	ClearCompletedOperations(ctx context.Context) (int, error)

	// Sync status
	UpdateSyncStatus(ctx context.Context, remotePath string, status SyncStatus) error
	GetSyncStatus(ctx context.Context, remotePath string) (SyncStatus, error)
	GetAllSyncStatus(ctx context.Context) (map[string]SyncStatus, error)
	DeleteSyncStatus(ctx context.Context, remotePath string) error

	// Cleanup
	ClearAll(ctx context.Context) error
	Vacuum(ctx context.Context) error
}
