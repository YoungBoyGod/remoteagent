package service

import (
	"fmt"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/store"
)

func (s *Service) CreateReleaseNote(req api.ReleaseNoteCreateRequest) (*api.ReleaseNoteItem, error) {
	item, err := store.InsertReleaseNote(s.db, req)
	if err != nil {
		return nil, fmt.Errorf("insert release note: %w", err)
	}
	return item, nil
}

func (s *Service) GetReleaseNote(id int64) (*api.ReleaseNoteItem, error) {
	return store.GetReleaseNoteByID(s.db, id)
}

func (s *Service) ListReleaseNotes(req api.ReleaseNoteListRequest) (*api.ReleaseNoteListResponse, error) {
	return store.ListReleaseNotes(s.db, req)
}

func (s *Service) UpdateReleaseNote(id int64, req api.ReleaseNoteUpdateRequest) error {
	return store.UpdateReleaseNote(s.db, id, req)
}

func (s *Service) DeleteReleaseNote(id int64) error {
	return store.DeleteReleaseNote(s.db, id)
}
