package search

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// IndexDocument 索引单个文档
func (c *Client) IndexDocument(doc SearchDoc) error {
	idx := c.ms.Index(IndexName)
	_, err := idx.AddDocuments([]SearchDoc{doc}, nil)
	return err
}

// IndexDocuments 批量索引文档
func (c *Client) IndexDocuments(docs []SearchDoc) error {
	if len(docs) == 0 {
		return nil
	}
	idx := c.ms.Index(IndexName)
	_, err := idx.AddDocuments(docs, nil)
	return err
}

// DeleteDocument 从索引中删除文档
func (c *Client) DeleteDocument(id int) error {
	idx := c.ms.Index(IndexName)
	_, err := idx.DeleteDocument(fmt.Sprintf("%d", id), nil)
	return err
}

// RebuildIndex 全量重建索引（从数据库读取所有文档）
func (c *Client) RebuildIndex(db *sql.DB) error {
	log.Printf("[search] rebuilding index...")

	// 先清空索引
	idx := c.ms.Index(IndexName)
	_, err := idx.DeleteAllDocuments(nil)
	if err != nil {
		return fmt.Errorf("delete all documents: %w", err)
	}

	// 从数据库读取所有文档
	rows, err := db.Query(
		`SELECT d.id, d.slug, d.title, d.content_key, d.category_id, COALESCE(c.name,''),
		        d.format, d.language, d.author, d.status, d.sort_order,
		        COALESCE(d.metadata,'{}'), d.created_at, d.updated_at
		 FROM documents d
		 LEFT JOIN doc_categories c ON d.category_id = c.id
		 ORDER BY d.id`)
	if err != nil {
		return fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	var docs []SearchDoc
	for rows.Next() {
		var doc SearchDoc
		var categoryID sql.NullInt64
		var metaStr string
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&doc.ID, &doc.Slug, &doc.Title, &doc.Content, &categoryID, &doc.CategoryName,
			&doc.Format, &doc.Language, &doc.Author, &doc.Status, &doc.SortOrder,
			&metaStr, &createdAt, &updatedAt,
		)
		if err != nil {
			return fmt.Errorf("scan row: %w", err)
		}

		if categoryID.Valid {
			v := int(categoryID.Int64)
			doc.CategoryID = &v
		}
		json.Unmarshal([]byte(metaStr), &doc.Metadata)
		doc.CreatedAt = createdAt.Unix()
		doc.UpdatedAt = updatedAt.Unix()

		docs = append(docs, doc)
	}

	if len(docs) > 0 {
		if err := c.IndexDocuments(docs); err != nil {
			return fmt.Errorf("index documents: %w", err)
		}
	}

	log.Printf("[search] rebuild complete, indexed %d documents", len(docs))
	return nil
}

// Search 全文搜索文档
func (c *Client) Search(query string, categoryID *int, language string, status string, page, pageSize int) (*SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 构建筛选条件
	filters := []string{}
	if categoryID != nil {
		filters = append(filters, fmt.Sprintf("category_id = %d", *categoryID))
	}
	if language != "" {
		filters = append(filters, fmt.Sprintf("language = '%s'", language))
	}
	if status != "" {
		filters = append(filters, fmt.Sprintf("status = '%s'", status))
	}

	filterStr := ""
	for i, f := range filters {
		if i > 0 {
			filterStr += " AND "
		}
		filterStr += f
	}

	searchReq := &meilisearch.SearchRequest{
		Limit:                 int64(pageSize),
		Offset:                int64((page - 1) * pageSize),
		AttributesToHighlight: []string{"title", "content"},
		HighlightPreTag:       "<mark>",
		HighlightPostTag:      "</mark>",
		AttributesToCrop:      []string{"content"},
		CropLength:            80,
	}
	if filterStr != "" {
		searchReq.Filter = filterStr
	}

	idx := c.ms.Index(IndexName)
	resp, err := idx.Search(query, searchReq)
	if err != nil {
		return nil, err
	}

	hits := make([]SearchHit, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		var m hitMap
		if err := h.DecodeInto(&m); err != nil {
			continue
		}

		hit := SearchHit{
			ID:         m.ID,
			Slug:       m.Slug,
			Title:      m.Title,
			Category:   m.CategoryName,
			CategoryID: m.CategoryID,
			Author:     m.Author,
			Status:     m.Status,
			UpdatedAt:  m.UpdatedAt,
		}

		// 高亮片段（从 _formatted 字段提取）
		if m.Formatted != nil {
			if snippet, ok := m.Formatted["content"].(string); ok && snippet != "" {
				hit.Snippet = snippet
			}
			if title, ok := m.Formatted["title"].(string); ok && title != "" {
				hit.Title = title
			}
		}

		hits = append(hits, hit)
	}

	return &SearchResult{
		Hits:             hits,
		Total:            resp.EstimatedTotalHits,
		Query:            query,
		ProcessingTimeMs: resp.ProcessingTimeMs,
	}, nil
}

// Suggest 搜索建议（自动补全）
func (c *Client) Suggest(query string, limit int) (*SuggestResult, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	idx := c.ms.Index(IndexName)
	resp, err := idx.Search(query, &meilisearch.SearchRequest{
		Limit:                int64(limit),
		AttributesToRetrieve: []string{"title"},
	})
	if err != nil {
		return nil, err
	}

	suggestions := make([]string, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		var m hitMap
		if err := h.DecodeInto(&m); err != nil {
			continue
		}
		if m.Title != "" {
			suggestions = append(suggestions, m.Title)
		}
	}

	return &SuggestResult{
		Suggestions: suggestions,
		Query:       query,
	}, nil
}
