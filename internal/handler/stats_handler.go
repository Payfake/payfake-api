package handler

import (
	"net/http"

	"github.com/GordenArcher/payfake/internal/middleware"
	"github.com/GordenArcher/payfake/internal/response"
	"github.com/GordenArcher/payfake/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StatsHandler struct {
	db       *gorm.DB
	statsSvc *service.StatsService
	authSvc  *service.AuthService
}

func NewStatsHandler(db *gorm.DB, statsSvc *service.StatsService, authSvc *service.AuthService) *StatsHandler {
	return &StatsHandler{db: db, statsSvc: statsSvc, authSvc: authSvc}
}

// GetStats handles GET /api/v1/control/stats
// Powers the dashboard overview page, returns all aggregated numbers
// in one call so the overview doesn't need to fire multiple requests.
func (h *StatsHandler) GetStats(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	stats, err := h.statsSvc.GetStats(merchantID)
	if err != nil {
		response.InternalErr(c, "Failed to fetch stats")
		return
	}

	response.Success(c, http.StatusOK, "Stats fetched",
		response.StatsFetched, gin.H{
			"transactions": gin.H{
				"total":        stats.TotalTransactions,
				"successful":   stats.SuccessfulCount,
				"failed":       stats.FailedCount,
				"pending":      stats.PendingCount,
				"abandoned":    stats.AbandonedCount,
				"success_rate": stats.SuccessRate,
			},
			"volume": gin.H{
				"total_amount": stats.TotalVolume,
			},
			"customers": gin.H{
				"total": stats.TotalCustomers,
			},
			"webhooks": gin.H{
				"total":     stats.TotalWebhooks,
				"delivered": stats.DeliveredWebhooks,
				"failed":    stats.FailedWebhooks,
			},
			"daily_activity": stats.DailyActivity,
		})
}
