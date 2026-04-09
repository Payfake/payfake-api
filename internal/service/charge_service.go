package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/repository"
	"github.com/GordenArcher/payfake/pkg/uid"
	"gorm.io/gorm"
)

type ChargeService struct {
	chargeRepo      *repository.ChargeRepository
	transactionRepo *repository.TransactionRepository
	merchantRepo    *repository.MerchantRepository
	simulatorSvc    *SimulatorService
	webhookSvc      *WebhookService
}

func NewChargeService(
	chargeRepo *repository.ChargeRepository,
	transactionRepo *repository.TransactionRepository,
	merchantRepo *repository.MerchantRepository,
	simulatorSvc *SimulatorService,
	webhookSvc *WebhookService,
) *ChargeService {
	return &ChargeService{
		chargeRepo:      chargeRepo,
		transactionRepo: transactionRepo,
		merchantRepo:    merchantRepo,
		simulatorSvc:    simulatorSvc,
		webhookSvc:      webhookSvc,
	}
}

// ChargeCardInput is the input for a direct card charge.
type ChargeCardInput struct {
	MerchantID string
	AccessCode string
	Reference  string
	CardNumber string
	CardExpiry string
	CardCVV    string
	Email      string
}

// ChargeMomoInput is the input for a mobile money charge.
type ChargeMomoInput struct {
	MerchantID string
	AccessCode string
	Reference  string
	Phone      string
	Provider   domain.MomoProvider
	Email      string
}

// ChargeBankInput is the input for a bank transfer charge.
type ChargeBankInput struct {
	MerchantID    string
	AccessCode    string
	Reference     string
	BankCode      string
	AccountNumber string
	Email         string
}

// ChargeOutput is returned after any charge attempt.
// The handler uses Status and ErrorCode to build the correct response.
type ChargeOutput struct {
	Transaction *domain.Transaction
	Charge      *domain.Charge
	Status      domain.TransactionStatus
	ErrorCode   string
}

// ChargeCard processes a card payment.
// Flow:
//  1. Find the pending transaction by access_code or reference
//  2. Create the charge record
//  3. Run it through the simulator to get the outcome
//  4. Update charge and transaction status
//  5. Fire the appropriate webhook event
func (s *ChargeService) ChargeCard(input ChargeCardInput) (*ChargeOutput, error) {
	tx, err := s.findPendingTransaction(input.AccessCode, input.Reference, input.MerchantID)
	if err != nil {
		return nil, err
	}

	charge := &domain.Charge{
		Base:          domain.Base{ID: uid.NewChargeID()},
		MerchantID:    input.MerchantID,
		TransactionID: tx.ID,
		Channel:       domain.ChannelCard,
		Status:        domain.TransactionPending,
		// We store only the last 4 digits of the card number,
		// never the full number. Even in a simulator we build
		// good habits around not storing sensitive card data.
		CardLast4: safeCardLast4(input.CardNumber),
		CardBrand: detectCardBrand(input.CardNumber),
	}

	if err := s.chargeRepo.Create(charge); err != nil {
		return nil, fmt.Errorf("failed to create charge: %w", err)
	}

	return s.resolveAndFinalize(tx, charge, domain.ChannelCard)
}

// ChargeMobileMoney processes a mobile money payment.
// MoMo charges are async by nature — in real life the customer gets
// a USSD prompt on their phone and must approve it. We simulate this
// by returning "pending" immediately for MoMo charges and resolving
// the outcome asynchronously via webhook — same as real Paystack.
func (s *ChargeService) ChargeMobileMoney(input ChargeMomoInput) (*ChargeOutput, error) {
	tx, err := s.findPendingTransaction(input.AccessCode, input.Reference, input.MerchantID)
	if err != nil {
		return nil, err
	}

	charge := &domain.Charge{
		Base:          domain.Base{ID: uid.NewChargeID()},
		MerchantID:    input.MerchantID,
		TransactionID: tx.ID,
		Channel:       domain.ChannelMobileMoney,
		Status:        domain.TransactionPending,
		MomoPhone:     input.Phone,
		MomoProvider:  input.Provider,
	}

	if err := s.chargeRepo.Create(charge); err != nil {
		return nil, fmt.Errorf("failed to create charge: %w", err)
	}

	// Update transaction channel now that we know it.
	tx.Channel = domain.ChannelMobileMoney
	s.transactionRepo.UpdateStatus(tx.ID, domain.TransactionPending, nil)

	// For MoMo we return pending immediately and resolve asynchronously.
	// The simulator runs in a goroutine — it applies the configured delay
	// (simulating the customer approval window) then fires the webhook.
	go s.resolveMomoAsync(tx, charge)

	return &ChargeOutput{
		Transaction: tx,
		Charge:      charge,
		Status:      domain.TransactionPending,
		ErrorCode:   "",
	}, nil
}

// ChargeBank processes a bank transfer payment.
func (s *ChargeService) ChargeBank(input ChargeBankInput) (*ChargeOutput, error) {
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
		BankCode:          input.BankCode,
		BankAccountNumber: input.AccountNumber,
	}

	if err := s.chargeRepo.Create(charge); err != nil {
		return nil, fmt.Errorf("failed to create charge: %w", err)
	}

	return s.resolveAndFinalize(tx, charge, domain.ChannelBankTransfer)
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

