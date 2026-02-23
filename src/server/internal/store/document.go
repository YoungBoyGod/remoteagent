package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"luoyi2026/server/internal/api"
)

// ============================================================
// 文档分类 CRUD
// ============================================================

// InsertDocCategory 创建文档分类
func InsertDocCategory(db *sql.DB, req api.DocCategoryCreateRequest) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int
	err := db.QueryRowContext(ctx,
		`INSERT INTO doc_categories (name, slug, icon, color, parent_id, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		req.Name, req.Slug, req.Icon, req.Color, req.ParentID, req.SortOrder,
	).Scan(&id)
	return id, err
}

// UpdateDocCategory 更新文档分类
func UpdateDocCategory(db *sql.DB, id int, req api.DocCategoryUpdateRequest) error {
	sets := []string{}
	args := []any{}
	idx := 1

	if req.Name != "" {
		sets = append(sets, fmt.Sprintf("name = $%d", idx))
		args = append(args, req.Name)
		idx++
	}
	if req.Slug != "" {
		sets = append(sets, fmt.Sprintf("slug = $%d", idx))
		args = append(args, req.Slug)
		idx++
	}
	if req.Icon != "" {
		sets = append(sets, fmt.Sprintf("icon = $%d", idx))
		args = append(args, req.Icon)
		idx++
	}
	if req.Color != "" {
		sets = append(sets, fmt.Sprintf("color = $%d", idx))
		args = append(args, req.Color)
		idx++
	}
	if req.ParentID != nil {
		sets = append(sets, fmt.Sprintf("parent_id = $%d", idx))
		args = append(args, *req.ParentID)
		idx++
	}
	if req.SortOrder != nil {
		sets = append(sets, fmt.Sprintf("sort_order = $%d", idx))
		args = append(args, *req.SortOrder)
		idx++
	}

	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE doc_categories SET %s WHERE id = $%d", strings.Join(sets, ", "), idx)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("category not found")
	}
	return nil
}

// DeleteDocCategory 删除文档分类
func DeleteDocCategory(db *sql.DB, id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, "DELETE FROM doc_categories WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("category not found")
	}
	return nil
}

// GetDocCategory 获取单个分类
func GetDocCategory(db *sql.DB, id int) (*api.DocCategoryItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var item api.DocCategoryItem
	var parentID sql.NullInt64
	var createdAt time.Time

	err := db.QueryRowContext(ctx,
		`SELECT id, name, slug, icon, color, parent_id, sort_order, created_at
		 FROM doc_categories WHERE id = $1`, id,
	).Scan(&item.ID, &item.Name, &item.Slug, &item.Icon, &item.Color, &parentID, &item.SortOrder, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if parentID.Valid {
		v := int(parentID.Int64)
		item.ParentID = &v
	}
	item.CreatedAt = createdAt.Unix()
	return &item, nil
}

// ListDocCategoryTree 查询分类树
func ListDocCategoryTree(db *sql.DB) ([]*api.DocCategoryItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT id, name, slug, icon, color, parent_id, sort_order, created_at
		 FROM doc_categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	all := []*api.DocCategoryItem{}
	for rows.Next() {
		var item api.DocCategoryItem
		var parentID sql.NullInt64
		var createdAt time.Time

		err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Icon, &item.Color, &parentID, &item.SortOrder, &createdAt)
		if err != nil {
			return nil, err
		}
		if parentID.Valid {
			v := int(parentID.Int64)
			item.ParentID = &v
		}
		item.CreatedAt = createdAt.Unix()
		all = append(all, &item)
	}

	// 构建树形结构
	byID := map[int]*api.DocCategoryItem{}
	for _, c := range all {
		byID[c.ID] = c
	}

	var roots []*api.DocCategoryItem
	for _, c := range all {
		if c.ParentID != nil {
			parent, ok := byID[*c.ParentID]
			if ok {
				parent.Children = append(parent.Children, c)
				continue
			}
		}
		roots = append(roots, c)
	}
	return roots, nil
}

// ============================================================
// 文档 CRUD
// ============================================================

// InsertDocument 创建文档
func InsertDocument(db *sql.DB, req api.DocCreateRequest) (int, error) {
	format := req.Format
	if format == "" {
		format = "markdown"
	}
	language := req.Language
	if language == "" {
		language = "zh"
	}
	author := req.Author
	if author == "" {
		author = "admin"
	}
	status := req.Status
	if status == "" {
		status = "draft"
	}
	meta, _ := json.Marshal(req.Metadata)
	if req.Metadata == nil {
		meta = []byte("{}")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int
	err := db.QueryRowContext(ctx,
		`INSERT INTO documents (slug, title, category_id, content_key, format, language, author, status, sort_order, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		 RETURNING id`,
		req.Slug, req.Title, req.CategoryID, req.ContentKey,
		format, language, author, status, req.SortOrder, string(meta),
	).Scan(&id)
	return id, err
}

// UpdateDocument 更新文档
func UpdateDocument(db *sql.DB, id int, req api.DocUpdateRequest) error {
	sets := []string{}
	args := []any{}
	idx := 1

	if req.Slug != "" {
		sets = append(sets, fmt.Sprintf("slug = $%d", idx))
		args = append(args, req.Slug)
		idx++
	}
	if req.Title != "" {
		sets = append(sets, fmt.Sprintf("title = $%d", idx))
		args = append(args, req.Title)
		idx++
	}
	if req.CategoryID != nil {
		sets = append(sets, fmt.Sprintf("category_id = $%d", idx))
		args = append(args, *req.CategoryID)
		idx++
	}
	if req.ContentKey != "" {
		sets = append(sets, fmt.Sprintf("content_key = $%d", idx))
		args = append(args, req.ContentKey)
		idx++
	}
	if req.Format != "" {
		sets = append(sets, fmt.Sprintf("format = $%d", idx))
		args = append(args, req.Format)
		idx++
	}
	if req.Language != "" {
		sets = append(sets, fmt.Sprintf("language = $%d", idx))
		args = append(args, req.Language)
		idx++
	}
	if req.Author != "" {
		sets = append(sets, fmt.Sprintf("author = $%d", idx))
		args = append(args, req.Author)
		idx++
	}
	if req.Status != "" {
		sets = append(sets, fmt.Sprintf("status = $%d", idx))
		args = append(args, req.Status)
		idx++
	}
	if req.SortOrder != nil {
		sets = append(sets, fmt.Sprintf("sort_order = $%d", idx))
		args = append(args, *req.SortOrder)
		idx++
	}
	if req.Metadata != nil {
		meta, _ := json.Marshal(req.Metadata)
		sets = append(sets, fmt.Sprintf("metadata = $%d::jsonb", idx))
		args = append(args, string(meta))
		idx++
	}

	if len(sets) == 0 {
		return nil
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE documents SET %s WHERE id = $%d", strings.Join(sets, ", "), idx)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

// DeleteDocument 删除文档
func DeleteDocument(db *sql.DB, id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, "DELETE FROM documents WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

// GetDocument 获取单个文档详情
func GetDocument(db *sql.DB, id int) (*api.DocItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var item api.DocItem
	var categoryID sql.NullInt64
	var categoryName sql.NullString
	var metaStr string
	var createdAt, updatedAt time.Time

	err := db.QueryRowContext(ctx,
		`SELECT d.id, d.slug, d.title, d.category_id, c.name,
		        d.content_key, d.format, d.language, d.author, d.status,
		        d.sort_order, COALESCE(d.metadata,'{}'), d.created_at, d.updated_at
		 FROM documents d
		 LEFT JOIN doc_categories c ON d.category_id = c.id
		 WHERE d.id = $1`, id,
	).Scan(
		&item.ID, &item.Slug, &item.Title, &categoryID, &categoryName,
		&item.ContentKey, &item.Format, &item.Language, &item.Author, &item.Status,
		&item.SortOrder, &metaStr, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if categoryID.Valid {
		v := int(categoryID.Int64)
		item.CategoryID = &v
	}
	if categoryName.Valid {
		item.CategoryName = categoryName.String
	}
	json.Unmarshal([]byte(metaStr), &item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.CreatedAt = createdAt.Unix()
	item.UpdatedAt = updatedAt.Unix()
	return &item, nil
}

// ListDocuments 分页查询文档列表
func ListDocuments(db *sql.DB, req api.DocListRequest) (*api.DocListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	where := []string{"1=1"}
	args := []any{}
	idx := 1

	if req.CategoryID != nil {
		where = append(where, fmt.Sprintf("d.category_id = $%d", idx))
		args = append(args, *req.CategoryID)
		idx++
	}
	if req.Status != "" {
		where = append(where, fmt.Sprintf("d.status = $%d", idx))
		args = append(args, req.Status)
		idx++
	}
	if req.Search != "" {
		where = append(where, fmt.Sprintf("(d.title ILIKE $%d OR d.slug ILIKE $%d)", idx, idx))
		args = append(args, "%"+req.Search+"%")
		idx++
	}

	whereClause := strings.Join(where, " AND ")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents d WHERE "+whereClause, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	query := fmt.Sprintf(
		`SELECT d.id, d.slug, d.title, d.category_id, c.name,
		        d.content_key, d.format, d.language, d.author, d.status,
		        d.sort_order, COALESCE(d.metadata,'{}'), d.created_at, d.updated_at
		 FROM documents d
		 LEFT JOIN doc_categories c ON d.category_id = c.id
		 WHERE %s
		 ORDER BY d.sort_order, d.created_at DESC
		 LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.DocItem{}
	for rows.Next() {
		var item api.DocItem
		var categoryID sql.NullInt64
		var categoryName sql.NullString
		var metaStr string
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&item.ID, &item.Slug, &item.Title, &categoryID, &categoryName,
			&item.ContentKey, &item.Format, &item.Language, &item.Author, &item.Status,
			&item.SortOrder, &metaStr, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		if categoryID.Valid {
			v := int(categoryID.Int64)
			item.CategoryID = &v
		}
		if categoryName.Valid {
			item.CategoryName = categoryName.String
		}
		json.Unmarshal([]byte(metaStr), &item.Metadata)
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		item.CreatedAt = createdAt.Unix()
		item.UpdatedAt = updatedAt.Unix()

		items = append(items, item)
	}

	return &api.DocListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

// GetDocumentBySlug 按 slug 查询文档
func GetDocumentBySlug(db *sql.DB, slug string) (*api.DocItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var item api.DocItem
	var categoryID sql.NullInt64
	var categoryName sql.NullString
	var metaStr string
	var createdAt, updatedAt time.Time

	err := db.QueryRowContext(ctx,
		`SELECT d.id, d.slug, d.title, d.category_id, c.name,
		        d.content_key, d.format, d.language, d.author, d.status,
		        d.sort_order, COALESCE(d.metadata,'{}'), d.created_at, d.updated_at
		 FROM documents d
		 LEFT JOIN doc_categories c ON d.category_id = c.id
		 WHERE d.slug = $1`, slug,
	).Scan(
		&item.ID, &item.Slug, &item.Title, &categoryID, &categoryName,
		&item.ContentKey, &item.Format, &item.Language, &item.Author, &item.Status,
		&item.SortOrder, &metaStr, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if categoryID.Valid {
		v := int(categoryID.Int64)
		item.CategoryID = &v
	}
	if categoryName.Valid {
		item.CategoryName = categoryName.String
	}
	json.Unmarshal([]byte(metaStr), &item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.CreatedAt = createdAt.Unix()
	item.UpdatedAt = updatedAt.Unix()
	return &item, nil
}

// ============================================================
// 文档版本
// ============================================================

// InsertDocVersion 创建文档版本
func InsertDocVersion(db *sql.DB, documentID int, req api.DocVersionCreateRequest) (int, error) {
	createdBy := req.CreatedBy
	if createdBy == "" {
		createdBy = "admin"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int
	err := db.QueryRowContext(ctx,
		`INSERT INTO doc_versions (document_id, version, content_key, changelog, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		documentID, req.Version, req.ContentKey, req.Changelog, createdBy,
	).Scan(&id)
	return id, err
}

// ListDocVersions 按文档查询版本列表
func ListDocVersions(db *sql.DB, documentID int) ([]api.DocVersionItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT id, document_id, version, content_key, changelog, created_by, created_at
		 FROM doc_versions
		 WHERE document_id = $1
		 ORDER BY version DESC`, documentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.DocVersionItem{}
	for rows.Next() {
		var item api.DocVersionItem
		var createdAt time.Time
		err := rows.Scan(&item.ID, &item.DocumentID, &item.Version, &item.ContentKey, &item.Changelog, &item.CreatedBy, &createdAt)
		if err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.Unix()
		items = append(items, item)
	}
	return items, nil
}

// ============================================================
// 文档附件
// ============================================================

// InsertDocAttachment 创建文档附件
func InsertDocAttachment(db *sql.DB, documentID int, req api.DocAttachmentCreateRequest) (int, error) {
	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int
	err := db.QueryRowContext(ctx,
		`INSERT INTO doc_attachments (document_id, filename, storage_key, content_type, size_bytes)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		documentID, req.Filename, req.StorageKey, contentType, req.SizeBytes,
	).Scan(&id)
	return id, err
}

// ListDocAttachments 按文档查询附件列表
func ListDocAttachments(db *sql.DB, documentID int) ([]api.DocAttachmentItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT id, document_id, filename, storage_key, content_type, size_bytes, created_at
		 FROM doc_attachments
		 WHERE document_id = $1
		 ORDER BY created_at DESC`, documentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.DocAttachmentItem{}
	for rows.Next() {
		var item api.DocAttachmentItem
		var createdAt time.Time
		err := rows.Scan(&item.ID, &item.DocumentID, &item.Filename, &item.StorageKey, &item.ContentType, &item.SizeBytes, &createdAt)
		if err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.Unix()
		items = append(items, item)
	}
	return items, nil
}

// DeleteDocAttachment 删除文档附件
func DeleteDocAttachment(db *sql.DB, id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, "DELETE FROM doc_attachments WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("attachment not found")
	}
	return nil
}

// GetDocAttachment 获取单个附件
func GetDocAttachment(db *sql.DB, id int) (*api.DocAttachmentItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var item api.DocAttachmentItem
	var createdAt time.Time
	err := db.QueryRowContext(ctx,
		`SELECT id, document_id, filename, storage_key, content_type, size_bytes, created_at
		 FROM doc_attachments WHERE id = $1`, id,
	).Scan(&item.ID, &item.DocumentID, &item.Filename, &item.StorageKey, &item.ContentType, &item.SizeBytes, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.CreatedAt = createdAt.Unix()
	return &item, nil
}

// ============================================================
// 文档反馈
// ============================================================

// InsertDocFeedback 创建文档反馈
func InsertDocFeedback(db *sql.DB, documentID int, req api.DocFeedbackCreateRequest) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int
	err := db.QueryRowContext(ctx,
		`INSERT INTO doc_feedback (document_id, type, description, email)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		documentID, req.Type, req.Description, req.Email,
	).Scan(&id)
	return id, err
}

// UpdateDocFeedbackStatus 更新反馈状态
func UpdateDocFeedbackStatus(db *sql.DB, id int, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx,
		`UPDATE doc_feedback SET status = $1 WHERE id = $2`, status, id,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("feedback not found")
	}
	return nil
}

// ListDocFeedback 分页查询文档反馈
func ListDocFeedback(db *sql.DB, req api.DocFeedbackListRequest) (*api.DocFeedbackListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	where := []string{"1=1"}
	args := []any{}
	idx := 1

	if req.DocumentID != nil {
		where = append(where, fmt.Sprintf("document_id = $%d", idx))
		args = append(args, *req.DocumentID)
		idx++
	}
	if req.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, req.Status)
		idx++
	}

	whereClause := strings.Join(where, " AND ")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM doc_feedback WHERE "+whereClause, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	query := fmt.Sprintf(
		`SELECT id, document_id, type, description, email, status, created_at
		 FROM doc_feedback
		 WHERE %s
		 ORDER BY created_at DESC
		 LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.DocFeedbackItem{}
	for rows.Next() {
		var item api.DocFeedbackItem
		var createdAt time.Time
		err := rows.Scan(&item.ID, &item.DocumentID, &item.Type, &item.Description, &item.Email, &item.Status, &createdAt)
		if err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.Unix()
		items = append(items, item)
	}

	return &api.DocFeedbackListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

// GetDocFeedbackStats 按文档统计反馈数量
func GetDocFeedbackStats(db *sql.DB) ([]api.DocFeedbackStatsItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT d.id, d.slug, d.title,
		        COUNT(f.id) AS total,
		        COUNT(f.id) FILTER (WHERE f.status = 'pending') AS pending,
		        COUNT(f.id) FILTER (WHERE f.status = 'resolved') AS resolved,
		        COUNT(f.id) FILTER (WHERE f.status = 'rejected') AS rejected
		 FROM documents d
		 LEFT JOIN doc_feedback f ON d.id = f.document_id
		 GROUP BY d.id, d.slug, d.title
		 HAVING COUNT(f.id) > 0
		 ORDER BY total DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []api.DocFeedbackStatsItem
	for rows.Next() {
		var item api.DocFeedbackStatsItem
		err := rows.Scan(&item.DocumentID, &item.DocumentSlug, &item.Title,
			&item.Total, &item.Pending, &item.Resolved, &item.Rejected)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
