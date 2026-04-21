package repository

import (
	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
)

type LogRepository struct {
	db *gorm.DB
}

func NewLogRepository(db *gorm.DB) *LogRepository {
	return &LogRepository{db: db}
}

// Create inserts a request/response log entry.
// Called by the logger middleware after every request completes.
// These logs power the /control/logs introspection endpoint —
// developers can see every request their app made and the exact
// response Payfake returned, which makes debugging integrations easy.
func (r *LogRepository) Create(entry *domain.RequestLog) error {
	return r.db.Create(entry).Error
}

// List returns paginated log entries scoped to a merchant.
// Ordered by logged_at DESC, most recent requests first.
func (r *LogRepository) List(merchantID string, offset, limit int) ([]domain.RequestLog, int64, error) {
	var logs []domain.RequestLog
	var total int64

	r.db.Model(&domain.RequestLog{}).
		Where("merchant_id = ?", merchantID).
		Count(&total)

	result := r.db.Where("merchant_id = ?", merchantID).
		Order("logged_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&logs)

	return logs, total, result.Error
}

// ClearByMerchant deletes all log entries for a merchant.
// Hard delete, we use Unscoped() to bypass GORM's soft delete
// because logs don't need to be recoverable. Clearing logs is
// a deliberate action and should be permanent.
func (r *LogRepository) ClearByMerchant(merchantID string) error {
	return r.db.Unscoped().
		Where("merchant_id = ?", merchantID).
		Delete(&domain.RequestLog{}).Error
}
