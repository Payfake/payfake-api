package domain

type MomoProvider string

const (
	ProviderMTN        MomoProvider = "mtn"
	ProviderVodafone   MomoProvider = "vodafone"
	ProviderAirtelTigo MomoProvider = "airteltigo"
)

// These are the error code constants the simulator uses to populate
// SimulationResult.ErrorCode. Using domain constants instead of raw
// strings means if we rename a code we catch every reference at compile time.
const (
	ChargeDoNotHonor         = "CHARGE_DO_NOT_HONOR"
	ChargeMomoTimeout        = "CHARGE_MOMO_TIMEOUT"
	ChargeBankTransferFailed = "CHARGE_BANK_TRANSFER_FAILED"
	ChargeFailed             = "CHARGE_FAILED"
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
