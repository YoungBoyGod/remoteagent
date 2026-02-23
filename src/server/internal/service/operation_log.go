package service

import (
	"log"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

// ListOperationLogs 分页查询操作日志
func (s *Service) ListOperationLogs(req api.OperationLogListRequest) (*api.OperationLogListResponse, error) {
	return store.ListOperationLogs(s.db, req)
}

// recordOpLog 内部方法，异步记录操作日志（不阻塞主流程）
func (s *Service) recordOpLog(resourceType, resourceID, action string, detail any) {
	go func() {
		err := store.InsertOperationLog(s.db, resourceType, resourceID, action, "admin", detail)
		if err != nil {
			log.Printf("failed to record operation log: %v", err)
		}
	}()
}
