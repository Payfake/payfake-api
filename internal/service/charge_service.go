package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/repository"
	"github.com/GordenArcher/payfake/pkg/otp"
	"github.com/GordenArcher/payfake/pkg/uid"
	"gorm.io/gorm"
)

type ChargeService struct {
	chargeRepo      *repository.ChargeRepository
	transactionRepo *repository.TransactionRepository
	merchantRepo    *repository.MerchantRepository
	otpRepo         *repository.OTPRepository
	simulatorSvc    *SimulatorService
	webhookSvc      *WebhookService
	frontendURL     string
}

func NewChargeService(
	chargeRepo *repository.ChargeRepository,
	transactionRepo *repository.TransactionRepository,
	merchantRepo *repository.MerchantRepository,
	otpRepo *repository.OTPRepository,
	simulatorSvc *SimulatorService,
	webhookSvc *WebhookService,
	frontendURL string,
) *ChargeService {
	return &ChargeService{
		chargeRepo:      chargeRepo,
		transactionRepo: transactionRepo,
		merchantRepo:    merchantRepo,
		otpRepo:         otpRepo,
		simulatorSvc:    simulatorSvc,
		webhookSvc:      webhookSvc,
		frontendURL:     frontendURL,
	}
}

// ChargeCardInput is the input for initiating a card charge.
type ChargeCardInput struct {
	MerchantID string
	AccessCode string
	Reference  string
	CardNumber string
	CardExpiry string
	CardCVV    string
	Email      string
}

// ChargeMomoInput is the input for initiating a mobile money charge.
type ChargeMomoInput struct {
	MerchantID string
	AccessCode string
	Reference  string
	Phone      string
	Provider   domain.MomoProvider
	Email      string
}

// ChargeBankInput is the input for initiating a bank transfer charge.
type ChargeBankInput struct {
	MerchantID    string
	AccessCode    string
	Reference     string
	BankCode      string
	AccountNumber string
	Email         string
}

// SubmitPINInput is the input for submitting a card PIN.
type SubmitPINInput struct {
	MerchantID string
	Reference  string
	PIN        string
}

// SubmitOTPInput is the input for submitting an OTP.
type SubmitOTPInput struct {
	MerchantID string
	Reference  string
	OTP        string
}

// SubmitBirthdayInput is the input for submitting a date of birth.
type SubmitBirthdayInput struct {
	MerchantID string
	Reference  string
	Birthday   string // format: YYYY-MM-DD
}

// SubmitAddressInput is the input for submitting a billing address.
type SubmitAddressInput struct {
	MerchantID string
	Reference  string
	Address    string
	City       string
	State      string
	ZipCode    string
	Country    string
}

// ChargeFlowResponse is returned by every charge step endpoint.
// The checkout page reads FlowStatus and renders the appropriate next step.
type ChargeFlowResponse struct {
	Status      domain.ChargeFlowStatus
	Reference   string
	DisplayText string
	// OTPCode is populated only in the service layer for logging.
	// It is NEVER sent to the client, the handler strips it.
	// Developers read it from /control/logs during testing.
	OTPCode     string
	ThreeDSURL  string
	Transaction *domain.Transaction
	Charge      *domain.Charge
}

