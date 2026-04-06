package domain

import "time"

type RequestLog struct {
	Base
	MerchantID   string    `gorm:"type:varchar(36);index" json:"merchant_id"`
	Method       string    `gorm:"type:varchar(10)" json:"method"`
	Path         string    `gorm:"type:varchar(500)" json:"path"`
	StatusCode   int       `json:"status_code"`
	RequestBody  string    `gorm:"type:text" json:"request_body"`
	ResponseBody string    `gorm:"type:text" json:"response_body"`
	IPAddress    string    `gorm:"type:varchar(50)" json:"ip_address"`
	Duration     int64     `json:"duration_ms"`
	RequestID    string    `gorm:"type:varchar(100)" json:"request_id"`
	LoggedAt     time.Time `json:"logged_at"`
}
