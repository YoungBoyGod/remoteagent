package search

import (
	"log"

	"github.com/meilisearch/meilisearch-go"
)

const IndexName = "documents"

// Client Meilisearch 客户端封装
type Client struct {
	ms meilisearch.ServiceManager
}

// NewClient 创建 Meilisearch 客户端
func NewClient(url, masterKey string) *Client {
	ms := meilisearch.New(url, meilisearch.WithAPIKey(masterKey))
	return &Client{ms: ms}
}

// EnsureIndex 确保索引存在并配置好字段设置
func (c *Client) EnsureIndex() error {
	_, err := c.ms.CreateIndex(&meilisearch.IndexConfig{
		Uid:        IndexName,
		PrimaryKey: "id",
	})
	if err != nil {
		// 索引已存在不算错误
		log.Printf("[search] create index (may already exist): %v", err)
	}

	idx := c.ms.Index(IndexName)

	// 可搜索字段
	_, err = idx.UpdateSearchableAttributes(&[]string{
		"title", "content", "category_name", "author",
	})
	if err != nil {
		return err
	}

	// 可筛选字段
	filterAttrs := []interface{}{"category_id", "language", "status"}
	_, err = idx.UpdateFilterableAttributes(&filterAttrs)
	if err != nil {
		return err
	}

	// 可排序字段
	_, err = idx.UpdateSortableAttributes(&[]string{
		"created_at", "updated_at", "sort_order",
	})
	if err != nil {
		return err
	}

	log.Printf("[search] index '%s' configured", IndexName)
	return nil
}

// Healthy 检查 Meilisearch 是否可用
func (c *Client) Healthy() bool {
	return c.ms.IsHealthy()
}

// SearchDoc Meilisearch 中的文档结构
type SearchDoc struct {
	ID           int            `json:"id"`
	Slug         string         `json:"slug"`
	Title        string         `json:"title"`
	Content      string         `json:"content"`
	CategoryID   *int           `json:"category_id"`
	CategoryName string         `json:"category_name"`
	Format       string         `json:"format"`
	Language     string         `json:"language"`
	Author       string         `json:"author"`
	Status       string         `json:"status"`
	SortOrder    int            `json:"sort_order"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    int64          `json:"created_at"`
	UpdatedAt    int64          `json:"updated_at"`
}

// SearchHit 搜索结果条目
type SearchHit struct {
	ID         int    `json:"id"`
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	Snippet    string `json:"snippet"`
	Category   string `json:"category"`
	CategoryID *int   `json:"category_id,omitempty"`
	Author     string `json:"author"`
	Status     string `json:"status"`
	UpdatedAt  int64  `json:"updated_at"`
}

// SearchResult 搜索响应
type SearchResult struct {
	Hits             []SearchHit `json:"hits"`
	Total            int64       `json:"total"`
	Query            string      `json:"query"`
	ProcessingTimeMs int64       `json:"processing_time_ms"`
}

// SuggestResult 搜索建议响应
type SuggestResult struct {
	Suggestions []string `json:"suggestions"`
	Query       string   `json:"query"`
}

// hitMap 用于从 Meilisearch Hit 中解码的中间结构
type hitMap struct {
	ID           int                    `json:"id"`
	Slug         string                 `json:"slug"`
	Title        string                 `json:"title"`
	Content      string                 `json:"content"`
	CategoryID   *int                   `json:"category_id"`
	CategoryName string                 `json:"category_name"`
	Author       string                 `json:"author"`
	Status       string                 `json:"status"`
	UpdatedAt    int64                  `json:"updated_at"`
	Formatted    map[string]interface{} `json:"_formatted"`
}
