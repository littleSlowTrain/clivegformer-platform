package model

type UploadCompleteEvent struct {
	EventID     string `json:"event_id"`
	UploadID    uint64 `json:"upload_id"`
	FileHash    string `json:"file_hash"`
	StoragePath string `json:"storage_path"`
}
