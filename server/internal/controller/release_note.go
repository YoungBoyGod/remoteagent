package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

func CreateReleaseNoteHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.ReleaseNoteCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}
		item, err := svc.CreateReleaseNote(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, item)
	}
}

func ListReleaseNotesHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.ReleaseNoteListRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid query params: "+err.Error())
			return
		}
		resp, err := svc.ListReleaseNotes(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, resp)
	}
}

func GetReleaseNoteHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}
		item, err := svc.GetReleaseNote(id)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		if item == nil {
			Fail(c, http.StatusNotFound, 404, "release note not found")
			return
		}
		OK(c, item)
	}
}

func UpdateReleaseNoteHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}
		var req api.ReleaseNoteUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}
		if err := svc.UpdateReleaseNote(id, req); err != nil {
			if err.Error() == "release note not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

func DeleteReleaseNoteHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}
		if err := svc.DeleteReleaseNote(id); err != nil {
			if err.Error() == "release note not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}
