package erp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ERPHandler struct {
	service ERPService
}

func NewERPHandler(service ERPService) *ERPHandler {
	return &ERPHandler{service: service}
}

func (h *ERPHandler) RegisterRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1/erp")
	{
		v1.POST("/products", h.SaveProduct)
		v1.GET("/dropdowns", h.GetDropdownData)
		v1.POST("/categories", h.SaveCategory)
		v1.POST("/brands", h.SaveBrand)
		v1.POST("/units", h.SaveUnit)
		v1.POST("/warehouses", h.SaveWarehouse)
	}
}

func (h *ERPHandler) SaveProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}
	if req.BusinessID == "" {
		req.BusinessID = "rox_khata_business"
	}

	product, err := h.service.SaveProduct(c.Request.Context(), req)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to save product", err.Error())
		return
	}

	c.JSON(http.StatusCreated, product)
}

func (h *ERPHandler) GetDropdownData(c *gin.Context) {
	businessID := c.Query("business_id")
	if businessID == "" {
		businessID = "rox_khata_business"
	}

	data, err := h.service.GetDropdownData(c.Request.Context(), businessID)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to fetch dropdown data", err.Error())
		return
	}

	c.JSON(http.StatusOK, data)
}

func (h *ERPHandler) SaveCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid category request payload", err.Error())
		return
	}
	if req.BusinessID == "" {
		req.BusinessID = "rox_khata_business"
	}
	cat, err := h.service.SaveCategory(c.Request.Context(), req)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to save category", err.Error())
		return
	}
	c.JSON(http.StatusCreated, cat)
}

func (h *ERPHandler) SaveBrand(c *gin.Context) {
	var req CreateBrandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid brand request payload", err.Error())
		return
	}
	if req.BusinessID == "" {
		req.BusinessID = "rox_khata_business"
	}
	brand, err := h.service.SaveBrand(c.Request.Context(), req)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to save brand", err.Error())
		return
	}
	c.JSON(http.StatusCreated, brand)
}

func (h *ERPHandler) SaveUnit(c *gin.Context) {
	var req CreateUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid unit request payload", err.Error())
		return
	}
	if req.BusinessID == "" {
		req.BusinessID = "rox_khata_business"
	}
	unit, err := h.service.SaveUnit(c.Request.Context(), req)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to save unit", err.Error())
		return
	}
	c.JSON(http.StatusCreated, unit)
}

func (h *ERPHandler) SaveWarehouse(c *gin.Context) {
	var req CreateWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid warehouse request payload", err.Error())
		return
	}
	if req.BusinessID == "" {
		req.BusinessID = "rox_khata_business"
	}
	wh, err := h.service.SaveWarehouse(c.Request.Context(), req)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to save warehouse", err.Error())
		return
	}
	c.JSON(http.StatusCreated, wh)
}

func (h *ERPHandler) respondWithError(c *gin.Context, statusCode int, message string, details string) {
	c.JSON(statusCode, APIError{
		Error:   message,
		Details: details,
	})
}
