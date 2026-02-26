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

func genHostID() string {
	buf := make([]byte, 8)
	rand.Read(buf)
	return "host-" + hex.EncodeToString(buf)
}

// GetHostTagsByAgentIDs 批量查询 agent 关联的 host tags
func GetHostTagsByAgentIDs(db *sql.DB, agentIDs []string) (map[string][]string, error) {
	if len(agentIDs) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 构建 ANY($1) 参数
	placeholders := make([]string, len(agentIDs))
	args := make([]any, len(agentIDs))
	for i, id := range agentIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT agent_id, COALESCE(tags, '[]') FROM hosts WHERE agent_id IN (%s) AND agent_id IS NOT NULL`,
		strings.Join(placeholders, ","),
	)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var agentID, tagsStr string
		if err := rows.Scan(&agentID, &tagsStr); err != nil {
			return nil, err
		}
		var tags []string
		json.Unmarshal([]byte(tagsStr), &tags)
		if len(tags) > 0 {
			result[agentID] = tags
		}
	}
	return result, nil
}

// InsertHost 插入新主机（手动创建，source='manual'）
func InsertHost(db *sql.DB, req api.HostCreateRequest) (string, error) {
	hostID := genHostID()
	port := req.Port
	if port == 0 {
		port = 22
	}
	username := req.Username
	if username == "" {
		username = "root"
	}
	tags, _ := json.Marshal(req.Tags)
	if req.Tags == nil {
		tags = []byte("[]")
	}

	storedPassword := req.Password

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx,
		`INSERT INTO hosts (host_id, name, ip, hostname, port, username, auth_type, password, ssh_key,
		                    vnc_addr, jupyter_addr, ext_ssh_addr, ext_vnc_addr, ext_jupyter_addr,
		                    assigned_to, description, tags, source)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17::jsonb, 'manual')`,
		hostID, req.Name, req.IP, req.Hostname, port, username, req.AuthType,
		storedPassword, req.SSHKey, req.VNCAddr, req.JupyterAddr,
		req.ExtSSHAddr, req.ExtVNCAddr, req.ExtJupyterAddr,
		req.AssignedTo, req.Description, string(tags),
	)
	return hostID, err
}

// AutoCreateOrLinkHost Agent 注册时自动创建或关联主机
// 逻辑：先按 agent_id 查找已关联的 host，再按 IP 查找未关联的 host，都没有则自动创建
func AutoCreateOrLinkHost(db *sql.DB, req api.HostAutoCreateRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. 已有 host 关联了此 agent → 更新 IP/hostname，同步状态
	var existID string
	err := db.QueryRowContext(ctx,
		`SELECT host_id FROM hosts WHERE agent_id = $1 LIMIT 1`, req.AgentID,
	).Scan(&existID)
	if err == nil {
		// 已关联，更新 agent 上报的 IP/hostname
		_, _ = db.ExecContext(ctx,
			`UPDATE hosts SET ip = $1, hostname = COALESCE(NULLIF($2,''), hostname), status = 'online', updated_at = now()
			 WHERE host_id = $3`,
			req.IP, req.Hostname, existID,
		)
		return nil
	}

	// 2. 按 IP 查找未关联 agent 的 host → 关联
	err = db.QueryRowContext(ctx,
		`SELECT host_id FROM hosts WHERE ip = $1 AND agent_id IS NULL LIMIT 1`, req.IP,
	).Scan(&existID)
	if err == nil {
		_, _ = db.ExecContext(ctx,
			`UPDATE hosts SET agent_id = $1, hostname = COALESCE(NULLIF($2,''), hostname), status = 'online', updated_at = now()
			 WHERE host_id = $3`,
			req.AgentID, req.Hostname, existID,
		)
		return nil
	}

	// 3. 都没有 → 自动创建 host（source='agent'）
	hostID := genHostID()
	name := req.Hostname
	if name == "" {
		name = req.IP
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO hosts (host_id, name, ip, hostname, agent_id, source, status)
		 VALUES ($1, $2, $3, $4, $5, 'agent', 'online')`,
		hostID, name, req.IP, req.Hostname, req.AgentID,
	)
	return err
}

