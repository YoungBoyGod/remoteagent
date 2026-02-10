package state

import (
	"database/sql"
	"sync"
	"time"

	"luoyi2026/server/internal/model"
)

type ServerState struct {
	Mu      sync.Mutex
	DB      *sql.DB
	Agents  map[string]*model.AgentRecord
	Tokens  map[string]model.TokenRecord
	Tasks   map[string]*model.TaskRecord
	Pending map[string][]any
}

func New(db *sql.DB) *ServerState {
	return &ServerState{
		DB:      db,
		Agents:  make(map[string]*model.AgentRecord),
		Tokens:  make(map[string]model.TokenRecord),
		Tasks:   make(map[string]*model.TaskRecord),
		Pending: make(map[string][]any),
	}
}

func (s *ServerState) Enqueue(agentID string, payload any) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Pending[agentID] = append(s.Pending[agentID], payload)
}

func (s *ServerState) Pop(agentID string) (any, bool) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	queue := s.Pending[agentID]
	if len(queue) == 0 {
		return nil, false
	}
	item := queue[0]
	s.Pending[agentID] = queue[1:]
	return item, true
}

func (s *ServerState) WaitPoll(agentID string, timeout time.Duration) any {
	deadline := time.Now().Add(timeout)
	for {
		if item, ok := s.Pop(agentID); ok {
			return item
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *ServerState) Auth(token string) (string, bool) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	record, ok := s.Tokens[token]
	if !ok {
		return "", false
	}
	if time.Now().After(record.ExpiresAt) {
		delete(s.Tokens, token)
		return "", false
	}
	return record.AgentID, true
}
