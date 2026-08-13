package repository

import (
	"time"

	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
)

// RefreshSessionRepository persists one-time refresh-token state.
// The conditional updates below are deliberately performed by PostgreSQL,
// rather than by a read followed by a write in Go, so two simultaneous refresh
// requests cannot both consume the same token successfully.
type RefreshSessionRepository struct {
	db *gorm.DB
}

func NewRefreshSessionRepository(db *gorm.DB) *RefreshSessionRepository {
	return &RefreshSessionRepository{db: db}
}

func (r *RefreshSessionRepository) Create(session *domain.RefreshSession) error {
	return r.db.Create(session).Error
}

// Rotate consumes oldTokenID and creates its replacement in one transaction.
// If the old token was already used, revoked, or expired, RowsAffected is zero
// and the replacement is never inserted. This is the replay protection that a
// signed refresh JWT cannot provide on its own.
func (r *RefreshSessionRepository) Rotate(oldTokenID string, replacement *domain.RefreshSession) (bool, error) {
	var rotated bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		result := tx.Model(&domain.RefreshSession{}).
			Where("token_id = ? AND used_at IS NULL AND revoked_at IS NULL AND expires_at > ?", oldTokenID, now).
			Update("used_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}

		if err := tx.Create(replacement).Error; err != nil {
			return err
		}
		rotated = true
		return nil
	})
	return rotated, err
}

func (r *RefreshSessionRepository) Revoke(tokenID string) error {
	now := time.Now()
	return r.db.Model(&domain.RefreshSession{}).
		Where("token_id = ? AND revoked_at IS NULL", tokenID).
		Update("revoked_at", now).Error
}

// RevokeAll closes every refresh session after a password change. Access JWTs
// remain short lived, but no existing browser can silently extend its session
// using credentials issued before the password was changed.
func (r *RefreshSessionRepository) RevokeAll(merchantID string) error {
	now := time.Now()
	return r.db.Model(&domain.RefreshSession{}).
		Where("merchant_id = ? AND revoked_at IS NULL", merchantID).
		Update("revoked_at", now).Error
}
