package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/middleware"
	"github.com/payfake/payfake-api/internal/response"
	"github.com/payfake/payfake-api/internal/service"
	"gorm.io/gorm"
)

type ChargeHandler struct {
	db        *gorm.DB
	chargeSvc *service.ChargeService
}

func NewChargeHandler(db *gorm.DB, chargeSvc *service.ChargeService) *ChargeHandler {
	return &ChargeHandler{db: db, chargeSvc: chargeSvc}
}

// Request shapes

// chargeRequest mirrors Paystack's POST /charge body exactly.
// Channel is determined by which sub-object is present in the body:
//
//	card{}          → card flow (local: send_pin, international: open_url)
//	mobile_money{}  → MoMo flow (send_otp → pay_offline → webhook)
//	bank{}          → bank flow (send_birthday → send_otp → success/failed)
//
// This single endpoint replaces the old /charge/card, /charge/mobile_money,
// /charge/bank split, matching https://paystack.com/docs/api/charge/ exactly.
type chargeRequest struct {
	Email       string       `json:"email" binding:"required,email"`
	Amount      int64        `json:"amount"`
	Currency    string       `json:"currency"`
	Reference   string       `json:"reference"`
	Card        *cardDetails `json:"card"`
	MobileMoney *momoDetails `json:"mobile_money"`
	Bank        *bankDetails `json:"bank"`
	Birthday    string       `json:"birthday"`
	Metadata    domain.JSON  `json:"metadata"`
}

type cardDetails struct {
	Number      string `json:"number"`
	CVV         string `json:"cvv"`
	ExpiryMonth string `json:"expiry_month"`
	ExpiryYear  string `json:"expiry_year"`
}

type momoDetails struct {
	Phone    string `json:"phone"`
	Provider string `json:"provider"`
}

type bankDetails struct {
	Code          string `json:"code"`
	AccountNumber string `json:"account_number"`
}

type submitPINRequest struct {
	Reference string `json:"reference" binding:"required"`
	PIN       string `json:"pin" binding:"required"`
}

type submitOTPRequest struct {
	Reference string `json:"reference" binding:"required"`
	OTP       string `json:"otp" binding:"required"`
}

type submitBirthdayRequest struct {
	Reference string `json:"reference" binding:"required"`
	Birthday  string `json:"birthday" binding:"required"`
}

type submitAddressRequest struct {
	Reference string `json:"reference" binding:"required"`
	Address   string `json:"address" binding:"required"`
	City      string `json:"city" binding:"required"`
	State     string `json:"state" binding:"required"`
	ZipCode   string `json:"zip_code" binding:"required"`
	Country   string `json:"country" binding:"required"`
}

type resendOTPRequest struct {
	Reference string `json:"reference" binding:"required"`
}

// Helpers

// flowMessage returns the human-readable message for each flow step.
// Matches Paystack's message values for charge step responses.
func flowMessage(status domain.ChargeFlowStatus) string {
	switch status {
	case domain.FlowSendPIN:
		return "Please enter your PIN"
	case domain.FlowSendOTP:
		return "Please enter OTP sent to your phone"
	case domain.FlowSendBirthday:
		return "Please enter your date of birth"
	case domain.FlowSendAddress:
		return "Please enter your billing address"
	case domain.FlowOpenURL:
		return "Please complete authentication on the provided url"
	case domain.FlowPayOffline:
		return "Please complete the transaction on your mobile device"
	case domain.FlowSuccess:
		return "Charge successful"
	default:
		return "Charge attempt failed"
	}
}

// flowResponseCode maps a ChargeFlowStatus to a Payfake response code.
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

// expiryFromCard builds an expiry string from month and year.
// Paystack sends expiry_month and expiry_year separately, we combine them.
func expiryFromCard(month, year string) string {
	if len(year) == 4 {
		year = year[2:]
	}
	return month + "/" + year
}

