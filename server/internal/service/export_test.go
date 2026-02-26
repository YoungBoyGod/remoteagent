package service

import (
	"luoyi2026/server/internal/model"
	"time"
)

// GetTask 返回内存中的任务记录，用于测试断言。
func (s *Service) GetTask(taskID string) *model.TaskRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tasks[taskID]
}

// GetAgent 返回内存中的 agent 记录，用于测试断言。
func (s *Service) GetAgent(agentID string) *model.AgentRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agents[agentID]
}

// SetAgent 在内存中设置 agent 记录，用于测试前置数据准备。
func (s *Service) SetAgent(agentID string, record *model.AgentRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[agentID] = record
}

// SetToken 在内存中设置 token 记录，用于 Auth 测试的前置数据准备。
func (s *Service) SetToken(token string, rec model.TokenRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = rec
}

// GetTokenMap 返回所有 token 的副本，用于测试中验证 token 是否被正确清理。
func (s *Service) GetTokenMap() map[string]model.TokenRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make(map[string]model.TokenRecord, len(s.tokens))
	for k, v := range s.tokens {
		cp[k] = v
	}
	return cp
}

// PendingLen 返回指定 agent 的待处理消息队列长度，用于验证入队/出队行为。
func (s *Service) PendingLen(agentID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending[agentID])
}

// SetTask 在内存中设置任务记录，用于测试前置数据准备。
func (s *Service) SetTask(taskID string, rec *model.TaskRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[taskID] = rec
}

// ExpiredToken 创建一个已过期的 token 记录（过期时间为1小时前），用于测试过期 token 被拒绝的场景。
func ExpiredToken(agentID string) model.TokenRecord {
	return model.TokenRecord{AgentID: agentID, ExpiresAt: time.Now().Add(-1 * time.Hour)}
}

// ValidToken 创建一个有效的 token 记录（1小时后过期），用于测试有效 token 通过认证的场景。
func ValidToken(agentID string) model.TokenRecord {
	return model.TokenRecord{AgentID: agentID, ExpiresAt: time.Now().Add(1 * time.Hour)}
}
