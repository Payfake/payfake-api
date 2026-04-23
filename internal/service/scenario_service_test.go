package service

import (
	"errors"
	"testing"

	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
)

type fakeScenarioRepo struct {
	findResult   *domain.ScenarioConfig
	findErr      error
	createCalled bool
	updateCalled bool
}

func (f *fakeScenarioRepo) FindByMerchantID(string) (*domain.ScenarioConfig, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.findResult, nil
}

func (f *fakeScenarioRepo) Create(scenario *domain.ScenarioConfig) error {
	f.createCalled = true
	f.findResult = scenario
	return nil
}

func (f *fakeScenarioRepo) Update(*domain.ScenarioConfig) error {
	f.updateCalled = true
	return nil
}

func TestScenarioServiceGetCreatesDefaultOnlyWhenMissing(t *testing.T) {
	repo := &fakeScenarioRepo{findErr: gorm.ErrRecordNotFound}
	svc := &ScenarioService{scenarioRepo: repo}

	scenario, err := svc.Get("MRC_123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !repo.createCalled {
		t.Fatal("expected default scenario to be created")
	}
	if scenario.MerchantID != "MRC_123" {
		t.Fatalf("expected merchant ID to be preserved, got %q", scenario.MerchantID)
	}
}

func TestScenarioServiceGetReturnsRepositoryError(t *testing.T) {
	repo := &fakeScenarioRepo{findErr: errors.New("db unavailable")}
	svc := &ScenarioService{scenarioRepo: repo}

	_, err := svc.Get("MRC_123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if repo.createCalled {
		t.Fatal("did not expect create on repository failure")
	}
}
