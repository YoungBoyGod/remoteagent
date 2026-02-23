package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"luoyi2026/server/internal/api"
)

// InsertReleaseNote 创建发布说明草稿
func InsertReleaseNote(db *sql.DB, req api.ReleaseNoteCreateRequest) (*api.ReleaseNoteItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var item api.ReleaseNoteItem
	var createdAt, updatedAt time.Time

	err := db.QueryRowContext(ctx,
		`INSERT INTO release_note_drafts (title, content, version, created_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, title, content, version, created_by, created_at, updated_at`,
		req.Title, req.Content, req.Version, req.CreatedBy,
	).Scan(&item.ID, &item.Title, &item.Content, &item.Version, &item.CreatedBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	item.CreatedAt = createdAt.Unix()
	item.UpdatedAt = updatedAt.Unix()
	return &item, nil
}

// GetReleaseNoteByID 按主键查询
func GetReleaseNoteByID(db *sql.DB, id int64) (*api.ReleaseNoteItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var item api.ReleaseNoteItem
	var createdAt, updatedAt time.Time

	err := db.QueryRowContext(ctx,
		`SELECT id, title, content, version, created_by, created_at, updated_at
		 FROM release_note_drafts WHERE id = $1`, id,
	).Scan(&item.ID, &item.Title, &item.Content, &item.Version, &item.CreatedBy, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	item.CreatedAt = createdAt.Unix()
	item.UpdatedAt = updatedAt.Unix()
	return &item, nil
}

// ListReleaseNotes 分页查询发布说明列表
func ListReleaseNotes(db *sql.DB, req api.ReleaseNoteListRequest) (*api.ReleaseNoteListResponse, error) {
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

	if req.Search != "" {
		where = append(where, fmt.Sprintf("(title ILIKE $%d OR content ILIKE $%d OR version ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+req.Search+"%")
		idx++
	}

	whereClause := strings.Join(where, " AND ")

	orderBy := "created_at DESC"
	allowedSorts := map[string]string{
		"created_at": "created_at",
		"title":      "title",
		"version":    "version",
		"updated_at": "updated_at",
	}
	if col, ok := allowedSorts[req.SortBy]; ok {
		dir := "DESC"
		if strings.EqualFold(req.SortDir, "asc") {
			dir = "ASC"
		}
		orderBy = col + " " + dir
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM release_note_drafts WHERE "+whereClause, countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	query := fmt.Sprintf(
		`SELECT id, title, content, version, created_by, created_at, updated_at
		 FROM release_note_drafts WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		whereClause, orderBy, idx, idx+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.ReleaseNoteItem{}
	for rows.Next() {
		var item api.ReleaseNoteItem
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.Title, &item.Content, &item.Version, &item.CreatedBy, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.Unix()
		item.UpdatedAt = updatedAt.Unix()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &api.ReleaseNoteListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

// UpdateReleaseNote 更新发布说明
func UpdateReleaseNote(db *sql.DB, id int64, req api.ReleaseNoteUpdateRequest) error {
	sets := []string{}
	args := []any{}
	idx := 1

	if req.Title != "" {
		sets = append(sets, fmt.Sprintf("title = $%d", idx))
		args = append(args, req.Title)
		idx++
	}
	if req.Content != "" {
		sets = append(sets, fmt.Sprintf("content = $%d", idx))
		args = append(args, req.Content)
		idx++
	}
	if req.Version != "" {
		sets = append(sets, fmt.Sprintf("version = $%d", idx))
		args = append(args, req.Version)
		idx++
	}

	if len(sets) == 0 {
		return nil
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE release_note_drafts SET %s WHERE id = $%d",
		strings.Join(sets, ", "), idx)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("release note not found")
	}
	return nil
}

// DeleteReleaseNote 删除发布说明
func DeleteReleaseNote(db *sql.DB, id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, "DELETE FROM release_note_drafts WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("release note not found")
	}
	return nil
}
