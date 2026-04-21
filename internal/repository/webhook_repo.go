package repository

import (
	"time"

	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
)

type WebhookRepository struct {
	db *gorm.DB
}

func NewWebhookRepository(db *gorm.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// CreateEvent inserts a new webhook event record.
// Called immediately after a transaction reaches a terminal state —
// we record the intent to deliver before attempting delivery.
// This way if the delivery goroutine crashes we still have a record
// of the event and can retry it from the control panel.
func (r *WebhookRepository) CreateEvent(event *domain.WebhookEvent) error {
	return r.db.Create(event).Error
}

// FindEventByID retrieves a webhook event by ID scoped to a merchant.
func (r *WebhookRepository) FindEventByID(id, merchantID string) (*domain.WebhookEvent, error) {
	var event domain.WebhookEvent
	result := r.db.Preload("AttemptLogs").
		Where("id = ? AND merchant_id = ?", id, merchantID).
		First(&event)
	if result.Error != nil {
		return nil, result.Error
	}
	return &event, nil
}

// ListEvents returns paginated webhook events for a merchant.
// Ordered by created_at DESC so the most recent events appear first.
func (r *WebhookRepository) ListEvents(merchantID string, offset, limit int) ([]domain.WebhookEvent, int64, error) {
	var events []domain.WebhookEvent
	var total int64

	r.db.Model(&domain.WebhookEvent{}).
		Where("merchant_id = ?", merchantID).
		Count(&total)

	result := r.db.Where("merchant_id = ?", merchantID).
		Preload("AttemptLogs").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&events)

	return events, total, result.Error
}

// CreateAttempt records a single webhook delivery attempt.
// Each attempt captures the HTTP status code we received from the
// merchant's webhook endpoint, the response body, and whether it
// was considered successful (2xx status code).
func (r *WebhookRepository) CreateAttempt(attempt *domain.WebhookAttempt) error {
	return r.db.Create(attempt).Error
}

// UpdateEventDelivery updates the delivery status of a webhook event
// after a delivery attempt. We update attempts count, last_attempt_at,
// and delivered flag in a single query to keep the record consistent.
func (r *WebhookRepository) UpdateEventDelivery(id string, delivered bool, attempts int) error {
	now := time.Now()
	return r.db.Model(&domain.WebhookEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"delivered":       delivered,
			"attempts":        attempts,
			"last_attempt_at": now,
		}).Error
}

// FindUndeliveredEvents retrieves all webhook events that have not
// been successfully delivered and have fewer than 3 attempts.
// This is used by the webhook retry worker, a background process
// that periodically picks up failed deliveries and retries them.
// We cap at 3 attempts to avoid hammering a consistently failing endpoint.
func (r *WebhookRepository) FindUndeliveredEvents() ([]domain.WebhookEvent, error) {
	var events []domain.WebhookEvent
	result := r.db.Where("delivered = ? AND attempts < ?", false, 3).
		Find(&events)
	return events, result.Error
}

// GetAttempts retrieves all delivery attempts for a webhook event.
// Ordered by attempted_at ASC so attempts are shown in chronological order.
func (r *WebhookRepository) GetAttempts(webhookEventID string) ([]domain.WebhookAttempt, error) {
	var attempts []domain.WebhookAttempt
	result := r.db.Where("webhook_event_id = ?", webhookEventID).
		Order("attempted_at ASC").
		Find(&attempts)
	return attempts, result.Error
}
