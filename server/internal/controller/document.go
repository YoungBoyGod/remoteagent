package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
	"luoyi2026/server/internal/storage"
)

// ============================================================
// 文档分类
// ============================================================

// ListDocCategoriesHandler godoc
// @Summary 分类树
// @Tags doc-categories
// @Produce json
// @Success 200 {object} api.Envelope
// @Router /api/v1/docs/categories [get]
func ListDocCategoriesHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tree, err := svc.ListDocCategoryTree()
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, tree)
	}
}

// CreateDocCategoryHandler godoc
// @Summary 创建分类
// @Tags doc-categories
// @Accept json
// @Produce json
// @Param body body api.DocCategoryCreateRequest true "分类创建请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Router /api/v1/docs/categories [post]
func CreateDocCategoryHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.DocCategoryCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		id, err := svc.CreateDocCategory(req)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, err.Error())
			return
		}
		OK(c, map[string]any{"id": id})
	}
}

// UpdateDocCategoryHandler godoc
// @Summary 更新分类
// @Tags doc-categories
// @Accept json
// @Produce json
// @Param id path int true "分类ID"
// @Param body body api.DocCategoryUpdateRequest true "分类更新请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Router /api/v1/docs/categories/{id} [put]
func UpdateDocCategoryHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}
		var req api.DocCategoryUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		if err := svc.UpdateDocCategory(id, req); err != nil {
			if err.Error() == "category not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// DeleteDocCategoryHandler godoc
// @Summary 删除分类
// @Tags doc-categories
// @Produce json
// @Param id path int true "分类ID"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/categories/{id} [delete]
func DeleteDocCategoryHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}
		if err := svc.DeleteDocCategory(id); err != nil {
			if err.Error() == "category not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// ============================================================
// 文档 CRUD
// ============================================================

// docCreateBody 创建文档请求体（含 Markdown 内容）
type docCreateBody struct {
	api.DocCreateRequest
	Content string `json:"content"`
}

// docUpdateBody 更新文档请求体（含 Markdown 内容）
type docUpdateBody struct {
	api.DocUpdateRequest
	Content *string `json:"content"`
}

// ListDocsHandler godoc
// @Summary 文档列表
// @Tags documents
// @Produce json
// @Param category_id query int false "分类ID"
// @Param status query string false "状态筛选"
// @Param search query string false "标题/slug 模糊搜索"
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20"
// @Success 200 {object} api.Envelope
// @Router /api/v1/docs [get]
func ListDocsHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.DocListRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid query params: "+err.Error())
			return
		}
		data, err := svc.ListDocuments(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, data)
	}
}

// GetDocHandler godoc
// @Summary 文档详情
// @Tags documents
// @Produce json
// @Param slug path string true "文档 slug"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/{slug} [get]
func GetDocHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			Fail(c, http.StatusBadRequest, 400, "slug required")
			return
		}
		doc, content, err := svc.GetDocumentBySlug(slug, sto)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		if doc == nil {
			Fail(c, http.StatusNotFound, 404, "document not found")
			return
		}
		result := map[string]any{
			"id": doc.ID, "slug": doc.Slug, "title": doc.Title,
			"category_id": doc.CategoryID, "category_name": doc.CategoryName,
			"content_key": doc.ContentKey, "format": doc.Format,
			"language": doc.Language, "author": doc.Author,
			"status": doc.Status, "sort_order": doc.SortOrder,
			"metadata": doc.Metadata, "created_at": doc.CreatedAt,
			"updated_at": doc.UpdatedAt, "content": content,
		}
		OK(c, result)
	}
}

// CreateDocHandler godoc
// @Summary 创建文档
// @Tags documents
// @Accept json
// @Produce json
// @Param body body docCreateBody true "文档创建请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Router /api/v1/docs [post]
func CreateDocHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body docCreateBody
		if err := c.ShouldBindJSON(&body); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		id, err := svc.CreateDocument(body.DocCreateRequest, body.Content, sto)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, err.Error())
			return
		}
		doc, _, _ := svc.GetDocumentBySlug(body.Slug, sto)
		if doc != nil {
			OK(c, doc)
			return
		}
		OK(c, map[string]any{"id": id})
	}
}

