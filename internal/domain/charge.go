package domain

type MomoProvider string

const (
	ProviderMTN        MomoProvider = "mtn"
	ProviderVodafone   MomoProvider = "vodafone"
	ProviderAirtelTigo MomoProvider = "airteltigo"
)

// ChargeFlowStatus represents where a charge is in its multi-step flow.
// Mirrors Paystack's charge status values exactly so developers
// testing against Payfake learn the real Paystack flow at the same time.
type ChargeFlowStatus string

const (
	// Intermediate states, more input required from the customer.
	// The checkout page reads this and renders the appropriate form step.
	FlowSendPIN      ChargeFlowStatus = "send_pin"
	FlowSendOTP      ChargeFlowStatus = "send_otp"
	FlowSendBirthday ChargeFlowStatus = "send_birthday"
	FlowSendAddress  ChargeFlowStatus = "send_address"
	FlowOpenURL      ChargeFlowStatus = "open_url"

	// MoMo specific, prompt sent to phone, waiting for customer approval.
	FlowPayOffline ChargeFlowStatus = "pay_offline"

	// Terminal states, flow is complete.
	FlowSuccess ChargeFlowStatus = "success"
	FlowFailed  ChargeFlowStatus = "failed"
)

// CardType distinguishes local from international cards.
// Local Ghana cards go through PIN → OTP.
// International cards go through 3DS (open_url).
type CardType string

const (
	CardTypeLocal         CardType = "local"
	CardTypeInternational CardType = "international"
)

type Charge struct {
	Base
	MerchantID    string             `gorm:"type:varchar(36);not null;index" json:"merchant_id"`
	TransactionID string             `gorm:"type:varchar(36);not null;index" json:"transaction_id"`
	Channel       TransactionChannel `gorm:"type:varchar(20)" json:"channel"`
	Status        TransactionStatus  `gorm:"type:varchar(20);default:'pending'" json:"status"`

	// FlowStatus tracks where in the multi-step flow this charge is.
	// This is what the checkout page uses to decide which form to show.
	// It starts as send_pin for local cards, send_otp for MoMo,
	// send_birthday for bank, open_url for international cards.
	FlowStatus ChargeFlowStatus `gorm:"type:varchar(30);default:'send_pin'" json:"flow_status"`

	// OTPCode is the generated OTP for this charge.
	// Never returned in any API response, only visible in introspection logs.
	// The developer reads it from /control/logs during testing.
	// In a real system this would be sent via SMS.
	OTPCode string `gorm:"type:varchar(10)" json:"-"`

	// ThreeDSURL is the simulated 3DS verification URL for international cards.
	ThreeDSURL string `gorm:"type:varchar(500)" json:"three_ds_url,omitempty"`

	// Card fields
	CardNumber string   `gorm:"type:varchar(20)" json:"-"`
	CardExpiry string   `gorm:"type:varchar(10)" json:"-"`
	CardCVV    string   `gorm:"type:varchar(5)" json:"-"`
	CardBrand  string   `gorm:"type:varchar(20)" json:"card_brand"`
	CardLast4  string   `gorm:"type:varchar(4)" json:"card_last4"`
	CardType   CardType `gorm:"type:varchar(20)" json:"card_type"`

	// Mobile money fields
	MomoPhone    string       `gorm:"type:varchar(20)" json:"momo_phone"`
	MomoProvider MomoProvider `gorm:"type:varchar(20)" json:"momo_provider"`

	// Bank fields
	BankCode          string `gorm:"type:varchar(20)" json:"bank_code"`
	BankAccountNumber string `gorm:"type:varchar(20)" json:"-"`

	// Error codes for failed charges
	ChargeErrorCode string `gorm:"type:varchar(100)" json:"error_code,omitempty"`
}

// Error codes as constants, used by the simulator and force endpoint.
const (
	ChargeDoNotHonor         = "CHARGE_DO_NOT_HONOR"
	ChargeMomoTimeout        = "CHARGE_MOMO_TIMEOUT"
	ChargeBankTransferFailed = "CHARGE_BANK_TRANSFER_FAILED"
	ChargeFailed             = "CHARGE_FAILED"
)
