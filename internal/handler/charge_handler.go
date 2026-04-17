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

// buildFlowResponse builds the response data from a ChargeFlowResponse.
// OTPCode is logged but never returned to the client.
// Developers read it from /control/logs during testing.
func buildFlowResponse(out *service.ChargeFlowResponse) gin.H {
	return gin.H{
		"status":       out.Status,
		"reference":    out.Reference,
		"display_text": out.DisplayText,
		"charge":       out.Charge,
		"transaction":  out.Transaction,
		"three_ds_url": out.ThreeDSURL,
	}
}

// flowResponseCode maps a ChargeFlowStatus to the correct response code.
func flowResponseCode(status domain.ChargeFlowStatus) response.Code {
	switch status {
	case domain.FlowSendPIN:
		return response.ChargeSendPIN
	case domain.FlowSendOTP:
		return response.ChargeSendOTP
	case domain.FlowSendBirthday:
		return response.ChargeSendBirthday
	case domain.FlowSendAddress:
		return response.ChargeSendAddress
	case domain.FlowOpenURL:
		return response.ChargeOpenURL
	case domain.FlowPayOffline:
		return response.ChargePayOffline
	case domain.FlowSuccess:
		return response.ChargeSuccessful
	default:
		return response.ChargeFailed
	}
}

