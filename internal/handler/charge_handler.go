package handler

import (
	"errors"
	"net/http"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/middleware"
	"github.com/GordenArcher/payfake/internal/response"
	"github.com/GordenArcher/payfake/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChargeHandler struct {
	db        *gorm.DB
	chargeSvc *service.ChargeService
}

func NewChargeHandler(db *gorm.DB, chargeSvc *service.ChargeService) *ChargeHandler {
	return &ChargeHandler{db: db, chargeSvc: chargeSvc}
}

type chargeCardRequest struct {
	AccessCode string `json:"access_code"`
	Reference  string `json:"reference"`
	CardNumber string `json:"card_number" binding:"required"`
	CardExpiry string `json:"card_expiry" binding:"required"`
	CardCVV    string `json:"cvv" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
}

// ChargeCard handles POST /api/v1/charge/card
func (h *ChargeHandler) ChargeCard(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	var req chargeCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	// Either access_code (popup flow) or reference (direct API flow)
	// must be provided, both being empty is a bad request.
	if req.AccessCode == "" && req.Reference == "" {
		response.ValidationErr(c, []response.ErrorField{
			{Field: "access_code", Message: "Either access_code or reference is required"},
		})
		return
	}

	out, err := h.chargeSvc.ChargeCard(service.ChargeCardInput{
		MerchantID: merchant.ID,
		AccessCode: req.AccessCode,
		Reference:  req.Reference,
		CardNumber: req.CardNumber,
		CardExpiry: req.CardExpiry,
		CardCVV:    req.CardCVV,
		Email:      req.Email,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	// The response code and HTTP status vary based on the simulation outcome.
	// Success → 200 + CHARGE_SUCCESSFUL
	// Failed  → 400 + CHARGE_FAILED (with specific error code in the data)
	if out.Status == domain.TransactionSuccess {
		response.Success(c, http.StatusOK, "Charge successful",
			response.ChargeSuccessful, gin.H{
				"transaction": out.Transaction,
				"charge":      out.Charge,
			})
	} else {
		response.Error(c, http.StatusBadRequest, "Charge failed",
			response.ChargeFailed, []response.ErrorField{
				{Field: "charge", Message: out.ErrorCode},
			})
	}
}

type chargeMomoRequest struct {
	AccessCode string `json:"access_code"`
	Reference  string `json:"reference"`
	Phone      string `json:"phone" binding:"required"`
	Provider   string `json:"provider" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
}

// ChargeMobileMoney handles POST /api/v1/charge/mobile_money
func (h *ChargeHandler) ChargeMobileMoney(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	var req chargeMomoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	if req.AccessCode == "" && req.Reference == "" {
		response.ValidationErr(c, []response.ErrorField{
			{Field: "access_code", Message: "Either access_code or reference is required"},
		})
		return
	}

	// Validate the MoMo provider is one we support.
	validProviders := map[string]bool{
		string(domain.ProviderMTN):        true,
		string(domain.ProviderVodafone):   true,
		string(domain.ProviderAirtelTigo): true,
	}
	if !validProviders[req.Provider] {
		response.ValidationErr(c, []response.ErrorField{
			{Field: "provider", Message: "Supported providers: mtn, vodafone, airteltigo"},
		})
		return
	}

	out, err := h.chargeSvc.ChargeMobileMoney(service.ChargeMomoInput{
		MerchantID: merchant.ID,
		AccessCode: req.AccessCode,
		Reference:  req.Reference,
		Phone:      req.Phone,
		Provider:   domain.MomoProvider(req.Provider),
		Email:      req.Email,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	// MoMo always returns 200 + CHARGE_PENDING immediately.
	// The actual outcome arrives via webhook after the async resolution.
	// Developers must implement webhook handling to know the final status,
	// they cannot poll the charge endpoint and expect it to change synchronously.
	response.Success(c, http.StatusOK, "Mobile money prompt sent",
		response.ChargePending, gin.H{
			"transaction": out.Transaction,
			"charge":      out.Charge,
		})
}

type chargeBankRequest struct {
	AccessCode    string `json:"access_code"`
	Reference     string `json:"reference"`
	BankCode      string `json:"bank_code" binding:"required"`
	AccountNumber string `json:"account_number" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
}

// ChargeBank handles POST /api/v1/charge/bank
func (h *ChargeHandler) ChargeBank(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	var req chargeBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	if req.AccessCode == "" && req.Reference == "" {
		response.ValidationErr(c, []response.ErrorField{
			{Field: "access_code", Message: "Either access_code or reference is required"},
		})
		return
	}

	out, err := h.chargeSvc.ChargeBank(service.ChargeBankInput{
		MerchantID:    merchant.ID,
		AccessCode:    req.AccessCode,
		Reference:     req.Reference,
		BankCode:      req.BankCode,
		AccountNumber: req.AccountNumber,
		Email:         req.Email,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	if out.Status == domain.TransactionSuccess {
		response.Success(c, http.StatusOK, "Bank charge successful",
			response.ChargeSuccessful, gin.H{
				"transaction": out.Transaction,
				"charge":      out.Charge,
			})
	} else {
		response.Error(c, http.StatusBadRequest, "Bank charge failed",
			response.ChargeFailed, []response.ErrorField{
				{Field: "charge", Message: out.ErrorCode},
			})
	}
}

// FetchCharge handles GET /api/v1/charge/:reference
func (h *ChargeHandler) FetchCharge(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	reference := c.Param("reference")

	charge, err := h.chargeSvc.FetchCharge(reference, merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrChargeNotFound) {
			response.NotFoundErr(c, "Charge not found")
			return
		}
		response.InternalErr(c, "Failed to fetch charge")
		return
	}

	response.Success(c, http.StatusOK, "Charge fetched",
		response.ChargeInitiated, charge)
}

// handleChargeError maps charge-specific sentinel errors to responses.
// Extracted into its own method because all three charge endpoints
// share the same error mapping, DRY over the common cases.
func (h *ChargeHandler) handleChargeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTransactionNotFound):
		response.NotFoundErr(c, "Transaction not found")
	case errors.Is(err, service.ErrTransactionNotPending):
		response.Error(c, http.StatusConflict,
			"Transaction is not in a chargeable state",
			response.TransactionAlreadyVerified, []response.ErrorField{})
	default:
		response.InternalErr(c, "Charge failed")
	}
}
