package domain

type MomoProvider string

const (
	ProviderMTN        MomoProvider = "mtn"
	ProviderVodafone   MomoProvider = "vodafone"
	ProviderAirtelTigo MomoProvider = "airteltigo"
)

type Charge struct {
	Base
	MerchantID    string             `gorm:"type:varchar(36);not null;index" json:"merchant_id"`
	TransactionID string             `gorm:"type:varchar(36);not null;index" json:"transaction_id"`
	Channel       TransactionChannel `gorm:"type:varchar(20)" json:"channel"`
	Status        TransactionStatus  `gorm:"type:varchar(20);default:'pending'" json:"status"`

	// Card fields
	CardNumber string `gorm:"type:varchar(20)" json:"-"`
	CardExpiry string `gorm:"type:varchar(10)" json:"-"`
	CardCVV    string `gorm:"type:varchar(5)" json:"-"`
	CardBrand  string `gorm:"type:varchar(20)" json:"card_brand"`
	CardLast4  string `gorm:"type:varchar(4)" json:"card_last4"`

	// Mobile money fields
	MomoPhone    string       `gorm:"type:varchar(20)" json:"momo_phone"`
	MomoProvider MomoProvider `gorm:"type:varchar(20)" json:"momo_provider"`

	// Bank fields
	BankCode          string `gorm:"type:varchar(20)" json:"bank_code"`
	BankAccountNumber string `gorm:"type:varchar(20)" json:"-"`
}
