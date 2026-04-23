package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/middleware"
	"github.com/payfake/payfake-api/internal/repository"
	"github.com/payfake/payfake-api/internal/response"
	"github.com/payfake/payfake-api/internal/service"
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
	otpRepo     *repository.OTPRepository
}

func NewControlHandler(
	db *gorm.DB,
	scenarioSvc *service.ScenarioService,
	webhookSvc *service.WebhookService,
	txSvc *service.TransactionService,
	logSvc *service.LogService,
	authSvc *service.AuthService,
	customerSvc *service.CustomerService,
	otpRepo *repository.OTPRepository,
) *ControlHandler {
	return &ControlHandler{
		db: db, scenarioSvc: scenarioSvc, webhookSvc: webhookSvc,
		txSvc: txSvc, logSvc: logSvc, authSvc: authSvc,
		customerSvc: customerSvc, otpRepo: otpRepo,
	}
}

func (h *ControlHandler) paginationParams(c *gin.Context) (page, perPage int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ = strconv.Atoi(c.DefaultQuery("perPage", "50"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	return
}

func (h *ControlHandler) merchantIDFromJWT(c *gin.Context) (string, bool) {
	return middleware.GetMerchantIDFromJWT(c, h.authSvc)
}

// ListTransactions handles GET /api/v1/control/transactions
func (h *ControlHandler) ListTransactions(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	page, perPage := h.paginationParams(c)
	status := domain.TransactionStatus(c.Query("status"))
	search := c.Query("search")

	transactions, total, err := h.txSvc.ListWithSearch(merchantID, status, search, page, perPage)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	var data []gin.H
	for i := range transactions {
		data = append(data, buildTransactionData(&transactions[i], nil))
	}
	if data == nil {
		data = []gin.H{}
	}

	response.SuccessList(c, "Transactions retrieved",
		response.TransactionListFetched,
		data,
		response.BuildPaystackMeta(total, page, perPage))
}

// ListCustomers handles GET /api/v1/control/customers
func (h *ControlHandler) ListCustomers(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	page, perPage := h.paginationParams(c)

	customers, total, err := h.customerSvc.List(merchantID, page, perPage)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	var data []gin.H
	for i := range customers {
		data = append(data, buildCustomerData(&customers[i]))
	}
	if data == nil {
		data = []gin.H{}
	}

	response.SuccessList(c, "Customers retrieved",
		response.CustomerListFetched,
		data,
		response.BuildPaystackMeta(total, page, perPage))
}

// GetScenario handles GET /api/v1/control/scenario
func (h *ControlHandler) GetScenario(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	scenario, err := h.scenarioSvc.Get(merchantID)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Scenario config retrieved",
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
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
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
			response.UnprocessableErr(c, "Invalid scenario configuration",
				response.ScenarioInvalidConfig,
				fields(
					"failure_rate", "range", "Must be between 0.0 and 1.0",
					"delay_ms", "range", "Must be between 0 and 30000",
				))
		case errors.Is(err, service.ErrInvalidForceStatus):
			response.UnprocessableErr(c, "Invalid force status",
				response.TransactionForceInvalidStatus,
				field("force_status", "oneof", "Valid values: success, failed, abandoned"))
		default:
			response.InternalErr(c, "An error occurred, please try again later")
		}
		return
	}

	response.Success(c, http.StatusOK, "Scenario config updated",
		response.ScenarioUpdated, scenario)
}

// ResetScenario handles POST /api/v1/control/scenario/reset
func (h *ControlHandler) ResetScenario(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	scenario, err := h.scenarioSvc.Reset(merchantID)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Scenario config reset to defaults",
		response.ScenarioReset, scenario)
}

// ListWebhooks handles GET /api/v1/control/webhooks
func (h *ControlHandler) ListWebhooks(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	page, perPage := h.paginationParams(c)

	webhooks, total, err := h.webhookSvc.List(merchantID, page, perPage)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.SuccessList(c, "Webhooks retrieved",
		response.WebhookListFetched,
		webhooks,
		response.BuildPaystackMeta(total, page, perPage))
}

// RetryWebhook handles POST /api/v1/control/webhooks/:id/retry
func (h *ControlHandler) RetryWebhook(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	if err := h.webhookSvc.Retry(c.Param("id"), merchantID); err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			response.NotFoundErr(c, "Webhook event not found")
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Webhook retry triggered",
		response.WebhookRetried, nil)
}

// GetWebhookAttempts handles GET /api/v1/control/webhooks/:id/attempts
func (h *ControlHandler) GetWebhookAttempts(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	attempts, err := h.webhookSvc.GetAttempts(c.Param("id"), merchantID)
	if err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			response.NotFoundErr(c, "Webhook event not found")
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Webhook attempts retrieved",
		response.WebhookAttemptsFetched, gin.H{"data": attempts})
}

type forceTransactionRequest struct {
	Status    string `json:"status" binding:"required"`
	ErrorCode string `json:"error_code"`
}

// ForceTransaction handles POST /api/v1/control/transactions/:ref/force
func (h *ControlHandler) ForceTransaction(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	var req forceTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	tx, err := h.txSvc.ForceOutcome(c.Param("ref"), merchantID, req.Status, req.ErrorCode)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTransactionNotFound):
			response.NotFoundErr(c, "Transaction not found")
		case errors.Is(err, service.ErrInvalidForceStatus):
			response.UnprocessableErr(c, "Invalid force status",
				response.TransactionForceInvalidStatus,
				field("status", "oneof", "Valid values: success, failed, abandoned"))
		case errors.Is(err, service.ErrTransactionNotPending):
			response.ConflictErr(c, "Only pending transactions can be forced",
				response.TransactionAlreadyVerified)
		default:
			response.InternalErr(c, "An error occurred, please try again later")
		}
		return
	}

	response.Success(c, http.StatusOK, "Transaction outcome updated",
		response.TransactionForced, buildTransactionData(tx, nil))
}

// GetLogs handles GET /api/v1/control/logs
func (h *ControlHandler) GetLogs(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	page, perPage := h.paginationParams(c)

	logs, total, err := h.logSvc.List(merchantID, page, perPage)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	if total == 0 {
		response.Error(c, http.StatusNotFound, "No logs found",
			response.LogsEmpty, nil)
		return
	}

	response.SuccessList(c, "Logs retrieved",
		response.LogsFetched,
		logs,
		response.BuildPaystackMeta(total, page, perPage))
}

// ClearLogs handles DELETE /api/v1/control/logs
func (h *ControlHandler) ClearLogs(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	if err := h.logSvc.Clear(merchantID); err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Logs cleared", response.LogsCleared, nil)
}

// GetOTPLogs handles GET /api/v1/control/otp-logs
func (h *ControlHandler) GetOTPLogs(c *gin.Context) {
	merchantID, ok := h.merchantIDFromJWT(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	reference := c.Query("reference")

	if reference != "" {
		logs, err := h.otpRepo.FindByReference(reference, merchantID)
		if err != nil {
			response.InternalErr(c, "An error occurred, please try again later")
			return
		}
		response.Success(c, http.StatusOK, "OTP logs retrieved",
			response.OTPLogsFetched, gin.H{"data": logs})
		return
	}

	page, perPage := h.paginationParams(c)

	logs, total, err := h.otpRepo.ListByMerchant(merchantID, (page-1)*perPage, perPage)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "OTP logs retrieved",
		response.OTPLogsFetched, gin.H{
			"data": logs,
			"meta": response.BuildPaystackMeta(total, page, perPage),
		})
}
