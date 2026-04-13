package domain

// MerchantStats holds all aggregated numbers the dashboard overview needs.
// Defined in domain so both service and repository can reference it
// without creating an import cycle.
type MerchantStats struct {
	TotalTransactions int64
	SuccessfulCount   int64
	FailedCount       int64
	PendingCount      int64
	AbandonedCount    int64
	TotalVolume       int64
	SuccessRate       float64
	TotalCustomers    int64
	TotalWebhooks     int64
	DeliveredWebhooks int64
	FailedWebhooks    int64
	DailyActivity     []DailyActivity
}

type DailyActivity struct {
	Date   string `json:"date"`
	Count  int64  `json:"count"`
	Volume int64  `json:"volume"`
}
