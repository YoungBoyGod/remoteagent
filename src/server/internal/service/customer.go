package service

import (
	"fmt"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

// CreateCustomer 创建客户，校验 + 记录操作日志
func (s *Service) CreateCustomer(req api.CustomerCreateRequest) (string, error) {
	if req.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	customerID, err := store.InsertCustomer(s.db, req)
	if err != nil {
		return "", err
	}
	s.recordOpLog("customer", customerID, "create", map[string]any{"name": req.Name})
	return customerID, nil
}

// UpdateCustomer 更新客户信息 + 记录操作日志
func (s *Service) UpdateCustomer(customerID string, req api.CustomerUpdateRequest) error {
	err := store.UpdateCustomer(s.db, customerID, req)
	if err != nil {
		return err
	}
	s.recordOpLog("customer", customerID, "update", map[string]any{"fields": req})
	return nil
}

// DeleteCustomer 删除客户 + 记录操作日志
func (s *Service) DeleteCustomer(customerID string) error {
	err := store.DeleteCustomer(s.db, customerID)
	if err != nil {
		return err
	}
	s.recordOpLog("customer", customerID, "delete", nil)
	return nil
}

// GetCustomer 查询单个客户详情
func (s *Service) GetCustomer(customerID string) (*api.CustomerItem, error) {
	return store.GetCustomer(s.db, customerID)
}

// ListCustomers 分页查询客户列表
func (s *Service) ListCustomers(req api.CustomerListRequest) (*api.CustomerListResponse, error) {
	return store.ListCustomers(s.db, req)
}

// AssignHost 分配主机给客户 + 记录操作日志
func (s *Service) AssignHost(customerID string, req api.CustomerHostAssignRequest) error {
	if req.HostID == "" {
		return fmt.Errorf("host_id is required")
	}
	err := store.AssignHost(s.db, customerID, req)
	if err != nil {
		return err
	}
	s.recordOpLog("host_assign", customerID, "assign_host", map[string]any{"host_id": req.HostID, "note": req.Note})
	return nil
}

// UnassignHost 回收主机 + 记录操作日志
func (s *Service) UnassignHost(customerID string, hostID string) error {
	err := store.UnassignHost(s.db, customerID, hostID)
	if err != nil {
		return err
	}
	s.recordOpLog("host_assign", customerID, "unassign_host", map[string]any{"host_id": hostID})
	return nil
}

// ListCustomerHosts 查询客户已分配的主机列表
func (s *Service) ListCustomerHosts(customerID string) ([]api.CustomerHostItem, error) {
	return store.ListCustomerHosts(s.db, customerID)
}
