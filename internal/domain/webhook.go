package domain

import "time"

type WebhookEventType string

const (
	EventChargeSuccess   WebhookEventType = "charge.success"
	EventChargeFailed    WebhookEventType = "charge.failed"
	EventTransferSuccess WebhookEventType = "transfer.success"
	EventTransferFailed  WebhookEventType = "transfer.failed"
	EventRefundProcessed WebhookEventType = "refund.processed"
)

type WebhookEndpoint struct {
	Base
	MerchantID string `gorm:"type:varchar(36);not null;index" json:"merchant_id"`
	URL        string `gorm:"type:varchar(500);not null" json:"url"`
	IsActive   bool   `gorm:"default:true" json:"is_active"`
}

type WebhookEvent struct {
	Base
	MerchantID    string           `gorm:"type:varchar(36);not null;index" json:"merchant_id"`
	TransactionID string           `gorm:"type:varchar(36);not null;index" json:"transaction_id"`
	Event         WebhookEventType `gorm:"type:varchar(50)" json:"event"`
	Payload       JSON             `gorm:"type:jsonb" json:"payload"`
	Delivered     bool             `gorm:"default:false" json:"delivered"`
	Attempts      int              `gorm:"default:0" json:"attempts"`
	LastAttemptAt *time.Time       `json:"last_attempt_at"`

	AttemptLogs []WebhookAttempt `gorm:"foreignKey:WebhookEventID" json:"-"`
}

type WebhookAttempt struct {
	Base
	WebhookEventID string    `gorm:"type:varchar(36);not null;index" json:"webhook_event_id"`
	StatusCode     int       `json:"status_code"`
	ResponseBody   string    `gorm:"type:text" json:"response_body"`
	AttemptedAt    time.Time `json:"attempted_at"`
	Succeeded      bool      `gorm:"default:false" json:"succeeded"`
}
