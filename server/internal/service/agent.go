package service

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/store"
)

func randHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

func (s *Service) Register(req api.RegisterRequest, jwtTTL time.Duration, pollTimeout time.Duration) (map[string]any, error) {
	token := randHex(24)
	expiresAt := time.Now().Add(jwtTTL)
	heartbeatInterval := 30
	pollTimeoutSec := int(pollTimeout / time.Second)

	s.mu.Lock()
	s.tokens[token] = model.TokenRecord{AgentID: req.AgentID, ExpiresAt: expiresAt}
	record, ok := s.agents[req.AgentID]
	if !ok {
		record = &model.AgentRecord{AgentID: req.AgentID, RunningTasks: make(map[string]struct{})}
		s.agents[req.AgentID] = record
	}
	record.DeviceCode = req.DeviceCode
	record.Token = token
	record.TokenExpiresAt = expiresAt
	record.HeartbeatInterval = heartbeatInterval
	record.PollTimeoutSeconds = pollTimeoutSec
	s.mu.Unlock()

	if err := store.UpsertAgent(s.db, req, heartbeatInterval, pollTimeoutSec); err != nil {
		log.Printf("register persist failed: %v", err)
		return nil, err
	}

	return map[string]any{
		"token":              token,
		"heartbeat_interval": heartbeatInterval,
		"poll_timeout":       pollTimeoutSec,
		"server_time":        time.Now().Unix(),
	}, nil
}

func (s *Service) Heartbeat(req api.HeartbeatRequest) error {
	s.mu.Lock()
	record := s.agents[req.AgentID]
	record.LastHeartbeatAt = time.Now()
	record.RunningTasks = toTaskSet(req.RunningTasks)
	s.mu.Unlock()

	if err := store.UpdateHeartbeat(s.db, req.AgentID, req.Timestamp); err != nil {
		log.Printf("heartbeat persist failed: %v", err)
		return err
	}
	return nil
}

func (s *Service) Auth(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.tokens[token]
	if !ok {
		return "", false
	}
	if time.Now().After(record.ExpiresAt) {
		delete(s.tokens, token)
		return "", false
	}
	return record.AgentID, true
}
