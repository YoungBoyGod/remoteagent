package service

import "time"

func (s *Service) Enqueue(agentID string, payload any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[agentID] = append(s.pending[agentID], payload)
}

func (s *Service) pop(agentID string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	queue := s.pending[agentID]
	if len(queue) == 0 {
		return nil, false
	}
	item := queue[0]
	s.pending[agentID] = queue[1:]
	return item, true
}

func (s *Service) WaitPoll(agentID string, timeout time.Duration) any {
	deadline := time.Now().Add(timeout)
	for {
		if item, ok := s.pop(agentID); ok {
			return item
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}
