package api

// ============================================================
// 文档分类
// ============================================================

// DocCategoryCreateRequest POST /v1/doc-categories — 创建分类
type DocCategoryCreateRequest struct {
	Name      string `json:"name" binding:"required"`
	Slug      string `json:"slug" binding:"required"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	ParentID  *int   `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}

// DocCategoryUpdateRequest PUT /v1/doc-categories/:id — 更新分类
type DocCategoryUpdateRequest struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	ParentID  *int   `json:"parent_id"`
	SortOrder *int   `json:"sort_order"`
}

// DocCategoryItem 分类详情
type DocCategoryItem struct {
	ID        int                `json:"id"`
	Name      string             `json:"name"`
	Slug      string             `json:"slug"`
	Icon      string             `json:"icon"`
	Color     string             `json:"color"`
	ParentID  *int               `json:"parent_id"`
	SortOrder int                `json:"sort_order"`
	CreatedAt int64              `json:"created_at"`
	Children  []*DocCategoryItem `json:"children,omitempty"`
}

// ============================================================
// 文档
// ============================================================

// DocCreateRequest POST /v1/documents — 创建文档
type DocCreateRequest struct {
	Slug       string         `json:"slug" binding:"required"`
	Title      string         `json:"title" binding:"required"`
	CategoryID *int           `json:"category_id"`
	ContentKey string         `json:"content_key"`
	Format     string         `json:"format"`
	Language   string         `json:"language"`
	Author     string         `json:"author"`
	Status     string         `json:"status"`
	SortOrder  int            `json:"sort_order"`
	Metadata   map[string]any `json:"metadata"`
}

// DocUpdateRequest PUT /v1/documents/:id — 更新文档
type DocUpdateRequest struct {
	Slug       string         `json:"slug"`
	Title      string         `json:"title"`
	CategoryID *int           `json:"category_id"`
	ContentKey string         `json:"content_key"`
	Format     string         `json:"format"`
	Language   string         `json:"language"`
	Author     string         `json:"author"`
	Status     string         `json:"status"`
	SortOrder  *int           `json:"sort_order"`
	Metadata   map[string]any `json:"metadata"`
}

// DocItem 文档详情
type DocItem struct {
	ID           int            `json:"id"`
	Slug         string         `json:"slug"`
	Title        string         `json:"title"`
	CategoryID   *int           `json:"category_id"`
	CategoryName string         `json:"category_name,omitempty"`
	ContentKey   string         `json:"content_key"`
	Format       string         `json:"format"`
	Language     string         `json:"language"`
	Author       string         `json:"author"`
	Status       string         `json:"status"`
	SortOrder    int            `json:"sort_order"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    int64          `json:"created_at"`
	UpdatedAt    int64          `json:"updated_at"`
}

// DocListRequest GET /v1/documents 查询参数
type DocListRequest struct {
	CategoryID *int   `form:"category_id"`
	Status     string `form:"status"`
	Search     string `form:"search"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// DocListResponse GET /v1/documents 分页响应
type DocListResponse struct {
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Items    []DocItem `json:"items"`
}

// ============================================================
// 文档版本
// ============================================================

// DocVersionCreateRequest POST /v1/documents/:id/versions — 创建版本
type DocVersionCreateRequest struct {
	Version    string `json:"version" binding:"required"`
	ContentKey string `json:"content_key" binding:"required"`
	Changelog  string `json:"changelog"`
	CreatedBy  string `json:"created_by"`
}

// DocVersionItem 版本详情
type DocVersionItem struct {
	ID         int    `json:"id"`
	DocumentID int    `json:"document_id"`
	Version    string `json:"version"`
	ContentKey string `json:"content_key"`
	Changelog  string `json:"changelog"`
	CreatedBy  string `json:"created_by"`
	CreatedAt  int64  `json:"created_at"`
}

// ============================================================
// 文档附件
// ============================================================

// DocAttachmentCreateRequest POST /v1/documents/:id/attachments — 创建附件
type DocAttachmentCreateRequest struct {
	Filename    string `json:"filename" binding:"required"`
	StorageKey  string `json:"storage_key" binding:"required"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

// DocAttachmentItem 附件详情
type DocAttachmentItem struct {
	ID          int    `json:"id"`
	DocumentID  int    `json:"document_id"`
	Filename    string `json:"filename"`
	StorageKey  string `json:"storage_key"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	CreatedAt   int64  `json:"created_at"`
}

// ============================================================
// 文档反馈
// ============================================================

// DocFeedbackCreateRequest POST /v1/documents/:id/feedback — 创建反馈
type DocFeedbackCreateRequest struct {
	Type        string `json:"type" binding:"required"`
	Description string `json:"description" binding:"required"`
	Email       string `json:"email"`
}

// DocFeedbackUpdateRequest PATCH /v1/doc-feedback/:id — 更新反馈状态
type DocFeedbackUpdateRequest struct {
	Status string `json:"status" binding:"required"`
}

// DocFeedbackItem 反馈详情
type DocFeedbackItem struct {
	ID          int    `json:"id"`
	DocumentID  int    `json:"document_id"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Email       string `json:"email"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
}

// DocFeedbackListRequest GET /v1/doc-feedback 查询参数
type DocFeedbackListRequest struct {
	DocumentID *int   `form:"document_id"`
	Status     string `form:"status"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// DocFeedbackListResponse GET /v1/doc-feedback 分页响应
type DocFeedbackListResponse struct {
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Items    []DocFeedbackItem `json:"items"`
}

// ============================================================
// 版本 Diff
// ============================================================

// DocDiffResponse GET /v1/docs/:slug/diff 版本差异响应
type DocDiffResponse struct {
	FromVersion string         `json:"from_version"`
	ToVersion   string         `json:"to_version"`
	Stats       DocDiffStats   `json:"stats"`
	Changes     []DocDiffChunk `json:"changes"`
}

// DocDiffStats 差异统计
type DocDiffStats struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Equal   int `json:"equal"`
}

// DocDiffChunk 差异块
type DocDiffChunk struct {
	Type string `json:"type"` // "equal", "insert", "delete"
	Text string `json:"text"`
}

// ============================================================
// PDF 导出
// ============================================================

// DocExportHTMLResponse HTML 导出响应
type DocExportHTMLResponse struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	HTML    string `json:"html"`
	Version string `json:"version,omitempty"`
}

// ============================================================
// 反馈统计
// ============================================================

// DocFeedbackStatsItem 按文档统计反馈数量
type DocFeedbackStatsItem struct {
	DocumentID   int    `json:"document_id"`
	DocumentSlug string `json:"document_slug"`
	Title        string `json:"title"`
	Total        int    `json:"total"`
	Pending      int    `json:"pending"`
	Resolved     int    `json:"resolved"`
	Rejected     int    `json:"rejected"`
}
