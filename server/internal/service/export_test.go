package service

import "luoyi2026/server/internal/model"

// GetTask returns the in-memory task record for testing.
func (s *Service) GetTask(taskID string) *model.TaskRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tasks[taskID]
}

// GetAgent returns the in-memory agent record for testing.
func (s *Service) GetAgent(agentID string) *model.AgentRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agents[agentID]
}

// SetAgent sets an agent record in memory for testing.
func (s *Service) SetAgent(agentID string, record *model.AgentRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[agentID] = record
}