// UpdateDocHandler godoc
// @Summary 更新文档
// @Tags documents
// @Accept json
// @Produce json
// @Param slug path string true "文档 slug"
// @Param body body docUpdateBody true "文档更新请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/{slug} [put]
func UpdateDocHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			Fail(c, http.StatusBadRequest, 400, "slug required")
			return
		}
		var body docUpdateBody
		if err := c.ShouldBindJSON(&body); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		if err := svc.UpdateDocument(slug, body.DocUpdateRequest, body.Content, sto); err != nil {
			if err.Error() == "document not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		doc, _, _ := svc.GetDocumentBySlug(slug, sto)
		OK(c, doc)
	}
}

// DeleteDocHandler godoc
// @Summary 删除文档
// @Tags documents
// @Produce json
// @Param slug path string true "文档 slug"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/{slug} [delete]
func DeleteDocHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			Fail(c, http.StatusBadRequest, 400, "slug required")
			return
		}
		if err := svc.DeleteDocument(slug, sto); err != nil {
			if err.Error() == "document not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// ============================================================
// 文档版本
// ============================================================

// ListDocVersionsHandler godoc
// @Summary 版本列表
// @Tags doc-versions
// @Produce json
// @Param slug path string true "文档 slug"
// @Success 200 {object} api.Envelope
// @Router /api/v1/docs/{slug}/versions [get]
func ListDocVersionsHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			Fail(c, http.StatusBadRequest, 400, "slug required")
			return
		}
		items, err := svc.ListDocVersions(slug)
		if err != nil {
			if err.Error() == "document not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, items)
	}
}

// GetDocVersionHandler godoc
// @Summary 指定版本内容
// @Tags doc-versions
// @Produce json
// @Param slug path string true "文档 slug"
// @Param version path int true "版本号"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/{slug}/versions/{version} [get]
func GetDocVersionHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		version := c.Param("version")
		if slug == "" || version == "" {
			Fail(c, http.StatusBadRequest, 400, "invalid slug or version")
			return
		}
		item, content, err := svc.GetDocVersionContent(slug, version, sto)
		if err != nil {
			if err.Error() == "document not found" || err.Error() == "version not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, map[string]any{
			"version": item,
			"content": content,
		})
	}
}

// CreateDocVersionHandler godoc
// @Summary 创建版本快照
// @Tags doc-versions
// @Accept json
// @Produce json
// @Param slug path string true "文档 slug"
// @Param body body api.DocVersionCreateRequest true "版本创建请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Router /api/v1/docs/{slug}/versions [post]
func CreateDocVersionHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			Fail(c, http.StatusBadRequest, 400, "slug required")
			return
		}
		var req api.DocVersionCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		id, err := svc.CreateDocVersion(slug, req, sto)
		if err != nil {
			if err.Error() == "document not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusBadRequest, 400, err.Error())
			return
		}
		OK(c, map[string]any{"id": id})
	}
}

// ============================================================
// 文档附件
// ============================================================

// UploadAttachmentHandler godoc
// @Summary 上传附件
// @Tags doc-attachments
// @Accept multipart/form-data
// @Produce json
// @Param slug path string true "文档 slug"
// @Param file formData file true "附件文件"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Router /api/v1/docs/{slug}/attachments [post]
func UploadAttachmentHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			Fail(c, http.StatusBadRequest, 400, "slug required")
			return
		}
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "file required: "+err.Error())
			return
		}
		defer file.Close()

		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		att, err := svc.UploadAttachment(slug, header.Filename, file, contentType, header.Size, sto)
		if err != nil {
			if err.Error() == "document not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, att)
	}
}

// GetAttachmentHandler godoc
// @Summary 获取附件预签名URL
// @Tags doc-attachments
// @Produce json
// @Param id path int true "附件ID"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/attachments/{id} [get]
func GetAttachmentHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}
		url, err := svc.GetAttachmentURL(id, sto)
		if err != nil {
			if err.Error() == "attachment not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, map[string]any{"url": url})
	}
}