// handleChargeError maps service errors to responses.
// Centralised here so every charge endpoint uses the same error messages
// matching what real Paystack returns.
func (h *ChargeHandler) handleChargeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTransactionNotFound):
		response.Error(c, http.StatusNotFound,
			"Transaction reference not found",
			response.TransactionNotFound, nil)
	case errors.Is(err, service.ErrTransactionNotPending):
		response.Error(c, http.StatusConflict,
			"Transaction has already been completed or reversed",
			response.TransactionAlreadyVerified, nil)
	case errors.Is(err, service.ErrChargeFlowInvalidStep):
		response.Error(c, http.StatusConflict,
			"Invalid action for current charge state",
			response.ChargeFlowInvalidStep, nil)
	case errors.Is(err, service.ErrChargeNotFound):
		response.NotFoundErr(c, "Charge not found")
	case errors.Is(err, service.ErrInvalidOTP):
		response.UnprocessableErr(c,
			"Invalid OTP",
			response.ChargeInvalidOTP,
			field("otp", "invalid", "OTP is incorrect or has expired"))
	case errors.Is(err, service.ErrOTPExpired):
		response.UnprocessableErr(c,
			"OTP has expired",
			response.ChargeInvalidOTP,
			field("otp", "expired", "OTP has expired, please request a new one"))
	default:
		response.InternalErr(c, "An error occurred, please try again later")
	}
}

// Handlers

// Charge handles POST /charge, unified endpoint matching Paystack exactly.
// Channel is detected from which sub-object is present (card/mobile_money/bank).
func (h *ChargeHandler) Charge(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	var req chargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	switch {
	case req.Card != nil:
		out, err := h.chargeSvc.ChargeCard(service.ChargeCardInput{
			MerchantID: merchant.ID,
			CardNumber: req.Card.Number,
			CardExpiry: expiryFromCard(req.Card.ExpiryMonth, req.Card.ExpiryYear),
			CardCVV:    req.Card.CVV,
			Email:      req.Email,
			Amount:     req.Amount,
			Reference:  req.Reference,
		})
		if err != nil {
			h.handleChargeError(c, err)
			return
		}
		_ = out.OTPCode
		response.Success(c, http.StatusOK, flowMessage(out.Status),
			flowResponseCode(out.Status), buildChargeFlowData(out))

	case req.MobileMoney != nil:
		if req.MobileMoney.Phone == "" || req.MobileMoney.Provider == "" {
			response.ValidationErr(c, fields(
				"mobile_money.phone", "required", "Phone is required",
				"mobile_money.provider", "required", "Provider is required",
			))
			return
		}
		out, err := h.chargeSvc.ChargeMobileMoney(service.ChargeMomoInput{
			MerchantID: merchant.ID,
			Phone:      req.MobileMoney.Phone,
			Provider:   domain.MomoProvider(req.MobileMoney.Provider),
			Email:      req.Email,
			Reference:  req.Reference,
		})
		if err != nil {
			h.handleChargeError(c, err)
			return
		}
		_ = out.OTPCode
		response.Success(c, http.StatusOK, flowMessage(out.Status),
			flowResponseCode(out.Status), buildChargeFlowData(out))

	case req.Bank != nil:
		if req.Bank.Code == "" || req.Bank.AccountNumber == "" {
			response.ValidationErr(c, fields(
				"bank.code", "required", "Bank code is required",
				"bank.account_number", "required", "Account number is required",
			))
			return
		}
		out, err := h.chargeSvc.ChargeBank(service.ChargeBankInput{
			MerchantID:    merchant.ID,
			BankCode:      req.Bank.Code,
			AccountNumber: req.Bank.AccountNumber,
			Email:         req.Email,
			Reference:     req.Reference,
		})
		if err != nil {
			h.handleChargeError(c, err)
			return
		}
		response.Success(c, http.StatusOK, flowMessage(out.Status),
			flowResponseCode(out.Status), buildChargeFlowData(out))

	default:
		response.ValidationErr(c, field("channel", "required",
			"Pass one of: card, mobile_money, or bank object"))
	}
}

