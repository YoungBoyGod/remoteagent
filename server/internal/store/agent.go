package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"luoyi2026/server/internal/api"
)

func UpsertAgent(db *sql.DB, req api.RegisterRequest, heartbeatInterval int, pollTimeout int) error {
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	labels, err := json.Marshal(req.Labels)
	if err != nil {
		return err
	}
	capabilities, err := json.Marshal(req.Capabilities)
	if err != nil {
		return err
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
	_, err = db.ExecContext(
		ctx,
		`insert into agents (
			agent_id, tenant_id, device_code, agent_version, status,
			hostname, os, arch, ip, external_ip, labels, capabilities,
			heartbeat_interval, poll_timeout, last_heartbeat_at, updated_at
		)
		values ($1,$2,$3,$4,'online',$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12,$13,now(),now())
		on conflict (agent_id) do update set
			tenant_id = excluded.tenant_id,
			device_code = excluded.device_code,
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
			updated_at = now()`,
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
	)
	return err
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
