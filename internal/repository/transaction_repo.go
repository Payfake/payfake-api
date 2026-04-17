package repository

import (
	"github.com/GordenArcher/payfake/internal/domain"
	"gorm.io/gorm"
)

type TransactionRepository struct {
	db *gorm.DB
}

// DB returns the underlying gorm.DB instance.
// Used sparingly, only when a raw query is needed outside
// the repository's standard method set.
func (r *TransactionRepository) DB() *gorm.DB {
	return r.db
}

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create inserts a new transaction record.
// Called during initialize, at this point the transaction is in
// "pending" state. It transitions to success/failed after the charge.
func (r *TransactionRepository) Create(tx *domain.Transaction) error {
	return r.db.Create(tx).Error
}

// FindByID retrieves a transaction by primary ID scoped to a merchant.
func (r *TransactionRepository) FindByID(id, merchantID string) (*domain.Transaction, error) {
	var tx domain.Transaction
	// Preload Customer so the response includes the full customer object
	// without a second query in the service layer. GORM's Preload issues
	// a separate SELECT under the hood but it's clean and readable.
	result := r.db.Preload("Customer").
		Where("id = ? AND merchant_id = ?", id, merchantID).
		First(&tx)
	if result.Error != nil {
		return nil, result.Error
	}
	return &tx, nil
}

// FindByReference retrieves a transaction by its unique reference.
// This is the primary lookup for the verify endpoint, Paystack's
// verify flow uses the reference, not the internal ID.
// Scoped to merchant so references are isolated per merchant.
func (r *TransactionRepository) FindByReference(reference, merchantID string) (*domain.Transaction, error) {
	var tx domain.Transaction
	result := r.db.Preload("Customer").
		Where("reference = ? AND merchant_id = ?", reference, merchantID).
		First(&tx)
	if result.Error != nil {
		return nil, result.Error
	}
	return &tx, nil
}

// FindByReferenceOnly retrieves a transaction by reference without merchant scope.
// Used by public endpoints where merchant ID is not available (checkout page polling).
// Returns limited data, only what's needed for public display.
func (r *TransactionRepository) FindByReferenceOnly(reference string) (*domain.Transaction, error) {
	var tx domain.Transaction
	err := r.db.Preload("Customer").Preload("Merchant").
		Where("reference = ?", reference).
		First(&tx).Error
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

// FindByAccessCode retrieves a transaction by its access code.
// We no longer filter by status here, that's the service's job.
// The repository just fetches the record, the service decides what
// to do based on the current status.
func (r *TransactionRepository) FindByAccessCode(accessCode string) (*domain.Transaction, error) {
	var tx domain.Transaction
	result := r.db.Preload("Customer").Preload("Merchant").
		Where("access_code = ?", accessCode).
		First(&tx)
	if result.Error != nil {
		return nil, result.Error
	}
	return &tx, nil
}

// List returns a paginated list of transactions for a merchant.
// Supports optional status filter, if status is empty we return all.
func (r *TransactionRepository) List(merchantID string, status domain.TransactionStatus, offset, limit int) ([]domain.Transaction, int64, error) {
	var transactions []domain.Transaction
	var total int64

	query := r.db.Model(&domain.Transaction{}).Where("merchant_id = ?", merchantID)

	// Only apply the status filter if one was provided.
	// This lets the service call List with an empty status to get all
	// transactions, or a specific status to filter by it.
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)

	result := query.Preload("Customer").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&transactions)

	return transactions, total, result.Error
}

// UpdateStatus updates only the status and paid_at fields of a transaction.
// We use Select to update only these columns, never a full Save —
// because transactions are sensitive records. A full Save could
// accidentally overwrite fields if the struct was partially loaded.
func (r *TransactionRepository) UpdateStatus(id string, status domain.TransactionStatus, paidAt any) error {
	updates := map[string]any{"status": status}
	if paidAt != nil {
		updates["paid_at"] = paidAt
	}
	return r.db.Model(&domain.Transaction{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// ReferenceExists checks if a reference is already used by this merchant.
// References must be unique per merchant, this is how developers
// implement idempotency on their end. Same reference = same transaction.
func (r *TransactionRepository) ReferenceExists(reference, merchantID string) (bool, error) {
	var count int64
	result := r.db.Model(&domain.Transaction{}).
		Where("reference = ? AND merchant_id = ?", reference, merchantID).
		Count(&count)
	return count > 0, result.Error
}

// FindByCustomer returns all transactions for a specific customer
// under a merchant. Used by the customer transactions endpoint.
func (r *TransactionRepository) FindByCustomer(customerID, merchantID string, offset, limit int) ([]domain.Transaction, int64, error) {
	var transactions []domain.Transaction
	var total int64

	r.db.Model(&domain.Transaction{}).
		Where("customer_id = ? AND merchant_id = ?", customerID, merchantID).
		Count(&total)

	result := r.db.Where("customer_id = ? AND merchant_id = ?", customerID, merchantID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&transactions)

	return transactions, total, result.Error
}
