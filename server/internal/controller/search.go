package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/search"
)

// SearchDocsHandler godoc
// @Summary 全文搜索文档
// @Tags docs
// @Produce json
// @Param q query string true "搜索关键词"
// @Param category query int false "分类ID筛选"
// @Param lang query string false "语言筛选"
// @Param status query string false "状态筛选"
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20"
// @Success 200 {object} api.Envelope
// @Router /api/v1/docs/search [get]
func SearchDocsHandler(sc *search.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		if q == "" {
			Fail(c, http.StatusBadRequest, 400, "q is required")
			return
		}

		var categoryID *int
		if catStr := c.Query("category"); catStr != "" {
			v, err := strconv.Atoi(catStr)
			if err != nil {
				Fail(c, http.StatusBadRequest, 400, "invalid category")
				return
			}
			categoryID = &v
		}

		lang := c.Query("lang")
		status := c.Query("status")

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

		result, err := sc.Search(q, categoryID, lang, status, page, pageSize)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, "search failed: "+err.Error())
			return
		}
		OK(c, result)
	}
}

// SuggestDocsHandler godoc
// @Summary 搜索建议（自动补全）
// @Tags docs
// @Produce json
// @Param q query string true "搜索关键词"
// @Param limit query int false "返回条数，默认5"
// @Success 200 {object} api.Envelope
// @Router /api/v1/docs/search/suggest [get]
func SuggestDocsHandler(sc *search.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		if q == "" {
			Fail(c, http.StatusBadRequest, 400, "q is required")
			return
		}

		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))

		result, err := sc.Suggest(q, limit)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, "suggest failed: "+err.Error())
			return
		}
		OK(c, result)
	}
}