// SubmitPIN handles POST /charge/submit_pin
func (h *ChargeHandler) SubmitPIN(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
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

	_ = out.OTPCode
	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildChargeFlowData(out))
}

// SubmitOTP handles POST /charge/submit_otp
func (h *ChargeHandler) SubmitOTP(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
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
		h.handleChargeError(c, err)
		return
	}

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildChargeFlowData(out))
}

// SubmitBirthday handles POST /charge/submit_birthday
func (h *ChargeHandler) SubmitBirthday(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
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
		flowResponseCode(out.Status), buildChargeFlowData(out))
}

// SubmitAddress handles POST /charge/submit_address
func (h *ChargeHandler) SubmitAddress(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
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
		flowResponseCode(out.Status), buildChargeFlowData(out))
}

// ResendOTP handles POST /charge/resend_otp
func (h *ChargeHandler) ResendOTP(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	var req resendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	out, err := h.chargeSvc.ResendOTP(service.ResendOTPInput{
		MerchantID: merchant.ID,
		Reference:  req.Reference,
	})
	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	_ = out.OTPCode
	response.Success(c, http.StatusOK, "OTP has been resent",
		response.ChargeSendOTP, buildChargeFlowData(out))
}

// FetchCharge handles GET /charge/:reference
func (h *ChargeHandler) FetchCharge(c *gin.Context) {
	merchant, ok := middleware.GetMerchant(c)
	if !ok {
		response.UnauthorizedErr(c, "Invalid key. Please ensure you are using the correct key.")
		return
	}

	charge, err := h.chargeSvc.FetchCharge(c.Param("reference"), merchant.ID)
	if err != nil {
		if errors.Is(err, service.ErrChargeNotFound) {
			response.NotFoundErr(c, "Charge not found")
			return
		}
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Charge retrieved",
		response.ChargeInitiated, gin.H{
			"status":        string(charge.Status),
			"flow_status":   string(charge.FlowStatus),
			"channel":       string(charge.Channel),
			"reference":     charge.TransactionID,
			"card_brand":    charge.CardBrand,
			"card_last4":    charge.CardLast4,
			"momo_phone":    charge.MomoPhone,
			"momo_provider": string(charge.MomoProvider),
			"error_code":    charge.ChargeErrorCode,
		})
}

// Public checkout handlers
// Same flow as above but authenticated via access_code in the body instead
// of a secret key header. The checkout page calls these directly from the browser.

// publicChargeRequest adds access_code to the standard charge request.
type publicChargeRequest struct {
	AccessCode  string       `json:"access_code" binding:"required"`
	Email       string       `json:"email" binding:"required,email"`
	Card        *cardDetails `json:"card"`
	MobileMoney *momoDetails `json:"mobile_money"`
	Bank        *bankDetails `json:"bank"`
	Birthday    string       `json:"birthday"`
}

// PublicCharge handles POST /api/v1/public/charge
func (h *ChargeHandler) PublicCharge(c *gin.Context) {
	var req publicChargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.chargeSvc.GetMerchantByAccessCode(req.AccessCode)
	if err != nil {
		response.NotFoundErr(c, "Invalid or expired access code")
		return
	}

	switch {
	case req.Card != nil:
		out, err := h.chargeSvc.ChargeCard(service.ChargeCardInput{
			MerchantID: merchant.ID,
			AccessCode: req.AccessCode,
			CardNumber: req.Card.Number,
			CardExpiry: expiryFromCard(req.Card.ExpiryMonth, req.Card.ExpiryYear),
			CardCVV:    req.Card.CVV,
			Email:      req.Email,
		})
		if err != nil {
			h.handleChargeError(c, err)
			return
		}
		response.Success(c, http.StatusOK, flowMessage(out.Status),
			flowResponseCode(out.Status), buildChargeFlowData(out))

	case req.MobileMoney != nil:
		out, err := h.chargeSvc.ChargeMobileMoney(service.ChargeMomoInput{
			MerchantID: merchant.ID,
			AccessCode: req.AccessCode,
			Phone:      req.MobileMoney.Phone,
			Provider:   domain.MomoProvider(req.MobileMoney.Provider),
			Email:      req.Email,
		})
		if err != nil {
			h.handleChargeError(c, err)
			return
		}
		response.Success(c, http.StatusOK, flowMessage(out.Status),
			flowResponseCode(out.Status), buildChargeFlowData(out))

	case req.Bank != nil:
		out, err := h.chargeSvc.ChargeBank(service.ChargeBankInput{
			MerchantID:    merchant.ID,
			AccessCode:    req.AccessCode,
			BankCode:      req.Bank.Code,
			AccountNumber: req.Bank.AccountNumber,
			Email:         req.Email,
		})
		if err != nil {
			h.handleChargeError(c, err)
			return
		}
		response.Success(c, http.StatusOK, flowMessage(out.Status),
			flowResponseCode(out.Status), buildChargeFlowData(out))

	default:
		response.ValidationErr(c, field("channel", "required",
			"Pass one of: card, mobile_money, or bank object"))
	}
}

