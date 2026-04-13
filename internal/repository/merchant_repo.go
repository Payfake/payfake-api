package repository

import (
	"github.com/GordenArcher/payfake/internal/domain"
	"gorm.io/gorm"
)

// MerchantRepository handles all database operations for merchants.
// Every method takes exactly what it needs and returns exactly what
// the service layer needs, no leaking of GORM internals upward.
type MerchantRepository struct {
	db *gorm.DB
}

func NewMerchantRepository(db *gorm.DB) *MerchantRepository {
	return &MerchantRepository{db: db}
}

// Create inserts a new merchant record into the database.
// The merchant struct is passed by pointer so GORM can populate
// the ID and timestamps after the insert.
func (r *MerchantRepository) Create(merchant *domain.Merchant) error {
	return r.db.Create(merchant).Error
}

// FindByID retrieves a merchant by their primary ID.
// Returns gorm.ErrRecordNotFound if no merchant exists with that ID.
// The service layer checks for this specific error to return the
// correct response code, MerchantNotFound vs InternalError.
func (r *MerchantRepository) FindByID(id string) (*domain.Merchant, error) {
	var merchant domain.Merchant
	result := r.db.Where("id = ?", id).First(&merchant)
	if result.Error != nil {
		return nil, result.Error
	}
	return &merchant, nil
}

// FindByEmail retrieves a merchant by email.
// Used during login to look up the merchant before verifying password,
// and during registration to check if the email is already taken.
func (r *MerchantRepository) FindByEmail(email string) (*domain.Merchant, error) {
	var merchant domain.Merchant
	result := r.db.Where("email = ?", email).First(&merchant)
	if result.Error != nil {
		return nil, result.Error
	}
	return &merchant, nil
}

// FindBySecretKey retrieves a merchant by their secret key.
// This is called by the RequireSecretKey middleware on every
// authenticated request, it must be fast. The secret_key column
// has a unique index on it so this is always an index scan, never
// a full table scan regardless of how many merchants exist.
func (r *MerchantRepository) FindBySecretKey(secretKey string) (*domain.Merchant, error) {
	var merchant domain.Merchant
	result := r.db.Where("secret_key = ? AND is_active = ?", secretKey, true).First(&merchant)
	if result.Error != nil {
		return nil, result.Error
	}
	return &merchant, nil
}

// FindByPublicKey retrieves a merchant by their public key.
// Used when the frontend sends the public key to initialize a
// transaction, we need to know which merchant owns that key.
func (r *MerchantRepository) FindByPublicKey(publicKey string) (*domain.Merchant, error) {
	var merchant domain.Merchant
	result := r.db.Where("public_key = ? AND is_active = ?", publicKey, true).First(&merchant)
	if result.Error != nil {
		return nil, result.Error
	}
	return &merchant, nil
}

// Update saves changes to an existing merchant record.
// We use Save here which updates ALL fields. For partial updates
// (e.g. only updating webhook_url) the service should set only
// the fields it wants to change before calling this.
func (r *MerchantRepository) Update(merchant *domain.Merchant) error {
	return r.db.Save(merchant).Error
}

// UpdateKeys updates only the public and secret key columns.
// Called during key regeneration, we use Select to update only
// these two columns instead of the entire record. This is safer
// than a full Save because it can't accidentally wipe other fields
// if the merchant struct is partially populated.
func (r *MerchantRepository) UpdateKeys(id, publicKey, secretKey string) error {
	return r.db.Model(&domain.Merchant{}).
		Where("id = ?", id).
		Select("public_key", "secret_key").
		Updates(map[string]any{
			"public_key": publicKey,
			"secret_key": secretKey,
		}).Error
}

// EmailExists checks whether a merchant with the given email already
// exists. Used during registration before creating a new merchant.
// We use Count instead of First because we don't need the record —
// just the yes/no answer. Count is cheaper than fetching a full row.
func (r *MerchantRepository) EmailExists(email string) (bool, error) {
	var count int64
	result := r.db.Model(&domain.Merchant{}).
		Where("email = ?", email).
		Count(&count)
	return count > 0, result.Error
}

// UpdatePassword updates only the password column.
// Uses Select to ensure only password is touched, never a full Save
// which could accidentally wipe other fields.
func (r *MerchantRepository) UpdatePassword(id, hashedPassword string) error {
	return r.db.Model(&domain.Merchant{}).
		Where("id = ?", id).
		Update("password", hashedPassword).Error
}

// UpdateProfile updates merchant business name and webhook URL.
func (r *MerchantRepository) UpdateProfile(id, businessName, webhookURL string) error {
	return r.db.Model(&domain.Merchant{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"business_name": businessName,
			"webhook_url":   webhookURL,
		}).Error
}