// ChargeCard initiates a card charge.
// For local cards: returns send_pin, customer must enter PIN.
// For international cards: returns open_url, customer completes 3DS.
// We detect card type from the number, Visa/Mastercard starting
// with certain ranges are treated as international.
func (s *ChargeService) ChargeCard(input ChargeCardInput) (*ChargeFlowResponse, error) {
	tx, err := s.findPendingTransaction(input.AccessCode, input.Reference, input.MerchantID)
	if err != nil {
		return nil, err
	}

	cardType := detectCardType(input.CardNumber)

	charge := &domain.Charge{
		Base:          domain.Base{ID: uid.NewChargeID()},
		MerchantID:    input.MerchantID,
		TransactionID: tx.ID,
		Channel:       domain.ChannelCard,
		Status:        domain.TransactionPending,
		CardLast4:     safeCardLast4(input.CardNumber),
		CardBrand:     detectCardBrand(input.CardNumber),
		CardType:      cardType,
	}

	// Determine the first step based on card type.
	if cardType == domain.CardTypeInternational {
		// International cards go through 3DS verification.
		// We generate a simulated 3DS URL, the checkout page
		// opens this in an iframe or redirect, simulates the
		// customer completing verification, then the flow resolves.
		charge.FlowStatus = domain.FlowOpenURL
		charge.ThreeDSURL = fmt.Sprintf("http://localhost:3000/simulate/3ds/%s", tx.Reference)
	} else {
		// Local Ghana cards start with PIN entry.
		charge.FlowStatus = domain.FlowSendPIN
	}

	if err := s.chargeRepo.Create(charge); err != nil {
		return nil, fmt.Errorf("failed to create charge: %w", err)
	}

	// Update transaction channel now that we know it.
	s.transactionRepo.UpdateStatus(tx.ID, domain.TransactionPending, nil)

	resp := &ChargeFlowResponse{
		Status:      charge.FlowStatus,
		Reference:   tx.Reference,
		Charge:      charge,
		Transaction: tx,
	}

	if cardType == domain.CardTypeInternational {
		resp.DisplayText = "Complete 3D Secure verification to proceed"
		resp.ThreeDSURL = charge.ThreeDSURL
	} else {
		resp.DisplayText = "Please enter your card PIN"
	}

	return resp, nil
}

