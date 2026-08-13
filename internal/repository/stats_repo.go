package repository

import (
	"fmt"
	"time"

	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/gorm"
)

type StatsRepository struct {
	db *gorm.DB
}

func NewStatsRepository(db *gorm.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

func (r *StatsRepository) GetMerchantStats(merchantID string) (*domain.MerchantStats, error) {
	stats := &domain.MerchantStats{}

	type txCounts struct {
		Total     int64
		Success   int64
		Failed    int64
		Pending   int64
		Abandoned int64
		Reversed  int64
		Volume    int64
	}
	var counts txCounts

	err := r.db.Raw(`
		SELECT
			COUNT(*)                                                       AS total,
			COUNT(*) FILTER (WHERE status = 'success')                    AS success,
			COUNT(*) FILTER (WHERE status = 'failed')                     AS failed,
			COUNT(*) FILTER (WHERE status = 'pending')                    AS pending,
			COUNT(*) FILTER (WHERE status = 'abandoned')                  AS abandoned,
			COUNT(*) FILTER (WHERE status = 'reversed')                   AS reversed,
			COALESCE(SUM(amount) FILTER (WHERE status = 'success'), 0)    AS volume
		FROM transactions
		WHERE merchant_id = ? AND deleted_at IS NULL
	`, merchantID).Scan(&counts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction counts: %w", err)
	}

	stats.TotalTransactions = counts.Total
	stats.SuccessfulCount = counts.Success
	stats.FailedCount = counts.Failed
	stats.PendingCount = counts.Pending
	stats.AbandonedCount = counts.Abandoned
	stats.ReversedCount = counts.Reversed
	stats.TotalVolume = counts.Volume

	if counts.Total > 0 {
		stats.SuccessRate = float64(counts.Success) / float64(counts.Total) * 100
	}

	if err := r.db.Raw(`
		SELECT COUNT(*) FROM customers
		WHERE merchant_id = ? AND deleted_at IS NULL
	`, merchantID).Scan(&stats.TotalCustomers).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch customer count: %w", err)
	}

	type webhookCounts struct {
		Total     int64
		Delivered int64
		Failed    int64
	}
	var wCounts webhookCounts
	if err := r.db.Raw(`
		SELECT
			COUNT(*)                                   AS total,
			COUNT(*) FILTER (WHERE delivered = true)   AS delivered,
			COUNT(*) FILTER (WHERE delivered = false)  AS failed
		FROM webhook_events
		WHERE merchant_id = ? AND deleted_at IS NULL
	`, merchantID).Scan(&wCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch webhook counts: %w", err)
	}

	stats.TotalWebhooks = wCounts.Total
	stats.DeliveredWebhooks = wCounts.Delivered
	stats.FailedWebhooks = wCounts.Failed

	type dailyRow struct {
		Date   string
		Count  int64
		Volume int64
	}
	var dailyRows []dailyRow

	if err := r.db.Raw(`
		SELECT
			TO_CHAR(DATE_TRUNC('day', created_at), 'YYYY-MM-DD')          AS date,
			COUNT(*)                                                        AS count,
			COALESCE(SUM(amount) FILTER (WHERE status = 'success'), 0)    AS volume
		FROM transactions
		WHERE merchant_id = ?
		  AND deleted_at IS NULL
		  AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY DATE_TRUNC('day', created_at)
		ORDER BY date ASC
	`, merchantID).Scan(&dailyRows).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch daily activity: %w", err)
	}

	dailyMap := make(map[string]domain.DailyActivity)
	for _, row := range dailyRows {
		dailyMap[row.Date] = domain.DailyActivity{
			Date:   row.Date,
			Count:  row.Count,
			Volume: row.Volume,
		}
	}

	activity := make([]domain.DailyActivity, 7)
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if entry, ok := dailyMap[date]; ok {
			activity[6-i] = entry
		} else {
			activity[6-i] = domain.DailyActivity{Date: date, Count: 0, Volume: 0}
		}
	}
	stats.DailyActivity = activity

	return stats, nil
}
