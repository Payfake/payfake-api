package service

import (
	"fmt"

	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/repository"
)

type MerchantService struct {
	merchantRepo *repository.MerchantRepository
}

func NewMerchantService(merchantRepo *repository.MerchantRepository) *MerchantService {
	return &MerchantService{merchantRepo: merchantRepo}
}

func (s *MerchantService) GetProfile(merchantID string) (*domain.Merchant, error) {
	merchant, err := s.merchantRepo.FindByID(merchantID)
	if err != nil {
		return nil, ErrMerchantNotFound
	}
	return merchant, nil
}

// UpdateProfile updates the merchant's business name and webhook URL.
// Empty strings are ignored, we only update fields the merchant actually sent.
func (s *MerchantService) UpdateProfile(merchantID, businessName, webhookURL string) (*domain.Merchant, error) {
	merchant, err := s.merchantRepo.FindByID(merchantID)
	if err != nil {
		return nil, ErrMerchantNotFound
	}

	if businessName != "" {
		merchant.BusinessName = businessName
	}
	// WebhookURL can be set to empty string to clear it —
	// unlike businessName where empty means "don't change".
	merchant.WebhookURL = webhookURL

	if err := s.merchantRepo.UpdateProfile(merchantID, merchant.BusinessName, merchant.WebhookURL); err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	return merchant, nil
}
