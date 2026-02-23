package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// CreateCustomerHandler POST /api/v1/customers
func CreateCustomerHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.CustomerCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		customerID, err := svc.CreateCustomer(req)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, err.Error())
			return
		}
		OK(c, map[string]any{"customer_id": customerID})
	}
}

// ListCustomersHandler GET /api/v1/customers
func ListCustomersHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.CustomerListRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid query params: "+err.Error())
			return
		}
		data, err := svc.ListCustomers(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, data)
	}
}

// GetCustomerHandler GET /api/v1/customers/:id
func GetCustomerHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			Fail(c, http.StatusBadRequest, 400, "customer id required")
			return
		}
		item, err := svc.GetCustomer(id)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		if item == nil {
			Fail(c, http.StatusNotFound, 404, "customer not found")
			return
		}
		OK(c, item)
	}
}

// UpdateCustomerHandler PUT /api/v1/customers/:id
func UpdateCustomerHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			Fail(c, http.StatusBadRequest, 400, "customer id required")
			return
		}
		var req api.CustomerUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		if err := svc.UpdateCustomer(id, req); err != nil {
			if err.Error() == "customer not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// DeleteCustomerHandler DELETE /api/v1/customers/:id
func DeleteCustomerHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			Fail(c, http.StatusBadRequest, 400, "customer id required")
			return
		}
		if err := svc.DeleteCustomer(id); err != nil {
			if err.Error() == "customer not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusBadRequest, 400, err.Error())
			return
		}
		OK(c, nil)
	}
}

// AssignHostHandler POST /api/v1/customers/:id/hosts
func AssignHostHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			Fail(c, http.StatusBadRequest, 400, "customer id required")
			return
		}
		var req api.CustomerHostAssignRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		if err := svc.AssignHost(id, req); err != nil {
			Fail(c, http.StatusBadRequest, 400, err.Error())
			return
		}
		OK(c, nil)
	}
}

// UnassignHostHandler DELETE /api/v1/customers/:id/hosts/:host_id
func UnassignHostHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		hostID := c.Param("host_id")
		if id == "" || hostID == "" {
			Fail(c, http.StatusBadRequest, 400, "customer id and host_id required")
			return
		}
		if err := svc.UnassignHost(id, hostID); err != nil {
			if err.Error() == "assignment not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// ListCustomerHostsHandler GET /api/v1/customers/:id/hosts
func ListCustomerHostsHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			Fail(c, http.StatusBadRequest, 400, "customer id required")
			return
		}
		items, err := svc.ListCustomerHosts(id)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, map[string]any{"items": items})
	}
}
