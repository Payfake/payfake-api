package service

import (
	"fmt"

	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/repository"
	"github.com/payfake/payfake-api/pkg/webhookurl"
)

type MerchantService struct {
	merchantRepo            *repository.MerchantRepository
	allowPrivateWebhookURLs bool
}

func NewMerchantService(merchantRepo *repository.MerchantRepository, allowPrivateWebhookURLs bool) *MerchantService {
	return &MerchantService{merchantRepo: merchantRepo, allowPrivateWebhookURLs: allowPrivateWebhookURLs}
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
func (s *MerchantService) UpdateProfile(merchantID string, businessName, webhookURL *string) (*domain.Merchant, error) {
	merchant, err := s.merchantRepo.FindByID(merchantID)
	if err != nil {
		return nil, ErrMerchantNotFound
	}

	if businessName != nil && *businessName != "" {
		merchant.BusinessName = *businessName
	}
	// A nil pointer means the JSON field was omitted and must be preserved.
	// An explicit empty string clears the URL, which keeps PATCH-like behavior
	// without accidentally erasing it during a business-name-only update.
	if webhookURL != nil && *webhookURL != "" {
		if err := webhookurl.Validate(*webhookURL, s.allowPrivateWebhookURLs); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidWebhookURL, err)
		}
	}
	if webhookURL != nil {
		merchant.WebhookURL = *webhookURL
	}

	if err := s.merchantRepo.UpdateProfile(merchantID, merchant.BusinessName, merchant.WebhookURL); err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	return merchant, nil
}
