package storage

import (
	"context"
	"io"
	"time"
)

// ObjectInfo 对象元信息
type ObjectInfo struct {
	Key          string    // 对象键
	Size         int64     // 字节大小
	ContentType  string    // MIME 类型
	LastModified time.Time // 最后修改时间
	ETag         string    // ETag 校验值
}

// Storage 对象存储接口，兼容 S3/MinIO/RustFS
type Storage interface {
	// PutObject 上传对象
	PutObject(ctx context.Context, key string, reader io.Reader, contentType string) error
	// GetObject 下载对象，调用方需负责关闭返回的 ReadCloser
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	// DeleteObject 删除对象
	DeleteObject(ctx context.Context, key string) error
	// GetPresignedURL 生成预签名下载链接
	GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	// CopyObject 复制对象
	CopyObject(ctx context.Context, srcKey, dstKey string) error
	// ListObjects 按前缀列举对象
	ListObjects(ctx context.Context, prefix string) ([]ObjectInfo, error)
}

// 存储路径约定
const (
	// PathDocLatest 文档最新版: documents/{slug}/latest.md
	PathDocLatest = "documents/%s/latest.md"
	// PathDocVersion 文档历史版本: documents/{slug}/versions/{version}.md
	PathDocVersion = "documents/%s/versions/%s.md"
	// PathAttachment 附件: attachments/{doc_id}/{filename}
	PathAttachment = "attachments/%s/%s"
	// PathExport 导出文件: exports/{slug}-{version}.pdf
	PathExport = "exports/%s-%s.pdf"
)
