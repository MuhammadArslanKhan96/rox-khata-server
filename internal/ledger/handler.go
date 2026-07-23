package ledger

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// LedgerHandler handles the HTTP routing and request processing.
type LedgerHandler struct {
	service LedgerService
}

// NewLedgerHandler initializes a new LedgerHandler.
func NewLedgerHandler(service LedgerService) *LedgerHandler {
	return &LedgerHandler{service: service}
}

// RegisterRoutes attaches the ledger API endpoints to the provided Gin engine.
func (h *LedgerHandler) RegisterRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1")
	{
		v1.POST("/tenants/register", h.RegisterTenant)
		v1.POST("/accounts", h.CreateAccount)
		v1.GET("/accounts/:id", h.GetAccount)
		v1.POST("/transfers", h.Transfer)
		v1.GET("/entries", h.GetStatement)
		v1.POST("/sync/items", h.SyncItem)
		v1.POST("/sync/banks", h.SyncBank)
		v1.POST("/sync/team-members", h.SyncTeamMember)

		// GET endpoints for two-way synchronization
		v1.GET("/sync/items", h.GetItems)
		v1.GET("/sync/banks", h.GetBanks)
		v1.GET("/sync/team-members", h.GetTeamMembers)
		v1.GET("/sync/accounts", h.GetAccounts)
	}
}

// RegisterTenant handles requests to register a new tenant and owner account.
func (h *LedgerHandler) RegisterTenant(c *gin.Context) {
	var req RegisterTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	if err := h.service.RegisterTenant(c.Request.Context(), req); err != nil {
		h.respondWithError(c, http.StatusConflict, "Registration failed: Phone number or Email already registered", err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "message": "Tenant registered successfully", "phone": req.Phone})
}

// CreateAccount handles requests to create a new ledger account.
func (h *LedgerHandler) CreateAccount(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	account, err := h.service.CreateAccount(c.Request.Context(), req)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to create ledger account", err.Error())
		return
	}

	c.JSON(http.StatusCreated, account)
}

// GetAccount retrieves account details by ID.
func (h *LedgerHandler) GetAccount(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid account ID format", "Account ID must be a numeric integer")
		return
	}

	account, err := h.service.GetAccount(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			h.respondWithError(c, http.StatusNotFound, "Account not found", err.Error())
			return
		}
		h.respondWithError(c, http.StatusInternalServerError, "Failed to fetch ledger account", err.Error())
		return
	}

	c.JSON(http.StatusOK, account)
}

// Transfer transfers funds atomically between two accounts.
func (h *LedgerHandler) Transfer(c *gin.Context) {
	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	entry, err := h.service.Transfer(c.Request.Context(), req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		
		// Map domain errors to appropriate HTTP status codes
		switch {
		case errors.Is(err, ErrAccountNotFound):
			statusCode = http.StatusNotFound
		case errors.Is(err, ErrTenantMismatch):
			statusCode = http.StatusBadRequest
		case errors.Is(err, ErrInsufficientFunds):
			statusCode = http.StatusBadRequest
		case errors.Is(err, ErrSameAccountTransfer):
			statusCode = http.StatusBadRequest
		case errors.Is(err, ErrInvalidAmount):
			statusCode = http.StatusBadRequest
		}

		h.respondWithError(c, statusCode, "Transfer failed", err.Error())
		return
	}

	c.JSON(http.StatusOK, entry)
}

// GetStatement lists the transaction history for an account.
func (h *LedgerHandler) GetStatement(c *gin.Context) {
	businessID := c.Query("business_id")
	accountIDStr := c.Query("account_id")

	if businessID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Missing business_id parameter", "Query parameter 'business_id' is required")
		return
	}

	if accountIDStr == "" {
		h.respondWithError(c, http.StatusBadRequest, "Missing account_id parameter", "Query parameter 'account_id' is required")
		return
	}

	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid account_id format", "Query parameter 'account_id' must be a numeric integer")
		return
	}

	entries, err := h.service.GetStatement(c.Request.Context(), businessID, accountID)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, ErrAccountNotFound) {
			statusCode = http.StatusNotFound
		} else if errors.Is(err, ErrTenantMismatch) {
			statusCode = http.StatusBadRequest
		}
		h.respondWithError(c, statusCode, "Failed to retrieve statement", err.Error())
		return
	}

	if entries == nil {
		entries = []JournalEntry{} // Return empty list instead of null in JSON response
	}

	c.JSON(http.StatusOK, entries)
}

// respondWithError builds and sends a standard error response.
func (h *LedgerHandler) respondWithError(c *gin.Context, statusCode int, message string, details string) {
	c.JSON(statusCode, APIError{
		Error:   message,
		Details: details,
	})
}

// SyncItem handles sync requests for items.
func (h *LedgerHandler) SyncItem(c *gin.Context) {
	var req ItemSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	if err := h.service.SyncItem(c.Request.Context(), req); err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to sync item", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "id": req.ID})
}

// SyncBank handles sync requests for banks.
func (h *LedgerHandler) SyncBank(c *gin.Context) {
	var req BankSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	if err := h.service.SyncBank(c.Request.Context(), req); err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to sync bank", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "id": req.ID})
}

// SyncTeamMember handles sync requests for team members.
func (h *LedgerHandler) SyncTeamMember(c *gin.Context) {
	var req TeamMemberSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	if err := h.service.SyncTeamMember(c.Request.Context(), req); err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to sync team member", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "id": req.ID})
}

// GetItems retrieves all items synced for the given business.
func (h *LedgerHandler) GetItems(c *gin.Context) {
	businessID := c.Query("business_id")
	if businessID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Missing business_id parameter", "Query parameter 'business_id' is required")
		return
	}

	items, err := h.service.GetItems(c.Request.Context(), businessID)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to fetch items", err.Error())
		return
	}

	if items == nil {
		items = []ItemSyncRequest{}
	}
	c.JSON(http.StatusOK, items)
}

// GetBanks retrieves all banks synced for the given business.
func (h *LedgerHandler) GetBanks(c *gin.Context) {
	businessID := c.Query("business_id")
	if businessID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Missing business_id parameter", "Query parameter 'business_id' is required")
		return
	}

	banks, err := h.service.GetBanks(c.Request.Context(), businessID)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to fetch banks", err.Error())
		return
	}

	if banks == nil {
		banks = []BankSyncRequest{}
	}
	c.JSON(http.StatusOK, banks)
}

// GetTeamMembers retrieves all team members synced for the given business.
func (h *LedgerHandler) GetTeamMembers(c *gin.Context) {
	businessID := c.Query("business_id")
	if businessID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Missing business_id parameter", "Query parameter 'business_id' is required")
		return
	}

	members, err := h.service.GetTeamMembers(c.Request.Context(), businessID)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to fetch team members", err.Error())
		return
	}

	if members == nil {
		members = []TeamMemberSyncRequest{}
	}
	c.JSON(http.StatusOK, members)
}

// GetAccounts retrieves all party accounts synced for the given business.
func (h *LedgerHandler) GetAccounts(c *gin.Context) {
	businessID := c.Query("business_id")
	if businessID == "" {
		h.respondWithError(c, http.StatusBadRequest, "Missing business_id parameter", "Query parameter 'business_id' is required")
		return
	}

	accounts, err := h.service.GetAccounts(c.Request.Context(), businessID)
	if err != nil {
		h.respondWithError(c, http.StatusInternalServerError, "Failed to fetch accounts", err.Error())
		return
	}

	if accounts == nil {
		accounts = []LedgerAccount{}
	}
	c.JSON(http.StatusOK, accounts)
}
