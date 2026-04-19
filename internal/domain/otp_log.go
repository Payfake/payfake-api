package domain

import "time"

// OTPLog stores every OTP generated during a charge flow.
// OTPs are never returned in API responses, developers read them
// here during testing instead of needing a real phone.
// In production this table would be empty since real OTPs go via SMS.
type OTPLog struct {
	Base
	MerchantID string    `gorm:"type:varchar(36);not null;index" json:"merchant_id"`
	Reference  string    `gorm:"type:varchar(100);not null;index" json:"reference"`
	Channel    string    `gorm:"type:varchar(20)" json:"channel"`
	OTPCode    string    `gorm:"type:varchar(10)" json:"otp_code"`
	Step       string    `gorm:"type:varchar(30)" json:"step"`
	Used       bool      `gorm:"default:false" json:"used"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
}
