package service

import (
	"github.com/payfake/payfake-api/internal/domain"
	"github.com/payfake/payfake-api/internal/repository"
)

type LogService struct {
	logRepo *repository.LogRepository
}

func NewLogService(logRepo *repository.LogRepository) *LogService {
	return &LogService{logRepo: logRepo}
}

// List returns paginated request logs for a merchant.
func (s *LogService) List(merchantID string, page, perPage int) ([]domain.RequestLog, int64, error) {
	offset := (page - 1) * perPage
	return s.logRepo.List(merchantID, offset, perPage)
}

// Clear permanently deletes all logs for a merchant.
func (s *LogService) Clear(merchantID string) error {
	return s.logRepo.ClearByMerchant(merchantID)
}
