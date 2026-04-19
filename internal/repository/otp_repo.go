package repository

import (
	"time"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/pkg/uid"
	"gorm.io/gorm"
)

type OTPRepository struct {
	db *gorm.DB
}

func NewOTPRepository(db *gorm.DB) *OTPRepository {
	return &OTPRepository{db: db}
}

// Create stores a newly generated OTP.
// OTPs expire after 10 minutes, same window as real Paystack OTPs.
// The used flag is set to true when the OTP is successfully submitted
// so we can distinguish used vs expired vs active OTPs in the logs.
func (r *OTPRepository) Create(merchantID, reference, channel, step, otpCode string) error {
	log := &domain.OTPLog{
		Base:       domain.Base{ID: uid.NewRequestLogID()},
		MerchantID: merchantID,
		Reference:  reference,
		Channel:    channel,
		OTPCode:    otpCode,
		Step:       step,
		Used:       false,
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}
	return r.db.Create(log).Error
}

// MarkUsed marks an OTP as used after successful submission.
func (r *OTPRepository) MarkUsed(reference string) error {
	return r.db.Model(&domain.OTPLog{}).
		Where("reference = ? AND used = ?", reference, false).
		Update("used", true).Error
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

	r.db.Model(&domain.OTPLog{}).
		Where("merchant_id = ?", merchantID).
		Count(&total)

	result := r.db.Where("merchant_id = ?", merchantID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&logs)

	return logs, total, result.Error
}
