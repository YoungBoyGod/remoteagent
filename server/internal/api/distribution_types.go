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
	CustomerEmail  string `json:"customer_email"`
	ReleaseNotes   string `json:"release_notes"`
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
