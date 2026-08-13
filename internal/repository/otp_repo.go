package repository

import (
	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
)

type OTPRepository struct {
	db *gorm.DB
}

func NewOTPRepository(db *gorm.DB) *OTPRepository {
	return &OTPRepository{db: db}
}

// FindByReference returns all OTP logs for a transaction reference.
// Ordered by created_at DESC so the most recent OTP appears first.
func (r *OTPRepository) FindByReference(reference, merchantID string) ([]domain.OTPLog, error) {
	var logs []domain.OTPLog
	result := r.db.Where("reference = ? AND merchant_id = ?", reference, merchantID).
		Order("created_at DESC").
		Find(&logs)
	return logs, result.Error
}

// ListByMerchant returns paginated OTP logs for a merchant.
func (r *OTPRepository) ListByMerchant(merchantID string, offset, limit int) ([]domain.OTPLog, int64, error) {
	var logs []domain.OTPLog
	var total int64

	if err := r.db.Model(&domain.OTPLog{}).
		Where("merchant_id = ?", merchantID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	result := r.db.Where("merchant_id = ?", merchantID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&logs)

	return logs, total, result.Error
}
