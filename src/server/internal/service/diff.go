package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/storage"
	"luoyi2026/server/internal/store"
)

// DiffDocVersions 对比两个版本的 Markdown 内容，返回结构化 diff
func (s *Service) DiffDocVersions(slug, fromVersion, toVersion string, sto storage.Storage) (*api.DocDiffResponse, error) {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	fromContent, err := s.readVersionContent(slug, fromVersion, doc, sto)
	if err != nil {
		return nil, fmt.Errorf("读取版本 %s 失败: %w", fromVersion, err)
	}

	toContent, err := s.readVersionContent(slug, toVersion, doc, sto)
	if err != nil {
		return nil, fmt.Errorf("读取版本 %s 失败: %w", toVersion, err)
	}

	// 使用 go-diff 生成差异
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(fromContent, toContent, true)
	dmp.DiffCleanupSemantic(diffs)

	// 构建响应
	resp := &api.DocDiffResponse{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	}

	for _, d := range diffs {
		chunk := api.DocDiffChunk{Text: d.Text}
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			chunk.Type = "equal"
			resp.Stats.Equal += len(strings.Split(d.Text, "\n"))
		case diffmatchpatch.DiffInsert:
			chunk.Type = "insert"
			resp.Stats.Added += len(strings.Split(d.Text, "\n"))
		case diffmatchpatch.DiffDelete:
			chunk.Type = "delete"
			resp.Stats.Removed += len(strings.Split(d.Text, "\n"))
		}
		resp.Changes = append(resp.Changes, chunk)
	}

	return resp, nil
}

// readVersionContent 读取指定版本的内容，"latest" 表示当前版本
func (s *Service) readVersionContent(slug, version string, doc *api.DocItem, sto storage.Storage) (string, error) {
	if sto == nil {
		return "", fmt.Errorf("storage not configured")
	}

	var key string
	if version == "latest" {
		key = fmt.Sprintf(storage.PathDocLatest, slug)
	} else {
		key = fmt.Sprintf(storage.PathDocVersion, slug, version)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rc, err := sto.GetObject(ctx, key)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