// resolveAndFinalize runs the simulator and writes the outcome to
// the charge and transaction records, then fires the webhook.
// This is the shared finalization logic for card and bank charges.
// MoMo uses resolveMomoAsync instead because it runs asynchronously.
func (s *ChargeService) resolveAndFinalize(
	tx *domain.Transaction,
	charge *domain.Charge,
	channel domain.TransactionChannel,
) (*ChargeOutput, error) {
	// Run the simulation, this is where the outcome is decided.
	result := s.simulatorSvc.ResolveOutcome(tx.MerchantID, channel)

	// Update the charge status first.
	if err := s.chargeRepo.UpdateStatus(charge.ID, result.Status); err != nil {
		return nil, fmt.Errorf("failed to update charge status: %w", err)
	}

	// Update the transaction status.
	var paidAt *time.Time
	if result.Status == domain.TransactionSuccess {
		now := time.Now()
		paidAt = &now
	}

	if err := s.transactionRepo.UpdateStatus(tx.ID, result.Status, paidAt); err != nil {
		return nil, fmt.Errorf("failed to update transaction status: %w", err)
	}

	tx.Status = result.Status
	tx.Channel = channel
	charge.Status = result.Status

	// Fire the appropriate webhook event based on outcome.
	eventType := domain.EventChargeSuccess
	if result.Status == domain.TransactionFailed {
		eventType = domain.EventChargeFailed
	}

	// Webhook dispatch is fire-and-forget, errors here don't fail the charge.
	// The developer can retry failed webhooks from the control panel.
	s.webhookSvc.Dispatch(tx.MerchantID, tx.ID, eventType, tx)

	return &ChargeOutput{
		Transaction: tx,
		Charge:      charge,
		Status:      result.Status,
		ErrorCode:   result.ErrorCode,
	}, nil
}

// resolveMomoAsync simulates the async nature of MoMo payments.
// In real Paystack MoMo flows, the charge returns "send_otp" or "pending"
// and the final status arrives via webhook after the customer approves.
// We simulate this by sleeping for the configured delay then resolving.
func (s *ChargeService) resolveMomoAsync(tx *domain.Transaction, charge *domain.Charge) {
	// The simulator already applied DelayMS inside ResolveOutcome,
	// for MoMo we run ResolveOutcome here in the goroutine so the
	// delay happens asynchronously and doesn't block the response.
	result := s.simulatorSvc.ResolveOutcome(tx.MerchantID, domain.ChannelMobileMoney)

	var paidAt *time.Time
	if result.Status == domain.TransactionSuccess {
		now := time.Now()
		paidAt = &now
	}

	s.chargeRepo.UpdateStatus(charge.ID, result.Status)
	s.transactionRepo.UpdateStatus(tx.ID, result.Status, paidAt)

	tx.Status = result.Status

	eventType := domain.EventChargeSuccess
	if result.Status == domain.TransactionFailed {
		eventType = domain.EventChargeFailed
	}

	s.webhookSvc.Dispatch(tx.MerchantID, tx.ID, eventType, tx)
}

// findPendingTransaction looks up a transaction by access_code or reference.
// We try access_code first (popup flow), then fall back to reference
// (direct API flow). The transaction must be in pending state,
// you can't charge a transaction that's already been completed.
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

	// Guard against double-charging, once a transaction leaves pending
	// state it cannot be charged again. The developer must initialize
	// a new transaction for a new charge attempt.
	if tx.Status != domain.TransactionPending {
		return nil, ErrTransactionNotPending
	}

	return tx, nil
}

// safeCardLast4 extracts the last 4 digits of a card number safely.
// If the card number is shorter than 4 characters we return an empty
// string rather than panicking with an index out of bounds error.
func safeCardLast4(cardNumber string) string {
	if len(cardNumber) < 4 {
		return ""
	}
	return cardNumber[len(cardNumber)-4:]
}

// detectCardBrand identifies the card network from the card number prefix.
// This is a simplified version of the full BIN lookup, just enough
// to return a brand name in the charge response.
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

// GetMerchantByAccessCode looks up the merchant who owns the transaction
// that this access code belongs to. Used by the public charge endpoints
// to resolve the merchant without an Authorization header.
// Returns an error if the access code is invalid or the transaction
// is not in pending state — you can't charge a completed transaction.
func (s *ChargeService) GetMerchantByAccessCode(accessCode string) (*domain.Merchant, error) {
	tx, err := s.transactionRepo.FindByAccessCode(accessCode)
	if err != nil {
		return nil, ErrTransactionNotFound
	}

	// We need the merchant to apply their scenario config during simulation.
	// The transaction carries the merchant_id — one DB lookup gets us there.
	merchant, err := s.merchantRepo.FindByID(tx.MerchantID)
	if err != nil {
		return nil, ErrTransactionNotFound
	}

	return merchant, nil
}
