package domain

import "time"

// RefreshSession is the server-side half of refresh-token rotation.
// A signed JWT proves what token was issued, while this record proves that the
// token has not already been used or revoked. Stateless JWT validation alone
// cannot provide one-time rotation because an old token remains cryptographically
// valid until its expiry date.
type RefreshSession struct {
	Base
	MerchantID string     `gorm:"type:varchar(36);not null;index" json:"merchant_id"`
	TokenID    string     `gorm:"type:varchar(36);not null;uniqueIndex" json:"-"`
	ExpiresAt  time.Time  `gorm:"not null;index" json:"expires_at"`
	UsedAt     *time.Time `json:"used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}
