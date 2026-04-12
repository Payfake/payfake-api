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

type TransactionHandler struct {
	db    *gorm.DB
	txSvc *service.TransactionService
}

func NewTransactionHandler(db *gorm.DB, txSvc *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{db: db, txSvc: txSvc}
}

type initializeRequest struct {
	Email       string                      `json:"email" binding:"required,email"`
	Amount      int64                       `json:"amount" binding:"required,min=1"`
	Currency    string                      `json:"currency"`
	Reference   string                      `json:"reference"`
	CallbackURL string                      `json:"callback_url"`
	Channels    []domain.TransactionChannel `json:"channels"`
	Metadata    domain.JSON                 `json:"metadata"`
}

// Initialize handles POST /api/v1/transaction/initialize
func (h *TransactionHandler) Initialize(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	var req initializeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	// Default currency to GHS if not provided,
	// most integrations in Ghana won't bother specifying it.
	currency := domain.CurrencyGHS
	if req.Currency != "" {
		currency = domain.Currency(req.Currency)
	}

	out, err := h.txSvc.Initialize(service.InitializeInput{
		MerchantID:  merchant.ID,
		Email:       req.Email,
		Amount:      req.Amount,
		Currency:    currency,
		Reference:   req.Reference,
		CallbackURL: req.CallbackURL,
		Channels:    req.Channels,
		Metadata:    req.Metadata,
	})

	if err != nil {
		switch {
		case errors.Is(err, service.ErrReferenceTaken):
			response.Error(c, http.StatusConflict, "Transaction reference already exists",
				response.TransactionReferenceTaken, []response.ErrorField{
					{Field: "reference", Message: "This reference has already been used"},
				})
		case errors.Is(err, service.ErrInvalidAmount):
			response.Error(c, http.StatusUnprocessableEntity, "Invalid amount",
				response.TransactionInvalidAmount, []response.ErrorField{
					{Field: "amount", Message: "Amount must be greater than zero"},
				})
		case errors.Is(err, service.ErrInvalidCurrency):
			response.Error(c, http.StatusUnprocessableEntity, "Unsupported currency",
				response.TransactionInvalidCurrency, []response.ErrorField{
					{Field: "currency", Message: "Supported currencies: GHS, NGN, KES, USD"},
				})
		default:
			response.InternalErr(c, "Failed to initialize transaction")
		}
		return
	}

	response.Success(c, http.StatusOK, "Authorization URL created",
		response.TransactionInitialized, gin.H{
			"authorization_url": out.AuthorizationURL,
			"access_code":       out.AccessCode,
			"reference":         out.Reference,
		})
}

// Verify handles GET /api/v1/transaction/verify/:reference
func (h *TransactionHandler) Verify(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	reference := c.Param("reference")
	if reference == "" {
		response.BadRequestErr(c, "Reference is required")
		return
	}

	tx, err := h.txSvc.Verify(reference, merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrTransactionNotFound) {
			response.NotFoundErr(c, "Transaction not found")
			return
		}
		response.InternalErr(c, "Failed to verify transaction")
		return
	}

	response.Success(c, http.StatusOK, "Transaction verified",
		response.TransactionVerified, tx)
}

// Fetch handles GET /api/v1/transaction/:id
func (h *TransactionHandler) Fetch(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	id := c.Param("id")

	tx, err := h.txSvc.Get(id, merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrTransactionNotFound) {
			response.NotFoundErr(c, "Transaction not found")
			return
		}
		response.InternalErr(c, "Failed to fetch transaction")
		return
	}

	response.Success(c, http.StatusOK, "Transaction fetched",
		response.TransactionFetched, tx)
}

// List handles GET /api/v1/transaction
func (h *TransactionHandler) List(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	// Parse pagination query params with sensible defaults.
	// page=1, perPage=50 means first page of 50 results.
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	status := domain.TransactionStatus(c.Query("status"))

	// Clamp page and perPage to sensible bounds.
	// Never let a client request 10,000 records in one call.
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	transactions, total, err := h.txSvc.List(merchant.ID, status, page, perPage)
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

// Refund handles POST /api/v1/transaction/:id/refund
func (h *TransactionHandler) Refund(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	id := c.Param("id")

	tx, err := h.txSvc.Refund(id, merchant.ID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTransactionNotFound):
			response.NotFoundErr(c, "Transaction not found")
		case errors.Is(err, service.ErrTransactionAlreadyRefunded):
			response.Error(c, http.StatusConflict, "Transaction already refunded",
				response.TransactionAlreadyRefunded, []response.ErrorField{})
		case errors.Is(err, service.ErrTransactionAlreadyVerified):
			response.Error(c, http.StatusConflict, "Only successful transactions can be refunded",
				response.TransactionAlreadyVerified, []response.ErrorField{})
		default:
			response.InternalErr(c, "Failed to refund transaction")
		}
		return
	}

	response.Success(c, http.StatusOK, "Transaction refunded successfully",
		response.TransactionRefunded, tx)
}

// / PublicFetchByAccessCode handles GET /api/v1/public/transaction/:access_code
// Called by the React checkout page on mount.
// Returns everything the checkout UI needs to render, amount, currency,
// merchant branding (name), and the customer email pre-filled on the form.
// Mirrors Paystack's popup which shows the merchant name and customer email.
func (h *TransactionHandler) PublicFetchByAccessCode(c *gin.Context) {
	accessCode := c.Param("access_code")
	if accessCode == "" {
		response.BadRequestErr(c, "Access code is required")
		return
	}

	tx, err := h.txSvc.GetByAccessCode(accessCode)
	if err != nil {
		// Only a genuine not-found (wrong access code) gets this message.
		response.NotFoundErr(c, "Invalid payment link")
		return
	}

	// Build the base data payload, same fields regardless of status.
	data := gin.H{
		"amount":       tx.Amount,
		"currency":     tx.Currency,
		"status":       tx.Status,
		"reference":    tx.Reference,
		"callback_url": tx.CallbackURL,
		"access_code":  tx.AccessCode,
		"merchant": gin.H{
			"business_name": tx.Merchant.BusinessName,
			"public_key":    tx.Merchant.PublicKey,
		},
		"customer": gin.H{
			"email":      tx.Customer.Email,
			"first_name": tx.Customer.FirstName,
			"last_name":  tx.Customer.LastName,
		},
	}

	// Return a meaningful message based on the current transaction status.
	// The React checkout app uses this to decide what screen to show —
	// payment form, success screen, failure screen, or already-paid screen.
	switch tx.Status {
	case domain.TransactionSuccess:
		// Payment already completed — don't show the payment form again.
		// Return 200 so the checkout app can render a "already paid" screen
		// instead of an error page. The data is still included so the app
		// can show the amount and merchant name in the confirmation.
		response.Success(c, http.StatusOK,
			"Payment already completed", response.TransactionVerified, data)

	case domain.TransactionFailed:
		// Previous charge attempt failed, the customer can try again
		// by initializing a new transaction. We return 200 here too
		// so the checkout app can show a proper "payment failed, please
		// try again" screen rather than a generic error.
		response.Success(c, http.StatusOK,
			"Payment was not successful", response.TransactionVerified, data)

	case domain.TransactionAbandoned:
		response.Success(c, http.StatusOK,
			"This payment link has expired", response.TransactionVerified, data)

	case domain.TransactionReversed:
		response.Success(c, http.StatusOK,
			"This payment has been refunded", response.TransactionVerified, data)

	default:
		// Pending, normal flow, show the payment form.
		response.Success(c, http.StatusOK,
			"Transaction fetched", response.TransactionFetched, data)
	}
}