// publicSubmitRequest is shared by all public submit endpoints.
type publicSubmitRequest struct {
	Reference string `json:"reference" binding:"required"`
}

func (h *ChargeHandler) resolvePublicMerchant(reference string) (*domain.Merchant, error) {
	return h.chargeSvc.GetMerchantByReference(reference)
}

// PublicSubmitPIN handles POST /api/v1/public/charge/submit_pin
func (h *ChargeHandler) PublicSubmitPIN(c *gin.Context) {
	var req submitPINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.resolvePublicMerchant(req.Reference)
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
		flowResponseCode(out.Status), buildChargeFlowData(out))
}

// PublicSubmitOTP handles POST /api/v1/public/charge/submit_otp
func (h *ChargeHandler) PublicSubmitOTP(c *gin.Context) {
	var req submitOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.resolvePublicMerchant(req.Reference)
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
		h.handleChargeError(c, err)
		return
	}

	response.Success(c, http.StatusOK, flowMessage(out.Status),
		flowResponseCode(out.Status), buildChargeFlowData(out))
}

// PublicSubmitBirthday handles POST /api/v1/public/charge/submit_birthday
func (h *ChargeHandler) PublicSubmitBirthday(c *gin.Context) {
	var req submitBirthdayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.resolvePublicMerchant(req.Reference)
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
		flowResponseCode(out.Status), buildChargeFlowData(out))
}

// PublicSubmitAddress handles POST /api/v1/public/charge/submit_address
func (h *ChargeHandler) PublicSubmitAddress(c *gin.Context) {
	var req submitAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.resolvePublicMerchant(req.Reference)
	if err != nil {
		response.NotFoundErr(c, "Transaction not found")
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
		flowResponseCode(out.Status), buildChargeFlowData(out))
}

// PublicResendOTP handles POST /api/v1/public/charge/resend_otp
func (h *ChargeHandler) PublicResendOTP(c *gin.Context) {
	var req resendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationErr(c, parseBindingErrors(err))
		return
	}

	merchant, err := h.resolvePublicMerchant(req.Reference)
	if err != nil {
		response.NotFoundErr(c, "Transaction not found")
		return
	}

	out, err := h.chargeSvc.ResendOTP(service.ResendOTPInput{
		MerchantID: merchant.ID,
		Reference:  req.Reference,
	})
	if err != nil {
		h.handleChargeError(c, err)
		return
	}

	_ = out.OTPCode
	response.Success(c, http.StatusOK, "OTP has been resent",
		response.ChargeSendOTP, buildChargeFlowData(out))
}

// Simulate3DS handles POST /api/v1/public/simulate/3ds/:reference
// Called by the React checkout app after the customer confirms on the 3DS page.
func (h *ChargeHandler) Simulate3DS(c *gin.Context) {
	reference := c.Param("reference")
	if reference == "" {
		response.BadRequestErr(c, "Reference is required")
		return
	}

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
		flowResponseCode(out.Status), buildChargeFlowData(out))
}
