package service

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"luoyi2026/server/internal/model"
)

type Service struct {
	mu      sync.Mutex
	db      *sql.DB
	agents  map[string]*model.AgentRecord
	tokens  map[string]model.TokenRecord
	tasks   map[string]*model.TaskRecord
	pending map[string][]any

	// Token GC 相关字段
	gcStop chan struct{} // 用于通知 GC goroutine 停止
	gcDone chan struct{} // GC goroutine 退出后关闭，用于等待退出完成
}

func New(db *sql.DB) *Service {
	return &Service{
		db:      db,
		agents:  make(map[string]*model.AgentRecord),
		tokens:  make(map[string]model.TokenRecord),
		tasks:   make(map[string]*model.TaskRecord),
		pending: make(map[string][]any),
	}
}

// StartTokenGC 启动 Token 垃圾回收，定期清理过期 token
func (s *Service) StartTokenGC(interval time.Duration) {
	s.gcStop = make(chan struct{})
	s.gcDone = make(chan struct{})
	go func() {
		defer close(s.gcDone)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.cleanExpiredTokens()
			case <-s.gcStop:
				return
			}
		}
	}()
	log.Printf("token GC started, interval=%v", interval)
}

// StopTokenGC 停止 Token 垃圾回收，用于优雅关闭
func (s *Service) StopTokenGC() {
	if s.gcStop == nil {
		return
	}
	close(s.gcStop)
	<-s.gcDone // 等待 GC goroutine 退出
	log.Printf("token GC stopped")
}

// cleanExpiredTokens 扫描并删除所有已过期的 token
func (s *Service) cleanExpiredTokens() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for token, record := range s.tokens {
		if now.After(record.ExpiresAt) {
			delete(s.tokens, token)
			count++
		}
	}
	if count > 0 {
		log.Printf("token GC: cleaned %d expired tokens", count)
	}
}
