package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

// DistributeResponse POST /v1/distribute 响应
type DistributeResponse struct {
	DistributionID int64  `json:"distribution_id"`
	TaskID         string `json:"task_id"`
	DistTaskID     string `json:"dist_task_id"`
	Status         string `json:"status"`
}

// CreateDistribution 创建分发记录并派发加密任务到任务队列
func (s *Service) CreateDistribution(req api.DistributionCreateRequest) (*DistributeResponse, error) {
	// 1. 创建 Distribution 记录
	dist, err := store.InsertDistribution(s.db, req)
	if err != nil {
		return nil, fmt.Errorf("insert distribution: %w", err)
	}

	// 2. 构建加密任务 payload，通过 Phase 2 任务系统派发给 Agent
	algo := req.EncryptionAlgo
	if algo == "" {
		algo = "AES-256"
	}

	taskReq := api.TaskCreateRequest{
		TaskType: "distribute",
		Payload: api.TaskPayload{
			Command: "scripts/secure-distribute.sh",
			Args: []string{
				"--action", "encrypt",
				"--file", req.FileName,
				"--algo", algo,
				"--customer", req.CustomerName,
			},
			Env: map[string]string{
				"DIST_TASK_ID":    dist.TaskID,
				"CUSTOMER_EMAIL":  req.CustomerEmail,
				"SHA256_ORIGINAL": req.SHA256Original,
			},
			Timeout: 600, // 加密任务给 10 分钟
		},
		ExecMode:    "exclusive",
		Priority:    60,
		MaxAttempts: 2,
	}

	taskResp, err := s.CreateTask(taskReq)
	if err != nil {
		log.Printf("[CreateDistribution] create task failed for dist %s: %v", dist.TaskID, err)
		// 任务创建失败不影响分发记录，后续可重试
		return &DistributeResponse{
			DistributionID: dist.ID,
			DistTaskID:     dist.TaskID,
			Status:         dist.Status,
		}, nil
	}

	return &DistributeResponse{
		DistributionID: dist.ID,
		TaskID:         taskResp.TaskID,
		DistTaskID:     dist.TaskID,
		Status:         dist.Status,
	}, nil
}

// GetDistribution 查询单条分发详情
func (s *Service) GetDistribution(id int64) (*api.DistributionItem, error) {
	item, err := store.GetDistributionByID(s.db, id)
	if err != nil {
		return nil, fmt.Errorf("get distribution: %w", err)
	}
	return item, nil
}

// ListDistributions 分页查询分发列表
func (s *Service) ListDistributions(req api.DistributionListRequest) (*api.DistributionListResponse, error) {
	resp, err := store.ListDistributions(s.db, req)
	if err != nil {
		return nil, fmt.Errorf("list distributions: %w", err)
	}
	return resp, nil
}

// UpdateDistribution 更新分发记录
func (s *Service) UpdateDistribution(id int64, req api.DistributionUpdateRequest) error {
	return store.UpdateDistribution(s.db, id, req)
}

// UpdateDistributionStatus 更新分发状态
func (s *Service) UpdateDistributionStatus(id int64, req api.DistributionStatusRequest) error {
	return store.UpdateDistributionStatus(s.db, id, req)
}

// HandleDistributionCallback Agent 完成加密任务后的回调处理
// 解析 Agent 回报的 stdout JSON，更新 Distribution 记录
func (s *Service) HandleDistributionCallback(distTaskID string, stdout string) error {
	// 查找对应的 Distribution 记录
	dist, err := store.GetDistributionByTaskID(s.db, distTaskID)
	if err != nil {
		return fmt.Errorf("get distribution by task_id: %w", err)
	}
	if dist == nil {
		return fmt.Errorf("distribution not found for task_id: %s", distTaskID)
	}

	// 解析 Agent 回报的 JSON 结果
	var result struct {
		EncryptedFilePath string `json:"encrypted_file_path"`
		SHA256Encrypted   string `json:"sha256_encrypted"`
		SessionKeyHash    string `json:"session_key_hash"`
		PresignedURL      string `json:"presigned_url"`
		URLExpiresAt      *int64 `json:"url_expires_at"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return fmt.Errorf("parse agent result: %w", err)
	}

	// 更新分发记录字段
	updateReq := api.DistributionUpdateRequest{
		EncryptedFilePath: result.EncryptedFilePath,
		SHA256Encrypted:   result.SHA256Encrypted,
		SessionKeyHash:    result.SessionKeyHash,
		PresignedURL:      result.PresignedURL,
		URLExpiresAt:      result.URLExpiresAt,
	}
	if err := store.UpdateDistribution(s.db, dist.ID, updateReq); err != nil {
		return fmt.Errorf("update distribution fields: %w", err)
	}

	// 推进状态：encrypting -> uploaded
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx

	statusReq := api.DistributionStatusRequest{Status: store.DistStatusUploaded}
	if err := store.UpdateDistributionStatus(s.db, dist.ID, statusReq); err != nil {
		return fmt.Errorf("update distribution status: %w", err)
	}

	log.Printf("[HandleDistributionCallback] distribution %s (id=%d) updated to uploaded", distTaskID, dist.ID)
	return nil
}
