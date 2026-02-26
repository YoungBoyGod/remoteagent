package service

import (
	"fmt"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

// CreateHost 创建主机，校验认证信息后调用 store 层
func (s *Service) CreateHost(req api.HostCreateRequest) (string, error) {
	if req.AuthType == "password" && req.Password == "" {
		return "", fmt.Errorf("password is required when auth_type is password")
	}
	if req.AuthType == "key" && req.SSHKey == "" {
		return "", fmt.Errorf("ssh_key is required when auth_type is key")
	}
	return store.InsertHost(s.db, req)
}

// UpdateHost 更新主机信息
func (s *Service) UpdateHost(hostID string, req api.HostUpdateRequest) error {
	return store.UpdateHost(s.db, hostID, req)
}

// DeleteHost 删除主机
func (s *Service) DeleteHost(hostID string) error {
	return store.DeleteHost(s.db, hostID)
}

// GetHost 查询单个主机详情
func (s *Service) GetHost(hostID string) (*api.HostItem, error) {
	return store.GetHost(s.db, hostID)
}

// ListHosts 分页查询主机列表
func (s *Service) ListHosts(req api.HostListRequest) (*api.HostListResponse, error) {
	return store.ListHosts(s.db, req)
}
