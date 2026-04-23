package domain

import (
	"time"
)

type TransactionStatus string
type TransactionChannel string
type Currency string

const (
	TransactionPending   TransactionStatus = "pending"
	TransactionSuccess   TransactionStatus = "success"
	TransactionFailed    TransactionStatus = "failed"
	TransactionAbandoned TransactionStatus = "abandoned"
	TransactionReversed  TransactionStatus = "reversed"
)

const (
	ChannelCard         TransactionChannel = "card"
	ChannelMobileMoney  TransactionChannel = "mobile_money"
	ChannelBankTransfer TransactionChannel = "bank_transfer"
)

const (
	CurrencyGHS Currency = "GHS"
	CurrencyNGN Currency = "NGN"
	CurrencyKES Currency = "KES"
	CurrencyUSD Currency = "USD"
)

type Transaction struct {
	Base
	MerchantID  string             `gorm:"type:varchar(36);not null;index;index:idx_transactions_merchant_reference,unique" json:"merchant_id"`
	CustomerID  string             `gorm:"type:varchar(36);index" json:"customer_id"`
	Reference   string             `gorm:"type:varchar(100);not null;index;index:idx_transactions_merchant_reference,unique" json:"reference"`
	Amount      int64              `gorm:"not null" json:"amount"`
	Currency    Currency           `gorm:"type:varchar(10);default:'GHS'" json:"currency"`
	Status      TransactionStatus  `gorm:"type:varchar(20);default:'pending'" json:"status"`
	Channel     TransactionChannel `gorm:"type:varchar(20)" json:"channel"`
	Fees        int64              `gorm:"default:0" json:"fees"`
	AccessCode  string             `gorm:"type:varchar(100)" json:"access_code"`
	CallbackURL string             `gorm:"type:varchar(500)" json:"callback_url"`
	PaidAt      *time.Time         `json:"paid_at"`
	Metadata    JSON               `gorm:"type:jsonb" json:"metadata"`

	// Associations, always at the bottom, after all column fields.
	// GORM resolves foreign keys top-down so the FK column must exist
	// before the association that references it.
	Merchant Merchant       `gorm:"foreignKey:MerchantID" json:"-"`
	Customer Customer       `gorm:"foreignKey:CustomerID" json:"customer"`
	Charge   *Charge        `gorm:"foreignKey:TransactionID" json:"-"`
	Webhooks []WebhookEvent `gorm:"foreignKey:TransactionID" json:"-"`
}