// UpdateHost 更新主机信息
func UpdateHost(db *sql.DB, hostID string, req api.HostUpdateRequest) error {
	sets := []string{}
	args := []any{}
	idx := 1

	if req.Name != "" {
		sets = append(sets, fmt.Sprintf("name = $%d", idx))
		args = append(args, req.Name)
		idx++
	}
	if req.IP != "" {
		sets = append(sets, fmt.Sprintf("ip = $%d", idx))
		args = append(args, req.IP)
		idx++
	}
	if req.Hostname != "" {
		sets = append(sets, fmt.Sprintf("hostname = $%d", idx))
		args = append(args, req.Hostname)
		idx++
	}
	if req.Port != nil {
		sets = append(sets, fmt.Sprintf("port = $%d", idx))
		args = append(args, *req.Port)
		idx++
	}
	if req.Username != "" {
		sets = append(sets, fmt.Sprintf("username = $%d", idx))
		args = append(args, req.Username)
		idx++
	}
	if req.AuthType != "" {
		sets = append(sets, fmt.Sprintf("auth_type = $%d", idx))
		args = append(args, req.AuthType)
		idx++
	}
	if req.Password != "" {
		sets = append(sets, fmt.Sprintf("password = $%d", idx))
		args = append(args, req.Password)
		idx++
	}
	if req.SSHKey != "" {
		sets = append(sets, fmt.Sprintf("ssh_key = $%d", idx))
		args = append(args, req.SSHKey)
		idx++
	}
	if req.VNCAddr != "" {
		sets = append(sets, fmt.Sprintf("vnc_addr = $%d", idx))
		args = append(args, req.VNCAddr)
		idx++
	}
	if req.JupyterAddr != "" {
		sets = append(sets, fmt.Sprintf("jupyter_addr = $%d", idx))
		args = append(args, req.JupyterAddr)
		idx++
	}
	if req.ExtSSHAddr != "" {
		sets = append(sets, fmt.Sprintf("ext_ssh_addr = $%d", idx))
		args = append(args, req.ExtSSHAddr)
		idx++
	}
	if req.ExtVNCAddr != "" {
		sets = append(sets, fmt.Sprintf("ext_vnc_addr = $%d", idx))
		args = append(args, req.ExtVNCAddr)
		idx++
	}
	if req.ExtJupyterAddr != "" {
		sets = append(sets, fmt.Sprintf("ext_jupyter_addr = $%d", idx))
		args = append(args, req.ExtJupyterAddr)
		idx++
	}
	if req.Description != "" {
		sets = append(sets, fmt.Sprintf("description = $%d", idx))
		args = append(args, req.Description)
		idx++
	}
	if req.AssignedTo != nil {
		sets = append(sets, fmt.Sprintf("assigned_to = $%d", idx))
		args = append(args, *req.AssignedTo)
		idx++
	}
	if req.Tags != nil {
		tags, _ := json.Marshal(req.Tags)
		sets = append(sets, fmt.Sprintf("tags = $%d::jsonb", idx))
		args = append(args, string(tags))
		idx++
	}

	if len(sets) == 0 {
		return nil
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, hostID)

	query := fmt.Sprintf("UPDATE hosts SET %s WHERE host_id = $%d", strings.Join(sets, ", "), idx)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("host not found")
	}
	return nil
}

// DeleteHost 删除主机
func DeleteHost(db *sql.DB, hostID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ExecContext(ctx, "DELETE FROM hosts WHERE host_id = $1", hostID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("host not found")
	}
	return nil
}

// GetHost 获取单个主机详情
func GetHost(db *sql.DB, hostID string) (*api.HostItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var item api.HostItem
	var tags string
	var agentID, agentStatus, agentHostname, agentOS, agentArch, agentVersion, externalIP, customerID sql.NullString
	var lastHeartbeat sql.NullTime
	var createdAt, updatedAt time.Time

	err := db.QueryRowContext(ctx,
		`SELECT h.host_id, h.name, h.ip, COALESCE(h.hostname,''), h.port, h.username, h.auth_type, COALESCE(h.password,''), h.status, h.source,
		        COALESCE(h.vnc_addr,''), COALESCE(h.jupyter_addr,''), COALESCE(h.ext_ssh_addr,''), COALESCE(h.ext_vnc_addr,''), COALESCE(h.ext_jupyter_addr,''),
		        h.agent_id, COALESCE(h.assigned_to,''), COALESCE(h.description,''), COALESCE(h.tags,'[]'), h.created_at, h.updated_at,
		        a.status, a.hostname, a.os, a.arch, a.agent_version, a.external_ip, a.last_heartbeat_at,
		        h.customer_id
		 FROM hosts h
		 LEFT JOIN agents a ON h.agent_id = a.agent_id
		 WHERE h.host_id = $1`, hostID,
	).Scan(
		&item.HostID, &item.Name, &item.IP, &item.Hostname, &item.Port, &item.Username,
		&item.AuthType, &item.Password, &item.Status, &item.Source,
		&item.VNCAddr, &item.JupyterAddr, &item.ExtSSHAddr, &item.ExtVNCAddr, &item.ExtJupyterAddr,
		&agentID, &item.AssignedTo, &item.Description, &tags,
		&createdAt, &updatedAt,
		&agentStatus, &agentHostname, &agentOS, &agentArch, &agentVersion, &externalIP, &lastHeartbeat,
		&customerID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(tags), &item.Tags)
	if item.Tags == nil {
		item.Tags = []string{}
	}
	item.CreatedAt = createdAt.Unix()
	item.UpdatedAt = updatedAt.Unix()
	if agentID.Valid {
		item.AgentID = agentID.String
	}
	if agentStatus.Valid {
		item.AgentStatus = agentStatus.String
		// 关联了 agent 时，用 agent 实时状态覆盖 host 静态状态
		item.Status = agentStatus.String
	}
	if agentHostname.Valid {
		item.AgentHostname = agentHostname.String
	}
	if agentOS.Valid {
		item.AgentOS = agentOS.String
	}
	if agentArch.Valid {
		item.AgentArch = agentArch.String
	}
	if agentVersion.Valid {
		item.AgentVersion = agentVersion.String
	}
	if externalIP.Valid {
		item.ExternalIP = externalIP.String
	}
	if lastHeartbeat.Valid {
		ts := lastHeartbeat.Time.Unix()
		item.LastHeartbeat = &ts
	}
	if customerID.Valid {
		item.CustomerID = customerID.String
	}

	return &item, nil
}

