package service

import (
	"github.com/GordenArcher/payfake/internal/domain"
	"github.com/GordenArcher/payfake/internal/repository"
)

type StatsService struct {
	statsRepo *repository.StatsRepository
}

func NewStatsService(statsRepo *repository.StatsRepository) *StatsService {
	return &StatsService{statsRepo: statsRepo}
}

// MerchantStats holds all aggregated numbers the dashboard overview needs.
type MerchantStats struct {
	// Transaction counts by status
	TotalTransactions int64
	SuccessfulCount   int64
	FailedCount       int64
	PendingCount      int64
	AbandonedCount    int64

	// Volume = total amount across successful transactions only.
	// Amounts are in smallest currency unit (pesewas).
	TotalVolume int64

	// Success rate as a percentage = 0 to 100.
	SuccessRate float64

	// Customer count
	TotalCustomers int64

	// Webhook stats
	TotalWebhooks     int64
	DeliveredWebhooks int64
	FailedWebhooks    int64

	// Recent activity = last 7 days daily breakdown for the chart.
	// Each entry is one day: date + transaction count + volume.
	DailyActivity []DailyActivity
}

type DailyActivity struct {
	Date   string `json:"date"`
	Count  int64  `json:"count"`
	Volume int64  `json:"volume"`
}

func (s *StatsService) GetStats(merchantID string) (*domain.MerchantStats, error) {
	return s.statsRepo.GetMerchantStats(merchantID)
}
