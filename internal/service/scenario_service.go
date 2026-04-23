package service

import (
	"errors"
	"fmt"

	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/repository"
	"github.com/payfake/payfake-api/pkg/uid"
	"gorm.io/gorm"
)

type scenarioRepository interface {
	FindByMerchantID(string) (*domain.ScenarioConfig, error)
	Create(*domain.ScenarioConfig) error
	Update(*domain.ScenarioConfig) error
}

type ScenarioService struct {
	scenarioRepo scenarioRepository
}

func NewScenarioService(scenarioRepo *repository.ScenarioRepository) *ScenarioService {
	return &ScenarioService{scenarioRepo: scenarioRepo}
}

type UpdateScenarioInput struct {
	FailureRate *float64
	DelayMS     *int
	ForceStatus *string
	ErrorCode   *string
}

// Get retrieves the scenario config for a merchant.
// If no config exists yet we create a default one on the fly,
// merchants should always have a scenario config to update.
func (s *ScenarioService) Get(merchantID string) (*domain.ScenarioConfig, error) {
	scenario, err := s.scenarioRepo.FindByMerchantID(merchantID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to fetch scenario: %w", err)
		}

		// No config exists yet, create the default one.
		scenario = &domain.ScenarioConfig{
			Base:        domain.Base{ID: uid.NewScenarioID()},
			MerchantID:  merchantID,
			FailureRate: 0,
			DelayMS:     0,
			ForceStatus: "",
			ErrorCode:   "",
			IsActive:    true,
		}
		if err := s.scenarioRepo.Create(scenario); err != nil {
			return nil, fmt.Errorf("failed to create default scenario: %w", err)
		}
	}
	return scenario, nil
}

// Update applies partial updates to a merchant's scenario config.
// We use pointers for every field in UpdateScenarioInput so the caller
// can send only the fields they want to change, nil means "don't touch".
// This is the Go equivalent of a PATCH request at the service level.
func (s *ScenarioService) Update(merchantID string, input UpdateScenarioInput) (*domain.ScenarioConfig, error) {
	scenario, err := s.Get(merchantID)
	if err != nil {
		return nil, err
	}

	// Apply only the fields that were provided (non-nil).
	if input.FailureRate != nil {
		// Failure rate must be between 0.0 and 1.0.
		// 0.0 = always succeed, 1.0 = always fail.
		if *input.FailureRate < 0 || *input.FailureRate > 1 {
			return nil, ErrInvalidScenarioConfig
		}
		scenario.FailureRate = *input.FailureRate
	}

	if input.DelayMS != nil {
		// Cap delay at 30 seconds, beyond that the HTTP client
		// will have timed out anyway and the delay serves no purpose.
		if *input.DelayMS < 0 || *input.DelayMS > 30000 {
			return nil, ErrInvalidScenarioConfig
		}
		scenario.DelayMS = *input.DelayMS
	}

	if input.ForceStatus != nil {
		// Only allow valid terminal statuses to be forced.
		// You can't force "pending", that's the initial state,
		// not an outcome.
		validStatuses := map[string]bool{
			string(domain.TransactionSuccess):   true,
			string(domain.TransactionFailed):    true,
			string(domain.TransactionAbandoned): true,
			"":                                  true, // empty string clears the force
		}
		if !validStatuses[*input.ForceStatus] {
			return nil, ErrInvalidForceStatus
		}
		scenario.ForceStatus = *input.ForceStatus
	}

	if input.ErrorCode != nil {
		scenario.ErrorCode = *input.ErrorCode
	}

	if err := s.scenarioRepo.Update(scenario); err != nil {
		return nil, fmt.Errorf("failed to update scenario: %w", err)
	}

	return scenario, nil
}

// Reset clears all scenario config back to defaults.
// After reset all transactions will succeed with no delay,
// clean slate for the next test run.
func (s *ScenarioService) Reset(merchantID string) (*domain.ScenarioConfig, error) {
	scenario, err := s.Get(merchantID)
	if err != nil {
		return nil, err
	}

	scenario.FailureRate = 0
	scenario.DelayMS = 0
	scenario.ForceStatus = ""
	scenario.ErrorCode = ""

	if err := s.scenarioRepo.Update(scenario); err != nil {
		return nil, fmt.Errorf("failed to reset scenario: %w", err)
	}

	return scenario, nil
}
