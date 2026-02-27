package api

// ============================================================
// 安全分发管理
// ============================================================

// DistributionCreateRequest POST /v1/distributions — 创建分发记录
type DistributionCreateRequest struct {
	FileName       string `json:"file_name" binding:"required"`
	FileSize       int64  `json:"file_size"`
	SHA256Original string `json:"sha256_original"`
	EncryptionAlgo string `json:"encryption_algo"` // 默认 AES-256
	CustomerName   string `json:"customer_name"`
	CustomerEmail  string `json:"customer_email" binding:"required,email"`
	ReleaseNotes   string `json:"release_notes"`
	SourceType     string `json:"source_type"` // s3 | local
	S3Key          string `json:"s3_key"`
	ScheduledAt    *int64 `json:"scheduled_at,omitempty"`
}

// DistributionUpdateRequest PUT /v1/distributions/:id — 更新分发记录
type DistributionUpdateRequest struct {
	EncryptedFilePath string  `json:"encrypted_file_path"`
	SHA256Encrypted   string  `json:"sha256_encrypted"`
	SessionKeyHash    string  `json:"session_key_hash"`
	PresignedURL      string  `json:"presigned_url"`
	URLExpiresAt      *int64  `json:"url_expires_at"`
	ReleaseNotes      string  `json:"release_notes"`
	CustomerName      string  `json:"customer_name"`
	CustomerEmail     string  `json:"customer_email"`
}

// DistributionStatusRequest PATCH /v1/distributions/:id/status — 更新状态
type DistributionStatusRequest struct {
	Status     string `json:"status" binding:"required"`
	DownloadIP string `json:"download_ip"`
}

// DistributionItem 分发记录详情
type DistributionItem struct {
	ID                int64  `json:"id"`
	TaskID            string `json:"task_id"`
	FileName          string `json:"file_name"`
	FileSize          int64  `json:"file_size"`
	EncryptedFilePath string `json:"encrypted_file_path,omitempty"`
	SHA256Original    string `json:"sha256_original"`
	SHA256Encrypted   string `json:"sha256_encrypted,omitempty"`
	EncryptionAlgo    string `json:"encryption_algo"`
	CustomerName      string `json:"customer_name"`
	CustomerEmail     string `json:"customer_email"`
	SessionKeyHash    string `json:"session_key_hash,omitempty"`
	PresignedURL      string `json:"presigned_url,omitempty"`
	URLExpiresAt      *int64 `json:"url_expires_at,omitempty"`
	Status            string `json:"status"`
	DownloadIP        string `json:"download_ip,omitempty"`
	DownloadAt        *int64 `json:"download_at,omitempty"`
	ReleaseNotes      string `json:"release_notes,omitempty"`
	ScheduledAt       *int64 `json:"scheduled_at,omitempty"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

// DistributionListRequest GET /v1/distributions 查询参数
type DistributionListRequest struct {
	Status   string `form:"status"`
	Search   string `form:"search"`
	SortBy   string `form:"sort_by"`   // created_at, file_name, status
	SortDir  string `form:"sort_dir"`  // asc, desc
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// DistributionListResponse GET /v1/distributions 分页响应
type DistributionListResponse struct {
	Total    int                `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Items    []DistributionItem `json:"items"`
}

// DistributionS3ListRequest GET /v1/distributions/s3-objects 查询参数
type DistributionS3ListRequest struct {
	Prefix            string `form:"prefix"`
	PageSize          int    `form:"page_size"`
	ContinuationToken string `form:"continuation_token"`
}

// DistributionS3ObjectItem S3 对象元信息
type DistributionS3ObjectItem struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified int64  `json:"last_modified"`
}

// DistributionS3ListResponse GET /v1/distributions/s3-objects 响应
type DistributionS3ListResponse struct {
	Items     []DistributionS3ObjectItem `json:"items"`
	NextToken string                     `json:"next_token,omitempty"`
	HasMore   bool                       `json:"has_more"`
}


// ReleaseNoteCreateRequest POST /v1/release-notes
type ReleaseNoteCreateRequest struct {
	Title     string `json:"title" binding:"required"`
	Content   string `json:"content"`
	Version   string `json:"version"`
	CreatedBy string `json:"created_by"`
}

// ReleaseNoteUpdateRequest PUT /v1/release-notes/:id
type ReleaseNoteUpdateRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Version string `json:"version"`
}

// ReleaseNoteItem 发布说明详情
type ReleaseNoteItem struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Version   string `json:"version"`
	CreatedBy string `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ReleaseNoteListRequest GET /v1/release-notes
type ReleaseNoteListRequest struct {
	Search   string `form:"search"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// ReleaseNoteListResponse GET /v1/release-notes 分页响应
type ReleaseNoteListResponse struct {
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Items    []ReleaseNoteItem `json:"items"`
}
