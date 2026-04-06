package domain

type ScenarioConfig struct {
	Base
	MerchantID  string  `gorm:"type:varchar(36);uniqueIndex;not null" json:"merchant_id"`
	FailureRate float64 `gorm:"type:decimal(5,2);default:0" json:"failure_rate"`
	DelayMS     int     `gorm:"default:0" json:"delay_ms"`
	ForceStatus string  `gorm:"type:varchar(20)" json:"force_status"`
	ErrorCode   string  `gorm:"type:varchar(100)" json:"error_code"`
	IsActive    bool    `gorm:"default:true" json:"is_active"`
}