// ChargeMobileMoney initiates a MoMo charge.
// Returns send_otp, the customer must enter the OTP sent to their phone.
// After OTP verification the flow moves to pay_offline while waiting
// for the customer to approve the USSD prompt.
func (s *ChargeService) ChargeMobileMoney(input ChargeMomoInput) (*ChargeFlowResponse, error) {
	tx, err := s.findPendingTransaction(input.AccessCode, input.Reference, input.MerchantID)
	if err != nil {
		return nil, err
	}

	// Generate OTP for MoMo verification.
	// In real Paystack this is sent via SMS to the customer's phone.
	// In Payfake we log it to the introspection logs so the developer
	// can read it without needing a real phone.
	otpCode, err := otp.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	// Log OTP so developer can read it from /control/otp-logs during testing.
	// This never goes to the client, only stored in the DB.
	s.otpRepo.Create(tx.MerchantID, tx.Reference, string(domain.ChannelMobileMoney), "send_otp", otpCode)

	charge := &domain.Charge{
		Base:          domain.Base{ID: uid.NewChargeID()},
		MerchantID:    input.MerchantID,
		TransactionID: tx.ID,
		Channel:       domain.ChannelMobileMoney,
		Status:        domain.TransactionPending,
		FlowStatus:    domain.FlowSendOTP,
		MomoPhone:     input.Phone,
		MomoProvider:  input.Provider,
		OTPCode:       otpCode,
	}

	if err := s.chargeRepo.Create(charge); err != nil {
		return nil, fmt.Errorf("failed to create charge: %w", err)
	}

	return &ChargeFlowResponse{
		Status:      domain.FlowSendOTP,
		Reference:   tx.Reference,
		DisplayText: fmt.Sprintf("Enter the OTP sent to %s", maskPhone(input.Phone)),
		// OTPCode is included so the handler can log it.
		// The handler strips it before sending to the client.
		OTPCode:     otpCode,
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// ChargeBank initiates a bank transfer charge.
// Returns send_birthday, the customer must enter their date of birth
// as the first verification step, same as real Paystack bank charges.
func (s *ChargeService) ChargeBank(input ChargeBankInput) (*ChargeFlowResponse, error) {
	tx, err := s.findPendingTransaction(input.AccessCode, input.Reference, input.MerchantID)
	if err != nil {
		return nil, err
	}

	charge := &domain.Charge{
		Base:              domain.Base{ID: uid.NewChargeID()},
		MerchantID:        input.MerchantID,
		TransactionID:     tx.ID,
		Channel:           domain.ChannelBankTransfer,
		Status:            domain.TransactionPending,
		FlowStatus:        domain.FlowSendBirthday,
		BankCode:          input.BankCode,
		BankAccountNumber: input.AccountNumber,
	}

	if err := s.chargeRepo.Create(charge); err != nil {
		return nil, fmt.Errorf("failed to create charge: %w", err)
	}

	return &ChargeFlowResponse{
		Status:      domain.FlowSendBirthday,
		Reference:   tx.Reference,
		DisplayText: "Enter your date of birth to verify your identity",
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// SubmitPIN processes the card PIN submission.
// If the scenario is set to force failure it fails here.
// Otherwise it advances to the OTP step.
// Any 4-digit PIN is accepted unless the simulator rejects it —
// we're simulating behavior, not real PIN validation.
func (s *ChargeService) SubmitPIN(input SubmitPINInput) (*ChargeFlowResponse, error) {
	charge, err := s.chargeRepo.FindByTransactionReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, ErrChargeNotFound
	}

	// Validate we're at the right step, can't submit PIN if flow
	// is already past the PIN step or in a terminal state.
	if charge.FlowStatus != domain.FlowSendPIN {
		return nil, ErrChargeFlowInvalidStep
	}

	// Check scenario, the simulator may force a PIN failure here.
	// CHARGE_INVALID_PIN specifically means the PIN step fails.
	result := s.simulatorSvc.ResolveOutcome(input.MerchantID, domain.ChannelCard)
	// ChargeInvalidPIN is a response code string, not a domain constant.
	// We compare against the string directly.
	if result.Status == domain.TransactionFailed && result.ErrorCode == "CHARGE_INVALID_PIN" {
		return s.failCharge(charge, result.ErrorCode)
	}

	// PIN accepted, generate OTP and advance to OTP step.
	otpCode, err := otp.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}
	s.otpRepo.Create(input.MerchantID, input.Reference, string(domain.ChannelCard), "submit_pin", otpCode)

	if err := s.chargeRepo.UpdateFlowStatus(charge.ID, domain.FlowSendOTP, otpCode); err != nil {
		return nil, fmt.Errorf("failed to update flow status: %w", err)
	}

	charge.FlowStatus = domain.FlowSendOTP

	tx, _ := s.transactionRepo.FindByReference(input.Reference, input.MerchantID)

	return &ChargeFlowResponse{
		Status:      domain.FlowSendOTP,
		Reference:   input.Reference,
		DisplayText: "Enter the OTP sent to your registered phone number",
		OTPCode:     otpCode,
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// SubmitOTP processes the OTP submission for both card and MoMo flows.
// For cards: verifies OTP then resolves the final outcome.
// For MoMo: verifies OTP then moves to pay_offline (waiting for USSD approval).
func (s *ChargeService) SubmitOTP(input SubmitOTPInput) (*ChargeFlowResponse, error) {
	charge, err := s.chargeRepo.FindByTransactionReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, ErrChargeNotFound
	}

	if charge.FlowStatus != domain.FlowSendOTP {
		return nil, ErrChargeFlowInvalidStep
	}

	// Verify OTP, constant time comparison to prevent timing attacks.
	// We compare the submitted OTP against what we generated and stored.
	// In simulation any OTP works unless the scenario forces failure —
	// but we still validate the format (6 digits).
	if !isValidOTPFormat(input.OTP) {
		return nil, ErrInvalidOTP
	}

	// Check if the scenario forces an OTP failure.
	result := s.simulatorSvc.ResolveOutcome(input.MerchantID, charge.Channel)

	// For card charges, resolve final outcome after OTP.
	if charge.Channel == domain.ChannelCard {
		if result.Status == domain.TransactionFailed {
			return s.failCharge(charge, result.ErrorCode)
		}
		// Mark OTP as used so the log shows it was consumed
		s.otpRepo.MarkUsed(input.Reference)
		return s.succeedCharge(charge, input.Reference, input.MerchantID)
	}

	// For MoMo, advance to pay_offline after OTP.
	// The customer now needs to approve the USSD prompt on their phone.
	if charge.Channel == domain.ChannelMobileMoney {
		if err := s.chargeRepo.UpdateFlowStatus(charge.ID, domain.FlowPayOffline, ""); err != nil {
			return nil, fmt.Errorf("failed to update flow status: %w", err)
		}
		charge.FlowStatus = domain.FlowPayOffline

		tx, _ := s.transactionRepo.FindByReference(input.Reference, input.MerchantID)

		// Now resolve MoMo asynchronously, same as before but triggered
		// after OTP is verified, not immediately on charge initiation.
		go s.resolveMomoAsync(charge, input.Reference, input.MerchantID)

		return &ChargeFlowResponse{
			Status:      domain.FlowPayOffline,
			Reference:   input.Reference,
			DisplayText: fmt.Sprintf("Approve the payment prompt on %s", charge.MomoPhone),
			Charge:      charge,
			Transaction: tx,
		}, nil
	}

	return nil, ErrChargeFlowInvalidStep
}

// SubmitBirthday processes the date of birth submission for bank charges.
// Any valid date format is accepted, we're simulating, not validating
// against a real bank's records. After birthday, OTP is sent.
func (s *ChargeService) SubmitBirthday(input SubmitBirthdayInput) (*ChargeFlowResponse, error) {
	charge, err := s.chargeRepo.FindByTransactionReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, ErrChargeNotFound
	}

	if charge.FlowStatus != domain.FlowSendBirthday {
		return nil, ErrChargeFlowInvalidStep
	}

	// Check scenario for birthday failure.
	result := s.simulatorSvc.ResolveOutcome(input.MerchantID, domain.ChannelBankTransfer)
	if result.Status == domain.TransactionFailed {
		return s.failCharge(charge, result.ErrorCode)
	}

	// Birthday accepted, generate OTP and advance.
	otpCode, err := otp.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}
	s.otpRepo.Create(input.MerchantID, input.Reference, string(domain.ChannelBankTransfer), "submit_birthday", otpCode)

	if err := s.chargeRepo.UpdateFlowStatus(charge.ID, domain.FlowSendOTP, otpCode); err != nil {
		return nil, fmt.Errorf("failed to update flow status: %w", err)
	}

	charge.FlowStatus = domain.FlowSendOTP

	tx, _ := s.transactionRepo.FindByReference(input.Reference, input.MerchantID)

	return &ChargeFlowResponse{
		Status:      domain.FlowSendOTP,
		Reference:   input.Reference,
		DisplayText: "Enter the OTP sent to your registered phone number",
		OTPCode:     otpCode,
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// SubmitAddress processes the billing address for AVS (Address Verification).
// After address verification the charge resolves directly.
func (s *ChargeService) SubmitAddress(input SubmitAddressInput) (*ChargeFlowResponse, error) {
	charge, err := s.chargeRepo.FindByTransactionReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, ErrChargeNotFound
	}

	if charge.FlowStatus != domain.FlowSendAddress {
		return nil, ErrChargeFlowInvalidStep
	}

	result := s.simulatorSvc.ResolveOutcome(input.MerchantID, domain.ChannelCard)
	if result.Status == domain.TransactionFailed {
		return s.failCharge(charge, result.ErrorCode)
	}

	return s.succeedCharge(charge, input.Reference, input.MerchantID)
}

// Simulate3DS handles the simulated 3DS verification completion.
// In real Paystack the customer completes 3DS on their bank's page
// and gets redirected back. We simulate this with a dedicated endpoint
// that the checkout page calls after showing a fake 3DS form.
func (s *ChargeService) Simulate3DS(reference, merchantID string) (*ChargeFlowResponse, error) {
	charge, err := s.chargeRepo.FindByTransactionReference(reference, merchantID)
	if err != nil {
		return nil, ErrChargeNotFound
	}

	if charge.FlowStatus != domain.FlowOpenURL {
		return nil, ErrChargeFlowInvalidStep
	}

	result := s.simulatorSvc.ResolveOutcome(merchantID, domain.ChannelCard)
	if result.Status == domain.TransactionFailed {
		return s.failCharge(charge, result.ErrorCode)
	}

	return s.succeedCharge(charge, reference, merchantID)
}

// ResendOTPInput is the input for resending an OTP.
type ResendOTPInput struct {
	MerchantID string
	Reference  string
}

// ResendOTP generates a fresh OTP and resets the flow back to send_otp.
// Called when the customer requests a new OTP because the first one
// expired or wasn't received. We generate a completely new OTP —
// the old one is invalidated by overwriting it in the DB.
func (s *ChargeService) ResendOTP(input ResendOTPInput) (*ChargeFlowResponse, error) {
	charge, err := s.chargeRepo.FindByTransactionReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, ErrChargeNotFound
	}

	// Can only resend OTP if currently at the OTP step.
	// If the flow has moved past OTP (or failed) resending makes no sense.
	if charge.FlowStatus != domain.FlowSendOTP {
		return nil, ErrChargeFlowInvalidStep
	}

	newOTP, err := otp.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}
	s.otpRepo.Create(input.MerchantID, input.Reference, string(charge.Channel), "resend_otp", newOTP)

	if err := s.chargeRepo.UpdateFlowStatus(charge.ID, domain.FlowSendOTP, newOTP); err != nil {
		return nil, fmt.Errorf("failed to update OTP: %w", err)
	}

	tx, _ := s.transactionRepo.FindByReference(input.Reference, input.MerchantID)

	return &ChargeFlowResponse{
		Status:      domain.FlowSendOTP,
		Reference:   input.Reference,
		DisplayText: "A new OTP has been sent to your phone",
		OTPCode:     newOTP,
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// FetchCharge retrieves a charge by transaction reference.
func (s *ChargeService) FetchCharge(reference, merchantID string) (*domain.Charge, error) {
	charge, err := s.chargeRepo.FindByReference(reference, merchantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChargeNotFound
		}
		return nil, fmt.Errorf("failed to find charge: %w", err)
	}
	return charge, nil
}

// GetMerchantByAccessCode resolves the merchant through the transaction
// that owns the given access code. Used by public charge endpoints.
func (s *ChargeService) GetMerchantByAccessCode(accessCode string) (*domain.Merchant, error) {
	tx, err := s.transactionRepo.FindByAccessCode(accessCode)
	if err != nil {
		return nil, ErrTransactionNotFound
	}
	merchant, err := s.merchantRepo.FindByID(tx.MerchantID)
	if err != nil {
		return nil, ErrTransactionNotFound
	}
	return merchant, nil
}

// succeedCharge marks a charge and its transaction as successful
// then fires the charge.success webhook.
func (s *ChargeService) succeedCharge(charge *domain.Charge, reference, merchantID string) (*ChargeFlowResponse, error) {
	now := time.Now()

	if err := s.chargeRepo.UpdateFlowStatus(charge.ID, domain.FlowSuccess, ""); err != nil {
		return nil, fmt.Errorf("failed to update charge: %w", err)
	}
	if err := s.chargeRepo.UpdateStatus(charge.ID, domain.TransactionSuccess); err != nil {
		return nil, fmt.Errorf("failed to update charge status: %w", err)
	}
	if err := s.transactionRepo.UpdateStatus(charge.TransactionID, domain.TransactionSuccess, &now); err != nil {
		return nil, fmt.Errorf("failed to update transaction: %w", err)
	}

	charge.FlowStatus = domain.FlowSuccess
	charge.Status = domain.TransactionSuccess

	tx, _ := s.transactionRepo.FindByReference(reference, merchantID)
	if tx != nil {
		tx.Status = domain.TransactionSuccess
		tx.PaidAt = &now
		s.webhookSvc.Dispatch(merchantID, charge.TransactionID, domain.EventChargeSuccess, tx)
	}

	return &ChargeFlowResponse{
		Status:      domain.FlowSuccess,
		Reference:   reference,
		DisplayText: "Payment successful",
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// failCharge marks a charge and its transaction as failed
// then fires the charge.failed webhook.
func (s *ChargeService) failCharge(charge *domain.Charge, errorCode string) (*ChargeFlowResponse, error) {
	if err := s.chargeRepo.UpdateFlowStatus(charge.ID, domain.FlowFailed, ""); err != nil {
		return nil, fmt.Errorf("failed to update charge: %w", err)
	}
	if err := s.chargeRepo.UpdateStatus(charge.ID, domain.TransactionFailed); err != nil {
		return nil, fmt.Errorf("failed to update charge status: %w", err)
	}
	if err := s.chargeRepo.UpdateChargeError(charge.ID, errorCode); err != nil {
		return nil, fmt.Errorf("failed to update charge error: %w", err)
	}
	if err := s.transactionRepo.UpdateStatus(charge.TransactionID, domain.TransactionFailed, nil); err != nil {
		return nil, fmt.Errorf("failed to update transaction: %w", err)
	}

	charge.FlowStatus = domain.FlowFailed
	charge.Status = domain.TransactionFailed
	charge.ChargeErrorCode = errorCode

	tx, _ := s.transactionRepo.FindByID(charge.TransactionID, charge.MerchantID)
	if tx != nil {
		s.webhookSvc.Dispatch(charge.MerchantID, charge.TransactionID, domain.EventChargeFailed, tx)
	}

	return &ChargeFlowResponse{
		Status:      domain.FlowFailed,
		Reference:   charge.TransactionID,
		DisplayText: "Payment failed",
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// resolveMomoAsync resolves a MoMo charge asynchronously after OTP verification.
func (s *ChargeService) resolveMomoAsync(charge *domain.Charge, reference, merchantID string) {
	result := s.simulatorSvc.ResolveOutcome(charge.MerchantID, domain.ChannelMobileMoney)

	if result.Status == domain.TransactionFailed {
		s.failCharge(charge, result.ErrorCode)
		return
	}
	s.succeedCharge(charge, reference, merchantID)
}

// findPendingTransaction looks up a transaction by access_code or reference.
func (s *ChargeService) findPendingTransaction(accessCode, reference, merchantID string) (*domain.Transaction, error) {
	var tx *domain.Transaction
	var err error

	if accessCode != "" {
		tx, err = s.transactionRepo.FindByAccessCode(accessCode)
	} else if reference != "" {
		tx, err = s.transactionRepo.FindByReference(reference, merchantID)
	} else {
		return nil, fmt.Errorf("access_code or reference is required")
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}

	if tx.Status != domain.TransactionPending {
		return nil, ErrTransactionNotPending
	}

	return tx, nil
}

// GetMerchantByReference resolves the merchant through a transaction reference.
// Used by public submit endpoints and the 3DS simulation endpoint.
func (s *ChargeService) GetMerchantByReference(reference string) (*domain.Merchant, error) {
	// We search across all merchants since public endpoints don't have a merchant context.
	// We find the transaction by reference (unique across the system) then get its merchant.
	var tx domain.Transaction
	result := s.transactionRepo.DB().Where("reference = ?", reference).First(&tx)
	if result.Error != nil {
		return nil, ErrTransactionNotFound
	}
	merchant, err := s.merchantRepo.FindByID(tx.MerchantID)
	if err != nil {
		return nil, ErrTransactionNotFound
	}
	return merchant, nil
}

// FetchChargeByTransactionID retrieves the charge for a transaction by its ID.
// Used by the public transaction endpoint to include flow_status in the response
// so the checkout page knows where in the flow a MoMo charge is during polling.
func (s *ChargeService) FetchChargeByTransactionID(transactionID string) (*domain.Charge, error) {
	charge, err := s.chargeRepo.FindByTransactionID(transactionID)
	if err != nil {
		return nil, ErrChargeNotFound
	}
	return charge, nil
}

func safeCardLast4(cardNumber string) string {
	if len(cardNumber) < 4 {
		return ""
	}
	return cardNumber[len(cardNumber)-4:]
}

// detectCardBrand identifies the card network from the first digit.
func detectCardBrand(cardNumber string) string {
	if len(cardNumber) == 0 {
		return "unknown"
	}
	switch {
	case cardNumber[0] == '4':
		return "visa"
	case cardNumber[0] == '5':
		return "mastercard"
	case len(cardNumber) >= 2 && cardNumber[:2] == "37":
		return "amex"
	default:
		return "unknown"
	}
}

// detectCardType identifies whether a card is local or international.
// Visa cards starting with 4 followed by certain ranges and Mastercard
// starting with 5 are treated as international.
// Cards with a 0 as the second digit are treated as local Ghana cards.
// This is a simplified heuristic, real BIN lookup would be more accurate.
func detectCardType(cardNumber string) domain.CardType {
	if len(cardNumber) < 6 {
		return domain.CardTypeLocal
	}
	// Test card ranges, 4111xxxx is the standard Visa test card (international)
	// 5061xxxx is a local Verve card range
	prefix := cardNumber[:4]
	switch prefix {
	case "5061", "5062", "5063", "6500", "6501":
		// Verve card ranges, local Ghana/Nigeria cards
		return domain.CardTypeLocal
	default:
		// Treat all other Visa/Mastercard as international
		if cardNumber[0] == '4' || cardNumber[0] == '5' {
			return domain.CardTypeInternational
		}
		return domain.CardTypeLocal
	}
}

// maskPhone masks the middle digits of a phone number for display.
// +233241234567 → +233241***567
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:6] + "***" + phone[len(phone)-3:]
}

// isValidOTPFormat checks that an OTP is 6 digits.
func isValidOTPFormat(otpCode string) bool {
	if len(otpCode) != 6 {
		return false
	}
	for _, c := range otpCode {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
