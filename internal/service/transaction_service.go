package service

import (
	"errors"
	"fmt"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/repository"
	"github.com/GordenArcher/payfake/pkg/uid"
	"gorm.io/gorm"
)

type TransactionService struct {
	transactionRepo *repository.TransactionRepository
	customerService *CustomerService
}

func NewTransactionService(
	transactionRepo *repository.TransactionRepository,
	customerService *CustomerService,
) *TransactionService {
	return &TransactionService{
		transactionRepo: transactionRepo,
		customerService: customerService,
	}
}

type InitializeInput struct {
	MerchantID  string
	Email       string
	Amount      int64
	Currency    domain.Currency
	Reference   string
	CallbackURL string
	Channels    []domain.TransactionChannel
	Metadata    domain.JSON
}

// InitializeOutput is returned to the handler after a transaction
// is created. The authorization_url is what the frontend uses to
// open the payment popup. The access_code is the token the popup
// sends with the charge request.
type InitializeOutput struct {
	AuthorizationURL string
	AccessCode       string
	Reference        string
}

// Initialize creates a new pending transaction.
// This mirrors Paystack's POST /transaction/initialize exactly.
// No money moves here, we just create the record and return
// the tokens the frontend needs to open the payment popup.
func (s *TransactionService) Initialize(input InitializeInput) (*InitializeOutput, error) {
	// Validate amount early, no point hitting the DB if the amount
	// is invalid. We validate in the service not the handler because
	// business rules belong in the service layer.
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	// Validate currency is one we support.
	validCurrencies := map[domain.Currency]bool{
		domain.CurrencyGHS: true,
		domain.CurrencyNGN: true,
		domain.CurrencyKES: true,
		domain.CurrencyUSD: true,
	}
	if !validCurrencies[input.Currency] {
		return nil, ErrInvalidCurrency
	}

	// Check reference uniqueness per merchant.
	// If the same reference is sent twice it means the developer is
	// retrying an existing transaction, we reject the duplicate
	// to enforce idempotency. They should verify the existing
	// transaction instead of initializing a new one.
	if input.Reference != "" {
		exists, err := s.transactionRepo.ReferenceExists(input.Reference, input.MerchantID)
		if err != nil {
			return nil, fmt.Errorf("failed to check reference: %w", err)
		}
		if exists {
			return nil, ErrReferenceTaken
		}
	}

	// Find or create the customer by email.
	// Paystack's initialize accepts an email and handles the customer
	// lookup/creation transparently, we do the same.
	customer, err := s.customerService.FindOrCreate(input.MerchantID, input.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to find or create customer: %w", err)
	}

	// Generate the reference if none was provided.
	// Paystack generates one if the developer doesn't send one,
	// we do the same so the reference field is always populated.
	reference := input.Reference
	if reference == "" {
		reference = uid.NewTransactionID()
	}

	accessCode := uid.NewAccessCode()

	// The authorization URL is what the frontend opens.
	// It points to Payfake's payment popup page, the same UX
	// as Paystack's hosted payment page but running locally.
	authURL := fmt.Sprintf("http://localhost:8080/pay/%s", accessCode)

	tx := &domain.Transaction{
		Base:        domain.Base{ID: uid.NewTransactionID()},
		MerchantID:  input.MerchantID,
		CustomerID:  customer.ID,
		Reference:   reference,
		Amount:      input.Amount,
		Currency:    input.Currency,
		Status:      domain.TransactionPending,
		AccessCode:  accessCode,
		CallbackURL: input.CallbackURL,
		Metadata:    input.Metadata,
	}

	// Set the channel if only one was provided.
	// If multiple channels are allowed we leave it empty until
	// the customer selects one in the popup.
	if len(input.Channels) == 1 {
		tx.Channel = input.Channels[0]
	}

	if err := s.transactionRepo.Create(tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	return &InitializeOutput{
		AuthorizationURL: authURL,
		AccessCode:       accessCode,
		Reference:        reference,
	}, nil
}

// Verify retrieves a transaction by reference and returns its current state.
// This is what developers call after the payment popup closes to confirm
// the outcome, same as Paystack's GET /transaction/verify/:reference.
func (s *TransactionService) Verify(reference, merchantID string) (*domain.Transaction, error) {
	tx, err := s.transactionRepo.FindByReference(reference, merchantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}
	return tx, nil
}

// Get retrieves a single transaction by ID.
func (s *TransactionService) Get(id, merchantID string) (*domain.Transaction, error) {
	tx, err := s.transactionRepo.FindByID(id, merchantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}
	return tx, nil
}

// List returns paginated transactions for a merchant with optional status filter.
func (s *TransactionService) List(merchantID string, status domain.TransactionStatus, page, perPage int) ([]domain.Transaction, int64, error) {
	offset := (page - 1) * perPage
	return s.transactionRepo.List(merchantID, status, offset, perPage)
}

// Refund marks a successful transaction as reversed.
// We only allow refunding transactions that are in "success" state,
// you can't refund a failed or pending transaction.
func (s *TransactionService) Refund(id, merchantID string) (*domain.Transaction, error) {
	tx, err := s.transactionRepo.FindByID(id, merchantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}

	if tx.Status == domain.TransactionReversed {
		return nil, ErrTransactionAlreadyRefunded
	}

	if tx.Status != domain.TransactionSuccess {
		return nil, ErrTransactionAlreadyVerified
	}

	if err := s.transactionRepo.UpdateStatus(id, domain.TransactionReversed, nil); err != nil {
		return nil, fmt.Errorf("failed to refund transaction: %w", err)
	}

	tx.Status = domain.TransactionReversed
	return tx, nil
}
