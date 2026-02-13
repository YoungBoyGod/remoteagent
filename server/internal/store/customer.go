package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"luoyi2026/server/internal/api"
)

func genCustomerID() string {
	buf := make([]byte, 8)
	rand.Read(buf)
	return "cust-" + hex.EncodeToString(buf)
}

// InsertCustomer 创建客户
func InsertCustomer(db *sql.DB, req api.CustomerCreateRequest) (string, error) {
	customerID := genCustomerID()
	tags, _ := json.Marshal(req.Tags)
	if req.Tags == nil {
		tags = []byte("[]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx,
		`INSERT INTO customers (customer_id, name, email, phone, company, description, tags)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
		customerID, req.Name, req.Email, req.Phone, req.Company, req.Description, string(tags),
	)
	return customerID, err
}

// UpdateCustomer 更新客户信息
func UpdateCustomer(db *sql.DB, customerID string, req api.CustomerUpdateRequest) error {
	sets := []string{}
	args := []any{}
	idx := 1

	if req.Name != "" {
		sets = append(sets, fmt.Sprintf("name = $%d", idx))
		args = append(args, req.Name)
		idx++
	}
	if req.Email != "" {
		sets = append(sets, fmt.Sprintf("email = $%d", idx))
		args = append(args, req.Email)
		idx++
	}
	if req.Phone != "" {
		sets = append(sets, fmt.Sprintf("phone = $%d", idx))
		args = append(args, req.Phone)
		idx++
	}
	if req.Company != "" {
		sets = append(sets, fmt.Sprintf("company = $%d", idx))
		args = append(args, req.Company)
		idx++
	}
	if req.Description != "" {
		sets = append(sets, fmt.Sprintf("description = $%d", idx))
		args = append(args, req.Description)
		idx++
	}
	if req.Tags != nil {
		tags, _ := json.Marshal(req.Tags)
		sets = append(sets, fmt.Sprintf("tags = $%d::jsonb", idx))
		args = append(args, string(tags))
		idx++
	}
	if req.Status != "" {
		sets = append(sets, fmt.Sprintf("status = $%d", idx))
		args = append(args, req.Status)
		idx++
	}

	if len(sets) == 0 {
		return nil
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, customerID)

	query := fmt.Sprintf("UPDATE customers SET %s WHERE customer_id = $%d", strings.Join(sets, ", "), idx)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("customer not found")
	}
	return nil
}

// DeleteCustomer 删除客户（需检查是否还有关联主机）
func DeleteCustomer(db *sql.DB, customerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM customer_hosts WHERE customer_id = $1`, customerID,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("customer still has %d assigned host(s), unassign them first", count)
	}

	result, err := db.ExecContext(ctx, "DELETE FROM customers WHERE customer_id = $1", customerID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("customer not found")
	}
	return nil
}

// GetCustomer 获取单个客户详情（含 host_count）
func GetCustomer(db *sql.DB, customerID string) (*api.CustomerItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var item api.CustomerItem
	var tags string
	var email, phone, company, description sql.NullString
	var createdAt, updatedAt time.Time

	err := db.QueryRowContext(ctx,
		`SELECT c.customer_id, c.name, c.email, c.phone, c.company, c.description,
		        COALESCE(c.tags, '[]'), c.status, c.created_at, c.updated_at,
		        (SELECT COUNT(*) FROM customer_hosts ch WHERE ch.customer_id = c.customer_id)
		 FROM customers c
		 WHERE c.customer_id = $1`, customerID,
	).Scan(
		&item.CustomerID, &item.Name, &email, &phone, &company, &description,
		&tags, &item.Status, &createdAt, &updatedAt,
		&item.HostCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if email.Valid {
		item.Email = email.String
	}
	if phone.Valid {
		item.Phone = phone.String
	}
	if company.Valid {
		item.Company = company.String
	}
	if description.Valid {
		item.Description = description.String
	}
	json.Unmarshal([]byte(tags), &item.Tags)
	if item.Tags == nil {
		item.Tags = []string{}
	}
	item.CreatedAt = createdAt.Unix()
	item.UpdatedAt = updatedAt.Unix()

	return &item, nil
}

// ListCustomers 分页查询客户列表
func ListCustomers(db *sql.DB, req api.CustomerListRequest) (*api.CustomerListResponse, error) {
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

	if req.Status != "" {
		where = append(where, fmt.Sprintf("c.status = $%d", idx))
		args = append(args, req.Status)
		idx++
	}
	if req.Search != "" {
		where = append(where, fmt.Sprintf("(c.name ILIKE $%d OR c.company ILIKE $%d OR c.phone ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+req.Search+"%")
		idx++
	}

	whereClause := strings.Join(where, " AND ")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM customers c WHERE "+whereClause, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	query := fmt.Sprintf(
		`SELECT c.customer_id, c.name, c.email, c.phone, c.company, c.description,
		        COALESCE(c.tags, '[]'), c.status, c.created_at, c.updated_at,
		        (SELECT COUNT(*) FROM customer_hosts ch WHERE ch.customer_id = c.customer_id)
		 FROM customers c
		 WHERE %s
		 ORDER BY c.created_at DESC
		 LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.CustomerItem{}
	for rows.Next() {
		var item api.CustomerItem
		var tagsStr string
		var email, phone, company, description sql.NullString
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&item.CustomerID, &item.Name, &email, &phone, &company, &description,
			&tagsStr, &item.Status, &createdAt, &updatedAt,
			&item.HostCount,
		)
		if err != nil {
			return nil, err
		}

		if email.Valid {
			item.Email = email.String
		}
		if phone.Valid {
			item.Phone = phone.String
		}
		if company.Valid {
			item.Company = company.String
		}
		if description.Valid {
			item.Description = description.String
		}
		json.Unmarshal([]byte(tagsStr), &item.Tags)
		if item.Tags == nil {
			item.Tags = []string{}
		}
		item.CreatedAt = createdAt.Unix()
		item.UpdatedAt = updatedAt.Unix()

		items = append(items, item)
	}

	return &api.CustomerListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

// AssignHost 分配主机给客户
func AssignHost(db *sql.DB, customerID string, req api.CustomerHostAssignRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 检查客户是否存在
	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM customers WHERE customer_id = $1)", customerID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("customer not found")
	}

	// 插入关联 + 更新 hosts.customer_id
	_, err = db.ExecContext(ctx,
		`INSERT INTO customer_hosts (customer_id, host_id, note) VALUES ($1, $2, $3)`,
		customerID, req.HostID, req.Note,
	)
	if err != nil {
		return fmt.Errorf("assign failed (host may not exist or already assigned): %w", err)
	}

	_, err = db.ExecContext(ctx,
		`UPDATE hosts SET customer_id = $1, updated_at = now() WHERE host_id = $2`,
		customerID, req.HostID,
	)
	return err
}

// UnassignHost 回收主机
func UnassignHost(db *sql.DB, customerID string, hostID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx,
		`DELETE FROM customer_hosts WHERE customer_id = $1 AND host_id = $2`,
		customerID, hostID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("assignment not found")
	}

	// 清空 hosts.customer_id（仅当该主机没有其他客户关联时）
	_, err = db.ExecContext(ctx,
		`UPDATE hosts SET customer_id = NULL, updated_at = now()
		 WHERE host_id = $1 AND NOT EXISTS (SELECT 1 FROM customer_hosts WHERE host_id = $1)`,
		hostID,
	)
	return err
}

// ListCustomerHosts 查询客户已分配的主机列表
func ListCustomerHosts(db *sql.DB, customerID string) ([]api.CustomerHostItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT h.host_id, h.name, h.ip, COALESCE(h.hostname,''), h.status, ch.assigned_at, COALESCE(ch.note,'')
		 FROM customer_hosts ch
		 JOIN hosts h ON ch.host_id = h.host_id
		 WHERE ch.customer_id = $1
		 ORDER BY ch.assigned_at DESC`, customerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.CustomerHostItem{}
	for rows.Next() {
		var item api.CustomerHostItem
		var assignedAt time.Time
		err := rows.Scan(&item.HostID, &item.HostName, &item.IP, &item.Hostname, &item.Status, &assignedAt, &item.Note)
		if err != nil {
			return nil, err
		}
		item.AssignedAt = assignedAt.Unix()
		items = append(items, item)
	}
	return items, nil
}
