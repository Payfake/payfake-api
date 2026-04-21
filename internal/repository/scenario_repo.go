package repository

import (
	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
)

type ScenarioRepository struct {
	db *gorm.DB
}

func NewScenarioRepository(db *gorm.DB) *ScenarioRepository {
	return &ScenarioRepository{db: db}
}

// FindByMerchantID retrieves the scenario config for a merchant.
// Each merchant has exactly one scenario config, the uniqueIndex
// on merchant_id in the domain struct enforces this at the DB level.
func (r *ScenarioRepository) FindByMerchantID(merchantID string) (*domain.ScenarioConfig, error) {
	var scenario domain.ScenarioConfig
	result := r.db.Where("merchant_id = ?", merchantID).First(&scenario)
	if result.Error != nil {
		return nil, result.Error
	}
	return &scenario, nil
}

// Create inserts a new scenario config record.
func (r *ScenarioRepository) Create(scenario *domain.ScenarioConfig) error {
	return r.db.Create(scenario).Error
}

// Update saves changes to an existing scenario config.
func (r *ScenarioRepository) Update(scenario *domain.ScenarioConfig) error {
	return r.db.Save(scenario).Error
}
