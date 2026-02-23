package api

// ============================================================
// 客户管理
// ============================================================

// CustomerCreateRequest POST /v1/customers — 创建客户
type CustomerCreateRequest struct {
	Name        string   `json:"name" binding:"required"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Company     string   `json:"company"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// CustomerUpdateRequest PUT /v1/customers/:id — 更新客户
type CustomerUpdateRequest struct {
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Company     string   `json:"company"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
}

// CustomerItem 客户详情
type CustomerItem struct {
	CustomerID  string   `json:"customer_id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Company     string   `json:"company"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
	HostCount   int      `json:"host_count"`
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
}

// CustomerListRequest GET /v1/customers 查询参数
type CustomerListRequest struct {
	Status   string `form:"status"`
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// CustomerListResponse GET /v1/customers 分页响应
type CustomerListResponse struct {
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Items    []CustomerItem `json:"items"`
}

// CustomerHostAssignRequest POST /v1/customers/:id/hosts — 分配主机
type CustomerHostAssignRequest struct {
	HostID string `json:"host_id" binding:"required"`
	Note   string `json:"note"`
}

// CustomerHostItem 客户已分配主机详情
type CustomerHostItem struct {
	HostID     string `json:"host_id"`
	HostName   string `json:"host_name"`
	IP         string `json:"ip"`
	Hostname   string `json:"hostname"`
	Status     string `json:"status"`
	AssignedAt int64  `json:"assigned_at"`
	Note       string `json:"note,omitempty"`
}

// CustomerHostListResponse GET /v1/customers/:id/hosts 响应
type CustomerHostListResponse struct {
	Items []CustomerHostItem `json:"items"`
}

// ============================================================
// 操作日志
// ============================================================

// OperationLogItem 操作日志详情
type OperationLogItem struct {
	LogID        int    `json:"log_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Action       string `json:"action"`
	Operator     string `json:"operator"`
	Detail       any    `json:"detail"`
	CreatedAt    int64  `json:"created_at"`
}

// OperationLogListRequest GET /v1/operation-logs 查询参数
type OperationLogListRequest struct {
	ResourceType string `form:"resource_type"`
	ResourceID   string `form:"resource_id"`
	Action       string `form:"action"`
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
}

// OperationLogListResponse GET /v1/operation-logs 分页响应
type OperationLogListResponse struct {
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Items    []OperationLogItem `json:"items"`
}
