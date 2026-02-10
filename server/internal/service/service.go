package service

import (
	"database/sql"
	"sync"

	"luoyi2026/server/internal/model"
)

type Service struct {
	mu      sync.Mutex
	db      *sql.DB
	agents  map[string]*model.AgentRecord
	tokens  map[string]model.TokenRecord
	tasks   map[string]*model.TaskRecord
	pending map[string][]any
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