// DeleteAttachmentHandler godoc
// @Summary 删除附件
// @Tags doc-attachments
// @Produce json
// @Param id path int true "附件ID"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/attachments/{id} [delete]
func DeleteAttachmentHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}
		if err := svc.DeleteAttachment(id, sto); err != nil {
			if err.Error() == "attachment not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// ============================================================
// 文档反馈
// ============================================================

// CreateDocFeedbackHandler godoc
// @Summary 提交反馈
// @Tags doc-feedback
// @Accept json
// @Produce json
// @Param slug path string true "文档 slug"
// @Param body body api.DocFeedbackCreateRequest true "反馈请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Router /api/v1/docs/{slug}/feedback [post]
func CreateDocFeedbackHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			Fail(c, http.StatusBadRequest, 400, "slug required")
			return
		}
		var req api.DocFeedbackCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		id, err := svc.CreateDocFeedback(slug, req)
		if err != nil {
			if err.Error() == "document not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusBadRequest, 400, err.Error())
			return
		}
		OK(c, map[string]any{"id": id})
	}
}

// ListDocFeedbackHandler godoc
// @Summary 反馈列表
// @Tags doc-feedback
// @Produce json
// @Param document_id query int false "文档ID"
// @Param status query string false "状态筛选"
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Success 200 {object} api.Envelope
// @Router /api/v1/docs/feedback [get]
func ListDocFeedbackHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.DocFeedbackListRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid query params: "+err.Error())
			return
		}
		data, err := svc.ListDocFeedback(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, data)
	}
}

// UpdateDocFeedbackHandler godoc
// @Summary 处理反馈
// @Tags doc-feedback
// @Accept json
// @Produce json
// @Param id path int true "反馈ID"
// @Param body body api.DocFeedbackUpdateRequest true "反馈状态更新"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/feedback/{id} [put]
func UpdateDocFeedbackHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}
		var req api.DocFeedbackUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		if err := svc.UpdateDocFeedbackStatus(id, req.Status); err != nil {
			if err.Error() == "feedback not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// ============================================================
// 版本 Diff
// ============================================================

// DiffDocVersionsHandler godoc
// @Summary 版本差异对比
// @Tags doc-versions
// @Produce json
// @Param slug path string true "文档 slug"
// @Param from query string true "起始版本（如 v1 或 latest）"
// @Param to query string true "目标版本（如 v2 或 latest）"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/{slug}/diff [get]
func DiffDocVersionsHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			Fail(c, http.StatusBadRequest, 400, "slug required")
			return
		}
		from := c.Query("from")
		to := c.Query("to")
		if from == "" || to == "" {
			Fail(c, http.StatusBadRequest, 400, "from and to query params required")
			return
		}
		diff, err := svc.DiffDocVersions(slug, from, to, sto)
		if err != nil {
			if err.Error() == "document not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, diff)
	}
}

// ============================================================
// PDF / HTML 导出
// ============================================================

// ExportDocHTMLHandler godoc
// @Summary 导出文档为 HTML
// @Tags documents
// @Produce json
// @Param slug path string true "文档 slug"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/docs/{slug}/export/html [get]
func ExportDocHTMLHandler(svc *service.Service, sto storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			Fail(c, http.StatusBadRequest, 400, "slug required")
			return
		}
		result, err := svc.ExportDocHTML(slug, sto)
		if err != nil {
			if err.Error() == "document not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, result)
	}
}

// ============================================================
// 反馈统计
// ============================================================

// DocFeedbackStatsHandler godoc
// @Summary 反馈统计（按文档）
// @Tags doc-feedback
// @Produce json
// @Success 200 {object} api.Envelope
// @Router /api/v1/docs/feedback/stats [get]
func DocFeedbackStatsHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := svc.GetDocFeedbackStats()
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, stats)
	}
}
