package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/middleware"
	"github.com/GordenArcher/payfake/internal/response"
	"github.com/GordenArcher/payfake/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ControlHandler struct {
	db          *gorm.DB
	scenarioSvc *service.ScenarioService
	webhookSvc  *service.WebhookService
	txSvc       *service.TransactionService
	logSvc      *service.LogService
	authSvc     *service.AuthService
	customerSvc *service.CustomerService
}

func NewControlHandler(
	db *gorm.DB,
	scenarioSvc *service.ScenarioService,
	webhookSvc *service.WebhookService,
	txSvc *service.TransactionService,
	logSvc *service.LogService,
	authSvc *service.AuthService,
	customerSvc *service.CustomerService,
) *ControlHandler {
	return &ControlHandler{
		db:          db,
		scenarioSvc: scenarioSvc,
		webhookSvc:  webhookSvc,
		txSvc:       txSvc,
		logSvc:      logSvc,
		authSvc:     authSvc,
		customerSvc: customerSvc,
	}
}

// GetScenario handles GET /api/v1/control/scenario
func (h *ControlHandler) GetScenario(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	scenario, err := h.scenarioSvc.Get(merchantID)
	if err != nil {
		response.InternalErr(c, "Failed to fetch scenario config")
		return
	}

	response.Success(c, http.StatusOK, "Scenario config fetched",
		response.ScenarioFetched, scenario)
}

type updateScenarioRequest struct {
	FailureRate *float64 `json:"failure_rate"`
	DelayMS     *int     `json:"delay_ms"`
	ForceStatus *string  `json:"force_status"`
	ErrorCode   *string  `json:"error_code"`
}

// UpdateScenario handles PUT /api/v1/control/scenario
func (h *ControlHandler) UpdateScenario(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	var req updateScenarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	scenario, err := h.scenarioSvc.Update(merchantID, service.UpdateScenarioInput{
		FailureRate: req.FailureRate,
		DelayMS:     req.DelayMS,
		ForceStatus: req.ForceStatus,
		ErrorCode:   req.ErrorCode,
	})

	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidScenarioConfig):
			response.Error(c, http.StatusUnprocessableEntity,
				"Invalid scenario configuration",
				response.ScenarioInvalidConfig, []response.ErrorField{
					{Field: "failure_rate", Message: "Must be between 0.0 and 1.0"},
					{Field: "delay_ms", Message: "Must be between 0 and 30000"},
				})
		case errors.Is(err, service.ErrInvalidForceStatus):
			response.Error(c, http.StatusUnprocessableEntity,
				"Invalid force status",
				response.TransactionForceInvalidStatus, []response.ErrorField{
					{Field: "force_status", Message: "Valid values: success, failed, abandoned"},
				})
		default:
			response.InternalErr(c, "Failed to update scenario config")
		}
		return
	}

	response.Success(c, http.StatusOK, "Scenario config updated",
		response.ScenarioUpdated, scenario)
}

// ResetScenario handles POST /api/v1/control/scenario/reset
func (h *ControlHandler) ResetScenario(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	scenario, err := h.scenarioSvc.Reset(merchantID)
	if err != nil {
		response.InternalErr(c, "Failed to reset scenario config")
		return
	}

	response.Success(c, http.StatusOK, "Scenario config reset to defaults",
		response.ScenarioReset, scenario)
}

// ListWebhooks handles GET /api/v1/control/webhooks
func (h *ControlHandler) ListWebhooks(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	webhooks, total, err := h.webhookSvc.List(merchantID, page, perPage)
	if err != nil {
		response.InternalErr(c, "Failed to fetch webhooks")
		return
	}

	response.Success(c, http.StatusOK, "Webhooks fetched",
		response.WebhookListFetched, gin.H{
			"webhooks": webhooks,
			"meta": gin.H{
				"total":    total,
				"page":     page,
				"per_page": perPage,
				"pages":    (total + int64(perPage) - 1) / int64(perPage),
			},
		})
}

// RetryWebhook handles POST /api/v1/control/webhooks/:id/retry
func (h *ControlHandler) RetryWebhook(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	id := c.Param("id")

	if err := h.webhookSvc.Retry(id, merchantID); err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			response.NotFoundErr(c, "Webhook event not found")
			return
		}
		response.InternalErr(c, "Failed to retry webhook")
		return
	}

	response.Success(c, http.StatusOK, "Webhook retry triggered",
		response.WebhookRetried, nil)
}

