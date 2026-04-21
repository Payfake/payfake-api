package service

import (
	"errors"
	"fmt"

	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/repository"
	"github.com/payfake/payfake-api/pkg/uid"
	"gorm.io/gorm"
)

type CustomerService struct {
	customerRepo *repository.CustomerRepository
}

func NewCustomerService(customerRepo *repository.CustomerRepository) *CustomerService {
	return &CustomerService{customerRepo: customerRepo}
}

type CreateCustomerInput struct {
	MerchantID string
	Email      string
	FirstName  string
	LastName   string
	Phone      string
	Metadata   domain.JSON
}

// Create creates a new customer under a merchant account.
// If a customer with the same email already exists under this merchant
// we return ErrCustomerEmailTaken, the handler maps this to the
// correct response code.
func (s *CustomerService) Create(input CreateCustomerInput) (*domain.Customer, error) {
	exists, err := s.customerRepo.EmailExists(input.Email, input.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, ErrCustomerEmailTaken
	}

	customer := &domain.Customer{
		Base:       domain.Base{ID: uid.NewCustomerID()},
		MerchantID: input.MerchantID,
		Email:      input.Email,
		FirstName:  input.FirstName,
		LastName:   input.LastName,
		Phone:      input.Phone,
		// Customer code is the public-facing identifier — CUS_xxxxxxxx.
		// This is what Paystack returns and what developers store
		// in their own DB to reference a customer.
		Code:     uid.NewCustomerID(),
		Metadata: input.Metadata,
	}

	if err := s.customerRepo.Create(customer); err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	return customer, nil
}

// FindOrCreate looks up a customer by email, creating them if they
// don't exist. This is called during transaction initialize,
// Paystack's initialize endpoint accepts an email and either finds
// the existing customer or creates a new one transparently.
// Developers don't need to pre-create customers before charging them.
func (s *CustomerService) FindOrCreate(merchantID, email string) (*domain.Customer, error) {
	customer, err := s.customerRepo.FindByEmail(email, merchantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Customer doesn't exist, create a minimal record.
			// We only have the email at this point, name and phone
			// can be filled in later via the update endpoint.
			return s.Create(CreateCustomerInput{
				MerchantID: merchantID,
				Email:      email,
			})
		}
		return nil, fmt.Errorf("failed to find customer: %w", err)
	}
	return customer, nil
}

type UpdateCustomerInput struct {
	FirstName *string
	LastName  *string
	Phone     *string
	Metadata  domain.JSON
}

// Update applies partial updates to a customer record.
// Pointer fields mean nil = don't touch, same pattern as scenario update.
func (s *CustomerService) Update(code, merchantID string, input UpdateCustomerInput) (*domain.Customer, error) {
	customer, err := s.customerRepo.FindByCode(code, merchantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCustomerNotFound
		}
		return nil, fmt.Errorf("failed to find customer: %w", err)
	}

	if input.FirstName != nil {
		customer.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		customer.LastName = *input.LastName
	}
	if input.Phone != nil {
		customer.Phone = *input.Phone
	}
	if input.Metadata != nil {
		customer.Metadata = input.Metadata
	}

	if err := s.customerRepo.Update(customer); err != nil {
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	return customer, nil
}

// Get retrieves a single customer by code scoped to a merchant.
func (s *CustomerService) Get(code, merchantID string) (*domain.Customer, error) {
	customer, err := s.customerRepo.FindByCode(code, merchantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCustomerNotFound
		}
		return nil, fmt.Errorf("failed to find customer: %w", err)
	}
	return customer, nil
}

// List returns a paginated list of customers for a merchant.
func (s *CustomerService) List(merchantID string, page, perPage int) ([]domain.Customer, int64, error) {
	offset := (page - 1) * perPage
	return s.customerRepo.List(merchantID, offset, perPage)
}
