package model

import "time"

const (
	FileUploading      = 0
	FileComplete       = 1
	FileDeleted        = 2
	UploadInit         = 0
	UploadUploading    = 1
	UploadComplete     = 2
	UploadFailed       = 3
	UploadPaused       = 4
	UploadCancelled    = 5
	UploadFinalizing   = 6
	ModeSingle         = 1
	ModeMultipart      = 2
	AnalysisProcessing = 0
	AnalysisComplete   = 1
	AnalysisFailed     = 2
)

type FileObject struct {
	ID          uint64  `gorm:"primaryKey"`
	FileHash    string  `gorm:"size:64;uniqueIndex;not null"`
	FileName    string  `gorm:"size:512;not null"`
	RegionCode  *string `gorm:"size:32"`
	BlockIndex  *uint32
	DataYear    *uint32
	FileSize    int64
	Bucket      string `gorm:"size:255;not null"`
	StoragePath string `gorm:"size:1024;not null"`
	Status      int8
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (FileObject) TableName() string { return "file_object" }

type UserFile struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64
	FileID    uint64
	Filename  string `gorm:"size:512;not null"`
	Folder    string `gorm:"size:1024;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (UserFile) TableName() string { return "user_file" }

type UploadSession struct {
	ID           uint64 `gorm:"primaryKey"`
	UserID       uint64
	FileHash     string `gorm:"size:64;uniqueIndex;not null"`
	Filename     string `gorm:"size:512;not null"`
	Folder       string `gorm:"size:1024;not null"`
	FileSize     int64
	ChunkSize    int
	ChunkCount   int
	UploadMode   int8
	CephUploadID string `gorm:"size:255"`
	Bucket       string `gorm:"size:255;not null"`
	StoragePath  string `gorm:"size:1024;not null"`
	Status       int8
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UploadSession) TableName() string { return "upload_session" }

type AnalysisResult struct {
	ID              uint64 `gorm:"primaryKey"`
	FileID          uint64
	AnalysisType    string  `gorm:"size:32;not null"`
	ParametersJSON  string  `gorm:"type:json;not null"`
	CacheKey        string  `gorm:"size:64;uniqueIndex;not null"`
	CacheVersion    string  `gorm:"size:128;not null"`
	Model           string  `gorm:"size:128;not null"`
	ResultJSON      *string `gorm:"type:json"`
	Provider        string  `gorm:"size:32;not null"`
	Status          int8
	LeaseToken      string `gorm:"size:36;not null"`
	LeaseExpiresAt  *time.Time
	ExpiresAt       *time.Time
	ErrorMessage    string `gorm:"size:1024;not null"`
	CreatedByUserID uint64
	HitCount        uint64
	LastAccessedAt  time.Time
	GeneratedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (AnalysisResult) TableName() string { return "analysis_result" }
