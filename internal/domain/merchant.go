package domain

type Merchant struct {
	Base
	BusinessName string `gorm:"type:varchar(255);not null" json:"business_name"`
	Email        string `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	Password     string `gorm:"type:varchar(255);not null" json:"-"`
	PublicKey    string `gorm:"type:varchar(100);uniqueIndex;not null" json:"public_key"`
	SecretKey    string `gorm:"type:varchar(100);uniqueIndex;not null" json:"-"`
	WebhookURL   string `gorm:"type:varchar(500)" json:"webhook_url"`
	IsActive     bool   `gorm:"default:true" json:"is_active"`

	Customers    []Customer      `gorm:"foreignKey:MerchantID" json:"-"`
	Transactions []Transaction   `gorm:"foreignKey:MerchantID" json:"-"`
	Scenario     *ScenarioConfig `gorm:"foreignKey:MerchantID" json:"-"`
}
