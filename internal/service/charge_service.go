package service

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/repository"
	"github.com/payfake/payfake-api/pkg/otp"
	"github.com/payfake/payfake-api/pkg/uid"
	"gorm.io/gorm"
)

type ChargeService struct {
	chargeRepo      *repository.ChargeRepository
	transactionRepo chargeTransactionRepository
	merchantRepo    *repository.MerchantRepository
	customerRepo    chargeCustomerRepository
	otpRepo         *repository.OTPRepository
	simulatorSvc    *SimulatorService
	webhookSvc      *WebhookService
	frontendURL     string
}

type chargeTransactionRepository interface {
	FindByReference(string, string) (*domain.Transaction, error)
	FindByAccessCode(string) (*domain.Transaction, error)
	UpdateStatus(string, domain.TransactionStatus, any) error
	FindByID(string, string) (*domain.Transaction, error)
	Create(*domain.Transaction) error
	UpdateChannel(string, domain.TransactionChannel) error
}

type chargeCustomerRepository interface {
	FindOrCreate(string, string) (*domain.Customer, error)
}

func NewChargeService(
	chargeRepo *repository.ChargeRepository,
	transactionRepo *repository.TransactionRepository,
	merchantRepo *repository.MerchantRepository,
	customerRepo *repository.CustomerRepository,
	otpRepo *repository.OTPRepository,
	simulatorSvc *SimulatorService,
	webhookSvc *WebhookService,
	frontendURL string,
) *ChargeService {
	return &ChargeService{
		chargeRepo:      chargeRepo,
		transactionRepo: transactionRepo,
		merchantRepo:    merchantRepo,
		customerRepo:    customerRepo,
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
	Amount     int64
}

// ChargeMomoInput is the input for initiating a mobile money charge.
type ChargeMomoInput struct {
	MerchantID string
	AccessCode string
	Reference  string
	Phone      string
	Provider   domain.MomoProvider
	Email      string
	Amount     int64
}

// ChargeBankInput is the input for initiating a bank transfer charge.
type ChargeBankInput struct {
	MerchantID    string
	AccessCode    string
	Reference     string
	BankCode      string
	AccountNumber string
	Email         string
	Amount        int64
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
	if !validCard(input.CardNumber, input.CardExpiry, input.CardCVV, time.Now()) {
		return nil, ErrInvalidCard
	}
	tx, err := s.findOrCreateTransaction(
		input.AccessCode, input.Reference,
		input.MerchantID, input.Email, input.Amount,
	)
	if err != nil {
		return nil, err
	}

	cardType := detectCardType(input.CardNumber)
	initialFlow := domain.FlowSendPIN
	if cardType == domain.CardTypeInternational {
		initialFlow = domain.FlowOpenURL
	}

	// Resolve the scenario exactly once for the whole charge. Sampling a
	// failure rate again at PIN, OTP, and 3DS steps compounds the configured
	// probability and makes a supposedly deterministic test difficult to
	// reproduce. A terminal failure or abandonment is applied immediately;
	// a successful sample allows the normal customer-interaction flow to run.
	// If force_status is set to failed we fail before creating the charge
	// and before starting any flow. This is the correct behavior,
	// developers testing failure scenarios shouldn't have to complete
	// the entire PIN → OTP flow just to get a failure.
	result := s.simulatorSvc.ResolveOutcome(input.MerchantID, domain.ChannelCard)
	if result.Status == domain.TransactionFailed || result.Status == domain.TransactionAbandoned {
		// Create the charge in failed state so it's visible in logs
		charge := &domain.Charge{
			Base:              domain.Base{ID: uid.NewChargeID()},
			MerchantID:        input.MerchantID,
			TransactionID:     tx.ID,
			Channel:           domain.ChannelCard,
			Status:            domain.TransactionPending,
			FlowStatus:        initialFlow,
			CardLast4:         safeCardLast4(input.CardNumber),
			CardExpiry:        input.CardExpiry,
			CardBrand:         detectCardBrand(input.CardNumber),
			CardType:          cardType,
			ChargeErrorCode:   result.ErrorCode,
			SimulationDelayMS: result.DelayMS,
		}
		if err := s.createChargeOnce(charge, nil); err != nil {
			return nil, err
		}
		if err := s.setTransactionChannel(tx, domain.ChannelCard); err != nil {
			return nil, err
		}
		if result.Status == domain.TransactionAbandoned {
			return s.abandonCharge(charge)
		}
		return s.failCharge(charge, result.ErrorCode)
	}

	charge := &domain.Charge{
		Base:              domain.Base{ID: uid.NewChargeID()},
		MerchantID:        input.MerchantID,
		TransactionID:     tx.ID,
		Channel:           domain.ChannelCard,
		Status:            domain.TransactionPending,
		CardLast4:         safeCardLast4(input.CardNumber),
		CardExpiry:        input.CardExpiry,
		CardBrand:         detectCardBrand(input.CardNumber),
		CardType:          cardType,
		SimulationDelayMS: result.DelayMS,
	}

	if cardType == domain.CardTypeInternational {
		charge.FlowStatus = domain.FlowOpenURL
		charge.ThreeDSURL = fmt.Sprintf("%s/simulate/3ds/%s", s.frontendURL, tx.Reference)
	} else {
		charge.FlowStatus = domain.FlowSendPIN
	}

	if err := s.createChargeOnce(charge, nil); err != nil {
		return nil, err
	}
	if err := s.setTransactionChannel(tx, domain.ChannelCard); err != nil {
		return nil, err
	}

	resp := &ChargeFlowResponse{
		Status:      charge.FlowStatus,
		Reference:   tx.Reference,
		Charge:      charge,
		Transaction: tx,
	}

	if cardType == domain.CardTypeInternational {
		resp.DisplayText = "Please complete authentication on the provided url"
		resp.ThreeDSURL = charge.ThreeDSURL
	} else {
		resp.DisplayText = "Please enter your PIN"
	}

	return resp, nil
}

// ChargeMobileMoney initiates a MoMo charge.
// Returns send_otp, the customer must enter the OTP sent to their phone.
// After OTP verification the flow moves to pay_offline while waiting
// for the customer to approve the USSD prompt.
func (s *ChargeService) ChargeMobileMoney(input ChargeMomoInput) (*ChargeFlowResponse, error) {
	if input.Phone == "" || !validMomoProvider(input.Provider) {
		return nil, ErrInvalidMomoProvider
	}
	tx, err := s.findOrCreateTransaction(
		input.AccessCode, input.Reference,
		input.MerchantID, input.Email, input.Amount,
	)
	if err != nil {
		return nil, err
	}

	// Check scenario on initiation.
	// MoMo failure can happen immediately, the provider is unavailable,
	// the number is invalid, etc. No point sending an OTP if we know
	// the outcome is forced to failed.
	result := s.simulatorSvc.ResolveOutcome(input.MerchantID, domain.ChannelMobileMoney)
	if result.Status == domain.TransactionFailed || result.Status == domain.TransactionAbandoned {
		charge := &domain.Charge{
			Base:              domain.Base{ID: uid.NewChargeID()},
			MerchantID:        input.MerchantID,
			TransactionID:     tx.ID,
			Channel:           domain.ChannelMobileMoney,
			Status:            domain.TransactionPending,
			FlowStatus:        domain.FlowSendOTP,
			MomoPhone:         input.Phone,
			MomoProvider:      input.Provider,
			ChargeErrorCode:   result.ErrorCode,
			SimulationDelayMS: result.DelayMS,
		}
		if err := s.createChargeOnce(charge, nil); err != nil {
			return nil, err
		}
		if err := s.setTransactionChannel(tx, domain.ChannelMobileMoney); err != nil {
			return nil, err
		}
		if result.Status == domain.TransactionAbandoned {
			return s.abandonCharge(charge)
		}
		return s.failCharge(charge, result.ErrorCode)
	}

	otpCode, err := otp.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	charge := &domain.Charge{
		Base:              domain.Base{ID: uid.NewChargeID()},
		MerchantID:        input.MerchantID,
		TransactionID:     tx.ID,
		Channel:           domain.ChannelMobileMoney,
		Status:            domain.TransactionPending,
		FlowStatus:        domain.FlowSendOTP,
		MomoPhone:         input.Phone,
		MomoProvider:      input.Provider,
		OTPCode:           otpCode,
		SimulationDelayMS: result.DelayMS,
	}

	otpLog := newOTPLog(input.MerchantID, tx.Reference, string(domain.ChannelMobileMoney), "send_otp", otpCode)
	if err := s.createChargeOnce(charge, otpLog); err != nil {
		return nil, err
	}
	if err := s.setTransactionChannel(tx, domain.ChannelMobileMoney); err != nil {
		return nil, err
	}

	return &ChargeFlowResponse{
		Status:      domain.FlowSendOTP,
		Reference:   tx.Reference,
		DisplayText: fmt.Sprintf("Please enter OTP sent to %s", maskPhone(input.Phone)),
		OTPCode:     otpCode,
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// ChargeBank initiates a bank transfer charge.
// Returns send_birthday, the customer must enter their date of birth
// as the first verification step, same as real Paystack bank charges.
func (s *ChargeService) ChargeBank(input ChargeBankInput) (*ChargeFlowResponse, error) {
	if input.BankCode == "" || input.AccountNumber == "" {
		return nil, ErrInvalidBankDetails
	}
	tx, err := s.findOrCreateTransaction(
		input.AccessCode, input.Reference,
		input.MerchantID, input.Email, input.Amount,
	)
	if err != nil {
		return nil, err
	}

	// Check scenario on initiation.
	result := s.simulatorSvc.ResolveOutcome(input.MerchantID, domain.ChannelBankTransfer)
	if result.Status == domain.TransactionFailed || result.Status == domain.TransactionAbandoned {
		charge := &domain.Charge{
			Base:              domain.Base{ID: uid.NewChargeID()},
			MerchantID:        input.MerchantID,
			TransactionID:     tx.ID,
			Channel:           domain.ChannelBankTransfer,
			Status:            domain.TransactionPending,
			FlowStatus:        domain.FlowSendBirthday,
			BankCode:          input.BankCode,
			BankAccountNumber: input.AccountNumber,
			ChargeErrorCode:   result.ErrorCode,
			SimulationDelayMS: result.DelayMS,
		}
		if err := s.createChargeOnce(charge, nil); err != nil {
			return nil, err
		}
		if err := s.setTransactionChannel(tx, domain.ChannelBankTransfer); err != nil {
			return nil, err
		}
		if result.Status == domain.TransactionAbandoned {
			return s.abandonCharge(charge)
		}
		return s.failCharge(charge, result.ErrorCode)
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
		SimulationDelayMS: result.DelayMS,
	}

	if err := s.createChargeOnce(charge, nil); err != nil {
		return nil, err
	}
	if err := s.setTransactionChannel(tx, domain.ChannelBankTransfer); err != nil {
		return nil, err
	}

	return &ChargeFlowResponse{
		Status:      domain.FlowSendBirthday,
		Reference:   tx.Reference,
		DisplayText: "Please enter your date of birth",
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
	if len(input.PIN) != 4 || !allDigits(input.PIN) {
		return nil, ErrInvalidPIN
	}
	charge, err := s.chargeRepo.FindByTransactionReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, ErrChargeNotFound
	}

	// Validate we're at the right step, can't submit PIN if flow
	// is already past the PIN step or in a terminal state.
	if charge.FlowStatus != domain.FlowSendPIN {
		return nil, ErrChargeFlowInvalidStep
	}
	tx, err := s.transactionRepo.FindByReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to load transaction for PIN step: %w", err)
	}

	// PIN accepted, generate OTP and advance to OTP step.
	otpCode, err := otp.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}
	otpLog := newOTPLog(input.MerchantID, input.Reference, string(domain.ChannelCard), "submit_pin", otpCode)
	advanced, err := s.chargeRepo.AdvanceFlow(
		charge.ID, domain.FlowSendPIN, domain.FlowSendOTP,
		"", otpCode, otpLog, "", input.MerchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update flow status: %w", err)
	}
	if !advanced {
		return nil, ErrChargeFlowInvalidStep
	}

	charge.FlowStatus = domain.FlowSendOTP

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

	// Reject malformed values before loading OTP state. The actual secret is
	// compared below in constant time so a caller cannot learn matching prefix
	// length from response timing.
	if !isValidOTPFormat(input.OTP) {
		return nil, ErrInvalidOTP
	}

	// Check OTP expiry, a 10-minute-old OTP is rejected.
	// We look up the most recent unused OTP log for this reference
	// and verify it hasn't expired. This prevents replay attacks where
	// a valid OTP from a previous session is reused.
	otpLogs, err := s.otpRepo.FindByReference(input.Reference, input.MerchantID)
	if err != nil || len(otpLogs) == 0 {
		return nil, ErrInvalidOTP
	}

	// The most recent OTP is first, FindByReference orders DESC.
	latestOTP := otpLogs[0]
	if latestOTP.Used {
		return nil, ErrInvalidOTP
	}
	if time.Now().After(latestOTP.ExpiresAt) {
		return nil, ErrOTPExpired
	}
	if !otpMatches(latestOTP.OTPCode, input.OTP) {
		return nil, ErrInvalidOTP
	}

	if charge.Channel == domain.ChannelCard {
		return s.succeedCharge(charge, input.Reference, input.MerchantID, latestOTP.ID)
	}

	// For MoMo, advance to pay_offline after OTP.
	// The customer now needs to approve the USSD prompt on their phone.
	if charge.Channel == domain.ChannelMobileMoney {
		tx, err := s.transactionRepo.FindByReference(input.Reference, input.MerchantID)
		if err != nil {
			return nil, fmt.Errorf("failed to load transaction for MoMo step: %w", err)
		}
		advanced, err := s.chargeRepo.AdvanceFlow(
			charge.ID, domain.FlowSendOTP, domain.FlowPayOffline,
			charge.OTPCode, "", nil, latestOTP.ID, input.MerchantID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update flow status: %w", err)
		}
		if !advanced {
			return nil, ErrChargeFlowInvalidStep
		}
		charge.FlowStatus = domain.FlowPayOffline

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

	// Bank channel OTP
	if charge.Channel == domain.ChannelBankTransfer {
		return s.succeedCharge(charge, input.Reference, input.MerchantID, latestOTP.ID)
	}

	return nil, ErrChargeFlowInvalidStep
}

// SubmitBirthday processes the date of birth submission for bank charges.
// Any valid date format is accepted, we're simulating, not validating
// against a real bank's records. After birthday, OTP is sent.
func (s *ChargeService) SubmitBirthday(input SubmitBirthdayInput) (*ChargeFlowResponse, error) {
	birthday, err := time.Parse("2006-01-02", input.Birthday)
	if err != nil || birthday.After(time.Now()) {
		return nil, ErrInvalidBirthday
	}
	charge, err := s.chargeRepo.FindByTransactionReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, ErrChargeNotFound
	}

	if charge.FlowStatus != domain.FlowSendBirthday {
		return nil, ErrChargeFlowInvalidStep
	}
	tx, err := s.transactionRepo.FindByReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to load transaction for birthday step: %w", err)
	}

	// Birthday accepted, generate OTP and advance.
	otpCode, err := otp.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}
	otpLog := newOTPLog(input.MerchantID, input.Reference, string(domain.ChannelBankTransfer), "submit_birthday", otpCode)
	advanced, err := s.chargeRepo.AdvanceFlow(
		charge.ID, domain.FlowSendBirthday, domain.FlowSendOTP,
		"", otpCode, otpLog, "", input.MerchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update flow status: %w", err)
	}
	if !advanced {
		return nil, ErrChargeFlowInvalidStep
	}

	charge.FlowStatus = domain.FlowSendOTP

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

	return s.succeedCharge(charge, input.Reference, input.MerchantID, "")
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

	return s.succeedCharge(charge, reference, merchantID, "")
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
	tx, err := s.transactionRepo.FindByReference(input.Reference, input.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to load transaction for OTP resend: %w", err)
	}

	newOTP, err := otp.GenerateOTP()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}
	otpLog := newOTPLog(input.MerchantID, input.Reference, string(charge.Channel), "resend_otp", newOTP)
	advanced, err := s.chargeRepo.AdvanceFlow(
		charge.ID, domain.FlowSendOTP, domain.FlowSendOTP,
		charge.OTPCode, newOTP, otpLog, "", input.MerchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update OTP: %w", err)
	}
	if !advanced {
		return nil, ErrChargeFlowInvalidStep
	}

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
func (s *ChargeService) succeedCharge(charge *domain.Charge, reference, merchantID, otpLogID string) (*ChargeFlowResponse, error) {
	applySimulationDelay(charge.SimulationDelayMS)
	now := time.Now()

	finalized, err := s.chargeRepo.Finalize(
		charge.ID, charge.TransactionID, charge.FlowStatus,
		domain.TransactionSuccess, domain.TransactionSuccess, domain.FlowSuccess,
		"", &now, otpLogID, merchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize successful charge: %w", err)
	}
	if !finalized {
		return nil, ErrChargeFlowInvalidStep
	}

	charge.FlowStatus = domain.FlowSuccess
	charge.Status = domain.TransactionSuccess

	tx, err := s.transactionRepo.FindByReference(reference, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload successful transaction: %w", err)
	}
	tx.Status = domain.TransactionSuccess
	tx.PaidAt = &now
	if err := s.webhookSvc.Dispatch(merchantID, charge.TransactionID, domain.EventChargeSuccess, tx); err != nil {
		log.Printf("[payfake] successful charge %s committed but webhook enqueue failed: %v", reference, err)
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
	applySimulationDelay(charge.SimulationDelayMS)
	finalized, err := s.chargeRepo.Finalize(
		charge.ID, charge.TransactionID, charge.FlowStatus,
		domain.TransactionFailed, domain.TransactionFailed, domain.FlowFailed,
		errorCode, nil, "", charge.MerchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize failed charge: %w", err)
	}
	if !finalized {
		return nil, ErrChargeFlowInvalidStep
	}

	charge.FlowStatus = domain.FlowFailed
	charge.Status = domain.TransactionFailed
	charge.ChargeErrorCode = errorCode

	tx, err := s.transactionRepo.FindByID(charge.TransactionID, charge.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload failed transaction: %w", err)
	}
	tx.Status = domain.TransactionFailed
	if err := s.webhookSvc.Dispatch(charge.MerchantID, charge.TransactionID, domain.EventChargeFailed, tx); err != nil {
		log.Printf("[payfake] failed charge %s committed but webhook enqueue failed: %v", tx.Reference, err)
	}

	return &ChargeFlowResponse{
		Status:      domain.FlowFailed,
		Reference:   tx.Reference,
		DisplayText: "Payment failed",
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// abandonCharge applies a forced-abandoned scenario without pretending the
// charge itself succeeded. Abandonment is a terminal transaction state but not
// a Paystack charge event, so no success/failed webhook is emitted here.
func (s *ChargeService) abandonCharge(charge *domain.Charge) (*ChargeFlowResponse, error) {
	applySimulationDelay(charge.SimulationDelayMS)
	finalized, err := s.chargeRepo.Finalize(
		charge.ID, charge.TransactionID, charge.FlowStatus,
		domain.TransactionAbandoned, domain.TransactionFailed, domain.FlowFailed,
		"CHARGE_ABANDONED", nil, "", charge.MerchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to abandon charge: %w", err)
	}
	if !finalized {
		return nil, ErrChargeFlowInvalidStep
	}

	tx, err := s.transactionRepo.FindByID(charge.TransactionID, charge.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload abandoned transaction: %w", err)
	}
	charge.Status = domain.TransactionFailed
	charge.FlowStatus = domain.FlowFailed
	charge.ChargeErrorCode = "CHARGE_ABANDONED"
	tx.Status = domain.TransactionAbandoned
	return &ChargeFlowResponse{
		Status:      domain.FlowFailed,
		Reference:   tx.Reference,
		DisplayText: "Payment abandoned",
		Charge:      charge,
		Transaction: tx,
	}, nil
}

// resolveMomoAsync resolves a MoMo charge asynchronously after OTP verification.
func (s *ChargeService) resolveMomoAsync(charge *domain.Charge, reference, merchantID string) {
	if _, err := s.succeedCharge(charge, reference, merchantID, ""); err != nil && !errors.Is(err, ErrChargeFlowInvalidStep) {
		log.Printf("[payfake] failed to resolve MoMo charge %s: %v", reference, err)
	}
}

// findOrCreateTransaction finds an existing pending transaction via access_code
// or reference, or creates one inline when called directly with email+amount.
// Real Paystack supports calling /charge directly without a prior /transaction/initialize.
func (s *ChargeService) findOrCreateTransaction(
	accessCode, reference, merchantID, email string, amount int64,
) (*domain.Transaction, error) {
	tx, hadLookupInput, err := s.findPendingTransaction(accessCode, reference, merchantID)
	if err != nil {
		return nil, err
	}
	if tx != nil {
		return tx, nil
	}
	if hadLookupInput {
		return nil, ErrTransactionNotFound
	}

	// Neither provided, create inline transaction
	if email == "" || amount <= 0 {
		if email == "" {
			return nil, ErrTransactionNotFound
		}
		return nil, ErrInvalidAmount
	}

	customer, err := s.customerRepo.FindOrCreate(merchantID, email)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve customer: %w", err)
	}

	tx = &domain.Transaction{
		Base:                domain.Base{ID: uid.NewTransactionID()},
		MerchantID:          merchantID,
		CustomerID:          customer.ID,
		Amount:              amount,
		Currency:            domain.CurrencyGHS,
		Status:              domain.TransactionPending,
		Reference:           reference,
		AccessCode:          uid.NewAccessCode(),
		AccessCodeExpiresAt: time.Now().Add(time.Hour),
	}
	if tx.Reference == "" {
		tx.Reference = uid.NewReference()
	}

	if err := s.transactionRepo.Create(tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	tx.Customer = *customer
	return tx, nil
}

func (s *ChargeService) findPendingTransaction(
	accessCode, reference, merchantID string,
) (*domain.Transaction, bool, error) {
	hadLookupInput := accessCode != "" || reference != ""

	if accessCode != "" {
		tx, err := s.transactionRepo.FindByAccessCode(accessCode)
		if err == nil {
			if tx.Status != domain.TransactionPending {
				return nil, true, ErrTransactionNotPending
			}
			return tx, true, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, true, fmt.Errorf("failed to find transaction by access code: %w", err)
		}
	}

	if reference != "" {
		tx, err := s.transactionRepo.FindByReference(reference, merchantID)
		if err == nil {
			if tx.Status != domain.TransactionPending {
				return nil, true, ErrTransactionNotPending
			}
			return tx, true, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, true, fmt.Errorf("failed to find transaction by reference: %w", err)
		}
	}

	return nil, hadLookupInput, nil
}

// GetMerchantByAccessCodeAndReference validates a public charge-step request.
// Public checkout actions must prove both the checkout token (access_code)
// and the transaction reference before mutating any charge state.
func (s *ChargeService) GetMerchantByAccessCodeAndReference(accessCode, reference string) (*domain.Merchant, error) {
	tx, err := s.transactionRepo.FindByAccessCode(accessCode)
	if err != nil {
		return nil, ErrTransactionNotFound
	}
	if tx.Reference != reference {
		return nil, ErrTransactionNotFound
	}

	_, err = s.chargeRepo.FindByTransactionID(tx.ID)
	if err != nil {
		return nil, ErrChargeNotFound
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

func (s *ChargeService) setTransactionChannel(tx *domain.Transaction, channel domain.TransactionChannel) error {
	if tx.Channel == channel {
		return nil
	}
	if err := s.transactionRepo.UpdateChannel(tx.ID, channel); err != nil {
		return fmt.Errorf("failed to update transaction channel: %w", err)
	}
	tx.Channel = channel
	return nil
}

func (s *ChargeService) createChargeOnce(charge *domain.Charge, otpLog *domain.OTPLog) error {
	created, err := s.chargeRepo.CreateOnce(charge, otpLog)
	if err != nil {
		return fmt.Errorf("failed to create charge: %w", err)
	}
	if !created {
		return ErrChargeFlowInvalidStep
	}
	return nil
}

func newOTPLog(merchantID, reference, channel, step, otpCode string) *domain.OTPLog {
	return &domain.OTPLog{
		Base:       domain.Base{ID: uid.NewRequestLogID()},
		MerchantID: merchantID,
		Reference:  reference,
		Channel:    channel,
		OTPCode:    otpCode,
		Step:       step,
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}
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

func otpMatches(expected, submitted string) bool {
	if !isValidOTPFormat(expected) || !isValidOTPFormat(submitted) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(submitted)) == 1
}

func validMomoProvider(provider domain.MomoProvider) bool {
	switch provider {
	case domain.ProviderMTN, domain.ProviderVodafone, domain.ProviderAirtelTigo:
		return true
	default:
		return false
	}
}

func validCard(number, expiry, cvv string, now time.Time) bool {
	if len(number) < 13 || len(number) > 19 || !allDigits(number) || !passesLuhn(number) {
		return false
	}
	if (len(cvv) != 3 && len(cvv) != 4) || !allDigits(cvv) {
		return false
	}
	parts := strings.Split(expiry, "/")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return false
	}
	month, monthErr := strconv.Atoi(parts[0])
	year, yearErr := strconv.Atoi(parts[1])
	if monthErr != nil || yearErr != nil || month < 1 || month > 12 {
		return false
	}
	fullYear := 2000 + year
	return fullYear > now.Year() || (fullYear == now.Year() && month >= int(now.Month()))
}

func passesLuhn(number string) bool {
	sum := 0
	double := len(number)%2 == 0
	for _, char := range number {
		digit := int(char - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	return sum%10 == 0
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func applySimulationDelay(delayMS int) {
	if delayMS > 0 {
		time.Sleep(time.Duration(delayMS) * time.Millisecond)
	}
}
