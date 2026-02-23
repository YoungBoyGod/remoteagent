package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"luoyi2026/server/internal/api"
)

type AgentListRow struct {
	AgentID           string
	DeviceCode        string
	AgentVersion      string
	Hostname          string
	OS                string
	Arch              string
	IP                string
	ExternalIP        string
	Labels            map[string]string
	Capabilities      []string
	HeartbeatInterval int
	LastHeartbeatAt   time.Time
	CreatedAt         time.Time
}

// UpsertAgent 以 device_code 为冲突键进行 upsert，返回数据库中实际的 agent_id。
// 当同一设备重复注册时（可能携带新的 agent_id），保留数据库中已有的 agent_id 以维护外键完整性。
func UpsertAgent(db *sql.DB, req api.RegisterRequest, heartbeatInterval int, pollTimeout int) (string, error) {
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	labels, err := json.Marshal(req.Labels)
	if err != nil {
		return "", err
	}
	capabilities, err := json.Marshal(req.Capabilities)
	if err != nil {
		return "", err
	}
	var ipValue any
	if req.Device.IP == "" {
		ipValue = nil
	} else {
		ipValue = req.Device.IP
	}
	var extIPValue any
	if req.Device.ExternalIP == "" {
		extIPValue = nil
	} else {
		extIPValue = req.Device.ExternalIP
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var actualAgentID string
	err = db.QueryRowContext(
		ctx,
		`insert into agents (
			agent_id, tenant_id, device_code, agent_version, status,
			hostname, os, arch, ip, external_ip, labels, capabilities,
			heartbeat_interval, poll_timeout, last_heartbeat_at, updated_at
		)
		values ($1,$2,$3,$4,'online',$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12,$13,now(),now())
		on conflict (device_code) do update set
			tenant_id = excluded.tenant_id,
			agent_version = excluded.agent_version,
			status = 'online',
			hostname = excluded.hostname,
			os = excluded.os,
			arch = excluded.arch,
			ip = excluded.ip,
			external_ip = excluded.external_ip,
			labels = excluded.labels,
			capabilities = excluded.capabilities,
			heartbeat_interval = excluded.heartbeat_interval,
			poll_timeout = excluded.poll_timeout,
			last_heartbeat_at = now(),
			updated_at = now()
		returning agent_id`,
		req.AgentID,
		tenantID,
		req.DeviceCode,
		req.AgentVersion,
		req.Device.Hostname,
		req.Device.OS,
		req.Device.Arch,
		ipValue,
		extIPValue,
		string(labels),
		string(capabilities),
		heartbeatInterval,
		pollTimeout,
	).Scan(&actualAgentID)
	return actualAgentID, err
}

// UpdateHeartbeat 更新 agent 心跳时间，若 agent 不存在则返回错误
func UpdateHeartbeat(db *sql.DB, agentID string, timestamp int64, externalIP string) error {
	heartbeatTime := time.Unix(timestamp, 0)
	var extIPValue any
	if externalIP == "" {
		extIPValue = nil
	} else {
		extIPValue = externalIP
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := db.ExecContext(
		ctx,
		`update agents
		 set status = 'online',
		     last_heartbeat_at = $2,
		     external_ip = coalesce($3, external_ip),
		     updated_at = now()
		 where agent_id = $1`,
		agentID,
		heartbeatTime,
		extIPValue,
	)
	if err != nil {
		return err
	}
	// 检查受影响行数，为 0 表示 agent 不存在
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("agent not found")
	}
	return nil
}

// ListAgents 从数据库读取 agent 列表快照（用于对外展示，避免仅依赖进程内存）
func ListAgents(db *sql.DB) ([]AgentListRow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT
			agent_id,
			COALESCE(device_code, ''),
			COALESCE(agent_version, ''),
			COALESCE(hostname, ''),
			COALESCE(os, ''),
			COALESCE(arch, ''),
			COALESCE(ip::text, ''),
			COALESCE(external_ip::text, ''),
			COALESCE(labels, '{}'::jsonb)::text,
			COALESCE(capabilities, '[]'::jsonb)::text,
			COALESCE(heartbeat_interval, 30),
			last_heartbeat_at,
			created_at
		FROM agents
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AgentListRow, 0)
	for rows.Next() {
		var (
			item          AgentListRow
			labelsText    string
			capsText      string
			lastHeartbeat sql.NullTime
			createdAt     sql.NullTime
		)
		if err := rows.Scan(
			&item.AgentID,
			&item.DeviceCode,
			&item.AgentVersion,
			&item.Hostname,
			&item.OS,
			&item.Arch,
			&item.IP,
			&item.ExternalIP,
			&labelsText,
			&capsText,
			&item.HeartbeatInterval,
			&lastHeartbeat,
			&createdAt,
		); err != nil {
			return nil, err
		}

		item.Labels = map[string]string{}
		if labelsText != "" {
			_ = json.Unmarshal([]byte(labelsText), &item.Labels)
		}
		item.Capabilities = []string{}
		if capsText != "" {
			_ = json.Unmarshal([]byte(capsText), &item.Capabilities)
		}
		if lastHeartbeat.Valid {
			item.LastHeartbeatAt = lastHeartbeat.Time
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
