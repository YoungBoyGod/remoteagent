package model

import "time"

// DocCategory 文档分类
type DocCategory struct {
	ID        int
	Name      string
	Slug      string
	Icon      string
	Color     string
	ParentID  *int
	SortOrder int
	CreatedAt time.Time
}

// Document 文档
type Document struct {
	ID         int
	Slug       string
	Title      string
	CategoryID *int
	ContentKey string
	Format     string
	Language   string
	Author     string
	Status     string
	SortOrder  int
	Metadata   map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// DocVersion 文档版本
type DocVersion struct {
	ID         int
	DocumentID int
	Version    string
	ContentKey string
	Changelog  string
	CreatedBy  string
	CreatedAt  time.Time
}

// DocAttachment 文档附件
type DocAttachment struct {
	ID          int
	DocumentID  int
	Filename    string
	StorageKey  string
	ContentType string
	SizeBytes   int64
	CreatedAt   time.Time
}

// DocFeedback 文档反馈
type DocFeedback struct {
	ID          int
	DocumentID  int
	Type        string
	Description string
	Email       string
	Status      string
	CreatedAt   time.Time
}
