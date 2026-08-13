package repository

import (
	"errors"
	"time"

	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errFinalizeConflict = errors.New("charge terminal transition lost a concurrent race")

type ChargeRepository struct {
	db *gorm.DB
}

func NewChargeRepository(db *gorm.DB) *ChargeRepository {
	return &ChargeRepository{db: db}
}

// CreateOnce relies on the unique transaction_id index to make charge
// initiation idempotent under concurrency. A read-before-create check cannot
// prevent two requests that race between those operations; ON CONFLICT lets
// PostgreSQL choose exactly one winner without returning a server error.
func (r *ChargeRepository) CreateOnce(charge *domain.Charge, otpLog *domain.OTPLog) (bool, error) {
	var created bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "transaction_id"}},
			TargetWhere: clause.Where{Exprs: []clause.Expression{
				clause.Eq{Column: "status", Value: domain.TransactionPending},
				clause.Eq{Column: "deleted_at", Value: nil},
			}},
			DoNothing: true,
		}).Create(charge)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errFinalizeConflict
		}
		if otpLog != nil {
			if err := tx.Create(otpLog).Error; err != nil {
				return err
			}
		}
		created = true
		return nil
	})
	if errors.Is(err, errFinalizeConflict) {
		return false, nil
	}
	return created, err
}

// AdvanceFlow performs an intermediate state transition with optional OTP
// creation/consumption. Keeping these writes in one transaction prevents a
// charge from moving to send_otp without a corresponding OTP record, and the
// expected state/OTP predicates ensure concurrent form submissions have one
// winner.
func (r *ChargeRepository) AdvanceFlow(
	chargeID string,
	expectedFlow, nextFlow domain.ChargeFlowStatus,
	expectedOTP, nextOTP string,
	newOTPLog *domain.OTPLog,
	consumeOTPLogID, merchantID string,
) (bool, error) {
	var advanced bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&domain.Charge{}).
			Where("id = ? AND status = ? AND flow_status = ?", chargeID, domain.TransactionPending, expectedFlow)
		if expectedOTP != "" {
			query = query.Where("otp_code = ?", expectedOTP)
		}
		result := query.Updates(map[string]any{"flow_status": nextFlow, "otp_code": nextOTP})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errFinalizeConflict
		}

		if consumeOTPLogID != "" {
			used := tx.Model(&domain.OTPLog{}).
				Where("id = ? AND merchant_id = ? AND used = ?", consumeOTPLogID, merchantID, false).
				Update("used", true)
			if used.Error != nil {
				return used.Error
			}
			if used.RowsAffected != 1 {
				return errFinalizeConflict
			}
		}
		if newOTPLog != nil {
			if err := tx.Create(newOTPLog).Error; err != nil {
				return err
			}
		}
		advanced = true
		return nil
	})
	if errors.Is(err, errFinalizeConflict) {
		return false, nil
	}
	return advanced, err
}

// Finalize atomically moves both sides of a payment into a terminal state.
// The flow/status predicates are the concurrency guard: if another request has
// already completed this charge, no row matches and the whole transaction is
// rolled back. The caller can therefore dispatch one webhook only for the
// request that actually won the state transition.
func (r *ChargeRepository) Finalize(
	chargeID, transactionID string,
	expectedFlow domain.ChargeFlowStatus,
	transactionStatus, chargeStatus domain.TransactionStatus,
	flowStatus domain.ChargeFlowStatus,
	errorCode string,
	paidAt *time.Time,
	otpLogID, merchantID string,
) (bool, error) {
	var finalized bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		chargeResult := tx.Model(&domain.Charge{}).
			Where("id = ? AND status = ? AND flow_status = ?", chargeID, domain.TransactionPending, expectedFlow).
			Updates(map[string]any{
				"status":            chargeStatus,
				"flow_status":       flowStatus,
				"charge_error_code": errorCode,
				"otp_code":          "",
			})
		if chargeResult.Error != nil {
			return chargeResult.Error
		}
		if chargeResult.RowsAffected != 1 {
			return errFinalizeConflict
		}

		txUpdates := map[string]any{"status": transactionStatus}
		if paidAt != nil {
			txUpdates["paid_at"] = paidAt
		}
		txResult := tx.Model(&domain.Transaction{}).
			Where("id = ? AND status = ?", transactionID, domain.TransactionPending).
			Updates(txUpdates)
		if txResult.Error != nil {
			return txResult.Error
		}
		if txResult.RowsAffected != 1 {
			return errFinalizeConflict
		}
		if otpLogID != "" {
			used := tx.Model(&domain.OTPLog{}).
				Where("id = ? AND merchant_id = ? AND used = ?", otpLogID, merchantID, false).
				Update("used", true)
			if used.Error != nil {
				return used.Error
			}
			if used.RowsAffected != 1 {
				return errFinalizeConflict
			}
		}

		finalized = true
		return nil
	})
	if errors.Is(err, errFinalizeConflict) {
		return false, nil
	}
	return finalized, err
}

// FindByTransactionID retrieves the most recent charge for a transaction.
// We order by created_at DESC so if a transaction has multiple charge
// attempts (retry after failure) we always get the latest one, which
// is the one the checkout page is currently working with.
func (r *ChargeRepository) FindByTransactionID(transactionID string) (*domain.Charge, error) {
	var charge domain.Charge
	result := r.db.
		Where("transaction_id = ?", transactionID).
		Order("created_at DESC").
		First(&charge)
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

// FindByTransactionReference finds a charge by its parent transaction reference.
// Used by submit endpoints which receive the transaction reference from the
// checkout page, not the internal charge ID.
func (r *ChargeRepository) FindByTransactionReference(reference, merchantID string) (*domain.Charge, error) {
	var charge domain.Charge
	result := r.db.Joins("JOIN transactions ON transactions.id = charges.transaction_id").
		Where("transactions.reference = ? AND charges.merchant_id = ?", reference, merchantID).
		First(&charge)
	if result.Error != nil {
		return nil, result.Error
	}
	return &charge, nil
}
