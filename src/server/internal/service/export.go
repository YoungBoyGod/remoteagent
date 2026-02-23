package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yuin/goldmark"

	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/storage"
	"luoyi2026/server/internal/store"
)

// ExportDocHTML 导出文档为 HTML（Markdown → HTML）
func (s *Service) ExportDocHTML(slug string, sto storage.Storage) (*api.DocExportHTMLResponse, error) {
	doc, err := store.GetDocumentBySlug(s.db, slug)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found")
	}

	// 从 S3 读取 Markdown 内容
	mdContent, err := s.readDocContent(doc.ContentKey, sto)
	if err != nil {
		return nil, fmt.Errorf("读取文档内容失败: %w", err)
	}

	// Markdown → HTML
	html, err := markdownToHTML(mdContent)
	if err != nil {
		return nil, fmt.Errorf("Markdown 转 HTML 失败: %w", err)
	}

	// 包装完整 HTML 页面
	fullHTML := wrapHTMLPage(doc.Title, html)

	// 缓存到 S3 exports/ 目录
	if sto != nil {
		exportKey := fmt.Sprintf("exports/%s-latest.html", slug)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = sto.PutObject(ctx, exportKey, strings.NewReader(fullHTML), "text/html")
	}

	return &api.DocExportHTMLResponse{
		Slug:  slug,
		Title: doc.Title,
		HTML:  fullHTML,
	}, nil
}

// readDocContent 从 S3 读取文档内容
func (s *Service) readDocContent(contentKey string, sto storage.Storage) (string, error) {
	if sto == nil {
		return "", fmt.Errorf("storage not configured")
	}
	if contentKey == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rc, err := sto.GetObject(ctx, contentKey)
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

// markdownToHTML 使用 goldmark 将 Markdown 转换为 HTML
func markdownToHTML(md string) (string, error) {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(md), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// wrapHTMLPage 包装为完整 HTML 页面
func wrapHTMLPage(title, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 800px; margin: 0 auto; padding: 2rem; line-height: 1.6; color: #333; }
  pre { background: #f5f5f5; padding: 1rem; border-radius: 4px; overflow-x: auto; }
  code { background: #f5f5f5; padding: 0.2em 0.4em; border-radius: 3px; font-size: 0.9em; }
  pre code { background: none; padding: 0; }
  table { border-collapse: collapse; width: 100%%; }
  th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
  th { background: #f5f5f5; }
  img { max-width: 100%%; }
  blockquote { border-left: 4px solid #ddd; margin: 0; padding-left: 1rem; color: #666; }
</style>
</head>
<body>
<h1>%s</h1>
%s
</body>
</html>`, title, title, body)
}
