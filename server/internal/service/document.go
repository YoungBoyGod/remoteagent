package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/storage"
	"luoyi2026/server/internal/store"
)

// ============================================================
// 文档分类
// ============================================================

// CreateDocCategory 创建文档分类
func (s *Service) CreateDocCategory(req api.DocCategoryCreateRequest) (int, error) {
	return store.InsertDocCategory(s.db, req)
}

// UpdateDocCategory 更新文档分类
func (s *Service) UpdateDocCategory(id int, req api.DocCategoryUpdateRequest) error {
	return store.UpdateDocCategory(s.db, id, req)
}

// DeleteDocCategory 删除文档分类
func (s *Service) DeleteDocCategory(id int) error {
	return store.DeleteDocCategory(s.db, id)
}

// ListDocCategoryTree 查询分类树
func (s *Service) ListDocCategoryTree() ([]*api.DocCategoryItem, error) {
	return store.ListDocCategoryTree(s.db)
}

// ============================================================
// 文档 CRUD
// ============================================================

// CreateDocument 创建文档：内容写入 S3，元数据写入 PG
func (s *Service) CreateDocument(req api.DocCreateRequest, content string, sto storage.Storage) (int, error) {
	// 生成 S3 存储路径
	contentKey := fmt.Sprintf(storage.PathDocLatest, req.Slug)
	req.ContentKey = contentKey

	// 写入 S3
	if content != "" && sto != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := sto.PutObject(ctx, contentKey, strings.NewReader(content), "text/markdown"); err != nil {
			return 0, fmt.Errorf("写入 S3 失败: %w", err)
		}
	}

	// 写入 PG
	id, err := store.InsertDocument(s.db, req)
	if err != nil {
		return 0, err
	}

	log.Printf("文档创建成功: id=%d slug=%s", id, req.Slug)
	return id, nil
}

// GetDocumentBySlug 按 slug 获取文档详情（含 S3 内容）
func (s *Service) GetDocumentBySlug(slug string, sto storage.Storage) (*api.DocItem, string, error) {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return nil, "", err
	}
	if doc == nil {
		return nil, "", nil
	}

	// 从 S3 读取内容
	var content string
	if doc.ContentKey != "" && sto != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rc, err := sto.GetObject(ctx, doc.ContentKey)
		if err != nil {
			log.Printf("读取 S3 内容失败: key=%s err=%v", doc.ContentKey, err)
		} else {
			defer rc.Close()
			data, _ := io.ReadAll(rc)
			content = string(data)
		}
	}

	return doc, content, nil
}

// UpdateDocument 更新文档：内容写入 S3，元数据更新 PG
func (s *Service) UpdateDocument(slug string, req api.DocUpdateRequest, content *string, sto storage.Storage) error {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}

	// 更新 S3 内容
	if content != nil && sto != nil {
		contentKey := doc.ContentKey
		if contentKey == "" {
			contentKey = fmt.Sprintf(storage.PathDocLatest, slug)
			req.ContentKey = contentKey
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := sto.PutObject(ctx, contentKey, strings.NewReader(*content), "text/markdown"); err != nil {
			return fmt.Errorf("写入 S3 失败: %w", err)
		}
	}

	return store.UpdateDocument(s.db, doc.ID, req)
}

// DeleteDocument 删除文档：同时删除 S3 内容
func (s *Service) DeleteDocument(slug string, sto storage.Storage) error {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found")
	}

	// 删除 S3 内容（尽力删除，不阻断流程）
	if doc.ContentKey != "" && sto != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := sto.DeleteObject(ctx, doc.ContentKey); err != nil {
			log.Printf("删除 S3 内容失败: key=%s err=%v", doc.ContentKey, err)
		}
	}

	return store.DeleteDocument(s.db, doc.ID)
}

// ListDocuments 分页查询文档列表
func (s *Service) ListDocuments(req api.DocListRequest) (*api.DocListResponse, error) {
	return store.ListDocuments(s.db, req)
}

// ============================================================
// 文档版本
// ============================================================

// CreateDocVersion 创建版本快照：复制 S3 当前内容到 versions/ 目录
func (s *Service) CreateDocVersion(slug string, req api.DocVersionCreateRequest, sto storage.Storage) (int, error) {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return 0, err
	}
	if doc == nil {
		return 0, fmt.Errorf("document not found")
	}

	// 获取当前最大版本号用于生成 content_key
	versions, err := store.ListDocVersions(s.db, doc.ID)
	if err != nil {
		return 0, err
	}
	nextVersion := len(versions) + 1

	// 复制 S3 当前内容到版本目录
	versionKey := fmt.Sprintf(storage.PathDocVersion, slug, fmt.Sprintf("v%d", nextVersion))
	req.ContentKey = versionKey

	if doc.ContentKey != "" && sto != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := sto.CopyObject(ctx, doc.ContentKey, versionKey); err != nil {
			return 0, fmt.Errorf("复制版本快照失败: %w", err)
		}
	}

	return store.InsertDocVersion(s.db, doc.ID, req)
}

