package domain

type Customer struct {
	Base
	MerchantID string `gorm:"type:varchar(36);not null;index" json:"merchant_id"`
	Email      string `gorm:"type:varchar(255);not null" json:"email"`
	FirstName  string `gorm:"type:varchar(100)" json:"first_name"`
	LastName   string `gorm:"type:varchar(100)" json:"last_name"`
	Phone      string `gorm:"type:varchar(20)" json:"phone"`
	Code       string `gorm:"type:varchar(100);uniqueIndex" json:"customer_code"`
	Metadata   JSON   `gorm:"type:jsonb" json:"metadata"`

	Transactions []Transaction `gorm:"foreignKey:CustomerID" json:"-"`
}