// flowMessage returns a human-readable message for each flow status.
func flowMessage(status domain.ChargeFlowStatus) string {
	switch status {
	case domain.FlowSendPIN:
		return "Enter your card PIN"
	case domain.FlowSendOTP:
		return "Enter the OTP sent to your phone"
	case domain.FlowSendBirthday:
		return "Enter your date of birth"
	case domain.FlowSendAddress:
		return "Enter your billing address"
	case domain.FlowOpenURL:
		return "Complete 3D Secure verification"
	case domain.FlowPayOffline:
		return "Approve the payment prompt on your phone"
	case domain.FlowSuccess:
		return "Payment successful"
	default:
		return "Payment failed"
	}
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
// Returns send_pin for local cards, open_url for international cards.
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

	// Log the OTP if generated (card flow after PIN step won't have it yet)
	// but never return it in the response.
	_ = out.OTPCode

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

type chargeMomoRequest struct {
	AccessCode string `json:"access_code"`
	Reference  string `json:"reference"`
	Phone      string `json:"phone" binding:"required"`
	Provider   string `json:"provider" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
}

// ChargeMobileMoney handles POST /api/v1/charge/mobile_money
// Returns send_otp — customer must verify phone with OTP first.
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

	// OTP is generated but never sent to client.
	// It appears in /control/logs so developer can see it during testing.
	// In production this would be delivered via SMS.
	_ = out.OTPCode

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

type chargeBankRequest struct {
	AccessCode    string `json:"access_code"`
	Reference     string `json:"reference"`
	BankCode      string `json:"bank_code" binding:"required"`
	AccountNumber string `json:"account_number" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
}

// ChargeBank handles POST /api/v1/charge/bank
// Returns send_birthday, customer must verify identity with DOB first.
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

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

type submitPINRequest struct {
	Reference string `json:"reference" binding:"required"`
	PIN       string `json:"pin" binding:"required"`
}

// SubmitPIN handles POST /api/v1/charge/submit_pin
func (h *ChargeHandler) SubmitPIN(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	var req submitPINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	out, err := h.chargeSvc.SubmitPIN(service.SubmitPINInput{
		MerchantID: merchant.ID,
		Reference:  req.Reference,
		PIN:        req.PIN,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	// Log OTP but don't return it
	_ = out.OTPCode

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

type submitOTPRequest struct {
	Reference string `json:"reference" binding:"required"`
	OTP       string `json:"otp" binding:"required"`
}

// SubmitOTP handles POST /api/v1/charge/submit_otp
// Works for both card and MoMo flows.
func (h *ChargeHandler) SubmitOTP(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	var req submitOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	out, err := h.chargeSvc.SubmitOTP(service.SubmitOTPInput{
		MerchantID: merchant.ID,
		Reference:  req.Reference,
		OTP:        req.OTP,
	})

	if err != nil {
		if errors.Is(err, service.ErrInvalidOTP) {
			response.Error(c, http.StatusUnprocessableEntity,
				"Invalid OTP — please check and try again",
				response.ChargeInvalidOTP, []response.ErrorField{
					{Field: "otp", Message: "OTP is incorrect or has expired"},
				})
			return
		}
		h.handleChargeError(c, err)
		return
	}

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

type submitBirthdayRequest struct {
	Reference string `json:"reference" binding:"required"`
	Birthday  string `json:"birthday" binding:"required"`
}

// SubmitBirthday handles POST /api/v1/charge/submit_birthday
func (h *ChargeHandler) SubmitBirthday(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	var req submitBirthdayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	out, err := h.chargeSvc.SubmitBirthday(service.SubmitBirthdayInput{
		MerchantID: merchant.ID,
		Reference:  req.Reference,
		Birthday:   req.Birthday,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	_ = out.OTPCode

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

type submitAddressRequest struct {
	Reference string `json:"reference" binding:"required"`
	Address   string `json:"address" binding:"required"`
	City      string `json:"city" binding:"required"`
	State     string `json:"state" binding:"required"`
	ZipCode   string `json:"zip_code" binding:"required"`
	Country   string `json:"country" binding:"required"`
}

// SubmitAddress handles POST /api/v1/charge/submit_address
func (h *ChargeHandler) SubmitAddress(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Unauthorized")
		return
	}

	var req submitAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	out, err := h.chargeSvc.SubmitAddress(service.SubmitAddressInput{
		MerchantID: merchant.ID,
		Reference:  req.Reference,
		Address:    req.Address,
		City:       req.City,
		State:      req.State,
		ZipCode:    req.ZipCode,
		Country:    req.Country,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

// Simulate3DS handles POST /api/v1/simulate/3ds/:reference
// The React checkout page calls this after showing a fake 3DS form.
// No auth required, public endpoint since the customer's browser calls it.
func (h *ChargeHandler) Simulate3DS(c *gin.Context) {
	reference := c.Param("reference")

	// For 3DS simulation we need the merchant, resolve via transaction.
	merchant, err := h.chargeSvc.GetMerchantByReference(reference)
	if err != nil {
		response.NotFoundErr(c, "Transaction not found")
		return
	}

	out, err := h.chargeSvc.Simulate3DS(reference, merchant.ID)
	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
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

// Public charge handlers, authenticated via access_code, no secret key.

func (h *ChargeHandler) PublicChargeCard(c *gin.Context) {
	var req chargeCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	if req.AccessCode == "" {
		response.ValidationErr(c, []response.ErrorField{
			{Field: "access_code", Message: "access_code is required"},
		})
		return
	}

	merchant, err := h.chargeSvc.GetMerchantByAccessCode(req.AccessCode)
	if err != nil {
		response.NotFoundErr(c, "Invalid or expired access code")
		return
	}

	out, err := h.chargeSvc.ChargeCard(service.ChargeCardInput{
		MerchantID: merchant.ID,
		AccessCode: req.AccessCode,
		CardNumber: req.CardNumber,
		CardExpiry: req.CardExpiry,
		CardCVV:    req.CardCVV,
		Email:      req.Email,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

func (h *ChargeHandler) PublicChargeMobileMoney(c *gin.Context) {
	var req chargeMomoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	if req.AccessCode == "" {
		response.ValidationErr(c, []response.ErrorField{
			{Field: "access_code", Message: "access_code is required"},
		})
		return
	}

	merchant, err := h.chargeSvc.GetMerchantByAccessCode(req.AccessCode)
	if err != nil {
		response.NotFoundErr(c, "Invalid or expired access code")
		return
	}

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
		Phone:      req.Phone,
		Provider:   domain.MomoProvider(req.Provider),
		Email:      req.Email,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

func (h *ChargeHandler) PublicChargeBank(c *gin.Context) {
	var req chargeBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	if req.AccessCode == "" {
		response.ValidationErr(c, []response.ErrorField{
			{Field: "access_code", Message: "access_code is required"},
		})
		return
	}

	merchant, err := h.chargeSvc.GetMerchantByAccessCode(req.AccessCode)
	if err != nil {
		response.NotFoundErr(c, "Invalid or expired access code")
		return
	}

	out, err := h.chargeSvc.ChargeBank(service.ChargeBankInput{
		MerchantID:    merchant.ID,
		AccessCode:    req.AccessCode,
		BankCode:      req.BankCode,
		AccountNumber: req.AccountNumber,
		Email:         req.Email,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

// Public submit endpoints, called from checkout page, no secret key.

func (h *ChargeHandler) PublicSubmitPIN(c *gin.Context) {
	var req submitPINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.chargeSvc.GetMerchantByReference(req.Reference)
	if err != nil {
		response.NotFoundErr(c, "Transaction not found")
		return
	}

	out, err := h.chargeSvc.SubmitPIN(service.SubmitPINInput{
		MerchantID: merchant.ID,
		Reference:  req.Reference,
		PIN:        req.PIN,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	_ = out.OTPCode
	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

func (h *ChargeHandler) PublicSubmitOTP(c *gin.Context) {
	var req submitOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.chargeSvc.GetMerchantByReference(req.Reference)
	if err != nil {
		response.NotFoundErr(c, "Transaction not found")
		return
	}

	out, err := h.chargeSvc.SubmitOTP(service.SubmitOTPInput{
		MerchantID: merchant.ID,
		Reference:  req.Reference,
		OTP:        req.OTP,
	})

	if err != nil {
		if errors.Is(err, service.ErrInvalidOTP) {
			response.Error(c, http.StatusUnprocessableEntity,
				"Invalid OTP — please check and try again",
				response.ChargeInvalidOTP, []response.ErrorField{
					{Field: "otp", Message: "OTP is incorrect"},
				})
			return
		}
		h.handleChargeError(c, err)
		return
	}

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

func (h *ChargeHandler) PublicSubmitBirthday(c *gin.Context) {
	var req submitBirthdayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.chargeSvc.GetMerchantByReference(req.Reference)
	if err != nil {
		response.NotFoundErr(c, "Transaction not found")
		return
	}

	out, err := h.chargeSvc.SubmitBirthday(service.SubmitBirthdayInput{
		MerchantID: merchant.ID,
		Reference:  req.Reference,
		Birthday:   req.Birthday,
	})

	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	_ = out.OTPCode
	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildFlowResponse(out))
}

func (h *ChargeHandler) handleChargeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTransactionNotFound):
		response.NotFoundErr(c, "Transaction not found")
	case errors.Is(err, service.ErrTransactionNotPending):
		response.Error(c, http.StatusConflict,
			"Transaction is not in a chargeable state",
			response.TransactionAlreadyVerified, []response.ErrorField{})
	case errors.Is(err, service.ErrChargeFlowInvalidStep):
		response.Error(c, http.StatusConflict,
			"Invalid step — this action is not allowed at this stage of the payment flow",
			response.ChargeFlowInvalidStep, []response.ErrorField{})
	case errors.Is(err, service.ErrChargeNotFound):
		response.NotFoundErr(c, "Charge not found")
	default:
		response.InternalErr(c, "Charge failed")
	}
}