// GetWebhookAttempts handles GET /api/v1/control/webhooks/:id/attempts
func (h *ControlHandler) GetWebhookAttempts(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	id := c.Param("id")

	attempts, err := h.webhookSvc.GetAttempts(id, merchantID)
	if err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			response.NotFoundErr(c, "Webhook event not found")
			return
		}
		response.InternalErr(c, "Failed to fetch webhook attempts")
		return
	}

	response.Success(c, http.StatusOK, "Webhook attempts fetched",
		response.WebhookAttemptsFetched, gin.H{
			"attempts": attempts,
		})
}

type forceTransactionRequest struct {
	Status    string `json:"status" binding:"required"`
	ErrorCode string `json:"error_code"`
}

// ForceTransaction handles POST /api/v1/control/transactions/:ref/force
// This is the killer feature, force any pending transaction to any
// terminal state deterministically, bypassing the scenario engine entirely.
func (h *ControlHandler) ForceTransaction(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	ref := c.Param("ref")

	var req forceTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	tx, err := h.txSvc.ForceOutcome(ref, merchantID, req.Status, req.ErrorCode)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTransactionNotFound):
			response.NotFoundErr(c, "Transaction not found")
		case errors.Is(err, service.ErrInvalidForceStatus):
			response.Error(c, http.StatusUnprocessableEntity,
				"Invalid force status",
				response.TransactionForceInvalidStatus, []response.ErrorField{
					{Field: "status", Message: "Valid values: success, failed, abandoned"},
				})
		case errors.Is(err, service.ErrTransactionNotPending):
			response.Error(c, http.StatusConflict,
				"Only pending transactions can be forced",
				response.TransactionAlreadyVerified, []response.ErrorField{})
		default:
			response.InternalErr(c, "Failed to force transaction outcome")
		}
		return
	}

	response.Success(c, http.StatusOK, "Transaction outcome forced",
		response.TransactionForced, tx)
}

// GetLogs handles GET /api/v1/control/logs
func (h *ControlHandler) GetLogs(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	logs, total, err := h.logSvc.List(merchantID, page, perPage)
	if err != nil {
		response.InternalErr(c, "Failed to fetch logs")
		return
	}

	if total == 0 {
		response.Error(c, http.StatusNotFound, "No logs found",
			response.LogsEmpty, []response.ErrorField{})
		return
	}

	response.Success(c, http.StatusOK, "Logs fetched",
		response.LogsFetched, gin.H{
			"logs": logs,
			"meta": gin.H{
				"total":    total,
				"page":     page,
				"per_page": perPage,
				"pages":    (total + int64(perPage) - 1) / int64(perPage),
			},
		})
}

// ClearLogs handles DELETE /api/v1/control/logs
func (h *ControlHandler) ClearLogs(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired token")
		return
	}

	if err := h.logSvc.Clear(merchantID); err != nil {
		response.InternalErr(c, "Failed to clear logs")
		return
	}

	response.Success(c, http.StatusOK, "Logs cleared",
		response.LogsCleared, nil)
}

// ListTransactions handles GET /api/v1/control/transactions
// JWT-authenticated version of transaction list for the dashboard.
// The regular /transaction endpoint requires secret key — this one
// uses the dashboard JWT so the dashboard doesn't need to store the secret key.
func (h *ControlHandler) ListTransactions(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	status := domain.TransactionStatus(c.Query("status"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	transactions, total, err := h.txSvc.List(merchantID, status, page, perPage)
	if err != nil {
		response.InternalErr(c, "Failed to fetch transactions")
		return
	}

	response.Success(c, http.StatusOK, "Transactions fetched",
		response.TransactionListFetched, gin.H{
			"transactions": transactions,
			"meta": gin.H{
				"total":    total,
				"page":     page,
				"per_page": perPage,
				"pages":    (total + int64(perPage) - 1) / int64(perPage),
			},
		})
}

// ListCustomers handles GET /api/v1/control/customers
// JWT-authenticated customer list for the dashboard.
func (h *ControlHandler) ListCustomers(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	customers, total, err := h.customerSvc.List(merchantID, page, perPage)
	if err != nil {
		response.InternalErr(c, "Failed to fetch customers")
		return
	}

	response.Success(c, http.StatusOK, "Customers fetched",
		response.CustomerListFetched, gin.H{
			"customers": customers,
			"meta": gin.H{
				"total":    total,
				"page":     page,
				"per_page": perPage,
				"pages":    (total + int64(perPage) - 1) / int64(perPage),
			},
		})
}