// ListHosts 分页查询主机列表
func ListHosts(db *sql.DB, req api.HostListRequest) (*api.HostListResponse, error) {
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
		where = append(where, fmt.Sprintf("h.status = $%d", idx))
		args = append(args, req.Status)
		idx++
	}
	if req.Search != "" {
		where = append(where, fmt.Sprintf("(h.name ILIKE $%d OR h.ip ILIKE $%d OR h.hostname ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+req.Search+"%")
		idx++
	}

	whereClause := strings.Join(where, " AND ")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM hosts h WHERE "+whereClause, countArgs...).Scan(&total)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	query := fmt.Sprintf(
		`SELECT h.host_id, h.name, h.ip, COALESCE(h.hostname,''), h.port, h.username, h.auth_type, COALESCE(h.password,''), h.status, h.source,
		        COALESCE(h.vnc_addr,''), COALESCE(h.jupyter_addr,''), COALESCE(h.ext_ssh_addr,''), COALESCE(h.ext_vnc_addr,''), COALESCE(h.ext_jupyter_addr,''),
		        h.agent_id, COALESCE(h.assigned_to,''), COALESCE(h.description,''), COALESCE(h.tags,'[]'), h.created_at, h.updated_at,
		        a.status, a.hostname, a.os, a.arch, a.agent_version, a.external_ip, a.last_heartbeat_at,
		        h.customer_id
		 FROM hosts h
		 LEFT JOIN agents a ON h.agent_id = a.agent_id
		 WHERE %s
		 ORDER BY h.created_at DESC
		 LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []api.HostItem{}
	for rows.Next() {
		var item api.HostItem
		var tagsStr string
		var agentID, agentStatus, agentHostname, agentOS, agentArch, agentVersion, extIP, custID sql.NullString
		var lastHB sql.NullTime
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&item.HostID, &item.Name, &item.IP, &item.Hostname, &item.Port, &item.Username,
			&item.AuthType, &item.Password, &item.Status, &item.Source,
			&item.VNCAddr, &item.JupyterAddr, &item.ExtSSHAddr, &item.ExtVNCAddr, &item.ExtJupyterAddr,
			&agentID, &item.AssignedTo, &item.Description, &tagsStr,
			&createdAt, &updatedAt,
			&agentStatus, &agentHostname, &agentOS, &agentArch, &agentVersion, &extIP, &lastHB,
			&custID,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal([]byte(tagsStr), &item.Tags)
		if item.Tags == nil {
			item.Tags = []string{}
		}
		item.CreatedAt = createdAt.Unix()
		item.UpdatedAt = updatedAt.Unix()
		if agentID.Valid {
			item.AgentID = agentID.String
		}
		if agentStatus.Valid {
			item.AgentStatus = agentStatus.String
			// 关联了 agent 时，用 agent 实时状态覆盖 host 静态状态
			item.Status = agentStatus.String
		}
		if agentHostname.Valid {
			item.AgentHostname = agentHostname.String
		}
		if agentOS.Valid {
			item.AgentOS = agentOS.String
		}
		if agentArch.Valid {
			item.AgentArch = agentArch.String
		}
		if agentVersion.Valid {
			item.AgentVersion = agentVersion.String
		}
		if extIP.Valid {
			item.ExternalIP = extIP.String
		}
		if lastHB.Valid {
			ts := lastHB.Time.Unix()
			item.LastHeartbeat = &ts
		}
		if custID.Valid {
			item.CustomerID = custID.String
		}

		items = append(items, item)
	}

	return &api.HostListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

func CountHosts(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM hosts").Scan(&n)
	return n, err
}
