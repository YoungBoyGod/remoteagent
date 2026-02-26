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

// InsertOperationLog 插入操作日志
func InsertOperationLog(db *sql.DB, resourceType, resourceID, action, operator string, detail any) error {
	detailJSON, _ := json.Marshal(detail)
	if detail == nil {
		detailJSON = []byte("{}")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx,
		`INSERT INTO operation_logs (resource_type, resource_id, action, operator, detail)
		 VALUES ($1, $2, $3, $4, $5::jsonb)`,
		resourceType, resourceID, action, operator, string(detailJSON),
	)
	return err
}

// ListOperationLogs 分页查询操作日志
func ListOperationLogs(db *sql.DB, req api.OperationLogListRequest) (*api.OperationLogListResponse, error) {
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

	if req.ResourceType != "" {
		where = append(where, fmt.Sprintf("resource_type = $%d", idx))
		args = append(args, req.ResourceType)
		idx++
	}
	if req.ResourceID != "" {
		where = append(where, fmt.Sprintf("resource_id = $%d", idx))
		args = append(args, req.ResourceID)
		idx++
	}
	if req.Action != "" {
		where = append(where, fmt.Sprintf("action = $%d", idx))
		args = append(args, req.Action)
		idx++
	}

	whereClause := strings.Join(where, " AND ")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM operation_logs WHERE "+whereClause, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	query := fmt.Sprintf(
		`SELECT log_id, resource_type, resource_id, action, operator, COALESCE(detail,'{}'), created_at
		 FROM operation_logs
		 WHERE %s
		 ORDER BY created_at DESC
		 LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.OperationLogItem{}
	for rows.Next() {
		var item api.OperationLogItem
		var detailStr string
		var createdAt time.Time

		err := rows.Scan(&item.LogID, &item.ResourceType, &item.ResourceID, &item.Action, &item.Operator, &detailStr, &createdAt)
		if err != nil {
			return nil, err
		}

		var detail any
		json.Unmarshal([]byte(detailStr), &detail)
		item.Detail = detail
		item.CreatedAt = createdAt.Unix()

		items = append(items, item)
	}

	return &api.OperationLogListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}
