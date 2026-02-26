package service

import (
	"context"
	"fmt"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

// DashboardSummary 返回 dashboard 所需聚合数据
func (s *Service) DashboardSummary(recentLimit int) (*api.DashboardSummaryResponse, error) {
	if recentLimit <= 0 {
		recentLimit = 10
	}
	if recentLimit > 100 {
		recentLimit = 100
	}

	agents := s.ListAgents("", "")
	online := 0
	for _, a := range agents {
		if a.Status == "online" {
			online++
		}
	}

	statusCounts, taskTotal, err := store.CountTasksByStatusV2(context.Background(), s.db)
	if err != nil {
		return nil, fmt.Errorf("count task status: %w", err)
	}

	rows, _, err := store.ListTasksV2(context.Background(), s.db, api.TaskListRequest{
		Page:     1,
		PageSize: recentLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("list recent tasks: %w", err)
	}
	recent := make([]api.TaskDetail, 0, len(rows))
	for _, row := range rows {
		recent = append(recent, taskRowToDetail(row))
	}

	hostTotal, _ := store.CountHosts(s.db)
	customerTotal, _ := store.CountCustomers(s.db)

	return &api.DashboardSummaryResponse{
		AgentTotal:      len(agents),
		AgentOnline:     online,
		HostTotal:       hostTotal,
		CustomerTotal:   customerTotal,
		TaskTotal:       taskTotal,
		TaskStatusCount: statusCounts,
		RecentTasks:     recent,
	}, nil
}