// ListDocVersions 查询文档版本列表
func (s *Service) ListDocVersions(slug string) ([]api.DocVersionItem, error) {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}
	return store.ListDocVersions(s.db, doc.ID)
}

// GetDocVersionContent 获取指定版本内容
func (s *Service) GetDocVersionContent(slug string, version string, sto storage.Storage) (*api.DocVersionItem, string, error) {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return nil, "", err
	}
	if doc == nil {
		return nil, "", fmt.Errorf("document not found")
	}

	versions, err := store.ListDocVersions(s.db, doc.ID)
	if err != nil {
		return nil, "", err
	}

	var target *api.DocVersionItem
	for i := range versions {
		if versions[i].Version == version {
			target = &versions[i]
			break
		}
	}
	if target == nil {
		return nil, "", fmt.Errorf("version not found")
	}

	var content string
	if target.ContentKey != "" && sto != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rc, err := sto.GetObject(ctx, target.ContentKey)
		if err != nil {
			log.Printf("读取版本内容失败: key=%s err=%v", target.ContentKey, err)
		} else {
			defer rc.Close()
			data, _ := io.ReadAll(rc)
			content = string(data)
		}
	}

	return target, content, nil
}

// ============================================================
// 文档附件
// ============================================================

// UploadAttachment 上传附件到 S3 并记录元数据
func (s *Service) UploadAttachment(slug string, filename string, reader io.Reader, contentType string, size int64, sto storage.Storage) (*api.DocAttachmentItem, error) {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	storageKey := fmt.Sprintf(storage.PathAttachment, fmt.Sprintf("%d", doc.ID), filename)

	// 上传到 S3
	if sto != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := sto.PutObject(ctx, storageKey, reader, contentType); err != nil {
			return nil, fmt.Errorf("上传附件失败: %w", err)
		}
	}

	id, err := store.InsertDocAttachment(s.db, doc.ID, api.DocAttachmentCreateRequest{
		Filename:    filename,
		StorageKey:  storageKey,
		ContentType: contentType,
		SizeBytes:   size,
	})
	if err != nil {
		return nil, err
	}

	return &api.DocAttachmentItem{
		ID:          id,
		DocumentID:  doc.ID,
		Filename:    filename,
		StorageKey:  storageKey,
		ContentType: contentType,
		SizeBytes:   size,
	}, nil
}

// GetAttachmentURL 获取附件预签名 URL
func (s *Service) GetAttachmentURL(id int, sto storage.Storage) (string, error) {
	att, err := store.GetDocAttachment(s.db, id)
	if err != nil {
		return "", err
	}
	if att == nil {
		return "", fmt.Errorf("attachment not found")
	}

	if sto == nil {
		return "", fmt.Errorf("storage not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return sto.GetPresignedURL(ctx, att.StorageKey, 15*time.Minute)
}

// DeleteAttachment 删除附件（S3 + PG）
func (s *Service) DeleteAttachment(id int, sto storage.Storage) error {
	att, err := store.GetDocAttachment(s.db, id)
	if err != nil {
		return err
	}
	if att == nil {
		return fmt.Errorf("attachment not found")
	}

	// 删除 S3 文件
	if sto != nil && att.StorageKey != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := sto.DeleteObject(ctx, att.StorageKey); err != nil {
			log.Printf("删除附件 S3 文件失败: key=%s err=%v", att.StorageKey, err)
		}
	}

	return store.DeleteDocAttachment(s.db, id)
}

// ============================================================
// 文档反馈
// ============================================================

// CreateDocFeedback 创建文档反馈
func (s *Service) CreateDocFeedback(slug string, req api.DocFeedbackCreateRequest) (int, error) {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return 0, err
	}
	if doc == nil {
		return 0, fmt.Errorf("document not found")
	}
	return store.InsertDocFeedback(s.db, doc.ID, req)
}

// ListDocFeedback 分页查询反馈列表
func (s *Service) ListDocFeedback(req api.DocFeedbackListRequest) (*api.DocFeedbackListResponse, error) {
	return store.ListDocFeedback(s.db, req)
}

// UpdateDocFeedbackStatus 更新反馈状态
func (s *Service) UpdateDocFeedbackStatus(id int, status string) error {
	return store.UpdateDocFeedbackStatus(s.db, id, status)
}

// GetDocFeedbackStats 按文档统计反馈数量
func (s *Service) GetDocFeedbackStats() ([]api.DocFeedbackStatsItem, error) {
	return store.GetDocFeedbackStats(s.db)
}
