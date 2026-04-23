package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/middleware"
	"github.com/payfake/payfake-api/internal/response"
	"github.com/payfake/payfake-api/internal/service"
	"gorm.io/gorm"
)

type TransactionHandler struct {
	db        *gorm.DB
	txSvc     *service.TransactionService
	chargeSvc *service.ChargeService
}

func NewTransactionHandler(db *gorm.DB, txSvc *service.TransactionService, chargeSvc *service.ChargeService) *TransactionHandler {
	return &TransactionHandler{db: db, txSvc: txSvc, chargeSvc: chargeSvc}
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

// Initialize handles POST /transaction/initialize
// Response matches Paystack exactly:
// { "status": true, "message": "Authorization URL created",
//
//	"data": { "authorization_url": "...", "access_code": "...", "reference": "..." } }
func (h *TransactionHandler) Initialize(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	var req initializeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

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
			response.UnprocessableErr(c,
				"Duplicate Transaction Reference",
				response.TransactionReferenceTaken,
				field("reference", "unique", "Transaction reference already exists"))
		case errors.Is(err, service.ErrInvalidAmount):
			response.UnprocessableErr(c,
				"Invalid amount",
				response.TransactionInvalidAmount,
				field("amount", "min", "Amount must be greater than 0"))
		case errors.Is(err, service.ErrInvalidCurrency):
			response.UnprocessableErr(c,
				"Invalid currency",
				response.TransactionInvalidCurrency,
				field("currency", "oneof", "Supported currencies: GHS, NGN, KES, USD"))
		default:
			response.InternalErr(c, "An error occurred, please try again later")
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

// Verify handles GET /transaction/verify/:reference
// Response data matches Paystack's verify response exactly.
func (h *TransactionHandler) Verify(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	reference := c.Param("reference")
	tx, err := h.txSvc.Verify(reference, merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrTransactionNotFound) {
			response.Error(c, http.StatusNotFound,
				"Transaction reference not found",
				response.TransactionNotFound, nil)
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	charge, _ := h.chargeSvc.FetchChargeByTransactionID(tx.ID)
	response.Success(c, http.StatusOK, "Verification successful",
		response.TransactionVerified, buildTransactionData(tx, charge))
}

// Fetch handles GET /transaction/:id
func (h *TransactionHandler) Fetch(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	tx, err := h.txSvc.Get(c.Param("id"), merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrTransactionNotFound) {
			response.NotFoundErr(c, "Transaction not found")
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	charge, _ := h.chargeSvc.FetchChargeByTransactionID(tx.ID)
	response.Success(c, http.StatusOK, "Transaction retrieved",
		response.TransactionFetched, buildTransactionData(tx, charge))
}

// List handles GET /transaction
// Pagination uses Paystack's perPage param (not per_page).
func (h *TransactionHandler) List(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("perPage", "50"))
	status := domain.TransactionStatus(c.Query("status"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	transactions, total, err := h.txSvc.List(merchant.ID, status, page, perPage)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	var data []gin.H
	for i := range transactions {
		charge, _ := h.chargeSvc.FetchChargeByTransactionID(transactions[i].ID)
		data = append(data, buildTransactionData(&transactions[i], charge))
	}
	if data == nil {
		data = []gin.H{}
	}

	response.SuccessList(c, "Transactions retrieved",
		response.TransactionListFetched,
		data,
		response.BuildPaystackMeta(total, page, perPage))
}

// Refund handles POST /transaction/:id/refund
func (h *TransactionHandler) Refund(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	tx, err := h.txSvc.Refund(c.Param("id"), merchant.ID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTransactionNotFound):
			response.NotFoundErr(c, "Transaction not found")
		case errors.Is(err, service.ErrTransactionAlreadyRefunded):
			response.ConflictErr(c, "Transaction has already been reversed", response.TransactionAlreadyRefunded)
		case errors.Is(err, service.ErrTransactionAlreadyVerified):
			response.ConflictErr(c, "Only successful transactions can be refunded", response.TransactionAlreadyVerified)
		default:
			response.InternalErr(c, "An error occurred, please try again later")
		}
		return
	}

	charge, _ := h.chargeSvc.FetchChargeByTransactionID(tx.ID)
	response.Success(c, http.StatusOK, "Transaction has been reversed",
		response.TransactionRefunded, buildTransactionData(tx, charge))
}

// PublicFetchByAccessCode handles GET /api/v1/public/transaction/:access_code
func (h *TransactionHandler) PublicFetchByAccessCode(c *gin.Context) {
	accessCode := c.Param("access_code")
	if accessCode == "" {
		response.BadRequestErr(c, "Access code is required")
		return
	}

	tx, err := h.txSvc.GetByAccessCode(accessCode)
	if err != nil {
		response.NotFoundErr(c, "Invalid payment link")
		return
	}

	merchant, err := h.txSvc.GetMerchantForTransaction(tx.MerchantID)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	charge, _ := h.chargeSvc.FetchChargeByTransactionID(tx.ID)

	var chargeData gin.H
	if charge != nil {
		chargeData = gin.H{
			"flow_status": string(charge.FlowStatus),
			"status":      string(charge.Status),
			"error_code":  charge.ChargeErrorCode,
			"channel":     string(charge.Channel),
		}
	}

	data := gin.H{
		"amount":       tx.Amount,
		"currency":     string(tx.Currency),
		"status":       string(tx.Status),
		"reference":    tx.Reference,
		"callback_url": tx.CallbackURL,
		"access_code":  tx.AccessCode,
		"charge":       chargeData,
		"merchant": gin.H{
			"business_name": merchant.BusinessName,
			"public_key":    merchant.PublicKey,
		},
		"customer": gin.H{
			"email":      tx.Customer.Email,
			"first_name": tx.Customer.FirstName,
			"last_name":  tx.Customer.LastName,
		},
	}

	switch tx.Status {
	case domain.TransactionSuccess:
		response.Success(c, http.StatusOK, "Payment already completed", response.TransactionVerified, data)
	case domain.TransactionFailed:
		response.Success(c, http.StatusOK, "Payment was not successful", response.TransactionVerified, data)
	case domain.TransactionAbandoned:
		response.Success(c, http.StatusOK, "This payment link has expired", response.TransactionVerified, data)
	case domain.TransactionReversed:
		response.Success(c, http.StatusOK, "This payment has been refunded", response.TransactionVerified, data)
	default:
		response.Success(c, http.StatusOK, "Transaction fetched", response.TransactionFetched, data)
	}
}

// PublicVerify handles GET /api/v1/public/transaction/verify/:reference
// Used by the checkout page to poll transaction status during MoMo pay_offline.
func (h *TransactionHandler) PublicVerify(c *gin.Context) {
	reference := c.Param("reference")
	if reference == "" {
		response.BadRequestErr(c, "Reference is required")
		return
	}

	tx, err := h.txSvc.GetByReference(reference)
	if err != nil {
		response.NotFoundErr(c, "Transaction not found")
		return
	}

	charge, _ := h.chargeSvc.FetchChargeByTransactionID(tx.ID)

	var chargeData gin.H
	if charge != nil {
		chargeData = gin.H{
			"flow_status": string(charge.FlowStatus),
			"status":      string(charge.Status),
			"error_code":  charge.ChargeErrorCode,
			"channel":     string(charge.Channel),
		}
	}

	response.Success(c, http.StatusOK, "Verification successful",
		response.TransactionVerified, gin.H{
			"status":    string(tx.Status),
			"reference": tx.Reference,
			"amount":    tx.Amount,
			"currency":  string(tx.Currency),
			"paid_at":   tx.PaidAt,
			"charge":    chargeData,
		})
}
