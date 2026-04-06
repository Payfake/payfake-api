package repository

import (
	"github.com/GordenArcher/payfake/internal/domain"
	"gorm.io/gorm"
)

type ChargeRepository struct {
	db *gorm.DB
}

func NewChargeRepository(db *gorm.DB) *ChargeRepository {
	return &ChargeRepository{db: db}
}

// Create inserts a new charge record linked to a transaction.
// Every charge is tied to a transaction, you can't charge without
// first initializing a transaction. The TransactionID is the foreign
// key that links them.
func (r *ChargeRepository) Create(charge *domain.Charge) error {
	return r.db.Create(charge).Error
}

// FindByTransactionID retrieves the charge for a given transaction.
// One transaction has at most one charge, if you need to retry
// a failed charge you initialize a new transaction.
func (r *ChargeRepository) FindByTransactionID(transactionID string) (*domain.Charge, error) {
	var charge domain.Charge
	result := r.db.Where("transaction_id = ?", transactionID).First(&charge)
	if result.Error != nil {
		return nil, result.Error
	}
	return &charge, nil
}

// FindByReference looks up a charge via its parent transaction reference.
// The charge endpoint GET /charge/:reference uses this, it receives
// a transaction reference and needs the charge details for that transaction.
func (r *ChargeRepository) FindByReference(reference, merchantID string) (*domain.Charge, error) {
	var charge domain.Charge

	// Join through the transactions table to find the charge by reference.
	// This is a JOIN query, we can't get there from the charges table alone
	// since reference lives on the transaction, not the charge.
	result := r.db.Joins("JOIN transactions ON transactions.id = charges.transaction_id").
		Where("transactions.reference = ? AND charges.merchant_id = ?", reference, merchantID).
		First(&charge)

	if result.Error != nil {
		return nil, result.Error
	}
	return &charge, nil
}

// UpdateStatus updates the status of a charge after the simulator
// has resolved the outcome.
func (r *ChargeRepository) UpdateStatus(id string, status domain.TransactionStatus) error {
	return r.db.Model(&domain.Charge{}).
		Where("id = ?", id).
		Update("status", status).Error
}
