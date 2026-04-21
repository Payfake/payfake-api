package repository

import (
	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
)

type CustomerRepository struct {
	db *gorm.DB
}

func NewCustomerRepository(db *gorm.DB) *CustomerRepository {
	return &CustomerRepository{db: db}
}

// Create inserts a new customer under a merchant account.
func (r *CustomerRepository) Create(customer *domain.Customer) error {
	return r.db.Create(customer).Error
}

// FindByID retrieves a customer by primary ID scoped to a merchant.
// Scoping by merchant_id is critical, without it a merchant could
// fetch customers belonging to another merchant just by guessing IDs.
// Every customer query is always scoped to the requesting merchant.
func (r *CustomerRepository) FindByID(id, merchantID string) (*domain.Customer, error) {
	var customer domain.Customer
	result := r.db.Where("id = ? AND merchant_id = ?", id, merchantID).First(&customer)
	if result.Error != nil {
		return nil, result.Error
	}
	return &customer, nil
}

// FindByCode retrieves a customer by their unique code (CUS_xxxxxxxx).
// Scoped to merchant for the same isolation reason as FindByID.
func (r *CustomerRepository) FindByCode(code, merchantID string) (*domain.Customer, error) {
	var customer domain.Customer
	result := r.db.Where("code = ? AND merchant_id = ?", code, merchantID).First(&customer)
	if result.Error != nil {
		return nil, result.Error
	}
	return &customer, nil
}

// FindByEmail retrieves a customer by email under a specific merchant.
// Used to check for duplicate emails before creating a new customer
// and to look up existing customers during transaction initialize.
func (r *CustomerRepository) FindByEmail(email, merchantID string) (*domain.Customer, error) {
	var customer domain.Customer
	result := r.db.Where("email = ? AND merchant_id = ?", email, merchantID).First(&customer)
	if result.Error != nil {
		return nil, result.Error
	}
	return &customer, nil
}

// List returns a paginated list of customers for a merchant.
// offset and limit implement pagination, the service calculates
// offset as (page - 1) * perPage before calling this.
// We order by created_at DESC so the newest customers appear first.
func (r *CustomerRepository) List(merchantID string, offset, limit int) ([]domain.Customer, int64, error) {
	var customers []domain.Customer
	var total int64

	// Count total matching records first, needed for pagination metadata.
	// We run count and fetch as separate queries because GORM's combined
	// count+find can behave unexpectedly with complex scopes.
	r.db.Model(&domain.Customer{}).
		Where("merchant_id = ?", merchantID).
		Count(&total)

	result := r.db.Where("merchant_id = ?", merchantID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&customers)

	return customers, total, result.Error
}

// Update saves changes to a customer record scoped to a merchant.
func (r *CustomerRepository) Update(customer *domain.Customer) error {
	return r.db.Save(customer).Error
}

// EmailExists checks for duplicate email within a merchant's customer list.
func (r *CustomerRepository) EmailExists(email, merchantID string) (bool, error) {
	var count int64
	result := r.db.Model(&domain.Customer{}).
		Where("email = ? AND merchant_id = ?", email, merchantID).
		Count(&count)
	return count > 0, result.Error
}
